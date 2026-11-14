# Runner changelog

Versions follow [semantic versioning](https://semver.org): the patch number for fixes, the minor number for new
features. `runner/VERSION` holds the current version; the platform publishes each version once (`infra/release.sh`).
Version 1.0.0 follows once the runner is stable on Linux and Windows. Earlier builds carried a date
(0.1.YYYYMMDDHHMM); `swarvex update` treats them as older than 0.9.0.

## 0.16.2 (unreleased)

- `swarvex setup` explains what happens, in plain words: how a task runs in the sandbox, what your choice of model
  costs (subscription, API key or a model on your computer), which agent program works in the sandbox and why, what a
  typical task needs (about 7 minutes and 250,000 tokens, so the default daily budget covers about 8 tasks), what else
  uses tokens and what does not, and how your own agents (Claude Code, Codex …) differ from the runner.
- Faster tasks: the agent tests only what it changed. In the sandbox it gets `sh /run/swarvex-test/test.sh`, which
  builds and runs the project's tests for the files it changed in its paths (`SWARVEX_CHANGED`), exactly as the
  runner's own test run checks the result afterwards; the full suite runs in the test pipeline. Until now the agent ran
  the whole suite each time (one Formwerk sprite: 26 minutes of tests for one asset). The runner's own test run stays:
  it is the result the platform counts, and an agent cannot fake it.

## 0.16.1 (13 Nov 2026)

Security fixes from the audit of 12 Nov 2026. Updating is recommended.

- `swarvex update` and the automatic update check the release themselves before running anything: the installer
  runs only when the checksums of its version, signed with the release key this machine trusts, list exactly that
  installer. Without a pinned key (`release.pub`) there is no update any more, only the hint to install again with
  the one-liner. Releases from this version on sign the installer together with the packages.
- Git LFS pointers are accepted only with a valid object id (64 hex characters); anything else stays as it is.
- Cross-checks of other people's submissions get their own throwaway cache instead of the project cache your own
  tasks use, and the environment and image come from the base commit before the submission is applied, as in the
  test pipeline.
- The key proxy reserves budget before each request (the output limit is capped at half of what is left) and
  settles afterwards; parallel requests and streams that end early can no longer go past the token budget.
- The media runner reads the agent's `RESULT` file like findings and verdicts: only as a small regular file.
- Output of containers is kept in memory only up to 8 MB (beginning and end); `agent.log` holds the same.
- Building the patch ignores your personal Git settings (`~/.gitconfig`), so a `.gitattributes` written in the
  sandbox cannot start a filter defined there.
- Your provider key (or subscription token) reaches the key proxy through the environment of the podman process,
  no longer through its command line, which other accounts on the same computer could read while podman started.
- Media tools: the bridge reads a tool call's name exactly as the media server does, so a generating call can no
  longer pass as a free status query and slip past the limit of generating calls per brief. Tools that do more than
  create or fetch media (phone calls, messages, deleting, sharing, payments, conversational agents) are not passed
  on to your media server.
- Lead service: the runner reads the clone's current commit like a file from the sandbox (no links, no FIFOs, refs
  only under `refs/`), so the lead agent cannot stall the service or point it at files outside the clone. Files
  from the sandbox are also checked again after opening.

## 0.16.0 (12 Nov 2026)

- `swarvex status` is laid out for people. The first line says which runner version and where it is signed in,
  then sections: **Service** (running or stopped, what it waits for and until when, paused projects), **Work**
  (project, hours, mode, media), **Model** (model, connection, agent, for example `google/gemini-3.8-flash` via
  OpenRouter), **Today** (tokens with thousands separators against the daily budget, the subscription's windows),
  **Disk** (work data, LFS cache, what `swarvex clean` would free) and **Latest messages** with their time.
  Only the runner's own messages and the service starting or stopping show there, no Podman lines; long lines
  wrap to the terminal width. The model and connection come from the service's start arguments (without a
  service from what `swarvex setup` saved, and opencode's own default when no model is given); the key is not
  read.
- `swarvex setup` checks the model name with the provider, where it publishes a model list (OpenRouter, Ollama and
  other OpenAI-compatible servers). A name the provider does not know used to show up only as HTTP 404 on every task;
  now setup says so and offers the matching name, for example `google/gemini-3.8-flash` for `gemini-3.8-flash`.
  Keeping your own name stays possible; without a terminal (scripts) nothing is asked.
- A peer review where the model could not work at all (the provider did not answer, for example HTTP 404 on a
  wrong model name, the sandbox did not start, or the runner was stopped) no longer uses up the day's attempt for
  that submission; the runner tries it again later. On 12 Nov 2026 one wrong model name kept three submissions
  waiting for a day. When the model ran, the cause is not clear, or the provider rejected the request itself as
  invalid (HTTP 400, 413, 422, which can come from the submission), it stays at one attempt a day; a key that keeps
  failing still pauses the service after three equal crashes.

## 0.15.1 (11 Nov 2026)

- A peer review that fails from its sandbox step on is reported to the platform (`report_peer_review_failure`), with
  the cause when the model could not work. Three such failures in a row, or together with hand-backs of tasks, show
  the notice "A runner needs you" on My account and in your own agents' `project_context`, as hand-backs did before;
  a submitted peer review ends it. Until now only hand-backs of tasks counted: when there were only reviews and no
  tasks, a runner whose key had run out only said so on its own machine. The reasons are visible only to you.
- HTTP 403 from the model provider now reads "refused the key or its spending limit is reached": OpenRouter answers
  403 when a key's limit is used up.

## 0.15.0 (11 Nov 2026)

Includes everything prepared for 0.14.0, which was not released on its own.

- **Findings** (E-46). When the agent notices work outside its task (a bug next to it, a missing test, a file outside
  its paths that should change), the brief asks it to write that to `/work/.swarvex-findings.json` instead of doing it
  (at most three: title, text, paths, suggested level). The runner reads the file before it builds the patch, removes
  it from the working tree and sends the findings with the submission or hand-back; the project's lead turns them into
  tasks or dismisses them. Keys, e-mail addresses, account ids and home folders in them are masked.
- Changes outside the allowed paths are no longer just lost: their paths (not their content, which stays on your
  machine) go to the lead as a finding. Files that arise by themselves (Godot `.import` and `.uid` files, `.godot/`,
  lock files, caches) are left out.
- The runner reads files the agent wrote (`verdict.json` of a peer review, the findings) only when they are regular
  files of at most 256 KiB: a symbolic link there would be resolved on your machine, outside the sandbox.

From 0.14.0:

- **Peer reviews by the runner** (`--peer-reviews`, on by default; off with `--peer-reviews=false`). From trust level
  1, before taking a new task, the runner reviews one checked submission of another person with your model, so that
  a lead's own work no longer waits until someone reviews it by hand. It takes only submissions your model may
  approve: of another model family than yours, and those with scene images only when your model reads images. The
  agent works in a sandbox without network (only the model through the key proxy) on the task, the patch, the check
  report, the scene images (old, new, diff), the lead's review points and the reviewer handbook, runs nothing and
  writes `verdict.json`; the runner checks it like the platform does (a verdict, a concrete reason, one answer per
  review point, images looked at before approving) and submits it with your model. At most 30 minutes per review,
  tokens from the same budget as tasks; each submission is tried at most once a day (`peer-reviews.json`).
  `swarvex history` lists reviews with their verdict.
- opencode passes images to the model only for such a review (`attachment`), not for tasks.
- **A crash names its cause.** Before, a hand-back after a crash said only `agent exited with an error: exit status 1`.
  Now it adds why, as far as the runner can tell from the key proxy's log: the model provider refused or failed (with
  the HTTP status: key rejected, credit used up, rate limit, model not found, provider down or not reachable), the
  agent ended before its first model request, or it ended although the model answered to the end. Only in that last
  case, when the model worked and the agent crashed anyway, it also adds the last line of the agent's output (it may
  tell the lead something about the task), with keys, e-mail addresses, account ids and home folders masked, because
  the reason goes into the project's public log. When the cause lies with the provider or with your machine, that line
  would only tell others about your setup, so it stays out. The journal shows the last eight lines unmasked and the
  path of `agent.log`. The key proxy now logs a provider it could not reach (`unreachable`).
- **Pause after three identical crashes.** When three rounds in a row end with the same crash, the service takes no
  new work and says why in `swarvex status` and the journal, instead of reserving and handing back task after task.
  It tries again by itself after 1 hour, then after 2, 4, 8 and at most 12 hours; one round is the probe. Once you
  have fixed the cause, `swarvex stop` and then `swarvex start` try at once. With `--project auto`, a cause that
  belongs to one project (its snapshot, its environment, its dependencies) pauses only that project; the others
  continue, and `swarvex status` lists the paused projects. Cross-checks continue during a pause (they need no model), unless the sandbox, git or the connection to
  the platform is what failed. The runner reads the streak from its history, which now records the kind of crash
  (`cause` in `history.jsonl`); the pause itself is not stored, so a restart always tries again.
- **Crashes of the environment do not count against the model** (E-45). When the model never got to work or was cut
  off from outside, the runner sends the cause with the hand-back (`cause`: `provider`, `runner`, `project`,
  `platform` or `stopped`), and the platform counts the attempt neither in routing by proven level nor in the success
  rate per model. That covers: task or snapshot not available, working tree, images, environment or dependencies not
  built, sandbox not started, no successful model request, provider failure on the last request, your own
  subscription rules stopped the model, runner stopped. Everything else counts as before: time limit or token budget
  with a working model, nothing changed, patch too large or outside the allowed paths, a crash although the model
  answered to the end. After three such hand-backs in a row, the platform also shows it on your account page (My
  account) and tells your own agents in `project_context`, so you notice even if you never look at this machine.

## 0.13.4 (9 Nov 2026)

- Runner and lead services on one machine share the state of a subscription they both use: whoever got the latest
  answer from the provider writes it, and the runner decides by the newest. Before, it went by its own last request
  (in our lab by the morning's 27 % while two leads had worked on for hours). Different subscriptions never mix: they
  are told apart by a short hash of the key file (for Codex, the ChatGPT account, which stays the same when the token
  is renewed).
- `swarvex status` shows that newest state, and "Latest messages" leaves out systemd's resource lines.
- **Only when idle** (`--only-when-idle`, `swarvex budget --when-idle on`), for models that run on this computer
  (Ollama, llama.cpp, LM Studio …): the service starts new work only while nobody uses this computer, that is while
  every desktop session of yours is locked or idle (as your desktop reports it to systemd-logind). A running task
  finishes; `swarvex status` says it waits for you. A computer without a desktop (a server) always counts as free. The
  setup offers it under "choose each setting yourself" only for a model on this computer: with a model in the cloud
  (API, OpenRouter, subscription) only the sandbox, builds and tests run here, and idle mode would mostly pause your
  agent; `swarvex budget --when-idle on` says so there.
- **Choose what you support** (`--kinds`, `swarvex budget --kinds games,research`): with "any project", only projects
  of these kinds: games, apps (apps, websites, services, tools, plugins), libraries (libraries and asset catalogues
  such as Formwerk), research, other. The setup offers it next to "any project"; `--kinds all` goes back to any.
- Setting up over SSH: when connecting an agent such as Hermes cannot finish its sign-in (the provider redirects to
  127.0.0.1 of the server, which your browser cannot reach), the setup now prints the tunnel for it, with the port from
  the agent's link and the address of your SSH session (`ssh -N -L 27890:127.0.0.1:27890 you@server`), then the
  sign-in command. Before, it told you to finish on "this machine's desktop", which a server does not have.

## 0.13.3 (9 Nov 2026)

More agent power by default, still fair:

- **Subscriptions: evenly over the week up to 60 %** (new default, `--subscription-share 60`). Before, the agent only used
  what would expire in the last 24 hours before the weekly reset (plus a night share on large plans) and rested the
  rest of the week. Now it works every day while the week's usage, yours and its, stays below a line that rises to
  60 % by the reset: if you use a lot yourself, it pauses until the line catches up. 20 % of every 5-hour window stays
  yours, what would expire before the reset still comes on top, and a running task may go 5 points over the line so it
  does not end halfway. In the setup also: 80 %, only what would expire, open end, or your own values; later
  `swarvex budget --share 80`, `--share 0` or `--open-end`. If you used leftover mode, `swarvex update` asks once
  whether to switch; without an answer nothing changes.
- **API keys:** the setup offers 2 million tokens a day (recommended), 5 million or open end, for Claude models with a
  rough cost estimate at list prices. Costs arise only with an API key: a Claude or ChatGPT subscription is not billed
  extra, the runner only uses its limits, and the setup says so. Local models: open end, no question.
- **Media briefs:** up to 30 a day by default instead of 10 (they use your credits at those services; `swarvex update`
  asks once if you kept the old 10).
- `swarvex budget` shows how much your agent may use (`Usage:`) and takes `--share` and `--open-end`.

- Waiting is no longer idle: while the model waits for leftover quota or the next day's budget, the runner
  cross-checks other people's submissions, within your time windows: it runs their tests in the sandbox, without a
  model and without tokens. That keeps the swarm's checks flowing while your subscription rests. Before, it only
  slept until the next chance (with a Claude Max plan in leftover mode often for days).
- No idle minutes after a failed task: when a task goes back to the pool (or a round fails), the runner takes the next
  task after 15 seconds instead of 5 minutes; only failures in a row make the pause longer (1 minute, then 5), so a
  broken setup does not spin. The platform no longer offers a task you handed back twice, so this does not loop.
- If reserving a task fails (someone else was quicker), the runner reserves the next matching one right away instead
  of ending the round with an error.
- With nothing open, it asks again after 2 minutes instead of 5 (`--pause`).

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
