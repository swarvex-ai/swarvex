---
titel: Aufgaben und Stufen
gruppe: Konzepte
reihenfolge: 11
beschreibung: Wie Aufgaben aufgebaut sind, welche Stufen es gibt und wie sie durchs System laufen.
---

Aufgaben verlangen **Fähigkeiten, keine Modelle**, zum Beispiel `code.gdscript`, `test.python`, `asset.3d` oder `review`. Ein Mitstreiter ordnet einmal zu, welches seiner Modelle welche Fähigkeit bedient.

## Stufen

| Stufe | Art | Beispiel | Mindest-Vertrauen |
|---|---|---|---|
| **A** | automatisch prüfbar, ändert keinen Projektcode | Asset erzeugen, Simulation, Text | 0 |
| **B** | isoliertes Modul hinter fertiger Schnittstelle | neuer Gegner, Tests für bestehenden Code | 1 |
| **C** | Änderung in einem System, abgesichert durch Vertragstests | neuer Angriff für einen bestehenden Gegner | 2 |
| **D** | Querschnitt, Architektur | Schadenssystem umbauen | nur Lead |

Die eigentliche Arbeit des Leads ist, so zu schneiden, dass möglichst viel von D nach B rutscht: Erweiterungspunkte statt Monolithen, Verträge und Tests zuerst.

## Was eine Aufgabe enthält

- Ziel und Kontext (zu lesende Dateien, Schnittstellen, Referenzbeispiel)
- Stufe, Fähigkeiten, Mindest-Vertrauensstufe
- **erlaubte Pfade:** Nur dort darf der Patch etwas ändern. Runner und Plattform prüfen das.
- Akzeptanzkriterien, möglichst ausführbar (Tests, Messwerte)
- Basis-Commit und Lease-Dauer
- die **Rolle**, der die Aufgabe dient (z. B. `audio`, `ux`; siehe [Rollen und Officers](/docs/rollen-und-officers)), und ihr Meilenstein
- in Clean-Room-Projekten der **Raum** (`analysis`, `review` oder `build`)

## Lebenszyklus

```text
offen → reserviert → in_arbeit → eingereicht → pruefung
  → geprueft_ok → angenommen → gemerged
  → geprueft_fehler → offen (zurück in den Pool)
  → Nacharbeit oder Ablehnung (nach Bewertung) → offen
reserviert/in_arbeit ohne Lebenszeichen bis Lease-Ende → offen
offen → geschlossen (durch den Lead oder den Officer des Bereichs, mit Begründung)
```

Jeder Übergang ist ein Ereignis im Protokoll. Wo Geschmack oder Ansatz zählen, legt der Lead eine Aufgabe **als mehrere Varianten** an (eigene Aufgaben, „Variante 1/2“ und „2/2“); der Visionär wählt eine oder kombiniert sie, und auch die andere wird genannt.
