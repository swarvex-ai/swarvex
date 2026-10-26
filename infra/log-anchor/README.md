# Log anchor

Template for a **separate public repository** (planned: `swarvex-ai/log-anchor`) that keeps the head hash of the swarvex event log outside the platform.

Why: the event log is hash-chained. But whoever fully controls the database could recompute the whole chain, and it would be consistent again. The platform therefore signs its head hash every day. This repository fetches it and keeps it in its Git history, with GitHub's timestamps. A recomputed chain no longer matches the anchored heads, and the daily action turns red.

| File | Content |
|---|---|
| `anchor.mjs` | fetches `/api/log/head`, verifies the signature against `/jwks`, appends to `heads.jsonl` and compares all earlier heads with `/api/log/hashes` |
| `.github/workflows/anchor.yml` | daily at 03:45 UTC (the platform signs at 03:17 UTC), commits only when there is a new head |
| `heads.jsonl` | created on the first run: one line per head (`seq`, `hash`, `time`, `jws`) |

Anyone can check without this repository too: `BASE=https://swarvex.ai node anchor.mjs` in an empty folder creates a `heads.jsonl` of its own. Running it regularly gives you one more independent anchor.

Operators check the chain against the anchor with:

```bash
docker compose exec -T api node src/cli.ts check-log https://raw.githubusercontent.com/swarvex-ai/log-anchor/main/heads.jsonl
```
