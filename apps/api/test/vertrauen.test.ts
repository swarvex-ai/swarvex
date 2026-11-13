// Regeln der Vertrauensstufen ohne Datenbank (vertrauen.ts, E-35).
import { describe, expect, it } from "vitest";
import { fehltFuer, stufeBerechnen, type VertrauenZahlen } from "../src/vertrauen.ts";

const jetzt = Date.parse("2026-11-04T12:00:00Z");
const tageVorher = (t: number) => new Date(jetzt - t * 86_400_000);
const z = (x: Partial<VertrauenZahlen>): VertrauenZahlen => ({ gemerged: 0, abStufeB: 0, fremdpruefungen: 0, ablehnungen: [], regressionen: [], bereichsLead: false, ...x });

describe("Vertrauensstufen", () => {
  it("steigt mit gemergten Beiträgen auf 1, mit Erfahrung auf 2", () => {
    expect(stufeBerechnen(0, null, z({ gemerged: 2 }), jetzt).stufe).toBe(0);
    expect(stufeBerechnen(0, null, z({ gemerged: 3 }), jetzt).stufe).toBe(1);
    expect(stufeBerechnen(1, null, z({ gemerged: 10, abStufeB: 3, fremdpruefungen: 4 }), jetzt).stufe).toBe(1);
    expect(stufeBerechnen(0, null, z({ gemerged: 10, abStufeB: 3, fremdpruefungen: 5 }), jetzt).stufe).toBe(2);
  });
  it("eine frische Ablehnung hält den Aufstieg auf, eine alte nicht", () => {
    expect(stufeBerechnen(0, null, z({ gemerged: 3, ablehnungen: [tageVorher(10)] }), jetzt).stufe).toBe(0);
    expect(stufeBerechnen(0, null, z({ gemerged: 3, ablehnungen: [tageVorher(31)] }), jetzt).stufe).toBe(1);
    expect(fehltFuer(1, z({ gemerged: 3, ablehnungen: [tageVorher(10)] }), jetzt)).toEqual(["no rejection for 20 more days"]);
  });
  it("zwei Ablehnungen in 30 Tagen: eine Stufe ab, aber nur einmal", () => {
    const zahlen = z({ gemerged: 12, abStufeB: 4, fremdpruefungen: 6, ablehnungen: [tageVorher(2), tageVorher(9)] });
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
