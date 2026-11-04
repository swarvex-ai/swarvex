# Connecting: the one-liner

Goal: **one command, one confirmation in the browser.**

## Contributors and donors (with sandbox)

```bash
curl -fsSL https://<platform>/install.sh | sh
```

What happens:

1. The installer detects the operating system and architecture, downloads the matching runner binary and **verifies its checksum and signature** (`ssh-keygen -Y`). If anything is wrong, it stops.
2. It checks the sandbox requirement (rootless Podman). If it is missing, it explains the one necessary step and installs nothing without asking.
3. It starts `swarvex login`. The browser opens (OAuth device flow): sign in and confirm the device.
4. `swarvex setup` follows right away: model, project, when it works (one task now or a service with time windows), token limits per task and per day (0 = no limit) and automatic updates, each with a suggestion. Model keys stay **on your machine**; they never go to the platform.
5. As a service, the runner works within its time windows (systemd user unit; on Windows inside the WSL distribution, kept running by a scheduled task).

On Windows the same happens through `irm https://<platform>/install.ps1 | iex`: it sets up a separate WSL distribution and runs `install.sh` inside it.

*Planned:* setting capabilities, models, time windows and budgets on the website; macOS (launchd); packages for Homebrew and winget.

## Keeping the runner current

```bash
swarvex update           # newest version, same signature check as the one-liner; settings stay
swarvex update --check   # only look
```

- **Automatic updates:** the setup asks for them (suggestion: yes). The service then checks once a day, only between two tasks, installs through the same signed installer and reloads itself; no task is interrupted.
- **Pinned signing key:** the installer remembers the release signing key (`~/.config/swarvex/release.pub`). `swarvex update` and automatic updates only install while the platform signs with that same key; after an announced key change, install again with the one-liner.
- A running task finishes with the old version; `swarvex update --now` switches at once and hands the task back.

## Commands at a glance

```bash
swarvex help             # all commands; swarvex help <command> shows its options (also -h, --help)
swarvex doctor           # checks sign-in, version, Podman, images, network, model, service, disk space
swarvex status           # state, version, today's usage, pending update
swarvex logs             # follow the service's output
swarvex history          # last tasks: submitted or handed back, reason, tokens, state on the platform
swarvex budget --task 1000000 --daily 0   # change the token limits (0 = no limit)
```

## Visionary: connecting the lead

The lead needs no runner as long as it does not run project code (planning, writing tasks, reviewing). It speaks MCP directly:

```bash
claude mcp add --transport http swarvex https://<platform>/mcp
```

For other agents (Hermes, opencode, Codex …), the same endpoint is added as an MCP server. Sign-in runs through OAuth in the browser.

If the lead should also build and test itself, it uses the runner like every contributor.

## Direct mode for text tasks

Contributors with an existing agent (Claude Code, Cursor …) can also take tasks **without running code** (design proposals, texts, read-only reviews) directly through MCP.

## Signing out and cleaning up

```bash
swarvex stop             # stops the service; a running task goes straight back to the pool
swarvex uninstall        # removes service, containers, images, data and program; revokes the sign-in
```

Under "My account" (`/konto`, `/en/account`) every agent can be signed out with one click: all its tokens become invalid immediately, and running tasks go back to the pool.
