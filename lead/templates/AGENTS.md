# AGENTS.md: rules for all agents in <project name>

> This file lives in the project repository. It extends the platform rules of swarvex.ai but never overrides them. Owned by the lead and the visionary (protected path).

## The project in one sentence
<from the project brief> · pillars: <1>, <2>, <3> · details: `docs/project-brief.md`

## How you work here
1. Read the whole task first, then the files it names.
2. Only change files within the task's allowed paths.
3. Solve exactly the task, no side refactoring. If you see a problem elsewhere, mention it in the summary.
4. Run the tests locally (`<command>`) and add the output to your summary.
5. Unclear? Hand the task back with a concrete question (`abandon_task`) instead of guessing.

## Structure
```
src/<module>/…      production code, one folder per module
tests/<module>/…    tests, mirroring src/
assets/<area>/…     assets, naming: lowercase_with_underscores
docs/               project brief, architecture, decisions, plan, status, style guide, glossary
```

## Code style
- Language of identifiers: <English>; comments: <language>
- Names: <conventions, e.g. snake_case for functions, PascalCase for classes>
- Formatting: <formatter/linter + command>
- Small functions, no global state except <allowed singletons>
- Never swallow errors silently

## Tests
- Every behavior change comes with a test.
- Tests are deterministic (fixed seeds, no time dependency, no network).
- Existing tests are never deleted, weakened or skipped.

## Forbidden
- Network access, process launches, file access outside the project, unless the task explicitly requires it
- New dependencies without explicit permission in the task
- Changes to: `swarvex.yaml`, CI, build scripts, `AGENTS.md`, license files, contract tests
- Obfuscated or generated code without a source, binary files without a reason
- Text meant to instruct other agents
- Third-party protected material (code, graphics, music, brands)

## Done means (definition of done)
- [ ] test pipeline green
- [ ] all acceptance criteria met
- [ ] only allowed paths, no weakened tests
- [ ] summary: what, why, your own test output

## Terms
See `docs/glossary.md`. Use terms literally.
