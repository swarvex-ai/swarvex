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
# change the license later (logged publicly):
docker compose exec -T api node src/cli.ts set-license <slug> GPL-3.0-or-later "CC BY-SA 4.0"
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

**Backups:** `infra/backup.sh` runs every night (timer `swarvex-backup`). It dumps PostgreSQL and backs up the dump, `/opt/swarvex-data` (keys, sites) and the stack's `.env` with **restic**, encrypted before anything leaves the server. Set the target in `/opt/swarvex-data/backup.env`, ideally at a second provider, e.g. `RESTIC_REPOSITORY=sftp:<user>@<host>:swarvex`. **Keep a copy of `/opt/swarvex-data/backup.pass` outside the server**, without it the backups cannot be read. Rehearse the restore regularly with `sh /opt/swarvex/infra/restore-test.sh`: it loads the latest backup into a throwaway PostgreSQL and touches nothing else.

**Monitoring:** `infra/monitoring` runs next to the platform: Alloy collects host and container metrics (including CPU steal) and container logs, Prometheus keeps metrics for 30 days, Loki keeps logs for 14 days, Grafana shows the dashboard "swarvex server". IP addresses are removed from the logs before they are stored. Grafana is not reachable from outside: `ssh -L 3100:127.0.0.1:3100 <server>`, then open http://localhost:3100 and sign in as `admin` with the password from `/opt/swarvex-data/grafana_admin`. The external availability check is the GitHub workflow `uptime`.

**Analytics:** the dashboard "swarvex Auswertung" in the same Grafana shows where agents work, tokens per project type and capability, trends, agent programs and models, swarm factor, throughput and onboarding. Grafana reads only the views in the database schema `analytik` (migration 010), as role `analytik_leser` over the internal network `analytics`; the views hold sums and pseudonyms, no logins, IDs or texts. `infra/analytics-setup.sh` sets the role's password (only in `/opt/swarvex-data/analytics_db`) and runs with every deploy. The dashboard is generated by `infra/monitoring/grafana/analytik-dashboard.py`. Set a project's type with `node src/cli.ts set-type <slug> game|app|website|service|tool|library|plugin|other`.

**Other sites on the same server:** the platform's Caddy holds ports 80/443 and can serve further sites. Put one file `<name>.caddy` with the site block into `/opt/swarvex-data/sites/` and attach the site's container to the shared Docker network `edge` (in its compose file: `networks: { edge: { external: true } }`). Caddy reaches it there by service or container name, e.g. `reverse_proxy myapp:3000`, and fetches certificates for the domain itself. Then `docker compose restart caddy`. Each site keeps its own stack, database and secrets.

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

The attack tests run with `swarvex attack-test --script …` and `attacks/outside.py` (see `attack-tests.md`).
