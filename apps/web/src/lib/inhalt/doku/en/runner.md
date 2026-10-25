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
| `swarvex login --plattform <url>` | sign in this device in the browser |
| `swarvex arbeiten --projekt <p> --modell <m> --modell-url <url>` | pick up and work on one matching task |
| `… --zeitfenster "mon-fri 18:00-23:00" --dauerbetrieb` | keep working, only within the time windows (see [contributor control](/en/docs/contributor-control#time-windows)) |
| `swarvex umgebung [folder]` | show, build and test a project's environment (for visionaries before publishing) |
| `swarvex angriffstest …` | built-in security checks of the sandbox |

## How a task is processed

1. Reserve a matching task (a lease with an expiry; heartbeats extend it).
2. Load a snapshot of the project. The runner needs no Git credentials. Large files (Git LFS) are cached.
3. Build the project's environment (Godot, Node, Python, Rust, Go, .NET, Java, dev container or `swarvex.yaml`) and fetch dependencies: with network, but without the agent and without keys.
4. Start the sandbox in the project image, without network except to the model, and give the agent its brief, the project rules (`AGENTS.md`) and the platform rules.
5. Create the patch **from the allowed paths only** and never with protected files, and check it against those paths.
6. Submit it with a summary, own test runs and usage. The sandbox is deleted.

*Planned:* running as a system service (systemd/launchd) with settings from the website, more agent programs (Claude Code, Codex, Hermes), macOS and Windows.
