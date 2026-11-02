# Register der Behauptungen für Forschungsprojekte: `docs/claims.md`

> Für Forschungsprojekte (Grundpaket R-1). Das Register sagt, was das Projekt zeigen will und wie eine Maschine oder ein ausdrückliches Verfahren darüber entscheidet. Jede Aufgabe nennt die Behauptung, der sie dient; die Ergebnistabelle wird daraus erzeugt. Toleranzen werden **vor** dem Messen festgelegt und danach nie verschoben. Deutsche Übersetzung; maßgeblich ist `lead/templates/research-claims.md`.

## Register

| ID | Quelle | Behauptung in unseren Worten | Abnahme | Toleranz | Hängt ab von | Status |
|---|---|---|---|---|---|---|
| C-1 | Smith et al. 2025, Tabelle 2, Zeile 3 | Die Methode erreicht eine Top-1-Genauigkeit von 81,4 % auf dem Testsplit | `make reproduce-table2` meldet den Wert | ± 0,5 Prozentpunkte (das Paper nennt ± 0,3 über 5 Seeds) | C-0 | offen |
| T-4 | Hardy & Wright, Satz 80 | Jede Primzahl p ≡ 1 (mod 4) ist Summe zweier Quadrate | `lake build` grün, kein sorry, `#print axioms` zeigt nur `propext`, `Classical.choice`, `Quot.sound` | – | T-2, T-3 | in Arbeit |
| S-7 | Issue #123 der Upstream-Bibliothek | Das Einlesen einer 1-GB-Datei braucht höchstens 200 MB Speicher | Benchmark `bench/parse_large` in der Prüfstraße | ≤ 200 MB | – | bestätigt |

**Status:** `offen`, `in Arbeit`, `bestätigt` (Abnahme innerhalb der Toleranz erfüllt), `widerlegt` (das Verfahren lief, die Behauptung gilt nicht; das ist ein Ergebnis), `Abweichung` (gilt nur mit einer dokumentierten Änderung, siehe `docs/deviations.md`), `außerhalb des Umfangs`.

## Regeln

- **Quelle genau:** Paper mit DOI oder arXiv-Kennung und Abschnitt, Tabelle, Abbildung oder Gleichung; Satz mit seiner Nummer in einer genannten Ausgabe; Software mit Repo und Revision.
- **Unsere Worte, die Bedeutung der Quelle:** Die wissenschaftliche Durchsicht prüft jede Behauptung an ihrer Quelle, bevor Aufgaben darauf aufbauen.
- **Wo möglich maschinell entscheidbar:** ein Beweisprüfer, ein Skript mit Toleranz, ein Benchmark. Jede Behauptung, die die Maschine allein entscheidet, ist eine Bewertung weniger, auf die der Schwarm wartet.
- **Negative Ergebnisse zählen:** Eine widerlegte Behauptung kommt mit derselben Sorgfalt in die Ergebnistabelle wie eine bestätigte.
- **IDs bleiben:** Eine ID wird nie wiederverwendet; eine geänderte Behauptung bekommt eine neue Version (`C-1.2`), die alte bleibt mit Status `außerhalb des Umfangs` und Begründung stehen.

## Ergebnistabelle (`results/summary.md`, erzeugt)

| ID | Erwartet | Gemessen | Toleranz | Status | Lauf |
|---|---|---|---|---|---|
| C-1 | 81,4 % | 81,1 % | ± 0,5 pp | bestätigt | `results/table2/2026-11-03.json` |

Vom Reproduktionsbefehl aus Register und Laufergebnissen erzeugt; eine von Hand bearbeitete Ergebnistabelle wird in der Bewertung abgelehnt.

## Abweichungen (`docs/deviations.md`)

Ein Eintrag je Annahme oder Änderung: was die Quelle sagt oder offenlässt, was wir getan haben, warum, und welche Behauptungen es betrifft.
