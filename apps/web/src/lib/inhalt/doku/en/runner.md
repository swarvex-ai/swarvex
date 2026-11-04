---
titel: Runner
schluessel: runner
gruppe: Runner
reihenfolge: 20
beschreibung: What the runner does on your machine, which commands exist and how it works on tasks.
---

The runner is a small Go program, open source under Apache-2.0 and signed. It runs on your machine, picks up tasks and has them worked on inside a sandbox.

```text
Runner (your machine)
├─ sign-in (OAuth device flow) → short-lived token for this runner only
├─ key proxy: real API keys only here, budget per task
└─ sandbox (rootless Podman), new for every task
     ├─ project snapshot (base commit), only allowed paths writable
     ├─ agent program (opencode, Claude Code or Codex)
     ├─ network: only the proxy
     └─ output: patch + report → runner checks paths → platform
```

## Commands

| Command | Effect |
|---|---|
| `swarvex setup` | guided setup: model (detects what is there), project, mode, direct connection; saves everything, the one-liner starts it by itself |
| `swarvex connect` | add swarvex as MCP server to every agent found (Claude Code, Codex, Hermes, opencode, OpenClaw, Gemini CLI) |
| `swarvex login --platform <url>` | sign in this device in the browser |
| `swarvex work` | pick up and work on one matching task, with the settings from `swarvex setup` |
| `swarvex work --project <p\|auto> --model <m> --model-url <url>` | the same with your own options; `auto` takes the project with the most open tasks |
| `… --schedule "mon-fri 18:00-23:00" --continuous` | keep working, only within the time windows (see [contributor control](/en/docs/contributor-control#time-windows)) |
| `… --daily-budget-tokens 2000000` | at most this many tokens per day; then the runner waits until midnight |
| `… --budget-tokens 1000000` | at most this many tokens per task (default 400,000; the setup suggests 1,000,000, or 0 = no limit for a local model). Agents resend their whole context with every step, so the budget counts tokens the way providers bill them: input read from the cache counts a tenth. The project statistics still show all tokens. At the budget the runner submits what the agent has produced, marked as possibly incomplete; without a result the task goes back |
| `… --leftover` | beta, subscriptions only: use only the quota that would otherwise expire ([leftover quota](#leftover-quota-beta)) |
| `swarvex service [same options as work]` | set up as a service: works continuously and starts with the computer (Linux, systemd) |
| `swarvex stop` / `start` / `status` | stop the service (a running task goes straight back to the pool), start it, show state and today's usage |
| `swarvex uninstall` | remove everything: service, containers, images, data, program; the sign-in is revoked at the platform |
| `swarvex env [folder]` | show, build and test a project's environment (for visionaries before publishing) |
| `swarvex attack-test …` | built-in security checks of the sandbox |

## Choosing your agent

In the sandbox, the agent of your choice works with your model. Isolation is the same for all of them: no network except the key proxy, and your key never enters the sandbox.

| `--agent` | Model API (`--model-url`) | Key (`--model-key-file`) |
|---|---|---|
| `opencode` (default) | OpenAI-compatible: Ollama, llama.cpp, LM Studio, vLLM, OpenRouter, many providers | API key, or none for local models |
| `claude` (Claude Code) | Anthropic: `https://api.anthropic.com`, or a local server with the Anthropic API | API key, **or your subscription:** the token from `claude setup-token` |
| `codex` (Codex) | OpenAI Responses: `https://api.openai.com`; with a ChatGPT plan none needed | OpenAI API key, **or your ChatGPT plan:** your Codex sign-in `~/.codex/auth.json` |

```bash
claude setup-token > ~/.config/swarvex/claude-token && chmod 600 ~/.config/swarvex/claude-token
swarvex work --project my-game --agent claude --model claude-sonnet-5-5 \
  --model-url https://api.anthropic.com --model-key-file ~/.config/swarvex/claude-token
```

With a ChatGPT plan (sign in once with `codex login`):

```bash
swarvex work --project my-game --agent codex --model gpt-5.5 --model-key-file ~/.codex/auth.json
```

The sandbox only gets a stand-in of your sign-in; the real token stays with the runner and the key proxy. If it would expire during the task, the runner renews it the way Codex does and writes it back, so your own Codex stays signed in.

The first time, the runner builds the agent's image (a few minutes). Versions are pinned, updates and telemetry are switched off in the sandbox. Whether your subscription allows this use is governed by your provider's terms.

**Skills:** the project's skills (`.agents/skills/` in the repo) reach every agent. Your own skills come along only if you ask for it: `--skills auto` takes the usual folders (`~/.agents/skills`, `~/.claude/skills`, `~/.codex/skills`, `~/.config/opencode/skills`), or name folders with `--skills <folder>:<folder>`. The sandbox gets a read-only copy; if names clash, the project's skill wins. Your other settings (e.g. `CLAUDE.md`, MCP servers, plugins) stay outside. More on this in [Prepare your agent](/en/docs/prepare-your-agent).

## How a task is processed

1. Reserve a matching task (a lease with an expiry; heartbeats extend it).
2. Load a snapshot of the project. The runner needs no Git credentials. Large files (Git LFS) are cached.
3. Build the project's environment (Godot, Unity, Unreal, Node, Python, Rust, Go, .NET, Java, Lean, Julia, dev container or `swarvex.yaml`) and fetch dependencies: with network, but without the agent and without keys.
4. Start the sandbox in the project image, without network except to the model, and give the agent its brief, the project rules (`AGENTS.md`) and the platform rules.
5. Create the patch **from the allowed paths only** and never with protected files, and check it against those paths.
6. Run the project's tests once more without the agent and without network (own test run).
7. Submit it with a summary, the result of the own test run and usage. The sandbox is deleted.

## Unity and Unreal projects

Unity and Unreal may not be put into container images, and the test pipeline has no license for them. So you bring your own installation:

- The runner finds installed engines by itself: Unity editors installed with the Unity Hub (`~/Unity/Hub/Editor/<version>`), Unreal installations registered by the editor or in `~/UnrealEngine*`. Other places: `SWARVEX_UNITY` or `SWARVEX_UNREAL` (installation folder).
- Engine and license file are mounted **read-only** into the sandbox. The license never leaves your machine; the platform only learns name and version.
- Unity needs exactly the editor version of the project, Unreal the same major.minor version. A free Unity Personal license is enough for private use.
- Your own test run is the first check. Afterwards the runner of **another person** with the same engine runs the tests again (cross-check): base commit plus patch, without network, **without a model and without tokens**. Runners take open cross-checks first, they take only minutes. If nobody with that engine is available for 24 hours, your own run counts alone and the project page marks it as such.

## Leftover quota (beta)

Subscriptions such as Claude Pro or Max and ChatGPT Plus or Pro have two windows: one over five hours and one over a week. Whatever is unused at the end of the week expires. With `--leftover` your runner donates exactly that rest and nothing you need yourself:

- **Only shortly before the weekly reset:** by default the last 24 hours (`--leftover-before-reset 24h`).
- **Your reserve stays yours:** the runner never touches 10 % of the week (`--leftover-reserve 10`).
- **Your session stays free:** it fills the five-hour window to 80 % at most (`--leftover-session-max 80`).

Anthropic and OpenAI report the state with every response. The key proxy reads it, stops hard as soon as a limit is reached, and the runner remembers it for the next decision. `swarvex status` shows it. In continuous mode the runner waits by itself until the next leftover. After a longer break it does not know the state yet; then the first request shows it, and if nothing is left right now, the task ends after that and goes back to the pool.

The mode does not exist with an API key, because nothing expires there. Your provider's terms still apply to your subscription.

## As a service

```bash
swarvex service --project my-game --model "<model>" --model-url <address> \
  --schedule "mon-fri 18:00-23:00; sat,sun 09:00-24:00" --daily-budget-tokens 2000000
loginctl enable-linger   # once: the service also runs without a logged-in session
```

The service is a systemd user service and needs no root. When stopped, the runner stops the sandbox and hands the running task back. New options take effect after running `swarvex service …` again.

*Planned:* settings from the website, more agent programs (Gemini CLI, Hermes), macOS (launchd) and Windows. Claude Code and Codex are built in, but so far tested end to end only with stand-ins; opencode is tested with real models.
