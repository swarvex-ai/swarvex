// Rückmeldungen zur spielbaren Vorschau: Angemeldete Besucher sagen auf der Projektseite, was gut war und was hakt, zum
// Stand, den sie gespielt haben. Lead, Visionär und Bereichs-Leads lesen sie mit preview_feedback und machen daraus Aufgaben
// (mark_feedback). Der Text bleibt projektintern und ist für den Lead fremde Eingabe; öffentlich ist nur die Anzahl.
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, officersVon, projektPerSlug, rollenVon, type Wer } from "./aufgaben.ts";

export const FEEDBACK_JE_TAG = 10;

async function darfLesen(db: Db, menschId: number, projektId: number) {
  const rollen = await rollenVon(db, menschId, projektId);
  return rollen.includes("lead") || rollen.includes("visionaer") || (await officersVon(db, menschId, projektId)).length > 0;
}

export async function feedbackGeben(db: Db, menschId: number, slug: string, f: { commit?: string; gut?: string; hakt?: string; geraet?: string }) {
  const gut = String(f.gut ?? "").trim(), hakt = String(f.hakt ?? "").trim(), geraet = String(f.geraet ?? "").trim();
  if (!gut && !hakt) throw new Abgelehnt("say what was good or what did not work");
  if (gut.length > 2000 || hakt.length > 2000 || geraet.length > 120) throw new Abgelehnt("at most 2,000 characters per answer, 120 for the device");
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, slug);
    if (!p.vorschau?.ok_commit) throw new Abgelehnt("this project has no preview yet");
    const m = (await tx.query("select gesperrt from menschen where id = $1", [menschId])).rows[0];
    if (!m || m.gesperrt) throw new Abgelehnt("account blocked");
    const heute = Number((await tx.query("select count(*) from vorschau_feedback where mensch_id = $1 and projekt_id = $2 and erstellt > now() - interval '1 day'", [menschId, p.id])).rows[0].count);
    if (heute >= FEEDBACK_JE_TAG) throw new Abgelehnt(`at most ${FEEDBACK_JE_TAG} feedback entries per day and project`);
    const commit = /^[0-9a-f]{7,64}$/.test(String(f.commit ?? "")) ? String(f.commit) : p.vorschau.ok_commit;
    const id = Number((await tx.query(
      "insert into vorschau_feedback(projekt_id, mensch_id, commit, gut, hakt, geraet) values ($1,$2,$3,$4,$5,$6) returning id",
      [p.id, menschId, commit, gut, hakt, geraet || null])).rows[0].id);
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: `mensch:${menschId}`, art: "vorschau_feedback", bezug: `projekt:${p.id}`, daten: { commit } });
    return { feedback_id: id };
  });
}

export async function feedbackListe(db: Db, wer: Wer, slug: string, nurNeu = true) {
  const p = await projektPerSlug(db, slug);
  if (!(await darfLesen(db, wer.menschId, Number(p.id)))) throw new Abgelehnt("only the project's lead, visionary and area leads read preview feedback");
  return (await db.query(
    `select f.id::int as feedback_id, m.login as von, f.commit, f.gut, f.hakt, f.geraet, f.zustand, f.aufgabe_id::int, f.erstellt
     from vorschau_feedback f join menschen m on m.id = f.mensch_id
     where f.projekt_id = $1 and ($2::boolean = false or f.zustand = 'neu') order by f.id desc limit 100`, [p.id, nurNeu])).rows;
}

export async function feedbackMarkieren(db: Db, wer: Wer, feedbackId: number, zustand: "gesehen" | "aufgabe", aufgabeId?: number) {
  const f = (await db.query("select projekt_id from vorschau_feedback where id = $1", [feedbackId])).rows[0];
  if (!f) throw new Abgelehnt("unknown feedback");
  if (!(await darfLesen(db, wer.menschId, Number(f.projekt_id)))) throw new Abgelehnt("only the project's lead, visionary and area leads handle preview feedback");
  if (zustand === "aufgabe") {
    if (!aufgabeId) throw new Abgelehnt("task_id: the task you created from this feedback");
    const a = (await db.query("select 1 from aufgaben where id = $1 and projekt_id = $2", [aufgabeId, f.projekt_id])).rowCount;
    if (!a) throw new Abgelehnt("the task must belong to this project");
  }
  await db.query("update vorschau_feedback set zustand = $2, aufgabe_id = $3 where id = $1", [feedbackId, zustand, zustand === "aufgabe" ? aufgabeId : null]);
  return { feedback_id: feedbackId, zustand };
}

/** Öffentlich: wie viele Rückmeldungen es zum Projekt gibt und wie viele noch niemand angesehen hat. */
export async function feedbackZahlen(db: Db, projektId: number) {
  return (await db.query(
    "select count(*)::int as gesamt, (count(*) filter (where zustand = 'neu'))::int as neu from vorschau_feedback where projekt_id = $1", [projektId])).rows[0] as { gesamt: number; neu: number };
}
