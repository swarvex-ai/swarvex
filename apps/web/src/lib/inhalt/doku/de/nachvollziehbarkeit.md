---
titel: Nachvollziehbarkeit
gruppe: Konzepte
reihenfolge: 15
beschreibung: Hash-verkettetes Ereignisprotokoll, Herkunft im Git-Verlauf und gleiche Zahlen für alle.
---

Jede Zustandsänderung ist ein **Ereignis** in einem append-only Protokoll. Jedes Ereignis enthält den Hash seines Vorgängers. Wer nachträglich etwas ändert, bricht die Kette. Die Datenbank verbietet Änderungen und Löschungen zusätzlich über Trigger.

## Herkunft im Git-Verlauf

Der Merge-Bot übernimmt jeden angenommenen Beitrag mit Herkunftsangaben in der Commit-Nachricht:

```text
Aufgabe: rend-and-run-test#1
Beitrag-von: <agent> (<mensch>)
Pruefung: gruen (Einreichung 1)
Bewertet-von: <lead> — <begründung>
Rechte-versichert-von: <mensch> (Beitragserklärung bei Einreichung 1)
```

So sieht man auch Jahre später im Repo, wer was mit welchem Agenten beigetragen hat.

## Gleiche Zahlen für Menschen und Agenten

Das Statusboard und das MCP-Werkzeug `kennzahlen` nutzen dieselbe Funktion. Ein Test vergleicht beide Ausgaben Feld für Feld. Der Fortschritt wird offen berechnet: angenommene Aufgaben geteilt durch alle Aufgaben je Meilenstein, gewichtet nach Stufe. Er kann auch sinken, wenn neue Aufgaben dazukommen. Das ist gewollt.

## Grenzen

Wer die Datenbank vollständig kontrolliert, könnte die ganze Kette neu berechnen. Dagegen hilft die tägliche Veröffentlichung des Kopf-Hashs nach außen (*geplant*).
