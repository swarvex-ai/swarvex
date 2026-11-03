// Wochenbericht für Visionäre: was in den letzten sieben Tagen in einem Projekt passiert ist und wo es hakt, aus Zahlen,
// die ohnehin öffentlich sind. Im Konto für die eigenen Projekte als Visionär und als MCP-Werkzeug weekly_report.
// Die Hinweise nennen nur Engpässe, die der Visionär selbst lösen kann (bewerten, Officer ernennen, Lead nachfragen).
import type { Db } from "./db.ts";
import { STUFEN_GEWICHT } from "./aufgaben.ts";
import { angebot } from "./fluss.ts";

const TAG = 86_400_000;

async function woche(db: Db, projektId: number, von: Date, bis: Date) {
  const z = (await db.query(
    `select (select count(*)::int from aufgaben where projekt_id = $1 and erstellt >= $2 and erstellt < $3) as angelegt,
            (select coalesce(json_agg(a.stufe), '[]') from bewertungen b join einreichungen e on e.id = b.einreichung_id join aufgaben a on a.id = e.aufgabe_id
               where a.projekt_id = $1 and b.urteil = 'annehmen' and b.erstellt >= $2 and b.erstellt < $3) as angenommen_stufen,
            (select count(*)::int from bewertungen b join einreichungen e on e.id = b.einreichung_id join aufgaben a on a.id = e.aufgabe_id
               where a.projekt_id = $1 and b.urteil = 'nacharbeit' and b.erstellt >= $2 and b.erstellt < $3) as nacharbeit,
            (select count(*)::int from bewertungen b join einreichungen e on e.id = b.einreichung_id join aufgaben a on a.id = e.aufgabe_id
               where a.projekt_id = $1 and b.urteil = 'ablehnen' and b.erstellt >= $2 and b.erstellt < $3) as abgelehnt,
            (select count(*)::int from ereignisse where projekt_id = $1 and art = 'gemerged' and zeit >= $2 and zeit < $3) as gemerged,
            (select count(distinct ag.mensch_id)::int from einreichungen e join agenten ag on ag.id = e.agent_id join aufgaben a on a.id = e.aufgabe_id
               where a.projekt_id = $1 and e.eingereicht >= $2 and e.eingereicht < $3) as menschen,
            (select count(*)::int from (select ag.mensch_id, min(e.eingereicht) as erste from einreichungen e join agenten ag on ag.id = e.agent_id
               join aufgaben a on a.id = e.aufgabe_id where a.projekt_id = $1 group by ag.mensch_id) x where erste >= $2 and erste < $3) as neue_menschen,
            (select count(*)::int from fremdpruefungen f join einreichungen e on e.id = f.einreichung_id join aufgaben a on a.id = e.aufgabe_id
               where a.projekt_id = $1 and f.erstellt >= $2 and f.erstellt < $3) as fremdpruefungen,
            (select coalesce(sum((e.verbrauch->>'tokens')::numeric), 0)::bigint from einreichungen e join aufgaben a on a.id = e.aufgabe_id
               where a.projekt_id = $1 and e.eingereicht >= $2 and e.eingereicht < $3) as tokens`,
    [projektId, von, bis])).rows[0];
  const stufen = z.angenommen_stufen as string[];
  const { angenommen_stufen, ...rest } = z;
  return { ...rest, angenommen: stufen.length, einheiten: stufen.reduce((s, x) => s + (STUFEN_GEWICHT[x as keyof typeof STUFEN_GEWICHT] ?? 1), 0), tokens: Number(z.tokens) };
}

export async function wochenbericht(db: Db, projektId: number, bis = new Date()) {
  const von = new Date(bis.getTime() - 7 * TAG);
  const diese = await woche(db, projektId, von, bis);
  const vorige = await woche(db, projektId, new Date(von.getTime() - 7 * TAG), von);
  const a = await angebot(db, projektId);
  const warten = (await db.query(
    `select count(*)::int as n, extract(epoch from now() - min(pr.erstellt))::int / 3600 as aelteste_h
     from aufgaben a join lateral (select erstellt from pruefungen p join einreichungen e on e.id = p.einreichung_id where e.aufgabe_id = a.id order by p.id desc limit 1) pr on true
     where a.projekt_id = $1 and a.zustand = 'geprueft_ok'`, [projektId])).rows[0];
  const freigaben = Number((await db.query("select count(*) from nachrichten where projekt_id = $1 and art = 'freigabe' and zustand = 'offen'", [projektId])).rows[0].count);
  const rueckmeldungen = Number((await db.query("select count(*) from vorschau_feedback where projekt_id = $1 and zustand = 'neu'", [projektId])).rows[0].count);
  // Hinweise mit Kennung und Zahlen (die Website übersetzt sie) und einem Satz für Agenten
  const hinweise: { code: string; text: string; n?: number; stunden?: number; frei?: number; aktiv?: number }[] = [];
  if (freigaben) hinweise.push({ code: "approvals", n: freigaben, text: `${freigaben} approval request${freigaben > 1 ? "s wait" : " waits"} for your decision (your account, approvals).` });
  if (warten.n && warten.aelteste_h >= 24) hinweise.push({ code: "review_wait", n: warten.n, stunden: warten.aelteste_h,
    text: `${warten.n} checked submission${warten.n > 1 ? "s wait" : " waits"} for review, the oldest for ${warten.aelteste_h} h: ask the lead, or appoint an officer for the busiest area.` });
  if (rueckmeldungen) hinweise.push({ code: "feedback", n: rueckmeldungen,
    text: `${rueckmeldungen} feedback entr${rueckmeldungen > 1 ? "ies" : "y"} on the preview ${rueckmeldungen > 1 ? "wait" : "waits"} for the lead (preview_feedback): good material for the next tasks.` });
  if (a.aktiv > 0 && a.frei < a.aktiv) hinweise.push({ code: "backlog_low", frei: a.frei, aktiv: a.aktiv,
    text: `Only ${a.frei} task${a.frei === 1 ? " is" : "s are"} free for ${a.aktiv} working agent${a.aktiv === 1 ? "" : "s"}: ask the lead to cut the next tasks.` });
  if (a.frei === 0 && a.blockiert > 0) hinweise.push({ code: "critical_path", n: a.blockiert, text: `All ${a.blockiert} open tasks wait for other tasks: the critical path decides the pace now.` });
  if (!diese.angenommen && !diese.angelegt) hinweise.push({ code: "quiet", text: "Nothing happened this week. If the project is paused, say so in its status; otherwise ask the lead what blocks it." });
  if (diese.nacharbeit + diese.abgelehnt > diese.angenommen && diese.angenommen + diese.nacharbeit + diese.abgelehnt >= 4)
    hinweise.push({ code: "rework", text: "More submissions went back for rework than were accepted: the tasks may be too vague. Ask the lead to sharpen acceptance criteria." });
  return {
    von: von.toISOString(), bis: bis.toISOString(), woche: diese,
    vorwoche: { angenommen: vorige.angenommen, einheiten: vorige.einheiten, menschen: vorige.menschen },
    jetzt: { frei: a.frei, blockiert: a.blockiert, aktiv: a.aktiv, warten_auf_bewertung: warten.n, aelteste_bewertung_h: warten.n ? warten.aelteste_h : null, offene_freigaben: freigaben },
    hinweise,
  };
}
