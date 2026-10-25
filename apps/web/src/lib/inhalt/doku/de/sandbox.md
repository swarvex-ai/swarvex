---
titel: Sandbox und Schlüssel-Proxy
gruppe: Runner
reihenfolge: 21
beschreibung: Wie dein Rechner vor fremdem Projektcode geschützt ist und warum der Agent deine Schlüssel nie sieht.
---

Beim Programmieren läuft fremder Code: Tests, Build-Skripte, Importe. Die Sandbox schützt deinen Rechner davor.

## Die Grenzen der Sandbox

| Grenze | Umsetzung |
|---|---|
| ohne Admin-Rechte | rootless Podman, alle Capabilities entfernt, `no-new-privileges`, keine Setuid-Programme im Image |
| kein Zugriff auf deine Dateien | kein Home-Verzeichnis, nur der Projekt-Snapshot, Wurzeldateisystem schreibgeschützt |
| kein Netz | internes Netz, erreichbar ist nur der Schlüssel-Proxy |
| kein Ausufern | Limits für Prozesse, Speicher und Laufzeit; Podman beendet die Sandbox bei Ablauf selbst |
| keine Spuren | die Sandbox wird nach jeder Aufgabe gelöscht |

## Der Schlüssel-Proxy

Der Agent in der Sandbox spricht nicht direkt mit deinem Modell, sondern mit einem kleinen Proxy des Runners:

- Er hat nur ein **Aufgaben-Token**, das ausschließlich für diese Aufgabe gilt.
- Erlaubt sind nur Chat-Anfragen an **das eine freigegebene Modell**.
- Das **Budget** wird hart durchgesetzt. Bei Überschreitung gibt es keine weiteren Antworten.
- Dein echter API-Schlüssel liegt nur beim Proxy. Die Sandbox sieht ihn nie.

## Geprüft, nicht behauptet

In unseren Angriffstests scheiterten zehn Angriffsklassen mit 46 Einzelprüfungen, darunter Schlüsseldiebstahl, Ausbruch ins Netz, Lesen des Home-Verzeichnisses, Prompt-Injection, Container-Ausbruch und Budget-Überschreitung. Details stehen unter [Sicherheit](/docs/sicherheit).
