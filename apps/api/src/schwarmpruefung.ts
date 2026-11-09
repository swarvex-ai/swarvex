// Bewertung durch den Schwarm: Geprüfte Einreichungen (Prüfstraße grün) bewerten auch andere Mitstreiter, damit die Bewertung
// mit dem Schwarm mitwächst und der Lead nicht der Engpass bleibt. Aufgaben der Stufe A werden nach zwei unabhängigen
// Zustimmungen automatisch angenommen; ab Stufe B entscheiden Lead oder Bereichs-Lead, mit den Fremdprüfungen als Vorarbeit.
// Eigene Beiträge des Leads (neVs 09.11.2026): Stellt ein Lead oder Visionär einen eigenen Agenten, darf er dessen Einreichungen
// nicht bewerten. Dann ersetzt die Zustimmung eines anderen Menschen die Bewertung durch den Lead (Stufe A und B), „Änderungen“
// schickt in die Nacharbeit; ab Stufe C braucht es einen anderen Menschen, der das Projekt führt.
// Wer prüft: nicht der Einreicher, nicht Lead oder Visionär des Projekts, Vertrauensstufe ab 1, in Clean-Room-Projekten nur von
// derselben Seite der Trennwand wie die Aufgabe. Jede Fremdprüfung steht mit Urteil im öffentlichen Protokoll.
// Nennt der Prüfer sein Modell, kommen Einreichungen eines anderen Modells zuerst: Ein zweites Modell findet andere Fehler.
import type { Db, Tx } from "./db.ts";
import { transaktion } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, rollenVon, variantenSchliessen, VOM_LEAD, wechseln, type Wer } from "./aufgaben.ts";
import { raeumeVon, trennwand } from "./cleanroom.ts";
import { modellSchluessel } from "./modelle.ts";
import { einreichungsVorschauUrl } from "./konfig.ts";
import { einreichungsVorschau } from "./katalog.ts";
import { vertrauenAktualisieren } from "./vertrauen.ts";

/** Zustimmungen verschiedener Menschen, nach denen eine Aufgabe der Stufe A ohne Lead angenommen wird. */
export const AUTO_ANNAHME = { stufen: ["A"], zustimmungen: 2 };
/** Eigene Beiträge des Leads: eine Zustimmung eines anderen Menschen ersetzt die Bewertung durch den Lead. */
export const LEAD_EIGEN = { stufen: ["A", "B"], zustimmungen: 1 };
const automatischAb = (stufe: string, vomLead: boolean) =>
  vomLead && LEAD_EIGEN.stufen.includes(stufe) ? LEAD_EIGEN.zustimmungen : AUTO_ANNAHME.stufen.includes(stufe) ? AUTO_ANNAHME.zustimmungen : null;
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
       a.projekt_id, ag.mensch_id::int as autor, e.modell as autor_modell, e.zusammenfassung, e.patch, pr.bericht, e.vorschau->>'status' as vorschau_status,
       ${VOM_LEAD("e", "a.projekt_id")} as vom_lead,
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
  // eigene Beiträge eines Leads zuerst (nur der Schwarm kann sie annehmen), dann ein anderes Modell (stabil sortiert,
  // sonst in der Reihenfolge des Eingangs)
  const eigenes = modell?.trim() ? modellSchluessel(modell) : null;
  const gleich = (z: { autor_modell: string | null }) => (eigenes && z.autor_modell && modellSchluessel(z.autor_modell) === eigenes ? 1 : 0);
  erlaubt.sort((x, y) => Number(y.vom_lead) - Number(x.vom_lead) || gleich(x) - gleich(y));
  return Promise.all(erlaubt.slice(0, 20).map(async ({ projekt_id, autor, raum, vorschau_status, ...z }) => ({
    ...(vorschau_status === "ok" && einreichungsVorschauUrl(1) ? { vorschau_url: await einreichungsVorschau(db, z.projekt, Number(z.einreichung_id)) } : {}),
    ...z, bericht: String(z.bericht ?? "").slice(-4000),
    automatisch_ab: automatischAb(z.stufe, z.vom_lead),
  })));
}

/**
 * Fremdprüfung abgeben; bei Stufe A und genug Zustimmungen wird die Aufgabe automatisch angenommen. Bei eigenen
 * Beiträgen des Leads (Stufe A und B) nimmt schon eine Zustimmung an, und „Änderungen“ schickt in die Nacharbeit.
 */
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
    await vertrauenAktualisieren(tx, wer.menschId, "peer review");
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: `agent:${wer.agentId}`, art: "fremdpruefung", bezug: `aufgabe:${a.id}`,
      daten: { einreichung: einreichungId, urteil, anderes_modell: modell?.trim() && e.modell ? modellSchluessel(modell) !== modellSchluessel(e.modell) : null } });
    const alle = (await tx.query(
      "select f.urteil, f.begruendung, f.agent_id, m.login from fremdpruefungen f join menschen m on m.id = f.mensch_id where f.einreichung_id = $1 order by f.id", [einreichungId])).rows;
    const zu = alle.filter((f) => f.urteil === "zustimmen");
    const vomLead = Boolean((await tx.query(`select ${VOM_LEAD("e", "$2")} as v from einreichungen e where e.id = $1`, [einreichungId, a.projekt_id])).rows[0].v);
    const leadEigen = vomLead && LEAD_EIGEN.stufen.includes(a.stufe);
    const ab = automatischAb(a.stufe, vomLead);
    const automatisch = ab !== null && zu.length >= ab && !alle.some((f) => f.urteil === "aenderungen");
    const ich = alle.filter((f) => Number(f.agent_id) === wer.agentId).at(-1)?.login ?? "";
    if (leadEigen && urteil === "aenderungen") {
      // eigener Beitrag des Leads: der Lead darf nicht entscheiden, also schickt der Einwand in die Nacharbeit
      const grund = `Sent back for rework by a peer review of the lead's own submission (${ich}): ${begruendung.trim()}`;
      await tx.query("insert into bewertungen(einreichung_id, agent_id, urteil, begruendung) values ($1,$2,'nacharbeit',$3)", [einreichungId, wer.agentId, grund.slice(0, 8000)]);
      await wechseln(tx, a, "offen", "system", "bewertet", { einreichung: einreichungId, urteil: "nacharbeit", automatisch: true, fremdpruefer: [ich] });
      return { einreichung: einreichungId, zustimmungen: zu.length, automatisch_angenommen: false, zurueckgegeben: true };
    }
    if (automatisch) {
      const grund = (leadEigen ? `Accepted by a peer review of the lead's own submission (${zu.map((f) => f.login).join(", ")}): `
        : `Automatically accepted after ${zu.length} independent peer reviews (${zu.map((f) => f.login).join(", ")}): `) + zu.map((f) => f.begruendung).join(" | ");
      await tx.query("insert into bewertungen(einreichung_id, agent_id, urteil, begruendung) values ($1,$2,'annehmen',$3)", [einreichungId, wer.agentId, grund.slice(0, 8000)]);
      await wechseln(tx, a, "angenommen", "system", "bewertet", { einreichung: einreichungId, urteil: "annehmen", automatisch: true, fremdpruefer: zu.map((f) => f.login) });
      await variantenSchliessen(tx, a);
    }
    return { einreichung: einreichungId, zustimmungen: zu.length, automatisch_angenommen: automatisch };
  });
}

/** Fremdprüfungen zu einer Einreichung, für Lead und Bereichs-Lead in pending_submissions. */
export async function fremdpruefungenZu(db: Db, einreichungId: number) {
  return (await db.query(
    "select f.urteil, f.begruendung, m.login as von, f.erstellt from fremdpruefungen f join menschen m on m.id = f.mensch_id where f.einreichung_id = $1 order by f.id", [einreichungId])).rows;
}
