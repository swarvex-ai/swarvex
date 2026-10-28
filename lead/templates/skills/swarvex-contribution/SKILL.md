---
name: swarvex-contribution
description: How to work on a swarvex task so the submission is accepted. Use at the start of every task in a swarvex sandbox, before reading or changing code.
---

# Working on a swarvex task

1. **Read first.** `AGENTS.md`, then the task: goal, acceptance criteria, allowed paths. Read the files the task names and one existing example of the same kind before writing anything.
2. **Stay inside the allowed paths.** Do not create notes, logs, scratch files or helper scripts in the project. Changes elsewhere are discarded, and protected files (`AGENTS.md`, `swarvex.yaml`, CI, licences, `.agents/`) make the whole submission fail.
3. **Small and focused.** Change only what the task needs. No drive-by refactors, no renaming, no reformatting of untouched code.
4. **Follow the project's conventions** from `AGENTS.md` and the surrounding code: naming, structure, comment style, language of identifiers.
5. **Tests decide.** Run the project's test command (it is in your brief) and fix failures until it passes. Never weaken, skip or delete a test to make it pass.
6. **Finish with an honest summary:** what you changed and why, how the tests went (quote the result line), what is uncertain or left open. For assets: which tools and models created them, with versions.
7. **Stuck?** Say so in the summary with the reason and what you tried. A clear "could not finish because …" helps the project more than a guess.
