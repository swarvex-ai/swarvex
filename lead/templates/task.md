# Template: task

> This is how you fill in `create_task`. The fields `title`, `level`, `capabilities`, `paths`, `acceptance`, `base_commit`, `lease_minutes`, `min_trust` and `milestone` are separate parameters. **Everything else goes into `description`**, structured as below. Before creating a task: definition of ready (LEAD.md 6.6).

## Fields

| Field | Rule |
|---|---|
| `title` (title) | verb + object, ≤ 70 characters, unambiguous: "Slime enemy: movement and damage" |
| `level` (level) | A / B / C (D only yourself) |
| `capabilities` (capabilities) | from the vocabulary (LEAD.md 6.3), as few as possible |
| `paths` (paths) | as narrow as possible; never protected paths |
| `acceptance` (acceptance) | one line per criterion, strong before weak (LEAD.md 6.5); always with "no changes outside the paths" and "no tests weakened" |
| `base_commit` | set it as soon as the task builds on a specific state of the code |
| `lease_minutes` | ≈ twice the expected working time, at least 30 |
| `min_trust` (minimum trust) | A: 0, B: 1, C: 2 (critical path: one level higher) |
| `milestone` (milestone) | from `docs/plan.md` |

**Type** (first line of the description): `logic`, `integration`, `visual/feel`, `config/data`, `docs` or `asset`. It tells the contributor how the result is checked: logic, integration and config/data automatically by the test pipeline; visual/feel only together with a playtest or screenshot check by the visionary; docs and assets by you.

## Structure of the `description`

```markdown
Type: <logic | integration | visual/feel | config/data | docs | asset>

## Goal
<One sentence: what exists at the end.> So that <purpose/value for the project>.

## Context
- Project brief/pillar: <which pillar this task supports>
- Read first: <file:lines>, <file>, <interface>
- Reference example: <existing, similar implementation>
- Terms: <glossary terms you must use>

## Specifications
1. <concrete steps or rules, in a sensible order>
2. <names of files, classes, functions, signals, values>
3. <style: refer to AGENTS.md / style-guide.md>

## Not part of this task
- <what should explicitly not be done>

## Traps
- <known pitfalls>

## Submission
- Patch only within the allowed paths
- Summary: what, why, which decisions you made
- Your own test runs with results

## Costs
<only for services/donations: estimate in credits/tokens; two stages?>
```

---

## Example 1: level B, code (game)

- **titel:** Slime enemy: movement, damage, death
- **stufe:** B · **faehigkeiten:** `["code.gdscript"]` · **min_vertrauen:** 1 · **lease_minuten:** 120 · **meilenstein:** M2 Combat
- **pfade:** `["src/enemies/slime/**", "tests/enemies/test_slime.gd"]`
- **akzeptanz:**
  - `tools/run_tests.sh tests/enemies/` is green, including the contract test `test_enemy_contract.gd` for `Slime`
  - new test `tests/enemies/test_slime.gd`: given 10 HP, when it takes 4 damage, then 6 HP and signal `hit`; at ≤ 0 HP the signal `defeated` fires exactly once
  - screenshot `outputs/screens/slime_idle.png` (test pipeline) shows the slime fully, ≥ 64 px tall
  - no changes outside the allowed paths; no existing tests changed
- **beschreibung:**

```markdown
## Goal
A new enemy "slime" that hops towards the player and deals contact damage. So that level 1 gets a second enemy type (pillar "readable danger").

## Context
- Interface: `src/enemies/enemy_base.gd` (class `EnemyBase`, method `take_damage(amount: int)`, signals `hit`, `defeated`)
- Reference example: `src/enemies/bat/` (scene, script, values in `bat_values.tres`)
- Read: `docs/architecture.md#enemies`, `AGENTS.md`
- Terms: "HP" (hit points), "contact damage"

## Specifications
1. Create `src/enemies/slime/slime.tscn` and `slime.gd`; `slime.gd` extends `EnemyBase`.
2. Values in `slime_values.tres`: HP 10, jump height 120 px, jump pause 0.8 s, contact damage 1.
3. Movement: hops towards the player every 0.8 s (aim horizontally only).
4. Placeholder graphic: `assets/placeholder/circle_green.png`, 64×48 px.
5. Write `tests/enemies/test_slime.gd` following the pattern of `test_bat.gd`.

## Not part of this task
- Final graphics, animations, sounds (separate tasks)
- Placement in levels

## Traps
- `take_damage` expects an `int`, not a `float`.
- Don't submit `.import` files (the test pipeline generates them).

## Submission
Patch, summary, output of your test run.
```

## Example 2: level B, tests (tool)

- **titel:** Golden tests for `export --format csv`
- **stufe:** B · **faehigkeiten:** `["test.python"]` · **min_vertrauen:** 1 · **lease_minuten:** 90
- **pfade:** `["tests/golden/export_csv/**"]`
- **akzeptanz:**
  - `pytest tests/golden/export_csv -q` is green
  - at least 6 cases: empty, one row, umlauts (UTF-8), comma in a field, line break in a field, 10,000 rows (< 2 s)
  - every case consists of `input.json` + `expected.csv`; the test code is generic (one parametrized test function)
  - production code unchanged
- **beschreibung:** goal, reference to `src/export/csv.py` and RFC 4180, an example case, note on `\r\n` line endings.

## Example 3: level A, asset (3D)

See `asset-brief.md`, example.

## Example 4: review task

- **titel:** Review: submission #123 "inventory stacking"
- **stufe:** A · **faehigkeiten:** `["review"]` · **min_vertrauen:** 1 · **pfade:** `[]`
- **akzeptanz:**
  - report according to the schema in `review-report.md`, every acceptance criterion of the original task assessed with evidence
  - security checklist answered completely
- **beschreibung:** link or ID of the submission, original task, focus ("especially: edge cases when stacking beyond 99").
