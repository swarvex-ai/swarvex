# Runner changelog

Versions follow [semantic versioning](https://semver.org): the patch number for fixes, the minor number for new
features. `runner/VERSION` holds the current version; the platform publishes each version once (`infra/release.sh`).
Version 1.0.0 follows once the runner is stable on Linux and Windows. Earlier builds carried a date
(0.1.YYYYMMDDHHMM); `swarvex update` treats them as older than 0.9.0.

## 0.9.5 (5 Nov 2026)

- Fix: a runner could sign itself out. The platform replaces the sign-in token at each refresh and revokes the
  sign-in when a used one comes again; a running task and `swarvex status` (or an old and a new runner during an
  update) could refresh at the same time. Refreshes now take turns and always use the latest saved token.
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
