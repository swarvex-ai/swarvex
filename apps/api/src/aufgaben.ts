// Koordinations-Kern: Aufgaben-Zustandsmaschine, Leases, Einreichungen, Bewertungen.
// Jede Zustandsänderung läuft in einer Transaktion und schreibt ihr Ereignis in dieselbe
// Transaktion – entweder beides oder nichts (docs/concept.md §3).
import { createHash } from "node:crypto";
import { matchesGlob } from "node:path";
import { transaktion, type Db, type Tx } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { aufgabePruefen, raeumeVon, trennwand, type Raum } from "./cleanroom.ts";
import { WERTE } from "./aussen.ts";
import { nachEinreichen, proprietaer } from "./gegenpruefung.ts";

const en = (zustand: string) => WERTE.zustand[zustand] ?? zustand;

export const ZUSTAENDE = ["offen", "reserviert", "in_arbeit", "eingereicht", "pruefung", "geprueft_ok", "geprueft_fehler", "angenommen", "gemerged", "geschlossen"] as const;
export type Zustand = (typeof ZUSTAENDE)[number];

/** Erlaubte Übergänge; alles andere ist ein Fehler. */
export const UEBERGAENGE: Record<Zustand, Zustand[]> = {
  offen: ["reserviert", "geschlossen"],
  reserviert: ["in_arbeit", "eingereicht", "offen", "geschlossen"],
  in_arbeit: ["eingereicht", "offen", "geschlossen"],
  eingereicht: ["pruefung", "geschlossen"],
  pruefung: ["geprueft_ok", "geprueft_fehler"],
  geprueft_ok: ["angenommen", "offen", "geschlossen"],
  geprueft_fehler: ["offen", "geschlossen"],
  angenommen: ["gemerged"],
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

async function wechseln(tx: Tx, aufgabe: any, nach: Zustand, wer: string, art: string, daten: Record<string, unknown> = {}) {
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
};

export async function aufgabeAnlegen(db: Db, wer: Wer, projektSlug: string, a: NeueAufgabe) {
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, projektSlug);
    await mussLeadSein(tx, wer.menschId, p.id);
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
    const r = await tx.query(
      `insert into aufgaben(projekt_id, meilenstein_id, titel, beschreibung, stufe, faehigkeiten, pfade, akzeptanz, basis_commit, lease_minuten, min_vertrauen, raum)
       values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) returning *`,
      [p.id, meilensteinId, a.titel, a.beschreibung, a.stufe, a.faehigkeiten, a.pfade, a.akzeptanz, a.basis_commit ?? null, a.lease_minuten ?? 60, a.min_vertrauen ?? 0, a.raum ?? null],
    );
    const aufgabe = r.rows[0];
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: akteur(wer), art: "aufgabe_angelegt", bezug: `aufgabe:${aufgabe.id}`, daten: { titel: a.titel, stufe: a.stufe, faehigkeiten: a.faehigkeiten, pfade: a.pfade, ...(a.raum ? { raum: a.raum } : {}) } });
    return aufgabe;
  });
}

/** Offene Aufgaben, die der Mensch nach Vertrauensstufe bearbeiten darf (optional nach Fähigkeiten gefiltert). */
export async function aufgabenPassend(db: Db, wer: Wer, projektSlug: string, faehigkeiten?: string[]) {
  const p = await projektPerSlug(db, projektSlug);
  const m = (await db.query("select vertrauensstufe from menschen where id = $1", [wer.menschId])).rows[0];
  const r = await db.query(
    "select id, titel, stufe, faehigkeiten, pfade, lease_minuten, raum from aufgaben where projekt_id = $1 and zustand = 'offen' and min_vertrauen <= $2 order by id",
    [p.id, m.vertrauensstufe],
  );
  // Clean Room: Aufgaben auf der anderen Seite der Trennwand gar nicht erst anbieten.
  const bisher = p.verfahren === "clean_room" ? await raeumeVon(db, wer.menschId, p.id) : [];
  const erlaubt = r.rows.filter((a) => (!a.raum || !trennwand(bisher, a.raum)) && (wer.runner !== false || ohneAusfuehrung(a.faehigkeiten)));
  return faehigkeiten?.length ? erlaubt.filter((a) => a.faehigkeiten.every((f: string) => faehigkeiten.includes(f))) : erlaubt;
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

async function reservierenIntern(db: Db, wer: Wer, aufgabeId: number) {
  return transaktion(db, async (tx) => {
    const a = await aufgabeSperren(tx, aufgabeId);
    const m = (await tx.query("select vertrauensstufe, gesperrt from menschen where id = $1", [wer.menschId])).rows[0];
    if (m.gesperrt) throw new Abgelehnt("account blocked");
    if ((await tx.query("select gesperrt from projekte where id = $1", [a.projekt_id])).rows[0]?.gesperrt) throw new Abgelehnt("project is blocked");
    if (m.vertrauensstufe < a.min_vertrauen) throw new Abgelehnt(`trust level ${a.min_vertrauen} required`);
    if (a.zustand !== "offen") throw new Abgelehnt(`task is ${en(a.zustand)}`);
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
  return { ...a, projekt: p.slug, repo_url: p.repo_url, engine: p.engine, grenzen: GRENZEN };
}

export async function fortschritt(db: Db, wer: Wer, aufgabeId: number, notiz: string, verbrauch: Record<string, number>) {
  return transaktion(db, async (tx) => {
    const a = await aufgabeSperren(tx, aufgabeId);
    const l = await aktiveLeaseVon(tx, aufgabeId, wer.agentId);
    const neu = (await tx.query("update leases set laeuft_ab = now() + make_interval(mins => $2) where id = $1 returning laeuft_ab", [l.id, a.lease_minuten])).rows[0];
    if (a.zustand === "reserviert") await wechseln(tx, a, "in_arbeit", akteur(wer), "arbeit_begonnen", { notiz });
    else await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: akteur(wer), art: "fortschritt", bezug: `aufgabe:${a.id}`, daten: { notiz, verbrauch } });
    return { laeuft_ab: neu.laeuft_ab };
  });
}

/** Geänderte Pfade aus einem Patch im git-format-patch/diff-Format. */
export function geaendertePfade(patch: string): string[] {
  const pfade = new Set<string>();
  for (const zeile of patch.split("\n")) {
    const m = /^diff --git a\/(.+?) b\/(.+)$/.exec(zeile);
    if (m) { pfade.add(m[1]); pfade.add(m[2]); continue; }
    const z = /^(?:\+\+\+|---) (?:a|b)\/(.+)$/.exec(zeile);
    if (z) pfade.add(z[1]);
  }
  return [...pfade];
}

/**
 * Dateien, die die Umgebung, die Projektregeln oder die CI bestimmen. Ein Beitrag darf sie nie ändern,
 * sonst könnte er Befehle auf den Rechnern anderer Mitstreiter einschleusen (gleiche Liste wie
 * runner/umgebung.Geschuetzt). Lead und Visionär ändern sie direkt im Repo.
 */
export const GESCHUETZT = ["swarvex.yaml", ".devcontainer/**", ".devcontainer.json", "AGENTS.md", ".github/workflows/**", ".gitlab-ci.yml", ".forgejo/workflows/**", ".gitea/workflows/**", "LICENSE*", "COPYING*",
  // Anweisungen an Agenten (Skills, Agenten-Konfiguration): sonst könnte ein Beitrag spätere Agenten steuern.
  ".agents/**", ".claude/**", ".codex/**", ".opencode/**", "CLAUDE.md", "opencode.json", "opencode.jsonc"];

export function patchPruefen(patch: string, erlaubt: string[]): string[] {
  const fehler: string[] = [];
  const pfade = geaendertePfade(patch);
  if (pfade.length === 0) fehler.push("patch contains no file changes");
  for (const p of pfade) {
    if (p.includes("..") || p.startsWith("/")) fehler.push(`invalid path: ${p}`);
    else if (GESCHUETZT.some((g) => matchesGlob(p, g))) fehler.push(`protected file (environment, project rules or CI): ${p}`);
    else if (!erlaubt.some((g) => matchesGlob(p, g))) fehler.push(`outside the allowed paths: ${p}`);
  }
  if (pfade.length > GRENZEN.dateien) fehler.push(`too many files: ${pfade.length} > ${GRENZEN.dateien}`);
  const zeilen = patch.split("\n").filter((z) => /^[+-](?![+-]{2} )/.test(z)).length;
  if (zeilen > GRENZEN.zeilen) fehler.push(`too many changed lines: ${zeilen} > ${GRENZEN.zeilen}`);
  return fehler;
}

export async function einreichen(db: Db, wer: Wer, aufgabeId: number, e: { patch: string; zusammenfassung: string; eigene_tests: string; verbrauch: Record<string, number>; eigenes_ergebnis?: "ok" | "fehler" }) {
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
      "insert into einreichungen(aufgabe_id, lease_id, agent_id, patch, patch_sha256, zusammenfassung, eigene_tests, verbrauch) values ($1,$2,$3,$4,$5,$6,$7,$8) returning id",
      [a.id, l.id, wer.agentId, e.patch, sha, e.zusammenfassung, e.eigene_tests, e.verbrauch],
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
    await mussLeadSein(tx, wer.menschId, Number(a.projekt_id));
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

export async function pruefungMelden(db: Db, einreichungId: number, ergebnis: "ok" | "fehler", bericht: string, signatur: string, quelle: "projekt" | "stichprobe" = "projekt") {
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select * from einreichungen where id = $1", [einreichungId])).rows[0];
    if (!e) throw new Abgelehnt("unknown submission");
    const a = await aufgabeSperren(tx, e.aufgabe_id);
    await tx.query("insert into pruefungen(einreichung_id, quelle, ergebnis, bericht, signatur) values ($1,$2,$3,$4,$5)", [einreichungId, quelle, ergebnis, bericht, signatur]);
    await wechseln(tx, a, ergebnis === "ok" ? "geprueft_ok" : "geprueft_fehler", "pruefstrasse", "pruefung_beendet", { einreichung: einreichungId, ergebnis, quelle });
    // Eine gescheiterte Prüfung gibt die Aufgabe automatisch zurück in den Pool.
    if (ergebnis === "fehler") await wechseln(tx, { ...a, zustand: "geprueft_fehler" }, "offen", "system", "zurueck_in_pool", { einreichung: einreichungId });
  });
}

export async function bewerten(db: Db, wer: Wer, einreichungId: number, urteil: "annehmen" | "nacharbeit" | "ablehnen", begruendung: string) {
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select * from einreichungen where id = $1", [einreichungId])).rows[0];
    if (!e) throw new Abgelehnt("unknown submission");
    const a = await aufgabeSperren(tx, e.aufgabe_id);
    await mussLeadSein(tx, wer.menschId, a.projekt_id);
    const autor = (await tx.query("select mensch_id from agenten where id = $1", [e.agent_id])).rows[0];
    if (Number(autor.mensch_id) === wer.menschId) throw new Abgelehnt("nobody reviews their own submission, not even through a second agent");
    if (a.zustand !== "geprueft_ok") throw new Abgelehnt(`review only after a passing check (task is ${en(a.zustand)})`);
    await tx.query("insert into bewertungen(einreichung_id, agent_id, urteil, begruendung) values ($1,$2,$3,$4)", [einreichungId, wer.agentId, urteil, begruendung]);
    await wechseln(tx, a, urteil === "annehmen" ? "angenommen" : "offen", akteur(wer), "bewertet", { einreichung: einreichungId, urteil, begruendung });
  });
}

export async function gemergedMelden(db: Db, einreichungId: number, commit: string) {
  return transaktion(db, async (tx) => {
    const e = (await tx.query("select * from einreichungen where id = $1", [einreichungId])).rows[0];
    const a = await aufgabeSperren(tx, e.aufgabe_id);
    await wechseln(tx, a, "gemerged", "pruefstrasse", "gemerged", { einreichung: einreichungId, commit });
  });
}
