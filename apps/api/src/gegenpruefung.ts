// Gegenprüfung durch die Runner der Mitstreiter (E-31): zuerst der einreichende Runner selbst, dann der Runner eines
// anderen Menschen (Basis-Commit + Patch, Tests ohne Netz, ohne Modell und ohne Tokens). Lead und Agenten haben damit
// keine Arbeit, und die Prüfkapazität wächst mit dem Schwarm statt an der Prüfstraße zu hängen.
//   - Proprietäre Engine (Unity, Unreal): Die Prüfstraße hat keine Lizenz; nur Runner mit derselben Engine prüfen.
//   - Alle anderen Projekte (Runner ab 0.11): Zwei grüne Läufe (eigener + Gegenprüfung) ersetzen die Prüfstraße.
//     Rot in der Gegenprüfung, kein eigener grüner Lauf oder niemand da: Die Prüfstraße prüft wie bisher und
//     entscheidet. Eine Gegenprüfung allein lehnt also nie ab; was durchkommt, testet der Merge-Zug noch einmal.
//   - Ändert eine Einreichung Referenzbilder (SZENENBILDER), prüft immer die Prüfstraße: Nur sie liefert die Szenenbilder
//     (alt, neu, Differenz) für Lead und Prüfer. Bis 11.11.2026 ersetzte die Gegenprüfung sie auch hier; Formwerk-Modelle
//     kamen dann ohne Bilder in die Fremdprüfung, ein Prüfer ohne Bildmodell durfte zustimmen, ohne dass jemand das
//     Ergebnis gesehen hatte (so rutschte am 9.11. die Schubkarre durch).
import { transaktion, type Db, type Tx } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, projektPerSlug, type Wer } from "./aufgaben.ts";

/** Engines, die nur beim Mitstreiter laufen können (Projektfeld engine, z. B. "Unity 6000.0" oder "unreal"). */
export const proprietaer = (engine: string) => /unity|unreal/i.test(engine ?? "");

/** Reservierung einer Gegenprüfung: so lange darf der Runner Snapshot und LFS-Dateien der Aufgabe laden. */
export const GEGENPRUEFUNG_MINUTEN = 90;
/** Findet sich so lange kein zweiter Runner, zählt der eigene Lauf allein (auf der Projektseite als „selbst geprüft“). */
export const OHNE_GEGENPRUEFUNG_STUNDEN = 24;
/** Projekte ohne proprietäre Engine: Reservierung kürzer (keine Engine-Bibliothek zu laden); bricht ein Runner ab, übernimmt bald die Prüfstraße. */
export const GEGENPRUEFUNG_MINUTEN_ALLGEMEIN = 45;
/** Projekte ohne proprietäre Engine: so lange wartet eine Einreichung auf einen Gegenprüfer, danach prüft die Prüfstraße. */
export const GEGENPRUEFUNG_WARTEN_MINUTEN = 15;
/** Ein Runner gilt als Gegenprüfer da, wenn er so kurz vorher danach gefragt hat (Runner fragen zu Beginn jeder Runde). */
export const GEGENPRUEFER_AKTIV_MINUTEN = 30;

/**
 * Einreichung in einem Projekt ohne proprietäre Engine: Der eigene Testlauf des Runners wird festgehalten (Quelle
 * „selbst“). Die Aufgabe bleibt „eingereicht“: Gegenprüfung oder Prüfstraße entscheiden (siehe pipeline/pending).
 */
export async function eigenenLaufFesthalten(tx: Tx, einreichungId: number, eigenesErgebnis: "ok" | "fehler" | undefined, bericht: string, wer: Wer) {
  if (!eigenesErgebnis) return;
  await tx.query("insert into pruefungen(einreichung_id, quelle, ergebnis, bericht, signatur) values ($1,'selbst',$2,$3,$4)",
    [einreichungId, eigenesErgebnis, bericht, akteur(wer)]);
}

/**
 * Regulärer Ausdruck (PostgreSQL) auf den Patch: ändert er ein Referenzbild, also eine PNG-Datei in einem Ordner
 * baseline oder baselines (bei three.js tests/scene/baseline, sonst nach swarvex.yaml images_baseline benannt)?
 */
export const SZENENBILDER = String.raw`(^|\n)diff --git a/(\S+/)?baselines?/\S+\.png `;

/** SQL-Bedingung (Einreichung e, Agent des Einreichers ag): Wartet die Einreichung gerade auf eine Gegenprüfung statt auf die Prüfstraße? */
export const WARTET_AUF_GEGENPRUEFUNG = `(
  e.patch !~ '${SZENENBILDER}'
  and exists (select 1 from pruefungen x where x.einreichung_id = e.id and x.quelle = 'selbst' and x.ergebnis = 'ok')
  and not exists (select 1 from pruefungen x where x.einreichung_id = e.id and x.quelle = 'gegenpruefung' and x.ergebnis = 'fehler')
  and (exists (select 1 from gegenpruefungen g where g.einreichung_id = e.id and g.beendet is null and g.laeuft_ab > now())
       or (e.eingereicht > now() - interval '${GEGENPRUEFUNG_WARTEN_MINUTEN} minutes'
           and exists (select 1 from agenten x where x.mensch_id <> ag.mensch_id and x.gegenpruefer_gesehen > now() - interval '${GEGENPRUEFER_AKTIV_MINUTEN} minutes'))))`;

const akteur = (w: Wer) => `agent:${w.agentId}`;

async function aufgabeSperren(tx: Tx, id: number) {
  return (await tx.query("select * from aufgaben where id = $1 for update", [id])).rows[0];
}

async function wechseln(tx: Tx, a: any, nach: string, wer: string, art: string, daten: Record<string, unknown>) {
  await tx.query("update aufgaben set zustand = $2 where id = $1", [a.id, nach]);
  await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: wer, art, bezug: `aufgabe:${a.id}`, daten: { von: a.zustand, nach, ...daten } });
  a.zustand = nach;
}

/**
 * Nach dem Einreichen in einem Engine-Projekt: Der eigene Testlauf des Runners wird als Prüfung „selbst“
 * festgehalten. Rot geht die Aufgabe sofort zurück in den Pool; grün (oder ohne Angabe) wartet sie auf die
 * Gegenprüfung. Läuft in der Transaktion des Einreichens.
 */
export async function nachEinreichen(tx: Tx, a: any, einreichungId: number, eigenesErgebnis: "ok" | "fehler" | undefined, bericht: string, wer: Wer) {
  await wechseln(tx, a, "pruefung", akteur(wer), "pruefung_gestartet", { einreichung: einreichungId, quelle: "selbst" });
  if (!eigenesErgebnis) return;
  await tx.query("insert into pruefungen(einreichung_id, quelle, ergebnis, bericht, signatur) values ($1,'selbst',$2,$3,$4)",
    [einreichungId, eigenesErgebnis, bericht, akteur(wer)]);
  if (eigenesErgebnis === "fehler") {
    await wechseln(tx, a, "geprueft_fehler", akteur(wer), "pruefung_beendet", { einreichung: einreichungId, ergebnis: "fehler", quelle: "selbst" });
    await wechseln(tx, a, "offen", "system", "zurueck_in_pool", { einreichung: einreichungId });
  }
}

/**
 * Nächste Gegenprüfung für einen Runner: eine Einreichung eines anderen Menschen, die er nicht schon geprüft hat und
 * die gerade niemand prüft. Engine-Projekte: im Zustand „pruefung“, nur mit derselben Engine. Andere Projekte (nur
 * Runner ab 0.11, `alle`): „eingereicht“, mit grünem eigenem Lauf, noch ohne Prüfstraße und ohne rote Gegenprüfung.
 * null, wenn nichts ansteht.
 */
export async function naechsteGegenpruefung(db: Db, wer: Wer, projektSlug: string, engines: string[], alle = false) {
  const p = await projektPerSlug(db, projektSlug);
  if (alle) await db.query("update agenten set gegenpruefer_gesehen = now() where id = $1", [wer.agentId]);
  if (!proprietaer(p.engine)) return alle ? allgemeineGegenpruefung(db, wer, p) : null;
  const engine = String(p.engine).toLowerCase();
  if (!engines.some((e) => engine.includes(e.toLowerCase()))) return null;
  return transaktion(db, async (tx) => {
    const m = (await tx.query("select gesperrt from menschen where id = $1", [wer.menschId])).rows[0];
    if (m?.gesperrt) throw new Abgelehnt("account blocked");
    const e = (await tx.query(
      `select e.id, e.patch, e.aufgabe_id, a.basis_commit, a.titel from einreichungen e
       join aufgaben a on a.id = e.aufgabe_id join agenten ag on ag.id = e.agent_id
       where a.projekt_id = $1 and a.zustand = 'pruefung' and not exists (select 1 from projekte where id = $1 and gesperrt)
         and e.id = (select max(id) from einreichungen where aufgabe_id = a.id)
         and ag.mensch_id <> $2
         and not exists (select 1 from gegenpruefungen g where g.einreichung_id = e.id and g.beendet is null and g.laeuft_ab > now())
         and not exists (select 1 from gegenpruefungen g join agenten x on x.id = g.agent_id where g.einreichung_id = e.id and x.mensch_id = $2)
       order by e.id limit 1 for update of a skip locked`,
      [p.id, wer.menschId])).rows[0];
    if (!e) return null;
    const g = (await tx.query(
      "insert into gegenpruefungen(einreichung_id, agent_id, laeuft_ab) values ($1,$2, now() + make_interval(mins => $3)) returning id, laeuft_ab",
      [e.id, wer.agentId, GEGENPRUEFUNG_MINUTEN])).rows[0];
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: akteur(wer), art: "gegenpruefung_reserviert", bezug: `aufgabe:${e.aufgabe_id}`, daten: { einreichung: Number(e.id), gegenpruefung: Number(g.id) } });
    return { gegenpruefung_id: Number(g.id), aufgabe_id: Number(e.aufgabe_id), einreichung_id: Number(e.id), titel: e.titel,
      basis_commit: e.basis_commit, patch: e.patch, engine: p.engine, laeuft_ab: g.laeuft_ab };
  });
}

async function allgemeineGegenpruefung(db: Db, wer: Wer, p: any) {
  return transaktion(db, async (tx) => {
    const m = (await tx.query("select gesperrt from menschen where id = $1", [wer.menschId])).rows[0];
    if (m?.gesperrt) throw new Abgelehnt("account blocked");
    const e = (await tx.query(
      `select e.id, e.patch, e.aufgabe_id, a.basis_commit, a.titel from einreichungen e
       join aufgaben a on a.id = e.aufgabe_id join agenten ag on ag.id = e.agent_id
       where a.projekt_id = $1 and a.zustand = 'eingereicht' and not exists (select 1 from projekte where id = $1 and gesperrt)
         and e.id = (select max(id) from einreichungen where aufgabe_id = a.id)
         and ag.mensch_id <> $2
         and exists (select 1 from pruefungen x where x.einreichung_id = e.id and x.quelle = 'selbst' and x.ergebnis = 'ok')
         and e.patch !~ '${SZENENBILDER}'   -- Referenzbilder geändert: die Prüfstraße liefert die Szenenbilder
         and not exists (select 1 from pruefungen x where x.einreichung_id = e.id and (x.quelle = 'projekt' or (x.quelle = 'gegenpruefung' and x.ergebnis = 'fehler')))
         and not exists (select 1 from gegenpruefungen g where g.einreichung_id = e.id and g.beendet is null and g.laeuft_ab > now())
         and not exists (select 1 from gegenpruefungen g join agenten x on x.id = g.agent_id where g.einreichung_id = e.id and x.mensch_id = $2)
       order by e.id limit 1 for update of a skip locked`,
      [p.id, wer.menschId])).rows[0];
    if (!e) return null;
    const g = (await tx.query(
      "insert into gegenpruefungen(einreichung_id, agent_id, laeuft_ab) values ($1,$2, now() + make_interval(mins => $3)) returning id, laeuft_ab",
      [e.id, wer.agentId, GEGENPRUEFUNG_MINUTEN_ALLGEMEIN])).rows[0];
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: akteur(wer), art: "gegenpruefung_reserviert", bezug: `aufgabe:${e.aufgabe_id}`, daten: { einreichung: Number(e.id), gegenpruefung: Number(g.id) } });
    return { gegenpruefung_id: Number(g.id), aufgabe_id: Number(e.aufgabe_id), einreichung_id: Number(e.id), titel: e.titel,
      basis_commit: e.basis_commit, patch: e.patch, engine: p.engine, laeuft_ab: g.laeuft_ab };
  });
}

/** Ergebnis einer Gegenprüfung. „nicht_moeglich“ (z. B. falsche Engine-Version) gibt sie nur frei. */
export async function gegenpruefungMelden(db: Db, wer: Wer, id: number, ergebnis: "ok" | "fehler" | "nicht_moeglich", bericht: string) {
  return transaktion(db, async (tx) => {
    const g = (await tx.query("select * from gegenpruefungen where id = $1 for update", [id])).rows[0];
    if (!g || Number(g.agent_id) !== wer.agentId) throw new Abgelehnt("this cross-check is not reserved for you");
    if (g.beendet) throw new Abgelehnt("cross-check already reported");
    if (new Date(g.laeuft_ab) < new Date()) throw new Abgelehnt("cross-check reservation expired");
    await tx.query("update gegenpruefungen set beendet = now(), ergebnis = $2 where id = $1", [id, ergebnis]);
    const e = (await tx.query("select * from einreichungen where id = $1", [g.einreichung_id])).rows[0];
    const a = await aufgabeSperren(tx, e.aufgabe_id);
    if (ergebnis === "nicht_moeglich") {
      await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: akteur(wer), art: "gegenpruefung_zurueckgegeben", bezug: `aufgabe:${a.id}`, daten: { einreichung: Number(e.id), grund: bericht.slice(0, 300) } });
      return { ergebnis };
    }
    const p = (await tx.query("select engine from projekte where id = $1", [a.projekt_id])).rows[0];
    if (!proprietaer(p.engine)) {
      // Projekt ohne proprietäre Engine: Grün mit grünem eigenem Lauf ersetzt die Prüfstraße; Rot entscheidet die Prüfstraße.
      if (a.zustand !== "eingereicht") throw new Abgelehnt("the submission is no longer waiting for a check");
      await tx.query("insert into pruefungen(einreichung_id, quelle, ergebnis, bericht, signatur) values ($1,'gegenpruefung',$2,$3,$4)",
        [e.id, ergebnis, bericht, akteur(wer)]);
      if (ergebnis === "fehler") {
        await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: akteur(wer), art: "gegenpruefung_rot", bezug: `aufgabe:${a.id}`, daten: { einreichung: Number(e.id), entscheidet: "test_pipeline" } });
        return { ergebnis, entscheidet: "test_pipeline" };
      }
      await wechseln(tx, a, "pruefung", akteur(wer), "pruefung_gestartet", { einreichung: Number(e.id), quelle: "gegenpruefung" });
      await wechseln(tx, a, "geprueft_ok", akteur(wer), "pruefung_beendet", { einreichung: Number(e.id), ergebnis, quelle: "gegenpruefung" });
      return { ergebnis };
    }
    if (a.zustand !== "pruefung") throw new Abgelehnt("the submission is no longer waiting for a check");
    await tx.query("insert into pruefungen(einreichung_id, quelle, ergebnis, bericht, signatur) values ($1,'gegenpruefung',$2,$3,$4)",
      [e.id, ergebnis, bericht, akteur(wer)]);
    const selbst = (await tx.query("select ergebnis from pruefungen where einreichung_id = $1 and quelle = 'selbst' order by id desc limit 1", [e.id])).rows[0]?.ergebnis;
    await wechseln(tx, a, ergebnis === "ok" ? "geprueft_ok" : "geprueft_fehler", akteur(wer), "pruefung_beendet",
      { einreichung: Number(e.id), ergebnis, quelle: "gegenpruefung", ...(selbst && selbst !== ergebnis ? { widerspricht_selbst: true } : {}) });
    if (ergebnis === "fehler") await wechseln(tx, a, "offen", "system", "zurueck_in_pool", { einreichung: Number(e.id) });
    return { ergebnis };
  });
}

/**
 * Läuft periodisch: Einreichungen, die selbst grün geprüft wurden und seit OHNE_GEGENPRUEFUNG_STUNDEN
 * keinen zweiten Runner gefunden haben, gehen mit dem eigenen Lauf allein zum Lead.
 */
export async function ohneGegenpruefungFreigeben(db: Db): Promise<number> {
  return transaktion(db, async (tx) => {
    const offen = (await tx.query(
      `select a.id, e.id as einreichung_id from aufgaben a join einreichungen e on e.aufgabe_id = a.id
       where a.zustand = 'pruefung' and e.id = (select max(id) from einreichungen where aufgabe_id = a.id)
         and e.eingereicht < now() - make_interval(hours => $1)
         and exists (select 1 from pruefungen p where p.einreichung_id = e.id and p.quelle = 'selbst' and p.ergebnis = 'ok')
         and not exists (select 1 from gegenpruefungen g where g.einreichung_id = e.id and g.beendet is null and g.laeuft_ab > now())
       for update of a skip locked`, [OHNE_GEGENPRUEFUNG_STUNDEN])).rows;
    for (const o of offen) {
      const a = await aufgabeSperren(tx, o.id);
      await wechseln(tx, a, "geprueft_ok", "system", "pruefung_beendet", { einreichung: Number(o.einreichung_id), ergebnis: "ok", quelle: "selbst", ohne_gegenpruefung: true });
    }
    return offen.length;
  });
}

/** Darf dieser Agent die Aufgabe gerade für eine Gegenprüfung laden (Snapshot, LFS)? */
export async function gegenpruefungAktiv(db: Db, agentId: number, aufgabeId: number): Promise<boolean> {
  const r = await db.query(
    `select 1 from gegenpruefungen g join einreichungen e on e.id = g.einreichung_id
     where e.aufgabe_id = $1 and g.agent_id = $2 and g.beendet is null and g.laeuft_ab > now()`, [aufgabeId, agentId]);
  return r.rowCount! > 0;
}
