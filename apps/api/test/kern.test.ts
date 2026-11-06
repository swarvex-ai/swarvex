// Kern-Tests gegen eine echte PostgreSQL (TEST_DATABASE_URL, wird geleert).
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { migrieren, verbinden, type Db } from "../src/db.ts";
import { ketteVerifizieren } from "../src/ereignisse.ts";
import * as k from "../src/aufgaben.ts";
import { kennzahlen, projektListe } from "../src/kennzahlen.ts";
import { projektAnlegen, projektPruefen, repoVerbinden, uebersetzungSetzen, type NeuesProjekt } from "../src/projekte.ts";
import * as g from "../src/gegenpruefung.ts";
import { createServer, type Server } from "node:http";
import { createHash, randomBytes } from "node:crypto";
import { anbieterErstellen } from "../src/auth/anbieter.ts";
import { interaktionBehandeln } from "../src/auth/interaktion.ts";
import { schluesselLaden } from "../src/auth/schluessel.ts";
import { konfig, RESSOURCE } from "../src/konfig.ts";

let db: Db;
const lead = { menschId: 0, agentId: 0 };
const mit = { menschId: 0, agentId: 0 };
const fremd = { menschId: 0, agentId: 0 };

const PATCH_OK = `diff --git a/game/modules/vfx/neu.gd b/game/modules/vfx/neu.gd
new file mode 100644
--- /dev/null
+++ b/game/modules/vfx/neu.gd
@@ -0,0 +1,2 @@
+extends Node3D
+## Neuer Effekt
`;

beforeAll(async () => {
  db = verbinden(process.env.TEST_DATABASE_URL!);
  await db.query("drop schema if exists analytik cascade; drop schema public cascade; create schema public;");
  await migrieren(db);
  const mensch = async (login: string) => Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea',$1,$1) returning id", [login])).rows[0].id);
  const agent = async (m: number, c: string) => Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,$2,$2) returning id", [m, c])).rows[0].id);
  lead.menschId = await mensch("nevs"); lead.agentId = await agent(lead.menschId, "claude-code");
  mit.menschId = await mensch("mitstreiterin"); mit.agentId = await agent(mit.menschId, "runner-1");
  fremd.menschId = await mensch("fremd"); fremd.agentId = await agent(fremd.menschId, "runner-2");
  const p = Number((await db.query(
    `insert into projekte(slug,name,vision_md,vision_kurz,engine,lizenz_code,lizenz_assets,repo_url,repo_git,lead_modell,ziel)
     values ('test','Test','Vision','kurz','godot-4.7','MIT','CC BY 4.0','http://x','git@example.org:test.git','Claude Opus 5.5','Demo') returning id`)).rows[0].id);
  await db.query("insert into projekt_rollen values ($1,$2,'lead')", [p, lead.menschId]);
});
afterAll(async () => { await db.end(); });

const neueAufgabe = (titel: string): k.NeueAufgabe => ({
  titel, beschreibung: "Effekt bauen", stufe: "B", faehigkeiten: ["code.gdscript"], pfade: ["game/modules/vfx/**"], akzeptanz: ["Tests grün"], meilenstein: "M1", neuer_meilenstein: true,
});

describe("Meilensteine", () => {
  it("lehnt unbekannte Titel ab und legt neue nur ausdrücklich an", async () => {
    await expect(k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Tippfehler"), meilenstein: "1", neuer_meilenstein: false })).rejects.toThrow(/unknown milestone "1"/);
    const a = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Neu"), meilenstein: "M2", neuer_meilenstein: true });
    const b = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Wieder"), meilenstein: "M2", neuer_meilenstein: false });
    expect(b.meilenstein_id).toBe(a.meilenstein_id);
  });
});

describe("Basis beim Reservieren", () => {
  it("eine offene Aufgabe bekommt beim Reservieren den gemeldeten Stand von main, eine reservierte behält ihre Basis", async () => {
    const alt = "1".repeat(40), kopf = "2".repeat(40), neuer = "3".repeat(40);
    const a = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Auf altem Stand"), basis_commit: alt });
    // Ohne gemeldeten Stand bleibt die Basis, wie der Lead sie gesetzt hat
    await k.reservieren(db, mit, a.id);
    expect((await db.query("select basis_commit from aufgaben where id = $1", [a.id])).rows[0].basis_commit).toBe(alt);
    await k.abbrechen(db, mit, a.id, "zurück");
    await db.query("update projekte set kopf_commit = $1 where slug = 'test'", [kopf]);
    await k.reservieren(db, mit, a.id);
    expect((await db.query("select basis_commit from aufgaben where id = $1", [a.id])).rows[0].basis_commit).toBe(kopf);
    const e = (await db.query("select daten from ereignisse where art = 'aufgabe_reserviert' and bezug = $1 order by seq desc limit 1", [`aufgabe:${a.id}`])).rows[0];
    expect(e.daten.neue_basis).toBe(kopf);
    // main bewegt sich weiter: die laufende Reservierung behält ihre Basis
    await db.query("update projekte set kopf_commit = $1 where slug = 'test'", [neuer]);
    expect((await k.details(db, mit, a.id) as { basis_commit: string }).basis_commit).toBe(kopf);
    await k.abbrechen(db, mit, a.id, "zurück");
    await db.query("update projekte set kopf_commit = null where slug = 'test'");
  });
});

describe("Aufgabe schließen", () => {
  it("nur der Lead, nur offene Aufgaben, mit Grund und im Protokoll", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Überholt"));
    await expect(k.schliessen(db, mit, a.id, "weg")).rejects.toThrow(/lead/);
    await expect(k.schliessen(db, lead, a.id, " ")).rejects.toThrow(/reason/);
    await k.schliessen(db, lead, a.id, "verschoben nach v1");
    expect((await db.query("select zustand from aufgaben where id = $1", [a.id])).rows[0].zustand).toBe("geschlossen");
    expect((await db.query("select daten from ereignisse where art = 'aufgabe_geschlossen' and bezug = $1", [`aufgabe:${a.id}`])).rows[0].daten.grund).toBe("verschoben nach v1");
    const b = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Reserviert"));
    await k.reservieren(db, mit, b.id);
    await expect(k.schliessen(db, lead, b.id, "weg")).rejects.toThrow(/only open tasks/);
    await k.abbrechen(db, mit, b.id, "zurück");
  });
});

describe("Abhängigkeiten (Fluss ohne Warten)", () => {
  it("Folgeaufgabe wird erst frei, wenn alle Vorgänger angenommen sind", async () => {
    const v = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Vorgänger"));
    await expect(k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Kaputt"), abhaengig_von: [999999] })).rejects.toThrow(/depends_on/);
    const f = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Folge"), abhaengig_von: [Number(v.id)] });
    const frei = async () => (await k.aufgabenPassend(db, mit, "test")).map((x) => Number(x.id));
    expect(await frei()).toContain(Number(v.id));
    expect(await frei()).not.toContain(Number(f.id));
    await expect(k.reservieren(db, mit, f.id)).rejects.toThrow(/waits for/);
    // Varianten-Paar: Folgeaufgabe braucht v und eine der beiden Varianten
    const x = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Variante 1"));
    const y = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Variante 2"));
    const g = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Nach dem Paar"), abhaengig_von: [Number(v.id)], abhaengig_eins_von: [[Number(x.id), Number(y.id)]] });
    await expect(k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Allein"), abhaengig_eins_von: [[Number(x.id)]] })).rejects.toThrow(/depends_on_any/);
    await db.query("update aufgaben set zustand = 'angenommen' where id = $1", [v.id]);
    expect(await frei()).toContain(Number(f.id));
    expect(await frei()).not.toContain(Number(g.id));
    expect(await k.wartetAuf(db, (await db.query("select * from aufgaben where id = $1", [g.id])).rows[0])).toEqual([[Number(x.id), Number(y.id)]]);
    await k.schliessen(db, lead, x.id, "Variante verworfen");
    expect(await frei()).not.toContain(Number(g.id));      // geschlossen zählt nicht als erfüllt
    await db.query("update aufgaben set zustand = 'gemerged' where id = $1", [y.id]);
    expect(await frei()).toContain(Number(g.id));
    for (const t of [f, g]) await k.schliessen(db, lead, t.id, "Test");
  });
  it("wait_for_work kehrt sofort zurück, wenn Nachschub fehlt, sonst nach der Frist", async () => {
    const { aufArbeitWarten } = await import("../src/fluss.ts");
    const ohne = await aufArbeitWarten(db, lead, "test", 0, ["reviews", "messages"], async () => {});
    expect(ohne.weiter).toBe(true);
    await expect(aufArbeitWarten(db, mit, "test", 0, ["reviews"], async () => {})).rejects.toThrow(/lead/);
  });
});

describe("Direkt-Modus (#8)", () => {
  it("direkt angebundene Agenten bekommen nur Aufgaben ohne Code-Ausführung", async () => {
    expect(k.ohneAusfuehrung(["text.en", "design.ui"])).toBe(true);
    expect(k.ohneAusfuehrung(["text.en", "code.gdscript"])).toBe(false);
    expect(k.ohneAusfuehrung(["simulation"])).toBe(false);
    expect(k.ohneAusfuehrung([])).toBe(false);
    const code = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Code direkt"));
    const text = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Text direkt"), faehigkeiten: ["text.en"], pfade: ["docs/**"] });
    const direkt = { ...mit, runner: false };
    const ids = (await k.aufgabenPassend(db, direkt, "test")).map((x) => Number(x.id));
    expect(ids).toContain(Number(text.id));
    expect(ids).not.toContain(Number(code.id));
    expect((await k.aufgabenPassend(db, { ...mit, runner: true }, "test")).map((x) => Number(x.id))).toContain(Number(code.id));
    await expect(k.reservieren(db, direkt, code.id)).rejects.toThrow(/runner/);
    await k.reservieren(db, direkt, text.id);
    await k.abbrechen(db, direkt, text.id, "nur Test");
    await k.schliessen(db, lead, code.id, "Test");
    await k.schliessen(db, lead, text.id, "Test");
  });
});

describe("Abbrüche", () => {
  it("zweimal in 24 Stunden abgebrochen: demselben Agenten vorerst nicht mehr angeboten", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Bricht zweimal ab"), faehigkeiten: ["text.en"], pfade: ["docs/**"] });
    const ids = async (w: typeof mit) => (await k.aufgabenPassend(db, w, "test")).map((x) => Number(x.id));
    await k.reservieren(db, mit, a.id);
    await k.abbrechen(db, mit, a.id, "Budget");
    expect(await ids(mit)).toContain(Number(a.id));
    await k.reservieren(db, mit, a.id);
    await k.abbrechen(db, mit, a.id, "Budget");
    expect(await ids(mit)).not.toContain(Number(a.id));
    await k.schliessen(db, lead, a.id, "Test");
  });
});

describe("Analytik (intern)", () => {
  it("Sichten liefern Summen ohne Personenbezug, der Leser sieht nur sie", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Analytik"), faehigkeiten: ["text.en"], pfade: ["docs/**"] });
    await k.reservieren(db, mit, a.id);
    await k.einreichen(db, mit, a.id, { patch: "diff --git a/docs/x.md b/docs/x.md\nnew file mode 100644\n--- /dev/null\n+++ b/docs/x.md\n@@ -0,0 +1,2 @@\n+a\n+b\n", zusammenfassung: "x", eigene_tests: "", verbrauch: { tokens: 1500, sekunden: 60 }, agent_programm: "opencode", modell: "qwen3-coder" });
    const e = (await db.query("select * from analytik.einreichungen where aufgabe = $1", [a.id])).rows[0];
    expect(e).toMatchObject({ agent_programm: "opencode", modell: "qwen3-coder", familie: "text" });
    expect(Number(e.zeilen)).toBe(2);
    expect(Number(e.tokens)).toBe(1500);
    expect(e.mitstreiter).toMatch(/^[0-9a-f]{12}$/);
    // Ein Runner ohne Angabe erscheint als "runner", nie mit seinem Namen (enthält den Rechnernamen)
    const alterName = (await db.query("select name from agenten where id = $1", [mit.agentId])).rows[0].name;
    await db.query("update agenten set runner = true, name = 'swarvex runner (rechner-von-jemand)' where id = $1", [mit.agentId]);
    await db.query("update einreichungen set agent_programm = null where aufgabe_id = $1", [a.id]);
    expect((await db.query("select agent_programm from analytik.einreichungen where aufgabe = $1", [a.id])).rows[0].agent_programm).toBe("runner");
    await db.query("update agenten set runner = null, name = $2 where id = $1", [mit.agentId, alterName]);
    // keine Spalte mit Login, Mensch-ID oder freiem Text
    const spalten = (await db.query("select column_name from information_schema.columns where table_schema = 'analytik'")).rows.map((r) => r.column_name);
    for (const verboten of ["login", "mensch_id", "patch", "zusammenfassung", "begruendung", "beschreibung"]) expect(spalten).not.toContain(verboten);
    for (const sicht of ["projekte", "aufgaben", "aktivitaet", "trend", "schwarm", "trichter", "signale"]) await db.query(`select * from analytik.${sicht} limit 1`);
    // Der Leser darf die Sichten lesen, aber keine Tabelle dahinter
    await db.query("begin");
    try {
      await db.query("set local role analytik_leser");
      expect(Number((await db.query("select count(*) from analytik.einreichungen")).rows[0].count)).toBeGreaterThan(0);
      await expect(db.query("select count(*) from public.menschen")).rejects.toThrow(/permission denied/);
    } finally { await db.query("rollback"); }
    await db.query("begin");
    try {
      await db.query("set local role analytik_leser");
      await expect(db.query("select * from public.analytik_salz")).rejects.toThrow(/permission denied/);
    } finally { await db.query("rollback"); }
  });
});

describe("Officers (lead/roles)", () => {
  it("führen nur ihren Bereich: Stufen A und B, Pfade und Fähigkeiten im Bereich; nur der Visionär ernennt", async () => {
    const vis = { menschId: Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea','vis','vis') returning id")).rows[0].id), agentId: 0 };
    vis.agentId = Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,'vis-agent','vis-agent') returning id", [vis.menschId])).rows[0].id);
    await db.query("insert into projekt_rollen select id, $1, 'visionaer' from projekte where slug = 'test'", [vis.menschId]);
    const o = { menschId: fremd.menschId, agentId: fremd.agentId };
    await expect(k.officerErnennen(db, lead, "test", "fremd", "audio", ["game/audio/**"], ["asset.audio"])).rejects.toThrow(/visionary/);
    await expect(k.officerErnennen(db, vis, "test", "fremd", "audio", ["**"], ["asset.audio"])).rejects.toThrow(/whole repository/);
    // Bereichs-Leads brauchen Vertrauensstufe 2 (E-35)
    await expect(k.officerErnennen(db, vis, "test", "fremd", "audio", ["game/audio/**"], ["asset.audio", "text.*"])).rejects.toThrow(/trust level 0; area leads need level 2/);
    await db.query("update menschen set vertrauensstufe = 2 where login = 'fremd'");
    await k.officerErnennen(db, vis, "test", "fremd", "audio", ["game/audio/**"], ["asset.audio", "text.*"]);
    expect((await db.query("select vertrauensstufe from menschen where login = 'fremd'")).rows[0].vertrauensstufe).toBe(3);
    const basis = { beschreibung: "Klang", akzeptanz: ["ok"], meilenstein: "M1" };
    const t = await k.aufgabeAnlegen(db, o, "test", { ...basis, titel: "UI-Klänge", stufe: "A", faehigkeiten: ["asset.audio"], pfade: ["game/audio/ui/**"] });
    expect(t.rolle).toBe("audio");
    await expect(k.aufgabeAnlegen(db, o, "test", { ...basis, titel: "Fremder Pfad", stufe: "A", faehigkeiten: ["asset.audio"], pfade: ["game/player/**"] })).rejects.toThrow(/area lead for this area/);
    await expect(k.aufgabeAnlegen(db, o, "test", { ...basis, titel: "Zu groß", stufe: "C", faehigkeiten: ["asset.audio"], pfade: ["game/audio/**"] })).rejects.toThrow(/area lead for this area/);
    await expect(k.aufgabeAnlegen(db, o, "test", { ...basis, titel: "Code", stufe: "B", faehigkeiten: ["code.gdscript"], pfade: ["game/audio/x.gd"] })).rejects.toThrow(/area lead for this area/);
    await expect(k.aufgabeAnlegen(db, o, "test", { ...basis, titel: "Trick", stufe: "A", faehigkeiten: ["asset.audio"], pfade: ["game/audio/../player/**"] })).rejects.toThrow(/area lead for this area/);
    // Bewerten im Bereich; eine Lead-Aufgabe außerhalb sieht und bewertet er nicht
    const PATCH_AUDIO = "diff --git a/game/audio/ui/klick.txt b/game/audio/ui/klick.txt\nnew file mode 100644\n--- /dev/null\n+++ b/game/audio/ui/klick.txt\n@@ -0,0 +1 @@\n+klick\n";
    await k.reservieren(db, mit, t.id);
    const e = (await k.einreichen(db, mit, t.id, { patch: PATCH_AUDIO, zusammenfassung: "x", eigene_tests: "", verbrauch: {} })) as any;
    await k.pruefungStarten(db, e.einreichung_id);
    await k.pruefungMelden(db, e.einreichung_id, "ok", "grün", "sig");
    const fremdeAufgabe = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Lead-Aufgabe"));
    expect((await k.wartendeEinreichungen(db, o, "test")).map((x) => Number(x.aufgabe_id))).toEqual([Number(t.id)]);
    await expect(k.schliessen(db, o, fremdeAufgabe.id, "nicht meins")).rejects.toThrow(/area lead for this area/);
    await k.bewerten(db, o, e.einreichung_id, "annehmen", "passt zur Sound Bible");
    // Bilanz und Kennzahlen zeigen den Officer
    const kz = await kennzahlen(db, "test");
    expect(kz.lead_bilanz.officers.map((x: any) => x.bereich)).toEqual(["audio"]);
    expect(kz.lead_bilanz.je_rolle.some((x: any) => x.rolle === "audio")).toBe(true);
    // Viele offene Aufgaben ohne Bewegung: kein Officer, der Engpass sind Mitstreiter
    const ruhig = [];
    for (let i = 0; i < 16; i++) ruhig.push(await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe(`QA ${i}`), rolle: "qa" }));
    const still = await kennzahlen(db, "test");
    expect(still.lead_bilanz.officer_empfehlung).toEqual([]);
    expect(still.lead_bilanz.engpass).toBe("mitstreiter");
    for (const r of ruhig) await k.schliessen(db, lead, r.id, "Test");
    // Amtszeit beendet: keine Rechte mehr
    await k.officerBeenden(db, vis, "test", "audio", "Test vorbei");
    await expect(k.aufgabeAnlegen(db, o, "test", { ...basis, titel: "Nachher", stufe: "A", faehigkeiten: ["asset.audio"], pfade: ["game/audio/ui/**"] })).rejects.toThrow(/lead/);
    const arten = (await db.query("select art from ereignisse where art like 'officer_%' order by seq")).rows.map((r) => r.art);
    expect(arten).toEqual(["officer_ernannt", "officer_beendet"]);
    await k.schliessen(db, lead, fremdeAufgabe.id, "Test");
    await db.query("delete from projekt_rollen where mensch_id = $1", [vis.menschId]);
  });
});

describe("Nachrichten (Lead, Visionär, Officers)", () => {
  it("Fragen und Freigaben laufen projektintern; Freigaben entscheidet nur der Visionär, öffentlich protokolliert", async () => {
    const n = await import("../src/nachrichten.ts");
    const neuer = async (login: string) => {
      const m = Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea',$1,$1) returning id", [login])).rows[0].id);
      const a = Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,$2,$2) returning id", [m, login + "-agent"])).rows[0].id);
      return { menschId: m, agentId: a };
    };
    const vis = await neuer("msgvis"), off = await neuer("msgoff");
    await db.query("insert into projekt_rollen select id, $1, 'visionaer' from projekte where slug = 'test'", [vis.menschId]);
    await db.query("update menschen set vertrauensstufe = 2 where login = 'msgoff'");
    await k.officerErnennen(db, vis, "test", "msgoff", "ux", ["game/ui/**"], ["design.ui"]);
    // Mitstreiter ohne Rolle schreiben nicht
    await expect(n.nachrichtSenden(db, mit, "test", { an: "lead", art: "frage", betreff: "Hallo", inhalt: "x" })).rejects.toThrow(/only the project's lead/);
    // Officer fragt den Lead
    const f = await n.nachrichtSenden(db, off, "test", { an: "lead", art: "frage", betreff: "Fokus im Menü", inhalt: "Soll Zurück immer B sein?" });
    await expect(n.nachrichtSenden(db, off, "test", { an: "bereich:audio", art: "frage", betreff: "Klang", inhalt: "x" })).rejects.toThrow(/no active area lead/);
    await expect(n.nachrichtSenden(db, off, "test", { an: "lead", art: "freigabe", betreff: "Release", inhalt: "x" })).rejects.toThrow(/approvals go to the visionary/);
    expect((await n.offeneFuerMich(db, lead.menschId, (await k.projektPerSlug(db, "test")).id))?.offen).toBe(1);
    expect((await kennzahlen(db, "test")).lead_bilanz.nachrichten_offen).toBe(1);
    await expect(n.nachrichtBeantworten(db, off.menschId, off.agentId, f.nachricht_id, "selbst")).rejects.toThrow(/only the recipient/);
    await n.nachrichtBeantworten(db, lead.menschId, lead.agentId, f.nachricht_id, "Ja, B ist immer Zurück.");
    expect((await n.posteingang(db, off, "test"))[0]).toMatchObject({ zustand: "beantwortet", antwort: "Ja, B ist immer Zurück." });
    // Freigabe an den Visionär
    const g = await n.nachrichtSenden(db, off, "test", { an: "visionaer", art: "freigabe", betreff: "Menü-Styleguide Variante 2", inhalt: "Empfehlung: Variante 2" });
    await expect(n.nachrichtBeantworten(db, lead.menschId, lead.agentId, g.nachricht_id, "ok", "freigegeben")).rejects.toThrow(/only the recipient/);
    await expect(n.nachrichtBeantworten(db, vis.menschId, vis.agentId, g.nachricht_id, "ok")).rejects.toThrow(/needs a decision/);
    expect((await n.offeneFreigaben(db, vis.menschId)).map((x) => x.nachricht_id)).toContain(g.nachricht_id);
    await n.nachrichtBeantworten(db, vis.menschId, null, g.nachricht_id, "Variante 2, passt zur Vision", "freigegeben");
    const e = (await db.query("select art, daten from ereignisse where bezug = $1 order by seq", [`nachricht:${g.nachricht_id}`])).rows;
    expect(e.map((x) => x.art)).toEqual(["freigabe_angefragt", "freigabe_entschieden"]);
    expect(e[1].daten).toEqual({ betreff: "Menü-Styleguide Variante 2", entscheidung: "freigegeben" });
    // Freigeben geht ohne Begründung, Ablehnen nicht
    const h = await n.nachrichtSenden(db, off, "test", { an: "visionaer", art: "freigabe", betreff: "Push", inhalt: "Bitte freigeben" });
    await expect(n.nachrichtBeantworten(db, vis.menschId, null, h.nachricht_id, " ", "abgelehnt")).rejects.toThrow(/a decline needs a reason/);
    await n.nachrichtBeantworten(db, vis.menschId, null, h.nachricht_id, "", "freigegeben");
    expect((await db.query("select zustand, antwort, entscheidung from nachrichten where id = $1", [h.nachricht_id])).rows[0])
      .toEqual({ zustand: "beantwortet", antwort: null, entscheidung: "freigegeben" });
    await k.officerBeenden(db, vis, "test", "ux", "Test");
    await db.query("delete from projekt_rollen where mensch_id = $1", [vis.menschId]);
  });
});

describe("Inhalt von Medien-Dateien", () => {
  it("erkennt Modelle, Bilder und Klänge am Anfang der Datei", async () => {
    const { inhaltPasst } = await import("../src/medien.ts");
    expect(inhaltPasst("png", Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0]))).toBe(true);
    expect(inhaltPasst("png", Buffer.from("PNG? nein"))).toBe(false);
    expect(inhaltPasst("wav", Buffer.from("RIFF\u0000\u0000\u0000\u0000WAVEfmt "))).toBe(true);
    expect(inhaltPasst("webp", Buffer.from("RIFF\u0000\u0000\u0000\u0000WAVEfmt "))).toBe(false);
    expect(inhaltPasst("obj", Buffer.from("# Kiste\nmtllib kiste.mtl\no Kiste\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"))).toBe(true);
    expect(inhaltPasst("obj", Buffer.from("export OPENAI_API_KEY=sk-geheim\nv 1 2 3\n"))).toBe(false);
    expect(inhaltPasst("mp4", Buffer.from("\u0000\u0000\u0000\u0018ftypmp42"))).toBe(true);
    expect(inhaltPasst("md", Buffer.from("# Notiz"))).toBeNull();
  });
});

describe("Credit-Spenden (Auftrag + Upload)", () => {
  it("Zusage, Auftrag des Leads, Upload mit Prüfsumme, Bewertung; Budget der Zusage hält", async () => {
    const sp = await import("../src/spenden.ts");
    const { Readable } = await import("node:stream");
    const { mkdtempSync, readFileSync } = await import("node:fs");
    const { tmpdir } = await import("node:os");
    const { join } = await import("node:path");
    process.env.SPENDEN_DIR = mkdtempSync(join(tmpdir(), "swx-spenden-"));
    const morgen = new Date(Date.now() + 86_400_000).toISOString().slice(0, 10);
    await expect(sp.zusageAnlegen(db, mit.menschId, { projekt: "test", dienst: "meshy", credits: 100, verfaellt: "2020-01-01" })).rejects.toThrow(/expiry/);
    await expect(sp.zusageAnlegen(db, mit.menschId, { projekt: "test", dienst: "bitcoin", credits: 100, verfaellt: morgen })).rejects.toThrow(/service/);
    const { zusage_id } = await sp.zusageAnlegen(db, mit.menschId, { projekt: "test", dienst: "meshy", credits: 100, verfaellt: morgen, notiz: "Abo endet morgen" });
    // Nur Lead, Visionär oder Officers sehen Zusagen und legen Aufträge an
    await expect(sp.zusagenFuerLead(db, fremd.menschId, "test")).rejects.toThrow(/only the project's lead/);
    expect((await sp.zusagenFuerLead(db, lead.menschId, "test"))[0]).toMatchObject({ dienst: "meshy", credits: 100, frei: 100 });
    const basis = { zusage_id, titel: "Drohne als 3D-Modell", anleitung: "Prompt: kleine Sprengstoff-Drohne, low poly, PBR", zielpfad: "asset_library/models/drohne.glb", dateitypen: ["glb"], credits: 60 };
    const { auftrag_id } = await sp.auftragAnlegen(db, lead.menschId, basis);
    await expect(sp.auftragAnlegen(db, lead.menschId, { ...basis, credits: 50 })).rejects.toThrow(/40 credits left/);
    await expect(sp.auftragAnlegen(db, lead.menschId, { ...basis, credits: 10, dateitypen: ["exe"] })).rejects.toThrow(/file types/);
    // Upload: nur der Spender, nur erlaubte Endung, mit Prüfsumme
    const inhalt = Buffer.from("glTF-Testinhalt");
    const anfrage = (b: Buffer) => Object.assign(Readable.from([b]), { headers: { "content-length": String(b.length) } }) as never;
    await expect(sp.hochladen(db, fremd.menschId, auftrag_id, anfrage(inhalt), "drohne.glb")).rejects.toThrow(/unknown brief/);
    await expect(sp.hochladen(db, mit.menschId, auftrag_id, anfrage(inhalt), "drohne.exe")).rejects.toThrow(/not allowed/);
    const h = await sp.hochladen(db, mit.menschId, auftrag_id, anfrage(inhalt), "drohne.glb");
    expect(h.bytes).toBe(inhalt.length);
    const a = (await sp.auftraegeFuerLead(db, lead.menschId, "test"))[0];
    expect(a).toMatchObject({ zustand: "hochgeladen", sha256: h.sha256, datei_url: `/donations/${auftrag_id}/file` });
    const pfad = (await db.query("select datei from spendenauftraege where id = $1", [auftrag_id])).rows[0].datei;
    expect(readFileSync(pfad).equals(inhalt)).toBe(true);
    // Bewertung: nicht durch den Spender, mit Begründung, öffentlich
    await expect(sp.spendeBewerten(db, mit.menschId, mit.agentId, auftrag_id, "annehmen", "selbst")).rejects.toThrow(/only the project's lead/);
    await sp.spendeBewerten(db, lead.menschId, lead.agentId, auftrag_id, "annehmen", "passt zum Styleguide");
    const zahlen = await sp.creditSpenden(db, (await k.projektPerSlug(db, "test")).id);
    expect(zahlen).toMatchObject({ angenommen: 1, credits_angenommen: 60 });
    expect(zahlen.zugesagt[0]).toMatchObject({ dienst: "meshy", credits: 100 });
    const arten = (await db.query("select art from ereignisse where bezug in ($1, $2) order by seq", [`zusage:${zusage_id}`, `spendenauftrag:${auftrag_id}`])).rows.map((r) => r.art);
    expect(arten).toEqual(["zusage_eingegangen", "spendenauftrag_angelegt", "spende_hochgeladen", "spende_bewertet"]);
    await sp.zusageBeenden(db, mit.menschId, zusage_id);
    expect((await sp.zusagenFuerLead(db, lead.menschId, "test")).length).toBe(0);
  });
});

describe("Zusage über den Runner (Medien)", () => {
  it("Runner bietet Dienste an, Lead beauftragt, Tageslimit, Upload nur begonnener Aufträge, Nacharbeit mit Grund", async () => {
    const sp = await import("../src/spenden.ts");
    const { Readable } = await import("node:stream");
    const { mkdtempSync } = await import("node:fs");
    const { tmpdir } = await import("node:os");
    const { join } = await import("node:path");
    process.env.SPENDEN_DIR = mkdtempSync(join(tmpdir(), "swx-medien-"));
    const runnerId = Number((await db.query("insert into agenten(mensch_id, client_id, name, runner) values ($1, 'runner-medien', 'swarvex runner (test)', true) returning id", [mit.menschId])).rows[0].id);
    const runner = { menschId: mit.menschId, agentId: runnerId, runner: true };
    await expect(sp.runnerZusagenSetzen(db, { ...mit, runner: false }, { dienste: ["meshy"] })).rejects.toThrow(/only the swarvex runner/);
    await expect(sp.runnerZusagenSetzen(db, runner, { dienste: ["meshy"], tageslimit: 31 })).rejects.toThrow(/daily limit/);
    expect(await sp.runnerZusagenSetzen(db, runner, { dienste: ["meshy", "elevenlabs"], tageslimit: 2 })).toMatchObject({ tageslimit: 2 });
    const zusagen = (await sp.zusagenFuerLead(db, lead.menschId, "test")).filter((z: any) => z.weg === "runner");
    expect(zusagen.map((z: any) => z.dienst).sort()).toEqual(["elevenlabs", "meshy"]);
    expect(zusagen[0]).toMatchObject({ alle_projekte: true, tageslimit: 2, heute_frei: 2, credits: null });
    const meshy = zusagen.find((z: any) => z.dienst === "meshy").zusage_id;
    const audio = zusagen.find((z: any) => z.dienst === "elevenlabs").zusage_id;
    const brief = { zusage_id: meshy, titel: "Drohne", anleitung: "low poly Drohne, PBR, unter 5000 Dreiecke", zielpfad: "asset_library/models/drohne.glb", dateitypen: ["glb"], credits: 30 };
    await expect(sp.auftragAnlegen(db, lead.menschId, brief)).rejects.toThrow(/name yours/);
    const b1 = (await sp.auftragAnlegen(db, lead.menschId, { ...brief, projekt: "test" })).auftrag_id;
    const b2 = (await sp.auftragAnlegen(db, lead.menschId, { ...brief, titel: "Schild", projekt: "test" })).auftrag_id;
    const b3 = (await sp.auftragAnlegen(db, lead.menschId, { ...brief, titel: "Wache", projekt: "test" })).auftrag_id;
    const b4 = (await sp.auftragAnlegen(db, lead.menschId, { ...brief, zusage_id: audio, titel: "Schritte", zielpfad: "audio/steps.ogg", dateitypen: ["ogg"], projekt: "test" })).auftrag_id;
    expect((await sp.runnerAuftraege(db, runner)).auftraege.map((a: any) => a.auftrag_id)).toEqual([b1, b2]);
    await expect(sp.runnerAuftraege(db, mit)).rejects.toThrow(/only the swarvex runner/);
    // Upload nur nach dem Beginnen; Beginnen zählt gegen das Tageslimit
    const anfrage = (b: Buffer) => Object.assign(Readable.from([b]), { headers: { "content-length": String(b.length) } }) as never;
    await expect(sp.hochladen(db, mit.menschId, b1, anfrage(Buffer.from("glTF")), "drohne.glb", runnerId)).rejects.toThrow(/start the brief first/);
    await sp.runnerAuftragBeginnen(db, runner, b1);
    await expect(sp.runnerAuftragBeginnen(db, runner, b1)).rejects.toThrow(/already started/);
    await sp.runnerAuftragBeginnen(db, runner, b2);
    await expect(sp.runnerAuftragBeginnen(db, runner, b3)).rejects.toThrow(/daily limit reached/);
    expect((await sp.runnerAuftraege(db, runner)).auftraege).toEqual([]);
    await expect(sp.runnerAuftragBeginnen(db, { ...runner, agentId: lead.agentId }, b3)).rejects.toThrow(/unknown brief/);
    const h = await sp.hochladen(db, mit.menschId, b1, anfrage(Buffer.from("glTF-Drohne")), "drohne.glb", runnerId);
    expect((await db.query("select zustand, begonnen from spendenauftraege where id = $1", [b1])).rows[0]).toMatchObject({ zustand: "hochgeladen", begonnen: null });
    expect(h.bytes).toBe(11);
    // Der Inhalt muss zur Endung passen (sonst bleibt nichts liegen); über den Runner keine Texte oder Archive
    await expect(sp.hochladen(db, mit.menschId, b2, anfrage(Buffer.from("-----BEGIN OPENSSH PRIVATE KEY-----")), "drohne.glb", runnerId)).rejects.toThrow(/content is not a \.glb/);
    expect((await db.query("select zustand from spendenauftraege where id = $1", [b2])).rows[0].zustand).toBe("offen");
    await expect(sp.auftragAnlegen(db, lead.menschId, { ...brief, titel: "Notiz", dateitypen: ["txt"], projekt: "test" })).rejects.toThrow(/file types via the runner/);
    expect(await sp.runnerAuftragAbbrechen(db, runner, b2, "Meshy antwortete nicht")).toEqual({ auftrag_id: b2, zustand: "offen" });
    // Nacharbeit kommt zuerst, mit der Begründung des Leads (Limit höher gestellt)
    await sp.spendeBewerten(db, lead.menschId, lead.agentId, b1, "nacharbeit", "Zu viele Dreiecke: unter 5000 bleiben");
    await sp.runnerZusagenSetzen(db, runner, { dienste: ["meshy", "elevenlabs"], tageslimit: 5 });
    const jetzt = (await sp.runnerAuftraege(db, runner)).auftraege;
    expect(jetzt[0]).toMatchObject({ auftrag_id: b1, zustand: "nacharbeit", begruendung: "Zu viele Dreiecke: unter 5000 bleiben", projekt: "test" });
    // b2 ist gescheitert: 24 Stunden Pause, der Lead sieht den Grund; danach wieder frei
    expect(jetzt.map((a: any) => a.auftrag_id)).toEqual([b1, b3, b4]);
    await expect(sp.runnerAuftragBeginnen(db, runner, b2)).rejects.toThrow(/handed back less than 24 hours/);
    expect((await sp.auftraegeFuerLead(db, lead.menschId, "test")).find((a: any) => a.auftrag_id === b2)).toMatchObject({ zustand: "offen", abbruchgrund: "Meshy antwortete nicht" });
    await db.query("update spendenauftraege set abgebrochen = now() - interval '25 hours' where id = $1", [b2]);
    expect((await sp.runnerAuftraege(db, runner)).auftraege.map((a: any) => a.auftrag_id)).toEqual([b1, b2, b3]);
    // Nur angehalten (kein Fehlschlag): gleich wieder frei
    await sp.runnerAuftragBeginnen(db, runner, b3);
    await sp.runnerAuftragAbbrechen(db, runner, b3, "runner was stopped", true);
    expect((await db.query("select begonnen, abgebrochen from spendenauftraege where id = $1", [b3])).rows[0]).toEqual({ begonnen: null, abgebrochen: null });
    // Dienst fällt weg: seine Zusage endet, offene Aufträge werden zurückgezogen
    await sp.runnerZusagenSetzen(db, runner, { dienste: ["meshy"], tageslimit: 5 });
    expect((await db.query("select zustand from spendenauftraege where id = $1", [b4])).rows[0].zustand).toBe("zurueckgezogen");
    expect((await sp.zusagenFuerLead(db, lead.menschId, "test")).filter((z: any) => z.weg === "runner").map((z: any) => z.dienst)).toEqual(["meshy"]);
    const arten = (await db.query("select art from ereignisse where bezug = $1 order by seq", [`spendenauftrag:${b1}`])).rows.map((r) => r.art);
    expect(arten).toEqual(["spendenauftrag_angelegt", "spendenauftrag_begonnen", "spende_hochgeladen", "spende_bewertet"]);
    await sp.runnerZusagenSetzen(db, runner, { dienste: [] });
  });
});

describe("Übersetzung von Vision und Ziel", () => {
  it("nur Lead oder Visionär, nicht in die Sprache des Originals, Original bleibt, im Protokoll", async () => {
    const v = Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea','uebersetzerin','uebersetzerin') returning id")).rows[0].id);
    const fremd = Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea','fremde','fremde') returning id")).rows[0].id);
    await projektAnlegen(db, { slug: "deutsches-spiel", name: "Deutsches Spiel", vision_md: "# Vision\nEin Spiel.", vision_short: "Ein kleines Spiel.", engine: "open",
      license_code: "MIT", license_assets: "CC BY 4.0", lead_model: "Codex · GPT", goal: "Erste Strecke", type: "game", language: "de" }, v);
    await expect(uebersetzungSetzen(db, "deutsches-spiel", fremd, "en", { goal: "First route" })).rejects.toThrow(/lead or the visionary/);
    await expect(uebersetzungSetzen(db, "deutsches-spiel", v, "de", { goal: "x" })).rejects.toThrow(/original is already/);
    await expect(uebersetzungSetzen(db, "deutsches-spiel", v, "fr", { goal: "x" })).rejects.toThrow(/language/);
    expect((await uebersetzungSetzen(db, "deutsches-spiel", v, "en", { vision_md: "# Vision\nA game.", vision_short: "A small game.", goal: "First route" })).fields.sort())
      .toEqual(["goal", "vision_md", "vision_short"]);
    // leeres Feld entfernt nur diesen Teil
    await uebersetzungSetzen(db, "deutsches-spiel", v, "en", { goal: "" });
    const p = (await db.query("select sprache, vision_kurz, ziel, uebersetzungen from projekte where slug = 'deutsches-spiel'")).rows[0];
    expect(p.sprache).toBe("de");
    expect(p.vision_kurz).toBe("Ein kleines Spiel.");
    expect(p.ziel).toBe("Erste Strecke");
    expect(p.uebersetzungen).toEqual({ en: { vision_md: "# Vision\nA game.", vision_short: "A small game." } });
    const ev = (await db.query("select count(*)::int n from ereignisse where art = 'uebersetzung_gesetzt'")).rows[0].n;
    expect(ev).toBe(2);
    expect(projektPruefen({ slug: "x-spiel", name: "X", vision_md: "", vision_short: "X.", engine: "open", license_code: "MIT", license_assets: "CC BY 4.0",
      lead_model: "a", goal: "b", language: "fr" }).join()).toMatch(/language/);
  });
});

describe("Projekt starten (Assistent)", () => {
  it("prüft Eingaben, begrenzt je Mensch, macht Visionär und Lead, bleibt Entwurf bis zur ersten Aufgabe", async () => {
    const m = Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea','visionaerin','visionaerin') returning id")).rows[0].id);
    const a = Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,'codex','codex') returning id", [m])).rows[0].id);
    const neu: NeuesProjekt = { slug: "mein-spiel", name: "Mein Spiel", vision_md: "", vision_short: "Ein kleines Spiel.", engine: "open", license_code: "GPL-3.0-or-later",
      license_assets: "CC BY-SA 4.0", lead_model: "Codex · GPT", goal: "Prototyp", type: "game" };
    await expect(projektAnlegen(db, { ...neu, license_code: "proprietary" }, m)).rejects.toThrow(/OSI/);
    await expect(projektAnlegen(db, { ...neu, slug: "A" }, m)).rejects.toThrow(/slug/);
    await expect(projektAnlegen(db, { ...neu, procedure: "clean_room", original: { name: "X" } }, m)).rejects.toThrow(/clean-room/);
    await expect(projektAnlegen(db, { ...neu, procedure: "clean_room", original: { name: "X", acquisition: "gekauft", depth: "inspect" } }, m)).rejects.toThrow(/legal_basis/);
    await expect(projektAnlegen(db, { ...neu, procedure: "clean_room", original: { name: "X", acquisition: "gekauft", target: "amiga" } }, m)).rejects.toThrow(/target/);
    await expect(projektAnlegen(db, { ...neu, slug: "test" }, m)).rejects.toThrow(/taken/);
    // nur beobachten: ohne Rechtsgrundlage und ohne Umfang zulässig
    expect(projektPruefen({ ...neu, procedure: "clean_room", original: { name: "X", acquisition: "gekauft", target: "windows", depth: "observe" } })).toEqual([]);
    // hineinsehen: welche Modelle die Werkzeug-Ausgaben sehen dürfen, ist Pflicht; bei Anbietern auch welche
    const innen = { name: "X", acquisition: "gekauft", target: "format_protocol", depth: "inspect", inspect: "nur das Dateiformat", legal_basis: "Interoperabilität" };
    expect(projektPruefen({ ...neu, procedure: "clean_room", original: innen }).join()).toMatch(/original.models/);
    expect(projektPruefen({ ...neu, procedure: "clean_room", original: { ...innen, models: "cloud" } }).join()).toMatch(/original.models/);
    expect(projektPruefen({ ...neu, procedure: "clean_room", original: { ...innen, models: "providers" } }).join()).toMatch(/original.providers/);
    expect(projektPruefen({ ...neu, procedure: "clean_room", original: { ...innen, models: "providers", providers: "Anthropic" } })).toEqual([]);
    expect(projektPruefen({ ...neu, procedure: "clean_room", original: { ...innen, models: "local" } })).toEqual([]);
    // Forschung: Art, Quelle und Erfolgskriterium sind Pflicht, Clean Room passt nicht dazu
    const forschung: NeuesProjekt = { ...neu, type: "research", research: { kind: "formal_proof", source: "Hardy & Wright, Theorem 366", success: "lake build green without sorry" } };
    expect(projektPruefen(forschung)).toEqual([]);
    expect(projektPruefen({ ...forschung, research: { kind: "alchemy", source: "x", success: "y" } }).join()).toMatch(/research.kind/);
    expect(projektPruefen({ ...forschung, research: { kind: "reproduction" } }).join()).toMatch(/research.source/);
    expect(projektPruefen({ ...forschung, procedure: "clean_room", original: { name: "X", acquisition: "gekauft" } }).join()).toMatch(/standard procedure/);
    // Weiterführen: Original-Repo und Zustand sind Pflicht, ohne Antwort auch der Link zur Anfrage; die Lizenz darf älter sein
    const weiter: NeuesProjekt = { ...neu, procedure: "continuation", license_code: "GPL-2.0-only", license_assets: "GPL-2.0-only",
      upstream: { repo: "https://github.com/alt/spiel", status: "archived" } };
    expect(projektPruefen(weiter)).toEqual([]);
    expect(projektPruefen({ ...weiter, procedure: "standard" }).join()).toMatch(/license_assets/);
    expect(projektPruefen({ ...weiter, upstream: { repo: "https://github.com/alt/spiel", status: "no_answer" } }).join()).toMatch(/upstream.contact/);
    expect(projektPruefen({ ...weiter, upstream: { repo: "ftp://x", status: "archived" } }).join()).toMatch(/upstream.repo/);
    expect(projektPruefen({ ...weiter, repo_url: "https://github.com/alt/spiel" }).join()).toMatch(/fork/);
    const r = await projektAnlegen(db, neu, m);
    const rollen = (await db.query("select rolle from projekt_rollen where projekt_id = $1 and mensch_id = $2 order by rolle", [r.projekt_id, m])).rows.map((x) => x.rolle);
    expect(rollen).toEqual(["lead", "visionaer"]);
    expect((await db.query("select projektart, vision_md from projekte where id = $1", [r.projekt_id])).rows[0]).toEqual({ projektart: "spiel", vision_md: "Ein kleines Spiel." });
    await expect(projektAnlegen(db, { ...neu, slug: "zweites-spiel" }, m)).rejects.toThrow(/per day/);
    expect((await projektListe(db)).projekte.map((p) => p.projekt)).not.toContain("mein-spiel");
    await k.aufgabeAnlegen(db, { menschId: m, agentId: a }, "mein-spiel", { ...neueAufgabe("Hauptmenü"), pfade: ["game/**"] });
    expect((await projektListe(db)).projekte.map((p) => p.projekt)).toContain("mein-spiel");
    // ohne verbundenes Repo bekommt der Runner nichts angeboten (kein Snapshot möglich)
    expect(await k.aufgabenPassend(db, { menschId: mit.menschId, agentId: mit.agentId, runner: true }, "mein-spiel")).toEqual([]);
    await db.query("update projekte set repo_git = 'https://github.com/x/mein-spiel.git' where slug = 'mein-spiel'");
    expect((await k.aufgabenPassend(db, { menschId: mit.menschId, agentId: mit.agentId, runner: true }, "mein-spiel")).length).toBe(1);
    await expect(repoVerbinden(db, "mein-spiel", mit.menschId, "https://github.com/x/y")).rejects.toThrow(/only the visionary/);
    await expect(repoVerbinden(db, "mein-spiel", m, "https://github.com/x/y")).rejects.toThrow(/sign in with GitHub/);
  });
});

describe("Bewertung durch den Schwarm", () => {
  const geprueft = async (titel: string, stufe: "A" | "B") => {
    const a = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe(titel), stufe });
    await k.reservieren(db, mit, a.id);
    const e: any = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "fertig", eigene_tests: "ok", verbrauch: { tokens: 10 } });
    await k.pruefungStarten(db, e.einreichung_id);
    await k.pruefungMelden(db, e.einreichung_id, "ok", "grün", "sig");
    return { a, eid: Number(e.einreichung_id) };
  };
  const pruefer = async (login: string, stufe = 1) => {
    const m = Number((await db.query("insert into menschen(konto_provider, konto_id, login, vertrauensstufe) values ('gitea',$1,$1,$2) returning id", [login, stufe])).rows[0].id);
    const ag = Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,$2,$2) returning id", [m, `${login}-agent`])).rows[0].id);
    return { menschId: m, agentId: ag };
  };
  it("Stufe A: nach zwei unabhängigen Zustimmungen automatisch angenommen; Selbst, Lead und Neulinge prüfen nicht", async () => {
    const { fremdPruefen, offeneFremdpruefungen } = await import("../src/schwarmpruefung.ts");
    const { a, eid } = await geprueft("Fremd geprüft", "A");
    const p1 = await pruefer("pruefer-1"), p2 = await pruefer("pruefer-2"), neu = await pruefer("neuling", 0);
    expect((await offeneFremdpruefungen(db, p1, "test")).map((x) => x.einreichung_id)).toContain(eid);
    expect((await offeneFremdpruefungen(db, mit, "test")).map((x) => x.einreichung_id)).not.toContain(eid);
    await expect(fremdPruefen(db, mit, eid, "zustimmen", "ich finde meine Arbeit gut genug")).rejects.toThrow(/own submission/);
    await expect(fremdPruefen(db, lead, eid, "zustimmen", "als Lead prüfe ich anders, nicht so")).rejects.toThrow(/review_submission/);
    await expect(fremdPruefen(db, neu, eid, "zustimmen", "sieht für mich vollkommen gut aus")).rejects.toThrow(/trust level/);
    // Nennt der Prüfer sein Modell, kommen Einreichungen eines anderen Modells zuerst
    const { eid: eid2 } = await geprueft("Fremd geprüft, anderes Modell", "A");
    await db.query("update einreichungen set modell = 'Modell Eins' where id = $1", [eid]);
    await db.query("update einreichungen set modell = 'modell-zwei' where id = $1", [eid2]);
    const reihe = (await offeneFremdpruefungen(db, p1, "test", "Modell Zwei")).map((x) => x.einreichung_id);
    expect(reihe.indexOf(eid)).toBeLessThan(reihe.indexOf(eid2));
    expect((await offeneFremdpruefungen(db, p1, "test", "modell-eins")).map((x) => x.einreichung_id).indexOf(eid2)).toBeLessThan(
      (await offeneFremdpruefungen(db, p1, "test", "modell-eins")).map((x) => x.einreichung_id).indexOf(eid));
    expect(await fremdPruefen(db, p1, eid, "zustimmen", "Akzeptanz erfüllt, Pfade passen, Tests grün", "Modell Zwei")).toMatchObject({ zustimmungen: 1, automatisch_angenommen: false });
    await expect(fremdPruefen(db, p1, eid, "zustimmen", "noch einmal dieselbe Zustimmung")).rejects.toThrow(/already reviewed/);
    expect(await fremdPruefen(db, p2, eid, "zustimmen", "Geprüft: Effekt korrekt, keine Nebenwirkungen")).toMatchObject({ zustimmungen: 2, automatisch_angenommen: true });
    expect((await db.query("select zustand from aufgaben where id = $1", [a.id])).rows[0].zustand).toBe("angenommen");
    const b = (await db.query("select b.urteil, ag.mensch_id::int as m from bewertungen b join agenten ag on ag.id = b.agent_id where b.einreichung_id = $1", [eid])).rows[0];
    expect(b).toEqual({ urteil: "annehmen", m: p2.menschId });   // vier Augen: ein anderer Mensch als der Einreicher
  });
  it("ab Stufe B und bei Einwänden entscheidet der Lead; Merge-Konflikt schickt zurück in den Pool", async () => {
    const { fremdPruefen } = await import("../src/schwarmpruefung.ts");
    const { a, eid } = await geprueft("Stufe B", "B");
    const p3 = await pruefer("pruefer-3"), p4 = await pruefer("pruefer-4");
    await fremdPruefen(db, p3, eid, "zustimmen", "Akzeptanz erfüllt und sauber umgesetzt");
    expect(await fremdPruefen(db, p4, eid, "zustimmen", "Auch von mir: passt genau zur Aufgabe")).toMatchObject({ automatisch_angenommen: false });
    const wartend = (await k.wartendeEinreichungen(db, lead, "test")).find((x) => Number(x.einreichung_id) === eid);
    expect(wartend.fremdpruefungen.map((f: { von: string }) => f.von)).toEqual(["pruefer-3", "pruefer-4"]);
    await k.bewerten(db, lead, eid, "annehmen", "mit den Fremdprüfungen bestätigt");
    const kopf = "a".repeat(40);
    await k.konfliktMelden(db, eid, kopf, "patch does not apply");
    expect((await db.query("select zustand, basis_commit from aufgaben where id = $1", [a.id])).rows[0]).toEqual({ zustand: "offen", basis_commit: kopf });
    await k.schliessen(db, lead, a.id, "Test");
  });
  it("Merge-Zug rot: zurück in den Pool mit neuem Stand, der nächste Versuch bekommt den Bericht", async () => {
    const { a, eid } = await geprueft("Zug rot", "A");
    await expect(k.zugRotMelden(db, eid, "b".repeat(40), "FAIL x", "sig")).rejects.toThrow(/task is/);   // noch nicht angenommen
    await k.bewerten(db, lead, eid, "annehmen", "passt, ab in den Zug damit");
    await expect(k.zugRotMelden(db, eid, "kein-hash", "FAIL x", "sig")).rejects.toThrow(/head/);
    const kopf = "b".repeat(40);
    await k.zugRotMelden(db, eid, kopf, "Merge train: 3 accepted contributions\nFAIL headless pause_test", "sig");
    expect((await db.query("select zustand, basis_commit from aufgaben where id = $1", [a.id])).rows[0]).toEqual({ zustand: "offen", basis_commit: kopf });
    const d: any = await k.details(db, mit, a.id);
    expect(d.letzte_pruefung?.bericht).toContain("FAIL headless pause_test");
    const ev = (await db.query("select daten from ereignisse where bezug = $1 and art = 'merge_zug_rot'", [`aufgabe:${a.id}`])).rows[0].daten;
    expect(ev).toMatchObject({ von: "angenommen", nach: "offen", neue_basis: kopf });
    await k.schliessen(db, lead, a.id, "Test");
  });
  it("Varianten: wird eine angenommen, schließen sich die nicht begonnenen; Nacharbeit-Muster bündeln Gründe je Rolle", async () => {
    const v: any = await k.variantenAnlegen(db, lead, "test", { ...neueAufgabe("Klickgeräusch"), stufe: "B", rolle: "audio" }, 3);
    expect(v.varianten).toHaveLength(3);
    expect(v.folgeaufgaben_mit).toEqual({ depends_on_any: [v.varianten] });
    const [v1, v2, v3] = v.varianten;
    await k.reservieren(db, mit, v1);
    const e: any = await k.einreichen(db, mit, v1, { patch: PATCH_OK, zusammenfassung: "Variante 1", eigene_tests: "ok", verbrauch: { tokens: 10 } });
    await k.pruefungStarten(db, e.einreichung_id);
    await k.pruefungMelden(db, e.einreichung_id, "ok", "grün", "sig");
    await k.reservieren(db, fremd, v2);   // begonnen: läuft weiter bis zur Bewertung
    await k.bewerten(db, lead, e.einreichung_id, "annehmen", "diese Variante passt am besten");
    const z = async (id: number) => (await db.query("select zustand from aufgaben where id = $1", [id])).rows[0].zustand;
    expect([await z(v1), await z(v2), await z(v3)]).toEqual(["angenommen", "reserviert", "geschlossen"]);
    await k.abbrechen(db, fremd, v2, "Test");
    await k.schliessen(db, lead, v2, "Test");
    const { nacharbeitMuster } = await import("../src/lernen.ts");
    const n: any = await nacharbeitMuster(db, lead, "test", 30);
    expect(n.gesamt).toBeGreaterThanOrEqual(0);
    await expect(nacharbeitMuster(db, fremd, "test", 30)).rejects.toThrow(/lead/);
  });
  it("eigene D-Arbeit des Leads: nur Lead, nur Stufe D, mit Commit; schaltet Folgeaufgaben frei; kritischer Pfad zuerst", async () => {
    const d = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Eigene Architektur"), stufe: "D" });
    const f1 = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Folge 1"), abhaengig_von: [Number(d.id)] });
    await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Folge 2"), abhaengig_von: [Number(f1.id)] });
    const einzeln = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Ohne Folgen"));
    await expect(k.leadErledigt(db, mit, d.id, "abc1234", "habe das selbst gebaut")).rejects.toThrow(/only the project's lead/);
    await expect(k.leadErledigt(db, lead, einzeln.id, "abc1234", "keine D-Aufgabe, nicht so")).rejects.toThrow(/level D/);
    await expect(k.leadErledigt(db, lead, d.id, "kein-hash", "Architektur steht jetzt")).rejects.toThrow(/commit/);
    // vorher: D schaltet zwei Aufgaben frei und steht vor der Aufgabe ohne Folgen
    // Ein neuer Agent bekommt D nicht angeboten (Zuteilung nach nachgewiesener Stufe); der Lead sieht die Reihenfolge
    expect((await k.aufgabenPassend(db, mit, "test")).map((x) => Number(x.id))).not.toContain(Number(d.id));
    const vorher = await k.aufgabenPassend(db, lead, "test");
    const dZeile = vorher.find((x) => Number(x.id) === Number(d.id))!;
    expect(dZeile.folgeaufgaben).toBe(2);
    expect(vorher.findIndex((x) => Number(x.id) === Number(d.id))).toBeLessThan(vorher.findIndex((x) => Number(x.id) === Number(einzeln.id)));
    await k.leadErledigt(db, lead, d.id, "abc1234", "Architektur steht in docs/architektur.md");
    expect((await db.query("select zustand, ohne_fremdbewertung from aufgaben where id = $1", [d.id])).rows[0]).toEqual({ zustand: "gemerged", ohne_fremdbewertung: true });
    expect((await k.aufgabenPassend(db, mit, "test")).map((x) => Number(x.id))).toContain(Number(f1.id));
  });
});

describe("Zustandsmaschine", () => {
  it("läuft einmal komplett durch und protokolliert jeden Schritt", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Durchlauf"));
    const passend = await k.aufgabenPassend(db, mit, "test", ["code.gdscript"]);
    expect(passend.map((x) => Number(x.id))).toContain(Number(a.id));
    await k.reservieren(db, mit, a.id);
    await k.fortschritt(db, mit, a.id, "angefangen", { tokens: 1200 });
    const e = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "fertig", eigene_tests: "ok", verbrauch: { tokens: 5000 } });
    expect(e).toHaveProperty("einreichung_id");
    const eid = (e as any).einreichung_id;
    await expect(k.bewerten(db, lead, eid, "annehmen", "gut")).rejects.toThrow(/after a passing check/);
    await k.pruefungStarten(db, eid);
    await k.pruefungMelden(db, eid, "ok", "alle Tests grün", "sig");
    await expect(k.bewerten(db, mit, eid, "annehmen", "selbst")).rejects.toThrow(/lead/);
    // Der Lead mit einem zweiten Agenten: auch das ist eine Selbstbewertung, falls er der Autor wäre.
    await db.query("insert into projekt_rollen values ((select id from projekte where slug='test'), $1, 'lead') on conflict do nothing", [mit.menschId]);
    await expect(k.bewerten(db, { menschId: mit.menschId, agentId: lead.agentId }, eid, "annehmen", "selbst")).rejects.toThrow(/own submission/);
    await db.query("delete from projekt_rollen where mensch_id = $1 and rolle = 'lead'", [mit.menschId]);
    await k.bewerten(db, lead, eid, "annehmen", "sauber umgesetzt");
    await k.gemergedMelden(db, eid, "abc123");
    const zustand = (await db.query("select zustand from aufgaben where id = $1", [a.id])).rows[0].zustand;
    expect(zustand).toBe("gemerged");
    const arten = (await db.query("select art from ereignisse where bezug = $1 order by seq", [`aufgabe:${a.id}`])).rows.map((r) => r.art);
    expect(arten).toEqual(["aufgabe_angelegt", "aufgabe_reserviert", "arbeit_begonnen", "ergebnis_eingereicht", "pruefung_gestartet", "pruefung_beendet", "bewertet", "gemerged"]);
  });

  it("verbietet unerlaubte Übergänge und fremde Leases", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Fremd"));
    await expect(k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "", eigene_tests: "", verbrauch: {} })).rejects.toThrow(/not reserved for you/);
    await k.reservieren(db, mit, a.id);
    await expect(k.reservieren(db, fremd, a.id)).rejects.toThrow(/reserved/);
    await expect(k.fortschritt(db, fremd, a.id, "x", {})).rejects.toThrow(/not reserved for you/);
    await expect(k.aufgabeAnlegen(db, mit, "test", neueAufgabe("kein Lead"))).rejects.toThrow(/lead/);
  });

  it("gibt abgelaufene Reservierungen zurück in den Pool", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Ablauf"));
    await k.reservieren(db, mit, a.id);
    await db.query("update leases set laeuft_ab = now() - interval '1 minute' where aufgabe_id = $1", [a.id]);
    expect(await k.leasesAufraeumen(db)).toBeGreaterThanOrEqual(1);
    expect((await db.query("select zustand from aufgaben where id = $1", [a.id])).rows[0].zustand).toBe("offen");
    await expect(k.fortschritt(db, mit, a.id, "zu spät", {})).rejects.toThrow();
  });

  it("schickt eine gescheiterte Prüfung automatisch zurück in den Pool", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Rot"));
    await k.reservieren(db, mit, a.id);
    const e = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "", eigene_tests: "", verbrauch: {} }) as any;
    await k.pruefungStarten(db, e.einreichung_id);
    await k.pruefungMelden(db, e.einreichung_id, "fehler", "Test rot", "sig");
    expect((await db.query("select zustand from aufgaben where id = $1", [a.id])).rows[0].zustand).toBe("offen");
    // Der nächste Versuch bekommt den Bericht, damit er den Fehler beheben kann
    const d = await k.details(db, mit, a.id) as any;
    expect(d.letzte_pruefung).toMatchObject({ ergebnis: "fehler", bericht: "Test rot" });
  });

  it("Betreiber-Ausnahme: eine nur an roten Projekt-Tests gescheiterte Einreichung doch bewerten", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Nachträglich"));
    await k.reservieren(db, mit, a.id);
    const e = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "", eigene_tests: "", verbrauch: {} }) as any;
    await k.pruefungStarten(db, e.einreichung_id);
    await expect(k.zurBewertungFreigeben(db, e.einreichung_id, "Projekt-Tests waren schon ohne Beitrag rot")).rejects.toThrow(/only an open task/);
    await k.pruefungMelden(db, e.einreichung_id, "fehler", "rot", "sig");
    await expect(k.zurBewertungFreigeben(db, e.einreichung_id, "kurz")).rejects.toThrow(/at least 20/);
    await k.zurBewertungFreigeben(db, e.einreichung_id, "Projekt-Tests waren schon ohne Beitrag rot");
    expect((await db.query("select zustand from aufgaben where id = $1", [a.id])).rows[0].zustand).toBe("geprueft_ok");
    const ev = (await db.query("select art, daten from ereignisse where bezug = $1 order by seq desc limit 1", [`aufgabe:${a.id}`])).rows[0];
    expect(ev).toMatchObject({ art: "zur_bewertung_freigegeben", daten: { einreichung: e.einreichung_id } });
    await k.bewerten(db, lead, e.einreichung_id, "annehmen", "passt");
    expect((await db.query("select zustand from aufgaben where id = $1", [a.id])).rows[0].zustand).toBe("angenommen");
  });

  it("überholte Einreichungen: nicht in der Liste des Leads, nicht bewertbar", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Zwei Versuche"));
    await k.reservieren(db, mit, a.id);
    const alt = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "", eigene_tests: "", verbrauch: {} }) as any;
    await k.pruefungStarten(db, alt.einreichung_id);
    await k.pruefungMelden(db, alt.einreichung_id, "fehler", "rot", "sig");
    await k.reservieren(db, mit, a.id);
    const neu = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "", eigene_tests: "", verbrauch: {} }) as any;
    await k.pruefungStarten(db, neu.einreichung_id);
    await k.pruefungMelden(db, neu.einreichung_id, "ok", "grün", "sig");
    const liste = (await k.wartendeEinreichungen(db, lead, "test") as any[]).filter((x) => Number(x.aufgabe_id) === a.id);
    expect(liste.map((x) => Number(x.einreichung_id))).toEqual([neu.einreichung_id]);
    await expect(k.bewerten(db, lead, alt.einreichung_id, "annehmen", "alt")).rejects.toThrow(/superseded/);
    await k.bewerten(db, lead, neu.einreichung_id, "annehmen", "neu passt");
  });

  it("Nacharbeit: der nächste Versuch bekommt die Begründung des Leads", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Nacharbeit mit Grund"));
    await k.reservieren(db, mit, a.id);
    const e = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "", eigene_tests: "", verbrauch: {} }) as any;
    await k.pruefungStarten(db, e.einreichung_id);
    await k.pruefungMelden(db, e.einreichung_id, "ok", "grün", "sig");
    expect((await k.details(db, mit, a.id) as any).letzte_bewertung).toBeNull();
    await k.bewerten(db, lead, e.einreichung_id, "nacharbeit", "Die Farben fehlen: nimm die Palette aus docs/art.md");
    const d = await k.details(db, mit, a.id) as any;
    expect(d.letzte_bewertung).toMatchObject({ urteil: "nacharbeit", begruendung: "Die Farben fehlen: nimm die Palette aus docs/art.md" });
    expect(d.letzte_pruefung).toBeNull();
  });

  it("Nacharbeit geht 24 Stunden an die Person zurück, die die Aufgabe gemacht hat", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Nacharbeit mit Vorrang"));
    await k.reservieren(db, mit, a.id);
    const e = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "", eigene_tests: "", verbrauch: {} }) as any;
    await k.pruefungStarten(db, e.einreichung_id);
    await k.pruefungMelden(db, e.einreichung_id, "ok", "grün", "sig");
    await k.bewerten(db, lead, e.einreichung_id, "nacharbeit", "Die Überschrift fehlt in Abschnitt 2");
    const ids = async (w: typeof mit) => (await k.aufgabenPassend(db, w, "test") as any[]).map((x) => Number(x.id));
    expect(await ids(fremd)).not.toContain(a.id);
    await expect(k.reservieren(db, fremd, a.id)).rejects.toThrow(/held for its contributor's rework/);
    const eigene = await k.aufgabenPassend(db, mit, "test") as any[];
    expect(eigene.find((x) => Number(x.id) === a.id)?.nacharbeit_bis).toBeTruthy();
    // eigene Nacharbeit zuerst (hier auch die aus dem Test davor)
    const erste = eigene.findIndex((x) => !x.nacharbeit_bis);
    expect(eigene.slice(erste).every((x) => !x.nacharbeit_bis)).toBe(true);
    expect(erste).toBeGreaterThan(0);
    // eine Chance: nach der ersten neuen Reservierung (hier abgebrochen) ist die Aufgabe wieder für alle offen
    await k.reservieren(db, mit, a.id);
    await k.abbrechen(db, mit, a.id, "schaffe ich nicht");
    expect(await ids(fremd)).toContain(a.id);
    // nach 24 Stunden ebenso
    const b = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Nacharbeit abgelaufen"));
    await k.reservieren(db, mit, b.id);
    const e2 = await k.einreichen(db, mit, b.id, { patch: PATCH_OK, zusammenfassung: "", eigene_tests: "", verbrauch: {} }) as any;
    await k.pruefungStarten(db, e2.einreichung_id);
    await k.pruefungMelden(db, e2.einreichung_id, "ok", "grün", "sig");
    await k.bewerten(db, lead, e2.einreichung_id, "nacharbeit", "Die Quelle fehlt");
    expect(await ids(fremd)).not.toContain(b.id);
    await db.query("update bewertungen set erstellt = now() - interval '25 hours' where einreichung_id = $1", [e2.einreichung_id]);
    expect(await ids(fremd)).toContain(b.id);
    await k.reservieren(db, fremd, b.id);
  });

  it("Gegenprobe: Projekt-Tests schon ohne den Beitrag rot → Bewertung statt Pool, eine Nachricht an den Lead", async () => {
    const n = await import("../src/nachrichten.ts");
    const vorher = Number((await db.query("select count(*) from nachrichten where von_mensch is null")).rows[0].count);
    for (const titel of ["Basis rot 1", "Basis rot 2"]) {
      const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe(titel));
      await k.reservieren(db, mit, a.id);
      const e = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "", eigene_tests: "", verbrauch: {} }) as any;
      await k.pruefungStarten(db, e.einreichung_id);
      await k.pruefungMelden(db, e.einreichung_id, "ok", "=== Counter-check without the contribution: failed too\nFAIL nav_test", "sig", "projekt", true);
      expect((await db.query("select zustand from aufgaben where id = $1", [a.id])).rows[0].zustand).toBe("geprueft_ok");
      expect((await k.details(db, mit, a.id) as any).letzte_pruefung).toBeNull();
    }
    // höchstens eine je Tag und Projekt
    expect(Number((await db.query("select count(*) from nachrichten where von_mensch is null")).rows[0].count)).toBe(vorher + 1);
    const post = await n.posteingang(db, lead, "test") as any[];
    const info = post.find((x) => x.betreff === k.BETREFF_TESTS_ROT);
    expect(info).toMatchObject({ von: "swarvex", an: "lead", nachricht_art: "info" });
    expect(info.inhalt).toMatch(/FAIL nav_test/);
  });
});

describe("Pfadprüfung (S5)", () => {
  it("weist Patches außerhalb der erlaubten Pfade ab und protokolliert das", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Pfade"));
    await k.reservieren(db, mit, a.id);
    const boese = PATCH_OK.replaceAll("game/modules/vfx/neu.gd", "tools/run_tests.sh");
    const r = await k.einreichen(db, mit, a.id, { patch: boese, zusammenfassung: "", eigene_tests: "", verbrauch: {} });
    expect(r).toHaveProperty("abgewiesen");
    expect(k.patchPruefen(PATCH_OK.replaceAll("game/modules/vfx/neu.gd", "game/modules/vfx/../../../etc/x"), ["game/modules/vfx/**"]).join()).toMatch(/invalid path/);
    const art = (await db.query("select art from ereignisse where bezug = $1 order by seq desc limit 1", [`aufgabe:${a.id}`])).rows[0].art;
    expect(art).toBe("einreichung_abgewiesen");
  });
});

describe("Clean Room (E-26)", () => {
  it("trennt Analyse und Bau pro Mensch und belegt es im Protokoll", async () => {
    const cr = Number((await db.query(
      `insert into projekte(slug,name,vision_md,vision_kurz,engine,lizenz_code,lizenz_assets,repo_url,lead_modell,ziel,verfahren,vorbild)
       values ('cr','CR','Vision','kurz','godot-4.7','MIT','CC BY 4.0','http://x','Claude','Demo','clean_room',$1) returning id`,
      [{ name: "Vorbild", rechtsgrundlage: "eigene Lizenz, Analyse zur Interoperabilität" }])).rows[0].id);
    await db.query("insert into projekt_rollen values ($1,$2,'lead')", [cr, lead.menschId]);
    const basis = { beschreibung: "x", stufe: "B" as const, faehigkeiten: ["spec"], akzeptanz: ["ok"] };
    // Raum ist Pflicht, Pfade passen zum Raum
    await expect(k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "ohne Raum", pfade: ["specification/a.md"] })).rejects.toThrow(/needs a room/);
    await expect(k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Analyse falsch", raum: "analyse", pfade: ["src/**"] })).rejects.toThrow(/only write under specification/);
    await expect(k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Bau falsch", raum: "bau", pfade: ["**"] })).rejects.toThrow(/must not change/);
    await expect(k.aufgabeAnlegen(db, lead, "test", { ...basis, titel: "Raum ohne CR", raum: "bau", pfade: ["game/**"] })).rejects.toThrow(/only exist in clean-room/);
    const analyse = await k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Speichern beschreiben", raum: "analyse", pfade: ["specification/**"] });
    const bau1 = await k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Speichern bauen", raum: "bau", pfade: ["src/**"] });
    const bau2 = await k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Laden bauen", raum: "bau", pfade: ["src/**"] });
    const analyse2 = await k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Laden beschreiben", raum: "analyse", pfade: ["specification/**"] });

    await k.reservieren(db, mit, analyse.id);                               // mit analysiert …
    await expect(k.reservieren(db, mit, bau1.id)).rejects.toThrow(/clean room/);   // … und darf nicht bauen
    expect((await k.aufgabenPassend(db, mit, "cr")).map((a) => a.raum)).not.toContain("bau");
    await k.reservieren(db, fremd, bau1.id);                               // fremd baut …
    await expect(k.reservieren(db, fremd, analyse2.id)).rejects.toThrow(/clean room/); // … und darf nicht analysieren
    await k.reservieren(db, fremd, bau2.id);                               // weiter bauen geht

    const verweigert = (await db.query("select count(*)::int as n from ereignisse where art = 'clean_room_trennung' and projekt_id = $1", [cr])).rows[0].n;
    expect(verweigert).toBe(2);
    const { projektDetail } = await import("../src/kennzahlen.ts");
    const d = await projektDetail(db, "cr");
    expect(d.clean_room?.trennung_eingehalten).toBe(true);
    expect(d.clean_room?.menschen_je_raum).toMatchObject({ analyse: 1, bau: 1 });

    // Officers im Clean Room: genau ein Raum, die Trennwand gilt auch für das Amt
    const vis = Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea','crvis','crvis') returning id")).rows[0].id);
    const visAgent = Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,'crvis-agent','crvis-agent') returning id", [vis])).rows[0].id);
    await db.query("insert into projekt_rollen values ($1,$2,'visionaer')", [cr, vis]);
    const v = { menschId: vis, agentId: visAgent };
    await db.query("update menschen set vertrauensstufe = greatest(vertrauensstufe, 2) where login in ('fremd', 'mitstreiterin')");
    await expect(k.officerErnennen(db, v, "cr", "fremd", "analyst", ["specification/drafts/**"], ["text.*"])).rejects.toThrow(/needs a room/);
    await expect(k.officerErnennen(db, v, "cr", "fremd", "analyst", ["specification/drafts/**"], ["text.*"], "analyse")).rejects.toThrow(/fremd has already worked in room "build"/);
    await k.officerErnennen(db, v, "cr", "mitstreiterin", "analyst", ["specification/drafts/**"], ["spec"], "analyse");
    const o = { menschId: mit.menschId, agentId: mit.agentId };
    await k.aufgabeAnlegen(db, o, "cr", { ...basis, titel: "Format beschreiben", stufe: "A", raum: "analyse", pfade: ["specification/drafts/format.md"] });
    await expect(k.aufgabeAnlegen(db, o, "cr", { ...basis, titel: "Prüfen", stufe: "A", raum: "pruefung", pfade: ["specification/drafts/format.md"] })).rejects.toThrow(/area lead's room/);
    await expect(k.officerErnennen(db, v, "test", "fremd", "audio", ["game/audio/**"], ["asset.audio"], "bau")).rejects.toThrow(/visionary|only exist in clean-room/);
    await k.officerBeenden(db, v, "cr", "analyst", "Test");
  });
});

describe("Meldungen und Sperren (#19, #29)", () => {
  it("nimmt Meldungen an, prüft sie und protokolliert ohne persönliche Daten", async () => {
    const { meldungAnlegen, meldungenListe, meldungAbschliessen } = await import("../src/meldungen.ts");
    const gut = { kind: "infringement", project: "test", location: "https://example.org/datei", description: "Diese Datei stammt aus meinem Spiel und wurde ohne Erlaubnis übernommen.",
      name: "Rita Rechte", email: "rita@example.org", rights_holder: true, confirmed: true };
    const { id } = await meldungAnlegen(db, gut);
    await expect(meldungAnlegen(db, { ...gut, confirmed: false })).rejects.toThrow();
    await expect(meldungAnlegen(db, { ...gut, description: "zu kurz" })).rejects.toThrow();
    await expect(meldungAnlegen(db, { ...gut, website: "spam" })).rejects.toThrow();
    expect((await meldungenListe(db)).map((m) => Number(m.id))).toContain(id);
    const ereignis = (await db.query("select daten from ereignisse where bezug = $1", [`meldung:${id}`])).rows[0].daten;
    expect(JSON.stringify(ereignis)).not.toMatch(/rita|Rita|example\.org/);
    await meldungAbschliessen(db, id, "erledigt", "Datei entfernt");
    expect((await meldungenListe(db)).map((m) => Number(m.id))).not.toContain(id);
  });

  it("sperrt Projekte und Menschen und hebt Sperren wieder auf", async () => {
    const { sperren } = await import("../src/meldungen.ts");
    const { projektListe } = await import("../src/kennzahlen.ts");
    const pid = Number((await db.query("select id from projekte where slug = 'test'")).rows[0].id);
    const offen = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Sperrtest"));
    await k.reservieren(db, mit, offen.id);
    await sperren(db, { projekt: pid }, true, "Takedown wegen Meldung");
    // Die Reservierung endet, die Aufgabe ist wieder frei (nicht in „reserviert“ hängen geblieben)
    expect((await db.query("select zustand from aufgaben where id = $1", [offen.id])).rows[0].zustand).toBe("offen");
    expect((await db.query("select count(*)::int as n from leases where aufgabe_id = $1 and beendet is null", [offen.id])).rows[0].n).toBe(0);
    await expect(k.aufgabenPassend(db, mit, "test")).rejects.toThrow(/blocked/);
    await expect(k.reservieren(db, mit, offen.id)).rejects.toThrow(/blocked/);
    expect((await projektListe(db)).projekte.map((x) => x.projekt)).not.toContain("test");
    await sperren(db, { projekt: pid }, false, "Meldung erledigt");
    expect((await k.aufgabenPassend(db, mit, "test")).map((a) => a.id)).toContain(offen.id);

    await sperren(db, { mensch: fremd.menschId }, true, "Spam-Einreichungen");
    await expect(k.reservieren(db, fremd, offen.id)).rejects.toThrow(/account blocked/);
    await sperren(db, { mensch: fremd.menschId }, false, "Einspruch berechtigt");
    const arten = (await db.query("select art from ereignisse where art like '%gesperrt' or art like '%entsperrt' order by seq")).rows.map((r) => r.art);
    expect(arten).toEqual(["projekt_gesperrt", "projekt_entsperrt", "mensch_gesperrt", "mensch_entsperrt"]);
  });
});

describe("Mein Konto (#25)", () => {
  it("zeigt eigene Agenten und Beiträge", async () => {
    const { kontoDaten } = await import("../src/konto.ts");
    const d = await kontoDaten(db, mit.menschId);
    expect(d.mensch.login).toBe("mitstreiterin");
    expect(d.agenten.map((a) => a.name)).toContain("runner-1");
    expect(d.summe.angenommen).toBeGreaterThanOrEqual(1);
    expect(d.summe.tokens).toBeGreaterThanOrEqual(5000);
    expect(d.beitraege.every((b) => b.projekt === "test")).toBe(true);
    // was entstanden ist: die Zustandsmaschine reicht PATCH_OK ein (eine neue GDScript-Datei, zwei Zeilen) und merged ihn
    const gemerged = d.beitraege.find((b) => b.commit);
    expect(gemerged?.dateien[0]).toMatchObject({ pfad: "game/modules/vfx/neu.gd", plus: 2, minus: 0, sorte: "code" });
    expect(d.summe.zeilen_code).toBeGreaterThanOrEqual(2);
    expect(d.modelle.length).toBeGreaterThanOrEqual(1);
    expect((await kontoDaten(db, fremd.menschId)).beitraege).toEqual([]);
  });

  it("meldet einen Agenten ab: Grants weg, Reservierung beendet, Aufgabe wieder offen", async () => {
    const { agentAbmelden } = await import("../src/konto.ts");
    const zweit = { menschId: mit.menschId, agentId: Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,'runner-3','runner-3') returning id", [mit.menschId])).rows[0].id) };
    const grant = async (id: string, m: number, client: string) => {
      await db.query("insert into oidc_payloads(id, typ, payload) values ($1, 'Grant', $2)", [id, { accountId: String(m), clientId: client }]);
      await db.query("insert into oidc_payloads(id, typ, payload, grant_id) values ($1, 'RefreshToken', $2, $3)", [`rt-${id}`, { grantId: id }, id]);
    };
    await grant("g-mit", mit.menschId, "runner-3");
    await grant("g-fremd", fremd.menschId, "runner-3");
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Abmeldetest"));
    await k.reservieren(db, zweit, a.id);

    await expect(agentAbmelden(db, fremd.menschId, zweit.agentId)).rejects.toThrow(/unknown/);
    expect(await agentAbmelden(db, mit.menschId, zweit.agentId)).toEqual({ reservierungen: 1 });
    const rest = (await db.query("select id from oidc_payloads order by id")).rows.map((r) => r.id);
    expect(rest).toEqual(["g-fremd", "rt-g-fremd"]);
    expect((await db.query("select abgemeldet from agenten where id = $1", [zweit.agentId])).rows[0].abgemeldet).not.toBeNull();
    expect((await db.query("select zustand from aufgaben where id = $1", [a.id])).rows[0].zustand).toBe("offen");
    expect((await db.query("select count(*)::int as n from ereignisse where art = 'agent_abgemeldet'")).rows[0].n).toBe(1);
  });

  it("Website-Sitzung: signiert, endet mit „Abmelden“ und bei Sperre", async () => {
    const { sitzungAusstellen, sitzungLesen } = await import("../src/konto.ts");
    const g = "geheim";
    const s = sitzungAusstellen(g, mit.menschId);
    expect(await sitzungLesen(db, g, s)).toBe(mit.menschId);
    expect(await sitzungLesen(db, g, s.replace(/^\d+/, String(fremd.menschId)))).toBeUndefined();
    expect(await sitzungLesen(db, "anderes", s)).toBeUndefined();
    expect(await sitzungLesen(db, g, sitzungAusstellen(g, mit.menschId, Date.now() - 15 * 86_400_000))).toBeUndefined();
    await db.query("update menschen set sitzungen_ab = now() where id = $1", [mit.menschId]);
    expect(await sitzungLesen(db, g, s)).toBeUndefined();
    const neu = sitzungAusstellen(g, mit.menschId, Date.now() + 5);
    expect(await sitzungLesen(db, g, neu)).toBe(mit.menschId);
    await db.query("update menschen set gesperrt = true where id = $1", [mit.menschId]);
    expect(await sitzungLesen(db, g, neu)).toBeUndefined();
    await db.query("update menschen set gesperrt = false, sitzungen_ab = null where id = $1", [mit.menschId]);
  });
});

describe("Ereignisprotokoll (S9)", () => {
  it("ist unveränderlich und erkennt Manipulation", async () => {
    expect((await ketteVerifizieren(db)).ok).toBe(true);
    await expect(db.query("update ereignisse set daten = '{}' where seq = 1")).rejects.toThrow(/unveränderlich/);
    await expect(db.query("delete from ereignisse where seq = 1")).rejects.toThrow(/unveränderlich/);
    // Ein Angreifer mit Superuser-Rechten schaltet den Trigger ab – die Kette verrät ihn trotzdem.
    await db.query("alter table ereignisse disable trigger ereignisse_kein_update");
    await db.query("update ereignisse set daten = '{\"titel\":\"gefälscht\"}' where seq = 2");
    await db.query("alter table ereignisse enable trigger ereignisse_kein_update");
    const e = await ketteVerifizieren(db);
    expect(e.ok).toBe(false);
    expect(e.fehler).toMatch(/Ereignis 2/);
  });
});

describe("Kennzahlen", () => {
  it("liefert Fortschritt und Statusboard-Werte", async () => {
    const z = await kennzahlen(db, "test");
    expect(z.fortschritt_prozent).toBeGreaterThan(0);
    expect(z.lead_modell).toBe("Claude Opus 5.5");
    expect(z.mitstreiter_menschen).toBeGreaterThanOrEqual(2);
    expect(z.spenden_monat.tokens).toBeGreaterThanOrEqual(5000); // Verbrauch aus dem ersten Durchlauf
  });
});

describe("Kopf-Hash veröffentlichen (#31)", () => {
  it("signiert den Kopf und entlarvt eine neu berechnete Kette", async () => {
    const { exportJWK, generateKeyPair, compactVerify } = await import("jose");
    const { kopfVeroeffentlichen, mitKoepfenVergleichen } = await import("../src/koepfe.ts");
    const { ereignisHash } = await import("../src/ereignisse.ts");
    const { privateKey, publicKey } = await generateKeyPair("ES256", { extractable: true });
    const jwk = { ...(await exportJWK(privateKey)), kid: "test" };
    // Was ein Angreifer mit voller Datenbankkontrolle tun kann: Ereignisse ändern und die Kette danach neu berechnen.
    // (Erster Aufruf ohne Änderung: repariert die Kette, die der Manipulationstest oben absichtlich beschädigt hat.)
    const neuBerechnen = async (faelschen: boolean) => {
      await db.query("alter table ereignisse disable trigger user");
      let vorgaenger = "0".repeat(64);
      for (const z of (await db.query("select * from ereignisse order by seq")).rows) {
        const daten = faelschen && Number(z.seq) === 1 ? { ...z.daten, gefaelscht: true } : z.daten;
        const hash = ereignisHash(vorgaenger, Number(z.seq), new Date(z.zeit).toISOString(),
          { projekt_id: z.projekt_id === null ? null : Number(z.projekt_id), akteur: z.akteur, art: z.art, bezug: z.bezug, daten });
        await db.query("update ereignisse set daten = $2, vorgaenger_hash = $3, hash = $4 where seq = $1", [z.seq, daten, vorgaenger, hash]);
        vorgaenger = hash;
      }
      await db.query("alter table ereignisse enable trigger user");
    };
    await neuBerechnen(false);

    const k = await kopfVeroeffentlichen(db, jwk);
    expect(k).not.toBeNull();
    const inhalt = JSON.parse(new TextDecoder().decode((await compactVerify(k!.jws, publicKey)).payload));
    expect(inhalt).toMatchObject({ seq: k!.seq, hash: k!.hash });
    expect(await kopfVeroeffentlichen(db, jwk)).toBeNull(); // nichts Neues seit dem letzten Kopf
    expect((await mitKoepfenVergleichen(db, [k!])).abweichungen).toEqual([]);

    await neuBerechnen(true);
    expect((await ketteVerifizieren(db)).ok).toBe(true); // in sich stimmig …
    const v = await mitKoepfenVergleichen(db, [k!]);   // … aber nicht mehr mit dem veröffentlichten Kopf
    expect(v.abweichungen).toHaveLength(1);
    expect(v.abweichungen[0]).toMatchObject({ seq: k!.seq, veroeffentlicht: k!.hash });
  });
});

describe("Modell-Rangliste", () => {
  it("zeigt Modelle erst ab Mindestzahlen, führt Schreibweisen zusammen und sortiert nach der Wilson-Untergrenze", async () => {
    const { modellRangliste, modellCacheLeeren, wilsonUnten } = await import("../src/modelle.ts");
    const { nachAussen } = await import("../src/aussen.ts");
    expect(wilsonUnten(5, 5)).toBe(56.6);
    expect(wilsonUnten(0, 0)).toBe(0);
    const a = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Rangliste"), faehigkeiten: ["daten"] });
    const menschen = [];
    for (const n of ["rang-a", "rang-b", "rang-c"]) {
      const m = Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea',$1,$1) returning id", [n])).rows[0].id);
      menschen.push(Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,'x',$2) returning id", [m, n])).rows[0].id));
    }
    // Modell X: 6 Bewertungen von 3 Menschen in zwei Schreibweisen, 5 angenommen; Modell Y: nur ein Mensch, bleibt ausgeblendet
    const einreichen = async (agent: number, modell: string, urteil: string) => {
      const l = Number((await db.query("insert into leases(aufgabe_id, agent_id, laeuft_ab, beendet) values ($1,$2,now(),now()) returning id", [a.id, agent])).rows[0].id);
      const e = Number((await db.query(`insert into einreichungen(aufgabe_id, lease_id, agent_id, patch, patch_sha256, zusammenfassung, verbrauch, modell)
        values ($1,$2,$3,'p','s','z','{"tokens": 1000}',$4) returning id`, [a.id, l, agent, modell])).rows[0].id);
      await db.query("insert into bewertungen(einreichung_id, agent_id, urteil, begruendung) values ($1,$2,$3,'b')", [e, lead.agentId, urteil]);
    };
    for (const [i, u] of ["annehmen", "annehmen", "annehmen", "annehmen", "annehmen", "nacharbeit"].entries()) await einreichen(menschen[i % 3], i % 2 ? "Modell X 1.0" : "modell-x-1.0", u);
    for (let i = 0; i < 6; i++) await einreichen(menschen[0], "Modell Y", "annehmen");
    modellCacheLeeren();
    const r = nachAussen(await modellRangliste(db)) as { models: Record<string, unknown>[]; hidden: number };
    const x = r.models.find((m) => m.key === "modell-x-1-0")!;
    expect(x).toMatchObject({ reviewed: 6, accepted: 5, rework: 1, acceptance_percent: 83, people: 3, median_tokens_per_accepted: 1000 });
    expect((x.kinds as Record<string, unknown>[])[0]).toMatchObject({ kind: "data", reviewed: 6 });
    expect(r.models.some((m) => m.key === "modell-y")).toBe(false);
    expect(r.hidden).toBeGreaterThanOrEqual(1);
    expect(JSON.stringify(r)).not.toMatch(/rang-[abc]/);
  });
});

describe("Öffentliches Profil", () => {
  it("gibt es nur auf Wunsch, mit angenommenen Beiträgen je Projekt und Abzeichen", async () => {
    const { profil, profilSetzen } = await import("../src/profil.ts");
    const { nachAussen } = await import("../src/aussen.ts");
    const login = (await db.query("select login from menschen where id = $1", [mit.menschId])).rows[0].login;
    expect(await profil(db, login)).toBeNull();
    await profilSetzen(db, mit.menschId, true);
    const p = nachAussen(await profil(db, login.toUpperCase())) as { login: string; accepted: number; projects: { project: string }[]; badges: string[]; kinds: unknown[] };
    expect(p.login).toBe(login);
    expect(p.accepted).toBeGreaterThan(0);
    expect(p.projects.map((x) => x.project)).toContain("test");
    expect(p.badges).toContain("first_accepted");
    expect(JSON.stringify(p)).not.toMatch(/"(patch|agent|email)"/);
    await profilSetzen(db, mit.menschId, false);
    expect(await profil(db, login)).toBeNull();
  });
});

describe("Wochenbericht", () => {
  it("zählt die letzten sieben Tage, vergleicht mit der Vorwoche und nennt Engpässe", async () => {
    const { wochenbericht } = await import("../src/wochenbericht.ts");
    const { nachAussen } = await import("../src/aussen.ts");
    const p = await k.projektPerSlug(db, "test");
    const w = nachAussen(await wochenbericht(db, Number(p.id))) as { week: Record<string, number>; now: Record<string, number>; hints: string[]; previous_week: Record<string, number> };
    expect(w.week.created).toBeGreaterThan(0);
    expect(w.week.accepted).toBeGreaterThan(0);
    expect(w.week.units).toBeGreaterThanOrEqual(w.week.accepted);
    expect(w.previous_week).toMatchObject({ accepted: 0 });
    expect(Array.isArray(w.hints)).toBe(true);
    const { kontoDaten } = await import("../src/konto.ts");
    // im Konto nur für Projekte, deren Visionär man ist
    expect((nachAussen(await kontoDaten(db, mit.menschId)) as { weekly_reports: unknown[] }).weekly_reports).toEqual([]);
    await db.query("insert into projekt_rollen(projekt_id, mensch_id, rolle) values ($1, $2, 'visionaer') on conflict do nothing", [p.id, lead.menschId]);
    try {
      expect((nachAussen(await kontoDaten(db, lead.menschId)) as { weekly_reports: { project: string }[] }).weekly_reports.map((r) => r.project)).toContain("test");
    } finally {
      await db.query("delete from projekt_rollen where projekt_id = $1 and mensch_id = $2 and rolle = 'visionaer'", [p.id, lead.menschId]);
    }
  });
});

describe("Aufgaben-Feed", () => {
  it("liefert offene Aufgaben als Atom, gefiltert nach Fähigkeit und Stufe, mit maskiertem Text", async () => {
    const { aufgabenFeed } = await import("../src/feed.ts");
    await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Feed <script> & Co"), faehigkeiten: ["code.rust"], stufe: "B" });
    const alle = await aufgabenFeed(db, {});
    expect(alle).toMatch(/^<\?xml/);
    expect(alle).toContain("Feed &lt;script&gt; &amp; Co");
    expect(alle).not.toContain("<script>");
    expect(await aufgabenFeed(db, { faehigkeit: "code" })).toContain("Feed &lt;script&gt;");
    expect(await aufgabenFeed(db, { faehigkeit: "code.python" })).not.toContain("Feed &lt;script&gt;");
    expect(await aufgabenFeed(db, { stufe: "A", faehigkeit: "code.rust" })).not.toContain("Feed &lt;script&gt;");
    expect(await aufgabenFeed(db, { projekt: "test", stufe: "B" })).toContain("/api/feed/tasks.atom?project=test&amp;level=B\"");
  });
});

describe("Vorschau nach dem Merge", () => {
  it("liefert Projekte nur mit eigener Vorschau-Adresse, merkt den letzten gelungenen Bau und zeigt ihn auf der Projektseite", async () => {
    const { createServer } = await import("node:http");
    const { pruefstrasseBehandeln, signatur } = await import("../src/pruefstrasse.ts");
    const { konfig } = await import("../src/konfig.ts");
    const { projektDetail } = await import("../src/kennzahlen.ts");
    const { nachAussen } = await import("../src/aussen.ts");
    const geheim = "vorschau-test-geheimnis";
    const h = pruefstrasseBehandeln(db, geheim);
    const server = createServer((req, res) => h(req, res, new URL(req.url!, "http://x")));
    await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
    const basis = `http://127.0.0.1:${(server.address() as { port: number }).port}`;
    const senden = async (methode: string, pfad: string, koerper?: unknown) => {
      const text = koerper ? JSON.stringify(koerper) : "", zeit = String(Date.now()), nonce = `n${Math.random().toString(36).slice(2)}${Date.now()}`;
      const kopf = { "x-time": zeit, "x-nonce": nonce, "x-signature": signatur(geheim, zeit, nonce, methode, pfad, text) };
      const r = await fetch(basis + pfad, { method: methode, body: text || undefined, headers: kopf });
      // Dieselbe Anfrage noch einmal: abgewiesen (Wiederholschutz)
      const nochmal = await fetch(basis + pfad, { method: methode, body: text || undefined, headers: kopf });
      expect(nochmal.status).toBe(401);
      return r.json();
    };
    try {
      await db.query("update projekte set repo_git = 'https://github.com/x/test.git' where slug = 'test'");
      konfig.vorschau = "";
      expect(await senden("GET", "/pipeline/previews")).toEqual([]);
      konfig.vorschau = "https://play.example";
      expect((await senden("GET", "/pipeline/previews") as { slug: string }[]).map((p) => p.slug)).toContain("test");
      await senden("POST", "/pipeline/previews/test", { commit: "aaa111", status: "ok", report: "fertig", bytes: 1234 });
      await senden("POST", "/pipeline/previews/test", { commit: "bbb222", status: "failed", report: "kaputt" });
      const d = nachAussen(await projektDetail(db, "test")) as { preview: Record<string, unknown> };
      expect(d.preview).toMatchObject({ url: "https://play.example/test/", playable_commit: "aaa111", commit: "bbb222", status: "failed" });
      const p = (await senden("GET", "/pipeline/previews") as { slug: string; last_commit: string }[]).find((x) => x.slug === "test");
      expect(p?.last_commit).toBe("bbb222");
      // Stand von main je Projekt: die Prüfstraße meldet ihn, die Plattform merkt ihn für die nächste Reservierung
      expect((await senden("GET", "/pipeline/heads") as { slug: string }[]).map((x) => x.slug)).toContain("test");
      expect(await senden("POST", "/pipeline/heads/test", { commit: "kein-hash" })).toMatchObject({ error: expect.stringMatching(/full commit hash/) });
      await senden("POST", "/pipeline/heads/test", { commit: "4".repeat(40) });
      expect((await senden("GET", "/pipeline/heads") as { slug: string; last_commit: string }[]).find((x) => x.slug === "test")?.last_commit).toBe("4".repeat(40));
      await db.query("update projekte set kopf_commit = null where slug = 'test'");
      // eigene Domain: je Projekt eine Subdomain, Zertifikate nur für Projekte mit gebauter Vorschau
      const { vorschauErlaubt } = await import("../src/pruefstrasse.ts");
      konfig.vorschauDomain = "vorschau.example";
      expect((nachAussen(await projektDetail(db, "test")) as { preview: { url: string } }).preview.url).toBe("https://test.vorschau.example/");
      expect(await vorschauErlaubt(db, "test.vorschau.example")).toBe(true);
      expect(await vorschauErlaubt(db, "TEST.Vorschau.Example")).toBe(true);
      expect(await vorschauErlaubt(db, "gibts-nicht.vorschau.example")).toBe(false);
      expect(await vorschauErlaubt(db, "a.b.vorschau.example")).toBe(false);
      expect(await vorschauErlaubt(db, "test.anderswo.example")).toBe(false);
      // Abzeichen und Kurzlink
      const { abzeichen, kurzlinkZiel } = await import("../src/kennzahlen.ts");
      expect(await abzeichen(db, "test")).toContain("▶ play");
      expect(await abzeichen(db, "gibts-nicht")).toContain("unknown");
      expect(await kurzlinkZiel(db, "test", true)).toBe("https://test.vorschau.example/");
      expect(await kurzlinkZiel(db, "gibts-nicht", false)).toMatch(/\/projekte$/);
      // nur Doku (z. B. eine Bibliothek): kein „Vorschau spielen“, aber der Link zur Doku
      await senden("POST", "/pipeline/previews/test", { commit: "ccc333", status: "ok", report: "", bytes: 10, preview: false, docs: true });
      const nurDoku = (nachAussen(await projektDetail(db, "test")) as { preview: { url: string | null; docs_url: string } }).preview;
      expect(nurDoku).toMatchObject({ url: null, docs_url: "https://test.vorschau.example/docs/" });
      expect(await abzeichen(db, "test")).toContain(">docs<");
      expect(await kurzlinkZiel(db, "test", true)).toBe("https://test.vorschau.example/docs/");
      // Vorschau je Einreichung: geprüfte Einreichungen vor der Bewertung, unter review.<Domain>/<id>/
      await senden("POST", "/pipeline/previews/test", { commit: "ddd444", status: "ok", report: "", bytes: 10, preview: true, docs: true });
      const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Mit Vorschau prüfen"));
      await k.reservieren(db, mit, a.id);
      const e: any = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "fertig", eigene_tests: "ok", verbrauch: { tokens: 10 } });
      await k.pruefungStarten(db, e.einreichung_id);
      await k.pruefungMelden(db, e.einreichung_id, "ok", "grün", "sig");
      const liste = await senden("GET", "/pipeline/submission-previews") as { open: { submission_id: number }[]; keep: number[] };
      expect(liste.open.map((x) => x.submission_id)).toContain(Number(e.einreichung_id));
      await senden("POST", `/pipeline/submission-previews/${e.einreichung_id}`, { status: "ok", report: "", bytes: 5 });
      const nachher = await senden("GET", "/pipeline/submission-previews") as { open: { submission_id: number }[]; keep: number[] };
      expect(nachher.open.map((x) => x.submission_id)).not.toContain(Number(e.einreichung_id));
      expect(nachher.keep).toContain(Number(e.einreichung_id));
      const warten = nachAussen(await k.wartendeEinreichungen(db, lead, "test")) as { submission_id: number; preview_url?: string }[];
      expect(warten.find((x) => x.submission_id === Number(e.einreichung_id))?.preview_url).toBe(`https://review.vorschau.example/${e.einreichung_id}/`);
      expect(await vorschauErlaubt(db, "review.vorschau.example")).toBe(true);
      expect(projektPruefen({ slug: "review", name: "R", vision_md: "", vision_short: "x", engine: "x", license_code: "MIT", license_assets: "CC0 1.0", lead_model: "x", goal: "x" }).join()).toMatch(/reserved/);
      // Rückmeldungen zur Vorschau: angemeldete Besucher geben sie, nur Lead und Officers lesen sie
      const { feedbackGeben, feedbackListe, feedbackMarkieren } = await import("../src/feedback.ts");
      await expect(feedbackGeben(db, mit.menschId, "test", {})).rejects.toThrow(/good or what/);
      const { feedback_id } = await feedbackGeben(db, mit.menschId, "test", { gut: "Steuerung fühlt sich gut an", hakt: "Sprung zu kurz", geraet: "Firefox" });
      await expect(feedbackListe(db, mit, "test")).rejects.toThrow(/only the project's lead/);
      expect((await feedbackListe(db, lead, "test"))[0]).toMatchObject({ feedback_id, gut: "Steuerung fühlt sich gut an", commit: "ddd444", zustand: "neu" });
      expect((nachAussen(await projektDetail(db, "test")) as { preview: { feedback: { total: number; new: number } } }).preview.feedback).toEqual({ total: 1, new: 1 });
      await expect(feedbackMarkieren(db, lead, feedback_id, "aufgabe")).rejects.toThrow(/task_id/);
      await feedbackMarkieren(db, lead, feedback_id, "gesehen");
      expect(await feedbackListe(db, lead, "test")).toEqual([]);
    } finally {
      konfig.vorschau = "";
      konfig.vorschauDomain = "";
      server.close();
    }
  });
});

describe("Englische Schnittstelle", () => {
  it("lässt keine deutschen Feldnamen nach außen", async () => {
    const { nachAussen, FELDER } = await import("../src/aussen.ts");
    const { projektListe, projektDetail } = await import("../src/kennzahlen.ts");
    const { kontoDaten } = await import("../src/konto.ts");
    const { koepfeListe } = await import("../src/koepfe.ts");
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Schnittstelle"));
    const proben = [a, await k.aufgabenPassend(db, mit, "test"), await k.details(db, mit, a.id), await k.reservieren(db, mit, a.id),
      await kennzahlen(db, "test"), await projektListe(db), await projektDetail(db, "test"), await kontoDaten(db, mit.menschId), await koepfeListe(db),
      await (await import("../src/modelle.ts")).modellRangliste(db), await (await import("../src/kennzahlen.ts")).schwarmLive(db)];
    // Erlaubt: englische Zielnamen und Felder, die in beiden Sprachen gleich sind
    const erlaubt = new Set([...Object.values(FELDER), "id", "slug", "name", "engine", "patch", "email", "commit", "code", "other"]);
    const fremd = new Set<string>();
    const sammeln = (x: unknown) => {
      if (Array.isArray(x)) return x.forEach(sammeln);
      // agents_active_by_kind hat dynamische Schlüssel (Fähigkeitsarten wie code, asset, spec)
      if (x && typeof x === "object" && !(x instanceof Date)) for (const [key, v] of Object.entries(x)) { if (!erlaubt.has(key)) fremd.add(key); if (key !== "agents_active_by_kind") sammeln(v); }
    };
    sammeln(nachAussen(proben));
    expect([...fremd]).toEqual([]);
    expect(JSON.stringify(nachAussen(proben))).not.toMatch(/"(offen|reserviert|angenommen|annehmen|visionaer|mitstreiter|analyse|bau)"/);
  });
});

describe("Gegenprüfung bei proprietären Engines (E-31)", () => {
  const PATCH_UNITY = `diff --git a/Assets/Scripts/Neu.cs b/Assets/Scripts/Neu.cs
new file mode 100644
--- /dev/null
+++ b/Assets/Scripts/Neu.cs
@@ -0,0 +1 @@
+public class Neu {}
`;
  const unityAufgabe = async (titel: string) => {
    await db.query(`insert into projekte(slug,name,vision_md,vision_kurz,engine,lizenz_code,lizenz_assets,repo_url,lead_modell,ziel)
      values ('unity-test','Unity','V','k','Unity 6000.0.32f1','MIT','CC BY 4.0','http://x','Claude','Demo') on conflict do nothing`);
    await db.query("insert into projekt_rollen values ((select id from projekte where slug='unity-test'), $1, 'lead') on conflict do nothing", [lead.menschId]);
    return k.aufgabeAnlegen(db, lead, "unity-test", { ...neueAufgabe(titel), faehigkeiten: ["code.csharp"], pfade: ["Assets/Scripts/**"] });
  };
  const zustand = async (id: number) => (await db.query("select zustand from aufgaben where id = $1", [id])).rows[0].zustand;

  it("eigener Lauf grün, Gegenprüfung durch einen anderen Menschen, dann Review", async () => {
    const a = await unityAufgabe("Gegenprüfung grün");
    await k.reservieren(db, mit, a.id);
    const e: any = await k.einreichen(db, mit, a.id, { patch: PATCH_UNITY, zusammenfassung: "z", eigene_tests: "EditMode 12/12", verbrauch: {}, eigenes_ergebnis: "ok" });
    expect(await zustand(a.id)).toBe("pruefung");
    // Die Prüfstraße bekommt Engine-Projekte nicht; der Autor prüft nicht gegen; ohne passende Engine nichts.
    expect(await g.naechsteGegenpruefung(db, mit, "unity-test", ["unity"])).toBeNull();
    expect(await g.naechsteGegenpruefung(db, fremd, "unity-test", ["unreal"])).toBeNull();
    const job: any = await g.naechsteGegenpruefung(db, fremd, "unity-test", ["unity"]);
    expect(job).toMatchObject({ aufgabe_id: Number(a.id), einreichung_id: e.einreichung_id, patch: PATCH_UNITY });
    // Reserviert: ein zweiter Runner bekommt dieselbe Einreichung nicht gleichzeitig.
    const dritter = { menschId: lead.menschId, agentId: lead.agentId };
    expect(await g.naechsteGegenpruefung(db, dritter, "unity-test", ["unity"])).toBeNull();
    expect(await g.gegenpruefungAktiv(db, fremd.agentId, Number(a.id))).toBe(true);
    await expect(g.gegenpruefungMelden(db, mit, job.gegenpruefung_id, "ok", "x")).rejects.toThrow(/not reserved for you/);
    await g.gegenpruefungMelden(db, fremd, job.gegenpruefung_id, "ok", "EditMode 12/12 passed");
    expect(await zustand(a.id)).toBe("geprueft_ok");
    expect(await g.gegenpruefungAktiv(db, fremd.agentId, Number(a.id))).toBe(false);
    const quellen = (await db.query("select quelle, ergebnis from pruefungen where einreichung_id = $1 order by id", [e.einreichung_id])).rows;
    expect(quellen).toEqual([{ quelle: "selbst", ergebnis: "ok" }, { quelle: "gegenpruefung", ergebnis: "ok" }]);
    await k.bewerten(db, lead, e.einreichung_id, "annehmen", "passt");
  });

  it("Gegenprüfung rot widerspricht dem eigenen Lauf: zurück in den Pool, im Protokoll vermerkt", async () => {
    const a = await unityAufgabe("Gegenprüfung rot");
    await k.reservieren(db, mit, a.id);
    await k.einreichen(db, mit, a.id, { patch: PATCH_UNITY, zusammenfassung: "z", eigene_tests: "grün", verbrauch: {}, eigenes_ergebnis: "ok" });
    const job: any = await g.naechsteGegenpruefung(db, fremd, "unity-test", ["unity"]);
    // Erst „nicht möglich“ (falsche Version): gibt nur frei, dieselbe Person bekommt sie nicht noch einmal.
    await g.gegenpruefungMelden(db, fremd, job.gegenpruefung_id, "nicht_moeglich", "6000.0.32f1 not installed");
    expect(await zustand(a.id)).toBe("pruefung");
    expect(await g.naechsteGegenpruefung(db, fremd, "unity-test", ["unity"])).toBeNull();
    const zweiter: any = await g.naechsteGegenpruefung(db, lead, "unity-test", ["unity"]);
    await g.gegenpruefungMelden(db, lead, zweiter.gegenpruefung_id, "fehler", "2 tests failed");
    expect(await zustand(a.id)).toBe("offen");
    const ev = (await db.query("select daten from ereignisse where art = 'pruefung_beendet' and bezug = $1 order by seq desc limit 1", [`aufgabe:${a.id}`])).rows[0].daten;
    expect(ev).toMatchObject({ quelle: "gegenpruefung", ergebnis: "fehler", widerspricht_selbst: true });
  });

  it("eigener Lauf rot geht sofort zurück; ohne zweiten Runner gibt die Frist frei", async () => {
    const a = await unityAufgabe("Selbst rot");
    await k.reservieren(db, mit, a.id);
    await k.einreichen(db, mit, a.id, { patch: PATCH_UNITY, zusammenfassung: "z", eigene_tests: "1 failed", verbrauch: {}, eigenes_ergebnis: "fehler" });
    expect(await zustand(a.id)).toBe("offen");

    const b = await unityAufgabe("Ohne Gegenprüfung");
    await k.reservieren(db, mit, b.id);
    const e: any = await k.einreichen(db, mit, b.id, { patch: PATCH_UNITY, zusammenfassung: "z", eigene_tests: "grün", verbrauch: {}, eigenes_ergebnis: "ok" });
    expect(await g.ohneGegenpruefungFreigeben(db)).toBe(0);
    await db.query("update einreichungen set eingereicht = now() - interval '25 hours' where id = $1", [e.einreichung_id]);
    expect(await g.ohneGegenpruefungFreigeben(db)).toBe(1);
    expect(await zustand(b.id)).toBe("geprueft_ok");
    expect(await ketteVerifizieren(db)).toMatchObject({ ok: true });
  });
});

describe("Zuteilung nach nachgewiesener Stufe", () => {
  const neuerAgent = async (login: string) => {
    const m = Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea',$1,$1) returning id", [login])).rows[0].id);
    const ag = Number((await db.query("insert into agenten(mensch_id, client_id, name, runner) values ($1,$2,$2,true) returning id", [m, `${login}-runner`])).rows[0].id);
    return { menschId: m, agentId: ag, runner: true };
  };
  // Ein angenommener Versuch auf Stufe s, ohne die ganze Strecke (Lease, Einreichung, Bewertung direkt)
  const geschafft = async (wer: { menschId: number; agentId: number }, stufe: string, modell: string, urteil = "annehmen") => {
    const a = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe(`Nachweis ${stufe}`), stufe: stufe as "A" });
    const l = Number((await db.query("insert into leases(aufgabe_id, agent_id, laeuft_ab, beendet) values ($1,$2, now() + interval '1 hour', now()) returning id", [a.id, wer.agentId])).rows[0].id);
    const e = Number((await db.query("insert into einreichungen(aufgabe_id, lease_id, agent_id, patch, patch_sha256, zusammenfassung, eigene_tests, verbrauch, modell) values ($1,$2,$3,'x','x','z','t','{}',$4) returning id", [a.id, l, wer.agentId, modell])).rows[0].id);
    await db.query("insert into bewertungen(einreichung_id, agent_id, urteil, begruendung) values ($1,$2,$3,'Test')", [e, lead.agentId, urteil]);
    await db.query("update aufgaben set zustand = 'geschlossen' where id = $1", [a.id]);
  };
  it("neu: A und B, C nur als Probe, D nie; nach drei Annahmen auf B ist B nachgewiesen, C weiter Probe; nach C auch C", async () => {
    const neu = await neuerAgent("zuteilung-neu");
    expect(await k.stufenFreigabe(db, neu.agentId)).toEqual({ bis: "B", probe: "C", probe_dran: true });
    const b = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Zuteilung B"), stufe: "B" });
    const c = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Zuteilung C"), stufe: "C" });
    const d = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe("Zuteilung D"), stufe: "D" });
    const ids = async (wer: typeof neu) => (await k.aufgabenPassend(db, wer, "test")).map((x: any) => [Number(x.id), !!x.probe]);
    let liste = await ids(neu);
    expect(liste).toContainEqual([Number(c.id), true]);      // Probe fällig (noch keine Versuche)
    expect(liste.map(([id]) => id)).not.toContain(Number(d.id));
    expect(liste[0]).toEqual([Number(c.id), true]);          // fällige Probe kommt vorn
    await expect(k.reservieren(db, neu, d.id)).rejects.toThrow(/level D needs an agent that has shown level C/);
    // Vier Versuche, keiner davon Probe: die nächste Probe ist dran; nach einer Probe erst nach vier weiteren
    for (let i = 0; i < 3; i++) await geschafft(neu, "B", "modell-x");
    expect(await k.stufenFreigabe(db, neu.agentId)).toMatchObject({ bis: "B", probe: "C" });
    await geschafft(neu, "C", "modell-x", "nacharbeit");
    expect((await k.stufenFreigabe(db, neu.agentId)).probe_dran).toBe(false);   // 1 Probe auf 4 Versuche
    liste = await ids(neu);
    expect(liste.map(([id]) => id)).not.toContain(Number(c.id));
    await expect(k.reservieren(db, neu, c.id)).rejects.toThrow(/trial/);
    for (let i = 0; i < 3; i++) await geschafft(neu, "C", "modell-x");
    expect(await k.stufenFreigabe(db, neu.agentId)).toMatchObject({ bis: "C", probe: "D" });   // 3 von 4 auf C angenommen
    // Vorschuss über das Modell: ein neuer Agent mit demselben Modell startet bei C
    const zweiter = await neuerAgent("zuteilung-gleiches-modell");
    expect((await k.stufenFreigabe(db, zweiter.agentId)).bis).toBe("B");   // noch ohne Modell
    await geschafft(zweiter, "A", "modell-x", "nacharbeit");
    expect((await k.stufenFreigabe(db, zweiter.agentId)).bis).toBe("C");
    // Der Lead ist ausgenommen und sieht alles
    expect((await k.aufgabenPassend(db, lead, "test")).map((x) => Number(x.id))).toEqual(expect.arrayContaining([Number(b.id), Number(c.id), Number(d.id)]));
    for (const a of [b, c, d]) await k.schliessen(db, lead, a.id, "Test");
  });
});

describe("MCP: Aufgaben gebündelt anlegen, Kontextpaket", () => {
  const werkzeuge = async (wer: { menschId: number; agentId: number }) => {
    const { mcpServerFuer } = await import("../src/mcp.ts");
    const { InMemoryTransport } = await import("@modelcontextprotocol/sdk/inMemory.js");
    const { Client } = await import("@modelcontextprotocol/sdk/client/index.js");
    const server = await mcpServerFuer(db, { ...wer, runner: false, clientId: "test", token: "t", scopes: ["mcp"], ablauf: Date.now() + 60_000 });
    const [a, b] = InMemoryTransport.createLinkedPair();
    const client = new Client({ name: "test", version: "1" });
    await Promise.all([server.connect(a), client.connect(b)]);
    return async (name: string, args: Record<string, unknown>) => {
      const r: any = await client.callTool({ name, arguments: args });
      return JSON.parse(r.content[0].text);
    };
  };
  const feld = { capabilities: ["code.gdscript"], paths: ["game/modules/vfx/**"], acceptance: ["tests green"] };
  it("legt Code und Tests in einem Aufruf an, after wird zu depends_on; das Kontextpaket kommt im Auftrag an", async () => {
    const rufen = await werkzeuge(lead);
    const r = await rufen("create_tasks", { project: "test", tasks: [
      { title: "Pausenmenü: Code", description: "Screen und Anbindung", level: "B", ...feld, context: ["game/modules/ui/menu_screen.gd", "game/tests/menu_test.gd"] },
      { title: "Pausenmenü: Tests", description: "neue Prüfungen", level: "B", ...feld, after: [0] },
    ] });
    expect(r.created).toHaveLength(2);
    const [code, tests] = r.created.map((x: { tasks: number[] }) => x.tasks[0]);
    const d: any = await k.details(db, mit, tests);
    expect(d.abhaengig_von.map(Number)).toEqual([code]);
    expect((await k.details(db, mit, code) as any).kontext).toEqual(["game/modules/ui/menu_screen.gd", "game/tests/menu_test.gd"]);
    // Abgewiesen: Index nach vorn, Kontext mit .., und was davor lag, bleibt angelegt
    const f = await rufen("create_tasks", { project: "test", tasks: [
      { title: "Gut", description: "d", level: "A", ...feld },
      { title: "Schlechter Kontext", description: "d", level: "A", ...feld, context: ["../geheim"] },
    ] });
    expect(f.created).toHaveLength(1);
    expect(f.rejected).toMatchObject({ index: 1, reason: expect.stringMatching(/context/) });
    expect((await rufen("create_tasks", { project: "test", tasks: [{ title: "X", description: "d", level: "A", ...feld, after: [0] }] })).rejected.index).toBe(0);
    for (const id of [code, tests, f.created[0].tasks[0]]) await k.schliessen(db, lead, id, "Test");
  });
});

describe("Gegenprüfung für alle Projekte (Runner ab 0.11)", () => {
  const zustand = async (id: number) => (await db.query("select zustand from aufgaben where id = $1", [id])).rows[0].zustand;
  const mitPruefstrasse = async (lauf: (senden: (m: string, p: string, k?: unknown) => Promise<any>) => Promise<void>) => {
    const { createServer } = await import("node:http");
    const { pruefstrasseBehandeln, signatur } = await import("../src/pruefstrasse.ts");
    const geheim = "gegenpruefung-alle";
    const h = pruefstrasseBehandeln(db, geheim);
    const server = createServer((req, res) => h(req, res, new URL(req.url!, "http://x")));
    await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
    const basis = `http://127.0.0.1:${(server.address() as { port: number }).port}`;
    try {
      await lauf(async (methode, pfad, koerper) => {
        const text = koerper ? JSON.stringify(koerper) : "", zeit = String(Date.now()), nonce = `n${Math.random().toString(36).slice(2)}${Date.now()}`;
        const r = await fetch(basis + pfad, { method: methode, body: text || undefined, headers: { "x-time": zeit, "x-nonce": nonce, "x-signature": signatur(geheim, zeit, nonce, methode, pfad, text) } });
        return r.json();
      });
    } finally {
      server.close();
    }
  };
  const einreichen = async (titel: string, eigenes?: "ok" | "fehler") => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe(titel));
    await k.reservieren(db, mit, a.id);
    const e: any = await k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "z", eigene_tests: "node --test: 3 passed", verbrauch: {}, eigenes_ergebnis: eigenes });
    return { a, eid: Number(e.einreichung_id) };
  };
  const wartend = async (senden: (m: string, p: string) => Promise<any>) => ((await senden("GET", "/pipeline/pending")) as { submission_id: number }[]).map((x) => Number(x.submission_id));

  it("ohne aktiven Gegenprüfer prüft die Prüfstraße sofort; mit einem wartet die Einreichung auf ihn, zwei grüne Läufe genügen", async () => {
    await mitPruefstrasse(async (senden) => {
      await db.query("update agenten set gegenpruefer_gesehen = null");
      const { eid: e1 } = await einreichen("Kein Gegenprüfer da", "ok");
      expect(await wartend(senden)).toContain(e1);
      // Runner ab 0.11 fragt (any): ab jetzt wartet eine neue Einreichung auf ihn statt auf die Prüfstraße
      expect(await g.naechsteGegenpruefung(db, fremd, "test", [], false)).toBeNull();   // alter Runner: nur Engine-Projekte
      const erster: any = await g.naechsteGegenpruefung(db, fremd, "test", [], true);
      expect(erster.einreichung_id).toBe(e1);
      await g.gegenpruefungMelden(db, fremd, erster.gegenpruefung_id, "nicht_moeglich", "podman fehlt");
      const { a, eid } = await einreichen("Gegenprüfer da", "ok");
      expect(await wartend(senden)).not.toContain(eid);
      expect(await g.naechsteGegenpruefung(db, mit, "test", [], true)).toBeNull();   // nicht die eigene
      const job: any = await g.naechsteGegenpruefung(db, fremd, "test", [], true);
      expect(job.einreichung_id).toBe(eid);
      expect(await g.gegenpruefungAktiv(db, fremd.agentId, Number(a.id))).toBe(true);
      await g.gegenpruefungMelden(db, fremd, job.gegenpruefung_id, "ok", "node --test: 3 passed");
      expect(await zustand(a.id)).toBe("geprueft_ok");
      expect(await wartend(senden)).not.toContain(eid);
      const quellen = (await db.query("select quelle, ergebnis from pruefungen where einreichung_id = $1 order by id", [eid])).rows;
      expect(quellen).toEqual([{ quelle: "selbst", ergebnis: "ok" }, { quelle: "gegenpruefung", ergebnis: "ok" }]);
      // Danach wie immer: Bewertung, und der Merge-Zug sieht die Prüfung als bestanden
      await k.bewerten(db, lead, eid, "annehmen", "zwei grüne Läufe, Akzeptanz erfüllt");
      const angenommen = ((await senden("GET", "/pipeline/accepted")) as { submission_id: number; check: string }[]).find((x) => Number(x.submission_id) === eid);
      expect(angenommen?.check).toBe("pass");
    });
  });

  it("rote Gegenprüfung, kein grüner eigener Lauf oder abgelaufene Wartezeit: die Prüfstraße entscheidet", async () => {
    await mitPruefstrasse(async (senden) => {
      await g.naechsteGegenpruefung(db, fremd, "test", [], true);   // aktiver Gegenprüfer
      const { a, eid } = await einreichen("Gegenprüfung rot", "ok");
      const job: any = await g.naechsteGegenpruefung(db, fremd, "test", [], true);
      expect(job.einreichung_id).toBe(eid);
      expect(await wartend(senden)).not.toContain(eid);   // läuft gerade
      expect(await g.gegenpruefungMelden(db, fremd, job.gegenpruefung_id, "fehler", "1 failed")).toMatchObject({ entscheidet: "test_pipeline" });
      expect(await zustand(a.id)).toBe("eingereicht");   // eine Gegenprüfung allein lehnt nie ab
      expect(await wartend(senden)).toContain(eid);
      expect((await g.naechsteGegenpruefung(db, lead, "test", [], true))?.einreichung_id).not.toBe(eid);   // kein zweiter Versuch über Runner
      await senden("POST", `/pipeline/${eid}/start`, {});
      await senden("POST", `/pipeline/${eid}/result`, { result: "pass", report: "grün auf der Prüfstraße" });
      expect(await zustand(a.id)).toBe("geprueft_ok");

      const { eid: ohneSelbst } = await einreichen("Ohne eigenen Lauf");
      const { eid: selbstRot } = await einreichen("Eigener Lauf rot", "fehler");
      const { eid: wartetZuLange } = await einreichen("Wartet zu lange", "ok");
      const liste = await wartend(senden);
      expect(liste).toContain(ohneSelbst);
      expect(liste).toContain(selbstRot);
      expect(liste).not.toContain(wartetZuLange);
      await db.query("update einreichungen set eingereicht = now() - interval '16 minutes' where id = $1", [wartetZuLange]);
      expect(await wartend(senden)).toContain(wartetZuLange);
    });
  });
});

describe("Besucherstatistik in der Datenbank", () => {
  it("zählt Aufrufe und eindeutige Besucher und behält nach Tagesende nur die Zahl", async () => {
    const { besuchZaehlen, besucheAbschliessen } = await import("../src/besuche.ts");
    const ua = "Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0";
    expect(await besuchZaehlen(db, { p: "/", r: "https://www.google.com/" }, "198.51.100.7", ua, "swarvex.ai")).toBe(true);
    await besuchZaehlen(db, { p: "/projekte" }, "198.51.100.7", ua, "swarvex.ai");
    await besuchZaehlen(db, { p: "/" }, "203.0.113.9", ua, "swarvex.ai");
    expect(await besuchZaehlen(db, { p: "javascript:alert(1)" }, "203.0.113.9", ua, "swarvex.ai")).toBe(false);
    const heute = (await db.query("select tag::text, besucher from analytik.besucher")).rows;
    expect(heute).toHaveLength(1);
    expect(heute[0].besucher).toBe(2);
    const seiten = (await db.query("select pfad, quelle, sum(aufrufe)::int as n from analytik.besuche group by 1, 2 order by 1, 2")).rows;
    expect(seiten).toEqual([{ pfad: "/", quelle: "direkt", n: 1 }, { pfad: "/", quelle: "google.com", n: 1 }, { pfad: "/projekte", quelle: "direkt", n: 1 }]);
    // Kein Hash enthält die IP-Adresse im Klartext
    const roh = (await db.query("select encode(kennung, 'escape') as k from besucher_heute")).rows.map((r) => r.k).join(" ");
    expect(roh).not.toContain("198.51.100.7");
    // Tag abschließen: so tun, als wäre gestern gezählt worden
    await db.query("update besucher_heute set tag = tag - 1; update besuch_salz set tag = tag - 1; update besuche set tag = tag - 1");
    await besucheAbschliessen(db);
    expect((await db.query("select count(*)::int as n from besucher_heute")).rows[0].n).toBe(0);
    expect((await db.query("select count(*)::int as n from besuch_salz")).rows[0].n).toBe(0);
    expect((await db.query("select besucher from analytik.besucher")).rows).toEqual([{ besucher: 2 }]);
  });
});

// Anmeldeablauf gegen den echten OAuth-Server (Review 4. Nov. 2026): Clients mit Rücksprung auf eine Website brauchen
// eine ausdrückliche Zustimmung; lokale Agenten nicht. Der Konto-Anbieter ist gespielt.
describe("Anmeldung: Zustimmung und Geräte-Bestätigung", () => {
  let server: Server;
  let basis = "";
  const keks = new Map<string, string>();
  // Vitest setzt BASE_URL selbst auf "/", deshalb hier ein fester Issuer (danach wieder der alte Wert)
  const BASIS = "http://localhost:3000";
  const vorher = konfig.basis;

  beforeAll(async () => {
    konfig.basis = BASIS;
    const schluessel = await schluesselLaden(db);
    const anbieter = anbieterErstellen(db, schluessel);
    const konto = { name: "spiel", titel: "Spiel", anmelden: (_r: string, state: string) => `https://konto.invalid/login?state=${encodeURIComponent(state)}`,
      nutzer: async () => ({ id: "oauth-1", login: "oauthtester", erstellt: null }) };
    const interaktion = interaktionBehandeln(anbieter, db, schluessel.cookies[0], [konto]);
    const oidc = anbieter.callback();
    server = createServer((req, res) => {
      const url = new URL(req.url ?? "/", konfig.basis);
      if (url.pathname.startsWith("/interaction/")) interaktion(req, res, url).catch((e) => { console.error(e); res.writeHead(500).end(); });
      else oidc(req, res);
    });
    await new Promise<void>((ok) => server.listen(0, "127.0.0.1", ok));
    basis = `http://127.0.0.1:${(server.address() as { port: number }).port}`;
  });
  afterAll(() => new Promise<void>((ok) => { konfig.basis = vorher; server.close(() => ok()); }));

  /** Eine Anfrage wie ein Browser ohne automatische Weiterleitung; Cookies merkt sich keks. */
  async function holen(pfad: string, init: RequestInit = {}) {
    const ziel = pfad.startsWith(BASIS) ? basis + pfad.slice(BASIS.length) : pfad.startsWith("http") ? pfad : basis + pfad;
    const r = await fetch(ziel, { ...init, redirect: "manual", headers: { ...(init.headers ?? {}), cookie: [...keks].map(([k, v]) => `${k}=${v}`).join("; ") } });
    for (const c of r.headers.getSetCookie()) {
      const [paar, ...attr] = c.split(";");
      const [k, ...v] = paar.split("=");
      if (attr.some((a) => /max-age=0|expires=thu, 01 jan 1970/i.test(a.trim()))) keks.delete(k.trim()); else keks.set(k.trim(), v.join("="));
    }
    return { status: r.status, ort: r.headers.get("location") ?? "", csp: r.headers.get("content-security-policy") ?? "", text: await r.text() };
  }
  const folgen = async (start: { status: number; ort: string; text: string }) => {
    let r = start;
    for (let i = 0; i < 10 && r.status >= 300 && r.status < 400 && !/^https?:\/\/(?!127\.0\.0\.1:\d+\/(?:auth|interaction|device))/.test(r.ort.replace(BASIS, basis)); i++) r = await holen(r.ort);
    return r;
  };
  const registrieren = async (body: Record<string, unknown>) => {
    const r = await fetch(`${basis}/reg`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ token_endpoint_auth_method: "none", ...body }) });
    return (await r.json()) as { client_id: string };
  };
  const autorisieren = (clientId: string, rueck: string) => {
    const pruef = createHash("sha256").update(randomBytes(32).toString("base64url")).digest("base64url");
    return holen(`/auth?${new URLSearchParams({ client_id: clientId, response_type: "code", redirect_uri: rueck, scope: "mcp", code_challenge: pruef, code_challenge_method: "S256", resource: RESSOURCE(), state: "s" })}`);
  };

  it("Web-Client: erst nach Zulassen gibt es einen Code; fremder Origin zählt nicht", async () => {
    const { client_id } = await registrieren({ client_name: "Böse <b>Seite</b>", redirect_uris: ["https://boese.invalid/cb"] });
    // Anmelden: Interaktion → gespielter Anbieter → callback → login
    let r = await folgen(await autorisieren(client_id, "https://boese.invalid/cb"));
    expect(r.ort).toMatch(/^https:\/\/konto\.invalid\/login\?state=/);
    const state = new URL(r.ort).searchParams.get("state")!;
    r = await holen(`/interaction/callback?${new URLSearchParams({ state, code: "x" })}`);
    const weiter = /url=([^"]+)"/.exec(r.text)![1];
    r = await folgen(await holen(weiter));
    // Jetzt die Zustimmungsseite, kein Code für die fremde Seite
    expect(r.status).toBe(200);
    expect(r.text).toMatch(/Allow this agent\?|Diesen Agenten zulassen\?/);
    expect(r.text).toContain("boese.invalid");
    expect(r.text).toContain("oauthtester");
    expect(r.text).not.toContain("<b>Seite</b>");
    const uid = /action="\/interaction\/([^/"]+)\/confirm"/.exec(r.text)![1];
    // Formular von einer fremden Seite abgeschickt: wieder nur die Seite
    const fremd = await holen(`/interaction/${uid}/confirm`, { method: "POST", headers: { origin: "https://boese.invalid" } });
    expect(fremd.status).toBe(200);
    expect(fremd.text).toMatch(/Allow this agent\?|Diesen Agenten zulassen\?/);
    // Von swarvex selbst bestätigt: Code an die Rücksprungadresse
    r = await folgen(await holen(`/interaction/${uid}/confirm`, { method: "POST", headers: { origin: BASIS } }));
    expect(r.ort).toMatch(/^https:\/\/boese\.invalid\/cb\?code=/);
  });

  it("Abbrechen schickt access_denied zurück", async () => {
    const { client_id } = await registrieren({ client_name: "Zweite Seite", redirect_uris: ["https://zwei.invalid/cb"] });
    let r = await folgen(await autorisieren(client_id, "https://zwei.invalid/cb")); // Sitzung besteht schon
    expect(r.text).toMatch(/Allow this agent\?|Diesen Agenten zulassen\?/);
    const uid = /action="\/interaction\/([^/"]+)\/abort"/.exec(r.text)![1];
    r = await folgen(await holen(`/interaction/${uid}/abort`, { method: "POST", headers: { origin: BASIS } }));
    expect(r.ort).toMatch(/^https:\/\/zwei\.invalid\/cb\?error=access_denied/);
  });

  it("lokaler Agent (Loopback) kommt ohne Rückfrage durch", async () => {
    const { client_id } = await registrieren({ client_name: "Lokal", redirect_uris: ["http://127.0.0.1:9/cb"] });
    const r = await folgen(await autorisieren(client_id, "http://127.0.0.1:9/cb"));
    expect(r.ort).toMatch(/^http:\/\/127\.0\.0\.1:9\/cb\?code=/);
  });

  it("Geräte-Bestätigung zeigt den Client-Namen maskiert", async () => {
    const { client_id } = await registrieren({ client_name: '<img src=x onerror="alert(1)">', grant_types: ["urn:ietf:params:oauth:grant-type:device_code", "refresh_token"], response_types: [], redirect_uris: [] });
    const d = (await (await fetch(`${basis}/device/auth`, { method: "POST", body: new URLSearchParams({ client_id, scope: "mcp", resource: RESSOURCE() }) })).json()) as { user_code: string };
    const leer = await holen("/device"); // das sichtbare Eingabefeld (ohne Code im Link)
    expect(leer.text).toMatch(/type="text" name="user_code"/);
    expect(leer.text).not.toMatch(/\son[a-z]+="/); // keine Inline-Handler: die CSP würde sie sperren
    const eingabe = await holen(`/device?user_code=${encodeURIComponent(d.user_code)}`);
    // Die Zwischenseite schickt ihr Formular per Inline-Skript ab: Die CSP muss genau dieses Skript erlauben
    const skript = /<script>([\s\S]*?)<\/script>/.exec(eingabe.text)![1];
    const hash = createHash("sha256").update(skript).digest("base64");
    expect(eingabe.csp).toMatch(new RegExp(`script-src[^;]*'sha256-${hash.replace(/[+/]/g, "\\$&")}'`));
    expect(eingabe.csp).toMatch(/default-src 'none'/);
    expect(leer.csp).toMatch(/script-src(;|$)/); // ohne Skript: nichts erlaubt
    const xsrf = /name="xsrf" value="([^"]+)"/.exec(eingabe.text)![1];
    const r = await folgen(await holen("/device", { method: "POST", headers: { "content-type": "application/x-www-form-urlencoded" }, body: new URLSearchParams({ xsrf, user_code: d.user_code }).toString() }));
    expect(r.text).toMatch(/Confirm the device|Gerät bestätigen/);
    expect(r.text).not.toMatch(/\son[a-z]+="/);
    expect(r.text).not.toContain("<img src=x");
    expect(r.text).toContain("&#60;img");
    expect(r.text).toMatch(/Only confirm if you started this sign-in yourself|Bestätige nur, wenn du diese Anmeldung gerade selbst/);
  });
});

describe("Vertrauensstufen im Ablauf (E-35)", () => {
  it("drei Merges heben auf Stufe 1, eine Regression stuft sofort ab, alles im Protokoll", async () => {
    const m = Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea','aufsteiger','aufsteiger') returning id")).rows[0].id);
    const wer = { menschId: m, agentId: Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,'auf-agent','auf-agent') returning id", [m])).rows[0].id) };
    const stufe = async () => Number((await db.query("select vertrauensstufe from menschen where id = $1", [m])).rows[0].vertrauensstufe);
    const ids: number[] = [];
    for (let i = 1; i <= 3; i++) {
      const a = await k.aufgabeAnlegen(db, lead, "test", { ...neueAufgabe(`Aufstieg ${i}`), meilenstein: "M1", neuer_meilenstein: false });
      await k.reservieren(db, wer, a.id);
      const e = (await k.einreichen(db, wer, a.id, { patch: PATCH_OK.replaceAll("neu.gd", `auf${i}.gd`), zusammenfassung: "fertig", eigene_tests: "ok", verbrauch: {} })) as { einreichung_id: number };
      await k.pruefungStarten(db, e.einreichung_id);
      await k.pruefungMelden(db, e.einreichung_id, "ok", "grün", "sig");
      await k.bewerten(db, lead, e.einreichung_id, "annehmen", "gut");
      expect(await stufe()).toBe(0);
      await k.gemergedMelden(db, e.einreichung_id, `c${i}`);
      ids.push(Number(a.id));
    }
    expect(await stufe()).toBe(1);
    const auf = (await db.query("select daten from ereignisse where art = 'vertrauen_geaendert' and bezug = $1 order by seq", [`mensch:${m}`])).rows;
    expect(auf.at(-1)!.daten).toMatchObject({ von: 0, nach: 1, anlass: "merged" });
    // Regression: nur Lead oder Bereichs-Lead, nur gemergte Aufgaben, mit Grund
    await expect(k.regressionMelden(db, mit, ids[0], "Das Spiel stürzt beim Start ab, seit dieser Beitrag drin ist.")).rejects.toThrow(/lead/);
    await expect(k.regressionMelden(db, lead, ids[0], "kurz")).rejects.toThrow(/at least 20/);
    const r = await k.regressionMelden(db, lead, ids[0], "Das Spiel stürzt beim Start ab, seit dieser Beitrag drin ist.");
    expect(r.vertrauen).toEqual({ von: 1, nach: 0 });
    expect(await stufe()).toBe(0);
    await expect(k.regressionMelden(db, lead, ids[0], "Das Spiel stürzt beim Start ab, seit dieser Beitrag drin ist.")).rejects.toThrow(/already reported/);
    // Der Stand für Konto und Agenten sagt, was fehlt
    const { vertrauenStand } = await import("../src/vertrauen.ts");
    const st = await vertrauenStand(db, m);
    expect(st).toMatchObject({ stufe: 0, gemerged: 3, naechste: { stufe: 1 } });
    expect(st.naechste!.fehlt.join()).toMatch(/no regression for (89|90) more days/);
  });
});

describe("Bereichs-Leads automatisch (E-35)", () => {
  it("lädt bei Last den passenden Kandidaten ein; Zusage, Freigabe, Stufe 3; ruhige Amtszeiten enden von selbst", async () => {
    const b = await import("../src/bereichsleads.ts");
    const { OFFICER_SCHWELLEN } = await import("../src/kennzahlen.ts");
    const neu = async (login: string) => {
      const m = Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea',$1,$1) returning id", [login])).rows[0].id);
      return { menschId: m, agentId: Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,$2,$2) returning id", [m, login + "-agent"])).rows[0].id) };
    };
    const kand = await neu("kandidatin"), vis = await neu("bl-vis");
    await db.query("insert into projekt_rollen select id, $1, 'visionaer' from projekte where slug = 'test'", [vis.menschId]);
    const rolle = { meilenstein: "M1", neuer_meilenstein: false, beschreibung: "Geschichte", akzeptanz: ["ok"], stufe: "B" as const, faehigkeiten: ["text.story"], pfade: ["game/story/**"], rolle: "narrative" };
    for (let i = 1; i <= 2; i++) {
      const a = await k.aufgabeAnlegen(db, lead, "test", { ...rolle, titel: `Szene ${i}` });
      await k.reservieren(db, kand, a.id);
      const e = (await k.einreichen(db, kand, a.id, { patch: PATCH_OK.replaceAll("game/modules/vfx/neu.gd", `game/story/s${i}.md`), zusammenfassung: "Szene", eigene_tests: "ok", verbrauch: {} })) as { einreichung_id: number };
      await k.pruefungStarten(db, e.einreichung_id);
      await k.pruefungMelden(db, e.einreichung_id, "ok", "grün", "sig");
      await k.bewerten(db, lead, e.einreichung_id, "annehmen", "gut");
      await k.gemergedMelden(db, e.einreichung_id, `s${i}`);
    }
    await k.aufgabeAnlegen(db, lead, "test", { ...rolle, titel: "Szene 3" }); // noch offen
    // Schwellen für den Test niedrig, damit die Bilanz die Rolle empfiehlt
    const alt = { ...OFFICER_SCHWELLEN };
    Object.assign(OFFICER_SCHWELLEN, { offen_je_rolle: 0, einreichungen_je_rolle_30t: 1 });
    try {
      // Ohne Stufe 2 niemand
      expect(await b.kandidaten(db, Number((await db.query("select id from projekte where slug='test'")).rows[0].id), "narrative")).toEqual([]);
      await db.query("update menschen set vertrauensstufe = 2 where id = $1", [kand.menschId]);
      expect(await b.vorschlaegeErzeugen(db)).toBeGreaterThanOrEqual(1);
      expect(await b.vorschlaegeErzeugen(db)).toBe(0); // nicht doppelt
    } finally { Object.assign(OFFICER_SCHWELLEN, alt); }
    const meine = await b.bereichsLeadsFuer(db, kand.menschId);
    const einl = meine.einladungen.find((x) => x.bereich === "narrative")!;
    expect(einl).toMatchObject({ projekt: "test", pfade: ["game/story/**"], faehigkeiten: ["text.story"] });
    await expect(b.einladungBeantworten(db, vis.menschId, einl.id, true)).rejects.toThrow(/no longer open/);
    expect(await b.einladungBeantworten(db, kand.menschId, einl.id, true)).toEqual({ state: "accepted, waiting for the visionary" });
    // Der Visionär sieht den Vorschlag und gibt frei
    const offen = (await b.bereichsLeadsFuer(db, vis.menschId)).freigaben.find((x) => x.bereich === "narrative")!;
    expect(offen.login).toBe("kandidatin");
    await expect(b.vorschlagEntscheiden(db, kand.menschId, offen.id, true)).rejects.toThrow(/nothing to decide/);
    expect(await b.vorschlagEntscheiden(db, vis.menschId, offen.id, true)).toEqual({ state: "appointed" });
    expect((await db.query("select vertrauensstufe from menschen where id = $1", [kand.menschId])).rows[0].vertrauensstufe).toBe(3);
    const arten = (await db.query("select art, akteur from ereignisse where bezug = $1 and art like '%ereich%' or (bezug = $1 and art = 'officer_ernannt') order by seq", [`mensch:${kand.menschId}`])).rows;
    expect(arten.map((r) => r.art)).toEqual(["bereichs_lead_vorgeschlagen", "bereichs_lead_zugesagt", "officer_ernannt"]);
    expect(arten.at(-1)!.akteur).toBe(`mensch:${vis.menschId}`);
    // Automatik: Zusage genügt
    expect(await b.automatikSetzen(db, vis.menschId, "test", true)).toMatchObject({ automatisch: true });
    await expect(b.automatikSetzen(db, kand.menschId, "test", true)).rejects.toThrow(/only the project's visionary/);
    // Ruhige Amtszeit: ernannt vor 30 Tagen, seitdem nichts im Projekt → endet von selbst
    const still = await neu("still");
    await db.query("update menschen set vertrauensstufe = 2 where id = $1", [still.menschId]);
    await k.officerErnennen(db, "betrieb", "test", "still", "economy", ["game/economy/**"], ["code.gdscript"]);
    await db.query("update officers set ernannt = now() - interval '30 days' where mensch_id = $1", [still.menschId]);
    expect(await b.aufraeumen(db)).toBeGreaterThanOrEqual(1);
    expect((await db.query("select beendet from officers where mensch_id = $1", [still.menschId])).rows[0].beendet).not.toBeNull();
    expect((await db.query("select vertrauensstufe from menschen where id = $1", [still.menschId])).rows[0].vertrauensstufe).toBe(2);
  });
});
