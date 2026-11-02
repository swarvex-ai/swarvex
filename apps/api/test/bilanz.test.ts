// Schwarm-Bilanz (src/bilanz.ts): Rechnung aus Einreichungen, nur Messungen, ohne Datenbank.
import { describe, expect, it } from "vitest";
import { gruppenBilanz } from "../src/bilanz.ts";

const z = (o: Record<string, unknown>) => ({ einreichung: 1, aufgabe: 1, stufe: "B", meilenstein: "M1", mensch: 2, agent: 20, modell: "m1", sekunden: 3600,
  urteil: "annehmen", bewertet: new Date("2026-11-02T10:00:00Z"), bewerter: 1, pruefung: "ok", zustand: "gemerged", ...o }) as never;

describe("Schwarm-Bilanz", () => {
  it("rechnet Schwarm gegen einen einzelnen Agenten, nur aus Messungen", () => {
    const start = new Date("2026-11-02T08:00:00Z");
    const b = gruppenBilanz([
      z({}),                                                                                         // Aufgabe 1 (B): 1 h, angenommen
      z({ einreichung: 2, aufgabe: 2, stufe: "C", agent: 21, mensch: 3, modell: "m2", sekunden: 5400, urteil: "nacharbeit", zustand: "angenommen" }),
      z({ einreichung: 3, aufgabe: 2, stufe: "C", agent: 21, mensch: 3, modell: "m2", sekunden: 1800, bewertet: new Date("2026-11-02T09:30:00Z"), zustand: "angenommen" }),
      z({ einreichung: 4, aufgabe: 3, stufe: "A", agent: 22, sekunden: 7200, urteil: null, bewertet: null, zustand: "eingereicht" }),
    ], start)!;
    expect(b.aufgaben_fertig).toBe(2);
    expect(b.einheiten).toBe(5);                  // B 2 + C 3; die offene Aufgabe zählt nicht
    expect(b.schwarm).toEqual({ kalender_stunden: 2, agenten_stunden: 5, agenten: 3, menschen: 2, modelle: 2,
      gleichzeitig: 2.5, agentenstunden_je_tag: 60, einheiten_je_tag: 60 });
    expect(b.ein_agent.stunden).toBe(3);          // 1 h + 1,5 h Nacharbeit-Versuch + 0,5 h
    expect(b.beschleunigung).toBe(1.5);           // 3 h nacheinander gegen 2 h im Kalender
    expect(b.vier_augen_prozent).toBe(100);
    expect(b.pruefstrasse_prozent).toBe(100);
    expect(JSON.stringify(b)).not.toMatch(/menschen_team|stunden_geschaetzt|cocomo/);
  });
  it("zählt Selbstbewertung nicht als vier Augen und liefert ohne Erledigtes nichts", () => {
    expect(gruppenBilanz([z({ bewerter: 2 })], new Date("2026-11-02T08:00:00Z"))!.vier_augen_prozent).toBe(0);
    expect(gruppenBilanz([z({ zustand: "eingereicht", urteil: null })], null)).toBeNull();
  });
});
