// Clean-Room-Verfahren (E-26): Trennung zwischen Analyse und Bau, durchgesetzt pro Mensch und Projekt.
// Die Plattform sieht das Vorbild nie: Analysierende untersuchen ihre eigene, rechtmäßig erworbene
// Kopie lokal (Runner-Sandbox, z. B. mit REA) und reichen nur Spezifikationstext ein.
import { matchesGlob } from "node:path";
import type { Db, Tx } from "./db.ts";

export const RAEUME = ["analyse", "pruefung", "bau"] as const;
export type Raum = (typeof RAEUME)[number];

/** Spezifikation: einziger Ort, den Analyse und Prüfung beschreiben dürfen; der Bau liest ihn nur. */
export const SPEZIFIKATION = "spezifikation/";

/** Wer in einem Raum war, darf pro Projekt nicht in diese Räume (symmetrisch). */
const UNVERTRAEGLICH: Record<Raum, Raum[]> = {
  analyse: ["bau", "pruefung"],   // wer das Vorbild gesehen hat, baut und prüft nicht
  pruefung: ["analyse"],          // Prüfende bleiben unabhängig von den Analysierenden
  bau: ["analyse"],               // wer baut, sieht das Vorbild nie
};

export const REGELN_TEXT = [
  "Clean-Room-Projekt: Das Vorbild wird neu gebaut, ohne geschützten Code oder geschützte Assets zu übernehmen.",
  "Analyse: Untersuche nur deine eigene, rechtmäßig erworbene Kopie (lokal, z. B. mit REA). Kein Umgehen von Kopierschutz, Lizenzprüfungen oder Anti-Cheat. Reiche ausschließlich eine Spezifikation ein (Verhalten, Schnittstellen, Datenformate) – keinen dekompilierten Code, keine Assets, keine wörtlichen Texte.",
  "Prüfung: Prüfe die Spezifikation darauf, dass sie nur Funktion beschreibt und keinen Ausdruck des Vorbilds enthält.",
  "Bau: Implementiere nur aus der freigegebenen Spezifikation. Öffne, untersuche oder suche das Vorbild nicht.",
  "Wer in einem Raum gearbeitet hat, kann im selben Projekt nicht auf die andere Seite wechseln; die Plattform setzt das durch und protokolliert es.",
];

/** Prüft beim Anlegen, ob Raum und Pfade zum Verfahren des Projekts passen. Liefert Fehlertexte. */
export function aufgabePruefen(verfahren: string, raum: string | null | undefined, pfade: string[]): string[] {
  if (verfahren !== "clean_room") return raum ? ["Räume gibt es nur in Clean-Room-Projekten"] : [];
  if (!raum || !RAEUME.includes(raum as Raum)) return ["Clean-Room-Projekt: Aufgabe braucht einen Raum (analyse, pruefung oder bau)"];
  const inSpez = (p: string) => p.startsWith(SPEZIFIKATION);
  if (raum !== "bau" && !pfade.every(inSpez)) return [`Raum ${raum} darf nur unter ${SPEZIFIKATION} schreiben`];
  if (raum === "bau" && pfade.some((p) => inSpez(p) || matchesGlob(SPEZIFIKATION + "x", p))) return [`Raum bau darf ${SPEZIFIKATION} nicht ändern`];
  return [];
}

/** Räume, in denen der Mensch in diesem Projekt schon gearbeitet hat (laut Reservierungen). */
export async function raeumeVon(tx: Db | Tx, menschId: number, projektId: number): Promise<Raum[]> {
  const r = await tx.query(
    `select distinct a.raum from leases l join aufgaben a on a.id = l.aufgabe_id join agenten g on g.id = l.agent_id
     where a.projekt_id = $1 and g.mensch_id = $2 and a.raum is not null`, [projektId, menschId]);
  return r.rows.map((z) => z.raum);
}

/** Grund, warum der Mensch diesen Raum nicht betreten darf, sonst null. */
export function trennwand(bisher: Raum[], ziel: Raum): string | null {
  const konflikt = bisher.find((r) => UNVERTRAEGLICH[ziel].includes(r));
  return konflikt ? `Clean Room: Du hast in diesem Projekt schon im Raum „${konflikt}“ gearbeitet und kannst deshalb nicht im Raum „${ziel}“ arbeiten.` : null;
}

/** Nachweis für ein Projekt: wie viele Menschen in welchem Raum, und ob die Trennung gehalten hat. */
export async function nachweis(tx: Db | Tx, projektId: number) {
  const r = await tx.query(
    `select g.mensch_id, array_agg(distinct a.raum) as raeume from leases l join aufgaben a on a.id = l.aufgabe_id
     join agenten g on g.id = l.agent_id where a.projekt_id = $1 and a.raum is not null group by g.mensch_id`, [projektId]);
  const zaehler: Record<Raum, number> = { analyse: 0, pruefung: 0, bau: 0 };
  let verletzungen = 0;
  for (const z of r.rows) {
    const raeume = z.raeume as Raum[];
    for (const raum of raeume) zaehler[raum]++;
    if (raeume.some((a) => raeume.some((b) => UNVERTRAEGLICH[a].includes(b)))) verletzungen++;
  }
  return { menschen_je_raum: zaehler, trennung_eingehalten: verletzungen === 0, verletzungen };
}
