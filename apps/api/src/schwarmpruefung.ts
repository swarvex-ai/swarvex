// Bewertung durch den Schwarm: Geprüfte Einreichungen (Prüfstraße grün) bewerten auch andere Mitstreiter, damit die Bewertung
// mit dem Schwarm mitwächst und der Lead nicht der Engpass bleibt. Aufgaben der Stufe A werden nach zwei unabhängigen
// Zustimmungen automatisch angenommen; ab Stufe B entscheiden Lead oder Officer, mit den Fremdprüfungen als Vorarbeit.
// Wer prüft: nicht der Einreicher, nicht Lead oder Visionär des Projekts, Vertrauensstufe ab 1, in Clean-Room-Projekten nur von
// derselben Seite der Trennwand wie die Aufgabe. Jede Fremdprüfung steht mit Urteil im öffentlichen Protokoll.
// Nennt der Prüfer sein Modell, kommen Einreichungen eines anderen Modells zuerst: Ein zweites Modell findet andere Fehler.
import type { Db, Tx } from "./db.ts";
import { transaktion } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, rollenVon, variantenSchliessen, wechseln, type Wer } from "./aufgaben.ts";
import { raeumeVon, trennwand } from "./cleanroom.ts";
import { modellSchluessel } from "./modelle.ts";

/** Zustimmungen verschiedener Menschen, nach denen eine Aufgabe der Stufe A ohne Lead angenommen wird. */
export const AUTO_ANNAHME = { stufen: ["A"], zustimmungen: 2 };
const MIN_VERTRAUEN = 1;

async function darfPruefen(tx: Db | Tx, wer: Wer, a: { projekt_id: number | string; raum: string | null }, autor: number) {
  if (autor === wer.menschId) return "nobody reviews their own submission, not even through a second agent";
  const rollen = await rollenVon(tx, wer.menschId, Number(a.projekt_id));
  if (rollen.includes("lead") || rollen.includes("visionaer")) return "lead and visionary review with review_submission, not as peers";
  const m = (await tx.query("select vertrauensstufe, gesperrt from menschen where id = $1", [wer.menschId])).rows[0];
  if (m.gesperrt) return "account blocked";
  if (m.vertrauensstufe < MIN_VERTRAUEN) return `peer reviews need trust level ${MIN_VERTRAUEN}`;
  if (a.raum) {
    const grund = trennwand(await raeumeVon(tx, wer.menschId, Number(a.projekt_id)), a.raum as never);
    if (grund) return grund;
  }
  return null;
}

/** Geprüfte Einreichungen, die auf Fremdprüfungen warten und die diese Person prüfen darf (ein Projekt oder alle). */
export async function offeneFremdpruefungen(db: Db, wer: Wer, projektSlug?: string, modell?: string) {
  const r = (await db.query(
    `select e.id::int as einreichung_id, a.id::int as aufgabe_id, p.slug as projekt, a.titel, a.beschreibung, a.akzeptanz, a.pfade, a.stufe, a.raum,
       a.projekt_id, ag.mensch_id::int as autor, e.modell as autor_modell, e.zusammenfassung, e.patch, pr.bericht,
       (select count(*)::int from fremdpruefungen f where f.einreichung_id = e.id and f.urteil = 'zustimmen') as zustimmungen
     from aufgaben a join projekte p on p.id = a.projekt_id
     join lateral (select * from einreichungen where aufgabe_id = a.id order by id desc limit 1) e on true
     join agenten ag on ag.id = e.agent_id
     left join lateral (select bericht from pruefungen where einreichung_id = e.id order by id desc limit 1) pr on true
     where a.zustand = 'geprueft_ok' and not p.gesperrt and ($2::text is null or p.slug = $2)
       and not exists (select 1 from fremdpruefungen f where f.einreichung_id = e.id and f.mensch_id = $1)
     order by e.id limit 50`, [wer.menschId, projektSlug ?? null])).rows;
  const erlaubt = [];
  for (const z of r) if (!(await darfPruefen(db, wer, z, z.autor))) erlaubt.push(z);
  // anderes Modell zuerst (stabil sortiert, sonst in der Reihenfolge des Eingangs)
  if (modell?.trim()) {
    const eigenes = modellSchluessel(modell);
    const gleich = (z: { autor_modell: string | null }) => (z.autor_modell && modellSchluessel(z.autor_modell) === eigenes ? 1 : 0);
    erlaubt.sort((x, y) => gleich(x) - gleich(y));
  }
  return erlaubt.slice(0, 20).map(({ projekt_id, autor, raum, ...z }) => ({
    ...z, bericht: String(z.bericht ?? "").slice(-4000),
    automatisch_ab: AUTO_ANNAHME.stufen.includes(z.stufe) ? AUTO_ANNAHME.zustimmungen : null,
  }));
}

/** Fremdprüfung abgeben; bei Stufe A und genug Zustimmungen wird die Aufgabe automatisch angenommen. */
export async function fremdPruefen(db: Db, wer: Wer, einreichungId: number, urteil: "zustimmen" | "aenderungen", begruendung: string, modell?: string) {
  if (begruendung.trim().length < 20) throw new Abgelehnt("reason: at least one concrete sentence (20 characters)");
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select e.*, ag.mensch_id::int as autor from einreichungen e join agenten ag on ag.id = e.agent_id where e.id = $1", [einreichungId])).rows[0];
    if (!e) throw new Abgelehnt("unknown submission");
    const a = (await tx.query("select * from aufgaben where id = $1 for update", [e.aufgabe_id])).rows[0];
    if (a.zustand !== "geprueft_ok") throw new Abgelehnt("peer reviews only for submissions with a passing check that wait for review");
    const letzte = (await tx.query("select max(id)::int as id from einreichungen where aufgabe_id = $1", [a.id])).rows[0].id;
    if (letzte !== einreichungId) throw new Abgelehnt("this submission was replaced by a newer one");
    const nein = await darfPruefen(tx, wer, a, e.autor);
    if (nein) throw new Abgelehnt(nein);
    try {
      await tx.query("insert into fremdpruefungen(einreichung_id, agent_id, mensch_id, urteil, begruendung, modell) values ($1,$2,$3,$4,$5,$6)",
        [einreichungId, wer.agentId, wer.menschId, urteil, begruendung.trim().slice(0, 4000), modell?.trim().slice(0, 120) || null]);
    } catch (err) {
      if ((err as { code?: string }).code === "23505") throw new Abgelehnt("you already reviewed this submission");
      throw err;
    }
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: `agent:${wer.agentId}`, art: "fremdpruefung", bezug: `aufgabe:${a.id}`,
      daten: { einreichung: einreichungId, urteil, anderes_modell: modell?.trim() && e.modell ? modellSchluessel(modell) !== modellSchluessel(e.modell) : null } });
    const alle = (await tx.query(
      "select f.urteil, f.begruendung, f.agent_id, m.login from fremdpruefungen f join menschen m on m.id = f.mensch_id where f.einreichung_id = $1 order by f.id", [einreichungId])).rows;
    const zu = alle.filter((f) => f.urteil === "zustimmen");
    const automatisch = AUTO_ANNAHME.stufen.includes(a.stufe) && zu.length >= AUTO_ANNAHME.zustimmungen && !alle.some((f) => f.urteil === "aenderungen");
    if (automatisch) {
      const grund = `Automatically accepted after ${zu.length} independent peer reviews (${zu.map((f) => f.login).join(", ")}): ` + zu.map((f) => f.begruendung).join(" | ");
      await tx.query("insert into bewertungen(einreichung_id, agent_id, urteil, begruendung) values ($1,$2,'annehmen',$3)", [einreichungId, wer.agentId, grund.slice(0, 8000)]);
      await wechseln(tx, a, "angenommen", "system", "bewertet", { einreichung: einreichungId, urteil: "annehmen", automatisch: true, fremdpruefer: zu.map((f) => f.login) });
      await variantenSchliessen(tx, a);
    }
    return { einreichung: einreichungId, zustimmungen: zu.length, automatisch_angenommen: automatisch };
  });
}

/** Fremdprüfungen zu einer Einreichung, für Lead und Officer in pending_submissions. */
export async function fremdpruefungenZu(db: Db, einreichungId: number) {
  return (await db.query(
    "select f.urteil, f.begruendung, m.login as von, f.erstellt from fremdpruefungen f join menschen m on m.id = f.mensch_id where f.einreichung_id = $1 order by f.id", [einreichungId])).rows;
}
