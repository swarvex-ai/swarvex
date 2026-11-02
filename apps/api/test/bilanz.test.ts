// Schwarm-Bilanz (src/bilanz.ts): Rechnung aus Einreichungen, ohne Datenbank.
import { describe, expect, it } from "vitest";
import { cocomo, gruppenBilanz } from "../src/bilanz.ts";

const PATCH = (n: number) => `diff --git a/src/x.gd b/src/x.gd\n--- /dev/null\n+++ b/src/x.gd\n@@ -0,0 +1,${n} @@\n` + "+zeile\n".repeat(n);
const z = (o: Record<string, unknown>) => ({ einreichung: 1, aufgabe: 1, meilenstein: "M1", mensch: 2, agent: 20, modell: "m1", sekunden: 3600, urteil: "annehmen",
  bewertet: new Date("2026-11-02T10:00:00Z"), bewerter: 1, pruefung: "ok", zustand: "gemerged", menschen_stunden: 8, patch: PATCH(10), ...o }) as never;

describe("Schwarm-Bilanz", () => {
  it("rechnet Schwarm gegen einen Agenten allein und ein geschätztes Team", () => {
    const start = new Date("2026-11-02T08:00:00Z");
    const b = gruppenBilanz([
      z({}),                                                                                         // Aufgabe 1: 1 h, angenommen
      z({ einreichung: 2, aufgabe: 2, agent: 21, mensch: 3, modell: "m2", sekunden: 5400, urteil: "nacharbeit", patch: null, zustand: "angenommen", menschen_stunden: 6 }),
      z({ einreichung: 3, aufgabe: 2, agent: 21, mensch: 3, modell: "m2", sekunden: 1800, bewertet: new Date("2026-11-02T09:30:00Z"), zustand: "angenommen", menschen_stunden: 6 }),
      z({ einreichung: 4, aufgabe: 3, agent: 22, sekunden: 7200, urteil: null, bewertet: null, zustand: "eingereicht", menschen_stunden: null, patch: null }),
    ], start)!;
    expect(b.aufgaben_fertig).toBe(2);
    expect(b.schwarm).toEqual({ kalender_stunden: 2, agenten_stunden: 5, agenten: 3, menschen: 2, modelle: 2 });
    expect(b.ein_agent.stunden).toBe(3);          // 1 h + 1,5 h Nacharbeit-Versuch + 0,5 h; die offene Aufgabe zählt nicht
    expect(b.beschleunigung).toBe(1.5);           // 3 h nacheinander gegen 2 h im Kalender
    expect(b.menschen_team).toMatchObject({ geschaetzte_stunden: 14, aufgaben_geschaetzt: 2, team: 4, wochen: 0.1, cocomo: null, durchsatz_je_tag: 168 });
    expect(b.vier_augen_prozent).toBe(100);
    expect(b.pruefstrasse_prozent).toBe(100);
  });
  it("zählt Selbstbewertung nicht als vier Augen und liefert ohne Erledigtes nichts", () => {
    const b = gruppenBilanz([z({ bewerter: 2 })], new Date("2026-11-02T08:00:00Z"))!;
    expect(b.vier_augen_prozent).toBe(0);
    expect(gruppenBilanz([z({ zustand: "eingereicht", urteil: null })], null)).toBeNull();
  });
  it("COCOMO erst ab 500 Zeilen", () => {
    expect(cocomo(400)).toBeNull();
    expect(cocomo(10_000)).toEqual({ zeilen: 10_000, personenmonate: 26.9, monate: 8.7, menschen: 3.1 });
  });
});
