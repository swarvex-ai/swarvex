# Agent compatibility test

swarvex lives on as many agents as possible connecting as easily as possible. This test signs **real agent programs** in to swarvex via MCP OAuth and checks that they can see the tools afterwards.

| File | Content |
|---|---|
| `compose.yaml` | test instance: own database (in memory), API, a fake account provider |
| `fake_provider.py` | Gitea-style account provider that approves every sign-in right away for the test account "compat" |
| `browser.py` | stand-in browser: follows the sign-in URL like a person until the redirect back to the agent |
| `agents.py` | test run: empty config directory per agent, setup, sign-in, check; result in `results.md` |
| `account.py` | "My account" end to end: website sign-in, signing an agent out, old tokens invalid |
| `fake_model.py` | fake model server for the runner's agents (E-32): answers the Anthropic Messages and OpenAI Responses APIs with a fixed text and logs path, auth headers and model of every request |

## Agents in the runner's sandbox (E-32)

Checks that Claude Code and Codex run in the sandbox exactly as the runner starts them: the real key stays in the proxy, there is no internet, own skills are mounted, usage is counted. `swarvex attack-test --agent <name>` uses the same sandbox setup as real tasks and runs a script instead of the agent.

```sh
# on the contributor VM, as the runner user
python3 compat/fake_model.py 8099 &
swarvex attack-test --agent claude --script agent-claude.sh --model claude-test \
  --model-url http://host.containers.internal:8099 --model-key-file <file with a fake key>
```

The script prints whether the fake key is visible anywhere in the sandbox (must be 0), whether the internet is reachable (must not be), lists the skills and then starts the agent. The fake model's log must show the real key (or subscription token) only there.

## Running it

```sh
# start the test instance on the platform VM (separate from the lab stack)
ssh <platform-vm> 'cd /opt/swarvex && docker compose -f compat/compose.yaml up -d --build'
# tunnel: agents see the instance as http://localhost:3800 (MCP allows HTTP only on localhost)
ssh -f -N -L 3800:localhost:3800 -L 3999:localhost:3999 <platform-vm>
# optional: install more agents only here (ignored by Git)
(cd compat/tools && npm i opencode-ai openclaw @google/gemini-cli)
compat/agents.py            # or single ones: agents.py claude codex-dcr
compat/account.py           # account page: sign-in, signing an agent out
# clean up afterwards
ssh <platform-vm> 'cd /opt/swarvex && docker compose -f compat/compose.yaml down'
```

The machine's real setup stays untouched: every agent gets its own `HOME` and config directories under `output/run-*`.

## Status 25 October 2026

| Agent | Version | Registration | Result |
|---|---|---|---|
| Claude Code | 2.1.310 | client ID metadata document | ✔ connected, tools visible |
| Codex | 0.154.0 | metadata document (default) and dynamic (`--oauth-client-registration dcr`) | ✔ both ways |
| Hermes | 0.20.6 | metadata document | ✔ 8 tools |
| opencode | 1.19.4 | dynamic | ✔ connected |
| OpenClaw | 2026.10.5 | dynamic | ✔ 8 tools (only with `--transport streamable-http`) |
| Gemini CLI | 0.65.0 | – | setup ✔; sign-in only interactive (`/mcp auth`), cannot be automated |

Found and fixed during the first run:

- Claude Code's metadata document lists `http://localhost/callback` without a port and then uses a random port. Under RFC 8252 that is correct for local programs, so the server treats clients with loopback-only redirects as "native".
- Without support for metadata documents, Claude Code and Hermes would not have signed in as intended. An agent is now the pair (person, client), because all users of a program share one client ID.
