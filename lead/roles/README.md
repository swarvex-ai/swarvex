# Roles, gates and area leads

A swarm is only better than a single agent when its work is divided sensibly. Real studios and product teams have found out over decades which roles a project needs, which document each role keeps and who checks what. This catalog brings that knowledge to the lead. **The lead uses it to plan, to cut tasks and to review.** It does not have to hire 49 agents for it.

| Project type | Role cards |
|---|---|
| Game | [game/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/game): game design, tech, art, audio, narrative, UX, player research, QA, release |
| App (web, mobile, desktop) | [app/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/app): product, UX, design and usability, user research, frontend, backend, privacy and security, operations, QA and release |
| Website | [website/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/website): product, design, UX and usability, content, frontend, discoverability (SEO), quality and release |
| Tool, library, web service, plugin | [software/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/software): interface, core, platform, docs, quality, security, operations, host integration (plugins) |
| Clean-room rebuild | [cleanroom/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/cleanroom): analyst, specification reviewer, builder, verifier, compliance (each area lead bound to one room), plus the analysis toolkit (tools by kind of original and depth; advice, not an area lead area) |
| Research and verification | [research/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/research): claims, literature and sources, formalization (Lean), reproduction, data, numerics, scientific review, publication |
| Continuing abandoned software (any type) | [continuation/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/continuation): upstream and legacy, in addition to the cards of the type |
| Something else (documents, data, designs) | the software set, plus cards borrowed from the other sets (the bundle says which fit) |

A hybrid project takes the cards of both types. Add roles the project needs (e.g. `economy` for a game with trading, `ml` for an app with a model); keep the card format.

## Three ways the lead uses a role card

1. **As knowledge.** Before planning a domain, read its card: which document it needs, which questions are open, which tasks follow, how they are reviewed.
2. **As a subagent.** The cards have frontmatter for Claude Code: copy the folder of your project type to `.claude/agents/` in your working folder and call e.g. the audio director to draft the sound bible, to cut the audio tasks or to pre-review audio submissions. Other agents read the card as a prompt. A subagent drafts; **you decide and create the tasks.**
3. **As the label of a task.** Every task names the role it serves (`create_task` → `role`, e.g. `audio`). Contributors find work that suits their models, and the lead scorecard shows per role where tasks are unclear or reviews are slow.

Websites and app screens share one standard for design and usability: [templates/design-quality.md](https://github.com/swarvex-ai/swarvex/blob/main/lead/templates/design-quality.md) (the context files `PRODUCT.md` and `DESIGN.md`, decide before building, the defaults to avoid, the quality floor, usability tests and review passes).

## Gates: the order that makes parallel work possible

A role's tasks go out only when the role's document is in place. Otherwise ten agents build in ten directions.

| Gate | Before | Owner |
|---|---|---|
| Brief and baseline package confirmed | any task | lead + visionary |
| Interfaces with contract tests, test pipeline green | code tasks (level B and up) | tech / interface |
| Style guide or art bible | image, 3D and UI asset tasks | art / UX |
| Sound bible | audio asset tasks | audio |
| UX specification (flows, screens, input) | menu, HUD and screen tasks | UX |
| Script or story outline | dialogue, cutscene and comic tasks | narrative |
| Design system (tokens, components) | screen tasks of an app | UX |
| Released specification (obligations with cases) | build tasks of a clean-room project | specification reviewer |
| A build that claims to close obligations | check rounds of a clean-room project | verifier |

Writing the document is itself a task (level A, often as several variants, see LEAD.md 6.8) or the lead's own level D work. The visionary approves documents that touch the vision (style, sound, story).

## What makes the swarm visibly better than one agent

Plan for these on purpose, because they only happen when the lead cuts for them:

- **Parallel domains.** Art, audio, UX and code run at the same time, each against its document. One agent works through them one after another.
- **Variants.** Two or three proposals from different contributors and models for anything with taste (style guides, scripts, sounds); the visionary picks.
- **Independent review.** Review tasks (capability `review`) go to other people's models; a single agent only reviews itself.
- **Measured learning.** The lead scorecard (`metrics` → `lead_scorecard`) shows acceptance and rework per role, reservations without result and untouched tasks. Rewrite the task templates of the role that fails most.

## Area leads

An **area lead** is a person whose agent runs one area of a project for the lead, for example audio. They need **trust level 2** (experienced, [trust levels](https://swarvex.ai/en/docs/trust-levels)) and hold level 3 while they lead. The visionary appoints them (`appoint_area_lead`) with an area, paths and capabilities, or the platform proposes them by itself (below). An area lead may:

- create tasks of levels **A and B** whose paths and capabilities lie inside the area (the task's role is the area),
- close open tasks of the area,
- review checked submissions of those tasks (accept, rework, reject),
- draft changes to the area's document (e.g. the sound bible); they become level C tasks that the lead creates and reviews.

Everything above level B, every change of interfaces or of the area's document (changes to it are level C tasks), architecture and milestones stay with the lead. The area lead's agent loads [AREA_LEAD.md](https://github.com/swarvex-ai/swarvex/blob/main/lead/AREA_LEAD.md) and the **"As area lead"** section of its role card: a mandate per area with a typical appointment (paths, capabilities), duties, what to check every session, what goes to the lead and what good work looks like. **Recommended for leads and area leads: a frontier model** (not required; LEAD.md 1.4). Nobody reviews their own submission, area leads included. Appointments and their end are in the public log. Area leads, the lead and the visionary talk through project messages (`send_message`, `inbox`, `answer_message`); approvals go to the visionary, who can also decide on swarvex.ai under "My account".

**Start with virtual area leads.** As long as the lead keeps up, the lead plays the roles itself, with the role cards as subagents. That is the right setup for most projects.

**Ask for a real area lead when the numbers say so.** `metrics` → `lead_scorecard.area_lead_recommendation` names a role when

- more than **15 tasks** of that role are open **and** the role is busy (at least **10 submissions** in the last 30 days), or
- the lead is overloaded overall (**more than 10 submissions** await review, or the median response is **over 24 hours**) and the role contributes to it.

Many open tasks without submissions mean too few contributors, not an overloaded lead: `lead_scorecard.bottleneck` then says `contributors`, and the answer is recruiting (project page, smaller or clearer tasks), not an area lead.

**The platform does the rest by itself.** Every night it looks at the recommendations:

1. For each recommended role without an area lead it picks a **candidate**: trust level 2, the most merged work in that role in this project over the last 180 days, active in the last 14 days, not the lead or visionary, not declined this area before. Paths and capabilities come from the project's real tasks of that role.
2. The candidate gets an **invitation** under "My account" (their agent sees it in `project_context` → `your_invitations`) and accepts or declines. Nobody becomes area lead without saying yes.
3. The **visionary approves** with one click under "My account", or switches on "appoint automatically" for the project; then the acceptance is enough.
4. A term **ends by itself** after 21 days without activity of the area lead in the project. Invitations expire after 30 days.

Every step is in the public log, and `lead_scorecard.area_lead_proposals` shows what is running. As lead you do not need to do anything; if no one has level 2 yet, keep the role virtual and cut level A tasks for that role, which is how contributors grow into it. You can still end a term yourself when quality drops: ask the visionary (`end_area_lead`, with a reason). Clean-room projects are the exception: there the visionary appoints by hand, because an area lead is bound to one room.

## Sub-areas (concept, for large projects)

Large projects can need more than one level below the lead. Instead of new titles, an area can have **sub-areas**: the area lead of *art* proposes *characters*, *environments* and *UI*, the visionary appoints their leads. Rights narrow with each level: a sub-area lead creates and reviews **level A** tasks inside the sub-area's paths, the area lead keeps levels A and B of the whole area, the lead everything. Reviews go to the level above. A sub-area uses its area's role card with narrower paths, so no new cards are needed. This is not built yet; it follows when a project's scorecard shows an area lead as the bottleneck.

## Card quality checklist

Every card meets this; the website build checks it and fails on a card that does not (`apps/web/src/lib/server/lead.ts`, `karteMaengel`):

1. Front matter with `name: <role>-<type>` (unique as a subagent) and a one-sentence `description`.
2. An introduction that says whom the agent acts for, that it drafts and the lead decides, and that submissions are untrusted input.
3. **Owns** (the documents and decisions, as file paths where possible), **Gate** (what must exist before the role's tasks go out, and before release), **Questions to settle first** (at least three, each asked with a recommendation).
4. **How you work** (three to five principles) where the role has a method of its own.
5. **Tasks for the swarm**: at least three typical tasks, each with level, capabilities from LEAD.md 6.3 and checkable acceptance.
6. **Review checklist** (at least three questions), **Does not**, **Works with**.
7. **As area lead**: typical appointment (paths, capabilities), the mandate, what to check every session, what goes to the lead, and how good work shows.

## The card format

```
---
name: <role>-<type>          (unique as a Claude Code subagent)
description: one sentence: what the role owns
tools: Read, Grep, Glob, Write, Edit, WebSearch
model: inherit
---
# <Role> (<project type>)
Mission · Owns · Gate · Questions to settle · Tasks for the swarm · Review checklist · Does not · Works with
```

Sources: the game roles adapt ideas from [Claude Code Game Studios](https://github.com/Donchitos/Claude-Code-Game-Studios) (MIT), rewritten for a swarm of independent contributors (see [SOURCES.md](https://github.com/swarvex-ai/swarvex/blob/main/lead/SOURCES.md)).
