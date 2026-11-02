// Fluss ohne Warten: Lead und Officers fragen nicht reihum nach, sondern warten mit wait_for_work, bis es etwas für sie gibt
// (geprüfte Einreichungen zum Bewerten, offene Nachrichten, auf Wunsch zu wenig freie Aufgaben für die aktiven Agenten).
import type { Db } from "./db.ts";
import { projektPerSlug, wartendeEinreichungen, type Wer } from "./aufgaben.ts";
import { offeneFuerMich } from "./nachrichten.ts";

export const WECKEN = ["reviews", "messages", "backlog"] as const;
export type Wecken = (typeof WECKEN)[number];

/** Freie Aufgaben (offen, alle Vorgänger angenommen) und gerade aktive Reservierungen eines Projekts. */
export async function angebot(db: Db, projektId: number) {
  return (await db.query(
    `select (select count(*)::int from aufgaben a where a.projekt_id = $1 and a.zustand = 'offen'
               and not exists (select 1 from aufgaben v where v.id = any(a.abhaengig_von) and v.zustand not in ('angenommen', 'gemerged'))) as frei,
            (select count(*)::int from aufgaben a where a.projekt_id = $1 and a.zustand = 'offen'
               and exists (select 1 from aufgaben v where v.id = any(a.abhaengig_von) and v.zustand not in ('angenommen', 'gemerged'))) as blockiert,
            (select count(*)::int from leases l join aufgaben a on a.id = l.aufgabe_id
               where a.projekt_id = $1 and l.beendet is null and l.laeuft_ab > now()) as aktiv`, [projektId])).rows[0] as { frei: number; blockiert: number; aktiv: number };
}

export async function aufArbeitWarten(db: Db, wer: Wer, slug: string, maxSekunden: number, wecken: Wecken[], schlaf = (ms: number) => new Promise((r) => setTimeout(r, ms))) {
  const p = await projektPerSlug(db, slug);
  const ende = Date.now() + Math.min(Math.max(maxSekunden, 0), 600) * 1000;
  for (;;) {
    // wartendeEinreichungen prüft die Rolle (Lead, Visionär, Officer) und filtert für Officers auf ihren Bereich
    const bereit = wecken.includes("reviews") ? (await wartendeEinreichungen(db, wer, slug)).filter((e) => e.zustand === "geprueft_ok")
      .map((e) => ({ einreichung_id: Number(e.einreichung_id), aufgabe_id: Number(e.aufgabe_id), titel: e.titel })) : [];
    const nachrichten = wecken.includes("messages") ? (await offeneFuerMich(db, wer.menschId, Number(p.id)))?.offen ?? 0 : 0;
    const a = await angebot(db, Number(p.id));
    const nachschub = wecken.includes("backlog") && a.frei < Math.max(1, a.aktiv);
    if (bereit.length || nachrichten || nachschub || Date.now() >= ende) {
      return { zum_bewerten: bereit, nachrichten_offen: nachrichten, angebot: a, nachschub_noetig: nachschub,
        weiter: !(bereit.length || nachrichten || nachschub) };
    }
    await schlaf(3000);
  }
}
