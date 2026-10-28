# Template: style guide

> Lives in the project repository as `docs/style-guide.md`. Created in phase 1, before asset tasks go out. Every asset brief refers to it; reviews check against it.

## Pillars of the look
<2–4 sentences: mood, era, what it should feel like> · references: <images or works with "adopt / don't adopt">

## Palette
| Role | Color | Use |
|---|---|---|
| base | #… | walls, floors |
| accent | #… | interactive things, danger |
| … | | |

## Shape language
- <e.g. "smooth, rounded industrial forms; sharp edges only on dangerous things">
- silhouettes readable at <size> on screen

## Characters and objects
- scale: <unit, e.g. 1 unit = 1 m; character height 1.8>
- polygon budget: <hero / enemy / prop>, textures: <resolution, PBR channels>
- naming: <lowercase_with_underscores, prefix per type>
- formats: <glTF/GLB, PNG, OGG/WAV>

## Animation
- skeleton: <name, bone naming>; frame rate: <fps>; root motion: <yes/no>
- source: own or generated motion, **no clips from libraries that cannot be published under the project license**

## Sound
- mood per area: <e.g. "server rooms: low hum, 50–120 Hz">
- loudness: <target LUFS for music, effects, voice>; formats: <OGG for music, WAV for short effects>
- voice: <languages, tone, synthesis or recording, source and license>

## Text and tone
- in-game language(s): <…>; tone: <e.g. "dry, technical; robots speak in short commands">
- typography: <fonts with license (OFL etc.)>, minimum size

## Rights
- assets under <project asset license>; every asset has a provenance note (source, tool, plan, prompt or author)
- forbidden: recognizable third-party characters, logos, product designs, brand names
