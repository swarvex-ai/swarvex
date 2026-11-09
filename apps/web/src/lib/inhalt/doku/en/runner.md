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
├─ key proxy: real API keys only here, token budget
└─ sandbox (rootless Podman), new for every task
     ├─ project snapshot (base commit), only allowed paths writable
     ├─ agent program (opencode, Claude Code or Codex)
     ├─ network: only the proxy
     └─ output: patch + report → runner checks paths → platform
```

## Commands

| Command | Effect |
|---|---|
| `swarvex setup` | guided setup: choose the model (it detects what is there), then one question for the recommended settings (any project, background service, signed updates, budgets by kind of access, fair use for subscriptions, found agents connected) or each setting yourself; the one-liner starts it by itself. `--new` asks only what a newer version added |
| `swarvex setup --project <project>` | the same setup, but the quick start works for this project only instead of anywhere (this is what a project's share link shows) |
| `swarvex connect` | add swarvex as MCP server to every agent found (Claude Code, Codex, Hermes, opencode, OpenClaw, Gemini CLI) |
| `swarvex login` | sign in this device in the browser (another platform: `--platform <url>`) |
| `swarvex work` | pick up and work on one matching task, with the settings from `swarvex setup` |
| `swarvex work --project <p\|auto> --model <m> --model-url <url>` | the same with your own options; `auto` takes the project with the most open tasks |
| `… --schedule "mon-fri 18:00-23:00" --continuous` | keep working, only within the time windows (see [contributor control](/en/docs/contributor-control#time-windows)) |
| `… --daily-budget-tokens 2000000` | at most this many tokens per day; then the runner waits until midnight |
| `… --budget-tokens 1000000` | at most this many tokens per task (default 0 = no limit: the daily budget or leftover mode caps every task anyway, mid-task to what is left of the day). Agents resend their whole context with every step, so the budget counts tokens the way providers bill them: input read from the cache counts a tenth. The project statistics still show all tokens. At the budget the runner submits what the agent has produced, marked as possibly incomplete; without a result the task goes back |
| `… --leftover` | beta, subscriptions only: use only the quota that would otherwise expire ([leftover quota](#leftover-quota-beta)) |
| `swarvex service [same options as work]` | set up as a service: works continuously and starts with the computer (Linux, systemd) |
| `swarvex stop` / `start` / `status` | stop the service (a running task goes straight back to the pool), start it, show state, today's usage and the space the work data takes (a stopped service shows how to start it again) |
| `swarvex help` | all commands with a short description (also `-h`, `--help`); `swarvex help <command>` shows its options |
| `swarvex update` | fetch the newest version from the platform, with the same signature check as the one-liner; your settings stay. A running task finishes first, the next one starts with the new version (`--now` switches at once, `--check` only looks and shows what is new). It asks nothing, except a question a new version brought along. If the new version does not start, the previous one comes back by itself; `--rollback` goes back by hand. On Windows the same command works in PowerShell. With automatic updates (the setup asks) the service does this itself once a day, between tasks. A service you stopped stays stopped; it runs the new version once you start it (`swarvex start`) |
| `swarvex history` | the last tasks of this runner: submitted or handed back with the reason, tokens, model time and the task's state on the platform today |
| `swarvex doctor` | checks sign-in, version, Podman, images, network, settings, whether the sandbox reaches the model, the service and free space, and says what to do for every problem |
| `swarvex budget` | show the token limits and media briefs per day; change them with `--daily 2000000`, `--task 0` (0 = no limit) and `--media 10` (0 = no media briefs, at most 30) without going through the setup. A running task finishes first (`--now` switches at once) |
| `swarvex logs` / `version` | follow the service's output; show the installed version, e.g. v0.12.0 (`status` also says when an update is available). Versions are semantic: the patch number for fixes, the minor number for new features; changes are listed in [runner/CHANGELOG.md](https://github.com/swarvex-ai/swarvex/blob/main/runner/CHANGELOG.md) |
| `swarvex clean` | free disk space: work trees left behind, old logs, and caches, LFS files and environment images nothing has used for 30 days. The service does this once a day by itself, and a task that goes back to the pool deletes its work tree at once. `--dry-run` only shows what would go, `--all` also takes younger ones, never what a running task uses |
| `swarvex uninstall` | remove everything: service, containers, images, data, program; the sign-in is revoked at the platform |
| `swarvex env [folder]` | show, build and test a project's environment (for visionaries before publishing) |
| `swarvex project lead setup <project>` | for visionaries and leads: run the project's lead as a service in its own sandbox; without the runner installed, in one step: `curl -fsSL https://swarvex.ai/install.sh \| sh -s -- lead <project>`; with `status`, `logs`, `attach`, `exec`, `cycle`, `stop`, `start`, `remove` ([lead service](/en/docs/lead-service)) |
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
3. Build the project's environment (Godot, Unity, Unreal, Node, Python, Rust, Go, .NET, Java, Lean, Julia, dev container or `swarvex.yaml`) and fetch dependencies: without the agent and without keys, out to the internet only through an egress proxy that blocks your local network and your computer (private and local addresses). For a package mirror in your own network: `--setup-allow <ip|cidr|host>`; `--setup-network lan` allows your whole local network, `--setup-network open` the full network as before v0.9.3. Blocked targets are listed in the setup log. This applies to the setup step only: the agent reaches your model through the key proxy, a model server on your own network too. A server on this computer is `http://host.containers.internal:<port>` from the sandbox; the runner rewrites `localhost`, `127.0.0.1` and this computer's own address to it.
4. Start the sandbox in the project image, without network except to the model, and give the agent its brief (with the files the lead named to read first and the patch limits), the project rules (`AGENTS.md`) and the platform rules.
5. Create the patch **from the allowed paths only** and never with protected files, and check it against those paths and the platform's limits (500 changed lines, 20 files). A larger patch is handed back with the reason, so the lead can split the task.
6. Run the project's tests once more without the agent and without network (own test run).
7. Submit it with a summary, the result of the own test run and usage. The sandbox is deleted. If the platform rejects the submission anyway, the task is handed back at once with its reasons.

**Cross-checks first.** At the start of each round the runner asks whether another person's submission waits for a cross-check and, if so, runs its tests again: base commit plus patch, without network, **without a model and without tokens**, then the round ends. Your green run together with the submitter's own green run replaces the test pipeline, so checking grows with the swarm. A red cross-check never rejects on its own; the test pipeline then decides.

## Media briefs

If your agents have media tools (Meshy, ElevenLabs, Suno, Runway, Leonardo, Kling, Higgsfield, Midjourney as MCP servers in Claude Code, Hermes, Codex, opencode or Gemini CLI), the runner can fulfil projects' media briefs with them, fully automatic ([donations](/en/docs/donations#media-tools-through-the-runner)). The setup finds them by itself; only which server is used is stored (`--media claude:meshy`), command, address and key are read afresh from your agent's settings for every brief.

1. A waiting brief comes before code tasks; starting it counts against your daily limit (`--media-daily`, default 10).
2. Each tool runs in its own container: it holds the key, may only write to an output folder and reaches the internet only through the egress proxy (not your home network).
3. The agent works in the sandbox as usual and reaches the tool only through the runner, with a throwaway token; it never sees the key. At most 8 generating calls per brief (`--media-max-calls`), status checks and downloads do not count.
4. The runner takes the file from the output folder, checks type, size and content (a `.glb` must really be a glTF model) and uploads it. Texts and archives never go through the runner. If something fails, the brief goes back with the reason for the lead and still counts; it then pauses for 24 hours, so a brief that keeps failing does not use up your credits (after a plain stop of the runner it is free again at once).

A key that is only exported in your shell is invisible to the service: put it into `~/.config/swarvex/media.env` (`NAME=value`); `swarvex setup` names what is missing.

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
- **Optionally at night:** in the night windows (`--leftover-night "daily 01:00-06:00"`) the runner may work before the reset as well, up to a share of the week per night (`--leftover-night-share 10`), and only while less than 80 % of the week is used (`--leftover-night-max 80`). The night is one five-hour window, so it has run out by the morning.

The setup picks fair values for your plan (Codex reads it from your sign-in, for Claude it asks once): Claude Pro and ChatGPT Plus only what would expire, Claude Max 5x also 5 % per night, Claude Max 20x and ChatGPT Pro 10 % per night. It never uses work accounts (Team, Business, Enterprise) on its own. Every value can be changed, up to 100 %: under "choose each setting yourself" in `swarvex setup`, or with the flags above.

Anthropic and OpenAI report the state with every response. The key proxy reads it, stops hard as soon as a limit is reached, and the runner remembers it for the next decision. `swarvex status` shows it. In continuous mode the model waits by itself until the next leftover; meanwhile the runner cross-checks other people's submissions (tests only, no model, no tokens), within your time windows. The same happens when the daily budget is used up. `swarvex status` shows what it waits for and until when. After a longer break it does not know the state yet; then the first request shows it, and if nothing is left right now, the task ends after that and goes back to the pool.

The mode does not exist with an API key, because nothing expires there. Your provider's terms still apply to your subscription.

## As a service

```bash
swarvex service --project my-game --model "<model>" --model-url <address> \
  --schedule "mon-fri 18:00-23:00; sat,sun 09:00-24:00" --daily-budget-tokens 2000000
loginctl enable-linger   # once: the service also runs without a logged-in session
```

The service is a systemd user service and needs no root. When stopped, the runner stops the sandbox and hands the running task back. New options take effect after running `swarvex service …` again.

*Planned:* settings from the website, more agent programs (Gemini CLI, Hermes), macOS (launchd) and Windows. Claude Code and Codex are built in, but so far tested end to end only with stand-ins; opencode is tested with real models.
