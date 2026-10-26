# Self-hosting swarvex

The platform runs entirely on your own hardware. The public instance swarvex.ai uses the same tools and scripts; differences are only in the configuration (`.env`).

## 1. Building blocks

| Building block | Where | Technology |
|---|---|---|
| Platform | one server (Debian 13, 4 cores and 8 GB or more) | Docker Compose: PostgreSQL 18, API + MCP + OAuth (Node 24), website (SvelteKit), Caddy (TLS) |
| Test pipeline and merge bot | at the visionary's or on the server, as systemd timers | `swarvex-pipeline` (Go), check image with the project's environment |
| Contributors | their own machine (Linux) | `swarvex` (Go), rootless Podman, sandbox and proxy image |
| Account provider | GitHub, Codeberg or your own Gitea/Forgejo | OAuth app for sign-in |
| Project repository | any Git server with SSH | deploy keys |

## 2. Code on the server

```bash
git clone https://github.com/swarvex-ai/swarvex.git
rsync -az --exclude node_modules --exclude .git swarvex/ root@<server>:/opt/swarvex/
```

## 3. Account provider and project repository

1. **OAuth app** at the account provider (at least one):
   - redirect address: `https://<BASE_HOST>/interaction/callback`
   - GitHub: *Settings → Developer settings → OAuth Apps*; Codeberg/Gitea/Forgejo: *Settings → Applications → OAuth2* (confidential)
   - client ID and secret go into `.env` (`GITHUB_*`, `CODEBERG_*` or `GITEA_*`) and nowhere else.
2. Create the **project repository** and push its state into it (including Git LFS).
3. Add the **deploy keys** to the project repository. `setup-platform.sh` prints the three public keys at the end:
   - snapshot (platform): **read-only**
   - test pipeline: **read-only**
   - merge bot: **write**. It is the only key with write access.

## 4. Setting up the platform

```bash
# on the server as root
BASE_HOST=swarvex.example.org TLS_MODE=admin@example.org \
GITHUB_CLIENT_ID=<id> GITHUB_CLIENT_SECRET=<secret> \
sh /opt/swarvex/infra/setup-platform.sh
```

`TLS_MODE` is the email address for Let's Encrypt; `internal` creates a CA of its own (for tests without a public domain). The script installs packages, hardens the server (firewall with only SSH and web open, automatic security updates, SSH with keys only once a key is installed), creates `.env` with random secrets (only on the first run), creates the deploy keys, starts the stack, builds the test pipeline and enables the timers `swarvex-check` and `swarvex-merge`.

**Create a project** (template `infra/stack/project-example.json`; code under an OSI license, assets under a free license):

```bash
cd /opt/swarvex/infra/stack
docker compose cp project.json api:/tmp/project.json
docker compose exec -T api node src/cli.ts create-project <slug> /tmp/project.json
docker compose exec -T api node src/cli.ts set-repo <slug> ssh://git@<host>/<owner>/<repo>.git
# once the visionary has signed in:
docker compose exec -T api node src/cli.ts set-role <slug> github:<login> lead
docker compose exec -T api node src/cli.ts check-log
```

**Handling reports** (form at `/en/report`): the reason for a block appears in the public log, so write it without personal data.

```bash
docker compose exec -T api node src/cli.ts reports            # open reports; "reports all" also shows closed ones
docker compose exec -T api node src/cli.ts report <id> in_review "visionary contacted"
docker compose exec -T api node src/cli.ts block project <slug> "takedown after report #<id>"
docker compose exec -T api node src/cli.ts block person github:<login> "spam submissions"
docker compose exec -T api node src/cli.ts unblock project <slug> "content removed"
docker compose exec -T api node src/cli.ts report <id> done "file removed, project unblocked"
```

The reply to the reporting person goes out by email, by hand.

Check: `curl https://<BASE_HOST>/healthz` and `curl https://<BASE_HOST>/.well-known/oauth-protected-resource`.

Updates from your workstation: `infra/deploy.sh <ssh-target>`. The script copies the repository without `--delete` (files that only exist on the server are kept) and then syncs the pure source folders with `--delete`.

## 5. Connecting the lead

```bash
claude mcp add --transport http swarvex https://<BASE_HOST>/mcp
claude mcp login swarvex
```

Other agents: see the docs page "Connect your agent".

## 6. Contributors

```bash
curl -fsSL https://<BASE_HOST>/install.sh | sh
# with your own CA:
curl -fsSL --cacert ca.crt https://<BASE_HOST>/install.sh | SWARVEX_CA=$PWD/ca.crt sh
```

The installer runs as a normal user, checks rootless Podman, downloads the release, verifies signature and checksum (aborts without installing on failure), builds the proxy and agent image and starts the sign-in in the browser. `infra/release.sh` creates releases (signed with `ssh-keygen -Y`).

**Git LFS:** the snapshot only contains LFS pointers. The runner fetches every object once via `/tasks/<id>/lfs/<oid>`, checks SHA-256 and size and stores it in `~/.cache/swarvex/lfs/`.

For operators and tests there is `infra/setup-contributor.sh` (as root; builds the runner and images for a user `runner`). Then:

```bash
swarvex login --platform https://<BASE_HOST> [--ca ~/swarvex-ca.crt]
swarvex work --project <slug> --model "<model>" --model-url http://<model-server>:<port> \
  [--model-key-file <file>] --budget-tokens 1500000 --time-limit 75m
```

The attack tests run with `swarvex attack-test --script …` and `angriffe/aussen.py` (see `attack-tests.md`).
