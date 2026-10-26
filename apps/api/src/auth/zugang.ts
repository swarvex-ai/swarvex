// Prüft Bearer-Tokens für die MCP-Ressource und ordnet sie Mensch und Agent zu.
// Token = JWT des eigenen OAuth-Servers mit Zielgruppe <basis>/mcp und Scope "mcp".
import { createLocalJWKSet, jwtVerify } from "jose";
import type Provider from "oidc-provider";
import { transaktion, type Db } from "../db.ts";
import { protokollieren } from "../ereignisse.ts";
import { konfig, RESSOURCE } from "../konfig.ts";
import { oeffentlich } from "./schluessel.ts";

export type Zugang = { menschId: number; agentId: number; clientId: string; token: string; scopes: string[]; ablauf: number };

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
    const agentId = await agentZuordnen(db, anbieter, menschId, clientId);
    // Auf der Konto-Seite abgemeldet: Tokens von vorher gelten nicht mehr, auch wenn sie noch nicht abgelaufen sind
    const abgemeldet: Date | null = (await db.query("select abgemeldet from agenten where id = $1", [agentId])).rows[0]?.abgemeldet ?? null;
    if (abgemeldet && Number(claims.iat) * 1000 <= abgemeldet.getTime()) throw new NichtAngemeldet("agent was signed out; please sign in again");
    return { menschId, agentId, clientId, token, scopes, ablauf: Number(claims.exp) };
  };
}

/** Jeder OAuth-Client eines Menschen ist ein Agent; beim ersten Aufruf wird er angelegt. */
async function agentZuordnen(db: Db, anbieter: Provider, menschId: number, clientId: string): Promise<number> {
  // Dynamisch registrierte Clients gehören genau einem Menschen; URL-Clients (Metadata-Dokument)
  // teilen sich alle Nutzer eines Agenten, dort ist der Agent das Paar (Mensch, Client).
  const vorhanden = (await db.query("select id, mensch_id from agenten where client_id = $1", [clientId])).rows;
  const eigener = vorhanden.find((a) => Number(a.mensch_id) === menschId);
  if (eigener) return Number(eigener.id);
  if (vorhanden.length && !clientId.startsWith("https://")) throw new NichtAngemeldet("client belongs to another person");
  const client = await anbieter.Client.find(clientId);
  const name = client?.clientName ?? clientId;
  return transaktion(db, async (tx) => {
    const id = Number((await tx.query("insert into agenten(mensch_id, client_id, name) values ($1,$2,$3) on conflict (mensch_id, client_id) do update set name = excluded.name returning id", [menschId, clientId, name])).rows[0].id);
    await protokollieren(tx, { projekt_id: null, akteur: `mensch:${menschId}`, art: "agent_angebunden", bezug: `agent:${id}`, daten: { name, client_id: clientId } });
    return id;
  });
}
