# Sources and third-party licenses

## Project interview (LEAD.md, phase 0)

The project interview procedure is based on the **"grilling"** skill (invoked via `grill-me` or `grill-with-docs`) from [mattpocock/skills](https://github.com/mattpocock/skills), `skills/productivity/grilling/SKILL.md`. Taken over were the ideas of:

- a decision tree
- questions in rounds along the "front"
- numbered questions with a recommendation, where "yes" accepts the recommendation
- the agent looks up facts, the human makes decisions
- finishing when the front is empty, plus confirmation

New for swarvex are:

- the starting trees per project type with required and later branches
- the three states in the project brief
- the limit per round
- the small interviews along the way

```
MIT License

Copyright (c) 2026 Matt Pocock

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Planning, review and contributor skills

From [obra/superpowers](https://github.com/obra/superpowers) by Jesse Vincent (commit `2ce8aa1`, MIT): ideas from "writing-plans" (plan for someone who has not seen the code, interfaces per task, review focus, task right-sizing, plan self-review), "subagent-driven-development" and "requesting-code-review" (fresh context per task, spec compliance before quality, do not trust the implementer's report, rulings recorded instead of stalls, a final whole-milestone review), "dispatching-parallel-agents" (independence before parallelism), "writing-skills" (test a skill with and without it) and "receiving-code-review" (the starter skill `receiving-review`). Rewritten for a swarm of strangers (LEAD.md 6.12, 7, 8.1, 7.1; `templates/skills/`); no text taken over.

## Clean-room method

- Clean-room rebuild (`roles/cleanroom/`, `templates/clean-room-specification.md`, LEAD.md "Clean-room rebuild"): ideas from [Craft Anything](https://github.com/storytold/craft-anything) by the ArtCraft team (commit `b09b99a`, MIT OR Apache-2.0): the loop study → build → check → refine, the split between working only from outside (Clean room) and looking inside (Deep), the mechanics sheet with values and their source, and evidence-backed questions; and from [REA](https://github.com/morluto/rea) by morluto (commit `e5e0dc5`, MIT): evidence with confidence (observed, derived, inferred), reconstruction obligation ledgers with cases and verifiers, coverage of a boundary, scenario capture and comparison, and the advice to keep REA unreachable from the implementation environment. Rewritten in our own words for a swarm separated into rooms; no code taken over. The analysis toolkit (`roles/cleanroom/toolkit.md`) also draws on [ReVa](https://github.com/cyberkaida/reverse-engineering-assistant) by cyberkaida (Apache-2.0) for asking for small answers with context instead of whole analyses, on [REA](https://github.com/morluto/rea)'s note that tool results reach the agent's model provider, and on the published limits of [LLM4Decompile](https://github.com/albertan017/LLM4Decompile) (MIT) for treating model decompilation as a hypothesis; the Android steps follow REA's [Android analysis](https://github.com/morluto/rea/blob/main/docs/android-analysis.md) and [guided prompts](https://github.com/morluto/rea/blob/main/docs/mcp-prompts.md) documentation (map first, one feature, observation / inference / unknown, "not found" is not "does not exist", tests that can fail); ideas only, no code.

## Research and user research role cards

Ideas, rewritten in our own words for a swarm of independent contributors and for any agent:

- `roles/research/reproduction.md` and `literature.md`: from [ai-paper-reproduction-skill](https://github.com/lllllllama/ai-paper-reproduction-skill) by lllllllama (MIT): README-first intake with the smallest trusted target before training, patches kept apart from changes that alter a result, a comparability report, failed attempts kept, bounded exploration whose results stay candidates, and resolving reproduction-critical details (splits, preprocessing, evaluation protocol, checkpoints) with their location and explicit README-paper conflicts.
- `roles/research/claims.md`, `numerics.md`, `data.md`: from [claude-scientific-skills](https://github.com/K-Dense-AI/claude-scientific-skills) by K-Dense (MIT): hypotheses labelled as candidates with a refuting test and competing explanations, power and multiple comparisons, DataLad for provenance; `literature.md`: verified citations against their records.
- `roles/app/user-research.md`, `roles/game/user-research.md`: from the `synthesize-research` skill in [knowledge-work-plugins](https://github.com/anthropics/knowledge-work-plugins) by Anthropic (Apache-2.0): observations before themes, behaviour apart from stated preference, frequency times impact, confidence levels and contradictions as signals.

## Engine reference and game rules

From [Claude Code Game Studios](https://github.com/Donchitos/Claude-Code-Game-Studios) (commit `05ad7c8`):

- `lead/engine-reference/godot/`: taken over almost unchanged from `docs/engine-reference/godot/` (changes listed in its README)
- ideas, rewritten in our own words: content as data, delta time and no singletons for game state (`templates/AGENTS.md`); a regression test must be seen failing (LEAD.md 6.5); the task type field (`templates/task.md`); the playtest report and style guide templates; the recurring checks in LEAD.md 7.1
- role cards (`lead/roles/game/`, commit `05ad7c8`): the idea of a studio hierarchy with one owner and one document per domain (art bible, sound bible, UX spec), review gates before asset production, delegation and escalation per role and the audio naming convention; rewritten for a swarm of independent contributors (one card per domain instead of 49 subagents, tasks for strangers instead of in-session work)

```
MIT License

Copyright (c) 2026 Donchitos

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Design and usability standard (templates/design-quality.md, roles/website/, roles/app/ux.md)

Written for swarvex; it takes up ideas from two Apache-2.0-licensed skills, without copying their text:

- **Anthropic, `frontend-design`** ([anthropics/skills](https://github.com/anthropics/skills/tree/main/skills/frontend-design)): ground the look in the subject, plan tokens and a wireframe before building and review the plan against the brief, a list of generic defaults to avoid, the quality floor (responsive, focus, reduced motion, contrast) and writing as part of the design.
- **Paul Bakaus, Impeccable** ([pbakaus/impeccable](https://github.com/pbakaus/impeccable)), which grew out of the former: the context files `PRODUCT.md` and `DESIGN.md`, named review passes (critique, audit, harden, polish, distill, clarify) and further anti-patterns (nested cards, grey text on colour, icon tiles above every heading, the "tasteful default").

Both are licensed under the Apache License, Version 2.0 (https://www.apache.org/licenses/LICENSE-2.0). The usability heuristics are Jakob Nielsen's (1994); the quality floor follows WCAG 2.2 (W3C).
