# three.js r187: what your model probably doesn't know

Most models learned three.js from code around r150 to r165. Since then the renderer, the shading language, imports and
many names changed. Code from memory then compiles but renders black, warns in the console or fails on WebGPU.

**Before you use an API you are not sure about, check here.** The authoritative source is the official
[migration guide](https://github.com/mrdoob/three.js/wiki/Migration-Guide) and the [docs](https://threejs.org/docs/).

| File | Contents |
|---|---|
| `breaking-changes.md` | what changed from r150 to r187 that code from memory gets wrong |
| `swarm-architecture.md` | the project layout that lets many agents work in parallel (core, content, render, scene tests) |
| `testing.md` | core tests in Node, scene tests in headless Chromium, budgets and baseline images |
| `procedural-assets.md` | geometry, TSL materials, textures and sound in code instead of model files or generator credits |

## Start

A new three.js project starts from the swarvex starter (`lead/templates/threejs`), laid out for parallel work: core
without three.js, content as data, procedural assets, scene tests in headless Chromium.

```sh
curl -fsSL https://swarvex.ai/lead/starter/threejs.sh | sh        # in the project folder; never overwrites a file
# in the lead service (swarvex project lead), through its proxy:
curl -fsSL -H "Authorization: Bearer $SWARVEX_TOKEN" "$SWARVEX_PROXY/lead/starter/threejs.sh" | sh
npm install && npm test                                            # writes the first baseline image: look at it
```

Commit everything, including `package-lock.json` and `tests/scene/baseline/`. In the lead service, `npm install` runs
in the runner's setup step after the commit (the sandbox has no registry access); commit the lockfile it creates.

## The most frequent traps


- **Renderer:** new projects use `WebGPURenderer` from `three/webgpu`; it falls back to WebGL 2 by itself. Call
  `await renderer.init()` before rendering on demand (`renderAsync()` is deprecated since r181).
- **Shaders:** `WebGPURenderer` does not run `ShaderMaterial` with GLSL. Write materials with TSL (`three/tsl`) and node
  materials (`MeshStandardNodeMaterial`, `colorNode`, `positionNode`).
- **Imports:** `three/webgpu`, `three/tsl`, and addons from `three/addons/…` (not `three/examples/jsm/…`).
- **Colour:** `texture.colorSpace = SRGBColorSpace` for colour textures (`encoding` is gone since r152); colour
  management is on by default. Lights use physical units (`useLegacyLights` is gone): old intensities look too dark.
- **Time:** `Clock` is deprecated (r183); use `Timer` (core since r179) or, better, the fixed step of the simulation.
- **Renames:** `mergeGeometries()` (was `mergeBufferGeometries`), `CapsuleGeometry(radius, height)` (was `length`),
  `HDRLoader` (was `RGBELoader`), `RenderPipeline` (was `PostProcessing`), `TextGeometry` `depth` (was `height`).

Version state: three.js r187 (npm `three@0.187.0`), checked against the migration guide on 2026-11-07. Pin the
version in `package.json`; the WebGPU and TSL APIs still move between releases. If you find a mistake here, mention it
in your submission summary.
