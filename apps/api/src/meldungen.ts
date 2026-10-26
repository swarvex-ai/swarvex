// Meldungen von außen (Formular auf der Website) und Sperren durch die Betreiber (Issue #19, #29).
// Ablauf: Meldung → Prüfung → bei Bedarf Sperre bzw. Entfernung → Antwort an die meldende Person (per E-Mail, von Hand).
import { z } from "zod";
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { leasesBeenden } from "./aufgaben.ts";

/** Report kinds: English outside, German in the database. */
const ARTEN = { infringement: "rechtsverletzung", abuse: "missbrauch", security: "sicherheit", other: "sonstiges" } as const;

export const MeldungSchema = z.object({
  kind: z.enum(["infringement", "abuse", "security", "other"]),
  project: z.string().max(80).optional().default(""),
  location: z.string().max(500).optional().default(""),
  description: z.string().trim().min(20, "Please describe the case in more detail (at least 20 characters).").max(5000),
  name: z.string().trim().max(200).optional().default(""),
  email: z.string().trim().email().max(200),
  rights_holder: z.boolean().optional().default(false),
  confirmed: z.literal(true, { error: "Please confirm that your information is accurate to the best of your knowledge." }),
  website: z.string().max(0).optional(), // Honigtopf: Menschen sehen das Feld nicht, Bots füllen es aus
});

export async function meldungAnlegen(db: Db, eingabe: unknown) {
  const m = MeldungSchema.parse(eingabe);
  const art = ARTEN[m.kind];
  return transaktion(db, async (tx) => {
    const p = m.project ? (await tx.query("select id from projekte where slug = $1", [m.project])).rows[0] : undefined;
    const r = (await tx.query(
      `insert into meldungen(art, projekt_id, bezug, beschreibung, name, email, rechteinhaber) values ($1,$2,$3,$4,$5,$6,$7) returning id`,
      [art, p?.id ?? null, m.location, m.description, m.name, m.email, m.rights_holder])).rows[0];
    // Öffentlich nur, dass es eine Meldung gibt – ohne Inhalt und ohne persönliche Angaben.
    await protokollieren(tx, { projekt_id: p ? Number(p.id) : null, akteur: "system", art: "meldung_eingegangen", bezug: `meldung:${r.id}`, daten: { art } });
    return { id: Number(r.id) };
  });
}

export async function meldungenListe(db: Db, nurOffen = true) {
  return (await db.query(
    `select m.id, m.art, p.slug as projekt, m.bezug, m.beschreibung, m.name, m.email, m.rechteinhaber, m.zustand, m.ergebnis, m.erstellt
     from meldungen m left join projekte p on p.id = m.projekt_id ${nurOffen ? "where m.zustand in ('offen', 'in_pruefung')" : ""} order by m.id`)).rows;
}

export async function meldungAbschliessen(db: Db, id: number, zustand: "in_pruefung" | "erledigt" | "abgelehnt", ergebnis: string) {
  return transaktion(db, async (tx) => {
    const r = await tx.query(`update meldungen set zustand = $2, ergebnis = $3, erledigt = case when $2 in ('erledigt','abgelehnt') then now() else null end
                              where id = $1 returning projekt_id`, [id, zustand, ergebnis]);
    if (!r.rowCount) throw new Error(`Meldung ${id} gibt es nicht`);
    await protokollieren(tx, { projekt_id: r.rows[0].projekt_id ? Number(r.rows[0].projekt_id) : null, akteur: "system", art: "meldung_bearbeitet",
      bezug: `meldung:${id}`, daten: { zustand, ergebnis } });
  });
}

/** Sperrt bzw. entsperrt einen Menschen (Anmeldung, MCP, Reservierungen) oder ein Projekt (Listen, Aufgaben, Snapshots). */
export async function sperren(db: Db, ziel: { mensch?: number; projekt?: number }, gesperrt: boolean, grund: string) {
  return transaktion(db, async (tx) => {
    if (ziel.mensch) {
      await tx.query("update menschen set gesperrt = $2, sperrgrund = $3 where id = $1", [ziel.mensch, gesperrt, gesperrt ? grund : null]);
      await protokollieren(tx, { projekt_id: null, akteur: "system", art: gesperrt ? "mensch_gesperrt" : "mensch_entsperrt", bezug: `mensch:${ziel.mensch}`, daten: { grund } });
    }
    if (ziel.projekt) {
      await tx.query("update projekte set gesperrt = $2, sperrgrund = $3 where id = $1", [ziel.projekt, gesperrt, gesperrt ? grund : null]);
      // Laufende Reservierungen enden sofort; offene Aufgaben bleiben, werden aber nicht mehr angeboten.
      if (gesperrt) await leasesBeenden(tx, { projekt: ziel.projekt }, "reservierung_beendet_sperre");
      await protokollieren(tx, { projekt_id: ziel.projekt, akteur: "system", art: gesperrt ? "projekt_gesperrt" : "projekt_entsperrt", bezug: `projekt:${ziel.projekt}`, daten: { grund } });
    }
  });
}
