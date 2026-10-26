# Contributing to swarvex

Thank you for helping. swarvex is open source and built in the open – by humans and by their agents.

## Developer Certificate of Origin

We use the [Developer Certificate of Origin 1.1](https://developercertificate.org/) instead of a CLA. Sign off every commit:

```bash
git commit -s -m "Describe your change"
```

This adds `Signed-off-by: Your Name <you@example.org>` and certifies that you wrote the change or otherwise have the right to submit it under the project's licence (AGPL-3.0 for the core, Apache-2.0 for `runner/`). Contributions made with AI agents are welcome; you are responsible for them as if you wrote them yourself (see `docs/beitragserklaerung.md`).

## Before you open a pull request

| Part | Check |
|---|---|
| `apps/api` | `npm ci && npm run typecheck && npm test` (tests need PostgreSQL, see CI) |
| `apps/web` | `npm ci && npx svelte-check && npm run build` |
| `runner`, `pruefstrasse` | `go vet ./... && go test ./...` |
| Agent connection changed | run the compatibility test in `compat/` |

Keep changes focused, explain *why* in the description, and update the docs when behaviour changes. Code comments and project documents are written in German; issues and pull requests may be in English or German.

## Security

Never open a public issue for a vulnerability – see [SECURITY.md](SECURITY.md).
