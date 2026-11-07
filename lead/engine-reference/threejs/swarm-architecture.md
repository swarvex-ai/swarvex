# A three.js project the swarm can build

One agent can keep a small scene in its head. A swarm only gets faster than one agent when the work splits into many
pieces that can be built, checked and merged independently. The template (`lead/templates/threejs`) is laid out for that.

## The four layers

| Layer | Folder | Rule | Why it helps the swarm |
|---|---|---|---|
| Simulation | `src/core/` | pure TypeScript: no three.js, no DOM, no `Math.random`, fixed 1/60 s steps | tests run in Node in milliseconds; every machine computes the same world; logic tasks never touch rendering |
| Content | `src/content/` | data only: kinds, levels, cards, items, texts | adding content is the most parallel work there is: one entry, one task, one validation test |
| Assets | `src/assets/procedural/` | functions that return geometry and TSL materials | an asset is a reviewable diff, not a binary; no generator credits; budgets per asset |
| View | `src/render/` | reads the world, builds and updates the scene; never writes the world | rendering can change without breaking logic; scene tests catch visual regressions |

`src/main.ts` only wires them together and has a test mode (`?test`) where the scene tests step the world on request.

## Cutting tasks

- One task, one folder where possible. Cross-folder changes (a new mechanic that needs core, content and view) are a
  level C or D task for the lead or an area lead, or three tasks with `depends_on`.
- Content and procedural assets are level A or B: an entry plus its test, or a builder function plus its scene.
- Every visual change names its scene in `tests/scene/scenes.json` and ships the new baseline image.
- Areas for area leads map to folders: e.g. `world` → `src/core/world/**`, `content` → `src/content/**`,
  `assets` → `src/assets/**`, `ui` → `src/ui/**`. Their paths go straight into `open_lead_position` and `take_area`.

## Contracts that keep parallel work compatible

- Types between layers live in `src/core` (e.g. `World`, `Body`); the view and content import them, never the other way.
- Content is validated by tests (complete, unique, in range), so a wrong entry fails the check, not the game.
- Budgets (draw calls, triangles, image difference) are protected in `scenes.json`: only the lead changes them.
- `package.json` and the lockfile are protected: new dependencies are a decision of the lead (supply chain).

## What one agent would struggle with and the swarm does well

Hundreds of content entries and assets, each checked; many levels or puzzles, each with its proof; translations;
balance runs; parallel scene tests across the whole game; and models of different families checking each other's work.
