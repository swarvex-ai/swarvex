---
titel: Sandbox and key proxy
schluessel: sandbox
gruppe: Runner
reihenfolge: 21
beschreibung: How your machine is protected from foreign project code and why the agent never sees your keys.
---

Programming means running foreign code: tests, build scripts, imports. The sandbox protects your machine from it.

## Sandbox boundaries

| Boundary | Implementation |
|---|---|
| no admin rights | rootless Podman, all capabilities dropped, `no-new-privileges`, no setuid programs in the image |
| no access to your files | no home directory, only the project snapshot, read-only root file system |
| no network | internal network; only the key proxy is reachable |
| no runaway | limits for processes, memory and runtime; Podman stops the sandbox itself when time is up |
| no traces | the sandbox is deleted after every task |

## The key proxy

The agent inside the sandbox does not talk to your model directly but to a small proxy run by the runner:

- It only holds a **task token** that is valid for this one task.
- Only chat requests to **the one approved model** are allowed.
- The **budget** is enforced strictly. Once it is used up, there are no more responses.
- Your real API key stays with the proxy. The sandbox never sees it.

## Tested, not claimed

In our attack tests, ten classes of attacks with 46 individual checks failed, among them key theft, escaping to the network, reading the home directory, prompt injection, container escape and exceeding the budget. Details are under [Security](/en/docs/security).
