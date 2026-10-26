// Einstiegspunkt der Plattform: ein HTTP-Server hinter Caddy (TLS).
//   /mcp                                  MCP-Gateway (Bearer-Token)
//   /.well-known/oauth-protected-resource Metadaten der Ressource (RFC 9728)
//   /.well-known/oauth-authorization-server Metadaten des OAuth-Servers (RFC 8414)
//   /tasks/<id>/snapshot                  Projektstand als tar.gz (nur Lease-Inhaber)
//   /tasks/<id>/lfs/<oid>                 ein Git-LFS-Objekt (nur Lease-Inhaber)
//   /api/projects[/<slug>[/metrics]]      öffentliche Projektdaten (Website, Rangliste)
//   /api/account/…, /api/reports, /api/log/…  Mein Konto, Meldeformular, veröffentlichte Kopf-Hashes
//   /pipeline/…                           Prüfstraße und Merge-Bot (HMAC-signiert)
//   /interaction/…                        Anmeldung über GitHub, Codeberg oder Gitea
//   /healthz                              Lebenszeichen
//   alles andere                          OAuth-/OIDC-Server (oidc-provider)
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { PgBoss } from "pg-boss";
import { konfig, RESSOURCE } from "./konfig.ts";
import { migrieren, verbinden } from "./db.ts";
import { Abgelehnt, leasesAufraeumen } from "./aufgaben.ts";
import { lfsObjektSenden, snapshotSenden } from "./snapshot.ts";
import { pruefstrasseBehandeln } from "./pruefstrasse.ts";
import { kennzahlen, projektDetail, projektListe } from "./kennzahlen.ts";
import { nachAussen } from "./aussen.ts";
import { schluesselLaden } from "./auth/schluessel.ts";
import { anbieterErstellen } from "./auth/anbieter.ts";
import { interaktionBehandeln } from "./auth/interaktion.ts";
import { kontoAnbieterLaden } from "./auth/konten.ts";
import { NichtAngemeldet, zugangPruefer } from "./auth/zugang.ts";
import { mcpBehandeln } from "./mcp.ts";
import { aufruferIp, grenzenErstellen } from "./grenzen.ts";
import { meldungAnlegen } from "./meldungen.ts";
import { kontoBehandeln } from "./konto.ts";
import { hashesAn, koepfeListe, kopfVeroeffentlichen } from "./koepfe.ts";
import { ZodError } from "zod";

const db = verbinden(konfig.datenbank);
const neu = await migrieren(db);
if (neu.length) console.log("migrations:", neu.join(", "));

const schluessel = await schluesselLaden(db);
const anbieter = anbieterErstellen(db, schluessel);
const konten = kontoAnbieterLaden();
const interaktion = interaktionBehandeln(anbieter, db, schluessel.cookies[0], konten);
console.log(`account providers: ${konten.map((k) => k.name).join(", ")}`);
const zugang = zugangPruefer(db, anbieter, schluessel.signatur);
const konto = kontoBehandeln(db, schluessel.cookies[0], konten);
const oidc = anbieter.callback();
const pruefstrasse = pruefstrasseBehandeln(db, konfig.pruefstrasseGeheimnis);

// Abgelaufene Reservierungen jede Minute zurück in den Pool (pg-boss in derselben Datenbank).
const boss = new PgBoss(konfig.datenbank);
boss.on("error", (e) => console.error("pg-boss:", e));
await boss.start();
await boss.createQueue("leases-aufraeumen");
await boss.schedule("leases-aufraeumen", "* * * * *");
await boss.work("leases-aufraeumen", async () => {
  const n = await leasesAufraeumen(db);
  if (n) console.log(`lease cleanup: ${n} expired`);
});
// Kopf-Hash täglich signieren (03:17 UTC); der externe Anker holt ihn danach ab.
await boss.createQueue("kopf-veroeffentlichen");
await boss.schedule("kopf-veroeffentlichen", "17 3 * * *");
await boss.work("kopf-veroeffentlichen", async () => {
  const k = await kopfVeroeffentlichen(db, schluessel.signatur);
  if (k) console.log(`head published: seq ${k.seq} ${k.hash.slice(0, 16)}…`);
});

const grenze = grenzenErstellen();

const RESSOURCEN_METADATEN = `${konfig.basis}/.well-known/oauth-protected-resource`;

function json(res: ServerResponse, status: number, wert: unknown, kopf: Record<string, string> = {}) {
  res.writeHead(status, { "content-type": "application/json", ...kopf });
  res.end(JSON.stringify(wert));
}

async function koerper(req: IncomingMessage): Promise<unknown> {
  const teile: Buffer[] = [];
  let groesse = 0;
  for await (const t of req) {
    groesse += (t as Buffer).length;
    if (groesse > 2_000_000) throw new Error("request too large");
    teile.push(t as Buffer);
  }
  const roh = Buffer.concat(teile).toString("utf8");
  return roh ? JSON.parse(roh) : undefined;
}

const server = createServer(async (req, res) => {
  const url = new URL(req.url ?? "/", konfig.basis);
  const ip = aufruferIp(req);
  /** true, wenn die Anfrage am Ratenlimit scheitert (Antwort 429 ist dann schon gesendet). */
  const gebremst = (regel: Parameters<typeof grenze>[0], schluessel: string) => {
    const warten = grenze(regel, schluessel);
    if (!warten) return false;
    json(res, 429, { error: "rate_limited", error_description: `Too many requests, retry in ${warten} s` }, { "retry-after": String(warten) });
    return true;
  };
  try {
    if (url.pathname === "/healthz") return json(res, 200, { ok: true });
    if (url.pathname === "/reg" && req.method === "POST" && gebremst("registrierung", ip)) return;
    if ((/^\/(interaction|auth|token|device)(\/|$)/.test(url.pathname) || url.pathname === "/api/account/login") && gebremst("anmeldung", ip)) return;
    if (url.pathname.startsWith("/api/") && gebremst("oeffentlich", ip)) return;

    if (url.pathname.startsWith("/.well-known/oauth-protected-resource")) {
      return json(res, 200, { resource: RESSOURCE(), authorization_servers: [konfig.basis], scopes_supported: ["mcp"], bearer_methods_supported: ["header"] });
    }
    if (url.pathname.startsWith("/.well-known/oauth-authorization-server")) {
      // Gleicher Inhalt wie die OIDC-Discovery; manche MCP-Clients fragen nur RFC 8414 ab.
      req.url = "/.well-known/openid-configuration";
      return oidc(req, res);
    }

    if (url.pathname === "/mcp") {
      if (req.method !== "POST") return json(res, 405, { error: "nur POST (zustandsloses Streamable HTTP)" }, { allow: "POST" });
      if (gebremst("mcpIp", ip)) return;
      let z;
      try { z = await zugang(req.headers.authorization); }
      catch (e) {
        if (e instanceof NichtAngemeldet) {
          return json(res, 401, { error: "invalid_token", error_description: e.message },
            { "www-authenticate": `Bearer resource_metadata="${RESSOURCEN_METADATEN}", scope="mcp"` });
        }
        throw e;
      }
      if (gebremst("mcp", `agent:${z.agentId}`)) return;
      return await mcpBehandeln(db, z, req, res, await koerper(req));
    }

    const snap = /^\/tasks\/(\d+)\/(snapshot|lfs\/([0-9a-f]{64}))$/.exec(url.pathname);
    if (snap && req.method === "GET") {
      let z;
      try { z = await zugang(req.headers.authorization); }
      catch (e) { if (e instanceof NichtAngemeldet) return json(res, 401, { error: "invalid_token" }); throw e; }
      try {
        if (snap[3]) return await lfsObjektSenden(db, z.agentId, Number(snap[1]), snap[3], res);
        return await snapshotSenden(db, z.agentId, Number(snap[1]), res);
      }
      catch (e) { if (e instanceof Abgelehnt) return json(res, 403, { error: e.message }); throw e; }
    }

    if (url.pathname.startsWith("/pipeline/")) return await pruefstrasse(req, res, url);

    // Veröffentlichte Kopf-Hashes des Protokolls (für den externen Anker und alle, die nachprüfen wollen)
    if (req.method === "GET" && url.pathname === "/api/log/head") {
      const k = (await koepfeListe(db, 1))[0];
      return k ? json(res, 200, nachAussen(k), { "cache-control": "public, max-age=300", "access-control-allow-origin": "*" }) : json(res, 404, { error: "no_head_yet" });
    }
    if (req.method === "GET" && url.pathname === "/api/log/heads") {
      return json(res, 200, nachAussen(await koepfeListe(db)), { "cache-control": "public, max-age=300", "access-control-allow-origin": "*" });
    }
    if (req.method === "GET" && url.pathname === "/api/log/hashes") {
      const seqs = (url.searchParams.get("seq") ?? "").split(",").map(Number).filter((n) => Number.isInteger(n) && n > 0).slice(0, 200);
      return json(res, 200, await hashesAn(db, seqs), { "cache-control": "no-store", "access-control-allow-origin": "*" });
    }

    // „Mein Konto“: Website-Anmeldung, eigene Agenten und Beiträge
    if (url.pathname === "/api/account" || url.pathname.startsWith("/api/account/")) return await konto(req, res, url);

    // Meldeformular der Website (Rechtsverletzung, Missbrauch, Sicherheit)
    if (url.pathname === "/api/reports" && req.method === "POST") {
      if (gebremst("meldung", ip)) return;
      try {
        return json(res, 201, await meldungAnlegen(db, await koerper(req)));
      } catch (e) {
        if (e instanceof ZodError) return json(res, 400, { error: "invalid", fields: e.issues.map((i) => ({ field: i.path.join("."), message: i.message })) });
        if (e instanceof SyntaxError) return json(res, 400, { error: "invalid" });
        throw e;
      }
    }

    // Öffentliche Rangliste für die Website (gleiche Zahlen wie Board und MCP)
    if (req.method === "GET" && url.pathname === "/api/projects") {
      return json(res, 200, nachAussen(await projektListe(db)), { "cache-control": "public, max-age=10", "access-control-allow-origin": "*" });
    }
    // Projektseite der Website bzw. nur die Kennzahlen (dieselben wie das MCP-Werkzeug `metrics`)
    const detail = /^\/api\/projects\/([a-z0-9-]+)(\/metrics)?$/.exec(url.pathname);
    if (req.method === "GET" && detail) {
      try { return json(res, 200, nachAussen(detail[2] ? await kennzahlen(db, detail[1]) : await projektDetail(db, detail[1])), { "cache-control": "public, max-age=10", "access-control-allow-origin": "*" }); }
      catch (e) { if (e instanceof Abgelehnt) return json(res, 404, { error: "project_not_found" }); throw e; }
    }

    if (url.pathname.startsWith("/interaction/")) return await interaktion(req, res, url);

    return oidc(req, res);
  } catch (e) {
    console.error(e);
    if (!res.headersSent) json(res, 500, { error: "server_error" });
  }
});

server.listen(konfig.port, () => console.log(`swarvex listening on :${konfig.port} (${konfig.basis})`));

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.on(signal, async () => { server.close(); await boss.stop(); await db.end(); process.exit(0); });
}
