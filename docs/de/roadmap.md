# Bauplan

Den aktuellen Stand führen die Issues und Meilensteine im Tracker. Diese Datei zeigt die Struktur.

Reihenfolge: **P Machbarkeitsnachweis (lokal, kostenlos)** → M0 → M1 → **M2 Durchstich** (alles einmal echt von Ende zu Ende) → M3–M7 (teils parallel) → M8 Pilot.


## P Machbarkeitsnachweis (lokal)

Abgeschlossen am 25.10.2026 mit „Go“.

- Testumgebung auf dem Heimserver  `umsetzung kern`
- Minimal-Plattform: Aufgaben, Leases, Ereignisprotokoll, MCP-Gateway mit OAuth  `umsetzung kern mcp`
- Lead-Anbindung: Claude Code per MCP  `umsetzung mcp lead`
- Runner-Prototyp: Device-Login, Podman-Sandbox, Schlüssel-Proxy, opencode + Ollama  `umsetzung runner sicherheit`
- Prüfstraße: Gitea Actions + Godot headless, signiertes Ergebnis, Merge-Bot  `umsetzung pruefstrasse`
- Angriffstests S1–S10  `umsetzung sicherheit`
- Einzeiler-Test in frischer VM  `umsetzung runner`
- Go/No-Go-Bericht  `entscheidung`

## M0 Grundlagen

Offene Grundsatzentscheidungen, Recht, Bedrohungsmodell. Ohne diese Punkte wird nichts gebaut, was später umgebaut werden müsste.

- E-01 Name und Domain festlegen  `entscheidung recht`
- E-06 Identitätslösung wählen (Zitadel vs. Bibliothek)  `entscheidung sicherheit`
- E-08 Objektspeicher wählen (Garage vs. SeaweedFS)  `entscheidung kern`
- E-13 Lizenz der Plattform und des Runners festlegen  `entscheidung recht`
- ~~E-16 Nutzungsbedingungen von Diensten und Abos prüfen~~ (entschieden: Rechte liegen bei den Beitragenden)  `entscheidung recht spenden`
- ~~E-17 Rend&Run als Open-Source-Pilot prüfen~~ (entschieden: Rend&Run)  `entscheidung pilot recht`
- Bedrohungsmodell ausarbeiten  `spezifikation sicherheit`

## M1 Spezifikation

Protokoll, Schemata, Zustandsmaschine, Nachvollziehbarkeit – schriftlich und als JSON-Schema, bevor Code entsteht.

- Aufgaben-Lebenszyklus als Zustandsmaschine spezifizieren  `spezifikation kern`
- Aufgaben-Schema und Prüfvertrag (swarvex.yaml) spezifizieren  `spezifikation kern pruefstrasse`
- MCP-Werkzeuge pro Rolle spezifizieren  `spezifikation mcp`
- Ereignisprotokoll und Hash-Kette spezifizieren  `spezifikation kern`
- Herkunft: Git-Trailer und Asset-Manifest spezifizieren  `spezifikation pruefstrasse`

## M2 Durchstich

Eine einzige Aufgabe läuft echt von Ende zu Ende: Lead legt an → Runner (Linux) bearbeitet in Sandbox → Prüfstraße (Godot) prüft → Lead nimmt an → Merge-Bot merged, alles protokolliert. Minimal, aber mit den endgültigen Komponenten.

- Monorepo-Gerüst mit CI  `umsetzung kern`
- PostgreSQL-Schema und Migrationen (Kern)  `umsetzung kern`
- Identität anbinden (Login + Device Flow)  `umsetzung sicherheit`
- Koordinations-Kern: Aufgaben, Leases, Zeitüberschreitungen  `umsetzung kern`
- Ereignisprotokoll mit Hash-Kette  `umsetzung kern`
- MCP-Gateway minimal (Streamable HTTP + OAuth)  `umsetzung mcp`
- Runner minimal (Linux): Login, Sandbox, Patch  `umsetzung runner`
- Prüfstraße minimal: Godot-Adapter + Gitea-Actions-Workflow  `umsetzung pruefstrasse`
- Merge-Bot mit Trailern und DCO  `umsetzung pruefstrasse`
- Durchstich-Test mit Rend&Run-Kopie  `umsetzung pilot`

## M3 Kern und Sicherheit

Den Durchstich belastbar machen: vollständige Werkzeuge, Leases, Stichproben, Runner für macOS/Windows, Härtung.

- MCP-Werkzeuge vollständig  `umsetzung mcp`
- Direkt-Modus für Text-Aufgaben  `umsetzung mcp`
- Runner: Zeitplan, Budgetwächter, Dienstbetrieb  `umsetzung runner`
- Runner: weitere Engines (Claude Code, Codex, Hermes)  `umsetzung runner`
- Runner für macOS und Windows  `umsetzung runner`
- ~~Runner: proprietäre Engines vom Rechner des Beitragenden (Unity, Unreal; E-31)~~ (gebaut: eigener Testlauf und Gegenprüfung; Test mit echtem Unity und Unreal steht aus)  `umsetzung runner`
- Installer-Einzeiler und Release-Pipeline  `umsetzung runner sicherheit`
- Stichproben-Prüfung auf Plattform-Hardware  `umsetzung pruefstrasse sicherheit`
- Betrieb: Compose-Produktion, Backups, Monitoring  `umsetzung kern`

## M4 Spenden

Credits und Kapazität spenden: Budgets, Adapter (Meshy), Auftrag+Upload, Asset-Prüfung, Lizenzliste.

- Budgets und Zusagen pro Dienst  `umsetzung spenden`
- Asset-Adapter Meshy (Vorschau → Freigabe → Finale)  `umsetzung spenden`
- Auftrag + Upload (beliebige Dienste)  `umsetzung spenden`
- Asset-Prüfung (Import, Polygone, Skelett, Screenshot)  `umsetzung pruefstrasse spenden`
- Beitragserklärung, Tarif-Hinweis und Meldeformular  `umsetzung spenden recht`

## M5 Lead und Erwartungen

Lead-Test, Erwartungsrechner, Kennzahlen, Mehrfachvergabe.

- Lead-Test: Aufgabensatz und Auswertung  `umsetzung lead`
- Erwartungsrechner  `umsetzung lead`
- Kennzahlen pro Projekt und Mitstreiter  `umsetzung lead`
- Mehrfachvergabe und Wettbewerb  `umsetzung lead kern`

## M6 Website

Öffentliche Seiten, Dashboards für beide Seiten, Projekt-Assistent.

- Website-Gerüst und öffentliche Seiten  `umsetzung web`
- Mitmachen: Einzeiler, Zuordnungen, Budgets  `umsetzung web`
- Dashboards: Meine Beiträge und Projekt-Cockpit  `umsetzung web`
- Projekt-starten-Assistent  `umsetzung web`
- Regeln, Transparenz, Doku  `umsetzung web`

## M7 Vertrauen und Missbrauch

Vertrauensstufen, Identität, Ratenlimits, Diff-Regeln, Meldungen.

- Vertrauensstufen: Auf- und Abstieg  `umsetzung sicherheit`
- Identität, Ratenlimits, Sperren, Meldungen  `umsetzung sicherheit`
- Diff-Regeln: geschützte Pfade und verbotene APIs  `umsetzung sicherheit pruefstrasse`

## M8 Pilot

Rend&Run als erstes öffentliches Projekt mit ersten externen Teilnehmern; Auswertung.

- Rend&Run schwarmfähig machen (Module, Verträge, Tests)  `umsetzung pilot`
- Rend&Run öffentlich auf der Plattform  `umsetzung pilot`
- Pilot-Auswertung  `umsetzung pilot lead`
