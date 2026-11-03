# Automatische Prüfungen für den Prüfvertrag

> Für Leads, in Phase 1 (Umgebung und Prüfvertrag, LEAD.md Schritt 5) und zum Grundpaket-Baustein B10. Jede Prüfung, die eine Maschine macht, ist eine Bewertung, auf die der Schwarm nicht warten muss, und etwas, an das ein Lead nicht denken muss. Diese Rezepte bringen die üblichen in `swarvex.yaml`: Lizenzen der Abhängigkeiten, Lizenzköpfe, Barrierefreiheit und Screenshot-Vergleich. Sie folgen den Regeln der Prüfstraße: Werkzeuge werden in `setup` installiert (mit Netz), die Prüfungen laufen in `test` (ohne Netz), deterministisch. Deutsche Übersetzung; maßgeblich ist `lead/templates/automated-checks.md`.

Wähle, was zum Projekt passt, prüfe es mit `swarvex env` und halte im Projektbrief fest, welche Prüfungen laufen und welche mit welcher Begründung fehlen.

## Lizenzen der Abhängigkeiten

Ein Beitrag darf keine Abhängigkeit einschleppen, deren Lizenz nicht zur Lizenz des Projekts passt. Die Prüfung lässt die Prüfstraße scheitern, bevor der Lead eine Lockdatei lesen muss.

| Stack | `setup` | `test` |
|---|---|---|
| Node | `npm ci && npm install --no-save license-checker-rseidelsohn` | `npx --no-install license-checker-rseidelsohn --production --onlyAllow "MIT;ISC;BSD-2-Clause;BSD-3-Clause;Apache-2.0;0BSD;MPL-2.0"` |
| Python | `pip install -r requirements.txt pip-licenses` | `pip-licenses --partial-match --allow-only "MIT;BSD;Apache;ISC;MPL-2.0;Python Software Foundation"` |
| Rust | eigenes Image mit `cargo-deny`, dazu `cargo fetch` | `cargo deny --offline check licenses` mit den erlaubten Lizenzen in `deny.toml` |
| Go | `go mod download && go install github.com/google/go-licenses@latest` | `go-licenses check ./... --allowed_licenses=MIT,BSD-2-Clause,BSD-3-Clause,Apache-2.0,ISC,MPL-2.0` |

Passe die Liste an die Lizenz des Projekts an: Ein GPL-Projekt darf auch GPL-verträgliche Lizenzen erlauben, eine MIT-Bibliothek sollte kein Copyleft einziehen. Leg die Liste in eine geschützte Datei (zum Beispiel `tools/licences.txt`), damit ein Beitrag sie nicht erweitern kann.

## Lizenzköpfe (REUSE)

Für Projekte, die in jeder Datei einen Lizenzkopf führen, und für Weiterführungen, deren Original welche hat:

- `setup`: `pip install reuse` (oder ein Image, das es mitbringt)
- `test`: `reuse lint`

## Barrierefreiheit (Web-Apps und Websites)

Automatische Prüfungen finden etwa ein Drittel der Barrieren: fehlende Beschriftungen, Kontrast, Rollen, Fokus. Dieses Drittel sollte nie bis zur Bewertung kommen.

- Image: ein Playwright-Image (`mcr.microsoft.com/playwright:v<version>-noble`) über `image:` in `swarvex.yaml`, oder ein Dev Container mit den Browsern.
- `setup`: `npm ci` mit `@playwright/test` und `@axe-core/playwright` als Entwicklungsabhängigkeiten.
- `test`: die eigenen Tests der App und je Seite ein Test, der `new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze()` ausführt und keine Verstöße erwartet. Die gebaute App läuft dabei auf `localhost` im Test (`webServer` in `playwright.config`), nie aus dem Internet.

## Screenshot-Vergleich

Optische Rückschritte lassen sich in einer Bewertung schwer beschreiben und für eine Maschine leicht sehen.

- **Web:** Playwright `await expect(page).toHaveScreenshot("start.png", { maxDiffPixelRatio: 0.01 })`. Die Referenzbilder liegen im Repo in einem geschützten Ordner (zum Beispiel `tests/screenshots/**` unter `protected`), damit ein Beitrag die Referenz nicht zusammen mit dem Fehler ändert. Eine gewollte Änderung des Aussehens ist eine eigene Aufgabe, für den Lead oder die UX-Verantwortung.
- **Godot:** Der Headless-Modus hat keinen Renderer; Screenshots brauchen ein Image mit virtuellem Bildschirm (`xvfb-run godot --rendering-driver opengl3 …`) und ein kleines Skript, das den Viewport speichert und mit der Referenz innerhalb einer Toleranz vergleicht. Lohnt sich, sobald die Art Direction feststeht, nicht vorher.
- Schriften und Darstellung unterscheiden sich zwischen Rechnern: Vergleiche immer im selben Container-Image, nie mit Screenshots vom Desktop.

## In swarvex.yaml

```yaml
setup: |
  npm ci
  npm install --no-save license-checker-rseidelsohn
test: |
  npm test
  npx --no-install license-checker-rseidelsohn --production --onlyAllow "$(cat tools/licences.txt)"
  npx playwright test tests/a11y tests/screenshots
protected: |
  - tools/licences.txt
  - tests/screenshots/**
```
