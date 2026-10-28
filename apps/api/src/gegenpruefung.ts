// Gegenprüfung für Projekte mit proprietärer Engine (Unity, Unreal; E-31). Die Prüfstraße hat keine
// Lizenz dafür, also prüfen die Runner der Mitstreiter: zuerst der einreichende Runner selbst, dann
// der Runner eines anderen Menschen mit derselben Engine (Basis-Commit + Patch, Tests ohne Netz, ohne
// Modell und ohne Tokens). Lead und Agenten haben damit keine Arbeit.
import { transaktion, type Db, type Tx } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, projektPerSlug, type Wer } from "./aufgaben.ts";

/** Engines, die nur beim Mitstreiter laufen können (Projektfeld engine, z. B. "Unity 6000.0" oder "unreal"). */
export const proprietaer = (engine: string) => /unity|unreal/i.test(engine ?? "");

/** Reservierung einer Gegenprüfung: so lange darf der Runner Snapshot und LFS-Dateien der Aufgabe laden. */
export const GEGENPRUEFUNG_MINUTEN = 90;
/** Findet sich so lange kein zweiter Runner, zählt der eigene Lauf allein (auf der Projektseite als „selbst geprüft“). */
export const OHNE_GEGENPRUEFUNG_STUNDEN = 24;

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
 * Nächste Gegenprüfung für einen Runner: eine Einreichung im Zustand „pruefung“ eines Engine-Projekts,
 * deren Engine der Runner hat, von einem anderen Menschen, die er nicht schon geprüft hat und die gerade
 * niemand prüft. null, wenn nichts ansteht.
 */
export async function naechsteGegenpruefung(db: Db, wer: Wer, projektSlug: string, engines: string[]) {
  const p = await projektPerSlug(db, projektSlug);
  if (!proprietaer(p.engine)) return null;
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
