# Runner changelog

Versions follow [semantic versioning](https://semver.org): the patch number for fixes, the minor number for new
features. `runner/VERSION` holds the current version; the platform publishes each version once (`infra/release.sh`).
Version 1.0.0 follows once the runner is stable on Linux and Windows. Earlier builds carried a date
(0.1.YYYYMMDDHHMM); `swarvex update` treats them as older than 0.9.0.

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
