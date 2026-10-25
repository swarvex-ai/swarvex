---
titel: Prüfstraße und Prüfvertrag
gruppe: Schnittstellen
reihenfolge: 31
beschreibung: Wie Beiträge automatisch geprüft und mit Herkunftsangaben übernommen werden.
---

Jedes Projekt legt einen Prüfvertrag `pruefung.yaml` ins Repo. Die Prüfstraße wendet jeden eingereichten Patch auf einen Prüfzweig an und führt die Schritte aus, in einem Container **ohne Netz**. Das Ergebnis meldet sie signiert an die Plattform.

```yaml
engine: godot-4.7
schritte:
  - name: import
    run: godot --headless --import
  - name: tests
    run: tools/run_tests.sh --headless
  - name: screenshots
    run: tools/capture.sh
    artefakte: [outputs/screens/*.png]
pfade_geschuetzt: [pruefung.yaml, AGENTS.md, project.godot, tools/**]
grenzen: {zeilen: 500, dateien: 20}
```

- **Geschützte Pfade** ändern nur Lead und Visionär. Der Prüfvertrag kann sich also nicht selbst aufweichen.
- Nimmt der Lead an, übernimmt der **Merge-Bot** den Beitrag. Er hält als Einziger einen Schreibschlüssel und schreibt Herkunftsangaben in den Commit.
- *Geplant:* Stichproben auf Plattform-Hardware. swarvex wiederholt einen Teil der Prüfungen und markiert Abweichungen öffentlich.

Ein ausführliches Beispiel liegt im [Lead-Handbuch](/docs/lead-handbuch) unter den Vorlagen.
