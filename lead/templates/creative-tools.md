# Creative tools for agents: the ArtCraft Crafting Apps

> For leads and area leads who cut asset, UI and media tasks, and for contributors whose agents work on them. Recommended, never required: contributors choose their tools. Checked on 2 Nov 2026; every app is young and moves fast, so name the version you used.

The [Crafting Apps](https://getartcraft.com/apps) by the ArtCraft team (the makers of Craft Anything) are open-source creative tools under MIT or Apache-2.0, native on Windows, macOS and Linux and in the browser. What makes them useful for a swarm: **every app can be driven by an agent** through a command line and an MCP server, so an agent can make a material, paint a texture or cut a trailer the way a person would.

## Rules for using them on swarvex

- **Locally, on the contributor's own machine.** They are trusted open-source tools, not project code, so they run outside the runner's sandbox. Give the MCP server only the folders the task needs: `--read-root` and `--write-root` point at a working folder, never at your home directory.
- **Files from the repository are untrusted input** for these apps like for every other tool: open only what the task names.
- **The result belongs to the project.** Output made from the apps' own procedural nodes, your own images or project assets falls under the project's asset licence. Anything third-party that goes in (a photo, a font, a brush) must itself allow redistribution (art role card, "Where assets come from").
- **Submit the source with the export.** The project file (`.mcraft`, `.svg`, `.csv`, the app's project file) goes in next to the exported files, so a later agent can change a parameter instead of starting over. Name app and version in `CREDITS.md`.
- **Maturity is self-assessed** by the makers. Treat a bug in an app like a bug in any tool: report it upstream, work around it, never ship a broken export.

## Which app for what

| App | What it does | Roles and tasks | Source in the repo | State (makers' estimate) |
|---|---|---|---|---|
| [**TextureCraft**](https://github.com/storytold/texturecraft) | 3D texture painting: layers, masks, smart materials, baking, PBR export, UDIM | art: texture a model from Meshy, TRELLIS or the swarm; fix seams; bake maps | its project file plus the exported maps | ≈ 93 % of a texture artist's daily work |
| [**MaterialCraft**](https://github.com/storytold/materialcraft) | procedural, tileable PBR materials from node graphs | art: materials after the art bible (walls, floors, metal, ground), variants by parameter (e.g. wear) | `.mcraft` plus exported maps | ≈ 67 % of a material artist's daily work |
| [**VectorCraft**](https://github.com/storytold/vectorcraft) | vector illustration: icons, logos, UI graphics, SVG, PDF | ux: icon sets, HUD and menu graphics, app icons, logos | `.svg` (text: every patch shows the change) | ≈ 65 % ready for real work |
| [**PhotoCraft**](https://github.com/storytold/photocraft) | image editing: layers, masks, adjustment layers, real PSD files | art, ux: sprites, concept clean-up, texture touch-ups, store images, screenshots | the layered file (`.psd`) plus the export | the most popular app of the family on GitHub |
| [**EffectCraft**](https://github.com/storytold/effectcraft) | motion graphics and compositing; exports video and **Lottie** | ux: UI and menu animations as Lottie (JSON, for apps and websites); release: trailer titles and effects | the project file; Lottie `.json` is itself text | young, already usable |
| [**FilmCraft**](https://github.com/storytold/filmcraft) | video editing, colour and sound | release: trailers, devlogs, store videos from captured gameplay | the project file (or an `.otio` timeline) plus the render | alpha, ≈ 56 % ready for real work |
| [**GridCraft**](https://github.com/storytold/gridcraft) | spreadsheets with formulas and charts | game-design: balancing and value tables, kept as `.csv` in the repo so the game reads them and patches stay readable | `.csv` (text) | clean-room spreadsheet, MCP server |

Less useful for our projects, mentioned so nobody has to look again: **DesignCraft** (page layout: a press kit or a printed manual at most), **LightCraft** (raw photo development: only for photographed references), **PdfCraft** (PDF editing) and **ArtCraft** itself (an AI image and video studio; for its output the rule for generated assets applies: the terms of the model used must allow the project's asset licence).

## Connecting them to an agent

Each app ships a `…-cli` with an `mcp` subcommand. For Claude Code, with the folders limited to the task's working folder:

```bash
claude mcp add materialcraft -- materialcraft-cli mcp --read-root ~/swarvex-work/materials --write-root ~/swarvex-work/materials
claude mcp add texturecraft  -- texturecraft-cli mcp --read-root ~/swarvex-work/textures --write-root ~/swarvex-work/textures
claude mcp add vectorcraft   -- vectorcraft-cli mcp
claude mcp add gridcraft     -- gridcraft-cli mcp
```

Other agents register the same command as a stdio MCP server. Each app's README has the exact flags; some can also drive the running desktop app through a control channel.

## Readable diffs for packaged sources

A `.mcraft` package is a zip with a `package.json` inside, so Git shows it as binary. With two lines the JSON becomes the diff:

```bash
# in the project repository, committed once (the lead adds it with the baseline package)
echo '*.mcraft diff=mcraft' >> .gitattributes
# on each machine that reviews
git config diff.mcraft.textconv 'unzip -p "$1" package.json #'
```
