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
