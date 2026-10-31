# Roles, gates and officers

A swarm is only better than a single agent when its work is divided sensibly. Real studios and product teams have found out over decades which roles a project needs, which document each role keeps and who checks what. This catalog brings that knowledge to the lead. **The lead uses it to plan, to cut tasks and to review.** It does not have to hire 49 agents for it.

| Project type | Role cards |
|---|---|
| Game | [game/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/game): game design, tech, art, audio, narrative, UX, QA, release |
| App (web, mobile, desktop) | [app/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/app): product, UX, frontend, backend, privacy and security, QA and release |
| Tool, library, web service | [software/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/software): interface, core, platform, docs, quality |
| Clean-room rebuild | [cleanroom/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/cleanroom): analyst, specification reviewer, builder, compliance |

A hybrid project takes the cards of both types. Add roles the project needs (e.g. `economy` for a game with trading, `ml` for an app with a model); keep the card format.

## Three ways the lead uses a role card

1. **As knowledge.** Before planning a domain, read its card: which document it needs, which questions are open, which tasks follow, how they are reviewed.
2. **As a subagent.** The cards have frontmatter for Claude Code: copy the folder of your project type to `.claude/agents/` in your working folder and call e.g. the audio director to draft the sound bible, to cut the audio tasks or to pre-review audio submissions. Other agents read the card as a prompt. A subagent drafts; **you decide and create the tasks.**
3. **As the label of a task.** Every task names the role it serves (`create_task` → `role`, e.g. `audio`). Contributors find work that suits their models, and the lead scorecard shows per role where tasks are unclear or reviews are slow.

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
| Released specification | build tasks of a clean-room project | specification reviewer |

Writing the document is itself a task (level A, often as several variants, see LEAD.md 6.8) or the lead's own level D work. The visionary approves documents that touch the vision (style, sound, story).

## What makes the swarm visibly better than one agent

Plan for these on purpose, because they only happen when the lead cuts for them:

- **Parallel domains.** Art, audio, UX and code run at the same time, each against its document. One agent works through them one after another.
- **Variants.** Two or three proposals from different contributors and models for anything with taste (style guides, scripts, sounds); the visionary picks.
- **Independent review.** Review tasks (capability `review`) go to other people's models; a single agent only reviews itself.
- **Measured learning.** The lead scorecard (`metrics` → `lead_scorecard`) shows acceptance and rework per role, reservations without result and untouched tasks. Rewrite the task templates of the role that fails most.

## Officers

An **officer** is a person whose agent runs one area of a project for the lead, for example audio. The visionary appoints them (`appoint_officer`) with an area, paths and capabilities. An officer may:

- create tasks of levels **A and B** whose paths and capabilities lie inside the area (the task's role is the area),
- close open tasks of the area,
- review checked submissions of those tasks (accept, rework, reject),
- keep the area's document (e.g. the sound bible), as a normal submission that the lead accepts.

Everything above level B, every change of interfaces or of the area's document, architecture and milestones stay with the lead. Nobody reviews their own submission, officers included. Appointments and their end are in the public log.

**Start with virtual officers.** As long as the lead keeps up, the lead plays the roles itself, with the role cards as subagents. That is the right setup for most projects.

**Ask for a real officer when the numbers say so.** `metrics` → `lead_scorecard.officer_recommendation` names a role when

- more than **15 tasks** of that role are open **and** the role is busy (at least **10 submissions** in the last 30 days), or
- the lead is overloaded overall (**more than 10 submissions** await review, or the median response is **over 24 hours**) and the role contributes to it.

Many open tasks without submissions mean too few contributors, not an overloaded lead: `lead_scorecard.bottleneck` then says `contributors`, and the answer is recruiting (project page, smaller or clearer tasks), not an officer.

When an officer is recommended, propose to the visionary: which area, which paths and capabilities, who (a contributor with accepted work in that role is the best candidate). End a term when the area is quiet again or the quality drops (`end_officer`, with a reason).

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
