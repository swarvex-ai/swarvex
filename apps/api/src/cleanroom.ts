// Clean-Room-Verfahren (E-26): Trennung zwischen Analyse und Bau, durchgesetzt pro Mensch und Projekt.
// Die Plattform sieht das Vorbild nie: Analysierende untersuchen ihre eigene, rechtmäßig erworbene
// Kopie lokal (Runner-Sandbox, z. B. mit REA) und reichen nur Spezifikationstext ein.
import { matchesGlob } from "node:path";
import type { Db, Tx } from "./db.ts";

export const RAEUME = ["analyse", "pruefung", "bau"] as const;
export type Raum = (typeof RAEUME)[number];
/** Raumnamen nach außen (MCP, Fehlertexte). */
export const RAUM_EN: Record<Raum, string> = { analyse: "analysis", pruefung: "review", bau: "build" };

/** Spezifikation: einziger Ort, den Analyse und Prüfung beschreiben dürfen; der Bau liest ihn nur. */
export const SPEZIFIKATION = "specification/";

/** Wer in einem Raum war, darf pro Projekt nicht in diese Räume (symmetrisch). */
const UNVERTRAEGLICH: Record<Raum, Raum[]> = {
  analyse: ["bau", "pruefung"],   // wer das Vorbild gesehen hat, baut und prüft nicht
  pruefung: ["analyse"],          // Prüfende bleiben unabhängig von den Analysierenden
  bau: ["analyse"],               // wer baut, sieht das Vorbild nie
};

export const REGELN_TEXT = [
  "Clean-room project: the original is rebuilt without taking over protected code or protected assets.",
  "Analysis: study only your own, lawfully acquired copy (locally, e.g. with REA). Observe from outside first (what a user sees, scenarios, files you created yourself); look inside (decompile) only where a task says so, for interoperability. Tool output from inside reaches your agent's model provider: then use only the models the project allows (original.models local: a model on your own machine; providers: only those in original.providers). A decompiler's or model's reading is a hypothesis until an observable case confirms it. Do not circumvent copy protection, license checks or anti-cheat. Submit only a specification (behaviour, interfaces, data formats, obligations with test cases): no decompiled code, no assets, no verbatim text. Your evidence bundles stay on your machine.",
  "Review: check that the specification only describes function and contains no expression of the original.",
  "Build: implement only from the approved specification and make its test cases pass. Do not open, study or look up the original, nor analysis drafts, evidence or REA output.",
  "Check: comparing a build with the original is analysis work (only people from the analysis side); differences come back as new specification items, never as code advice.",
  "Whoever has worked in one room cannot switch to the other side in the same project; the platform enforces and logs this.",
];

/** Prüft beim Anlegen, ob Raum und Pfade zum Verfahren des Projekts passen. Liefert Fehlertexte. */
export function aufgabePruefen(verfahren: string, raum: string | null | undefined, pfade: string[]): string[] {
  if (verfahren !== "clean_room") return raum ? ["rooms only exist in clean-room projects"] : [];
  if (!raum || !RAEUME.includes(raum as Raum)) return ["clean-room project: the task needs a room (analysis, review or build)"];
  const inSpez = (p: string) => p.startsWith(SPEZIFIKATION);
  if (raum !== "bau" && !pfade.every(inSpez)) return [`room ${RAUM_EN[raum as Raum]} may only write under ${SPEZIFIKATION}`];
  if (raum === "bau" && pfade.some((p) => inSpez(p) || matchesGlob(SPEZIFIKATION + "x", p))) return [`room build must not change ${SPEZIFIKATION}`];
  return [];
}

/** Räume, in denen der Mensch in diesem Projekt schon gearbeitet hat: Reservierungen und Bereichs-Lead-Ämter (auch beendete). */
export async function raeumeVon(tx: Db | Tx, menschId: number, projektId: number): Promise<Raum[]> {
  const r = await tx.query(
    `select a.raum from leases l join aufgaben a on a.id = l.aufgabe_id join agenten g on g.id = l.agent_id
     where a.projekt_id = $1 and g.mensch_id = $2 and a.raum is not null
     union select raum from officers where projekt_id = $1 and mensch_id = $2 and raum is not null`, [projektId, menschId]);
  return r.rows.map((z) => z.raum);
}

/** Grund, warum der Mensch diesen Raum nicht betreten darf, sonst null. */
export function trennwand(bisher: Raum[], ziel: Raum): string | null {
  const konflikt = bisher.find((r) => UNVERTRAEGLICH[ziel].includes(r));
  return konflikt ? `clean room: you have already worked in room "${RAUM_EN[konflikt]}" in this project and therefore cannot work in room "${RAUM_EN[ziel]}"` : null;
}

/** Nachweis für ein Projekt: wie viele Menschen in welchem Raum, und ob die Trennung gehalten hat. */
export async function nachweis(tx: Db | Tx, projektId: number) {
  const r = await tx.query(
    `select mensch_id, array_agg(distinct raum) as raeume from (
       select g.mensch_id, a.raum from leases l join aufgaben a on a.id = l.aufgabe_id join agenten g on g.id = l.agent_id
       where a.projekt_id = $1 and a.raum is not null
       union select mensch_id, raum from officers where projekt_id = $1 and raum is not null) x group by mensch_id`, [projektId]);
  const zaehler: Record<Raum, number> = { analyse: 0, pruefung: 0, bau: 0 };
  let verletzungen = 0;
  for (const z of r.rows) {
    const raeume = z.raeume as Raum[];
    for (const raum of raeume) zaehler[raum]++;
    if (raeume.some((a) => raeume.some((b) => UNVERTRAEGLICH[a].includes(b)))) verletzungen++;
  }
  return { menschen_je_raum: zaehler, trennung_eingehalten: verletzungen === 0, verletzungen };
}
