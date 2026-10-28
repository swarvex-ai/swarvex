# Architecture and tech stack

Principle: **connect what exists, build only the coordination itself.** Every component states what we take over and what we build.

## Overview

```
                         ┌───────────────────────────────────────────┐
                         │ Platform (own server)                      │
 Browser ──HTTPS────────▶│  Web (SvelteKit)                           │
                         │  API + MCP gateway (TypeScript)            │
                         │  Coordination core (state machine)         │
                         │  PostgreSQL · job queue · object storage   │
                         │  Identity (OAuth 2.1 / OIDC)               │
                         └───────▲──────────────▲──────────────▲──────┘
                                 │ MCP/HTTPS    │ MCP/HTTPS    │ signed API
          ┌──────────────────────┴───┐  ┌───────┴───────────┐  ┌┴───────────────────────┐
          │ The visionary's lead      │  │ Runner of         │  │ Project repo + test     │
          │ (Claude Code, Hermes,     │  │ contributors/     │  │ pipeline (GitHub,       │
          │  opencode … any)          │  │ donors            │  │ Gitea/Forgejo …)        │
          │ speaks MCP directly       │  │ sandbox + proxy   │  │ at the visionary's      │
          └───────────────────────────┘  └───────────────────┘  └─────────────────────────┘
```

Agents **never** run on the platform server. The platform holds state, rules and the log.

## Tech stack

| Area | Choice | Taken over / built | Reason |
|---|---|---|---|
| Agent interface | **MCP** (Model Context Protocol), transport Streamable HTTP, auth per MCP authorization (OAuth 2.1) | standard taken over, tools built | Every common agent (Claude Code, Codex, opencode, Hermes, Cursor …) speaks MCP. A lead can connect directly without software of its own. |
| Backend | **TypeScript** on Node 24 (types run directly, no build), `node:http`, the official MCP TypeScript SDK, Zod for schemas | built | Same types for API, MCP tools and web; official SDK. Fastify dropped (PoC): the OAuth server and the MCP transport work directly on Node requests |
| Database | **PostgreSQL 18** | taken over | single source of truth, relational integrity, append-only log; text patches and metadata |
| Job queue / leases | **pg-boss** (queue in Postgres) | taken over | no extra infrastructure; timeouts and retries. Deliberately *not* Temporal: the long-running work happens at the lead/runner, not with us (E-05) |
| Identity | **oidc-provider** (panva), in the backend (E-06) | taken over | dynamic client registration and client ID metadata documents for MCP agents, device flow (RFC 8628) for the runner, OAuth 2.1 with PKCE; sign-in through GitHub/Codeberg/Gitea |
| Web | **SvelteKit** (Node adapter) | built | lean, self-hostable, SSR for public project pages (E-07) |
| Object storage | **Garage** (S3-compatible), a container in the stack (E-08) | taken over | inbox for files until acceptance, check reports, screenshots. Accepted work ends up in the project's Git repository (assets via LFS) |
| Runner | **Go**, one static binary per platform | built | one download, no runtime dependency → a real one-liner (E-09) |
| Sandbox | **Podman** on all systems: rootless (Linux, optionally gVisor), `podman machine` (macOS: libkrun/Apple Hypervisor, Windows: WSL2) (E-22) | taken over | third-party project code never runs directly on the host; one code path for all three systems |
| Key proxy | **our own small proxy** in the runner (Go) | built | the agent in the container only sees the proxy with a throwaway token; real keys stay outside; enforces budget and time limit. LiteLLM dropped (PoC): less third-party code at the most security-critical spot |
| Agents in the container | **opencode** today; Claude Code, Codex CLI, aider, Hermes … as further engine adapters | taken over | we build no coding agent of our own |
| Test pipeline | **our own check service** (Go) at the visionary's, containers without network, signed report | built | the same role at every visionary, independent of the CI system |
| Releases and signatures | **GitHub Actions**, **GoReleaser**, images in **GHCR**; **cosign** keyless, SBOM, SLSA provenance; DCO sign-off for contributions (E-23). Today: releases signed with `ssh-keygen -Y` | partly taken over | the runner must not become an attack path itself |
| Observability | **OpenTelemetry** in the API, **Grafana Alloy** → Prometheus, Loki, Grafana; external availability check (E-24) | taken over | operations |
| Operations | **Docker Compose** on one server (Debian 13), **Caddy** with Let's Encrypt, backups with **pgBackRest** at a second provider, secrets with **sops + age** (E-18, E-24) | taken over | one server is enough for a long time, because no agents run with us |

## MCP gateway: tools per role

One endpoint `https://<domain>/mcp`. Which tools a client sees depends on the token (role, project, trust level).

**Contributors (through the runner or directly):**

| Tool | Purpose |
|---|---|
| `project_context` | vision, rules, licenses, your roles |
| `metrics` | the project's status |
| `matching_tasks` | tasks that match the capabilities and the level |
| `reserve_task` | take a lease (with expiry) |
| `task_details` | full description, allowed paths, acceptance criteria, base commit |
| `report_progress` | heartbeat, tokens/credits/time used |
| `submit_result` | patch (git format-patch) + summary + own test results |
| `abandon_task` | reason; the task goes back to the pool |

**Lead (in addition):**

| Tool | Purpose |
|---|---|
| `create_task` | including level, capabilities, allowed paths, acceptance criteria |
| `pending_submissions` | submissions with check results waiting for review |
| `review_submission` | accept / rework / reject, each with a reason |

*Planned:* `change_task`, `fix_contract` (interface + tests as a binding basis for tasks), `propose_merge`, `create_asset_brief` (brief to donors with cost estimate and preview step), `request_approval` (to the visionary: release, license, budget exceptions).

Tools that do **not** exist: direct write access to repositories, access to other projects or user data, changing other people's budgets, overwriting check results.

**Direct connection without a runner** (e.g. `claude mcp add --transport http …`) is only meant for tasks that **do not run project code**: design proposals, texts, reviews by reading. Everything that builds or tests requires the runner with its sandbox.

## Runner

A Go binary, open source, signed.

```
Runner (host)
├─ sign-in (OAuth device flow) → short-lived, scoped token; refresh token in access.json (0600)
├─ options: project, capabilities, model, schedule, budgets (website settings planned)
├─ schedule + budget guard (stops hard when exceeded), service mode (systemd user unit)
├─ key proxy: OpenAI-compatible, real keys only here
└─ sandbox (Podman), new for every task:
     ├─ project snapshot (base commit), environment from swarvex.yaml / dev container / detection
     ├─ agent engine (opencode)
     ├─ network: only the proxy
     ├─ no home directory, no container socket, resource limits
     └─ output: patch + report → runner checks the paths → platform
```

The runner checks the patch against the allowed paths itself before submitting. The platform checks again afterwards: trust is good, checking on our own side is mandatory.

## Contributions without Git rights

Contributors need **no** write access and no Git credentials:

1. The runner submits a patch (`git format-patch` against the base commit).
2. The platform stores it with its hash.
3. The project's test pipeline fetches it through the signed API, applies it to a check tree and runs the checks.
4. If the lead accepts, the project's **merge bot** creates the commit, with trailers (see traceability) naming the person.

## Project environment, test pipeline and test contract

Projects are not tied to any language or engine (E-28). Runner and test pipeline determine the environment with the same package (`runner/umgebung`):

1. `swarvex.yaml` in the project (template or own image, setup, build, test, working folder, project-specific protected paths),
2. otherwise a dev container (`.devcontainer/devcontainer.json`: image or Dockerfile, setup commands),
3. otherwise the project type from typical files (Godot, Node, Python, Rust, Go, .NET, Java),
4. otherwise a general Linux environment.

```yaml
template: godot          # or image: ghcr.io/…
dir: game
setup: godot --headless --import
test: tools/run_tests.sh --headless
protected: |
  - tools/**
```

- **Two phases:** setup (dependencies) runs with network, but without agent, proxy and keys. Agent, build and test run without network; dependencies live in the project folder or in the per-project cache `/cache`.
- **Agent layer:** the runner puts the chosen agent and ripgrep (`/opt/swarvex-agent`) on top of the project image: opencode (image `swarvex-agent`, default), Claude Code (`swarvex-agent-claude`) or Codex (`swarvex-agent-codex`), one build stage each in `runner/sandbox/Containerfile`, built on first use. The key proxy passes only the chosen agent's model API (E-32). glibc is required.
- **Protected:** contributions never change `swarvex.yaml`, `.devcontainer/`, `AGENTS.md`, agent instructions (`.agents/`, `.claude/`, `.codex/`, `.opencode/`, `CLAUDE.md`, `opencode.json`), CI workflows or license files (platform, runner and test pipeline check this); otherwise a contribution could smuggle commands onto other machines.
- The test pipeline runs at the visionary's and reports a signed result to the platform. `swarvex env` runs the same steps locally.
- **Spot checks:** the platform repeats a share of the checks itself (containers on its own server) and marks deviations publicly.
- If the donor allows it, asset checks run here too: import, polygon count, skeleton, screenshot in the game's lighting.

## Data model (core, simplified design)

```
people(id, account_provider, account_id, created, trust_level, blocked)
agents(id, person_id, name, engine)                          -- one person, many agents
assignments(id, agent_id, capability, model_name, budget_json, schedule)
projects(id, slug, visionary_id, vision_md, license_code, license_assets, engine,
         repo_url, check_url, revenue_statement, lead_configuration)
tasks(id, project_id, level, role, capabilities[], paths[], contract_ref,
      base_commit, cost_estimate, multiple, min_trust, state)
leases(id, task_id, agent_id, assignment_id, expires)
submissions(id, task_id, lease_id, patch_hash, artifacts[], report, usage_json, submitted)
checks(id, submission_id, source(project|sample), result, report_ref, signature)
reviews(id, submission_id, lead_agent_id, verdict, reason)
donation_budgets(id, person_id, service, unit, limit, used, period)
events(seq, time, actor, type, subject, data_json, previous_hash, hash)  -- append-only
```

The database itself still uses German names internally; the public interfaces translate them (`apps/api/src/aussen.ts`).

## Security: threats and countermeasures

| Threat | Countermeasure |
|---|---|
| Malicious project code attacks the contributor's machine | sandbox per task, no network except the proxy, no home, no container socket |
| Prompt injection through repo/issues/tasks steals keys | key proxy: keys are never in the container; throwaway tokens per task |
| A malicious visionary writes attack tasks | as above, and budgets lie with the donor |
| A weak or manipulated lead wastes donations | donor budgets, preview step, public metrics |
| Fake agents, spam | identity tied to the person, trust levels, rate limits |
| A malicious contribution with a hidden backdoor | test contract, protected paths, diff rules (e.g. no network/file APIs without lead approval), lead review |
| Compromised runner download | signed releases, checksum in the installer, reproducible builds as a goal |
| Project CI runs third-party code with secrets | check trees without secrets (avoid the `pull_request_target` trap), merge bot separate from the checks |
| Embellished check results of a project | spot checks on platform hardware, deviations public |
| Prompt injection on the path without code: the contributor's own agent (full rights on their machine) reads third-party task texts | origin label on every MCP response, third-party text separated from the brief, ready-made agent profiles without shell and network (see below) |
| Prompt injection against the lead through submitted contributions | a subagent reads contributions and only answers in a fixed schema; acceptance stays a lead decision with the test pipeline in front |

A residual risk remains (a contribution that passes all checks and still does harm). It is communicated openly.

### Third-party text: origin labels and fixed protection rules

Inspired by Archestra's guardrails (see below). Principle: whether an agent may still do risky things after reading third-party text is decided by **fixed rules outside the agent**, not by the model. Injected instructions cannot change that.

1. **Origin on every response.** Every MCP response separates the platform's brief (format, rules, allowed paths) from third-party text (the visionary's description, contributions, comments) and says who wrote it and with which trust level.
2. **Profiles for the path without code.** For Claude Code, Codex, opencode and others we ship ready-made settings for platform work: no shell, no network except the platform, writing only in one working folder. Whoever needs more rights takes the runner path with the sandbox.
3. **Reading in a fixed schema.** If an agent with many rights (above all the lead) has to assess third-party text, a subagent without tools reads it and only answers in a strict JSON schema (e.g. recommendation, findings with path and line). Free text is discarded. The platform checks the schema.

## Comparison: Archestra (checked on 25 Oct 2026)

[Archestra](https://github.com/archestra-ai/archestra) (AGPL-3.0 with an enterprise license, about 4,400 stars) is a platform for companies. It sits between the employees' agents and the company's systems: LLM proxy with budgets, MCP gateway with a catalog, agent runtime on Kubernetes, guardrails, audit log.

**Fundamental difference:** Archestra trusts known employees of a company, runs agents centrally and keeps the keys centrally on the server. We work with unknown contributors; their agents run on their machines, and the keys never leave the machine. Using Archestra as a building block would contradict our core message ("your keys stay with you") and put a lot of third-party code at the most security-critical spot. We do not use it.

**Ideas taken over:**

| Idea | With us |
|---|---|
| trust labels per session, fixed rules outside the agent | origin labels, agent profiles, reading in a fixed schema (section above) |
| a subagent only answers in a fixed schema | review tasks and the lead |
| budgets on several levels, all matching limits apply at the same time | proxy: per task, day, project |
| clear error codes: 402 for money budget, 429 with `Retry-After` for rate | proxy, so agents react cleanly |
| only the usage reported by the provider counts; prices of local models are set by hand | donations and usage in tokens; euros only where a price is known |
| OpenTelemetry and Prometheus from the start | operations |
| AGPL for a service: forks that run as a service must disclose their changes | input for the license decision (we do not adopt open core) |

## Comparison: agentgateway (checked on 25 Oct 2026)

[agentgateway](https://github.com/agentgateway/agentgateway) (Apache-2.0, a Linux Foundation project, Rust, about 5,600 stars, version 1.6) is a proxy for agent traffic:

- **LLM gateway:** about 20 providers, an OpenAI-compatible interface with translation, e.g. chat completions → Anthropic.
- **MCP gateway:** several MCP servers behind one endpoint, stdio, HTTP and Streamable HTTP, OpenAPI → MCP; OAuth as a resource server.
- **More:** A2A, roles and rules with CEL, rate limits, OpenTelemetry, guardrails.
- **Operations:** as a standalone binary or in Kubernetes.

**Fundamental difference from Archestra:** agentgateway is a single, neutrally governed program without an enterprise license and runs wherever you start it. So it can run at the contributor's, and the keys stay on their machine. That fits our core message.

**We do not use it on the platform.** For MCP, agentgateway is only a resource server and needs an external sign-in service (built in: Keycloak, Auth0). Our own OAuth server already does more of what we need: metadata documents, dynamic registration, device code for the runner, plus our stateful tools. A gateway in front would be an extra hop without benefit.

**At the contributor's (runner) it is interesting.** To be checked in a spike:

| Idea | With us |
|---|---|
| Translation between providers: whoever has an Anthropic, Gemini or Bedrock key can use it, although agents in the sandbox speak OpenAI-compatible | chain sandbox → our small key proxy (token, budget, time, log; stays the security boundary) → agentgateway on the host (translation, fallback on errors) → provider |
| MCP federation with a tool filter: donated services with their own MCP server (e.g. Meshy, ElevenLabs) appear in the sandbox as one endpoint, only with allowed tools and a rate limit, without the agent seeing the key | donation adapters and brief + upload; first check whether agentgateway can inject credentials for upstream servers (docs unclear) |
| same metric names for MCP and LLM calls in OpenTelemetry | operations, metrics in the runner |
| rules as CEL expressions instead of code | an idea for trust levels if the rules grow; fixed rules in code are enough today |
| guardrails via webhook (e.g. detecting injected instructions) | at most as an extra layer; the fixed rules outside the agent stay authoritative |

**Not taken over:** A2A (maybe later between leads), Kubernetes operations and a gateway in front of the platform.

**Conditions for use in the runner:** the spike measures size, memory and start time, checks tool calls and streaming with opencode against Anthropic and Gemini and looks at the licenses of the dependencies. It stays optional: without agentgateway the runner works as today with OpenAI-compatible endpoints (local or frontier).
