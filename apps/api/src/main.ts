// Einstiegspunkt der Plattform: ein HTTP-Server hinter Caddy (TLS).
//   /mcp                                  MCP-Gateway (Bearer-Token)
//   /.well-known/oauth-protected-resource Metadaten der Ressource (RFC 9728)
//   /.well-known/oauth-authorization-server Metadaten des OAuth-Servers (RFC 8414)
//   /tasks/<id>/snapshot                  Projektstand als tar.gz (nur Lease-Inhaber)
//   /submissions/<id>/images/<szene>/<old|new|diff>.png  Szenenbild (Lead, Bereichs-Lead, Fremdprüfer; Agenten-Token)
//   /tasks/<id>/lfs/<oid>                 ein Git-LFS-Objekt (nur Lease-Inhaber)
//   /donations/<id>/file                  hochgeladene Credit-Spende (nur Lead, Visionär, Bereichs-Leads)
//   /donations/<id>/upload (PUT)          Ergebnis eines Medien-Auftrags vom Runner (Zusage über den Runner)
//   /lead-service/git (POST)              Git-Zugang für das Repository eines Lead-Dienstes (an ein Projekt gebundene Anmeldung)
//   /api/projects[/<slug>[/metrics]]      öffentliche Projektdaten (Website, Rangliste)
//   /api/account/…, /api/reports[/challenge], /api/log/…  Mein Konto, Meldeformular (mit Captcha), veröffentlichte Kopf-Hashes
//   /pipeline/…                           Prüfstraße und Merge-Bot (HMAC-signiert)
//   /interaction/…                        Anmeldung über GitHub, Codeberg oder Gitea
//   /healthz                              Lebenszeichen
//   alles andere                          OAuth-/OIDC-Server (oidc-provider)
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { PgBoss } from "pg-boss";
import { konfig, RESSOURCE } from "./konfig.ts";
import { migrieren, verbinden } from "./db.ts";
import { Abgelehnt, leasesAufraeumen } from "./aufgaben.ts";
import { besucheAbschliessen, besuchZaehlen, zaehlbar } from "./besuche.ts";
import { ohneGegenpruefungFreigeben } from "./gegenpruefung.ts";
import { lfsObjektSenden, snapshotSenden } from "./snapshot.ts";
import { aufraeumen as spendenAufraeumen, dateiSenden as spendeSenden, hochladen as spendeHochladen } from "./spenden.ts";
import { pruefstrasseBehandeln } from "./pruefstrasse.ts";
import { leadAlarme, leadGitZugang } from "./leaddienst.ts";
import { leadKraftPruefen } from "./leadkraft.ts";
import { ideenFreigeben, ideenListe } from "./ideen.ts";
import { bedarfAktualisieren } from "./bedarf.ts";
import { bilderAufraeumen, bilderErlaubt, bildLesen } from "./bilder.ts";
import { fremdpruefungErlaubt } from "./schwarmpruefung.ts";
import { abzeichen, kennzahlen, kurzlinkZiel, projektDetail, projektListe, schwarmLive } from "./kennzahlen.ts";
import { englisch } from "./auth/interaktion.ts";
import { nachAussen } from "./aussen.ts";
import { schluesselLaden } from "./auth/schluessel.ts";
import { anbieterErstellen } from "./auth/anbieter.ts";
import { interaktionBehandeln } from "./auth/interaktion.ts";
import { kontoAnbieterLaden } from "./auth/konten.ts";
import { NichtAngemeldet, zugangPruefer } from "./auth/zugang.ts";
import { mcpBehandeln } from "./mcp.ts";
import { aufruferIp, grenzenErstellen } from "./grenzen.ts";
import { meldungAnlegen, MeldungSchema } from "./meldungen.ts";
import { captchaAufgabe, captchaPruefen } from "./captcha.ts";
import { kontoBehandeln } from "./konto.ts";
import { alleVertrauenAktualisieren } from "./vertrauen.ts";
import { aufraeumen as bereichsLeadsAufraeumen, vorschlaegeErzeugen } from "./bereichsleads.ts";
import { hashesAn, koepfeListe, kopfVeroeffentlichen } from "./koepfe.ts";
import { repoDaten } from "./repo.ts";
import { modellRangliste } from "./modelle.ts";
import { aufgabenFeed } from "./feed.ts";
import { profil } from "./profil.ts";
import { vorschauErlaubt } from "./pruefstrasse.ts";
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
const pruefstrasse = pruefstrasseBehandeln(db, konfig.pruefstrasseGeheimnis, konfig.pruefServerGeheimnis);

// Abgelaufene Reservierungen jede Minute zurück in den Pool (pg-boss in derselben Datenbank).
const boss = new PgBoss(konfig.datenbank);
boss.on("error", (e) => console.error("pg-boss:", e));
await boss.start();
await boss.createQueue("leases-aufraeumen");
await boss.schedule("leases-aufraeumen", "* * * * *");
await boss.work("leases-aufraeumen", async () => {
  const n = await leasesAufraeumen(db);
  if (n) console.log(`lease cleanup: ${n} expired`);
  const f = await ohneGegenpruefungFreigeben(db);
  if (f) console.log(`cross-check: ${f} submission(s) released with the contributor's own run only`);
});
// Besucherstatistik: abgeschlossene Tage behalten nur die Zahl, Tagesgeheimnis und Hashes werden gelöscht
await boss.createQueue("besuche-abschliessen");
await boss.schedule("besuche-abschliessen", "7 * * * *");
await boss.work("besuche-abschliessen", async () => { await besucheAbschliessen(db); });
// Kopf-Hash täglich signieren (03:17 UTC); der externe Anker holt ihn danach ab.
await boss.createQueue("lead-anwesenheit");
await boss.schedule("lead-anwesenheit", "*/5 * * * *");
await boss.work("lead-anwesenheit", async () => {
  const n = await leadAlarme(db);
  if (n) console.log(`lead services not reachable: ${n} visionaries told`);
  // Bedarf zwischen Projekten: fertige Lieferungen erledigt, geschlossene Aufgaben geben den Bedarf wieder frei
  const b = await bedarfAktualisieren(db);
  if (b.erledigt || b.wieder_offen) console.log(`requests between projects: ${b.erledigt} done, ${b.wieder_offen} open again`);
});
// Lead-Kraft (E-39): Tagesstand der Signale; halten sie an, schlägt die Plattform Lead und Visionär mehr Lead vor
await boss.createQueue("lead-kraft");
await boss.schedule("lead-kraft", "53 5 * * *");
await boss.work("lead-kraft", async () => {
  const n = await leadKraftPruefen(db);
  const frei = await ideenFreigeben(db);
  await bilderAufraeumen(db);
  if (n || frei) console.log(`lead power: ${n} project(s) told, ${frei} project idea(s) released from stale drafts`);
});
await boss.createQueue("kopf-veroeffentlichen");
// Dateien entschiedener Credit-Spenden nach 30 Tagen löschen
await boss.createQueue("spenden-aufraeumen");
await boss.schedule("spenden-aufraeumen", "41 4 * * *");
await boss.work("spenden-aufraeumen", async () => { const n = await spendenAufraeumen(db); if (n) console.log(`donation files removed: ${n}`); });
await boss.schedule("kopf-veroeffentlichen", "17 3 * * *");
// Vertrauensstufen: Zeitfenster laufen ab (alte Ablehnungen blockieren den Aufstieg nicht mehr)
await boss.createQueue("vertrauen");
await boss.schedule("vertrauen", "29 3 * * *");
await boss.work("vertrauen", async () => { const n = await alleVertrauenAktualisieren(db); if (n) console.log(`trust levels changed: ${n}`); });
// Bereichs-Leads: Vorschläge, wo die Lead-Bilanz es empfiehlt; verfallene Vorschläge und ruhige Amtszeiten beenden
await boss.createQueue("bereichs-leads");
await boss.schedule("bereichs-leads", "41 3 * * *");
await boss.work("bereichs-leads", async () => {
  const beendet = await bereichsLeadsAufraeumen(db);
  const neu = await vorschlaegeErzeugen(db);
  if (beendet || neu) console.log(`area leads: ${neu} invited, ${beendet} ended for inactivity`);
});
await boss.work("kopf-veroeffentlichen", async () => {
  const k = await kopfVeroeffentlichen(db, schluessel.signatur);
  if (k) console.log(`head published: seq ${k.seq} ${k.hash.slice(0, 16)}…`);
});

const grenze = grenzenErstellen();
// Ratenlimit-Treffer je Regel, einmal pro Minute als eine Zeile ins Log (Sicherheits-Dashboard, ohne Adressen)
const gebremstZaehler = new Map<string, number>();
setInterval(() => {
  for (const [regel, anzahl] of gebremstZaehler) console.log(JSON.stringify({ type: "rate_limited", rule: regel, count: anzahl }));
  gebremstZaehler.clear();
}, 60_000).unref();

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
    gebremstZaehler.set(regel, (gebremstZaehler.get(regel) ?? 0) + 1);
    json(res, 429, { error: "rate_limited", error_description: `Too many requests, retry in ${warten} s` }, { "retry-after": String(warten) });
    return true;
  };
  try {
    if (url.pathname === "/healthz") return json(res, 200, { ok: true });
    if (url.pathname === "/reg" && req.method === "POST" && gebremst("registrierung", ip)) return;
    if ((/^\/(interaction|auth|token|device)(\/|$)/.test(url.pathname) || url.pathname === "/api/account/login") && gebremst("anmeldung", ip)) return;
    if (url.pathname.startsWith("/api/") && gebremst("oeffentlich", ip)) return;
    // Besucherstatistik ohne Cookies (besuche.ts): immer 204, auch wenn nicht gezählt wird
    if (url.pathname === "/api/visit" && req.method === "POST") {
      if (zaehlbar(req.headers)) {
        try {
          const eingabe = (await koerper(req)) as { p?: unknown; r?: unknown } | undefined;
          await besuchZaehlen(db, eingabe ?? {}, ip, String(req.headers["user-agent"]), new URL(konfig.basis).hostname);
        } catch { /* eine Zählung darf nie eine Seite stören */ }
      }
      res.writeHead(204, { "cache-control": "no-store" }).end();
      return;
    }

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
      let eingabe;
      try { eingabe = await koerper(req); }
      catch { return json(res, 400, { jsonrpc: "2.0", error: { code: -32700, message: "parse error: invalid JSON or request too large" }, id: null }); }
      return await mcpBehandeln(db, z, req, res, eingabe);
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

    // Szenenbild einer Einreichung (bilder.ts) für Agenten: Lead, Visionär und Bereichs-Leads des Projekts, und wer die
    // Einreichung fremdbewerten darf (der Runner legt die Bilder seinem Prüfmodell vor, schwarmpruefung.ts)
    const bild = /^\/submissions\/(\d+)\/images\/([a-z0-9][a-z0-9_.-]{0,60})\/(old|new|diff)\.png$/.exec(url.pathname);
    if (bild && req.method === "GET") {
      let z;
      try { z = await zugang(req.headers.authorization); }
      catch (e) { if (e instanceof NichtAngemeldet) return json(res, 401, { error: "invalid_token" }); throw e; }
      const eid = Number(bild[1]);
      try {
        try {
          const projekt = await bilderErlaubt(db, z.menschId, eid);
          if (z.leadProjekt !== null && projekt !== z.leadProjekt) return json(res, 403, { error: "the lead service works only on its own project" });
        } catch (e) {
          if (!(e instanceof Abgelehnt) || z.leadProjekt !== null) throw e;
          await fremdpruefungErlaubt(db, { menschId: z.menschId, agentId: z.agentId }, eid);
        }
      } catch (e) { if (e instanceof Abgelehnt) return json(res, 403, { error: e.message }); throw e; }
      const png = await bildLesen(db, eid, bild[2], bild[3]);
      if (!png) return json(res, 404, { error: "no such image" });
      res.writeHead(200, { "content-type": "image/png", "cache-control": "private, max-age=3600", "content-security-policy": "default-src 'none'", "x-content-type-options": "nosniff" });
      res.end(png);
      return;
    }

    // Hochgeladene Credit-Spende herunterladen: nur Lead, Visionär oder Bereichs-Leads des Projekts (Agenten-Token)
    const spende = /^\/donations\/(\d+)\/file$/.exec(url.pathname);
    if (spende && req.method === "GET") {
      let z;
      try { z = await zugang(req.headers.authorization); }
      catch (e) { if (e instanceof NichtAngemeldet) return json(res, 401, { error: "invalid_token" }); throw e; }
      if (z.leadProjekt !== null) {
        const a = (await db.query("select projekt_id from spendenauftraege where id = $1", [Number(spende[1])])).rows[0];
        if (!a || Number(a.projekt_id) !== z.leadProjekt) return json(res, 403, { error: "the lead service works only on its own project" });
      }
      try { return await spendeSenden(db, z.menschId, Number(spende[1]), res); }
      catch (e) { if (e instanceof Abgelehnt) return json(res, 403, { error: e.message }); throw e; }
    }

    // Zusage über den Runner: der Runner lädt das Ergebnis eines begonnenen Medien-Auftrags hoch (Agenten-Token)
    const runnerUpload = /^\/donations\/(\d+)\/upload$/.exec(url.pathname);
    if (runnerUpload && req.method === "PUT") {
      let z;
      try { z = await zugang(req.headers.authorization); }
      catch (e) { if (e instanceof NichtAngemeldet) return json(res, 401, { error: "invalid_token" }); throw e; }
      if (z.runner !== true || z.leadProjekt !== null) return json(res, 403, { error: "only the swarvex runner uploads here" });
      if (gebremst("mcp", `agent:${z.agentId}`)) return;
      try { return json(res, 200, nachAussen(await spendeHochladen(db, z.menschId, Number(runnerUpload[1]), req, url.searchParams.get("name") ?? "", z.agentId))); }
      catch (e) { if (e instanceof Abgelehnt) return json(res, 403, { error: e.message }); throw e; }
    }

    // Lead-Dienst: Git-Zugang nur für das Repository seines Projekts (kurzlebiges Token der GitHub App)
    if (url.pathname === "/lead-service/git" && req.method === "POST") {
      let z;
      try { z = await zugang(req.headers.authorization); }
      catch (e) { if (e instanceof NichtAngemeldet) return json(res, 401, { error: "invalid_token" }); throw e; }
      if (z.leadProjekt === null) return json(res, 403, { error: "only a lead service bound to a project (swarvex project lead setup) gets git access here" });
      if (gebremst("mcp", `agent:${z.agentId}`)) return;
      try { return json(res, 200, nachAussen(await leadGitZugang(db, z.menschId, z.leadProjekt))); }
      catch (e) { if (e instanceof Abgelehnt) return json(res, 403, { error: e.message }); throw e; }
    }

    if (url.pathname.startsWith("/pipeline/")) {
      if (gebremst("pruefstrasse", ip)) return;
      return await pruefstrasse(req, res, url);
    }

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
    if (url.pathname === "/api/reports/challenge" && req.method === "GET") {
      return json(res, 200, captchaAufgabe(schluessel.cookies[0]), { "cache-control": "no-store" });
    }
    if (url.pathname === "/api/reports" && req.method === "POST") {
      if (gebremst("meldung", ip)) return;
      try {
        const eingabe = await koerper(req);
        const { altcha, ...meldung } = (eingabe ?? {}) as Record<string, unknown>;
        MeldungSchema.parse(meldung); // Feldfehler zuerst, damit sie die Captcha-Lösung nicht verbrauchen
        if (!captchaPruefen(schluessel.cookies[0], altcha)) return json(res, 400, { error: "captcha" });
        return json(res, 201, await meldungAnlegen(db, meldung));
      } catch (e) {
        if (e instanceof ZodError) return json(res, 400, { error: "invalid", fields: e.issues.map((i) => ({ field: i.path.join("."), message: i.message })) });
        if (e instanceof SyntaxError) return json(res, 400, { error: "invalid" });
        throw e;
      }
    }

    // Öffentliche Rangliste für die Website (gleiche Zahlen wie Board und MCP)
    // Sterne und Forks des Quellcode-Repos (zwischengespeichert, siehe repo.ts)
    if (req.method === "GET" && url.pathname === "/api/repo") {
      return json(res, 200, await repoDaten(), { "cache-control": "public, max-age=600" });
    }
    // Caddy fragt vor jedem Zertifikat für <slug>.<Vorschau-Domain>: nur für Projekte mit gebauter Vorschau (On-Demand-TLS)
    if (req.method === "GET" && url.pathname === "/api/preview-allowed") {
      const ok = await vorschauErlaubt(db, String(url.searchParams.get("domain") ?? ""));
      res.writeHead(ok ? 200 : 404, { "content-type": "text/plain" });
      return res.end(ok ? "ok" : "no");
    }
    // Öffentliches Beitragsprofil, nur wenn die Person es eingeschaltet hat (profil.ts)
    const person = /^\/api\/people\/([A-Za-z0-9_.-]{1,60})$/.exec(url.pathname);
    if (req.method === "GET" && person) {
      const p = await profil(db, person[1]);
      return p ? json(res, 200, nachAussen(p), { "cache-control": "public, max-age=60" }) : json(res, 404, { error: "no_public_profile" });
    }
    // Atom-Feed offener Aufgaben (feed.ts), gefiltert nach Projekt, Fähigkeit und Stufe
    if (req.method === "GET" && url.pathname === "/api/feed/tasks.atom") {
      const text = await aufgabenFeed(db, { projekt: url.searchParams.get("project"), faehigkeit: url.searchParams.get("capability"), stufe: url.searchParams.get("level") });
      res.writeHead(200, { "content-type": "application/atom+xml; charset=utf-8", "cache-control": "public, max-age=300", "access-control-allow-origin": "*" });
      return res.end(text);
    }
    // Modell-Rangliste aus echter Arbeit (modelle.ts), ohne personenbezogene Daten
    if (req.method === "GET" && url.pathname === "/api/models") {
      return json(res, 200, nachAussen(await modellRangliste(db)), { "cache-control": "public, max-age=300", "access-control-allow-origin": "*" });
    }
    // README-Abzeichen eines Projekts und Kurzlink swarvex.run/<slug> (infra/stack/vorschau-domain.caddy leitet hierher)
    const marke = /^\/api\/projects\/([a-z0-9-]+)\/badge\.svg$/.exec(url.pathname);
    if (req.method === "GET" && marke) {
      res.writeHead(200, { "content-type": "image/svg+xml; charset=utf-8", "cache-control": "public, max-age=300", "access-control-allow-origin": "*" });
      return res.end(await abzeichen(db, marke[1]));
    }
    const kurz = /^\/api\/go\/([a-z0-9-]{3,40})$/.exec(url.pathname);
    if (req.method === "GET" && kurz) {
      res.writeHead(302, { location: await kurzlinkZiel(db, kurz[1], englisch(req)), "cache-control": "public, max-age=60" });
      return res.end();
    }
    // Live-Ansicht der Startseite: letzte öffentliche Ereignisse, ohne Namen
    if (req.method === "GET" && url.pathname === "/api/activity") {
      return json(res, 200, nachAussen(await schwarmLive(db)), { "cache-control": "public, max-age=10", "access-control-allow-origin": "*" });
    }
    // Projektideen „Lead gesucht“ (ideen.ts): Katalog mit Stand, wer welche Idee übernommen hat
    if (req.method === "GET" && url.pathname === "/api/ideas") {
      return json(res, 200, nachAussen({ ideen: await ideenListe(db) }), { "cache-control": "public, max-age=30", "access-control-allow-origin": "*" });
    }
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
    console.error(JSON.stringify({ type: "server_error", path: url.pathname.replace(/\/\d+(?=\/|$)/g, "/:id").replace(/[0-9a-f]{32,}/g, ":hash").slice(0, 120) }));
    console.error(e);
    if (!res.headersSent) json(res, 500, { error: "server_error" });
  }
});

server.listen(konfig.port, () => console.log(`swarvex listening on :${konfig.port} (${konfig.basis})`));

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.on(signal, async () => { server.close(); await boss.stop(); await db.end(); process.exit(0); });
}
