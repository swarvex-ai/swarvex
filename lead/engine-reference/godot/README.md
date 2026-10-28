# Godot 4.7: what your model probably doesn't know

Most models were trained on Godot up to about 4.3. Versions 4.5, 4.6 and 4.7 came later and changed APIs, defaults and physics. Code written from memory then often fails to compile or behaves differently.

**Before you use an API you are not sure about, check here.**

| File | Contents |
|---|---|
| `breaking-changes.md` | what changed from 4.3 to 4.7, per version and subsystem |
| `deprecated-apis.md` | old names and patterns, and what to use instead |
| `current-best-practices.md` | new language features, physics, rendering, UI, command line |
| `modules/*.md` | quick references: animation, audio, input, navigation, networking, physics, rendering, UI |

The most frequent traps in 4.7:

- An override of a method with a typed return inherits that type and needs an explicit `return`.
- Mouse and keyboard events no longer report device `0`; check the event type.
- Jolt is the default 3D physics engine since 4.6 (existing projects keep their setting; check `project.godot`).
- `AudioStreamPlayer2D/3D.area_mask` defaults to `0`.

Version state: Godot 4.7.2, checked against the official migration guides on 2026-10-17. The authoritative source is always the [official documentation](https://docs.godotengine.org/en/stable/). If you find a mistake here, mention it in your submission summary.

## Source

Taken from [Claude Code Game Studios](https://github.com/Donchitos/Claude-Code-Game-Studios) (`docs/engine-reference/godot/`, commit `05ad7c8`), MIT license, Copyright (c) 2026 Donchitos. Changes for swarvex: this README instead of `VERSION.md`; in `current-best-practices.md` the reference to that project's own parse-check script is replaced by the project test command. License text in `lead/SOURCES.md`.
