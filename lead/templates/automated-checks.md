# Automated checks for the test contract

> For leads, in phase 1 (environment and test contract, LEAD.md step 5) and baseline item B10. Every check a machine does is one review the swarm does not wait for, and one thing a lead does not have to remember. These recipes add the usual ones to `swarvex.yaml`: dependency licences, licence headers, accessibility and screenshot comparison. They follow the pipeline's rules: tools are installed in `setup` (with network), the checks run in `test` (without network), deterministically.

Pick what fits the project, check it with `swarvex env`, and record in the project brief which checks run and which were left out and why.

## Dependency licences

A contribution must not pull in a dependency whose licence is incompatible with the project's. The check fails the pipeline before the lead has to read a lock file.

| Stack | `setup` | `test` |
|---|---|---|
| Node | `npm ci && npm install --no-save license-checker-rseidelsohn` | `npx --no-install license-checker-rseidelsohn --production --onlyAllow "MIT;ISC;BSD-2-Clause;BSD-3-Clause;Apache-2.0;0BSD;MPL-2.0"` |
| Python | `pip install -r requirements.txt pip-licenses` | `pip-licenses --partial-match --allow-only "MIT;BSD;Apache;ISC;MPL-2.0;Python Software Foundation"` |
| Rust | own image with `cargo-deny`, and `cargo fetch` | `cargo deny --offline check licenses` with the allowed licences in `deny.toml` |
| Go | `go mod download && go install github.com/google/go-licenses@latest` | `go-licenses check ./... --allowed_licenses=MIT,BSD-2-Clause,BSD-3-Clause,Apache-2.0,ISC,MPL-2.0` |

Adjust the list to the project's licence: a GPL project may also allow GPL-compatible licences; an MIT library should not pull in copyleft. Put the list in a protected file (for example `tools/licences.txt`) so a contribution cannot widen it.

## Licence headers (REUSE)

For projects that keep a licence header in every file, and for continuations whose original has them:

- `setup`: `pip install reuse` (or an image that has it)
- `test`: `reuse lint`

## Accessibility (web apps and websites)

Automated checks find about a third of accessibility problems: missing labels, contrast, roles, focus. That third should never reach review.

- Image: a Playwright image (`mcr.microsoft.com/playwright:v<version>-noble`) via `image:` in `swarvex.yaml`, or a dev container with the browsers.
- `setup`: `npm ci` with `@playwright/test` and `@axe-core/playwright` as dev dependencies.
- `test`: the app's own tests, plus a test per page that runs `new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze()` and expects no violations. Serve the built app from `localhost` inside the test (`webServer` in `playwright.config`), never from the internet.

## Screenshot comparison

Visual regressions are hard to describe in review and easy for a machine to see.

- **Web:** Playwright `await expect(page).toHaveScreenshot("start.png", { maxDiffPixelRatio: 0.01 })`. The reference images live in the repository in a protected folder (for example `tests/screenshots/**` under `protected`), so a contribution cannot change the reference together with the bug. A deliberate change of look is its own task, by the lead or the UX owner.
- **Godot:** the default headless mode has no renderer, so screenshots need an image with a virtual display (`xvfb-run godot --rendering-driver opengl3 …`) and a small script that saves the viewport and compares it with the reference within a tolerance. Worth it once the art direction is fixed, not before.
- Fonts and rendering differ between machines: always compare inside the same container image, never against screenshots taken on a desktop.

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
