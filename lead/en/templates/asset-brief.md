# Template: asset brief (design, 3D, image, audio, video)

> Level A, capability `asset.*` or `design.*`. Costs other people's credits, so be especially careful. **Always two stages:** preview → your approval → final.

```markdown
## Goal
<What is created, what for, where in the project?> So that <value>.

## Placement
- Location: <scene/screen/level>
- Size on screen: <e.g. about 96 px tall at 1080p; camera from above at 35°>
- Context: <what is next to it, background color, lighting>

## Style
- Style guide: `docs/style-guide.md#<section>`
- References (1–3): <existing assets/images in the repository>
- Palette: <color values>
- Mood: <3 adjectives>
- Negative: <no text, no logos, no watermarks, no realism …>

## Technical specifications
| | |
|---|---|
| Format | e.g. GLB (glTF 2.0) / PNG with alpha / OGG Vorbis 44.1 kHz |
| Dimensions/scale | e.g. 1 unit = 1 m, height 1.2 m, facing −Z, pivot at the bottom center |
| Limits | e.g. ≤ 5,000 triangles, one texture 1024², file ≤ 2 MB |
| Rig/animation | e.g. humanoid rig; animations `idle`, `walk`, `attack` |
| Naming/folder | e.g. `assets/enemies/slime/slime.glb` |

## Prompt suggestion (for the service)
> <concrete prompt with style, view, background; negative list>

## Process and costs
1. Preview (e.g. Meshy preview without texture, ~20 credits). Deliverable: preview images.
2. Approval by the lead (with a reason if declined, at most one more preview).
3. Final (e.g. texture/refine, ~10 credits). Deliverable: file + screenshot.
Estimated total: <x> credits. No multiple assignment.

## Sign-off
- [ ] Imports in the test pipeline without errors or warnings
- [ ] Limits met (triangles, texture, file size)
- [ ] Screenshot in project lighting: silhouette readable, style matches the reference
- [ ] Naming/folder correct
- [ ] Rights: contribution statement confirmed; the project license (e.g. CC BY 4.0) applies
```

## Example: 3D enemy

- **titel:** 3D model slime (preview → final)
- **faehigkeiten:** `["asset.3d"]`, **pfade:** `["assets/enemies/slime/**"]`
- Placement: side view, about 90 px tall, dark cave with cool light
- Style: like `assets/enemies/bat/bat.glb`, low poly, flat color areas, palette `#5cc28d`, `#2f7a55`, highlight `#e4ffe9`
- Prompt: *"cute low-poly slime monster, translucent green jelly body with two big eyes, flat shading, game asset, side view, plain background, no text"*
- Limits: ≤ 3,000 triangles, one texture 512², pivot at the bottom center, height 0.8 units
- Costs: preview ~20, final ~10 credits
