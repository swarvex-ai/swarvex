// Prüft Bearer-Tokens für die MCP-Ressource und ordnet sie Mensch und Agent zu.
// Token = JWT des eigenen OAuth-Servers mit Zielgruppe <basis>/mcp und Scope "mcp".
import { createLocalJWKSet, jwtVerify } from "jose";
import type Provider from "oidc-provider";
import { transaktion, type Db } from "../db.ts";
import { protokollieren } from "../ereignisse.ts";
import { konfig, RESSOURCE } from "../konfig.ts";
import { oeffentlich } from "./schluessel.ts";

export type Zugang = { menschId: number; agentId: number; runner: boolean; clientId: string; token: string; scopes: string[]; ablauf: number };

export class NichtAngemeldet extends Error {}

export function zugangPruefer(db: Db, anbieter: Provider, signaturJwk: Record<string, unknown>) {
  const jwks = createLocalJWKSet({ keys: [oeffentlich(signaturJwk) as any] });
  return async (authorization: string | undefined): Promise<Zugang> => {
    const token = /^Bearer (.+)$/i.exec(authorization ?? "")?.[1];
    if (!token) throw new NichtAngemeldet("no token");
    let claims;
    try {
      ({ payload: claims } = await jwtVerify(token, jwks, { issuer: konfig.basis, audience: RESSOURCE(), algorithms: ["ES256"] }));
    } catch (e) {
      throw new NichtAngemeldet(`invalid token: ${(e as Error).message}`);
    }
    const scopes = String(claims.scope ?? "").split(" ");
    if (!scopes.includes("mcp")) throw new NichtAngemeldet("scope mcp missing");
    const menschId = Number(claims.sub);
    const clientId = String(claims.client_id);
    const m = (await db.query("select gesperrt from menschen where id = $1", [menschId])).rows[0];
    if (!m || m.gesperrt) throw new NichtAngemeldet("account unknown or blocked");
    const { id: agentId, runner } = await agentZuordnen(db, anbieter, menschId, clientId);
    // Auf der Konto-Seite abgemeldet: Tokens von vorher gelten nicht mehr, auch wenn sie noch nicht abgelaufen sind
    const abgemeldet: Date | null = (await db.query("select abgemeldet from agenten where id = $1", [agentId])).rows[0]?.abgemeldet ?? null;
    if (abgemeldet && Number(claims.iat) * 1000 <= abgemeldet.getTime()) throw new NichtAngemeldet("agent was signed out; please sign in again");
    return { menschId, agentId, runner, clientId, token, scopes, ablauf: Number(claims.exp) };
  };
}

/** Der Runner meldet sich per Device Flow an (runner/zugang.go); direkt angebundene Agenten über den Browser-Ablauf. */
async function istRunner(anbieter: Provider, clientId: string): Promise<boolean> {
  const client = await anbieter.Client.find(clientId);
  return (client?.grantTypes ?? []).includes("urn:ietf:params:oauth:grant-type:device_code");
}

/** Jeder OAuth-Client eines Menschen ist ein Agent; beim ersten Aufruf wird er angelegt. */
async function agentZuordnen(db: Db, anbieter: Provider, menschId: number, clientId: string): Promise<{ id: number; runner: boolean }> {
  // Dynamisch registrierte Clients gehören genau einem Menschen; URL-Clients (Metadata-Dokument)
  // teilen sich alle Nutzer eines Agenten, dort ist der Agent das Paar (Mensch, Client).
  const vorhanden = (await db.query("select id, mensch_id, runner from agenten where client_id = $1", [clientId])).rows;
  const eigener = vorhanden.find((a) => Number(a.mensch_id) === menschId);
  if (eigener) {
    if (eigener.runner !== null) return { id: Number(eigener.id), runner: eigener.runner };
    const runner = await istRunner(anbieter, clientId);
    await db.query("update agenten set runner = $2 where id = $1", [eigener.id, runner]);
    return { id: Number(eigener.id), runner };
  }
  if (vorhanden.length && !clientId.startsWith("https://")) throw new NichtAngemeldet("client belongs to another person");
  const client = await anbieter.Client.find(clientId);
  const name = client?.clientName ?? clientId;
  const runner = (client?.grantTypes ?? []).includes("urn:ietf:params:oauth:grant-type:device_code");
  return transaktion(db, async (tx) => {
    const id = Number((await tx.query("insert into agenten(mensch_id, client_id, name, runner) values ($1,$2,$3,$4) on conflict (mensch_id, client_id) do update set name = excluded.name, runner = excluded.runner returning id", [menschId, clientId, name, runner])).rows[0].id);
    await protokollieren(tx, { projekt_id: null, akteur: `mensch:${menschId}`, art: "agent_angebunden", bezug: `agent:${id}`, daten: { name, client_id: clientId, runner } });
    return { id, runner };
  });
}
