// Fluss ohne Warten: Lead und Bereichs-Leads fragen nicht reihum nach, sondern warten mit wait_for_work, bis es etwas für sie gibt
// (geprüfte Einreichungen zum Bewerten, offene Nachrichten, auf Wunsch zu wenig freie Aufgaben für die aktiven Agenten).
import type { Db } from "./db.ts";
import { Abgelehnt, projektPerSlug, wartendeEinreichungen, type Wer } from "./aufgaben.ts";
import { offeneFuerMich } from "./nachrichten.ts";

export const WECKEN = ["reviews", "messages", "backlog"] as const;
export type Wecken = (typeof WECKEN)[number];

/** Gleichzeitig wartende Aufrufe (wait_for_work, Bereitschaft der Lead-Dienste) je Agent und je Mensch. Jeder fragt bis zu
 *  zehn Minuten lang alle paar Sekunden die Datenbank ab; ohne Grenze legten viele parallele Aufrufe eines einzigen Leads
 *  (auch eines selbst angelegten Entwurfs) den Pool lahm (Sicherheitsaudit, Nachtrag 13.11.2026). */
export const WARTEN_JE_AGENT = 2;
export const WARTEN_JE_MENSCH = 8;
const wartendAgent = new Map<number, number>();
const wartendMensch = new Map<number, number>();

export async function begrenztWarten<T>(wer: Wer, warten: () => Promise<T>): Promise<T> {
  const a = wartendAgent.get(wer.agentId) ?? 0, m = wartendMensch.get(wer.menschId) ?? 0;
  if (a >= WARTEN_JE_AGENT || m >= WARTEN_JE_MENSCH)
    throw new Abgelehnt(`wait_for_work is already waiting ${a >= WARTEN_JE_AGENT ? `${a} times for this agent` : `${m} times for your agents`}; wait until a call returns`);
  wartendAgent.set(wer.agentId, a + 1);
  wartendMensch.set(wer.menschId, m + 1);
  try {
    return await warten();
  } finally {
    const a2 = (wartendAgent.get(wer.agentId) ?? 1) - 1, m2 = (wartendMensch.get(wer.menschId) ?? 1) - 1;
    if (a2 > 0) wartendAgent.set(wer.agentId, a2); else wartendAgent.delete(wer.agentId);
    if (m2 > 0) wartendMensch.set(wer.menschId, m2); else wartendMensch.delete(wer.menschId);
  }
}

/** Freie Aufgaben (offen, alle Vorgänger angenommen) und gerade aktive Reservierungen eines Projekts. */
export async function angebot(db: Db, projektId: number) {
  return (await db.query(
    `select (select count(*)::int from aufgaben a where a.projekt_id = $1 and a.zustand = 'offen'
               and not aufgabe_blockiert(a.abhaengig_von, a.abhaengig_eins_von)) as frei,
            (select count(*)::int from aufgaben a where a.projekt_id = $1 and a.zustand = 'offen'
               and aufgabe_blockiert(a.abhaengig_von, a.abhaengig_eins_von)) as blockiert,
            (select count(*)::int from leases l join aufgaben a on a.id = l.aufgabe_id
               where a.projekt_id = $1 and l.beendet is null and l.laeuft_ab > now()) as aktiv`, [projektId])).rows[0] as { frei: number; blockiert: number; aktiv: number };
}

export async function aufArbeitWarten(db: Db, wer: Wer, slug: string, maxSekunden: number, wecken: Wecken[], schlaf = (ms: number) => new Promise((r) => setTimeout(r, ms)),
  signal?: AbortSignal) {
  const p = await projektPerSlug(db, slug);
  const ende = Date.now() + Math.min(Math.max(maxSekunden, 0), 600) * 1000;
  for (;;) {
    // wartendeEinreichungen prüft die Rolle (Lead, Visionär, Bereichs-Lead) und filtert für Bereichs-Leads auf ihren Bereich
    // eigene Beiträge wecken nicht: sie gehen an die Schwarmprüfung
    const bereit = wecken.includes("reviews") ? (await wartendeEinreichungen(db, wer, slug)).filter((e) => e.zustand === "geprueft_ok" && !e.eigener_beitrag)
      .map((e) => ({ einreichung_id: Number(e.einreichung_id), aufgabe_id: Number(e.aufgabe_id), titel: e.titel })) : [];
    const nachrichten = wecken.includes("messages") ? (await offeneFuerMich(db, wer.menschId, Number(p.id)))?.offen ?? 0 : 0;
    const a = await angebot(db, Number(p.id));
    const nachschub = wecken.includes("backlog") && a.frei < Math.max(1, a.aktiv);
    // signal: Der Aufrufer hat die Verbindung geschlossen, niemand liest die Antwort mehr
    if (bereit.length || nachrichten || nachschub || Date.now() >= ende || signal?.aborted) {
      return { zum_bewerten: bereit, nachrichten_offen: nachrichten, angebot: a, nachschub_noetig: nachschub,
        weiter: !(bereit.length || nachrichten || nachschub) };
    }
    await schlaf(3000);
  }
}
