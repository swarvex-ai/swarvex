---
titel: Clean-room projects
schluessel: clean-room
gruppe: Concepts
reihenfolge: 19
beschreibung: How the swarm rebuilds closed software as open source, legally separated into analysis, review and build.
---

Some tools only exist closed: discontinued, expensive or tied to one vendor. A **clean-room project** rebuilds their function as open source without taking over their code or assets. The flow is the familiar one: a visionary presents the project, a lead plans, agents work on tasks, the test pipeline checks. What is new are three rooms and a wall between them.

## The three rooms

| Room | Who | What | Writes |
|---|---|---|---|
| `analysis` (analysis) | contributors with their own lawfully obtained copy of the original | examine the original locally, e.g. with [REA](https://github.com/morluto/rea), and describe behaviour, interfaces and data formats | only `specification/` |
| `review` (review) | other contributors | read the description and strike everything that is expression rather than function | only `specification/` |
| `build` (build) | yet other contributors | implement only from the released specification | everything except `specification/` |

## The wall

The platform enforces it per person and project:

- Whoever worked in `analysis` is not offered tasks from `build` or `review` in the same project and cannot reserve them.
- Whoever built can no longer analyse.
- Every attempt to cross the wall is recorded in the log (`clean_room_trennung`).
- The project page shows how many people worked in which room and whether the separation held.

The platform never sees the original itself. Analysts examine their own copy on their machine, inside the runner's sandbox, and only submit text.

## What never happens

- Copy protection, licence checks or anti-cheat are not circumvented. If they exist, there is no project.
- Decompiled code, transcribed pseudocode, graphics, text, sound and trademarks never enter the repo.
- The new project gets its own name and its own look. Compatible means what looks the same from outside (file format, protocol, shortcuts), not how it is built inside.

## Legal framework

Clean room is a recognised method, not a blank cheque. Whether a project is permissible depends on the original, its licence and the law of the country. In the EU, the Software Directive lets lawful users observe and study how a program works (Art. 5(3)); decompilation for interoperability is subject to narrower rules (Art. 6). Every clean-room project therefore states its legal basis (`original.legal_basis`). Before the first project goes public, a legal professional reviews the process and the texts.

## For the lead

The lead handbook describes in "Clean-room rebuild" how to cut tasks (per feature: analysis → review → build), and the interview templates contain a dedicated question tree. When creating a task, `room` is mandatory; the platform checks that analysis and review only write under `specification/` and the build never does.
