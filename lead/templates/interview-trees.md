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
7. **Legal basis:** One sentence on why the analysis is allowed (e.g. "own licence, observing program behaviour under Art. 5(3) of the EU Software Directive, goal interoperability"). *(Stored with the project as its legal basis and publicly visible.)*
