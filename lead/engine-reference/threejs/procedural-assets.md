# Assets in code

A three.js project needs few or no asset files. Geometry, materials, textures, pixel sprites, sound and music can be
functions in `src/assets/procedural/`: reviewable as a diff, testable, tunable by parameters, and free of generator
credits.

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

## Textures

Canvas or `DataTexture` generated at start for patterns. Keep generation deterministic (seeded `rng`), so the scene
tests see the same image.

Surfaces that repeat (floors, walls) as TSL, live or baked into PNG maps (checked on r187):

- **Tiling:** `mx_noise_float` does not repeat. Build noise on a lattice that wraps after a whole number of cells per
  tile and hash the wrapped cell with `mx_cell_noise_float(vec3(cell + 0.5, seed))` (integer bit mixing: the same on
  every GPU, WebGL and WebGPU). Wrap with `c - p * floor((c + 0.5) / p)`, not `mod()`: GPU division is inexact and
  WGSL's `%` keeps the sign. Test it: bake the map half a tile further on; it must equal the first bake rolled by
  half.
- **Shader size:** a surface calls noise and Voronoi dozens of times. Give them a layout
  (`Fn(([p, period, seed]) => …, { p: "vec2", period: "vec2", seed: "float", return: "float" })`) so they become one
  shader function; inlined, a cobblestone shader grew to 339 KB, as a function 51 KB.
- **Normal from height:** central differences one texel apart, scaled by the real relief and tile size in metres;
  OpenGL convention (green up), as Godot and Blender expect.
- **Cost:** a live surface runs per pixel and frame (a big floor with Voronoi is too much for small GPUs). Bake it once
  into textures for large or many surfaces: a full-screen quad with `vertexNode = vec4(positionGeometry.xy, 0, 1)`
  into a `FloatType` `RenderTarget`, in pieces of at most 512 × 512 per draw (a long single draw loses the WebGL device
  in software rendering), one baker renderer for the whole session (every renderer is a WebGL context).
- Formwerk has surfaces built this way (`cobblestone()`, `bakedMaterial()`, `bakeSurface()` from `"formwerk"`).

## Sound and music

A hit is a few thousand numbers, a music loop a few million. Compute them in plain TypeScript, not with Web Audio
nodes: `src/assets/sound/` holds pure functions `(params, seed) → Float32Array` at one fixed rate (48 kHz). No browser
API is involved, so the core tests run them in Node, and the gallery or a script can export them as WAV.

```ts
export const RATE = 48000;

/** A short hit: a falling tone with noise. Same seed, same samples. */
export function hit(seed: number, pitch = 220) {
  const random = rng(seed);
  const out = new Float32Array(Math.round(0.4 * RATE));
  let phase = 0;
  for (let i = 0; i < out.length; i++) {
    const t = i / RATE;
    phase += (2 * Math.PI * (pitch - 120 * t)) / RATE;
    out[i] = (Math.sin(phase) * 0.6 + (random() * 2 - 1) * 0.25) * Math.exp(-t * 18) * Math.min(1, t / 0.002);
  }
  return out;
}
```

- **Building blocks:** oscillators as a phase accumulator (as above; sine, square, saw, triangle from the phase),
  noise from the seeded `rng`, envelopes (an attack of at least 2 ms, or the first sample clicks), filters (one-pole
  low-pass, a biquad from the Audio EQ Cookbook formulas), pitch slides, a little delay for space.
- **Playback:** one context with the same rate, handed to three.js before the first `AudioListener`:
  `const ctx = new window.AudioContext({ sampleRate: RATE }); AudioContext.setContext(ctx);` (three's `AudioContext`;
  keep using your `ctx`: `@types/three` 0.187 types `getContext()` as three's class). Then
  `const b = ctx.createBuffer(1, s.length, RATE); b.copyToChannel(s, 0);` and play it with an `AudioBufferSourceNode`,
  or on an object with `PositionalAudio.setBuffer(b)` and the `AudioListener` on the camera. Browsers start the
  context suspended: call `ctx.resume()` in the first `pointerdown` or `keydown`.
- **Web Audio nodes** (`OscillatorNode`, `BiquadFilterNode`, `GainNode`) are for what changes live: an engine hum
  that follows the speed, the music muffled under water. Anything that is an asset (a hit, a jingle, a loop) is
  samples: testable, exportable, and the same sound a Godot project can import.
- **Music as data:** a pattern of notes (step, pitch, length, instrument) rendered into a buffer of exactly
  `bars × beatsPerBar × 60 / bpm × RATE` samples. Add the tails of notes that ring past the end to the start, so the
  loop has no click at the seam. Render layers (drums, bass, melody) as separate buffers, start them at the same
  `ctx.currentTime` and fade them with `GainNode`s for music that follows the game.
- **WAV export:** a 44-byte header and 16-bit PCM, a few lines of code. Godot, Blender and every editor import it.
- **Checks in the core tests:** length; peak below −1 dBFS (0.89); no DC offset; first and last samples near zero for
  one-shots, a seamless seam for loops; loudness (RMS) in the range of its kind; same seed, same samples. Compare with
  a stored reference within a tolerance, not by hash: `Math.sin` may differ in the last bits between engines.
- **Agents cannot listen.** Write a spectrogram of every sound as a PNG next to the scene images (`pngjs`), so a
  reviewer sees what changed, and listen to new sounds yourself before accepting them.
- **No melodies of known games, films or songs**, not even rebuilt from notes: a melody is protected however it is
  synthesised.
- **Look in the catalogue first** (`find_assets`): Formwerk has sounds built this way, voices without words included (`import { hit,
  footstep, creature, babble } from "formwerk/sounds"`, no three.js, CC0). A sound it lacks is a good request to its
  lead (`request_work`).

## Pixel sprites and characters

Animated 3D characters are the one asset that is hard to make in code. Pixel-art sprites in a lit 3D world (2.5D)
avoid it: a character is a small image per frame, and code can draw those images.

- **Drawing:** into an RGBA `Uint8ClampedArray` with a small helper (pixel, line, rect, ellipse, fill, outline) in
  plain TypeScript with integer coordinates. No browser API, and the result is the same bit for bit on every machine,
  so a core test compares each sheet with its baseline PNG exactly.
- **Characters from parts:** a body template per direction (4 or 8), layers on top (clothes, hair, the held item),
  each material a ramp of 3 to 5 shades from one palette, then a one-pixel dark outline around the silhouette.
  Animations are offsets of the parts per frame (a walk: 4 frames, legs and arms shifted, the body bobbing one pixel).
  Parameters and a seed make a whole village from one builder.
- **Sheets:** one per character, rows = animation × direction, columns = frames, plus data with frame size, frame
  count and frames per second per animation, pivot at the feet.
- **Texture:** `new DataTexture(pixels, width, height)` (RGBA, then `needsUpdate = true`) or a `CanvasTexture`, with
  `magFilter = minFilter = NearestFilter`, `generateMipmaps = false`, `colorSpace = SRGBColorSpace`. Mind the row
  order: a `DataTexture` (`flipY` false) puts the first row of the array at v = 0, a `CanvasTexture` puts the top of
  the canvas at v = 1. Use one pixel density for everything (for example 32 pixels per metre), so pixels have the
  same size on characters, props and ground.
- **Lit and shadowed:** not `Sprite` (`SpriteMaterial` is unlit), but a plane with its origin at the feet, turned to
  the camera around y every frame:

  ```ts
  const geometry = new PlaneGeometry(width / 32, height / 32).translate(0, height / 64, 0);
  const body = new Mesh(geometry, new MeshStandardNodeMaterial({ map: sheet, alphaTest: 0.5 }));
  body.castShadow = true;
  // every frame:
  body.rotation.y = Math.atan2(camera.position.x - body.position.x, camera.position.z - body.position.z);
  ```

  In r187 the shadow pass of `WebGPURenderer` takes the map's alpha and the `alphaTest`, so the shadow has the
  character's outline, not a rectangle.
- **Frames:** `sheet.repeat.set(1 / columns, 1 / rows)` and `sheet.offset` per frame. For many characters on one
  sheet, clone the texture per character (`texture.clone()` shares the image) or pick the frame in TSL from a
  per-instance attribute.
- **Checks:** the core test compares every sheet with `tests/core/sprites/<id>.png` pixel for pixel; the scene test
  shows the characters lit in the world.

Formwerk has sprites built this way (`import { villager } from "formwerk/sprites"`, no three.js), with Godot
SpriteFrames and a JSON of the frames.

[Luminati](https://gitlab.com/stubborn-hug/lumina) (MIT) is a large example of this look, with characters, textures,
sound and music all made in code. It renders with `WebGLRenderer` and GLSL shaders, so take the ideas, not the
shaders.

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

Before you write a chair, a shelf, a tree or a footstep yourself, look at
[Formwerk](https://github.com/swarvex-ai/formwerk), the swarvex catalogue of assets built this way (models,
surfaces, pixel sprites, interface icons and sounds so far, for games and apps alike): code MIT, the assets CC0, so you can import them or copy a builder into your project. An asset it
lacks is a good request to its lead (`request_work`).

## When a file is the better choice

3D characters with skeletal animation, or scanned objects: use glTF (`.glb`) with a free licence and put the licence
next to it (`assets/LICENSES.md`: source, author, licence). Good CC0 sources: Kenney, Poly Haven, ambientCG,
Quaternius. If the game's look allows it, pixel sprites (above) avoid the animated 3D character altogether. Never
load assets from other sites at runtime, fonts included: bundle them.

## Budget per asset

Every builder has a scene in `tests/scene/scenes.json` with a triangle budget. A builder that grows past it fails
the check, before the game gets slow.
