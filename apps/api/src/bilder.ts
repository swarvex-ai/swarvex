// Bildvergleich im Review (E-39): Szenenbilder einer Einreichung (alt = Referenz vom Basis-Commit, neu = mit dem
// Beitrag, diff = Unterschied), von der Prüfstraße geliefert: gerendert, oder vorab aus dem Patch gelesen (E-50). Nur
// für Lead, Visionär und Bereichs-Leads des Projekts: Die Bilder zeigt ein fremder Beitrag, sie werden nicht öffentlich
// ausgeliefert.
import type { Db } from "./db.ts";
import { Abgelehnt, officersVon, rollenVon } from "./aufgaben.ts";

export const BILDER_JE_EINREICHUNG = 12;
const PNG = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
const ARTEN = { old: "alt", new: "neu", diff: "diff" } as const;

/** Prüfstraße: die Bilder einer Szene (Base64-PNGs) zu einer Einreichung speichern. */
export async function bilderSpeichern(db: Db, einreichungId: number, name: string, bilder: Record<string, unknown>) {
  if (!/^[a-z0-9][a-z0-9_.-]{0,60}$/.test(name)) throw new Abgelehnt("name: lowercase scene name");
  const schon = (await db.query("select count(distinct name)::int as n, bool_or(name = $2) as da from einreichung_bilder where einreichung_id = $1", [einreichungId, name])).rows[0];
  if (!schon.da && schon.n >= BILDER_JE_EINREICHUNG) throw new Abgelehnt(`at most ${BILDER_JE_EINREICHUNG} scenes per submission`);
  for (const [aussen, art] of Object.entries(ARTEN)) {
    const roh = bilder[aussen];
    if (roh === undefined || roh === null) continue;
    const png = Buffer.from(String(roh), "base64");
    if (png.length > 400_000 || !png.subarray(0, 8).equals(PNG)) throw new Abgelehnt(`${aussen}: a PNG of at most 400 kB`);
    await db.query(
      `insert into einreichung_bilder(einreichung_id, name, art, png) values ($1,$2,$3,$4)
       on conflict (einreichung_id, name, art) do update set png = excluded.png, erstellt = now()`, [einreichungId, name, art, png]);
  }
}

/** Darf diese Person die Bilder der Einreichung sehen? (Lead, Visionär oder Bereichs-Lead des Projekts) */
export async function bilderErlaubt(db: Db, menschId: number, einreichungId: number) {
  const p = (await db.query("select a.projekt_id from einreichungen e join aufgaben a on a.id = e.aufgabe_id where e.id = $1", [einreichungId])).rows[0];
  if (!p) throw new Abgelehnt(`no submission #${einreichungId}`);
  const rollen = await rollenVon(db, menschId, Number(p.projekt_id));
  if (!rollen.includes("lead") && !rollen.includes("visionaer") && !(await officersVon(db, menschId, Number(p.projekt_id))).length)
    throw new Abgelehnt("only the project's lead, visionary or an area lead sees the images of a submission");
  return Number(p.projekt_id);
}

/** Welche Szenen gibt es und welche Bilder je Szene. */
export async function bilderListe(db: Db, einreichungId: number) {
  const r = await db.query("select name, array_agg(art order by art) as arten from einreichung_bilder where einreichung_id = $1 group by name order by name", [einreichungId]);
  return r.rows.map((z) => ({ name: z.name as string, alt: (z.arten as string[]).includes("alt"), diff: (z.arten as string[]).includes("diff") }));
}

export async function bilderLesen(db: Db, einreichungId: number) {
  return (await db.query("select name, art, png from einreichung_bilder where einreichung_id = $1 order by name, art desc", [einreichungId])).rows as { name: string; art: "alt" | "neu" | "diff"; png: Buffer }[];
}

export async function bildLesen(db: Db, einreichungId: number, name: string, art: string): Promise<Buffer | null> {
  const a = ARTEN[art as keyof typeof ARTEN];
  if (!a) return null;
  return (await db.query("select png from einreichung_bilder where einreichung_id = $1 and name = $2 and art = $3", [einreichungId, name, a])).rows[0]?.png ?? null;
}

export async function bilderAufraeumen(db: Db) {
  return (await db.query("delete from einreichung_bilder where erstellt < now() - interval '30 days'")).rowCount ?? 0;
}
