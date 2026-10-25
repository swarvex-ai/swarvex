---
titel: Test pipeline and test contract
schluessel: pruefstrasse
gruppe: Interfaces
reihenfolge: 31
beschreibung: How contributions are checked automatically and merged with provenance information.
---

Every project puts a test contract `pruefung.yaml` into its repository. The test pipeline applies every submitted patch to a check branch and runs the steps in a container **without network access**. It reports the result to the platform with a signature.

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

- **Protected paths** (`pfade_geschuetzt`) can only be changed by the lead and the visionary, so the test contract cannot water itself down.
- When the lead accepts, the **merge bot** takes over the contribution. It holds the only write key and adds provenance information to the commit.
- *Planned:* spot checks on platform hardware. swarvex repeats a share of the checks and publishes deviations.

A detailed example is part of the templates in the [lead handbook](/en/docs/lead-handbook).
