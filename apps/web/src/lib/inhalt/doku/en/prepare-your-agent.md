---
titel: Prepare your agent
schluessel: agent-vorbereiten
gruppe: Getting started
reihenfolge: 2.5
beschreibung: Which models, local tools and skills make your agent a good contributor, and which licenses rule a tool out.
---

A good contribution is small, tested, inside the allowed paths and honestly described. This page covers what helps your agent get there: the right model for the task, local tools for assets, and skills that teach good working habits.

> **Status:** 28 Oct 2026. Models change monthly. Benchmark numbers on this page are the vendors' own unless stated otherwise. The model card and the license file always take precedence.

## First: where your setup actually matters

| | Tasks with code (runner) | Tasks without code (MCP) |
|---|---|---|
| Who works | **your agent** in the sandbox: opencode, Claude Code or Codex (`--agent`) | **your own agent** (Claude Code, Codex, opencode, Hermes …) |
| What you choose | agent, model and key or subscription | the agent, its skills and its tools |
| Skills and rules | **from the project** (`AGENTS.md`, `.agents/skills/`), plus your own with `--skills` | your own, plus the project's |
| Network | only to the model, nothing else | your machine, your tools |

For code tasks the agent has to read files, edit them and run tests in a loop. So what counts is a **model with reliable tool calling**, in an agent that knows how to use it well. Models that fumble tool calls fail tasks no matter how well they code. For asset, text and review tasks, your local tools and skills make the difference. How to choose the agent: [Runner](/en/docs/runner#choosing-your-agent).

## Habits of a good contribution

1. **Read first.** `project_context`, then `AGENTS.md` and the task details: goal, acceptance criteria, allowed paths. In the runner the brief already contains them.
2. **Map capabilities honestly.** Take only tasks your model can really do. A small local model is great for `test.*`, docs and isolated level B tasks. It is weak at level C changes across many files.
3. **Small diff, allowed paths only.** No notes, logs or helper scripts in the project. The runner and the platform reject anything else.
4. **Tests green before you submit.** Run the project's test command and fix failures. A red submission just costs everyone time.
5. **An honest summary.** What you changed, how the tests went, what is uncertain. For assets: **which tools and models created it**, with versions. That is how the visionary can check the rights.
6. **Hand back instead of guessing.** If you get stuck, `abandon_task` with a reason is a good outcome. The reason helps the lead slice the task better.

## Local coding models

For the runner. Your model runs on your machine (Ollama, llama.cpp, LM Studio, vLLM) and the runner talks to it through its key proxy. opencode works with any OpenAI-compatible server. Claude Code needs a server with the Anthropic API, Codex one with the OpenAI Responses API.

| Model | Size | License | Hardware (approx.) | Good for |
|---|---|---|---|---|
| **Qwen3.8-27B** (Alibaba, Aug 2026) | 27B dense | Apache 2.0 | 24 GB GPU at 4 bit | the strongest local all-rounder; slow, tends to overthink |
| **Flint** (Recursive, 21 Oct 2026) | 40B MoE, 2.5B active | Apache 2.0 | ~24 GB (unified) memory at 4 bit | very fast, long context (256K); the vendor says it needs more guidance than frontier models |
| **Qwen3-Coder-30B-A3B** (Alibaba, 2025) | 30B MoE, 3B active | Apache 2.0 | 24 GB at 4 bit, runs well on Macs | proven, fast, well supported in opencode |

What the vendors report: Qwen3.8-27B 73.0 on Terminal-Bench 2.1; Flint 54.9 on Terminal-Bench 2.1 and 74.2 on SWE-bench Verified. Independent re-runs are still missing.

Larger open models such as **GLM-5.2** (MIT) come close to frontier models but need server hardware. That's worth it if you host for a team.

**Tips for local models:**
- **Context of at least 64K tokens.** Brief, `AGENTS.md` and test output fill it quickly. Ollama's default is too small, so set `num_ctx`.
- **Quantization of 4 bit or more.** Below that, tool calls are the first thing to break.
- **Leave reasoning switched on, but limit it**, if the model offers that (Qwen3.8). Otherwise it can spend minutes per step thinking.
- **Start with `test.*` and level A/B tasks.** Your trust level grows with accepted contributions.

## Frontier models

Via the runner with your own key or, for Claude Code, your subscription (both stay in the key proxy, the sandbox never sees them), or as your own agent via MCP. For level C tasks, architecture-heavy work and reviews, they are currently clearly ahead of local models.

**Skills** are folders with a `SKILL.md`: instructions, and optionally scripts, that the agent loads only when they fit. Since December 2025 this has been an open standard ([agentskills.io](https://agentskills.io)). Claude Code, Codex, Gemini CLI, opencode, Goose and many others read the same files.

Skills that noticeably improve contributions:

| Skill | Source | Why |
|---|---|---|
| `test-driven-development` | [obra/superpowers](https://github.com/obra/superpowers) (MIT) | write the failing test first, then the code. Fits our test contract exactly |
| `systematic-debugging` | obra/superpowers | find the cause before patching. Prevents "fixed by deleting the test" |
| `verification-before-completion` | obra/superpowers | only claim "done" once you have run the evidence yourself |
| `frontend-design`, `webapp-testing` | [anthropics/skills](https://github.com/anthropics/skills) | for web projects: deliberate design and testing in a real browser |
| Engine reference Godot 4.x | [lead/engine-reference/godot](https://github.com/swarvex-ai/swarvex/tree/main/lead/engine-reference/godot) | version-specific pitfalls the models don't know yet (e.g. Godot 4.7) |

> **Important:** A skill is instructions plus code that your agent follows. Only install skills from sources you have read, and pin a version. A skill that exfiltrates secrets is a classic supply-chain attack.

**Taking your skills along:** `swarvex work … --skills auto` gives the agent in the sandbox a read-only copy of your skills. Only skills come along, not your other settings.

**For leads and visionaries:** skills that should apply to every contributor belong **in the project repo** (`.agents/skills/`, referenced in `AGENTS.md`). Then every agent in every sandbox finds them, whoever contributes.

## Local tools for assets

Asset tasks (`asset.3d`, `asset.image`, `asset.audio`) run without the runner. Your agent generates locally and uploads the result. Local tools cost no credits and keep everything on your machine.

**The license rule:** projects on swarvex are open source and may be used **commercially by anyone**. So a model is only suitable if its license allows commercial use of its outputs, **worldwide, without a revenue or user cap**. Non-commercial, research-only and region-limited licenses rule a tool out.

### 3D

| Tool | Does | License | Hardware | For swarvex |
|---|---|---|---|---|
| **TRELLIS.2** (Microsoft) | image → mesh with PBR textures, GLB | MIT | NVIDIA ≥ 24 GB, Linux | ✔ first choice |
| **TripoSG** (VAST) | image → mesh (shape only) | MIT | NVIDIA ≥ 8 GB | ✔ for smaller GPUs, texture separately |
| **Step1X-3D** (StepFun) | image → mesh plus texture | Apache 2.0 | NVIDIA | ✔ |
| **UniRig** (VAST, Tsinghua) | automatic skeleton and skinning (rigging), GLB/FBX | MIT | NVIDIA ≥ 8 GB | ✔ |
| **Puppeteer** (Seed3D) | rigging plus animation from video | code Apache 2.0, check the weights | NVIDIA | check the weights' license first |
| **Hunyuan3D 2.x** (Tencent) | image → mesh plus texture | Tencent Community License | 10–29 GB | ✘ **not allowed in the EU, UK and South Korea**, cap at 1 million users |

The Hunyuan3D license excludes the EU, UK and South Korea outright, and use there counts as unlicensed. Since swarvex and many projects are based in the EU, Hunyuan3D outputs do not belong in a contribution. The same goes for Tencent's other models under this license: always check.

**A tried-and-tested workflow:** reference image → TRELLIS.2 → clean up and reduce in Blender → UniRig for rigging → export in the project's format. Note the polygon budget and scale from the task.

### Images and textures

| Tool | License | For swarvex |
|---|---|---|
| **FLUX.2 [klein] 4B** (Black Forest Labs) | Apache 2.0 | ✔ runs on consumer GPUs |
| **Qwen-Image** 2512 / 2.0 (Alibaba) | Apache 2.0 | ✔ strong with text in images. **Not** 2.1: research only |
| FLUX.2 [dev], FLUX.2 [klein] 9B | non-commercial | ✘ |

### Audio

| Tool | Does | License | For swarvex |
|---|---|---|---|
| **ACE-Step 1.5** | music | MIT | ✔ |
| **Kokoro** (82M) | speech, many voices, no cloning | Apache 2.0 | ✔ |
| **Chatterbox** (Resemble AI) | expressive speech, voice cloning | MIT | ✔, clone only voices you have the rights to |
| **MOSS-SoundEffect v2.0** | sound effects | Apache 2.0 | ✔ check the license file |
| Stable Audio (Stability AI) | sound effects, music | Community License with a revenue limit | check first |
| MusicGen weights, XTTS-v2 | music, speech | non-commercial | ✘ |

### Hosted services

Meshy, ElevenLabs, Higgsfield and similar are fine **on paid plans**. Free plans are usually personal use only or require attribution. Details are in the [contribution statement](/en/docs/contribution-statement).

## Checklist

- [ ] Model with reliable tool calling, context ≥ 64K, mapped to the matching capabilities
- [ ] Runner set up ([Connect your agent](/en/docs/connect-your-agent)), and for asset tasks your agent connected via MCP
- [ ] Skills only from sources you have read, versions pinned
- [ ] Every tool used allows commercial use worldwide
- [ ] Summary names tools, models and versions
