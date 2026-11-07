// Schnittstelle der Prüfstraße (läuft beim Visionär; im Nachweis als eigener Dienst).
// Jede Anfrage ist mit HMAC-SHA256 signiert (Zeitstempel + Einmalnummer + Methode + Pfad + Körper) und nur
// fünf Minuten gültig; jede Einmalnummer gilt nur einmal. Unsignierte, veränderte oder wiederholte Meldungen
// werden abgewiesen (Angriff S8).
import { bilderSpeichern } from "./bilder.ts";
import { createHmac, timingSafeEqual } from "node:crypto";
import type { IncomingMessage, ServerResponse } from "node:http";
import type { Db } from "./db.ts";
import { transaktion } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import * as k from "./aufgaben.ts";
import { WARTET_AUF_GEGENPRUEFUNG } from "./gegenpruefung.ts";
import { nachAussen } from "./aussen.ts";
import { repoToken } from "./github.ts";
import { einreichungsVorschauUrl, konfig, vorschauUrl } from "./konfig.ts";

/** On-Demand-TLS: darf Caddy für diesen Namen ein Zertifikat holen? Nur <slug>.<Vorschau-Domain> mit gebauter Vorschau. */
export async function vorschauErlaubt(db: Db, name: string) {
  const d = konfig.vorschauDomain, n = name.toLowerCase();
  const slug = d && n.endsWith(`.${d}`) ? n.slice(0, -(d.length + 1)) : "";
  if (slug === "review") return true; // Vorschauen der Einreichungen, ein Zertifikat für alle
  if (!/^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$/.test(slug)) return false;
  return Boolean((await db.query("select 1 from projekte where slug = $1 and not gesperrt and vorschau->>'ok_commit' is not null", [slug])).rowCount);
}

/** Projekte über die GitHub App: die Prüfstraße bekommt je Repo einen Token (lesend; schreibend nur für den Merge-Bot). */
async function mitToken(zeilen: Record<string, unknown>[], schreiben: boolean) {
  return Promise.all(zeilen.map(async ({ github_installation, ...z }) => {
    if (!github_installation) return z;
    try { return { ...z, git_token: await repoToken(Number(github_installation), String(z.repo_git), schreiben) }; }
    catch (e) { console.error(`github token for ${z.slug}: ${(e as Error).message}`); return z; }
  }));
}

export function signatur(geheimnis: string, zeit: string, nonce: string, methode: string, pfad: string, koerper: string) {
  return createHmac("sha256", geheimnis).update(`${zeit}\n${nonce}\n${methode}\n${pfad}\n${koerper}`).digest("hex");
}

/** Gesehene Einmalnummern der letzten fünf Minuten (danach ist der Zeitstempel ohnehin abgelaufen). */
const GUELTIG_MS = 5 * 60_000;
const gesehen = new Map<string, number>();
function schonGesehen(nonce: string, jetzt = Date.now()) {
  for (const [n, t] of gesehen) if (jetzt - t > GUELTIG_MS) gesehen.delete(n);
  if (gesehen.has(nonce)) return true;
  gesehen.set(nonce, jetzt);
  return false;
}

function json(res: ServerResponse, status: number, wert: unknown) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(wert));
}

/** Höchstens so viel liest die Schnittstelle, bevor die Signatur geprüft ist (Berichte samt Build-Ausgabe passen bequem). */
const MAX_KOERPER = 16 << 20;

async function rohKoerper(req: IncomingMessage): Promise<string | null> {
  const teile: Buffer[] = [];
  let groesse = 0;
  for await (const t of req) {
    groesse += (t as Buffer).length;
    if (groesse > MAX_KOERPER) return null;
    teile.push(t as Buffer);
  }
  return Buffer.concat(teile).toString("utf8");
}

// Abgewiesene Anfragen kommen ins öffentliche Protokoll, aber höchstens eine je Minute (sonst ließe es sich fluten).
let zuletztProtokolliert = 0;

export function pruefstrasseBehandeln(db: Db, geheimnis: string) {
  return async (req: IncomingMessage, res: ServerResponse, url: URL) => {
    const koerper = await rohKoerper(req);
    if (koerper === null) return json(res, 413, { error: "request too large" });
    const zeit = String(req.headers["x-time"] ?? "");
    const sig = String(req.headers["x-signature"] ?? "");
    const nonce = String(req.headers["x-nonce"] ?? "");
    const erwartet = signatur(geheimnis, zeit, nonce, req.method ?? "", url.pathname, koerper);
    const frisch = Math.abs(Date.now() - Number(zeit)) < GUELTIG_MS;
    const echt = !!geheimnis && frisch && /^[A-Za-z0-9_-]{16,64}$/.test(nonce) && sig.length === erwartet.length && timingSafeEqual(Buffer.from(sig), Buffer.from(erwartet));
    if (!echt || schonGesehen(nonce)) {
      if (Date.now() - zuletztProtokolliert > 60_000) {
        zuletztProtokolliert = Date.now();
        await transaktion(db, (tx) => protokollieren(tx, { projekt_id: null, akteur: "unbekannt", art: "pruefstrasse_abgewiesen", bezug: url.pathname, daten: { grund: !frisch ? "zeitstempel" : echt ? "wiederholt" : "signatur" } }));
      } else console.warn(`pipeline: rejected ${req.method} ${url.pathname} (${!frisch ? "timestamp" : echt ? "replayed" : "signature"})`);
      return json(res, 401, { error: "invalid signature" });
    }
    const daten = koerper ? JSON.parse(koerper) : {};
    try {
      if (req.method === "GET" && url.pathname === "/pipeline/pending") {
        const r = await db.query(
          `select e.id as einreichung_id, e.patch, e.patch_sha256, a.id as aufgabe_id, a.basis_commit, a.pfade, p.slug, p.repo_git, p.github_installation
           from einreichungen e join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id join agenten ag on ag.id = e.agent_id
           where a.zustand = 'eingereicht' and p.engine !~* '(unity|unreal)'
           and not exists (select 1 from pruefungen x where x.einreichung_id = e.id and x.quelle = 'projekt')
           and not ${WARTET_AUF_GEGENPRUEFUNG}
           and e.id = (select max(id) from einreichungen where aufgabe_id = a.id) order by e.id`);
        return json(res, 200, nachAussen(await mitToken(r.rows, false)));
      }
      if (req.method === "GET" && url.pathname === "/pipeline/accepted") {
        const r = await db.query(
          `select e.id as einreichung_id, e.patch, a.id as aufgabe_id, a.titel, a.basis_commit, p.slug, p.repo_git, p.github_installation,
                  m.login as mensch, agent_oeffentlich(ag.name) as agent, b.begruendung, agent_oeffentlich(la.name) as lead_agent,
                  (select ergebnis from pruefungen where einreichung_id = e.id order by id desc limit 1) as pruefung
           from einreichungen e join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id
           join agenten ag on ag.id = e.agent_id join menschen m on m.id = ag.mensch_id
           join bewertungen b on b.einreichung_id = e.id and b.urteil = 'annehmen' join agenten la on la.id = b.agent_id
           where a.zustand = 'angenommen' order by e.id`);
        return json(res, 200, nachAussen(await mitToken(r.rows, true)));
      }
      // Stand von main je Projekt (pipeline/vorschau.go, alle fünf Minuten): beim Reservieren wird er zur Basis der Aufgabe
      if (req.method === "GET" && url.pathname === "/pipeline/heads") {
        const r = await db.query(
          `select slug, repo_git, github_installation, kopf_commit as letzter_commit from projekte
           where repo_git is not null and not gesperrt and not entwurf order by slug`);
        return json(res, 200, nachAussen(await mitToken(r.rows, false)));
      }
      const kopf = /^\/pipeline\/heads\/([a-z0-9-]+)$/.exec(url.pathname);
      if (req.method === "POST" && kopf) {
        const commit = String(daten.commit ?? "");
        if (!/^[0-9a-f]{40}$/.test(commit)) return json(res, 400, { error: "commit: a full commit hash" });
        await db.query("update projekte set kopf_commit = $2 where slug = $1", [kopf[1], commit]);
        return json(res, 200, { ok: true });
      }
      // Vorschauen (pipeline/vorschau.go): nur wenn eine eigene Adresse dafür eingerichtet ist; Unity und Unreal baut die Plattform nicht
      if (req.method === "GET" && url.pathname === "/pipeline/previews") {
        if (!vorschauUrl("x")) return json(res, 200, []);
        const r = await db.query(
          `select slug, repo_git, github_installation, vorschau->>'commit' as letzter_commit from projekte
           where repo_git is not null and not gesperrt and not entwurf and engine !~* '(unity|unreal)' order by slug`);
        return json(res, 200, nachAussen(await mitToken(r.rows, false)));
      }
      // Vorschau je Einreichung: geprüft, wartet auf Bewertung, Projekt hat schon eine Vorschau (also eine Vorschau-Einrichtung)
      if (req.method === "GET" && url.pathname === "/pipeline/submission-previews") {
        if (!einreichungsVorschauUrl(1)) return json(res, 200, { open: [], keep: [] });
        const offen = await db.query(
          `select e.id as einreichung_id, e.patch, e.patch_sha256, a.id as aufgabe_id, a.basis_commit, p.slug, p.repo_git, p.github_installation
           from einreichungen e join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id
           where a.zustand = 'geprueft_ok' and e.vorschau is null and e.id = (select max(id) from einreichungen where aufgabe_id = a.id)
             and p.vorschau->>'ok_commit' is not null and coalesce((p.vorschau->>'hat_vorschau')::boolean, true) and not p.gesperrt
           order by e.id limit 5`);
        const behalten = (await db.query(
          `select e.id::int from einreichungen e join aufgaben a on a.id = e.aufgabe_id
           where a.zustand = 'geprueft_ok' and e.vorschau->>'status' = 'ok'`)).rows.map((r) => r.id);
        return json(res, 200, { open: nachAussen(await mitToken(offen.rows, false)), keep: behalten });
      }
      const ev = /^\/pipeline\/submission-previews\/(\d+)$/.exec(url.pathname);
      if (req.method === "POST" && ev) {
        const status = ["ok", "failed", "none"].includes(daten.status) ? daten.status : "failed";
        await db.query("update einreichungen set vorschau = $2 where id = $1",
          [Number(ev[1]), { status, zeit: new Date().toISOString(), bytes: Number(daten.bytes ?? 0), bericht: String(daten.report ?? "").slice(-2000) }]);
        return json(res, 200, { ok: true });
      }
      const v = /^\/pipeline\/previews\/([a-z0-9-]+)$/.exec(url.pathname);
      if (req.method === "POST" && v) {
        const status = ["ok", "failed", "none"].includes(daten.status) ? daten.status : "failed";
        const commit = String(daten.commit ?? "").slice(0, 64);
        await transaktion(db, async (tx) => {
          const p = (await tx.query("select id, vorschau from projekte where slug = $1 for update", [v[1]])).rows[0];
          if (!p) throw new k.Abgelehnt("unknown project");
          const alt = p.vorschau ?? {};
          const ok = status === "ok";
          // was der gelungene Stand enthält: Vorschau (Wurzel) und/oder Doku (/docs/); ältere Meldungen ohne Angabe sind Vorschauen
          const neu = { commit, status, zeit: new Date().toISOString(), bytes: Number(daten.bytes ?? 0), bericht: String(daten.report ?? "").slice(-4000),
            ok_commit: ok ? commit : alt.ok_commit ?? null, ok_zeit: ok ? new Date().toISOString() : alt.ok_zeit ?? null,
            hat_vorschau: ok ? daten.preview !== false : alt.hat_vorschau ?? true, hat_doku: ok ? daten.docs === true : alt.hat_doku ?? false };
          await tx.query("update projekte set vorschau = $2 where id = $1", [p.id, neu]);
          if (status !== "none") await protokollieren(tx, { projekt_id: Number(p.id), akteur: "system", art: "vorschau_gebaut", bezug: `projekt:${p.id}`, daten: { commit, status } });
        });
        return json(res, 200, { ok: true });
      }
      // Szenenbilder einer Einreichung für den Bildvergleich im Review (bilder.ts)
      const bild = /^\/pipeline\/(\d+)\/images$/.exec(url.pathname);
      if (req.method === "POST" && bild) {
        await bilderSpeichern(db, Number(bild[1]), String(daten.name ?? ""), daten);
        return json(res, 200, { ok: true });
      }
      const m = /^\/pipeline\/(\d+)\/(start|result|merged|conflict|train-red)$/.exec(url.pathname);
      if (req.method === "POST" && m) {
        const eid = Number(m[1]);
        if (m[2] === "start") await k.pruefungStarten(db, eid);
        if (m[2] === "result") await k.pruefungMelden(db, eid, daten.result === "pass" ? "ok" : "fehler", String(daten.report ?? ""), sig, daten.source === "sample" ? "stichprobe" : "projekt", daten.base_red === true);
        if (m[2] === "merged") await k.gemergedMelden(db, eid, String(daten.commit));
        if (m[2] === "conflict") await k.konfliktMelden(db, eid, String(daten.head), String(daten.report ?? ""));
        if (m[2] === "train-red") await k.zugRotMelden(db, eid, String(daten.head), String(daten.report ?? ""), sig);
        return json(res, 200, { ok: true });
      }
      return json(res, 404, { error: "unknown" });
    } catch (e) {
      if (e instanceof k.Abgelehnt) return json(res, 409, { error: e.message });
      throw e;
    }
  };
}
