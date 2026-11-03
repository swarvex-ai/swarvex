// Schnittstelle der Prüfstraße (läuft beim Visionär; im Nachweis als eigener Dienst).
// Jede Anfrage ist mit HMAC-SHA256 signiert (Zeitstempel + Methode + Pfad + Körper) und nur
// fünf Minuten gültig. Unsignierte oder veränderte Meldungen werden abgewiesen (Angriff S8).
import { createHmac, timingSafeEqual } from "node:crypto";
import type { IncomingMessage, ServerResponse } from "node:http";
import type { Db } from "./db.ts";
import { transaktion } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import * as k from "./aufgaben.ts";
import { nachAussen } from "./aussen.ts";
import { repoToken } from "./github.ts";
import { konfig, vorschauUrl } from "./konfig.ts";

/** On-Demand-TLS: darf Caddy für diesen Namen ein Zertifikat holen? Nur <slug>.<Vorschau-Domain> mit gebauter Vorschau. */
export async function vorschauErlaubt(db: Db, name: string) {
  const d = konfig.vorschauDomain, n = name.toLowerCase();
  const slug = d && n.endsWith(`.${d}`) ? n.slice(0, -(d.length + 1)) : "";
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

export function signatur(geheimnis: string, zeit: string, methode: string, pfad: string, koerper: string) {
  return createHmac("sha256", geheimnis).update(`${zeit}\n${methode}\n${pfad}\n${koerper}`).digest("hex");
}

function json(res: ServerResponse, status: number, wert: unknown) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(wert));
}

async function rohKoerper(req: IncomingMessage): Promise<string> {
  const teile: Buffer[] = [];
  for await (const t of req) teile.push(t as Buffer);
  return Buffer.concat(teile).toString("utf8");
}

export function pruefstrasseBehandeln(db: Db, geheimnis: string) {
  return async (req: IncomingMessage, res: ServerResponse, url: URL) => {
    const koerper = await rohKoerper(req);
    const zeit = String(req.headers["x-time"] ?? "");
    const sig = String(req.headers["x-signature"] ?? "");
    const erwartet = signatur(geheimnis, zeit, req.method ?? "", url.pathname, koerper);
    const frisch = Math.abs(Date.now() - Number(zeit)) < 5 * 60_000;
    if (!geheimnis || !frisch || sig.length !== erwartet.length || !timingSafeEqual(Buffer.from(sig), Buffer.from(erwartet))) {
      await transaktion(db, (tx) => protokollieren(tx, { projekt_id: null, akteur: "unbekannt", art: "pruefstrasse_abgewiesen", bezug: url.pathname, daten: { grund: frisch ? "signatur" : "zeitstempel" } }));
      return json(res, 401, { error: "invalid signature" });
    }
    const daten = koerper ? JSON.parse(koerper) : {};
    try {
      if (req.method === "GET" && url.pathname === "/pipeline/pending") {
        const r = await db.query(
          `select e.id as einreichung_id, e.patch, e.patch_sha256, a.id as aufgabe_id, a.basis_commit, a.pfade, p.slug, p.repo_git, p.github_installation
           from einreichungen e join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id
           where a.zustand = 'eingereicht' and p.engine !~* '(unity|unreal)' and not exists (select 1 from pruefungen x where x.einreichung_id = e.id)
           and e.id = (select max(id) from einreichungen where aufgabe_id = a.id) order by e.id`);
        return json(res, 200, nachAussen(await mitToken(r.rows, false)));
      }
      if (req.method === "GET" && url.pathname === "/pipeline/accepted") {
        const r = await db.query(
          `select e.id as einreichung_id, e.patch, a.id as aufgabe_id, a.titel, a.basis_commit, p.slug, p.repo_git, p.github_installation,
                  m.login as mensch, ag.name as agent, b.begruendung, la.name as lead_agent,
                  (select ergebnis from pruefungen where einreichung_id = e.id order by id desc limit 1) as pruefung
           from einreichungen e join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id
           join agenten ag on ag.id = e.agent_id join menschen m on m.id = ag.mensch_id
           join bewertungen b on b.einreichung_id = e.id and b.urteil = 'annehmen' join agenten la on la.id = b.agent_id
           where a.zustand = 'angenommen' order by e.id`);
        return json(res, 200, nachAussen(await mitToken(r.rows, true)));
      }
      // Vorschauen (pipeline/vorschau.go): nur wenn eine eigene Adresse dafür eingerichtet ist; Unity und Unreal baut die Plattform nicht
      if (req.method === "GET" && url.pathname === "/pipeline/previews") {
        if (!vorschauUrl("x")) return json(res, 200, []);
        const r = await db.query(
          `select slug, repo_git, github_installation, vorschau->>'commit' as letzter_commit from projekte
           where repo_git is not null and not gesperrt and not entwurf and engine !~* '(unity|unreal)' order by slug`);
        return json(res, 200, nachAussen(await mitToken(r.rows, false)));
      }
      const v = /^\/pipeline\/previews\/([a-z0-9-]+)$/.exec(url.pathname);
      if (req.method === "POST" && v) {
        const status = ["ok", "failed", "none"].includes(daten.status) ? daten.status : "failed";
        const commit = String(daten.commit ?? "").slice(0, 64);
        await transaktion(db, async (tx) => {
          const p = (await tx.query("select id, vorschau from projekte where slug = $1 for update", [v[1]])).rows[0];
          if (!p) throw new k.Abgelehnt("unknown project");
          const alt = p.vorschau ?? {};
          const neu = { commit, status, zeit: new Date().toISOString(), bytes: Number(daten.bytes ?? 0), bericht: String(daten.report ?? "").slice(-4000),
            ok_commit: status === "ok" ? commit : alt.ok_commit ?? null, ok_zeit: status === "ok" ? new Date().toISOString() : alt.ok_zeit ?? null };
          await tx.query("update projekte set vorschau = $2 where id = $1", [p.id, neu]);
          if (status !== "none") await protokollieren(tx, { projekt_id: Number(p.id), akteur: "system", art: "vorschau_gebaut", bezug: `projekt:${p.id}`, daten: { commit, status } });
        });
        return json(res, 200, { ok: true });
      }
      const m = /^\/pipeline\/(\d+)\/(start|result|merged|conflict)$/.exec(url.pathname);
      if (req.method === "POST" && m) {
        const eid = Number(m[1]);
        if (m[2] === "start") await k.pruefungStarten(db, eid);
        if (m[2] === "result") await k.pruefungMelden(db, eid, daten.result === "pass" ? "ok" : "fehler", String(daten.report ?? ""), sig, daten.source === "sample" ? "stichprobe" : "projekt");
        if (m[2] === "merged") await k.gemergedMelden(db, eid, String(daten.commit));
        if (m[2] === "conflict") await k.konfliktMelden(db, eid, String(daten.head), String(daten.report ?? ""));
        return json(res, 200, { ok: true });
      }
      return json(res, 404, { error: "unknown" });
    } catch (e) {
      if (e instanceof k.Abgelehnt) return json(res, 409, { error: e.message });
      throw e;
    }
  };
}
