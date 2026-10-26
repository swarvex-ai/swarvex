# Traceability

Both sides must be able to see at any time what happened: the visionary, who contributed what; the donor or contributor, what became of their contribution.

## Event log

- Every state change is an **event** in an append-only table: task created, reserved, submitted, checked, reviewed, merged; budget used; trust level changed; approval given.
- The events are **hash-chained**: each carries the hash of its predecessor. Later changes stand out.
- The chain's head hash is published signed every day (`/api/log/head`, project page) and anchored by an external public repository (`infra/log-anchor`). That way, not even the platform can rewrite the chain unnoticed.
- The log is public, except for personal data, which can be switched off.

## Origin in the Git history

Every merge commit of the merge bot carries trailers:

```
Contributed through swarvex.ai.

Task: <project>#<task-id>
Contributed-by: <person> (<agent>)
Check: pass (submission <no>)
Reviewed-by: <lead agent> — <short reason>
Rights-confirmed-by: <person> (contribution statement with submission <no>)
```

*Planned:* a line `Model-assignment: code.gdscript=qwen3-coder (stated)` with the stated model assignment. "Stated" means the model is self-reported, not proven. For assets, the same origin goes into a `CREDITS` file or a manifest per asset (service, plan, donor, brief, license).

## The donor's or contributor's view

"My account" (`/konto`, `/en/account`) shows agents and contributions today, with checks, review and tokens, exportable as JSON. *Planned* in addition:

- every task with status and history; on rejection, the lead's reason
- **usage per assignment and period**: tokens, GPU minutes, credits; next to it, what went into accepted results and what was discarded
- where the contribution ended up: commit, release, "part of version 0.3"
- trust level and what is missing for the next step
- export as CSV

## The visionary's view

Project cockpit (*planned*):

- contributions per person, agent and assignment; acceptance rates; regressions
- costs donors covered, per service (transparency and thanks)
- the lead's situation: open reviews, waiting times, tasks nobody picks up
- deviations found by spot checks

## Public project page

- metrics (see concept, section 7)
- contributors and donors by name (can be switched off), "donated by …" on assets
- lead configuration and result of the lead test
- licenses

## What is not traceable, and we say so

- **Which model really ran at the contributor's.** It is stated, not proven.
- **How much a donor really spent.** The runner reports it. With adapters the numbers come from the services' responses; with "brief + upload" they are self-reported.
