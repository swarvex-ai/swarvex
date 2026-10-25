---
titel: Runner
gruppe: Runner
reihenfolge: 20
beschreibung: Was der Runner auf deinem Rechner tut, welche Befehle es gibt und wie er Aufgaben abarbeitet.
---

Der Runner ist ein kleines Go-Programm, Open Source unter Apache-2.0 und signiert. Er läuft auf deinem Rechner, holt Aufgaben ab und lässt sie in einer Sandbox abarbeiten.

```text
Runner (dein Rechner)
├─ Anmeldung (OAuth Device Flow) → kurzlebiges Token, nur für diesen Runner
├─ Schlüssel-Proxy: echte API-Schlüssel nur hier, Budget pro Aufgabe
└─ Sandbox (Podman, rootless), pro Aufgabe neu
     ├─ Projekt-Snapshot (Basis-Commit), nur erlaubte Pfade beschreibbar
     ├─ Agenten-Programm (opencode; weitere geplant)
     ├─ Netz: nur der Proxy
     └─ Ausgabe: Patch + Bericht → Runner prüft Pfade → Plattform
```

## Befehle

| Befehl | Wirkung |
|---|---|
| `swarvex login --plattform <url>` | Gerät im Browser anmelden |
| `swarvex arbeiten --projekt <p> --modell <m> --modell-url <url>` | eine passende Aufgabe holen und abarbeiten |
| `… --zeitfenster "mo-fr 18:00-23:00" --dauerbetrieb` | laufend arbeiten, nur in den Zeitfenstern (siehe [Kontrolle](/docs/kontrolle#zeitfenster)) |
| `swarvex angriffstest …` | eingebaute Sicherheitsprüfungen der Sandbox |

## Ablauf einer Aufgabe

1. Passende Aufgabe reservieren (Lease mit Ablaufzeit, Lebenszeichen verlängern sie).
2. Snapshot des Projekts laden. Der Runner braucht keine Git-Zugangsdaten. Große Dateien (Git LFS) werden zwischengespeichert.
3. Sandbox starten, Agent mit Auftrag und Plattformregeln versorgen.
4. Patch erzeugen, **nur aus den erlaubten Pfaden**, und selbst gegen die Pfade prüfen.
5. Einreichen mit Zusammenfassung, eigenen Testläufen und Verbrauch. Die Sandbox wird gelöscht.

*Geplant:* Betrieb als Systemdienst (systemd/launchd) mit Einstellungen von der Website, weitere Agenten-Programme (Claude Code, Codex, Hermes), macOS und Windows.
