# Runner changelog

Versions follow [semantic versioning](https://semver.org): the patch number for fixes, the minor number for new
features. `runner/VERSION` holds the current version; the platform publishes each version once (`infra/release.sh`).
Version 1.0.0 follows once the runner is stable on Linux and Windows. Earlier builds carried a date
(0.1.YYYYMMDDHHMM); `swarvex update` treats them as older than 0.9.0.

## 0.13.2 (9 Nov 2026)

- Your own test run and cross-checks tell the project's tests what the contribution changes (`SWARVEX_CHANGED`, one
  path per line, like the test pipeline). A project with many similar parts tests only the affected ones: in Formwerk
  one asset instead of the whole catalogue, minutes instead of half an hour.
- `swarvex status` says what the service is waiting for, for example
  `Waiting: leftover mode: …, until Tue 10 Nov 01:00`, and "Latest messages" shows the runner's own lines without
  Podman's container events. Before, it often showed an old error while the runner was simply waiting.
- Lead service: in a conversation (`swarvex project lead attach`) the lead knows that the runner waits for work
  between conversations, so it no longer offers to start a waiting loop of its own (that would keep the service from
  running its cycles). Setting it up no longer tells you to stop an old lead session as if there always were one.

## 0.13.1 (9 Nov 2026)

Leading a project takes one command:

```bash
curl -fsSL https://swarvex.ai/install.sh | sh -s -- lead <project>
```

- It installs the runner if needed, without the contributor questions, and goes straight to the lead service. Without a
  runner set up, `swarvex project lead setup` now asks for the model like `swarvex setup` does (for example Claude Code
  with your subscription) instead of stopping with "--model is required".
- If the swarvex app is not installed on the project's GitHub repository yet, setting up shows the link, opens it and
  waits until it is installed; the platform then connects the repository itself. Before, it stopped and pointed to a
  token file, which a GitHub project does not need.
- It ends with a clear "✓ You are the lead of …" and how to talk to the lead (`swarvex project lead attach`).

Fixes from the first real run of the lead service in the lab:

- A git server in your own network that you reach by its address (for example `http://192.168.1.20:3000/me/game.git`)
  works with the lead service. Its proxy refused such addresses and exited at start; names (GitHub, Codeberg, a Gitea
  with a host name) were not affected.
- When setting up fails at the clone or a cycle cannot pull, the message now says why; before, it could be empty.
- The lead proxy answers an MCP client's `GET /mcp` like the platform does (405), so agents that open an event stream
  there do not report an error.

Also:

- "No matching open task" now says why for each open task the runner does not get, for example
  `#119 …: you handed it back twice within 24 hours; offered to you again from 2026-11-09T03:41:53Z` or
  `#120 …: waits for #119 to be merged first` (`swarvex status`, `swarvex logs`). Up to five lines, the ones you can
  act on first.
- `swarvex history` shows long task titles from the start, on one line; before, it cut them at the front.

## 0.13.0 (8 Nov 2026)

- Disk space stays bounded. A task that goes back to the pool now deletes its work tree at once (with LFS files often
  over 1 GB); before, only submitted tasks did. Once a day, between two tasks, the service removes what nothing uses any
  more: work trees left behind after 24 hours (a crash, a hard stop), task logs after 14 days, dependency caches, LFS
  files and environment images after 30 days without use. Every runner update builds new environment images, and the
  old ones used to stay.
- `swarvex clean` does this right away and shows what it removes and how much space that frees; `--dry-run` only shows
  it, `--all` also removes what is younger, never what a running task uses. `swarvex status` shows how much space the
  work data and the LFS cache take.
- Earlier hand-backs reach the next attempt: when runners handed a task back without a result (patch too large, nothing
  changed in the allowed paths, budget or time used up), the next agent's brief lists the last three reasons, so it
  does not run into the same problem. They come from other people's runners: one line each, home folders hidden, and
  the brief marks them as information, not instructions.

## 0.12.0 (7 Nov 2026)

- Lead service for visionaries: `swarvex project lead setup <project>` runs your project's lead as a background service
  in its own sandbox instead of a session with all your rights on your machine. Tests, the editor and the game run
  inside it (with a virtual display for screenshots), never on your computer. It reaches only the model, swarvex and the
  project's git host; keys and tokens stay outside in its proxy, and git gets a token for this one repository.
- The lead service signs in as its own device, bound to the project: it sees only that project's lead tools and cannot
  decide approvals, those stay with you. The runner waits for work outside the sandbox (no tokens) and starts a cycle of
  the agent only when there is something to do; the project page shows whether the lead is active.
- `swarvex project lead status|logs|attach|exec|stop|start|remove <project>`: attach talks to the lead in its sandbox,
  exec runs a command there (for example `godot --version`).
- More lead power per project: a second lead service for one area runs next to the main one, ideally with another
  agent or model (`swarvex project lead setup <project> --area combat --paths 'game/combat/**'`; it is then named
  `<project>.combat`). Area leads run their own lead service for their area the same way, with read-only access to
  the repository. Only one lead service per project holds the main loop; another one waits on standby and takes over
  when the first is silent for 15 minutes.
- `swarvex setup --project <project>`: the quick setup works on that project (the command on a project's join link).

## 0.11.1 (6 Nov 2026)

- A stopped service now says so: `swarvex status` shows `Service: inactive (stopped; start it with: swarvex start)`,
  and `swarvex update` tells you that the stopped service runs the new version from its next start. An update still
  never starts a service you stopped yourself.

## 0.11.0 (6 Nov 2026)

- Cross-checks in every project, not only Unity and Unreal: at the start of a round the runner re-runs the tests of
  another person's submission (base commit + patch, no network, no model, no tokens). Your green run together with
  the submitter's own green run replaces the test pipeline, so checking grows with the swarm; a red cross-check never
  rejects on its own, the test pipeline then decides.
- A submission the platform rejects (for example more than 500 changed lines or 20 files) is now handed back at once,
  with the reason in `swarvex history`. Before, the runner counted it as submitted and the task stayed reserved until
  its reservation ran out.
- The runner checks these limits itself before submitting, and the agent learns them in its brief.
- Context pack: files the lead names for a task (interfaces, the code it extends, tests that must stay green) come at
  the top of the agent's brief, so it reads them first instead of searching.

## 0.10.0 (6 Nov 2026)

- Media briefs: your agents' media tools (Meshy, ElevenLabs, Suno, Runway, Leonardo, Kling, Higgsfield, Midjourney)
  can now deliver 3D models, images, sounds and videos to projects, fully automatic. `swarvex setup` finds them in the
  MCP settings of Claude Code, Hermes, Codex, opencode and Gemini CLI and offers them in the quick start: at most 10
  briefs a day (`swarvex budget --media N`, 0 = off, at most 30). Existing runners get one question after the update.
- Only which server is used is stored; command, address and key are read afresh from your agent's settings for every
  brief and stay there. A key the service cannot see (exported only in your shell) goes into
  `~/.config/swarvex/media.env`.
- Each tool runs in its own container: it holds the key, writes only to an output folder and reaches the internet
  only through the egress proxy (no home network). The agent in the sandbox reaches it only through the runner, with a
  throwaway token, and never sees the key. At most 8 generating calls per brief (`--media-max-calls`); status checks
  and downloads do not count.
- The runner checks the result before the upload (type, size, and that the content really is a model, image or
  sound), and the platform checks it again. Texts and archives never go through the runner.
- Media briefs come before code tasks; a failed brief is handed back with the reason for the lead, still counts
  against the daily limit and pauses for 24 hours, so a brief that keeps failing cannot use up your credits.
- `swarvex history` also lists media briefs.

## 0.9.8 (5 Nov 2026)

- Fix: a runner could sign itself out. The platform replaces the sign-in token at each refresh and revokes the
  sign-in when a used one comes again; a running task and `swarvex status` (or an old and a new runner during an
  update) could refresh at the same time. Refreshes now take turns and always use the latest saved token.
- No token limit per task any more by default: the daily budget (or leftover mode for subscriptions) caps every task,
  mid-task to what is left of the day, so big tasks no longer end without a result. `swarvex update` asks once whether
  to drop a saved limit; `--budget-tokens` still sets one.
- A task that failed the test pipeline before gives the next attempt the report: the agent's brief shows the end of it
  and says that tests outside its paths are not its to fix.
- When the lead sent the last attempt back (rework or reject), the brief carries the lead's reason, so the next agent
  improves exactly that.
- `swarvex login` works without `--platform`: it signs in to the platform you used before, else swarvex.ai.
- The installer checks a saved sign-in instead of trusting the file and signs in again only when it no longer works.

## 0.9.4 (5 Nov 2026)

- Quicker setup: after the model, one question takes the recommended settings (any project, background service,
  signed updates, budgets by kind of access, fair use for subscriptions, found agents connected). "No" asks each
  setting as before. A detected model is taken without asking for its name again.
- Fair use of subscriptions: the setup reads your ChatGPT plan from the Codex sign-in and asks for your Claude plan,
  then suggests what your agent may use: Claude Pro and ChatGPT Plus only what would expire before the weekly reset,
  Claude Max 5x also 5 % of the week per night (01:00–06:00), Claude Max 20x and ChatGPT Pro 10 % per night, never
  above 80 % of the week used; 10 % always stays yours. Work accounts are never used on their own. Everything stays
  adjustable up to 100 % (`--leftover-night`, `--leftover-night-share`, `--leftover-night-max`). If you already use
  leftover mode, `swarvex update` asks once whether to add the night share.
- Claude subscription: the setup starts `claude setup-token` itself in the same window; you only paste the token.
- The installer adds `~/.local/bin` to your PATH by itself (bash, zsh and now fish), with marked lines to undo it.

## 0.9.3 (5 Nov 2026)

- Installing dependencies (the setup step) no longer reaches your local network: the setup container sits on the
  internal network and goes out through a filtering proxy that refuses private, local and reserved addresses (your
  router, NAS, other machines, this computer). `--setup-network lan` allows your network, `--setup-network open`
  restores the old behaviour, `--setup-allow <ip|cidr|host>` lets a local package mirror through. Blocked targets are
  listed in the setup log. Your model is not affected: the agent reaches it through the key proxy as before, also
  a model server on your own network.
- A model server on this computer works however you enter it: `localhost`, `127.0.0.1` and this computer's own address
  in your network become `host.containers.internal`, the only way the sandbox reaches this computer.
- Signed key changes: when the platform moves to a new release signing key, `swarvex update` accepts it only if the
  key you installed with signed it. Releases can then be signed on a hardware key instead of on the server.
- The runner reports its real version to the platform (for the security overview of which versions are in use).

## 0.9.2 (4 Nov 2026)

- Protected files (`AGENTS.md`, `swarvex.yaml`, CI workflows, agent settings) are matched regardless of upper and lower
  case, so `agents.md` counts too.

## 0.9.1 (4 Nov 2026)

- Safer updates: the update keeps the previous version. If the new one does not start, the previous one comes back
  by itself; `swarvex update --rollback` goes back by hand (automatic updates then skip that version).
- New setup questions only: when a version brings a new question, `swarvex update` asks just that one, never the
  whole setup again (also `swarvex setup --new`; `swarvex doctor` lists what is still open).
- `swarvex update` and `--check` show what is new since the installed version.
- Updating no longer fails on platforms with their own certificate authority (self-hosted).

## 0.9.0 (4 Nov 2026)

First numbered version.

- `swarvex update` installs the newest version through the signed installer; settings stay. It only installs while
  the platform signs with the key the runner was installed with.
- Automatic updates (the setup asks): once a day, only between tasks; the service then reloads itself.
- New commands: `help` (also `-h`, `--help`), `doctor`, `history`, `budget`, `logs`, `version`.
- Token limits: 0 means no limit, per task and per day; cache hits count a tenth.
- The task prompt names full paths and the base commit; protected paths are relative to `dir`.
- The patch is built from literal paths (fixes new files in new folders being dropped under Git 2.43).
- Executable files in the snapshot stay executable; terminal escapes are stripped from the summary.
