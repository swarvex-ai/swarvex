# Roadmap

The issues and milestones in the tracker hold the current state. This file shows the structure.

Order: **P proof of concept (local, free)** → M0 → M1 → **M2 end-to-end run** (everything once for real, end to end) → M3–M7 (partly in parallel) → M8 pilot.

## P Proof of concept (local)

Completed on 25 Oct 2026 with "go".

- Test environment on the home server
- Minimal platform: tasks, leases, event log, MCP gateway with OAuth
- Lead connection: Claude Code via MCP
- Runner prototype: device sign-in, Podman sandbox, key proxy, opencode + a local model
- Test pipeline: headless Godot, signed result, merge bot
- Attack tests S1–S10
- One-liner test in a fresh VM
- Go/no-go report

## M0 Foundations

Open fundamental decisions, legal matters, threat model. Nothing gets built without them that would have to be rebuilt later.

- E-01 name and domain
- E-06 identity solution
- E-08 object storage
- E-13 license of the platform and the runner
- ~~E-16 check the terms of services and subscriptions~~ (decided: rights lie with the contributors)
- ~~E-17 pilot project as open source~~ (decided: Rend&Run)
- Threat model

## M1 Specification

Protocol, schemas, state machine, traceability: in writing and as JSON Schema before code exists.

- Task lifecycle as a state machine
- Task schema and test contract (`swarvex.yaml`)
- MCP tools per role
- Event log and hash chain
- Origin: Git trailers and asset manifest

## M2 End-to-end run

A single task runs for real from end to end: the lead creates it → the runner (Linux) works on it in the sandbox → the test pipeline checks it → the lead accepts it → the merge bot merges it, everything logged. Minimal, but with the final components.

- Monorepo skeleton with CI
- PostgreSQL schema and migrations
- Identity (sign-in + device flow)
- Coordination core: tasks, leases, timeouts
- Event log with hash chain
- Minimal MCP gateway (Streamable HTTP + OAuth)
- Minimal runner (Linux): sign-in, sandbox, patch
- Minimal test pipeline
- Merge bot with trailers and DCO
- End-to-end test with a copy of the pilot game

## M3 Core and security

Make the end-to-end run robust: complete tools, leases, spot checks, runners for macOS/Windows, hardening.

- Complete MCP tools
- Direct mode for text tasks
- Runner: schedule, budget guard, service mode
- Runner: more agent programs (Claude Code, Codex, Hermes)
- Runner for macOS and Windows
- ~~Runner: proprietary engines from the contributor's machine (Unity, Unreal; E-31)~~ (built: own test run and cross-check; still to test with a real Unity and Unreal)
- One-line installer and release pipeline
- Spot checks on platform hardware
- Operations: production compose, backups, monitoring

## M4 Donations

Donating credits and capacity: budgets, adapters (Meshy), brief + upload, asset checks.

- Budgets and commitments per service
- Meshy asset adapter (preview → approval → final)
- Brief + upload (any service)
- Asset checks (import, polygons, skeleton, screenshot)
- Contribution statement, plan note and report form

## M5 Lead and expectations

Lead test, expectation calculator, metrics, multiple assignment.

- Lead test: task set and evaluation
- Expectation calculator
- Metrics per project and contributor
- Multiple assignment and competition

## M6 Website

Public pages, dashboards for both sides, project assistant.

- Website skeleton and public pages
- Contribute: one-liner, assignments, budgets
- Dashboards: my contributions and project cockpit
- Start-a-project assistant
- Rules, transparency, docs

## M7 Trust and abuse

Trust levels, identity, rate limits, diff rules, reports.

- Trust levels: moving up and down
- Identity, rate limits, blocks, reports
- Diff rules: protected paths and forbidden APIs

## M8 Pilot

The pilot game as the first public project with the first external participants; evaluation.

- Make the pilot game swarm-ready (modules, contracts, tests)
- Pilot game public on the platform
- Pilot evaluation
