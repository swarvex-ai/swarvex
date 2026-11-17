// Spendenwerk: erzeugt die Aufträge an Zusagen mit API-Schlüssel (migrations/067). Läuft als eigener Container
// (infra/stack/compose.yaml, Dienst spendenwerk) und ist der einzige Ort mit dem privaten Schlüssel, der die
// Spender-Schlüssel entschlüsselt. Ein Auftrag nach dem anderen, damit Verbrauch und Grenze eindeutig bleiben.
//
// Harte Grenze je Zusage:
//   1. Beim Anlegen (spenden.ts): verbraucht + offene Aufträge (mit Preis) + neuer Auftrag ≤ zugesagt.
//   2. Vor dem Erzeugen, unter Sperre: dasselbe noch einmal, und das Datum der Zusage.
//   3. Danach: Hat der Dienst mehr abgebucht als der Preis vorab, endet die Zusage sofort (Schlüssel gelöscht).
//   4. Ist die Zusage aufgebraucht oder abgelaufen, endet sie und der Schlüssel wird gelöscht.
import { createHash, randomBytes } from "node:crypto";
import { mkdir, readFile, rename, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { transaktion, type Db, type Tx } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { inhaltPasst } from "./medien.ts";
import { DienstFehler, erzeugen, ERZEUGUNG, type SchluesselDienst } from "./spendendienste.ts";
import { entschluesseln } from "./spendenschluessel.ts";
import { speicher } from "./spenden.ts";

const SPERRE = "3 hours";        // so lange gilt ein begonnener Auftrag als in Arbeit
const PAUSE = "24 hours";        // nach einem Fehlschlag: der Lead soll den Auftrag ändern oder zurückziehen
const AKTEUR = "system";

/** Zusage beenden und Schlüssel löschen; offene Aufträge zurückziehen. */
async function zusageSchliessen(tx: Tx, zusageId: number, grund: string) {
  const z = (await tx.query("update zusagen set zustand = 'beendet', schluessel = null, endgrund = $2 where id = $1 and zustand = 'offen' returning projekt_id, dienst",
    [zusageId, grund.slice(0, 300)])).rows[0];
  if (!z) return;
  await tx.query("update spendenauftraege set zustand = 'zurueckgezogen', entschieden = now() where zusage_id = $1 and zustand in ('offen', 'nacharbeit')", [zusageId]);
  await protokollieren(tx, { projekt_id: Number(z.projekt_id), akteur: AKTEUR, art: "zusage_beendet", bezug: `zusage:${zusageId}`, daten: { dienst: z.dienst, grund: grund.slice(0, 300) } });
}

/** Abgelaufene Zusagen mit Schlüssel beenden (stündlich). */
export async function abgelaufeneBeenden(db: Db): Promise<number> {
  const ids = (await db.query("select id from zusagen where weg = 'schluessel' and zustand = 'offen' and verfaellt < current_date")).rows.map((r) => Number(r.id));
  for (const id of ids) await transaktion(db, (tx) => zusageSchliessen(tx, id, "the pledge expired"));
  return ids.length;
}

type Auftrag = { id: number; projekt_id: number; zusage_id: number; erzeugung: unknown; credits: number; max_mb: number; slug: string; dienst: SchluesselDienst };

/** Nimmt den nächsten Auftrag, prüft die Grenze und setzt „begonnen“. null: nichts zu tun; "grenze": über der Zusage. */
async function nehmen(db: Db): Promise<Auftrag | "grenze" | null> {
  return transaktion(db, async (tx) => {
    const a = (await tx.query(
      `select a.id, a.projekt_id, a.zusage_id, a.erzeugung, a.credits, a.max_mb, p.slug, z.dienst
       from spendenauftraege a join zusagen z on z.id = a.zusage_id join projekte p on p.id = a.projekt_id
       where z.weg = 'schluessel' and z.zustand = 'offen' and z.verfaellt >= current_date
         and a.zustand = 'offen' and a.erzeugung is not null
         and (a.begonnen is null or a.begonnen < now() - interval '${SPERRE}')
         and (a.abgebrochen is null or a.abgebrochen < now() - interval '${PAUSE}')
       order by a.erstellt limit 1 for update of a skip locked`)).rows[0];
    if (!a) return null;
    const z = (await tx.query("select credits, verbraucht from zusagen where id = $1 for update", [a.zusage_id])).rows[0];
    const laufend = Number((await tx.query(
      `select coalesce(sum(credits), 0) as s from spendenauftraege where zusage_id = $1 and id <> $2 and zustand = 'offen' and begonnen > now() - interval '${SPERRE}'`,
      [a.zusage_id, a.id])).rows[0].s);
    if (z.verbraucht + laufend + a.credits > z.credits) {
      const grund = `the pledge has ${Math.max(0, z.credits - z.verbraucht - laufend)} credits left; this brief costs ${a.credits}`;
      await tx.query("update spendenauftraege set abgebrochen = now(), abbruchgrund = $2 where id = $1", [a.id, grund]);
      await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: AKTEUR, art: "spendenauftrag_abgebrochen", bezug: `spendenauftrag:${a.id}`, daten: { grund } });
      return "grenze";
    }
    await tx.query("update spendenauftraege set begonnen = now() where id = $1", [a.id]);
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: AKTEUR, art: "spendenauftrag_begonnen", bezug: `spendenauftrag:${a.id}`, daten: { weg: "schluessel" } });
    return { ...a, id: Number(a.id), projekt_id: Number(a.projekt_id), zusage_id: Number(a.zusage_id) };
  });
}

/** Verbrauch buchen; endet die Zusage dadurch, wird der Schlüssel gelöscht. */
async function buchen(tx: Tx, a: Auftrag, verbraucht: number) {
  if (verbraucht > 0) await tx.query("update spendenauftraege set verbraucht = coalesce(verbraucht, 0) + $2 where id = $1", [a.id, verbraucht]);
  const z = (await tx.query("update zusagen set verbraucht = verbraucht + $2 where id = $1 returning credits, verbraucht", [a.zusage_id, verbraucht])).rows[0];
  if (verbraucht > a.credits) await zusageSchliessen(tx, a.zusage_id, `stopped to be safe: ${a.dienst} charged ${verbraucht} credits for a brief priced at ${a.credits}`);
  else if (z.verbraucht >= z.credits) await zusageSchliessen(tx, a.zusage_id, "all pledged credits are used");
}

/** Führt höchstens einen Auftrag aus. Ergebnis: was passiert ist (für Log und Tests). */
export async function einenAuftrag(db: Db): Promise<"nichts" | "fertig" | "abgebrochen" | "spaeter"> {
  const a = await nehmen(db);
  if (!a) return "nichts";
  if (a === "grenze") return "abgebrochen";
  const zeile = (await db.query("select schluessel from zusagen where id = $1 and zustand = 'offen'", [a.zusage_id])).rows[0];
  let ergebnis;
  try {
    if (!zeile?.schluessel) throw new DienstFehler("the pledge ended", "auftrag");
    const e = ERZEUGUNG.parse(a.erzeugung);
    const bildLaden = async (id: number) => {
      const b = (await db.query("select projekt_id, zustand, datei from spendenauftraege where id = $1", [id])).rows[0];
      if (!b || Number(b.projekt_id) !== a.projekt_id || !["hochgeladen", "angenommen"].includes(b.zustand) || !b.datei)
        throw new DienstFehler("image_from_brief: that brief has no image result (any more)", "auftrag");
      return readFile(b.datei);
    };
    let schluessel: string;
    try { schluessel = entschluesseln(zeile.schluessel); } catch (f) {
      if (/private key missing/.test((f as Error).message)) throw f;
      // Neues Schlüsselpaar (verloren, nicht gesichert: infra/spenden-schluessel.sh): diese Zusage kann nicht mehr erzeugen
      throw new DienstFehler("the platform can no longer read this key; please pledge again", "schluessel");
    }
    ergebnis = await erzeugen(a.dienst, schluessel, e, a.max_mb * 1048576, bildLaden);
  } catch (f) {
    const fehler = f instanceof DienstFehler ? f : new DienstFehler("internal error while generating", "auftrag");
    if (!(f instanceof DienstFehler)) console.error(`spendenwerk: brief ${a.id}:`, (f as Error).message);
    await transaktion(db, async (tx) => {
      await buchen(tx, a, fehler.verbraucht);
      if (fehler.art === "schluessel") await zusageSchliessen(tx, a.zusage_id, fehler.message);
      // Vorübergehend (Dienst beschäftigt): in etwa 10 Minuten wieder, ohne Pause und ohne Meldung an den Lead
      if (fehler.art === "spaeter") await tx.query(`update spendenauftraege set begonnen = now() - interval '${SPERRE}' + interval '10 minutes' where id = $1`, [a.id]);
      else {
        await tx.query("update spendenauftraege set begonnen = null, abgebrochen = now(), abbruchgrund = $2 where id = $1 and zustand = 'offen'", [a.id, fehler.message.slice(0, 500)]);
        await protokollieren(tx, { projekt_id: a.projekt_id, akteur: AKTEUR, art: "spendenauftrag_abgebrochen", bezug: `spendenauftrag:${a.id}`, daten: { grund: fehler.message.slice(0, 500), credits: fehler.verbraucht } });
      }
    });
    return fehler.art === "spaeter" ? "spaeter" : "abgebrochen";
  }

  // Ergebnis ablegen wie ein Upload (spenden.ts hochladen): Inhalt passt zur Endung, Prüfsumme, eindeutiger Name
  const ordner = join(speicher(), a.slug);
  await mkdir(ordner, { recursive: true });
  const sha = createHash("sha256").update(ergebnis.daten).digest("hex");
  const ziel = join(ordner, `${a.id}-${sha.slice(0, 16)}.${ergebnis.endung}`);
  const passt = inhaltPasst(ergebnis.endung, ergebnis.daten.subarray(0, 65536)) !== false;
  if (passt) {
    const tmp = join(ordner, `.${a.id}-${randomBytes(6).toString("hex")}.part`);
    await writeFile(tmp, ergebnis.daten, { flag: "wx" });
    await rename(tmp, ziel);
  }
  let abgelegt = false;
  await transaktion(db, async (tx) => {
    const jetzt = (await tx.query("select zustand from spendenauftraege where id = $1 for update", [a.id])).rows[0];
    if (!passt) {
      await tx.query("update spendenauftraege set begonnen = null, abgebrochen = now(), abbruchgrund = $2 where id = $1", [a.id, `the service returned no valid .${ergebnis.endung} file`]);
    } else if (jetzt?.zustand === "offen") {
      // Erst abliefern, dann buchen: Endet die Zusage durch diese Buchung, ist das bezahlte Ergebnis trotzdem da.
      // (Inzwischen zurückgezogen, weil der Spender beendet hat? Dann bleibt die Datei nicht liegen; gebucht wird trotzdem.)
      await tx.query("update spendenauftraege set zustand = 'hochgeladen', datei = $2, dateiname = $3, sha256 = $4, bytes = $5, hochgeladen = now(), begonnen = null, abgebrochen = null, abbruchgrund = null where id = $1",
        [a.id, ziel, `brief-${a.id}.${ergebnis.endung}`, sha, ergebnis.daten.length]);
      await protokollieren(tx, { projekt_id: a.projekt_id, akteur: AKTEUR, art: "spende_hochgeladen", bezug: `spendenauftrag:${a.id}`,
        daten: { sha256: sha, bytes: ergebnis.daten.length, weg: "schluessel", credits: ergebnis.verbraucht } });
      abgelegt = true;
    }
    await buchen(tx, a, ergebnis.verbraucht);
  });
  if (!abgelegt && passt) await rm(ziel, { force: true });
  return abgelegt ? "fertig" : "abgebrochen";
}

/** Dauerschleife des Containers: Aufträge der Reihe nach, stündlich abgelaufene Zusagen beenden. */
export async function laufen(db: Db, signal: AbortSignal) {
  let abgelaufen = 0;
  while (!signal.aborted) {
    try {
      if (Date.now() - abgelaufen > 3_600_000) { const n = await abgelaufeneBeenden(db); if (n) console.log(`spendenwerk: ${n} expired pledges ended`); abgelaufen = Date.now(); }
      const r = await einenAuftrag(db);
      if (r !== "nichts") { console.log(`spendenwerk: brief ${r}`); continue; }
    } catch (e) {
      console.error("spendenwerk:", (e as Error).message);
    }
    await new Promise((ok) => setTimeout(ok, 15_000));
  }
}
