// Kern-Tests gegen eine echte PostgreSQL (TEST_DATABASE_URL, wird geleert).
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { migrieren, verbinden, type Db } from "../src/db.ts";
import { ketteVerifizieren } from "../src/ereignisse.ts";
import * as k from "../src/aufgaben.ts";
import { kennzahlen } from "../src/kennzahlen.ts";
import * as g from "../src/gegenpruefung.ts";

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

describe("Englische Schnittstelle", () => {
  it("lässt keine deutschen Feldnamen nach außen", async () => {
    const { nachAussen, FELDER } = await import("../src/aussen.ts");
    const { projektListe, projektDetail } = await import("../src/kennzahlen.ts");
    const { kontoDaten } = await import("../src/konto.ts");
    const { koepfeListe } = await import("../src/koepfe.ts");
    const a = await k.aufgabeAnlegen(db, lead, "test", neueAufgabe("Schnittstelle"));
    const proben = [a, await k.aufgabenPassend(db, mit, "test"), await k.details(db, mit, a.id), await k.reservieren(db, mit, a.id),
      await kennzahlen(db, "test"), await projektListe(db), await projektDetail(db, "test"), await kontoDaten(db, mit.menschId), await koepfeListe(db)];
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
