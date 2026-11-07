# three.js project on swarvex

Started from the swarvex three.js template. The layout is what lets many agents work in parallel without stepping on
each other: every part has its own folder, its own tests and a contract.

| Folder | What lives there | Checked by |
|---|---|---|
| `src/core/` | the simulation: pure TypeScript, no three.js, no DOM, no `Math.random` (use `rng`), fixed steps | `tests/core/` (Vitest, milliseconds) |
| `src/content/` | content as data: kinds, levels, items, texts | `tests/core/content.test.ts` (complete, unique, valid) |
| `src/assets/procedural/` | geometry, TSL materials, textures and sound made in code | scene tests (image and budget) |
| `src/render/` | builds and updates the three.js scene from the world; never changes the world | scene tests |
| `tests/scene/` | scenes with budgets (`scenes.json`) and baseline images | headless Chromium |

Commands: `npm run dev` (play), `npm test` (types, core tests, build, scene tests), `npm run build` (the preview).

Rules: one task touches one folder where it can; a new scene or a changed image comes with its baseline in the same
submission; budgets in `tests/scene/scenes.json` only change through the lead. Engine traps and conventions:
https://github.com/swarvex-ai/swarvex/tree/main/lead/engine-reference/threejs
