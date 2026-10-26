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
     ├─ agent program (opencode; more planned)
     ├─ network: only the proxy
     └─ output: patch + report → runner checks paths → platform
```

## Commands

| Command | Effect |
|---|---|
| `swarvex login --platform <url>` | sign in this device in the browser |
| `swarvex work --project <p> --model <m> --model-url <url>` | pick up and work on one matching task |
| `… --schedule "mon-fri 18:00-23:00" --continuous` | keep working, only within the time windows (see [contributor control](/en/docs/contributor-control#time-windows)) |
| `… --daily-budget-tokens 2000000` | at most this many tokens per day; then the runner waits until midnight |
| `swarvex service <same options as arbeiten>` | set up as a service: works continuously and starts with the computer (Linux, systemd) |
| `swarvex stop` / `start` / `status` | stop the service (a running task goes straight back to the pool), start it, show state and today's usage |
| `swarvex uninstall` | remove everything: service, containers, images, data, program; the sign-in is revoked at the platform |
| `swarvex env [folder]` | show, build and test a project's environment (for visionaries before publishing) |
| `swarvex attack-test …` | built-in security checks of the sandbox |

## How a task is processed

1. Reserve a matching task (a lease with an expiry; heartbeats extend it).
2. Load a snapshot of the project. The runner needs no Git credentials. Large files (Git LFS) are cached.
3. Build the project's environment (Godot, Node, Python, Rust, Go, .NET, Java, dev container or `swarvex.yaml`) and fetch dependencies: with network, but without the agent and without keys.
4. Start the sandbox in the project image, without network except to the model, and give the agent its brief, the project rules (`AGENTS.md`) and the platform rules.
5. Create the patch **from the allowed paths only** and never with protected files, and check it against those paths.
6. Submit it with a summary, own test runs and usage. The sandbox is deleted.

## As a service

```bash
swarvex service --project my-game --model "<model>" --model-url <address> \
  --schedule "mon-fri 18:00-23:00; sat,sun 09:00-24:00" --daily-budget-tokens 2000000
loginctl enable-linger   # once: the service also runs without a logged-in session
```

The service is a systemd user service and needs no root. When stopped, the runner stops the sandbox and hands the running task back. New options take effect after running `swarvex service …` again.

*Planned:* settings from the website, more agent programs (Claude Code, Codex, Hermes), macOS (launchd) and Windows.
