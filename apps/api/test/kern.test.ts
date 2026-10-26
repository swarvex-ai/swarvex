// Kern-Tests gegen eine echte PostgreSQL (TEST_DATABASE_URL, wird geleert).
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { migrieren, verbinden, type Db } from "../src/db.ts";
import { ketteVerifizieren } from "../src/ereignisse.ts";
import * as k from "../src/aufgaben.ts";
import { kennzahlen } from "../src/kennzahlen.ts";

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
  await db.query("drop schema public cascade; create schema public;");
  await migrieren(db);
  const mensch = async (login: string) => Number((await db.query("insert into menschen(konto_provider, konto_id, login) values ('gitea',$1,$1) returning id", [login])).rows[0].id);
  const agent = async (m: number, c: string) => Number((await db.query("insert into agenten(mensch_id, client_id, name) values ($1,$2,$2) returning id", [m, c])).rows[0].id);
  lead.menschId = await mensch("nevs"); lead.agentId = await agent(lead.menschId, "claude-code");
  mit.menschId = await mensch("mitstreiterin"); mit.agentId = await agent(mit.menschId, "runner-1");
  fremd.menschId = await mensch("fremd"); fremd.agentId = await agent(fremd.menschId, "runner-2");
  const p = Number((await db.query(
    `insert into projekte(slug,name,vision_md,vision_kurz,engine,lizenz_code,lizenz_assets,repo_url,lead_modell,ziel)
     values ('test','Test','Vision','kurz','godot-4.7','MIT','CC BY 4.0','http://x','Claude Opus 5.5','Demo') returning id`)).rows[0].id);
  await db.query("insert into projekt_rollen values ($1,$2,'lead')", [p, lead.menschId]);
});
afterAll(async () => { await db.end(); });

const neueAufgabe = (titel: string): k.NeueAufgabe => ({
  titel, beschreibung: "Effekt bauen", stufe: "B", faehigkeiten: ["code.gdscript"], pfade: ["game/modules/vfx/**"], akzeptanz: ["Tests grün"], meilenstein: "M1",
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
    await expect(k.bewerten(db, lead, eid, "annehmen", "gut")).rejects.toThrow(/nach grüner Prüfung/);
    await k.pruefungStarten(db, eid);
    await k.pruefungMelden(db, eid, "ok", "alle Tests grün", "sig");
    await expect(k.bewerten(db, mit, eid, "annehmen", "selbst")).rejects.toThrow(/Lead/);
    // Der Lead mit einem zweiten Agenten: auch das ist eine Selbstbewertung, falls er der Autor wäre.
    await db.query("insert into projekt_rollen values ((select id from projekte where slug='test'), $1, 'lead') on conflict do nothing", [mit.menschId]);
    await expect(k.bewerten(db, { menschId: mit.menschId, agentId: lead.agentId }, eid, "annehmen", "selbst")).rejects.toThrow(/eigene Einreichung/);
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
    await expect(k.einreichen(db, mit, a.id, { patch: PATCH_OK, zusammenfassung: "", eigene_tests: "", verbrauch: {} })).rejects.toThrow(/nicht für dich reserviert/);
    await k.reservieren(db, mit, a.id);
    await expect(k.reservieren(db, fremd, a.id)).rejects.toThrow(/reserviert/);
    await expect(k.fortschritt(db, fremd, a.id, "x", {})).rejects.toThrow(/nicht für dich/);
    await expect(k.aufgabeAnlegen(db, mit, "test", neueAufgabe("kein Lead"))).rejects.toThrow(/Lead/);
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
  });
});

describe("Pfadprüfung (S5)", () => {
  it("weist Patches außerhalb der erlaubten Pfade ab und protokolliert das", async () => {
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Pfade"));
    await k.reservieren(db, mit, a.id);
    const boese = PATCH_OK.replaceAll("game/modules/vfx/neu.gd", "tools/run_tests.sh");
    const r = await k.einreichen(db, mit, a.id, { patch: boese, zusammenfassung: "", eigene_tests: "", verbrauch: {} });
    expect(r).toHaveProperty("abgewiesen");
    expect(k.patchPruefen(PATCH_OK.replaceAll("game/modules/vfx/neu.gd", "game/modules/vfx/../../../etc/x"), ["game/modules/vfx/**"]).join()).toMatch(/unzulässiger Pfad/);
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
    await expect(k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "ohne Raum", pfade: ["spezifikation/a.md"] })).rejects.toThrow(/braucht einen Raum/);
    await expect(k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Analyse falsch", raum: "analyse", pfade: ["src/**"] })).rejects.toThrow(/nur unter spezifikation/);
    await expect(k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Bau falsch", raum: "bau", pfade: ["**"] })).rejects.toThrow(/nicht ändern/);
    await expect(k.aufgabeAnlegen(db, lead, "test", { ...basis, titel: "Raum ohne CR", raum: "bau", pfade: ["game/**"] })).rejects.toThrow(/nur in Clean-Room/);
    const analyse = await k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Speichern beschreiben", raum: "analyse", pfade: ["spezifikation/**"] });
    const bau1 = await k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Speichern bauen", raum: "bau", pfade: ["src/**"] });
    const bau2 = await k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Laden bauen", raum: "bau", pfade: ["src/**"] });
    const analyse2 = await k.aufgabeAnlegen(db, lead, "cr", { ...basis, titel: "Laden beschreiben", raum: "analyse", pfade: ["spezifikation/**"] });

    await k.reservieren(db, mit, analyse.id);                               // mit analysiert …
    await expect(k.reservieren(db, mit, bau1.id)).rejects.toThrow(/Clean Room/);   // … und darf nicht bauen
    expect((await k.aufgabenPassend(db, mit, "cr")).map((a) => a.raum)).not.toContain("bau");
    await k.reservieren(db, fremd, bau1.id);                               // fremd baut …
    await expect(k.reservieren(db, fremd, analyse2.id)).rejects.toThrow(/Clean Room/); // … und darf nicht analysieren
    await k.reservieren(db, fremd, bau2.id);                               // weiter bauen geht

    const verweigert = (await db.query("select count(*)::int as n from ereignisse where art = 'clean_room_trennung' and projekt_id = $1", [cr])).rows[0].n;
    expect(verweigert).toBe(2);
    const { projektDetail } = await import("../src/kennzahlen.ts");
    const d = await projektDetail(db, "cr");
    expect(d.clean_room?.trennung_eingehalten).toBe(true);
    expect(d.clean_room?.menschen_je_raum).toMatchObject({ analyse: 1, bau: 1 });
  });
});

describe("Meldungen und Sperren (#19, #29)", () => {
  it("nimmt Meldungen an, prüft sie und protokolliert ohne persönliche Daten", async () => {
    const { meldungAnlegen, meldungenListe, meldungAbschliessen } = await import("../src/meldungen.ts");
    const gut = { art: "rechtsverletzung", projekt: "test", bezug: "https://example.org/datei", beschreibung: "Diese Datei stammt aus meinem Spiel und wurde ohne Erlaubnis übernommen.",
      name: "Rita Rechte", email: "rita@example.org", rechteinhaber: true, versichert: true };
    const { id } = await meldungAnlegen(db, gut);
    await expect(meldungAnlegen(db, { ...gut, versichert: false })).rejects.toThrow();
    await expect(meldungAnlegen(db, { ...gut, beschreibung: "zu kurz" })).rejects.toThrow();
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
    await sperren(db, { projekt: pid }, true, "Takedown wegen Meldung");
    await expect(k.aufgabenPassend(db, mit, "test")).rejects.toThrow(/gesperrt/);
    await expect(k.reservieren(db, mit, offen.id)).rejects.toThrow(/gesperrt/);
    expect((await projektListe(db)).projekte.map((x) => x.projekt)).not.toContain("test");
    await sperren(db, { projekt: pid }, false, "Meldung erledigt");
    expect((await k.aufgabenPassend(db, mit, "test")).map((a) => a.id)).toContain(offen.id);

    await sperren(db, { mensch: fremd.menschId }, true, "Spam-Einreichungen");
    await expect(k.reservieren(db, fremd, offen.id)).rejects.toThrow(/Konto gesperrt/);
    await sperren(db, { mensch: fremd.menschId }, false, "Einspruch berechtigt");
    const arten = (await db.query("select art from ereignisse where art like '%gesperrt' or art like '%entsperrt' order by seq")).rows.map((r) => r.art);
    expect(arten).toEqual(["projekt_gesperrt", "projekt_entsperrt", "mensch_gesperrt", "mensch_entsperrt"]);
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
