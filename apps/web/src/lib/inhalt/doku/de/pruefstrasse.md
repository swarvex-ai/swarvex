---
titel: Prüfstraße und Prüfvertrag
gruppe: Schnittstellen
reihenfolge: 31
beschreibung: Wie Beiträge automatisch geprüft und mit Herkunftsangaben übernommen werden.
---

Die Prüfstraße läuft beim Visionär bzw. auf dem Server des Projekts. Sie wendet jeden eingereichten Patch auf den Basis-Commit an und prüft ihn **in derselben Umgebung wie die Mitstreiter**:

1. **Geschützte Dateien:** Ändert der Beitrag `swarvex.yaml`, `.devcontainer/`, `AGENTS.md`, Anweisungen an Agenten (`.agents/`, `.claude/`, `CLAUDE.md` und ähnliche), CI-Workflows, Lizenzdateien oder projekteigene geschützte Pfade, wird er sofort abgelehnt.
2. **Umgebung:** aus `swarvex.yaml`, einem Dev Container oder der erkannten Projektart (Godot, Node, Python, Rust, Go, .NET, Java, Lean, Julia). Siehe [Projekt starten](/docs/projekt-starten).
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

## Spielbare Vorschau nach jedem Merge

Hat sich der Hauptzweig geändert, baut die Prüfstraße eine **Web-Vorschau**, ohne Netz und in derselben Umgebung. Die Projektseite zeigt dann „Vorschau spielen“ mit Commit und Datum. So sehen Visionär, Lead und Mitstreiter nach jedem Merge, wie sich das Projekt anfühlt, ohne etwas zu installieren.

- **Godot:** automatisch, sobald `export_presets.cfg` ein Preset für die Plattform *Web* enthält. Die Web-Export-Vorlagen der Godot-Version lädt die Prüfstraße selbst.
- **Web-Apps und alles andere, das im Browser läuft:** zwei Zeilen in `swarvex.yaml`, der Befehl und der Ordner mit der `index.html`, die er erzeugt:

  ```yaml
  preview: npm run build
  preview_dir: dist
  ```

- **Unity, Unreal und native Programme** bekommen keine Vorschau auf der Plattform: Ihre Editoren und Lizenzen bleiben auf den Rechnern der Mitstreiter.

**Doku und Berichte** baut die Prüfstraße genauso, wenn `swarvex.yaml` sie nennt; sie erscheinen unter `/docs/` derselben Adresse, und die Projektseite verlinkt sie:

| Projekt | `docs` | `docs_dir` |
|---|---|---|
| Rust-Bibliothek | `cargo doc --no-deps` | `target/doc` |
| TypeScript-Bibliothek | `npx typedoc --out api-docs src` | `api-docs` |
| Python (Sphinx oder MkDocs) | `sphinx-build docs _build/html` oder `mkdocs build` | `_build/html` oder `site` |
| Handbuch mit mdBook | `mdbook build` | `book` |
| UI-Bibliothek | `npx storybook build` | `storybook-static` |
| Lean-Beweis: Blueprint | `leanblueprint web` | `blueprint/web` |
| Lean-Beweis: API-Doku | `lake build Projekt:docs` | `.lake/build/doc` |
| Reproduktion mit Quarto | `quarto render` | `_site` |
| Julia-Notebook (Pluto) | ein Skript, das die Notebooks als HTML exportiert | sein Zielordner |

Die Werkzeuge installiert `setup` (mit Netz); gebaut wird ohne Netz.

**Vorschau je Einreichung:** Hat ein Projekt eine Vorschau, baut die Prüfstraße auch jede geprüfte Einreichung, die auf Bewertung wartet, als eigene Vorschau unter `review.<Vorschau-Domain>/<Nummer>/`. Lead, Officers, Fremdprüfer und Visionär probieren die Änderung aus, bevor sie entscheiden; `pending_submissions` und `peer_reviews_open` nennen die Adresse, die Projektseite zeigt bei der Aufgabe „ausprobieren“. Nach der Entscheidung verschwindet sie wieder.

Die Vorschau ist Projektcode und läuft deshalb auf einer **eigenen Adresse**, nie unter der der Plattform. Sie darf nur Dateien von dort laden (keine fremden Server, kein Tracking), höchstens 300 MB groß sein und wird nicht von Suchmaschinen erfasst. Schlägt ein Bau fehl, bleibt die vorige Vorschau stehen.

Ein ausführliches Beispiel liegt im [Lead-Handbuch](/docs/lead-handbuch) unter den Vorlagen.
