# three.js: changes from r150 to r187 that code from memory gets wrong

Grouped by topic, newest last. Source: the official [migration guide](https://github.com/mrdoob/three.js/wiki/Migration-Guide).

## Renderer and shading

| Since | Old | Now |
|---|---|---|
| r163 | WebGL 1, node materials on `WebGLRenderer` | WebGL 2 only; node materials need `WebGPURenderer` |
| r171 | `three/examples/jsm/renderers/webgpu/…`, `three/nodes` | `import { WebGPURenderer, MeshStandardNodeMaterial } from "three/webgpu"`, `import { color, uv, … } from "three/tsl"` |
| r181 | `renderAsync()`, `computeAsync()`, `waitForGPU()` | `await renderer.init()` once, then `render()` / `compute()`; `setAnimationLoop()` handles init itself |
| r182 | `colorBufferType` | `outputBufferType`; `WebGPURenderer` is the recommended renderer |
| r183 | `PostProcessing`, `WebGLCubeRenderTarget` with WebGPU | `RenderPipeline`, `CubeRenderTarget` |
| r185 | premultiplied alpha as before in `WebGPURenderer`; `directionToColor()` | changed premultiplication; `packNormalToRGB()` / `unpackRGBToNormal()` |
| r186 | `PCFSoftShadowMap` with WebGPU | `PCFShadowMap` (now soft) |

TSL renames that break old snippets: `TextureNode.uv()` → `sample()` (r172), `varying()` → `toVarying()`,
`vertexStage()` → `toVertexStage()` (r173), `viewportTopLeft` → `viewportUV`, `uniforms()` → `uniformArray()`,
chained effects like `outputPass.fxaa()` → `fxaa(outputPass)` (r167–r168), blend functions `blendBurn()`,
`blendDodge()`, `blendScreen()`, `blendOverlay()` (r171).

## Colour and light

| Since | Old | Now |
|---|---|---|
| r152 | `renderer.outputEncoding`, `texture.encoding`, `sRGBEncoding` | `renderer.outputColorSpace`, `texture.colorSpace`, `SRGBColorSpace`; `ColorManagement.enabled` is `true` by default |
| r155 | `useLegacyLights = true` | physically based light units by default; `useLegacyLights` later removed |
| r177 | `ColorManagement.fromWorkingColorSpace()` / `toWorkingColorSpace()` | `workingToColorSpace()` / `colorSpaceToWorking()` |

## Geometry, loaders, addons

| Since | Old | Now |
|---|---|---|
| r151 | `mergeBufferGeometries()`, `mergeBufferAttributes()` | `mergeGeometries()`, `mergeAttributes()` |
| r152 | attributes `uv2` … `uv4` | `uv1` … `uv3` |
| r159 | `BatchedMesh.applyGeometry()`, `attribute.updateRange` | `addGeometry()` (and `addInstance()` since r166), `updateRanges` |
| r163 | `TextGeometry({ height })` | `TextGeometry({ depth })` |
| r168 | `PointerLockControls.getObject()`, `DragControls.activate()` | `controls.object`, `connect()` / `disconnect()` |
| r169 | `TransformControls` added to the scene | add `controls.getHelper()` |
| r176 | `CapsuleGeometry(radius, length)` | `CapsuleGeometry(radius, height)` |
| r180 | `RGBELoader`, `RGBMLoader` | `HDRLoader`; RGBM removed |
| r183 | `Clock` | `Timer` (core since r179; call `timer.connect(document)` for page visibility) |
| r186 | `Source` | `TextureSource`; `Object3D.dispose()` exists; `SimplifyModifier.modify()` is async |

## Habits that are still wrong

- Creating geometry or materials every frame: create once, reuse, `dispose()` when removed.
- One mesh per small object in large numbers: use `InstancedMesh` or `BatchedMesh` (draw calls are the budget).
- `Math.random()` or wall-clock time in game logic: the swarm cannot test it. Use the seeded `rng` and fixed steps.
- Loading textures or models from other sites at runtime: assets live in the repository with their licence.
