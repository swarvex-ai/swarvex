# Continuing abandoned software: the first milestone

> For projects with process `continuation`: an open-source project whose maintenance has stopped is continued as a fork. The original's licence and authors stay; the swarm gets it building, safe and released again. Use this list as the first milestone and its gate tasks; cut each item as a task or record "not needed" with a reason in the project brief.

## Before the first task

| ID | Item | Done when |
|---|---|---|
| W-1 | **Asked upstream** | a public issue in the original asks the maintainers; their answer or 30 days of silence is linked in the project brief. An archived repository counts as an answer. |
| W-2 | **Fork with history** | the fork starts from the last release tag with the full Git history; the original repository is named in `README.md` and in `docs/project-brief.md` |
| W-3 | **Licence inherited** | `LICENSE`, `NOTICE`, copyright headers and author lists are unchanged; the project's code licence on swarvex is the original's; files under a different licence are listed in `docs/licences.md` |
| W-4 | **Name settled** | either the old name with a short reason, or a new name; the README's first line says "Continuation of <original> (<link>)" |

## First milestone: alive again

| ID | Item | Level | Done when |
|---|---|---|---|
| W-5 | **Builds on current toolchains** | C | the pipeline builds the last release on today's compiler, runtime or engine versions; every change that needed is in `CHANGELOG.md` |
| W-6 | **Old tests pass** | B | the original's test suite runs in the pipeline; tests that cannot pass are marked with a reason, never deleted |
| W-7 | **Dependencies checked** | B | every dependency with a known vulnerability or a dead source is listed in `docs/dependencies.md` with a plan: update, replace or vendor |
| W-8 | **Security fixes** | C | the vulnerabilities of W-7 are fixed or documented as not reachable, with a test each |
| W-9 | **Issues carried over** | A | the original's open issues and pull requests are triaged into `docs/inherited-issues.md`: take (as a task), keep linked, or close with a reason |
| W-10 | **Credit** | A | `CREDITS.md` names the original authors and maintainers; carried-over patches keep their author (`git am`, `Co-authored-by`) |
| W-11 | **First release** | C | a release with fixes only, a version number that continues the original's (say how in the README), release notes and a migration note for users |
| W-12 | **Tell the users** | A | where the original's users ask for help (issue tracker, forum, package registry), a short note points to the continuation |

## Rules while continuing

- **The original's licence binds everyone.** Contributions to the fork fall under it; the contribution statement on swarvex covers this. Where the original has licence headers, a check in the project's own tests (for example `reuse lint`) flags new files without one.
- **Do not compete with a living original.** If the maintainers come back, offer the fixes upstream as pull requests and talk about merging the efforts. The visionary decides.
- **Trademarks are not inherited.** A licence covers code, not the name or logo; follow W-4.
- **Compatibility is a promise.** Users came for the old behaviour: file formats, command-line flags and APIs stay compatible within a major version; breaking changes need a decision record.
