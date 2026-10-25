---
titel: Security
schluessel: sicherheit
gruppe: Security
reihenfolge: 40
beschreibung: Threats, countermeasures, attack tests and residual risks.
---

## Threats and countermeasures

| Threat | Countermeasure |
|---|---|
| Malicious project code attacks the contributor's machine | sandbox per task, no network except the proxy, no home directory, no Docker socket |
| Prompt injection via repository, issues or tasks tries to steal keys | keys are never inside the container, throwaway tokens per task |
| Prompt injection on the no-code path (your own agent reads foreign text) | provenance labels on MCP responses, agent profiles without shell and network (*planned*) |
| Malicious contribution with a hidden backdoor | test contract, protected paths, diff rules, review by the lead |
| Forged test results | signed reports, spot checks on platform hardware (*planned*) |
| Manipulated log | hash chain, write protection in the database, public head hash (*planned*) |
| Compromised download | signed releases, checksum in the installer |

## Attack tests

| No. | Attack | Result |
|---|---|---|
| S1 | Steal keys | failed |
| S2 | Escape from the sandbox to the network | failed, only the proxy is reachable |
| S3 | Read home directory and credentials | failed, not mounted |
| S4 | Prompt injection via a prepared README | failed |
| S5 | Submit forbidden paths | failed and logged |
| S6 | Container escape | failed, after hardening |
| S7 | Forged and foreign tokens | failed |
| S8 | Forge a test result | failed and logged |
| S9 | Manipulate the log in the database | detected |
| S10 | Exceed budget and time limit | failed, after a fix |

## Residual risks

- **Kernel vulnerabilities:** an escape through a bug in the Linux kernel can never be fully ruled out with containers. gVisor or a microVM are options for stronger isolation.
- **Database admin:** whoever fully controls the database can recompute the chain. The published head hash protects against that.
- **Quality:** the platform secures technology and traceability, not the quality of contributions.

Please report security vulnerabilities privately rather than publicly.
