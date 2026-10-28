---
titel: Prüfstraße und Prüfvertrag
gruppe: Schnittstellen
reihenfolge: 31
beschreibung: Wie Beiträge automatisch geprüft und mit Herkunftsangaben übernommen werden.
---

Die Prüfstraße läuft beim Visionär bzw. auf dem Server des Projekts. Sie wendet jeden eingereichten Patch auf den Basis-Commit an und prüft ihn **in derselben Umgebung wie die Mitstreiter**:

1. **Geschützte Dateien:** Ändert der Beitrag `swarvex.yaml`, `.devcontainer/`, `AGENTS.md`, Anweisungen an Agenten (`.agents/`, `.claude/`, `CLAUDE.md` und ähnliche), CI-Workflows, Lizenzdateien oder projekteigene geschützte Pfade, wird er sofort abgelehnt.
2. **Umgebung:** aus `swarvex.yaml`, einem Dev Container oder der erkannten Projektart (Godot, Node, Python, Rust, Go, .NET, Java). Siehe [Projekt starten](/docs/projekt-starten).
3. **Einrichten:** Abhängigkeiten laden, mit Netz, ohne Agent.
4. **Bauen und Testen:** im Container **ohne Netz**, mit festen Grenzen für Speicher, Prozesse und Zeit.

Das Ergebnis meldet sie signiert an die Plattform. Den Prüfvertrag legt `swarvex.yaml` fest:

```yaml
template: godot
dir: game
test: |
  gdlint src tests
  tools/run_tests.sh --headless
protected: |
  - tools/**
  - tests/vertraege/**
```

- **Geschützte Pfade** ändern nur Lead und Visionär. Der Prüfvertrag kann sich also nicht selbst aufweichen.
- Ohne Testbefehl meldet die Prüfstraße „keine automatischen Tests“, und der Lead prüft von Hand. Für einen echten Schwarm sollte jedes Projekt Tests haben.
- Nimmt der Lead an, übernimmt der **Merge-Bot** den Beitrag. Er hält als Einziger einen Schreibschlüssel und schreibt Herkunftsangaben in den Commit.
- **Unity- und Unreal-Projekte** prüft nicht die Prüfstraße (keine Lizenz im Container), sondern die Runner der Mitstreiter: der eigene Testlauf des Einreichenden, dann eine Gegenprüfung durch den Runner eines anderen Menschen mit derselben Engine. Siehe [Runner](/docs/runner).
- *Geplant:* Stichproben auf Plattform-Hardware. swarvex wiederholt einen Teil der Prüfungen und markiert Abweichungen öffentlich.

Ein ausführliches Beispiel liegt im [Lead-Handbuch](/docs/lead-handbuch) unter den Vorlagen.
