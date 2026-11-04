# Starting trees for the project interview

> For phase 0 (LEAD.md, "The project interview"). Every node is a decision. **"after"** names its prerequisites: only once they are settled does the node belong to the frontier.
> **R** = required branch (must be decided before the foundation), **L** = later branch (may stay open with a provisional default).
> The trees are a minimum. Add anything the specific idea raises.
> You research facts (existing repository, versions, licenses) yourself instead of asking.

## Common trunk (all project types)

| ID | Decision | R/L | after | Hint for the recommendation |
|---|---|---|---|---|
| G1 | **Type:** game, tool/CLI/library, app, hybrid? | R | – | determines the specialist tree below |
| G2 | **Core in one sentence:** what is it, for whom, which experience or problem? | R | – | suggest a phrasing from the idea |
| G3 | **Audience:** who exactly, prior knowledge, device, session length? | R | G2 | keep it narrow; "everyone" is not an audience |
| G4 | **Pillars:** 3–5 properties that must always hold | R | G2, G3 | each pillar with "how would you notice?" |
| G5 | **Non-goals:** what is it explicitly not? | R | G4 | at least 3; protects against scope creep |
| G6 | **Success criteria for version 1:** measurable or clearly observable | R | G4 | "done when …" |
| G7 | **Platforms** | R | G1, G3 | few at first; every platform adds testing effort |
| G8 | **Stack / engine / language** | R | G1, G7 | research which languages contributors' models handle well |
| G9 | **License for code / assets** | R | – | any OSI license / free asset license; recommend by type: web service or server AGPL-3.0, game/app/tool GPL-3.0-or-later, library MIT or Apache-2.0; assets CC BY-SA 4.0 |
| G10 | **Project language** (tasks, docs) and languages of the product | R | G3 | code identifiers usually English |
| G11 | **Quality level:** prototype / indie / polished | R | G6 | determines testing effort and asset ambition |
| G12 | **References and style:** "like X, but Y", mood, palette | R | G4 | 2–3 references with "adopt / don't adopt" |
| G13 | **Existing work:** what exists already (repository, code, assets, brand)? | R | – | look it up yourself, only ask about gaps |
| G14 | **Untouchables:** what must never change? | R | G13 | e.g. core controls, brand elements |
| G15 | **Resources:** expected swarm size, asset needs, budget for donation briefs | R | G11 | set a budget per milestone that needs no approval |
| G16 | **Accessibility** | L | G3, G7 | suggest a minimum standard |
| G17 | **Privacy / user data** | R if user data is involved, else L | G1 | as little data as possible |
| G18 | **Release target:** where to publish (itch.io, store, package registry, web), roughly when | L | G7, G9 | the release decision stays with the visionary |
| G19 | **Project revenue** (if ever): e.g. "free" or "funds lead and infrastructure" | L | G9 | declare in writing up front |
| G20 | **The visionary's day-to-day role:** how often available, what do they want to sign off themselves? | R | – | determines what is delegated to you |

## Specialist tree: game

| ID | Decision | R/L | after | Hint |
|---|---|---|---|---|
| S1 | **Genre and perspective** (2D/3D, camera) | R | G2, G12 | |
| S2 | **Core gameplay loop:** what does the player do every 10 s, 1 min, 10 min? | R | S1 | the most important question of this tree |
| S3 | **Controls and input devices** (keyboard/mouse, controller, touch) | R | G7, S2 | remappable? |
| S4 | **Session length and progression:** runs, saving, meta progression | R | G3, S2 | |
| S5 | **Feel:** response times, hit feedback, readability of danger | R | S2, G4 | phrase as measurable criteria |
| S6 | **Difficulty and fairness** | L | S2, G3 | difficulty levels? |
| S7 | **Amount of content in version 1:** levels, enemies, weapons, items | R | G6, S2 | start small: one complete level |
| S8 | **Content as data:** which content types should contributors be able to add? | R | S7, G8 | determines the extension points in the foundation |
| S9 | **Asset style and pipeline:** 2D/3D, resolution, polygon and texture limits, animations | R | S1, G12 | |
| S10 | **Audio:** music, effects, voice | L | G12 | |
| S11 | **Story/world** (if relevant) | L | S1 | |
| S12 | **Multiplayer** | R if yes, else L | S2, G7 | changes the architecture a lot |
| S13 | **Balancing method:** simulations, metrics | L | S2, S7 | |
| S14 | **Playtests:** who, how often, which questions | L | G6 | |

## Specialist tree: tool / CLI / library

| ID | Decision | R/L | after | Hint |
|---|---|---|---|---|
| T1 | **Form of use:** CLI, library/API, service, plugin for X | R | G2 | |
| T2 | **Core workflows:** the 3–5 most common user tasks, step by step | R | T1, G3 | |
| T3 | **Interface:** commands/functions, options, defaults | R | T2 | document before building |
| T4 | **Input and output formats** (files, streams, formats, encodings) | R | T2 | collect examples (golden tests) |
| T5 | **Error behavior:** messages, exit codes, resumption | R | T3 | |
| T6 | **Configuration:** files, environment variables, profiles | L | T3 | |
| T7 | **Compatibility:** operating systems, language/runtime versions, backward compatibility | R | G7, G8 | |
| T8 | **Performance and limits:** data volumes, time budgets, memory | R | T2 | make it measurable |
| T9 | **Dependencies:** allowed, forbidden, license compatibility | R | G8, G9 | research yourself |
| T10 | **Security:** handling untrusted input, permissions, secrets | R | T4 | |
| T11 | **Distribution:** package registry, binaries, containers | L | G18 | |
| T12 | **Documentation:** `--help`, README, examples, reference | R | T3 | part of every task |
| T13 | **Extensibility:** plugins, formats, third-party commands | L | T3 | determines extension points |

## Specialist tree: app (web / mobile / desktop)

| ID | Decision | R/L | after | Hint |
|---|---|---|---|---|
| A1 | **App type:** web, mobile (iOS/Android), desktop, cross-platform | R | G7 | |
| A2 | **Main user flows:** the 3–5 paths that carry the app | R | G2, G3 | sketch them as flows |
| A3 | **Screens** per flow | R | A2 | `design.ui` tasks follow from this |
| A4 | **Data model:** which things, relations, who owns which data | R | A2 | |
| A5 | **Accounts and sign-in** | R if user data, else L | A4, G17 | |
| A6 | **Backend:** none / own / existing service; API contract | R | A4, G8 | mock server for parallel work |
| A7 | **Offline capability and sync** | L | A1, A4 | |
| A8 | **Design system:** tokens (colors, spacing, type), components | R | G12 | comes before the screens |
| A9 | **Responsiveness / screen sizes** | R | A1 | |
| A10 | **Accessibility** (contrast, keyboard, screen reader) | R | A3 | required for apps |
| A11 | **Notifications, background activity** | L | A1, A2 | |
| A12 | **Analytics/telemetry** | L | G17 | only with consent, minimal |
| A13 | **Store requirements** (guidelines, accounts, fees) | L | A1, G18 | |

## Specialist tree: website

| ID | Decision | R/L | after | Hint |
|---|---|---|---|---|
| W1 | **Kind of site:** landing page, documentation, content site (blog, magazine), portfolio, community | R | G1 | determines the page types |
| W2 | **Audience and primary job:** who comes, from where, on which device, and the one thing they must be able to do | R | G2 | goes into `docs/PRODUCT.md` (templates/design-quality.md) |
| W3 | **Voice:** three adjectives it is and three it never is, with example sentences | R | W2 | the content role's style guide builds on it |
| W4 | **Visual direction:** existing brand rules, references with "take / leave", light and dark | R | W2, G12 | cut as 2–3 variants, the visionary picks; goes into `docs/DESIGN.md` |
| W5 | **Pages of the first release** and their jobs | R | W2 | nothing without a job |
| W6 | **Content source:** who writes, which languages, translation and review | R | W3 | |
| W7 | **Framework and hosting:** static or server-rendered, where it runs, headers we need | R | G8 | |
| W8 | **Performance budget and support matrix:** page weight, LCP/INP/CLS, browsers and devices | R | W7 | measured in the pipeline |
| W9 | **Accessibility:** WCAG 2.2 AA and the assistive technologies we test | R | W5 | the floor is AA, not a choice |
| W10 | **Findability:** what the audience searches for, when the site may be indexed | L | W2 | indexing is the visionary's decision |
| W11 | **Measurement:** none, or counted without personal data and without a consent banner | L | G17 | |
| W12 | **Forms, search, accounts:** what sends or stores data | L | W5, G17 | each needs its own journey and privacy note |

## Specialist tree: research (proofs, reproductions, scientific software)

| ID | Decision | R/L | after | Hint |
|---|---|---|---|---|
| F1 | **Kind of research:** formal proof, reproduction of a paper, scientific software, test and verification of an open-source component | R | G1 | determines the role cards to use first |
| F2 | **Source:** the paper (DOI or arXiv), the theorem and where it is stated, the software and its repository | R | F1 | pin a version or revision |
| F3 | **Claims:** the results that count as success, each with acceptance and, for numbers, a tolerance decided now | R | F2 | `templates/research-claims.md`; a refuted claim is a result too |
| F4 | **Toolchain:** proof assistant and library revision (Lean and Mathlib), languages and pinned versions | R | F1 | the environment must pass `swarvex env` offline |
| F5 | **Compute:** time and memory for one run; GPU needed? | R | F3 | a task must fit one runner session; split long runs into chained tasks |
| F6 | **Data:** datasets, sizes, licences, personal data? | R | F2 | registered by the data steward before use |
| F7 | **Original authors and code:** do they exist, may we use their code and data, do we contact them? | R | F2 | look before you build: an existing reproduction changes the plan |
| F8 | **Ethics and dual use:** could the result cause harm (weapons, surveillance, personal data)? | R | F1 | if yes: no project, or a narrower scope agreed with the visionary |
| F9 | **Publication:** where and how results are released, archived (DOI) and credited | L | F3 | CITATION.cff from the start |
| F10 | **Automatic acceptance:** which claims the machine decides alone (proof checker, tolerance check) and which need a human | L | F3 | the more the machine decides, the faster the swarm |

## Specialist tree: other (documents, data, designs and more)

| ID | Decision | R/L | after | Hint |
|---|---|---|---|---|
| O1 | **What is made:** documents, a dataset, hardware designs, translations, a collection, something else | R | G1 | determines which role cards to borrow |
| O2 | **Formats and tools:** open formats of the work, the tools and versions that read and write them | R | O1 | contributors need them offline |
| O3 | **What a machine can check:** spelling, links, schemas, rendering, export | R | O2 | goes into `swarvex.yaml` |
| O4 | **What a person must judge:** style, accuracy, completeness; who judges, against which rules | R | O3 | write the rules with one good example |
| O5 | **Licences per kind of work** and third-party material | R | G17 | |
| O6 | **Release form:** how people get the result (download, website, print) | L | O1 | |

## Closing

The required frontier is empty when all **R** nodes of the trunk and of the matching specialist tree are in the project brief as *decided*, *recommendation accepted* or *delegated*. Then follows the visionary's explicit confirmation (LEAD.md, phase 0, step 6).

## Additional tree: clean-room rebuild

Only when an existing program is to be rebuilt. Each question with a recommendation, as usual.

1. **Original:** Which program, which version, by whom? *(Recommendation: pin a concrete version so the specification has a reference.)*
2. **Acquisition:** Do you hold your own lawful licence? Everyone who analyses needs their own, too. *(Without it: no project.)*
3. **Protection measures:** Is there copy protection, a licence check, forced online use or anti-cheat? *(If so: analysis only without circumvention; otherwise no project.)*
4. **Goal:** Compatibility (files, protocols), replacing a discontinued product, preservation, learning? *(Recommendation: compatibility with open file formats as a measurable goal.)*
5. **Distinction:** New name, own look, own assets? *(Mandatory. Trademarks, logos, graphics, text and sound of the original are never taken over.)*
6. **Scope:** Which features first? *(Recommendation: the smallest chain that opens and saves a real file of the original.)*
7. **Depth:** May any part be looked at from inside (decompiled, inspected with REA or Ghidra), and why is observation not enough there? *(Recommendation: `observe` only, plus `inspect` for named file formats or protocols where interoperability needs it.)*
8. **Comparison:** How do we check a build against the original: same input files, recorded scenarios, measured values? *(Recommendation: obligations with cases per `templates/clean-room-specification.md`, check rounds after each build, a milestone is done when its required obligations are verified.)*
9. **Legal basis:** One sentence on why the analysis is allowed (e.g. "own licence, observing program behaviour under Art. 5(3) of the EU Software Directive, goal interoperability"). *(Stored with the project as its legal basis and publicly visible.)*

## Additional tree: continuing abandoned software

Only when an existing open-source project whose maintenance has stopped is to be continued as a fork. Each question with a recommendation, as usual.

1. **Original:** Which repository, which last release, which last commit? *(Recommendation: fork from the last release tag with the full history, so every line keeps its author.)*
2. **Is it really abandoned?** Archived repository, no answer from the maintainers, or do they agree or even hand over? *(Recommendation: ask first, publicly in an issue, and wait 30 days. A handover beats a fork; a fork beats a dead project.)*
3. **Licence:** What is the original's licence, and is it the same for every file? *(Mandatory: the fork keeps it. Copyright notices, `LICENSE`, `NOTICE` and the authors stay. New files may use a compatible licence only where the original's licence allows it.)*
4. **Name:** Keep the name or choose a new one? *(Recommendation: a new name if the old one is a trademark, if the maintainers ask for it or if the original might come back; say "continuation of X" in the README.)*
5. **Users:** Who still uses it, on which versions, and where do they ask for help? *(Recommendation: tell them where the continuation lives; a migration note for the first release.)*
6. **First goal:** Builds and passes its old tests again? Security fixes? A new platform? *(Recommendation: first a green build on current toolchains and a release with only fixes; features after that.)*
7. **Open issues and pull requests:** Which of the original's are carried over? *(Recommendation: link to them, re-cut the ones you take as swarvex tasks, credit the original authors of carried-over patches.)*
8. **Upstream:** Do fixes go back if the original comes alive again? *(Recommendation: yes, offered as pull requests; the continuation does not compete with a living original.)*
