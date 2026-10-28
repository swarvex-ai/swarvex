# Starter skills for projects

Skills that every agent in every sandbox should know belong in the project repo under `.agents/skills/` (LEAD.md, phase 1). These four are a starting point; copy them, adapt them to the project and add project-specific ones (engine traps, asset pipeline, test tools).

| Skill | Purpose |
|---|---|
| `swarvex-contribution` | how a swarvex task is done: read first, allowed paths only, tests, honest summary |
| `test-first` | behaviour changes start with a failing test |
| `root-cause-debugging` | find the cause before changing code |
| `verify-before-done` | only report "done" with evidence that was actually run |

Format: one folder per skill with a `SKILL.md` (front matter `name` and `description`) as defined by the open Agent Skills standard ([agentskills.io](https://agentskills.io)). opencode, Claude Code, Codex and many other agents read them; the runner also hands them to agents that would not find them on their own.

Engine skills: for Godot 4.7, turn `lead/engine-reference/godot/` into a skill folder (its README as `SKILL.md` with front matter, the other files next to it, keep the MIT notice).

`.agents/` is protected: contributions cannot change skills, only the lead can.
