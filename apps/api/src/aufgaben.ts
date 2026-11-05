// Koordinations-Kern: Aufgaben-Zustandsmaschine, Leases, Einreichungen, Bewertungen.
// Jede Zustandsänderung läuft in einer Transaktion und schreibt ihr Ereignis in dieselbe
// Transaktion – entweder beides oder nichts (docs/concept.md §3).
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { tmpdir } from "node:os";
import { matchesGlob } from "node:path";
import { transaktion, type Db, type Tx } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { aufgabePruefen, raeumeVon, trennwand, type Raum } from "./cleanroom.ts";
import { WERTE } from "./aussen.ts";
import { nachEinreichen, proprietaer } from "./gegenpruefung.ts";
import { einreichungsVorschauUrl } from "./konfig.ts";
import { vertrauenAktualisieren } from "./vertrauen.ts";

const en = (zustand: string) => WERTE.zustand[zustand] ?? zustand;

export const ZUSTAENDE = ["offen", "reserviert", "in_arbeit", "eingereicht", "pruefung", "geprueft_ok", "geprueft_fehler", "angenommen", "gemerged", "geschlossen"] as const;
export type Zustand = (typeof ZUSTAENDE)[number];

/** Erlaubte Übergänge; alles andere ist ein Fehler. */
export const UEBERGAENGE: Record<Zustand, Zustand[]> = {
  offen: ["reserviert", "geschlossen", "gemerged"],   // gemerged direkt: eigene D-Arbeit des Leads (leadErledigt)
  reserviert: ["in_arbeit", "eingereicht", "offen", "geschlossen"],
  in_arbeit: ["eingereicht", "offen", "geschlossen"],
  eingereicht: ["pruefung", "geschlossen"],
  pruefung: ["geprueft_ok", "geprueft_fehler"],
  geprueft_ok: ["angenommen", "offen", "geschlossen"],
  geprueft_fehler: ["offen", "geschlossen"],
  angenommen: ["gemerged", "offen"],   // offen: Merge-Konflikt, zurück in den Pool mit neuem Basis-Commit
  gemerged: [],
  geschlossen: [],
};

export const STUFEN_GEWICHT: Record<string, number> = { A: 1, B: 2, C: 3, D: 5 };
export const GRENZEN = { zeilen: 500, dateien: 20 };

export class Abgelehnt extends Error {}

/** runner: false = direkt angebundener Agent ohne Sandbox (Direkt-Modus, nur Aufgaben ohne Code-Ausführung). */
export type Wer = { menschId: number; agentId: number; runner?: boolean };

/** Fähigkeiten, die nichts ausführen: Text, Gestaltung, Assets, Review, Daten. Alles andere (code.*, test.*,
 *  simulation, Unbekanntes) läuft nur im Runner mit Sandbox. */
const OHNE_AUSFUEHRUNG = [/^text\./, /^design\./, /^asset\./, /^review$/, /^daten$/];
export const ohneAusfuehrung = (faehigkeiten: string[]) => faehigkeiten.length > 0 && faehigkeiten.every((f) => OHNE_AUSFUEHRUNG.some((r) => r.test(f)));
export const NUR_RUNNER = "this task runs code; work on it with the swarvex runner, which keeps it in a sandbox (https://swarvex.ai/en/docs/runner)";
const akteur = (w: Wer) => `agent:${w.agentId}`;

async function aufgabeSperren(tx: Tx, id: number) {
  const r = await tx.query("select * from aufgaben where id = $1 for update", [id]);
  if (!r.rows[0]) throw new Abgelehnt(`task ${id} does not exist`);
  return r.rows[0];
}

export async function wechseln(tx: Tx, aufgabe: any, nach: Zustand, wer: string, art: string, daten: Record<string, unknown> = {}) {
  const von = aufgabe.zustand as Zustand;
  if (!UEBERGAENGE[von].includes(nach)) throw new Abgelehnt(`transition ${en(von)} → ${en(nach)} is not allowed`);
  await tx.query("update aufgaben set zustand = $2 where id = $1", [aufgabe.id, nach]);
  await protokollieren(tx, { projekt_id: Number(aufgabe.projekt_id), akteur: wer, art, bezug: `aufgabe:${aufgabe.id}`, daten: { von, nach, ...daten } });
}

export async function rollenVon(db: Db | Tx, menschId: number, projektId: number): Promise<string[]> {
  const r = await db.query("select rolle from projekt_rollen where mensch_id = $1 and projekt_id = $2", [menschId, projektId]);
  return r.rows.map((z) => z.rolle);
}

async function mussLeadSein(tx: Tx, menschId: number, projektId: number) {
  const rollen = await rollenVon(tx, menschId, projektId);
  if (!rollen.includes("lead") && !rollen.includes("visionaer")) throw new Abgelehnt("only the project's lead may do this");
}

// Bereichs-Leads (lead/roles/README.md): führen einen Bereich eines Projekts. Sie dürfen Aufgaben der Stufen A und B
// anlegen, schließen und bewerten, deren Pfade und Fähigkeiten ganz in ihrem Bereich liegen. Alles andere bleibt beim Lead.
export type Officer = { id: number; bereich: string; pfade: string[]; faehigkeiten: string[]; raum: Raum | null };
export const OFFICER_STUFEN = ["A", "B"];
const NAME = /^[a-z][a-z0-9-]{1,30}$/;

export async function officersVon(db: Db | Tx, menschId: number, projektId?: number): Promise<Officer[]> {
  const r = await db.query(
    "select id, bereich, pfade, faehigkeiten, raum from officers where mensch_id = $1 and beendet is null and ($2::bigint is null or projekt_id = $2)", [menschId, projektId ?? null]);
  return r.rows.map((o) => ({ ...o, id: Number(o.id) }));
}

/** Pfad (Glob) einer Aufgabe liegt in einem Bereichspfad: gleich, oder unterhalb eines "<ordner>/**". */
export const pfadImBereich = (p: string, bereich: string[]) =>
  !p.includes("..") && !p.startsWith("/") && bereich.some((b) => b === p || (b.endsWith("/**") && p.startsWith(b.slice(0, -2))));
/** Fähigkeit liegt im Bereich: gleich, oder Familie "asset.*". */
const faehigkeitImBereich = (f: string, bereich: string[]) => bereich.some((b) => b === f || (b.endsWith(".*") && f.startsWith(b.slice(0, -1))));

export const imBereich = (o: Officer, a: { stufe: string; pfade: string[]; faehigkeiten: string[]; raum?: string | null }) =>
  OFFICER_STUFEN.includes(a.stufe) && a.pfade.length > 0 && a.faehigkeiten.length > 0 && (o.raum ?? null) === (a.raum ?? null) &&
  a.pfade.every((p) => pfadImBereich(p, o.pfade)) && a.faehigkeiten.every((f) => faehigkeitImBereich(f, o.faehigkeiten));

/** Lead oder Visionär darf alles; ein Bereichs-Lead nur Aufgaben in seinem Bereich. Liefert den Bereichs-Lead, falls er es ist. */
async function mussLeadOderOfficerSein(tx: Tx, menschId: number, projektId: number, a: { stufe: string; pfade: string[]; faehigkeiten: string[]; raum?: string | null }): Promise<Officer | null> {
  const rollen = await rollenVon(tx, menschId, projektId);
  if (rollen.includes("lead") || rollen.includes("visionaer")) return null;
  const o = (await officersVon(tx, menschId, projektId)).find((x) => imBereich(x, a));
  if (!o) throw new Abgelehnt("only the project's lead may do this, or an area lead for this area (levels A and B, paths and capabilities inside the area, and in clean-room projects the area lead's room)");
  return o;
}

export async function projektPerSlug(db: Db | Tx, slug: string) {
  const r = await db.query("select * from projekte where slug = $1", [slug]);
  if (!r.rows[0]) throw new Abgelehnt(`project ${slug} does not exist`);
  if (r.rows[0].gesperrt) throw new Abgelehnt(`project ${slug} is blocked`);
  return r.rows[0];
}

export type NeueAufgabe = {
  titel: string; beschreibung: string; stufe: "A" | "B" | "C" | "D"; faehigkeiten: string[]; pfade: string[];
  akzeptanz: string[]; basis_commit?: string; lease_minuten?: number; min_vertrauen?: number; meilenstein?: string;
  /** Unbekannter Meilenstein-Titel wird nur mit neuer_meilenstein angelegt, sonst abgelehnt (Tippfehler wie "1" statt "M1"). */
  neuer_meilenstein?: boolean;
  /** Nur in Clean-Room-Projekten: analyse, pruefung oder bau (cleanroom.ts). */
  raum?: Raum;
  /** Vorgänger im selben Projekt: die Aufgabe wird erst frei, wenn alle angenommen sind (kein Nachlegen, kein Warten). */
  abhaengig_von?: number[];
  /** Gruppen von Vorgängern, z. B. Varianten-Paare: aus jeder Gruppe muss mindestens eine angenommen sein. */
  abhaengig_eins_von?: number[][];
  /** Rolle aus dem Rollenkatalog (lead/roles), z. B. audio, ux, art; bei Bereichs-Leads immer ihr Bereich. */
  rolle?: string;
};

export async function aufgabeAnlegen(db: Db, wer: Wer, projektSlug: string, a: NeueAufgabe) {
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, projektSlug);
    const officer = await mussLeadOderOfficerSein(tx, wer.menschId, Number(p.id), a);
    if (a.rolle !== undefined && !NAME.test(a.rolle)) throw new Abgelehnt("role: lowercase letters, digits and dashes, e.g. audio or level-design");
    if (officer && a.rolle && a.rolle !== officer.bereich) throw new Abgelehnt(`as area lead for ${officer.bereich} you create tasks for that role only`);
    if (officer && a.neuer_meilenstein) throw new Abgelehnt("only the lead creates milestones");
    const rolle = officer?.bereich ?? a.rolle ?? null;
    const fehler = aufgabePruefen(p.verfahren, a.raum, a.pfade);
    if (fehler.length) throw new Abgelehnt(fehler.join("; "));
    let meilensteinId: number | null = null;
    if (a.meilenstein) {
      const m = await tx.query("select id from meilensteine where projekt_id = $1 and titel = $2", [p.id, a.meilenstein]);
      meilensteinId = m.rows[0]?.id ?? null;
      if (meilensteinId === null) {
        if (!a.neuer_meilenstein) {
          const alle = (await tx.query("select titel from meilensteine where projekt_id = $1 order by id", [p.id])).rows.map((r) => r.titel);
          throw new Abgelehnt(`unknown milestone "${a.meilenstein}" (existing: ${alle.length ? alle.join(", ") : "none"}); set new_milestone to create it`);
        }
        meilensteinId = (await tx.query("insert into meilensteine(projekt_id, titel) values ($1,$2) returning id", [p.id, a.meilenstein])).rows[0].id;
      }
    }
    // Vorgänger: nur Aufgaben desselben Projekts, höchstens 20; neue Aufgaben können so keinen Kreis bilden
    const vorgaenger = [...new Set((a.abhaengig_von ?? []).map(Number))];
    const gruppen = (a.abhaengig_eins_von ?? []).map((g) => [...new Set(g.map(Number))]);
    if (vorgaenger.length > 20 || vorgaenger.some((v) => !Number.isInteger(v))) throw new Abgelehnt("depends_on: at most 20 task numbers");
    if (gruppen.length > 10 || gruppen.some((g) => g.length < 2 || g.length > 10 || g.some((v) => !Number.isInteger(v))))
      throw new Abgelehnt("depends_on_any: at most 10 groups of 2 to 10 task numbers each");
    const alle = [...new Set([...vorgaenger, ...gruppen.flat()])];
    if (alle.length) {
      const da = (await tx.query("select id from aufgaben where projekt_id = $1 and id = any($2::bigint[]) and zustand <> 'geschlossen'", [p.id, alle])).rows.map((r) => Number(r.id));
      const fehlt = alle.filter((v) => !da.includes(v));
      if (fehlt.length) throw new Abgelehnt(`depends_on: no open or finished task ${fehlt.map((x) => `#${x}`).join(", ")} in this project`);
    }
    const r = await tx.query(
      `insert into aufgaben(projekt_id, meilenstein_id, titel, beschreibung, stufe, faehigkeiten, pfade, akzeptanz, basis_commit, lease_minuten, min_vertrauen, raum, rolle, abhaengig_von, abhaengig_eins_von)
       values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) returning *`,
      [p.id, meilensteinId, a.titel, a.beschreibung, a.stufe, a.faehigkeiten, a.pfade, a.akzeptanz, a.basis_commit ?? null, a.lease_minuten ?? 60, a.min_vertrauen ?? 0, a.raum ?? null, rolle, vorgaenger, JSON.stringify(gruppen)],
    );
    const aufgabe = r.rows[0];
    // Die erste Aufgabe macht aus dem Entwurf ein öffentlich gelistetes Projekt
    await tx.query("update projekte set entwurf = false where id = $1 and entwurf", [p.id]);
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: akteur(wer), art: "aufgabe_angelegt", bezug: `aufgabe:${aufgabe.id}`, daten: { titel: a.titel, stufe: a.stufe, faehigkeiten: a.faehigkeiten, pfade: a.pfade, ...(a.raum ? { raum: a.raum } : {}), ...(rolle ? { rolle } : {}), ...(officer ? { officer: officer.bereich } : {}) } });
    return aufgabe;
  });
}

/** Offene Aufgaben, die der Mensch nach Vertrauensstufe bearbeiten darf (optional nach Fähigkeiten gefiltert). */
export async function aufgabenPassend(db: Db, wer: Wer, projektSlug: string, faehigkeiten?: string[]) {
  const p = await projektPerSlug(db, projektSlug);
  // Ohne verbundenes Repo gibt es keinen Snapshot: dem Runner nichts anbieten, statt ihn reservieren und scheitern zu lassen.
  if (wer.runner === true && !p.repo_git) return [];
  const m = (await db.query("select vertrauensstufe from menschen where id = $1", [wer.menschId])).rows[0];
  const r = await db.query(
    `select id, titel, stufe, faehigkeiten, pfade, lease_minuten, raum from aufgaben a where projekt_id = $1 and zustand = 'offen' and min_vertrauen <= $2
       and not aufgabe_blockiert(a.abhaengig_von, a.abhaengig_eins_von)
       -- zweimal in 24 Stunden von diesem Agenten abgebrochen: ihm vorerst nicht mehr anbieten, statt ihn im Kreis laufen zu lassen
       and (select count(*) from ereignisse ev where ev.bezug = 'aufgabe:' || a.id and ev.art = 'abgebrochen' and ev.akteur = $3
              and ev.zeit > now() - interval '24 hours') < 2
     order by id`,
    [p.id, m.vertrauensstufe, akteur(wer)],
  );
  // Clean Room: Aufgaben auf der anderen Seite der Trennwand gar nicht erst anbieten.
  const bisher = p.verfahren === "clean_room" ? await raeumeVon(db, wer.menschId, p.id) : [];
  const erlaubt = r.rows.filter((a) => (!a.raum || !trennwand(bisher, a.raum)) && (wer.runner !== false || ohneAusfuehrung(a.faehigkeiten)));
  const passend = faehigkeiten?.length ? erlaubt.filter((a) => a.faehigkeiten.every((f: string) => faehigkeiten.includes(f))) : erlaubt;
  // Kritischer Pfad zuerst: was die meisten Folgeaufgaben (auch über mehrere Stufen) freischaltet, wird zuerst angeboten
  const folge = await folgeaufgaben(db, Number(p.id));
  return passend.map((a) => ({ ...a, folgeaufgaben: folge.get(Number(a.id)) ?? 0 }))
    .sort((x, y) => y.folgeaufgaben - x.folgeaufgaben || Number(x.id) - Number(y.id));
}

class TrennwandVerletzt extends Abgelehnt {
  projektId: number;
  raum: string;
  constructor(grund: string, projektId: number, raum: string) {
    super(grund);
    this.projektId = projektId;
    this.raum = raum;
  }
}

export async function reservieren(db: Db, wer: Wer, aufgabeId: number) {
  try {
    return await reservierenIntern(db, wer, aufgabeId);
  } catch (e) {
    // Der Versuch bleibt im Protokoll (eigene Transaktion, weil die Reservierung zurückgerollt wurde).
    if (e instanceof TrennwandVerletzt) {
      await transaktion(db, (tx) => protokollieren(tx, { projekt_id: e.projektId, akteur: akteur(wer), art: "clean_room_trennung", bezug: `aufgabe:${aufgabeId}`, daten: { raum: e.raum, mensch: wer.menschId, verweigert: true } }));
    }
    throw e;
  }
}

/** Je Aufgabe: wie viele noch nicht erledigte Aufgaben direkt oder über weitere Stufen auf sie warten. */
export async function folgeaufgaben(db: Db | Tx, projektId: number): Promise<Map<number, number>> {
  const r = await db.query(
    `with recursive kante(vor, nach) as (
       select unnest(a.abhaengig_von), a.id from aufgaben a where a.projekt_id = $1 and a.zustand not in ('angenommen', 'gemerged', 'geschlossen')
       union
       select x::bigint, a.id from aufgaben a cross join lateral jsonb_array_elements(a.abhaengig_eins_von) g cross join lateral jsonb_array_elements_text(g) x
       where a.projekt_id = $1 and a.zustand not in ('angenommen', 'gemerged', 'geschlossen')),
     erreicht(start, ziel) as (
       select vor, nach from kante
       union
       select e.start, k.nach from erreicht e join kante k on k.vor = e.ziel)
     select start::int as id, count(distinct ziel)::int as n from erreicht group by start`, [projektId]);
  return new Map(r.rows.map((z) => [z.id, z.n]));
}

/** Was einer Aufgabe noch fehlt, als Alternativen: [[12], [45, 46]] heißt „#12 und (#45 oder #46)“. */
export async function wartetAuf(db: Db | Tx, a: { abhaengig_von?: (number | string)[] | null; abhaengig_eins_von?: unknown }): Promise<number[][]> {
  return (await db.query("select aufgabe_wartet_auf($1::bigint[], $2::jsonb) as w", [(a.abhaengig_von ?? []).map(Number), JSON.stringify(a.abhaengig_eins_von ?? [])])).rows[0].w;
}
export const wartetText = (w: number[][]) => w.map((g) => g.map((x) => `#${x}`).join(" or ")).join(", ");

async function reservierenIntern(db: Db, wer: Wer, aufgabeId: number) {
  return transaktion(db, async (tx) => {
    const a = await aufgabeSperren(tx, aufgabeId);
    const m = (await tx.query("select vertrauensstufe, gesperrt from menschen where id = $1", [wer.menschId])).rows[0];
    if (m.gesperrt) throw new Abgelehnt("account blocked");
    if ((await tx.query("select gesperrt from projekte where id = $1", [a.projekt_id])).rows[0]?.gesperrt) throw new Abgelehnt("project is blocked");
    if (m.vertrauensstufe < a.min_vertrauen) throw new Abgelehnt(`trust level ${a.min_vertrauen} required`);
    if (a.zustand !== "offen") throw new Abgelehnt(`task is ${en(a.zustand)}`);
    const wartet = await wartetAuf(tx, a);
    if (wartet.length) throw new Abgelehnt(`task waits for ${wartetText(wartet)} to be accepted first`);
    if (wer.runner === false && !ohneAusfuehrung(a.faehigkeiten)) throw new Abgelehnt(NUR_RUNNER);
    if (a.raum) {
      const grund = trennwand(await raeumeVon(tx, wer.menschId, Number(a.projekt_id)), a.raum);
      if (grund) throw new TrennwandVerletzt(grund, Number(a.projekt_id), a.raum);
    }
    const rollen = await rollenVon(tx, wer.menschId, a.projekt_id);
    if (rollen.length === 0) {
      await tx.query("insert into projekt_rollen(projekt_id, mensch_id, rolle) values ($1,$2,'mitstreiter')", [a.projekt_id, wer.menschId]);
      await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: `mensch:${wer.menschId}`, art: "mitstreiter_beigetreten", bezug: `mensch:${wer.menschId}`, daten: {} });
    }
    const lease = (await tx.query(
      "insert into leases(aufgabe_id, agent_id, laeuft_ab) values ($1,$2, now() + make_interval(mins => $3)) returning *",
      [a.id, wer.agentId, a.lease_minuten],
    )).rows[0];
    await wechseln(tx, a, "reserviert", akteur(wer), "aufgabe_reserviert", { lease: Number(lease.id), laeuft_ab: lease.laeuft_ab.toISOString(), ...(a.raum ? { raum: a.raum, mensch: wer.menschId } : {}) });
    return { lease_id: Number(lease.id), laeuft_ab: lease.laeuft_ab };
  });
}

async function aktiveLeaseVon(tx: Tx, aufgabeId: number, agentId: number) {
  const l = (await tx.query("select * from leases where aufgabe_id = $1 and beendet is null", [aufgabeId])).rows[0];
  if (!l || Number(l.agent_id) !== agentId) throw new Abgelehnt("this task is not reserved for you");
  if (new Date(l.laeuft_ab) < new Date()) throw new Abgelehnt("reservation expired");
  return l;
}

export async function details(db: Db, wer: Wer, aufgabeId: number) {
  const a = (await db.query("select * from aufgaben where id = $1", [aufgabeId])).rows[0];
  if (!a) throw new Abgelehnt(`task ${aufgabeId} does not exist`);
  const p = (await db.query("select slug, repo_url, engine from projekte where id = $1", [a.projekt_id])).rows[0];
  // Ging der letzte Versuch an der Prüfstraße zurück in den Pool, bekommt der nächste Agent den Bericht: Er soll den
  // Fehler beheben können, statt blind von vorn anzufangen. Nur die Testausgabe, nicht der frühere Patch.
  const pr = (await db.query(
    `select p.ergebnis, p.bericht, p.erstellt from pruefungen p join einreichungen e on e.id = p.einreichung_id
     where e.aufgabe_id = $1 and p.quelle = 'projekt' order by p.id desc limit 1`, [aufgabeId])).rows[0];
  const wieder = ["offen", "reserviert", "in_arbeit"].includes(a.zustand);
  const letzte_pruefung = pr?.ergebnis === "fehler" && wieder
    ? { ergebnis: pr.ergebnis, zeit: pr.erstellt, bericht: String(pr.bericht).slice(-LETZTER_BERICHT) } : null;
  // Ebenso die Bewertung: Schickte der Lead die letzte Einreichung mit Nacharbeit (oder Ablehnung) zurück, bekommt der
  // nächste Versuch seine Begründung, damit er genau das verbessert.
  const bw = (await db.query(
    `select b.urteil, b.begruendung, b.erstellt from bewertungen b where b.einreichung_id = (select max(id) from einreichungen where aufgabe_id = $1)
     order by b.id desc limit 1`, [aufgabeId])).rows[0];
  const letzte_bewertung = bw && bw.urteil !== "annehmen" && wieder ? { urteil: bw.urteil, begruendung: bw.begruendung, zeit: bw.erstellt } : null;
  return { ...a, projekt: p.slug, repo_url: p.repo_url, engine: p.engine, grenzen: GRENZEN, letzte_pruefung, letzte_bewertung };
}

/** So viel vom letzten Prüfbericht geht an den nächsten Versuch (das Ende: dort stehen die roten Tests). */
export const LETZTER_BERICHT = 6000;

export async function fortschritt(db: Db, wer: Wer, aufgabeId: number, notiz: string, verbrauch: Record<string, number>,
  womit: { agent_programm?: string; modell?: string } = {}) {
  return transaktion(db, async (tx) => {
    const a = await aufgabeSperren(tx, aufgabeId);
    const l = await aktiveLeaseVon(tx, aufgabeId, wer.agentId);
    if (womit.agent_programm || womit.modell)
      await tx.query("update leases set agent_programm = coalesce($2, agent_programm), modell = coalesce($3, modell) where id = $1",
        [l.id, womit.agent_programm?.slice(0, 60) || null, womit.modell?.slice(0, 120) || null]);
    const neu = (await tx.query("update leases set laeuft_ab = now() + make_interval(mins => $2) where id = $1 returning laeuft_ab", [l.id, a.lease_minuten])).rows[0];
    if (a.zustand === "reserviert") await wechseln(tx, a, "in_arbeit", akteur(wer), "arbeit_begonnen", { notiz });
    else await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: akteur(wer), art: "fortschritt", bezug: `aufgabe:${a.id}`, daten: { notiz, verbrauch } });
    return { laeuft_ab: neu.laeuft_ab };
  });
}

/** Git-Schreibweise eines Pfads lesen: in Anführungszeichen mit C-Escapes ("a\\tb\\303\\244"), sonst wörtlich. */
export function gitPfadLesen(roh: string): { pfad: string; rest: string } | null {
  if (!roh.startsWith('"')) return null;
  const bytes: number[] = [];
  const ESC: Record<string, number> = { a: 7, b: 8, t: 9, n: 10, v: 11, f: 12, r: 13, '"': 34, "\\": 92 };
  for (let i = 1; i < roh.length; i++) {
    const z = roh[i];
    if (z === '"') return { pfad: Buffer.from(bytes).toString("utf8"), rest: roh.slice(i + 1) };
    if (z !== "\\") { bytes.push(...Buffer.from(z, "utf8")); continue; }
    const n = roh[++i];
    if (n === undefined) return null;
    if (/[0-7]/.test(n)) { bytes.push(parseInt(roh.slice(i, i + 3), 8)); i += 2; continue; }
    if (!(n in ESC)) return null;
    bytes.push(ESC[n]);
  }
  return null;
}

/** Ein Name aus einer Kopfzeile: in Anführungszeichen oder wörtlich bis zum Tab (Zeitstempel klassischer Diffs). */
function name(roh: string): string {
  return gitPfadLesen(roh)?.pfad ?? roh.split("\t")[0];
}

/** Erste Pfadkomponente weg, wie git apply -p1 ("a/x", "b/x", aber auch "irgendwas/x"). */
const ohnePraefix = (p: string) => p.replace(/^[^/]*\//, "");

/**
 * Pfade aus dem Text eines Patches: Kopfzeilen diff --git, ---/+++ (auch klassische Diffs ohne a/ b/),
 * rename/copy from/to, Namen in Anführungszeichen. Lieber ein Pfad zu viel als einer zu wenig:
 * Die Liste dient der Prüfung, und git apply darf nichts anfassen, was hier fehlt (siehe gitPfade).
 */
export function geaendertePfade(patch: string): string[] {
  const pfade = new Set<string>();
  const zeilen = patch.split("\n").map((z) => z.replace(/\r$/, ""));
  const kopfName = (roh: string) => { const n = name(roh).trimEnd(); if (n !== "/dev/null") pfade.add(ohnePraefix(n)); };
  zeilen.forEach((zeile, i) => {
    const umb = /^(?:rename|copy) (?:from|to) (.+)$/.exec(zeile);
    if (umb) { pfade.add(name(umb[1])); return; }
    // ---/+++ nur als Paar (wie git apply); so zählt eine entfernte Kommentarzeile "-- x" nicht als Dateiname
    if (zeile.startsWith("--- ") && zeilen[i + 1]?.startsWith("+++ ")) { kopfName(zeile.slice(4)); kopfName(zeilen[i + 1].slice(4)); return; }
    const kopf = /^diff --git (.+)$/.exec(zeile);
    if (!kopf) return;
    const rest = kopf[1];
    const erster = gitPfadLesen(rest);
    if (erster) {
      pfade.add(ohnePraefix(erster.pfad));
      const zweiter = erster.rest.trimStart();
      pfade.add(ohnePraefix(gitPfadLesen(zweiter)?.pfad ?? zweiter));
      return;
    }
    // Ohne Anführungszeichen ist die Trennung bei Leerzeichen im Namen mehrdeutig. git nimmt den Namen aus dieser
    // Zeile nur, wenn beide Hälften gleich sind (a/X b/X); sonst stehen die Namen in ---/+++ oder rename/copy.
    for (let j = rest.indexOf(" "); j > 0; j = rest.indexOf(" ", j + 1)) {
      const links = ohnePraefix(rest.slice(0, j)), rechts = ohnePraefix(gitPfadLesen(rest.slice(j + 1))?.pfad ?? rest.slice(j + 1));
      if (links && links === rechts) { pfade.add(links); break; }
    }
  });
  return [...pfade].filter(Boolean);
}

/**
 * Pfade, die git apply selbst aus dem Patch liest (git apply --numstat -z, wendet nichts an). Das ist maßgeblich:
 * genau diese Dateien würde git apply in Prüfstraße und Merge-Bot anlegen, ändern oder löschen.
 * null, wenn git den Patch nicht lesen kann.
 */
export function gitPfade(patch: string): string[] | null {
  let aus: string;
  try {
    aus = execFileSync("git", ["apply", "--numstat", "-z", "-"], {
      input: patch, cwd: tmpdir(), encoding: "utf8", timeout: 15_000, maxBuffer: 16 << 20,
      env: { ...process.env, GIT_CEILING_DIRECTORIES: tmpdir(), GIT_CONFIG_NOSYSTEM: "1", HOME: tmpdir() }, stdio: ["pipe", "pipe", "ignore"],
    });
  } catch {
    return null;
  }
  const pfade: string[] = [];
  const teile = aus.split("\0");
  for (let i = 0; i < teile.length; i++) {
    const m = /^(?:\d+|-)\t(?:\d+|-)\t([\s\S]*)$/.exec(teile[i]);
    if (!m) continue;
    if (m[1]) pfade.push(m[1]);
    else { pfade.push(teile[i + 1] ?? "", teile[i + 2] ?? ""); i += 2; } // Umbenennung: alter und neuer Name folgen
  }
  return pfade.filter(Boolean);
}

/**
 * Dateien, die die Umgebung, die Projektregeln oder die CI bestimmen. Ein Beitrag darf sie nie ändern,
 * sonst könnte er Befehle auf den Rechnern anderer Mitstreiter einschleusen (gleiche Liste wie
 * runner/umgebung.Geschuetzt). Lead und Visionär ändern sie direkt im Repo.
 */
export const GESCHUETZT = ["swarvex.yaml", ".devcontainer/**", ".devcontainer.json", "AGENTS.md", ".github/workflows/**", ".gitlab-ci.yml", ".forgejo/workflows/**", ".gitea/workflows/**", "LICENSE*", "COPYING*",
  // Anweisungen an Agenten (Skills, Agenten-Konfiguration): sonst könnte ein Beitrag spätere Agenten steuern.
  ".agents/**", ".claude/**", ".codex/**", ".opencode/**", "CLAUDE.md", "opencode.json", "opencode.jsonc"];

/** Geschützt, unabhängig von Groß- und Kleinschreibung (agents.md auf Systemen, die das nicht unterscheiden). */
export const istGeschuetzt = (p: string) => GESCHUETZT.some((g) => matchesGlob(p.toLowerCase(), g.toLowerCase()));

export function patchPruefen(patch: string, erlaubt: string[]): string[] {
  const fehler: string[] = [];
  const ausGit = gitPfade(patch);
  if (ausGit === null) return ["git cannot read the patch (git apply --numstat failed); submit a patch made with git diff"];
  // Beide Lesarten zusammen: was git anfasst, und was im Text steht (z. B. die Quelle einer Umbenennung)
  const pfade = [...new Set([...ausGit, ...geaendertePfade(patch)])];
  if (ausGit.length === 0) fehler.push("patch contains no file changes");
  for (const p of pfade) {
    if (p.includes("..") || p.startsWith("/") || /[\x00-\x1f\x7f]/.test(p) || p.split("/").includes(".git"))
      fehler.push(`invalid path: ${JSON.stringify(p)}`);
    else if (istGeschuetzt(p)) fehler.push(`protected file (environment, project rules or CI): ${p}`);
    else if (!erlaubt.some((g) => matchesGlob(p, g))) fehler.push(`outside the allowed paths: ${p}`);
  }
  const dateien = new Set(ausGit).size;
  if (dateien > GRENZEN.dateien) fehler.push(`too many files: ${dateien} > ${GRENZEN.dateien}`);
  const zeilen = patch.split("\n").filter((z) => /^[+-](?![+-]{2} )/.test(z)).length;
  if (zeilen > GRENZEN.zeilen) fehler.push(`too many changed lines: ${zeilen} > ${GRENZEN.zeilen}`);
  return fehler;
}

export async function einreichen(db: Db, wer: Wer, aufgabeId: number, e: { patch: string; zusammenfassung: string; eigene_tests: string; verbrauch: Record<string, number>; eigenes_ergebnis?: "ok" | "fehler"; agent_programm?: string; modell?: string }) {
  return transaktion(db, async (tx) => {
    const a = await aufgabeSperren(tx, aufgabeId);
    const l = await aktiveLeaseVon(tx, aufgabeId, wer.agentId);
    const fehler = patchPruefen(e.patch, a.pfade);
    if (fehler.length) {
      await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: akteur(wer), art: "einreichung_abgewiesen", bezug: `aufgabe:${a.id}`, daten: { fehler } });
      // Die Abweisung bleibt im Protokoll, auch wenn der Aufrufer einen Fehler sieht.
      return { abgewiesen: fehler };
    }
    const sha = createHash("sha256").update(e.patch).digest("hex");
    const r = (await tx.query(
      "insert into einreichungen(aufgabe_id, lease_id, agent_id, patch, patch_sha256, zusammenfassung, eigene_tests, verbrauch, agent_programm, modell) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) returning id",
      [a.id, l.id, wer.agentId, e.patch, sha, e.zusammenfassung, e.eigene_tests, e.verbrauch, e.agent_programm?.slice(0, 80) ?? null, e.modell?.slice(0, 120) ?? null],
    )).rows[0];
    await tx.query("update leases set beendet = now() where id = $1", [l.id]);
    await wechseln(tx, a, "eingereicht", akteur(wer), "ergebnis_eingereicht", { einreichung: Number(r.id), patch_sha256: sha, pfade: geaendertePfade(e.patch), verbrauch: e.verbrauch });
    // Proprietäre Engine: Runner prüfen statt der Prüfstraße (eigener Lauf, dann Gegenprüfung; E-31).
    const engine = (await tx.query("select engine from projekte where id = $1", [a.projekt_id])).rows[0]?.engine;
    if (proprietaer(engine)) await nachEinreichen(tx, { ...a, zustand: "eingereicht" }, Number(r.id), e.eigenes_ergebnis, e.eigene_tests, wer);
    return { einreichung_id: Number(r.id), patch_sha256: sha };
  });
}

export async function abbrechen(db: Db, wer: Wer, aufgabeId: number, grund: string) {
  return transaktion(db, async (tx) => {
    const a = await aufgabeSperren(tx, aufgabeId);
    const l = await aktiveLeaseVon(tx, aufgabeId, wer.agentId);
    await tx.query("update leases set beendet = now() where id = $1", [l.id]);
    await wechseln(tx, a, "offen", akteur(wer), "abgebrochen", { grund });
  });
}

/** Lead schließt eine offene Aufgabe (überholt, doppelt, verschoben); reservierte Arbeit anderer bleibt unberührt. */
export async function schliessen(db: Db, wer: Wer, aufgabeId: number, grund: string) {
  if (!grund.trim()) throw new Abgelehnt("a reason is required");
  return transaktion(db, async (tx) => {
    const a = await aufgabeSperren(tx, aufgabeId);
    await mussLeadOderOfficerSein(tx, wer.menschId, Number(a.projekt_id), a);
    if (a.zustand !== "offen") throw new Abgelehnt(`only open tasks can be closed (task ${aufgabeId} is ${en(a.zustand)})`);
    await wechseln(tx, a, "geschlossen", akteur(wer), "aufgabe_geschlossen", { grund });
  });
}

/** Läuft periodisch (pg-boss): abgelaufene Reservierungen geben ihre Aufgabe frei. */
export async function leasesAufraeumen(db: Db): Promise<number> {
  return transaktion(db, (tx) => leasesBeenden(tx, { abgelaufen: true }, "reservierung_abgelaufen"));
}

/** Beendet laufende Reservierungen (abgelaufen, Projekt gesperrt, Agent abgemeldet); die Aufgaben gehen zurück in den Pool. */
export async function leasesBeenden(tx: Tx, wo: { abgelaufen: true } | { projekt: number } | { agent: number }, art: string): Promise<number> {
  const [bedingung, wert] = "projekt" in wo ? ["aufgabe_id in (select id from aufgaben where projekt_id = $1)", wo.projekt]
    : "agent" in wo ? ["agent_id = $1", wo.agent] : ["laeuft_ab < now() and $1::int is null", null];
  const leases = (await tx.query(`select * from leases where beendet is null and ${bedingung} for update`, [wert])).rows;
  for (const l of leases) {
    await tx.query("update leases set beendet = now() where id = $1", [l.id]);
    const a = await aufgabeSperren(tx, l.aufgabe_id);
    if (a.zustand === "reserviert" || a.zustand === "in_arbeit") await wechseln(tx, a, "offen", "system", art, { lease: Number(l.id), agent: Number(l.agent_id) });
  }
  return leases.length;
}

/** Prüfstraße beginnt bzw. meldet ein Ergebnis (Signaturprüfung erfolgt vorher, siehe P5). */
export async function pruefungStarten(db: Db, einreichungId: number) {
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select * from einreichungen where id = $1", [einreichungId])).rows[0];
    if (!e) throw new Abgelehnt("unknown submission");
    const a = await aufgabeSperren(tx, e.aufgabe_id);
    await wechseln(tx, a, "pruefung", "pruefstrasse", "pruefung_gestartet", { einreichung: einreichungId });
  });
}

export async function pruefungMelden(db: Db, einreichungId: number, ergebnis: "ok" | "fehler", bericht: string, signatur: string, quelle: "projekt" | "stichprobe" = "projekt", basisRot = false) {
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select * from einreichungen where id = $1", [einreichungId])).rows[0];
    if (!e) throw new Abgelehnt("unknown submission");
    const a = await aufgabeSperren(tx, e.aufgabe_id);
    await tx.query("insert into pruefungen(einreichung_id, quelle, ergebnis, bericht, signatur) values ($1,$2,$3,$4,$5)", [einreichungId, quelle, ergebnis, bericht, signatur]);
    await wechseln(tx, a, ergebnis === "ok" ? "geprueft_ok" : "geprueft_fehler", "pruefstrasse", "pruefung_beendet", { einreichung: einreichungId, ergebnis, quelle });
    // Eine gescheiterte Prüfung gibt die Aufgabe automatisch zurück in den Pool.
    if (ergebnis === "fehler") await wechseln(tx, { ...a, zustand: "geprueft_fehler" }, "offen", "system", "zurueck_in_pool", { einreichung: einreichungId });
    // Gegenprobe der Prüfstraße: Die Tests sind schon ohne den Beitrag rot. Der Beitrag geht in die Bewertung, der Lead
    // bekommt eine Nachricht (höchstens eine je Tag und Projekt, solange das Projekt rot bleibt).
    if (ergebnis === "ok" && basisRot && quelle === "projekt") {
      await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: "pruefstrasse", art: "projekt_tests_rot", bezug: `aufgabe:${a.id}`, daten: { einreichung: einreichungId } });
      const schon = (await tx.query(`select 1 from nachrichten where projekt_id = $1 and von_mensch is null and betreff = $2 and erstellt > now() - interval '1 day'`,
        [a.projekt_id, BETREFF_TESTS_ROT])).rowCount;
      if (!schon) await tx.query(
        `insert into nachrichten(projekt_id, von_mensch, an, art, betreff, inhalt, aufgaben, zustand) values ($1, null, 'lead', 'info', $2, $3, $4, 'erledigt')`,
        [a.projekt_id, BETREFF_TESTS_ROT, `The project's tests fail at base commit ${String(a.basis_commit ?? "").slice(0, 12) || "(unknown)"} without any contribution `
          + `(counter-check on submission ${einreichungId}, task ${a.id}). Contributions are not rejected for it, but nobody can rely on the tests `
          + `until they are green again. The end of the report:\n\n${bericht.slice(-3000)}`, [Number(a.id)]]);
    }
  });
}

export const BETREFF_TESTS_ROT = "Project tests are red without contributions";

/**
 * Betreiber-Ausnahme mit Zustimmung des Visionärs: Eine Einreichung, die nur an den roten Tests des Projekts
 * scheiterte (vor der Gegenprobe), geht doch in die Bewertung. Nur die letzte Einreichung einer offenen Aufgabe,
 * deren letzte Prüfung rot war; öffentlich protokolliert mit Grund. Danach bewertet der Lead wie sonst.
 */
export async function zurBewertungFreigeben(db: Db, einreichungId: number, grund: string) {
  if (grund.trim().length < 20) throw new Abgelehnt("reason: why this submission may be reviewed after all (at least 20 characters)");
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select * from einreichungen where id = $1", [einreichungId])).rows[0];
    if (!e) throw new Abgelehnt("unknown submission");
    const a = await aufgabeSperren(tx, e.aufgabe_id);
    if (a.zustand !== "offen") throw new Abgelehnt(`task ${a.id} is ${en(a.zustand)}; only an open task (nobody working on it) can take a submission back`);
    const letzte = Number((await tx.query("select max(id) from einreichungen where aufgabe_id = $1", [a.id])).rows[0].max);
    if (letzte !== einreichungId) throw new Abgelehnt(`submission ${letzte} is newer for this task`);
    const pr = (await tx.query("select ergebnis from pruefungen where einreichung_id = $1 order by id desc limit 1", [einreichungId])).rows[0];
    if (pr?.ergebnis !== "fehler") throw new Abgelehnt("only a submission whose last check failed");
    await tx.query("update aufgaben set zustand = 'geprueft_ok' where id = $1", [a.id]);
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: "operator", art: "zur_bewertung_freigegeben", bezug: `aufgabe:${a.id}`,
      daten: { von: "offen", nach: "geprueft_ok", einreichung: einreichungId, grund: grund.trim().slice(0, 500) } });
  });
}

export async function bewerten(db: Db, wer: Wer, einreichungId: number, urteil: "annehmen" | "nacharbeit" | "ablehnen", begruendung: string) {
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select * from einreichungen where id = $1", [einreichungId])).rows[0];
    if (!e) throw new Abgelehnt("unknown submission");
    const a = await aufgabeSperren(tx, e.aufgabe_id);
    await mussLeadOderOfficerSein(tx, wer.menschId, Number(a.projekt_id), a);
    const autor = (await tx.query("select mensch_id from agenten where id = $1", [e.agent_id])).rows[0];
    if (Number(autor.mensch_id) === wer.menschId) throw new Abgelehnt("nobody reviews their own submission, not even through a second agent");
    if (a.zustand !== "geprueft_ok") throw new Abgelehnt(`review only after a passing check (task is ${en(a.zustand)})`);
    await tx.query("insert into bewertungen(einreichung_id, agent_id, urteil, begruendung) values ($1,$2,$3,$4)", [einreichungId, wer.agentId, urteil, begruendung]);
    await wechseln(tx, a, urteil === "annehmen" ? "angenommen" : "offen", akteur(wer), "bewertet", { einreichung: einreichungId, urteil, begruendung });
    if (urteil === "annehmen") await variantenSchliessen(tx, a);
    if (urteil === "ablehnen") await vertrauenAktualisieren(tx, Number(autor.mensch_id), "rejected");
  });
}

/**
 * Regression melden (Konzept §6: sofortiger Abstieg): Ein gemergter Beitrag hat etwas kaputt gemacht. Lead oder
 * Bereichs-Lead des Bereichs melden es mit Grund; die Person steigt sofort eine Stufe ab, alles im Protokoll.
 */
export async function regressionMelden(db: Db, wer: Wer, aufgabeId: number, grund: string) {
  if (grund.trim().length < 20) throw new Abgelehnt("reason: what broke and how you know (at least 20 characters)");
  return transaktion(db, async (tx) => {
    const a = await aufgabeSperren(tx, aufgabeId);
    await mussLeadOderOfficerSein(tx, wer.menschId, Number(a.projekt_id), a);
    if (a.zustand !== "gemerged") throw new Abgelehnt("only merged contributions can be reported as a regression");
    const e = (await tx.query(
      `select e.id, ag.mensch_id::int as autor from ereignisse x join einreichungen e on e.id = (x.daten->>'einreichung')::bigint join agenten ag on ag.id = e.agent_id
       where x.art = 'gemerged' and x.bezug = $1 order by x.seq desc limit 1`, [`aufgabe:${aufgabeId}`])).rows[0];
    if (!e) throw new Abgelehnt("this task was done by the lead without a submission; there is nobody to step down");
    if (e.autor === wer.menschId) throw new Abgelehnt("your own contribution: fix it with a new task instead");
    try {
      await tx.query("insert into regressionen(einreichung_id, gemeldet_von, grund) values ($1,$2,$3)", [e.id, wer.menschId, grund.trim().slice(0, 2000)]);
    } catch (err) {
      if ((err as { code?: string }).code === "23505") throw new Abgelehnt("this contribution was already reported");
      throw err;
    }
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: akteur(wer), art: "regression_gemeldet", bezug: `aufgabe:${aufgabeId}`, daten: { einreichung: Number(e.id), grund: grund.trim().slice(0, 500) } });
    const stufe = await vertrauenAktualisieren(tx, e.autor, "regression");
    return { aufgabe: aufgabeId, einreichung: Number(e.id), vertrauen: stufe ? { von: stufe.von, nach: stufe.nach } : null };
  });
}

/** Mehrere Varianten derselben Aufgabe für Geschmacksfragen: parallel von verschiedenen Agenten, eine wird angenommen. */
export async function variantenAnlegen(db: Db, wer: Wer, projektSlug: string, a: NeueAufgabe, anzahl: number) {
  if (!(Number.isInteger(anzahl) && anzahl >= 2 && anzahl <= 4)) throw new Abgelehnt("variants: 2 to 4");
  const ids: number[] = [];
  for (let i = 1; i <= anzahl; i++) {
    const t = await aufgabeAnlegen(db, wer, projektSlug, { ...a, titel: `${a.titel} (variant ${i}/${anzahl})`,
      beschreibung: `Variant ${i} of ${anzahl}: other agents work on the same task in parallel; the visionary or lead picks one. Make yours distinct.\n\n${a.beschreibung}` });
    ids.push(Number(t.id));
  }
  await db.query("update aufgaben set variante_von = $1 where id = any($2::bigint[])", [ids[0], ids]);
  return { varianten: ids, folgeaufgaben_mit: { depends_on_any: [ids] } };
}

/** Ist eine Variante angenommen, schließen sich die anderen, an denen noch niemand arbeitet; begonnene laufen bis zur Bewertung. */
export async function variantenSchliessen(tx: Tx, a: { id: number | string; variante_von: number | string | null }) {
  if (!a.variante_von) return;
  const andere = (await tx.query("select * from aufgaben where variante_von = $1 and id <> $2 and zustand = 'offen' for update", [a.variante_von, a.id])).rows;
  for (const v of andere) await wechseln(tx, v, "geschlossen", "system", "aufgabe_geschlossen", { grund: `variant #${a.id} was accepted` });
}

/** Eigene Arbeit des Leads (Stufe D), ohne Einreichung und ohne Fremdbewertung erledigt: schaltet Folgeaufgaben frei. */
export async function leadErledigt(db: Db, wer: Wer, aufgabeId: number, commit: string, notiz: string) {
  if (!/^[0-9a-f]{7,40}$/.test(commit)) throw new Abgelehnt("commit: the commit hash (7 to 40 hex characters) that contains the work");
  if (notiz.trim().length < 10) throw new Abgelehnt("note: one sentence on what was done");
  return transaktion(db, async (tx) => {
    const a = await aufgabeSperren(tx, aufgabeId);
    if (!(await rollenVon(tx, wer.menschId, a.projekt_id)).includes("lead")) throw new Abgelehnt("only the project's lead marks its own work as done");
    if (a.stufe !== "D") throw new Abgelehnt("only level D tasks (the lead's own work) can be marked done without review");
    if (a.zustand !== "offen") throw new Abgelehnt(`task ${aufgabeId} is ${en(a.zustand)}`);
    if ((await tx.query("select 1 from einreichungen where aufgabe_id = $1", [aufgabeId])).rowCount) throw new Abgelehnt("this task has submissions; review them instead");
    await tx.query("update aufgaben set ohne_fremdbewertung = true where id = $1", [aufgabeId]);
    await wechseln(tx, a, "gemerged", akteur(wer), "vom_lead_erledigt", { commit, notiz: notiz.trim().slice(0, 500) });
    return { aufgabe: aufgabeId, zustand: "gemerged", commit };
  });
}

/** Merge-Konflikt: Die angenommene Einreichung passt nicht mehr auf den neuen Stand; die Aufgabe geht mit neuem Basis-Commit zurück in den Pool. */
export async function konfliktMelden(db: Db, einreichungId: number, kopf: string, bericht: string) {
  if (!/^[0-9a-f]{40}$/.test(kopf)) throw new Abgelehnt("head: a full commit hash");
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select * from einreichungen where id = $1", [einreichungId])).rows[0];
    if (!e) throw new Abgelehnt("unknown submission");
    const a = await aufgabeSperren(tx, e.aufgabe_id);
    if (a.zustand !== "angenommen") throw new Abgelehnt(`task is ${en(a.zustand)}`);
    await tx.query("update aufgaben set basis_commit = $2 where id = $1", [a.id, kopf]);
    await wechseln(tx, a, "offen", "pruefstrasse", "merge_konflikt", { einreichung: einreichungId, neue_basis: kopf, bericht: bericht.slice(0, 2000) });
  });
}

export async function gemergedMelden(db: Db, einreichungId: number, commit: string) {
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select * from einreichungen where id = $1", [einreichungId])).rows[0];
    const a = await aufgabeSperren(tx, e.aufgabe_id);
    await wechseln(tx, a, "gemerged", "pruefstrasse", "gemerged", { einreichung: einreichungId, commit });
    const autor = (await tx.query("select mensch_id from agenten where id = $1", [e.agent_id])).rows[0];
    await vertrauenAktualisieren(tx, Number(autor.mensch_id), "merged");
  });
}

/** Einreichungen, die auf eine Bewertung warten; ein Bereichs-Lead sieht nur die seines Bereichs. */
export async function wartendeEinreichungen(db: Db, wer: Wer, projektSlug: string) {
  const p = await projektPerSlug(db, projektSlug);
  const rollen = await rollenVon(db, wer.menschId, p.id);
  const officers = rollen.includes("lead") || rollen.includes("visionaer") ? null : await officersVon(db, wer.menschId, Number(p.id));
  if (officers && !officers.length) throw new Abgelehnt("only the project's lead or one of its area leads may do this");
  const r = await db.query(
    `select e.id as einreichung_id, a.id as aufgabe_id, a.titel, a.zustand, a.stufe, a.rolle, a.raum, a.pfade, a.faehigkeiten, e.zusammenfassung, e.patch, p.ergebnis as pruefung, p.bericht,
       e.vorschau->>'status' as vorschau_status,
       (select coalesce(jsonb_agg(jsonb_build_object('urteil', f.urteil, 'begruendung', f.begruendung, 'von', m.login) order by f.id), '[]')
          from fremdpruefungen f join menschen m on m.id = f.mensch_id where f.einreichung_id = e.id) as fremdpruefungen
     from einreichungen e join aufgaben a on a.id = e.aufgabe_id
     left join lateral (select ergebnis, bericht from pruefungen where einreichung_id = e.id order by id desc limit 1) p on true
     where a.projekt_id = $1 and a.zustand in ('eingereicht', 'pruefung', 'geprueft_ok') order by e.id`, [p.id]);
  // Vorschau dieser Einreichung zum Ausprobieren vor der Bewertung (pipeline/vorschau.go)
  const zeilen = r.rows.map(({ vorschau_status, ...z }) => ({ ...z, ...(vorschau_status === "ok" && einreichungsVorschauUrl(1) ? { vorschau_url: einreichungsVorschauUrl(Number(z.einreichung_id)) } : {}) }));
  return officers ? zeilen.filter((a) => officers.some((o) => imBereich(o, a))) : zeilen;
}

/** Nur der Visionär ernennt Bereichs-Leads (Bereich, Pfade, Fähigkeiten); öffentlich protokolliert. */
export async function officerErnennen(db: Db, wer: Wer | "betrieb", projektSlug: string, login: string, bereich: string, pfade: string[], faehigkeiten: string[], raum?: Raum, akteurText?: string) {
  if (!NAME.test(bereich)) throw new Abgelehnt("area: lowercase letters, digits and dashes, e.g. audio or ux");
  if (!pfade.length || pfade.some((x) => x.includes("..") || x.startsWith("/") || !x.trim())) throw new Abgelehnt("paths: relative globs inside the repository, e.g. game/audio/**");
  if (pfade.some((x) => x === "**" || x === "*")) throw new Abgelehnt("an area lead's area cannot be the whole repository");
  if (!faehigkeiten.length) throw new Abgelehnt("capabilities: at least one, e.g. asset.audio or text.*");
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, projektSlug);
    if (wer !== "betrieb" && !(await rollenVon(tx, wer.menschId, p.id)).includes("visionaer")) throw new Abgelehnt("only the project's visionary appoints area leads");
    const [anbieter, name] = login.includes(":") ? login.split(":", 2) : [null, login];
    const m = (await tx.query("select id from menschen where login = $1 and ($2::text is null or konto_provider = $2)", [name, anbieter])).rows;
    if (m.length !== 1) throw new Abgelehnt(m.length ? `login ${name} exists at several providers; use provider:login` : `${login} must sign in to swarvex once first`);
    const menschId = Number(m[0].id);
    if (wer !== "betrieb" && menschId === wer.menschId) throw new Abgelehnt("the visionary already has every right; appoint someone else");
    // Bereichs-Lead nur mit verdientem Vertrauen (E-35); Betreiber können das über die Kommandozeile überstimmen
    const stufe = Number((await tx.query("select vertrauensstufe from menschen where id = $1", [menschId])).rows[0].vertrauensstufe);
    if (wer !== "betrieb" && stufe < 2) throw new Abgelehnt(`${name} is at trust level ${stufe}; area leads need level 2 (experienced: 10 merged contributions, 3 of them at level B or higher, 5 peer reviews). Until then, run the area yourself with its role card as a subagent`);
    // Clean Room: ein Bereichs-Lead gehört zu genau einem Raum; die Trennwand gilt für sein Amt wie für eine Reservierung
    if (p.verfahren === "clean_room") {
      if (!raum) throw new Abgelehnt("clean-room project: an area lead needs a room (analysis, review or build)");
      const grund = trennwand(await raeumeVon(tx, menschId, Number(p.id)), raum);
      if (grund) throw new Abgelehnt(grund.replace("you have", `${name} has`).replace("you therefore", "therefore"));
    } else if (raum) throw new Abgelehnt("rooms only exist in clean-room projects");
    await tx.query("update officers set beendet = now() where projekt_id = $1 and bereich = $2 and beendet is null", [p.id, bereich]);
    const id = Number((await tx.query("insert into officers(projekt_id, mensch_id, bereich, pfade, faehigkeiten, raum) values ($1,$2,$3,$4,$5,$6) returning id",
      [p.id, menschId, bereich, pfade, faehigkeiten, raum ?? null])).rows[0].id);
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: akteurText ?? (wer === "betrieb" ? "system" : akteur(wer)), art: "officer_ernannt", bezug: `mensch:${menschId}`,
      daten: { bereich, pfade, faehigkeiten, login: name, ...(raum ? { raum } : {}) } });
    await vertrauenAktualisieren(tx, menschId, "appointed area lead");
    return { officer_id: id, bereich };
  });
}

export async function officerBeenden(db: Db, wer: Wer | "betrieb", projektSlug: string, bereich: string, grund: string) {
  if (!grund.trim()) throw new Abgelehnt("a reason is required");
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, projektSlug);
    if (wer !== "betrieb" && !(await rollenVon(tx, wer.menschId, p.id)).includes("visionaer")) throw new Abgelehnt("only the project's visionary ends an area lead's term");
    const o = (await tx.query("update officers set beendet = now() where projekt_id = $1 and bereich = $2 and beendet is null returning mensch_id", [p.id, bereich])).rows[0];
    if (!o) throw new Abgelehnt(`no active area lead for ${bereich}`);
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: wer === "betrieb" ? "system" : akteur(wer), art: "officer_beendet", bezug: `mensch:${o.mensch_id}`, daten: { bereich, grund } });
    await vertrauenAktualisieren(tx, Number(o.mensch_id), "area lead term ended");
  });
}
