// Bewertung durch den Schwarm: Geprüfte Einreichungen (Prüfstraße grün) bewerten auch andere Mitstreiter, damit die Bewertung
// mit dem Schwarm mitwächst und der Lead nicht der Engpass bleibt. Aufgaben der Stufe A werden nach zwei unabhängigen
// Zustimmungen automatisch angenommen; ab Stufe B entscheiden Lead oder Bereichs-Lead, mit den Fremdprüfungen als Vorarbeit.
// Eigene Beiträge des Leads (neVs 09.11.2026): Stellt ein Lead oder Visionär einen eigenen Agenten, darf er dessen Einreichungen
// nicht bewerten. Dann ersetzt die Zustimmung eines anderen Menschen die Bewertung durch den Lead (Stufe A und B), „Änderungen“
// schickt in die Nacharbeit; ab Stufe C braucht es einen anderen Menschen, der das Projekt führt.
// Wer prüft: nicht der Einreicher, nicht Lead oder Visionär des Projekts, Vertrauensstufe ab 1, in Clean-Room-Projekten nur von
// derselben Seite der Trennwand wie die Aufgabe. Jede Fremdprüfung steht mit Urteil im öffentlichen Protokoll.
// Fremdbewertung durch den Runner (neVs 10.11.2026), vier Leitplanken: Der Prüfer nennt sein Modell, und es muss aus einer
// anderen Familie stammen als das der Einreichung (ein zweites Modell findet andere Fehler, eines derselben Familie oft
// dieselben nicht). Hat die Prüfstraße Szenenbilder geliefert, stimmt nur zu, wer sie mit einem Modell mit Bildverständnis
// angesehen hat. Prüfpunkte des Leads (Fragen, kein Urteil) beantwortet der Prüfer einzeln. Und widerlegte Zustimmungen
// (Regression gemeldet, oder eine Aufgabe behebt die angenommene) zählen je Modell (pruefer_bilanz in modelle.ts).
// Messung je Mensch und Modell (E-43, neVs 10.11.2026, noch ohne Folgen): Zustimmungen, die der Lead danach zurückschickt
// (übersehen) oder die widerlegt werden, ergeben im gleitenden Fenster Probezeit, bewährt oder gebremst (modelle.ts).
import type { Db, Tx } from "./db.ts";
import { transaktion } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, mussLeadOderOfficerSein, rollenVon, variantenSchliessen, VOM_LEAD, wechseln, type AbbruchUrsache, type Wer } from "./aufgaben.ts";
import { raeumeVon, trennwand } from "./cleanroom.ts";
import { modellFamilie, modellSchluessel, prueferStatus, siehtBilder } from "./modelle.ts";
import { einreichungsVorschauUrl } from "./konfig.ts";
import { einreichungsVorschau } from "./katalog.ts";
import { vertrauenAktualisieren } from "./vertrauen.ts";
import { aendertSzenenbilder } from "./gegenpruefung.ts";

/** Zustimmungen verschiedener Menschen, nach denen eine Aufgabe der Stufe A ohne Lead angenommen wird. Es zählen nur
 *  Prüfer mit einem gemergten Beitrag in diesem Projekt: Stufe 1 lässt sich mit Zweitkonten in einem eigenen Projekt
 *  verdienen, und zwei solche Konten nähmen sonst in fremden Projekten ohne deren Lead an (Sicherheitsaudit, Nachtrag
 *  13.11.2026). Andere Zustimmungen stehen weiter in der Bewertung und helfen dem Lead. */
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
    const bisher = await raeumeVon(tx, wer.menschId, Number(a.projekt_id));
    const grund = trennwand(bisher, a.raum as never);
    if (grund) return grund;
    // Clean Room: Die Liste der offenen Fremdprüfungen gibt den Patch schon heraus. Nur wer auf dieser Seite der Wand
    // schon arbeitet, sieht und prüft ihn (Sicherheitsaudit 12.11.2026).
    if (!bisher.includes(a.raum as never)) return "clean room: peer reviews of a room are for people who already work in that room of this project";
  }
  return null;
}

/** Geprüfte Einreichungen, die auf Fremdprüfungen warten und die diese Person prüfen darf (ein Projekt oder alle). */
export async function offeneFremdpruefungen(db: Db, wer: Wer, projektSlug?: string, modell?: string) {
  const r = (await db.query(
    `select e.id::int as einreichung_id, a.id::int as aufgabe_id, p.slug as projekt, a.titel, a.beschreibung, a.akzeptanz, a.pfade, a.stufe, a.raum,
       a.projekt_id, ag.mensch_id::int as autor, e.modell as autor_modell, e.zusammenfassung, e.patch, pr.bericht, e.vorschau->>'status' as vorschau_status,
       ${VOM_LEAD("e", "a.projekt_id")} as vom_lead, e.pruefpunkte,
       (select coalesce(array_agg(distinct b.name order by b.name), '{}') from einreichung_bilder b where b.einreichung_id = e.id) as bilder,
       (select count(*)::int from fremdpruefungen f where f.einreichung_id = e.id and f.urteil = 'zustimmen') as zustimmungen
     from aufgaben a join projekte p on p.id = a.projekt_id
     join lateral (select * from einreichungen where aufgabe_id = a.id order by id desc limit 1) e on true
     join agenten ag on ag.id = e.agent_id
     left join lateral (select bericht from pruefungen where einreichung_id = e.id order by id desc limit 1) pr on true
     where a.zustand = 'geprueft_ok' and not p.gesperrt and ($2::text is null or p.slug = $2)
       and not exists (select 1 from fremdpruefungen f where f.einreichung_id = e.id and f.mensch_id = $1)
     order by e.id limit 50`, [wer.menschId, projektSlug ?? null])).rows;
  const erlaubt = [];
  // Nennt der Prüfer sein Modell, fallen Einreichungen derselben Modellfamilie weg: die darf er nicht bewerten
  const familie = modell?.trim() ? modellFamilie(modell) : null;
  for (const z of r) {
    if (familie && z.autor_modell && modellFamilie(z.autor_modell) === familie) continue;
    if (!(await darfPruefen(db, wer, z, z.autor))) erlaubt.push(z);
  }
  // eigene Beiträge eines Leads zuerst (nur der Schwarm kann sie annehmen), dann die, denen dieses Modell zustimmen darf
  // (stabil sortiert, sonst in der Reihenfolge des Eingangs)
  // Sichtbar ist eine Änderung, wenn es Szenenbilder gibt oder der Patch Referenzbilder ändert (die Bilder kommen dann
  // gleich vom Prüfserver); zustimmen erst, wenn sie da sind, und nur mit einem Modell, das Bilder liest
  const zustimmbar = (z: { bilder: string[]; patch: string }) => (!z.bilder.length && !aendertSzenenbilder(z.patch))
    || (z.bilder.length > 0 && (modell?.trim() ? siehtBilder(modell) : true));
  erlaubt.sort((x, y) => Number(y.vom_lead) - Number(x.vom_lead) || Number(zustimmbar(y)) - Number(zustimmbar(x)));
  return Promise.all(erlaubt.slice(0, 20).map(async ({ projekt_id, autor, raum, vorschau_status, pruefpunkte, bilder, ...z }) => ({
    ...(vorschau_status === "ok" && einreichungsVorschauUrl(1) ? { vorschau_url: await einreichungsVorschau(db, z.projekt, Number(z.einreichung_id)) } : {}),
    ...z, bericht: String(z.bericht ?? "").slice(-4000),
    automatisch_ab: automatischAb(z.stufe, z.vom_lead),
    ...(pruefpunkte?.length ? { pruefpunkte } : {}),
    ...(bilder.length ? { bilder, bild_adresse: `/submissions/${z.einreichung_id}/images/<scene>/<old|new|diff>.png` } : {}),
    zustimmen_moeglich: zustimmbar({ bilder, patch: z.patch }),
    ...(!modell?.trim() ? { hinweis: "name your model (model): only a model of another family than the submission's may review it" }
      : !bilder.length && aendertSzenenbilder(z.patch) ? { hinweis: "this submission changes what you see; its scene images are not there yet (a few minutes): you may request changes, not approve yet" }
      : !zustimmbar({ bilder, patch: z.patch }) ? { hinweis: "this submission changes what you see (images) and your model does not read images: you may request changes, not approve" } : {}),
  })));
}

/**
 * Fremdprüfung abgeben; bei Stufe A und genug Zustimmungen wird die Aufgabe automatisch angenommen. Bei eigenen
 * Beiträgen des Leads (Stufe A und B) nimmt schon eine Zustimmung an, und „Änderungen“ schickt in die Nacharbeit.
 */
export async function fremdPruefen(db: Db, wer: Wer, einreichungId: number, urteil: "zustimmen" | "aenderungen", begruendung: string, modell?: string,
  zusatz: { antworten?: string[]; bilderGesehen?: boolean } = {}) {
  if (begruendung.trim().length < 20) throw new Abgelehnt("reason: at least one concrete sentence (20 characters)");
  if (!modell?.trim()) throw new Abgelehnt("model: name the model that did this review; it must be of another family than the submission's");
  const antworten = (zusatz.antworten ?? []).map((x) => String(x).trim());
  const ergebnis = await transaktion(db, async (tx) => {
    const e = (await tx.query("select e.*, ag.mensch_id::int as autor from einreichungen e join agenten ag on ag.id = e.agent_id where e.id = $1", [einreichungId])).rows[0];
    if (!e) throw new Abgelehnt("unknown submission");
    const a = (await tx.query("select * from aufgaben where id = $1 for update", [e.aufgabe_id])).rows[0];
    if (a.zustand !== "geprueft_ok") throw new Abgelehnt("peer reviews only for submissions with a passing check that wait for review");
    const letzte = (await tx.query("select max(id)::int as id from einreichungen where aufgabe_id = $1", [a.id])).rows[0].id;
    if (letzte !== einreichungId) throw new Abgelehnt("this submission was replaced by a newer one");
    const nein = await darfPruefen(tx, wer, a, e.autor);
    if (nein) throw new Abgelehnt(nein);
    if (e.modell && modellFamilie(modell) === modellFamilie(e.modell))
      throw new Abgelehnt(`the submission was written by ${e.modell}; a peer review needs a model of another family (yours: ${modellFamilie(modell)})`);
    const bilder = Number((await tx.query("select count(*) from einreichung_bilder where einreichung_id = $1", [einreichungId])).rows[0].count);
    // Ändert der Patch Referenzbilder, ist die Änderung sichtbar, auch bevor die Bilder da sind (die Schubkarre, 9.11.)
    if ((bilder || aendertSzenenbilder(e.patch)) && urteil === "zustimmen") {
      if (!siehtBilder(modell)) throw new Abgelehnt("this submission changes what you see (scene images); only a model that reads images may approve it: request changes or leave it to another reviewer");
      if (!bilder) throw new Abgelehnt("this submission changes what you see, but its scene images are not there yet; approve in a few minutes, after looking at them");
      if (!zusatz.bilderGesehen) throw new Abgelehnt("look at the scene images first (old, new, diff), then approve with images_seen: true");
    }
    const punkte: string[] = e.pruefpunkte ?? [];
    if (punkte.length && (antworten.length !== punkte.length || antworten.some((x) => x.length < 10)))
      throw new Abgelehnt(`answer each of the ${punkte.length} review points of the lead, in order, with what you found (answers, at least 10 characters each)`);
    try {
      await tx.query("insert into fremdpruefungen(einreichung_id, agent_id, mensch_id, urteil, begruendung, modell, antworten, bilder_gesehen) values ($1,$2,$3,$4,$5,$6,$7,$8)",
        [einreichungId, wer.agentId, wer.menschId, urteil, begruendung.trim().slice(0, 4000), modell.trim().slice(0, 120), punkte.length ? antworten.map((x) => x.slice(0, 1000)) : null,
         Boolean(bilder && zusatz.bilderGesehen)]);
    } catch (err) {
      if ((err as { code?: string }).code === "23505") throw new Abgelehnt("you already reviewed this submission");
      throw err;
    }
    await vertrauenAktualisieren(tx, wer.menschId, "peer review");
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: `agent:${wer.agentId}`, art: "fremdpruefung", bezug: `aufgabe:${a.id}`,
      daten: { einreichung: einreichungId, urteil, modell: modell.trim().slice(0, 120), anderes_modell: e.modell ? modellSchluessel(modell) !== modellSchluessel(e.modell) : null,
        ...(bilder ? { bilder_gesehen: Boolean(zusatz.bilderGesehen) } : {}), ...(punkte.length ? { pruefpunkte: punkte.length } : {}) } });
    const alle = (await tx.query(
      `select f.urteil, f.begruendung, f.agent_id, m.login,
         exists (select 1 from einreichungen e2 join agenten g2 on g2.id = e2.agent_id join aufgaben a2 on a2.id = e2.aufgabe_id
                 where g2.mensch_id = f.mensch_id and a2.projekt_id = $2
                   and exists (select 1 from ereignisse x where x.art = 'gemerged' and x.daten->>'einreichung' = e2.id::text)) as im_projekt
       from fremdpruefungen f join menschen m on m.id = f.mensch_id where f.einreichung_id = $1 order by f.id`, [einreichungId, a.projekt_id])).rows;
    const zu = alle.filter((f) => f.urteil === "zustimmen");
    const vomLead = Boolean((await tx.query(`select ${VOM_LEAD("e", "$2")} as v from einreichungen e where e.id = $1`, [einreichungId, a.projekt_id])).rows[0].v);
    const leadEigen = vomLead && LEAD_EIGEN.stufen.includes(a.stufe);
    const ab = automatischAb(a.stufe, vomLead);
    // Ohne Lead (Stufe A) zählen nur Prüfer, die in diesem Projekt schon gemergt haben (AUTO_ANNAHME)
    const zaehlen = leadEigen ? zu : zu.filter((f) => f.im_projekt);
    const automatisch = ab !== null && zaehlen.length >= ab && !alle.some((f) => f.urteil === "aenderungen");
    const meine = alle.filter((f) => Number(f.agent_id) === wer.agentId).at(-1);
    const hinweis = !leadEigen && ab !== null && urteil === "zustimmen" && meine && !meine.im_projekt
      ? "your approval is recorded and the lead sees it; it counts toward automatic acceptance once you have a merged contribution in this project" : undefined;
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
        : `Automatically accepted after ${zaehlen.length} independent peer reviews (${zaehlen.map((f) => f.login).join(", ")}): `) + zaehlen.map((f) => f.begruendung).join(" | ");
      await tx.query("insert into bewertungen(einreichung_id, agent_id, urteil, begruendung) values ($1,$2,'annehmen',$3)", [einreichungId, wer.agentId, grund.slice(0, 8000)]);
      await wechseln(tx, a, "angenommen", "system", "bewertet", { einreichung: einreichungId, urteil: "annehmen", automatisch: true, fremdpruefer: zaehlen.map((f) => f.login) });
      await variantenSchliessen(tx, a);
    }
    return { einreichung: einreichungId, zustimmungen: zu.length, automatisch_angenommen: automatisch, ...(hinweis ? { hinweis } : {}) };
  });
  // Messung (E-43, noch ohne Folgen): Der Prüfer sieht seinen Stand mit diesem Modell
  return { ...ergebnis, dein_pruefer_status: await prueferStatus(db, wer.menschId, modell) };
}

/**
 * Ein Runner meldet eine Fremdprüfung, die ab dem Sandbox-Schritt scheiterte (E-45, Nachtrag; ab Runner 0.15.1):
 * mit Ursache, wenn das Modell nicht arbeiten konnte, sonst ohne. Zählt nur für den Hinweis an den Betreiber
 * (runnerStoerungen), nicht öffentlich und nicht gegen das Modell. Nur Runner, nur für Einreichungen, die die Person
 * prüfen darf; „gestoppt“ wird nicht festgehalten (angehaltene Runden zählen nirgends).
 */
export async function fremdpruefungGescheitert(db: Db, wer: Wer, einreichungId: number, ursache: AbbruchUrsache | null, grund: string) {
  if (wer.runner !== true) throw new Abgelehnt("only the swarvex runner reports failed peer reviews");
  const e = (await db.query("select e.id, e.aufgabe_id, ag.mensch_id::int as autor, a.projekt_id, a.raum from einreichungen e join agenten ag on ag.id = e.agent_id join aufgaben a on a.id = e.aufgabe_id where e.id = $1", [einreichungId])).rows[0];
  if (!e) throw new Abgelehnt("unknown submission");
  const nein = await darfPruefen(db, wer, e, e.autor);
  if (nein) throw new Abgelehnt(nein);
  if (ursache === "gestoppt") return { recorded: false };
  await db.query("insert into fremdpruefung_abbrueche(einreichung_id, agent_id, ursache, grund) values ($1,$2,$3,$4)",
    [einreichungId, wer.agentId, ursache, grund.trim().slice(0, 500) || "-"]);
  return { recorded: true };
}

/** Fremdprüfungen zu einer Einreichung, für Lead und Bereichs-Lead in pending_submissions. */
export async function fremdpruefungenZu(db: Db, einreichungId: number) {
  return (await db.query(
    "select f.urteil, f.begruendung, m.login as von, f.erstellt from fremdpruefungen f join menschen m on m.id = f.mensch_id where f.einreichung_id = $1 order by f.id", [einreichungId])).rows;
}

/**
 * Prüfpunkte des Leads (neVs 10.11.2026): Zu einer Einreichung, die in die Fremdbewertung geht (vor allem eigene), gibt der
 * Lead bis zu fünf konkrete Fragen mit, damit auch ein schwächeres Prüfmodell das Entscheidende prüft. Fragen, kein Urteil:
 * „sieht gut aus“ würde ein Prüfmodell nur abnicken. Nur solange noch keine Fremdbewertung vorliegt; öffentlich protokolliert.
 */
export async function pruefpunkteSetzen(db: Db, wer: Wer, einreichungId: number, punkteRoh: string[]) {
  const punkte = [...new Set(punkteRoh.map((x) => String(x).trim().replace(/\s+/g, " ")))].filter(Boolean);
  if (punkte.length < 1 || punkte.length > 5) throw new Abgelehnt("points: 1 to 5 review points");
  if (punkte.some((x) => x.length < 15 || x.length > 300 || !x.endsWith("?")))
    throw new Abgelehnt("points: each a concrete question the reviewer can check (15 to 300 characters, ending with ?), not a verdict");
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select e.id, e.aufgabe_id from einreichungen e where e.id = $1", [einreichungId])).rows[0];
    if (!e) throw new Abgelehnt("unknown submission");
    const a = (await tx.query("select * from aufgaben where id = $1 for update", [e.aufgabe_id])).rows[0];
    await mussLeadOderOfficerSein(tx, wer.menschId, Number(a.projekt_id), a);
    if (!["eingereicht", "pruefung", "geprueft_ok"].includes(a.zustand)) throw new Abgelehnt("review points only while the submission waits for its check or its review");
    const letzte = (await tx.query("select max(id)::int as id from einreichungen where aufgabe_id = $1", [a.id])).rows[0].id;
    if (letzte !== einreichungId) throw new Abgelehnt("this submission was replaced by a newer one");
    if (Number((await tx.query("select count(*) from fremdpruefungen where einreichung_id = $1", [einreichungId])).rows[0].count))
      throw new Abgelehnt("peer reviews of this submission have started; review points can no longer change");
    await tx.query("update einreichungen set pruefpunkte = $2 where id = $1", [einreichungId, punkte]);
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: `agent:${wer.agentId}`, art: "pruefpunkte_gesetzt", bezug: `aufgabe:${a.id}`,
      daten: { einreichung: einreichungId, punkte } });
    return { einreichung: einreichungId, pruefpunkte: punkte };
  });
}

/** Dürfte diese Person die Einreichung fremdbewerten? Für den Abruf der Szenenbilder (main.ts). */
export async function fremdpruefungErlaubt(db: Db, wer: Wer, einreichungId: number) {
  const z = (await db.query(
    `select a.projekt_id, a.raum, a.zustand, ag.mensch_id::int as autor from einreichungen e join aufgaben a on a.id = e.aufgabe_id join agenten ag on ag.id = e.agent_id
     where e.id = $1`, [einreichungId])).rows[0];
  if (!z) throw new Abgelehnt(`no submission #${einreichungId}`);
  if (z.zustand !== "geprueft_ok") throw new Abgelehnt("images for peer reviewers only while the submission waits for review");
  const nein = await darfPruefen(db, wer, z, z.autor);
  if (nein) throw new Abgelehnt(nein);
}
