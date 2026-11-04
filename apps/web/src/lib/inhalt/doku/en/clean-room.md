---
titel: Clean-room projects
schluessel: clean-room
gruppe: Concepts
reihenfolge: 19
beschreibung: How the swarm rebuilds closed software as open source: study, release, build and check, separated into rooms.
---

Some tools only exist closed: discontinued, expensive or tied to one vendor. A **clean-room project** rebuilds their function as open source without taking over their code or assets. The flow is the familiar one: a visionary presents the project, a lead plans, agents work on tasks, the test pipeline checks. What is new are rooms, a wall between them and a specification that is the only thing crossing it.

Tools like [Craft Anything](https://github.com/storytold/craft-anything) and [REA](https://github.com/morluto/rea) show the method for one agent: study the original, build, check against the original, refine. swarvex runs the same loop with **separated people**: whoever studied the original never builds, and whoever builds never sees the original.

## The loop across rooms

| Step | Room | Who | Writes |
|---|---|---|---|
| Study | `analysis` | contributors with their own lawfully obtained copy | drafts under `specification/drafts/`: obligations with test cases, values, formats, scenarios |
| Release | `review` | other contributors | strike everything that is expression, release to `specification/approved/` |
| Build | `build` | yet other contributors | code that makes the released test cases pass; never `specification/` |
| Check | `analysis` | verifiers from the analysis side | `specification/checks/`: the build compared with the original; differences become new drafts |

The centre of the specification is the **obligation**: one claim in our own words ("a save file starts with a header that names the format version"), with input files and expected output, and a note on how it is known: observed, derived, documented or estimated. A milestone is done when all of its required obligations are verified against the original. The format is in the lead handbook (`lead/templates/clean-room-specification.md`).

## Two depths of analysis

- **observe** (default): only what a user sees from outside: using the program, recording scenarios, measuring, and files the analyst created with it. REA's capture tools and Craft Anything in its Clean-room mode work this way.
- **inspect**: looking inside (decompiling with REA or Ghidra), only for interoperability where observation cannot settle it (file formats, protocols), only where the project's interview allowed it, and only for contributors with a higher trust level. The result is still only a description of behaviour.

Craft Anything's Deep mode and rebuilding by the same person are never used for swarvex output.


The visionary sets the depth when creating the project in the wizard; it is the upper limit for all analysis tasks of the project. A project that only observes never gets tasks that look inside.

## Analysis tools

The lead of a clean-room project gets a toolkit (`lead/roles/cleanroom/toolkit.md`), sorted by the kind of original and the depth. [REA](https://github.com/morluto/rea) is the front door: it wraps most of the tools itself and returns every result as evidence with its origin, which is exactly what obligations and check rounds need. An agent's explanation is a lead, not evidence.

| Original | observe | inspect (if allowed) |
|---|---|---|
| Windows program | use and record, [Detect It Easy](https://github.com/horsicq/Detect-It-Easy) ("what is it built with?") | [Ghidra](https://github.com/NationalSecurityAgency/ghidra) through REA, [mcp-windbg](https://github.com/svnscha/mcp-windbg) for runtime state |
| Linux or macOS program | process and UI recordings | Ghidra through REA, [pwndbg](https://github.com/pwndbg/pwndbg) |
| Android app | emulator, screen and log recordings | [JADX](https://github.com/skylot/jadx) and [Apktool](https://github.com/iBotPeaches/Apktool) through REA |
| Web or Electron app | browser scenarios, network captures | shipped bundles through REA |
| Game | play, record, own save files, measured values | Unity and Godot packages through REA, Ghidra |
| Firmware or device | visible behaviour, protocol on the wire | [unblob](https://github.com/onekey-sec/unblob), then [Binwalk](https://github.com/ReFirmLabs/binwalk) through REA |
| File format or protocol | many files you produced, one setting varied at a time | reading and writing code only for open details |

Everything runs only on the analyst's own machine, never in the runner's sandbox and never in the repository. Not used: exploit tools, kernel debuggers, and anything that circumvents protection or unpacks packers.

## The wall

The platform enforces it per person and project:

- Whoever worked in `analysis` (including checks) is not offered tasks from `build` or `review` in the same project and cannot reserve them. Whoever built can no longer analyse.
- Area leads belong to one room; their appointment counts for the wall like a reservation.
- Builders get only the released specification: drafts, check rounds and open questions are left out of their working copy.
- Every attempt to cross the wall is recorded in the log (`clean_room_separation`). The project page shows how many people worked in which room and whether the separation held.

The platform never sees the original itself. Analysts examine their own copy on their own machine and only submit text. Their evidence (REA or Craft Anything bundles, screenshots, recordings) stays with them; the specification cites it by a local ID.

## What never happens

- Copy protection, licence checks or anti-cheat are not circumvented. If they exist, there is no project.
- Decompiled code, transcribed pseudocode, strings, identifiers, graphics, text, sound, screenshots and trademarks of the original never enter the repo.
- The new project gets its own name and its own look. Compatible means what looks the same from outside (file format, protocol, shortcuts, workflows), not how it is built inside or how it looks.

## Legal framework

Clean room is a recognised method, not a blank cheque. Whether a project is permissible depends on the original, its licence and the law of the country. In the EU, the Software Directive lets lawful users observe and study how a program works (Art. 5(3)); decompilation for interoperability is subject to narrower rules (Art. 6). Every clean-room project therefore states how the original was lawfully obtained (`original.acquisition`); a project that looks inside (`original.depth` inspect) also states its legal basis (`original.legal_basis`), which parts may be inspected (`original.inspect`) and which AI models may see tool output from inside (`original.models`: only local models, or only the providers named in `original.providers`), because whatever an agent is shown reaches its model provider. Before the first project goes public, a legal professional reviews the process and the texts.

## For the lead

The lead handbook describes the loop in "Clean-room rebuild", the role cards in `lead/roles/cleanroom/` (analyst, specification reviewer, builder, verifier, compliance) give each role its tasks, review checklist and area lead mandate, and the interview has a dedicated question tree. When creating a task, `room` is mandatory; the platform checks that analysis and review only write under `specification/` and the build never does.
