// Regeln der Vertrauensstufen ohne Datenbank (vertrauen.ts, E-35).
import { describe, expect, it } from "vitest";
import { ablehnungsQuote, fehltFuer, stufeBerechnen, type VertrauenZahlen } from "../src/vertrauen.ts";

const jetzt = Date.parse("2026-11-04T12:00:00Z");
const tageVorher = (t: number) => new Date(jetzt - t * 86_400_000);
const z = (x: Partial<VertrauenZahlen>): VertrauenZahlen => ({ gemerged: 0, abStufeB: 0, fremdpruefungen: 0, urteile: [], regressionen: [], bereichsLead: false, ...x });
// Urteile, neueste zuerst: abgelehnt vor so vielen Tagen, dazu angenommene (je einer pro Stunde, ganz frisch)
const urteile = (abgelehntVor: number[], angenommen = 0) => [
  ...Array.from({ length: angenommen }, (_, i) => ({ zeit: tageVorher(i / 24), abgelehnt: false })),
  ...abgelehntVor.map((t) => ({ zeit: tageVorher(t), abgelehnt: true })),
].sort((a, b) => b.zeit.getTime() - a.zeit.getTime());

describe("Vertrauensstufen", () => {
  it("steigt mit gemergten Beiträgen auf 1, mit Erfahrung auf 2", () => {
    expect(stufeBerechnen(0, null, z({ gemerged: 2 }), jetzt).stufe).toBe(0);
    expect(stufeBerechnen(0, null, z({ gemerged: 3 }), jetzt).stufe).toBe(1);
    expect(stufeBerechnen(1, null, z({ gemerged: 10, abStufeB: 3, fremdpruefungen: 4 }), jetzt).stufe).toBe(1);
    expect(stufeBerechnen(0, null, z({ gemerged: 10, abStufeB: 3, fremdpruefungen: 5 }), jetzt).stufe).toBe(2);
  });
  it("eine einzelne Ablehnung hält den Aufstieg nicht auf, zu viele schon, alte nicht", () => {
    expect(stufeBerechnen(0, null, z({ gemerged: 3, urteile: urteile([10], 3) }), jetzt).stufe).toBe(1);
    expect(stufeBerechnen(0, null, z({ gemerged: 3, urteile: urteile([10, 12], 3) }), jetzt).stufe).toBe(0);
    expect(stufeBerechnen(0, null, z({ gemerged: 3, urteile: urteile([31, 32], 3) }), jetzt).stufe).toBe(1);
    expect(fehltFuer(1, z({ gemerged: 3, urteile: urteile([10, 12], 3) }), jetzt))
      .toEqual(["too many rejections: 2 of your last 5 reviewed submissions within 30 days (at most 1, or 10 %)"]);
  });
  it("Quote: mindestens 2 und mehr als 10 % der letzten 20 Urteile", () => {
    expect(ablehnungsQuote(urteile([1, 2], 18), 30, jetzt).zuViele).toBe(false); // 2 von 20 = 10 %
    expect(ablehnungsQuote(urteile([1, 2, 3], 17), 30, jetzt).zuViele).toBe(true); // 3 von 20
    // nur die letzten 20 zählen: zwei ältere Ablehnungen hinter 20 angenommenen fallen heraus
    expect(ablehnungsQuote(urteile([2, 3], 20), 30, jetzt)).toMatchObject({ ablehnungen: 0, urteile: 20, zuViele: false });
    // 22.11.2026: viel Arbeit, wenige Ablehnungen (3 unter vielen Annahmen) sperren nicht mehr
    expect(stufeBerechnen(0, tageVorher(8), z({ gemerged: 180, abStufeB: 60, urteile: urteile([5, 7, 9], 183) }), jetzt).stufe).toBe(1);
  });
  it("zu viele Ablehnungen in 30 Tagen: eine Stufe ab, aber nur einmal", () => {
    const zahlen = z({ gemerged: 12, abStufeB: 4, fremdpruefungen: 6, urteile: urteile([2, 9], 3) });
    expect(stufeBerechnen(2, tageVorher(20), zahlen, jetzt).stufe).toBe(1);
    // nach der Abstufung (seit = jetzt) zählen dieselben Ablehnungen nicht noch einmal
    expect(stufeBerechnen(1, new Date(jetzt), zahlen, jetzt).stufe).toBe(1);
  });
  it("Regression: sofort eine Stufe ab, und 90 Tage kein Wiederaufstieg", () => {
    const zahlen = z({ gemerged: 5, regressionen: [tageVorher(0.01)] });
    expect(stufeBerechnen(1, tageVorher(5), zahlen, jetzt).stufe).toBe(0);
    expect(stufeBerechnen(0, new Date(jetzt), zahlen, jetzt).stufe).toBe(0);
    expect(stufeBerechnen(0, new Date(jetzt), z({ gemerged: 5, regressionen: [tageVorher(91)] }), jetzt).stufe).toBe(1);
  });
  it("Bereichs-Leads stehen auf 3, danach wieder berechnet", () => {
    expect(stufeBerechnen(2, null, z({ gemerged: 10, abStufeB: 3, fremdpruefungen: 5, bereichsLead: true }), jetzt).stufe).toBe(3);
    expect(stufeBerechnen(3, tageVorher(1), z({ gemerged: 10, abStufeB: 3, fremdpruefungen: 5 }), jetzt).stufe).toBe(2);
    // Ein Amt ohne verdiente Stufe 2 (Lead-Gesuch, E-39) hebt nichts, und nach dem Amt bleibt nur Verdientes (Audit 12.11.2026)
    expect(stufeBerechnen(0, null, z({ bereichsLead: true }), jetzt).stufe).toBe(0);
    expect(stufeBerechnen(1, null, z({ gemerged: 3, bereichsLead: true }), jetzt).stufe).toBe(1);
    expect(stufeBerechnen(0, null, z({ gemerged: 0 }), jetzt).stufe).toBe(0);
  });
});
