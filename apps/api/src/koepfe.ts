// Kopf-Hash des Ereignisprotokolls nach außen veröffentlichen (Issue #31, Restrisiko aus P6).
// Täglich signiert die Plattform den Kopf der Kette (JWS, ES256, derselbe Schlüssel wie unter /jwks).
// Ein öffentliches Anker-Repo holt ihn ab und hält ihn in seiner Git-Historie fest (infra/protokoll-anker).
// Prüfen: Die Kette muss in sich stimmen UND an jeder veröffentlichten Stelle denselben Hash haben.
import { CompactSign, importJWK } from "jose";
import type { Db } from "./db.ts";
import { ketteVerifizieren } from "./ereignisse.ts";
import { konfig } from "./konfig.ts";

export type Kopf = { seq: number; hash: string; zeit: string; jws: string };

/** Signiert den aktuellen Kopf und speichert ihn; ohne neue Ereignisse seit dem letzten Mal bleibt alles, wie es ist. */
export async function kopfVeroeffentlichen(db: Db, signatur: Record<string, unknown>): Promise<Kopf | null> {
  const kette = await ketteVerifizieren(db);
  if (!kette.ok) throw new Error(`log chain broken, no head published: ${kette.fehler}`);
  const kopf = (await db.query("select seq, hash from ereignisse order by seq desc limit 1")).rows[0];
  if (!kopf) return null;
  const seq = Number(kopf.seq);
  const letzter = (await db.query("select seq from koepfe order by seq desc limit 1")).rows[0];
  if (letzter && Number(letzter.seq) === seq) return null;
  const zeit = new Date().toISOString();
  const schluessel = await importJWK(signatur as any, "ES256");
  const jws = await new CompactSign(new TextEncoder().encode(JSON.stringify({ iss: konfig.basis, seq, hash: kopf.hash, time: zeit })))
    .setProtectedHeader({ alg: "ES256", kid: String(signatur.kid), typ: "swarvex-head" })
    .sign(schluessel);
  await db.query("insert into koepfe(seq, hash, zeit, jws) values ($1,$2,$3,$4)", [seq, kopf.hash, zeit, jws]);
  return { seq, hash: kopf.hash, zeit, jws };
}

export async function koepfeListe(db: Db, anzahl = 1000): Promise<Kopf[]> {
  return (await db.query("select seq, hash, zeit, jws from koepfe order by seq desc limit $1", [anzahl])).rows
    .map((k) => ({ seq: Number(k.seq), hash: k.hash, zeit: new Date(k.zeit).toISOString(), jws: k.jws }));
}

/** Hashes der Kette an bestimmten Stellen (für den Anker: stimmen frühere Köpfe noch?). */
export async function hashesAn(db: Db, seqs: number[]): Promise<Record<string, string | null>> {
  const r = (await db.query("select seq, hash from ereignisse where seq = any($1)", [seqs])).rows;
  const map = new Map(r.map((z) => [Number(z.seq), z.hash as string]));
  return Object.fromEntries(seqs.map((s) => [String(s), map.get(s) ?? null]));
}

/** Vergleicht die Kette mit veröffentlichten Köpfen (eigene Tabelle oder Datei aus dem Anker-Repo). */
export async function mitKoepfenVergleichen(db: Db, koepfe: { seq: number; hash: string }[]) {
  const ist = await hashesAn(db, koepfe.map((k) => k.seq));
  const abweichungen = koepfe.filter((k) => ist[String(k.seq)] !== k.hash)
    .map((k) => ({ seq: k.seq, veroeffentlicht: k.hash, kette: ist[String(k.seq)] }));
  return { geprueft: koepfe.length, abweichungen };
}
