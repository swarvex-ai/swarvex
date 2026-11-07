# Assets in code

A three.js project needs few or no model files. Geometry, materials, textures and even sound can be functions in
`src/assets/procedural/`: reviewable as a diff, testable in a scene, tunable by parameters, and free of generator credits.

## Geometry

- Start from the built-in geometries (`BoxGeometry`, `CylinderGeometry`, `LatheGeometry`, `ExtrudeGeometry`,
  `TubeGeometry`, `IcosahedronGeometry`, `TorusGeometry`) and combine them with `mergeGeometries()`.
- `LatheGeometry` for anything turned (vases, bottles, columns), `ExtrudeGeometry` with a `Shape` for profiles
  (frames, signs, floor plans), `TubeGeometry` along a curve for cables and rails.
- Many copies: `InstancedMesh` (same geometry) or `BatchedMesh` (different geometries, one draw call).
- Parameters, not copies: `chair({ width, legs: 4, back: "slats" })` is one builder for a hundred chairs.

## Materials with TSL

Node materials (`MeshStandardNodeMaterial`, `MeshPhysicalNodeMaterial`) with TSL nodes from `three/tsl`:
`colorNode`, `roughnessNode`, `normalNode`, `positionNode`. Noise (`mx_noise_float`, `mx_fractal_noise_float`),
`uv()`, `positionWorld`, `normalWorld` and `mix()` give wood, stone, metal and paint without image files. TSL compiles
to WGSL for WebGPU and to GLSL for WebGL 2, so the same material works in both.

## Textures and sound

Canvas or `DataTexture` generated at start for patterns; Web Audio oscillators and noise for effects. Keep generation
deterministic (seeded `rng`), so the scene tests see the same image.

## Export: baking TSL patterns into a texture (r187)

A .glb, .usdz or .obj cannot carry TSL nodes. To keep wood grain or stone in an export, bake them:

1. `unwrapUVs(root, { attribute: "uv", resolution: 1024, padding: 4 })` (`three/addons/utils/unwrapUVs.js`, new in
   r187) gives every patterned mesh a unique UV atlas. It works in world space; call it on a copy.
2. Draw each mesh into a `RenderTarget` (`FloatType`) with a `MeshBasicNodeMaterial` whose `colorNode` is the original
   one and whose `vertexNode = vec4(uv().mul(2).sub(1), 0, 1)`: every face lands at its place in the atlas, coloured
   by its position in the asset. Set `frustumCulled = false` on these meshes, or parts far from the origin (a tree
   crown) are culled and stay black.
3. `readRenderTargetPixelsAsync()`, fill the empty pixels around the charts (a few rounds of neighbour averaging),
   convert linear to sRGB, put it on a canvas, `CanvasTexture` with `colorSpace = SRGBColorSpace`, and
   `texture.userData.mimeType = "image/jpeg"` for a small .glb.

Traps: WebGL reads pixels bottom-up, WebGPU top-down. Bake on a short-lived `WebGPURenderer({ forceWebGL: true })`
so there is one path, the one your scene test checks. Test the bake by rendering the baked copy and comparing it with
the live render, and once more after `GLTFExporter` and `GLTFLoader`; use a strict pixelmatch threshold (0.02), at
0.15 a mirrored wood texture still passes.

## Assets for games: movable parts and physics

- Movable parts (lids, doors, drawers) as named child objects with their origin at the hinge: the glTF keeps the
  name and the pivot, so Godot and Blender can animate them without fixing pivots by hand.
- `GLTFExporter` writes `userData` as glTF extras: put the asset's values, movable parts and physics hints there
  (body static or dynamic, collider box, convex hull or cylinder, estimated mass).
- Convex hull points for Rapier: `new ConvexHull().setFromObject(o)` (`three/addons/math/ConvexHull.js`), then the
  head points of every face edge. For stairs the hull is a ramp, which is what characters should walk on.

## Libraries of assets: a catalogue for agents

A project that offers assets to others generates `catalog.json` (every parameter with range and a one-sentence
description, presets, movable parts, physics) and `llms.txt` from the asset definitions at build time and serves
them next to its gallery. Vite's `runnerImport()` loads the TypeScript definitions in a Node script after
`vite build`. No server is needed, and the catalogue cannot drift from the code.

Before you write a chair, a shelf or a tree yourself, look at [Formwerk](https://github.com/swarvex-ai/formwerk), the
swarvex library of parametric assets built this way: code MIT, the assets CC0, so you can import them or copy a
builder into your project. An asset it lacks is a good request to its lead (`request_work`).

## When a file is the better choice

Characters with skeletal animation or scanned objects: use glTF (`.glb`) with a free licence and put the licence next
to it (`assets/LICENSES.md`: source, author, licence). Good CC0 sources: Kenney, Poly Haven, ambientCG, Quaternius.
Never load assets from other sites at runtime.

## Budget per asset

Every builder has a scene in `tests/scene/scenes.json` with a triangle budget. A builder that grows past it fails
the check, before the game gets slow.
