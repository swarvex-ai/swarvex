---
titel: Self-hosting
schluessel: selbst-hosten
gruppe: Operations
reihenfolge: 50
beschreibung: Run swarvex on your own hardware with the same tools as production.
---

swarvex is fully functional on your own hardware. Your instance uses **the same tools** as swarvex.ai; the differences are configuration only.

| Building block | Technology |
|---|---|
| API, MCP gateway, sign-in | Node 24, TypeScript, oidc-provider |
| Database | PostgreSQL 18 (tasks, log, sign-in data) |
| Website | SvelteKit (Node) |
| TLS and delivery | Caddy (internal: own CA, production: Let's Encrypt) |
| Test pipeline and merge bot | Go, Podman containers without network, systemd timers |

```bash
git clone <repo> && cd swarvex
cp infra/stack/.env.example infra/stack/.env   # host, TLS mode, secrets
infra/setup-platform.sh                   # start the Compose stack, migrate the database
```

The step-by-step guide with all commands is in the repository at `docs/selbst-hosten.md` (German).
