// Hash-verkettetes Ereignisprotokoll (docs/traceability.md).
// Jedes Ereignis trägt den Hash seines Vorgängers; der Hash deckt alle Felder ab.
// Geschrieben wird nur innerhalb einer Transaktion unter einer Advisory-Sperre, damit die
// Kette auch bei parallelen Schreibern eindeutig bleibt.
import { createHash } from "node:crypto";
import type { Db, Tx } from "./db.ts";

export const START_HASH = "0".repeat(64);
const KETTEN_SPERRE = 7_314_001;

export type Ereignis = {
  projekt_id: number | null;
  akteur: string;
  art: string;
  bezug: string;
  daten: Record<string, unknown>;
};

/** Kanonisches JSON: Schlüssel sortiert, damit der Hash unabhängig von der Feldreihenfolge ist. */
export function kanonisch(wert: unknown): string {
  if (Array.isArray(wert)) return `[${wert.map(kanonisch).join(",")}]`;
  if (wert && typeof wert === "object") {
    const eintraege = Object.entries(wert as Record<string, unknown>)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
    return `{${eintraege.map(([k, v]) => `${JSON.stringify(k)}:${kanonisch(v)}`).join(",")}}`;
  }
  return JSON.stringify(wert);
}

export function ereignisHash(vorgaenger: string, seq: number, zeit: string, e: Ereignis): string {
  const inhalt = kanonisch({ seq, zeit, projekt_id: e.projekt_id, akteur: e.akteur, art: e.art, bezug: e.bezug, daten: e.daten });
  return createHash("sha256").update(vorgaenger).update("\n").update(inhalt).digest("hex");
}

/** Hängt ein Ereignis an die Kette an. Muss in einer offenen Transaktion laufen. */
export async function protokollieren(tx: Tx, e: Ereignis): Promise<{ seq: number; hash: string }> {
  await tx.query("select pg_advisory_xact_lock($1)", [KETTEN_SPERRE]);
  const letztes = await tx.query("select seq, hash from ereignisse order by seq desc limit 1");
  const vorgaenger: string = letztes.rows[0]?.hash ?? START_HASH;
  const seq = Number((await tx.query("select nextval('ereignisse_seq_seq') as n")).rows[0].n);
  const zeit = new Date().toISOString();
  const hash = ereignisHash(vorgaenger, seq, zeit, e);
  await tx.query(
    "insert into ereignisse(seq, zeit, projekt_id, akteur, art, bezug, daten, vorgaenger_hash, hash) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)",
    [seq, zeit, e.projekt_id, e.akteur, e.art, e.bezug, e.daten, vorgaenger, hash],
  );
  return { seq, hash };
}

export type Pruefergebnis = { ok: boolean; anzahl: number; kopf: string; fehler?: string };

/** Prüft die gesamte Kette neu: Verkettung und Hash jedes Ereignisses. */
export async function ketteVerifizieren(db: Db): Promise<Pruefergebnis> {
  const zeilen = (await db.query("select * from ereignisse order by seq")).rows;
  let vorgaenger = START_HASH;
  for (const z of zeilen) {
    const zeit = new Date(z.zeit).toISOString();
    if (z.vorgaenger_hash !== vorgaenger) {
      return { ok: false, anzahl: zeilen.length, kopf: vorgaenger, fehler: `Ereignis ${z.seq}: Verkettung gebrochen` };
    }
    const erwartet = ereignisHash(vorgaenger, Number(z.seq), zeit, {
      projekt_id: z.projekt_id === null ? null : Number(z.projekt_id), akteur: z.akteur, art: z.art, bezug: z.bezug, daten: z.daten,
    });
    if (erwartet !== z.hash) {
      return { ok: false, anzahl: zeilen.length, kopf: vorgaenger, fehler: `Ereignis ${z.seq}: Inhalt verändert` };
    }
    vorgaenger = z.hash;
  }
  return { ok: true, anzahl: zeilen.length, kopf: vorgaenger };
}
