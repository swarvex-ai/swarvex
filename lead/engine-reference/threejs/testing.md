# Testing a three.js project on swarvex

Tests are the acceptance of every task. They run without network and without a GPU, in the swarvex environment
`threejs` (Node 24, Vite, Chromium with SwiftShader), the same on every contributor's machine and in the test pipeline.

## Core tests (Vitest, milliseconds)

`tests/core/*.test.ts` import from `src/core` and `src/content` only. Check determinism (same seed, same world),
invariants after many steps, and that content is complete and valid. Most tasks are proven here.

## Scene tests (headless Chromium)

`tests/scene/run.mjs` serves `dist/`, opens each scene of `tests/scene/scenes.json` with `?test&<query>`, steps the
world `steps` times through `window.__swarvex.step(n)` and checks:

- no page errors and no console errors,
- `drawCalls` and `triangles` within the scene's budget (from `renderer.info.render`),
- the canvas image against `tests/scene/baseline/<name>.png`, at most `maxDiffPercent` of pixels changed.

The renderer runs with `forceWebGL` in test mode: WebGL 2 through SwiftShader is deterministic across machines (the
template's baseline matches between a desktop Chromium and the Debian Chromium of the environment). WebGPU is for the
game in a real browser.

A new scene or an intended visual change: run `npm run test:scene`, look at `tests/scene/out/<name>.png`, and commit
the new baseline in the same submission. A missing baseline fails on purpose. Reviewers compare the old and new image.

## Commands

| Command | Does |
|---|---|
| `npm test` | types, core tests, build, scene tests (what the pipeline runs) |
| `npm run test:core` | core tests only, while working on logic |
| `npm run test:scene` | build and scene tests |

Outside the swarvex environment, set `CHROME_PATH` to a Chromium or Chrome binary.
