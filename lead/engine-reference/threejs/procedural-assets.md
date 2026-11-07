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

## When a file is the better choice

Characters with skeletal animation or scanned objects: use glTF (`.glb`) with a free licence and put the licence next
to it (`assets/LICENSES.md`: source, author, licence). Good CC0 sources: Kenney, Poly Haven, ambientCG, Quaternius.
Never load assets from other sites at runtime.

## Budget per asset

Every builder has a scene in `tests/scene/scenes.json` with a triangle budget. A builder that grows past it fails
the check, before the game gets slow.
