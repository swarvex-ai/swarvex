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
          `select e.id as einreichung_id, e.patch, e.patch_sha256, a.id as aufgabe_id, a.basis_commit, a.pfade, p.slug, p.repo_git
           from einreichungen e join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id
           where a.zustand = 'eingereicht' and not exists (select 1 from pruefungen x where x.einreichung_id = e.id)
           and e.id = (select max(id) from einreichungen where aufgabe_id = a.id) order by e.id`);
        return json(res, 200, nachAussen(r.rows));
      }
      if (req.method === "GET" && url.pathname === "/pipeline/accepted") {
        const r = await db.query(
          `select e.id as einreichung_id, e.patch, a.id as aufgabe_id, a.titel, a.basis_commit, p.slug, p.repo_git,
                  m.login as mensch, ag.name as agent, b.begruendung, la.name as lead_agent,
                  (select ergebnis from pruefungen where einreichung_id = e.id order by id desc limit 1) as pruefung
           from einreichungen e join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id
           join agenten ag on ag.id = e.agent_id join menschen m on m.id = ag.mensch_id
           join bewertungen b on b.einreichung_id = e.id and b.urteil = 'annehmen' join agenten la on la.id = b.agent_id
           where a.zustand = 'angenommen' order by e.id`);
        return json(res, 200, nachAussen(r.rows));
      }
      const m = /^\/pipeline\/(\d+)\/(start|result|merged)$/.exec(url.pathname);
      if (req.method === "POST" && m) {
        const eid = Number(m[1]);
        if (m[2] === "start") await k.pruefungStarten(db, eid);
        if (m[2] === "result") await k.pruefungMelden(db, eid, daten.result === "pass" ? "ok" : "fehler", String(daten.report ?? ""), sig, daten.source === "sample" ? "stichprobe" : "projekt");
        if (m[2] === "merged") await k.gemergedMelden(db, eid, String(daten.commit));
        return json(res, 200, { ok: true });
      }
      return json(res, 404, { error: "unknown" });
    } catch (e) {
      if (e instanceof k.Abgelehnt) return json(res, 409, { error: e.message });
      throw e;
    }
  };
}
