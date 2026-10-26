# Security policy

swarvex runs foreign project code on contributors' machines, so security reports matter a lot to us.

## Reporting a vulnerability

Please report vulnerabilities **privately** through GitHub: *Security → Report a vulnerability* on this repository. Do not open a public issue.

Include what you found, how to reproduce it and what an attacker could achieve. We confirm receipt within a few days, keep you informed and credit you in the advisory unless you prefer otherwise.

## Scope

Especially relevant: escaping the runner sandbox, reaching the network or the host from inside it, extracting API keys from the key proxy, forging test results or the event log, bypassing the clean-room separation, and OAuth/MCP authorization flaws. Our own attack tests are in `attacks/` and documented in `docs/angriffstests.md`.
