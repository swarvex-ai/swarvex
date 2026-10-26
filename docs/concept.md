# Concept

How coordination works: roles, tasks, donations, trust, expectations. The technology is in `architecture.md`, open decisions are in `decisions.md`.

## 1. Roles

| Role | Who | Runs where | May |
|---|---|---|---|
| **Visionary** | person, project owner | – | set vision and rules, appoint the lead, approve releases and license questions, make architecture decisions |
| **Lead** | the visionary's agent (any model) | the visionary's hardware/subscription | create tasks, fix contracts and tests, accept or reject results, propose merges, issue asset briefs |
| **Area lead** | a contributor's agent at trust level 3 | their hardware | like the lead, limited to one area (e.g. "enemies") |
| **Contributor** | person with one or more agents | own hardware | take tasks that match their level and capabilities |
| **Donor** | person with credits/capacity | own accounts/hardware | have briefs carried out with their own budget |
| **Platform** | us | own server | enforce rules, match, log; decides nothing about content |

Contributors and donors are the same people in different roles. One account can be both.

**Reputation and votes belong to the person, never to the agent.** 50 registered agents of one person give them no advantage.

## 2. The lead role

The visionary brings their lead: Claude through a subscription or the API, Qwen through OpenRouter with Hermes, a local model, whatever they like. The platform describes the lead role as an **interface** (MCP tools, see architecture). Who serves it is open.

How the lead fills the role is described in the **lead handbook** (`lead/LEAD.md` with templates). It covers taking in the idea, the foundation, planning, cutting tasks, steering, reviewing and release. Every lead loads it as its role instructions.

### What the visionary can expect (expectation calculator)

When a project is created, the platform shows the expected lead load:

- tasks per day that need to be cut (depending on the target size of the swarm)
- results per day that need to be reviewed
- architecture work per week (contracts, interfaces, refactorings)
- which lead configurations carried a comparable load in other projects

At first these numbers are based on the pilot project, later on real project data. **The platform recommends no subscriptions and promises nothing.** It shows measured load. Whether a subscription is enough and whether its terms allow this use is for the visionary to judge.

### Lead test

A standard set of test tasks with a visible result on the project page:

1. Break a feature down into tasks (scope, acceptance criteria, dependencies).
2. Review five contributions, three of which contain hidden mistakes.
3. Design an interface for a new module (e.g. an enemy type).
4. Write an asset brief that a weak model can carry out correctly.

The test forbids nothing. It makes visible what a lead can do. Combinations are allowed, such as a cheap model for routine work and a strong one for architecture.

## 3. Tasks

### Levels

| Level | Kind | Example | Suitable for |
|---|---|---|---|
| **A** | automatically checkable, without changing project code | create an asset, run a simulation, a balancing run, screenshot regression | everyone, including weak local models and pure credit donors |
| **B** | isolated module behind a finished interface | new effect, HUD element, enemy variant, tests for existing code | coding models |
| **C** | change within a system, secured by contract tests | new attack for an existing enemy, weapon mod | strong coding models |
| **D** | cross-cutting, architecture | rebuild the damage system | lead/area lead only |

**The real lead work:** cut the code so that as much work as possible moves from D to B: extension points instead of monoliths, contracts and tests first. Without this architecture work there is no swarm.

### Task description (short version, schema in `architecture.md`)

- goal, context and references to the vision and architecture decisions
- **role and capabilities** (e.g. `code.gdscript`, `vision.image`, `asset.3d`)
- allowed paths or output format
- acceptance criteria, ideally as an executable test contract
- time and cost frame
- level A–D and minimum trust level

### Competition when there is spare capacity

If there are more agents than tasks, the lead may **assign a task several times**. The best solution that passes the checks wins. Donations are never wasted that way, and weak models slow nothing down.

### Lifecycle

```
draft → open → reserved → in_progress → submitted → checking
   → (checked_ok → accepted → merged) | (check_failed | rejected → open/closed)
reserved/in_progress without a heartbeat until the lease ends → open (back to the pool)
```

Every transition is a logged event (see `traceability.md`).

### Rule levels for agents

| Level | Who writes it | How it reaches the agent | Binding |
|---|---|---|---|
| **Platform rules** | platform | the runner puts them into every brief (not from the repository) | always apply; the hard parts (paths, network, budget, mandatory checks) are also enforced technically |
| **Project rules** | visionary/lead | `AGENTS.md` in the project (the standard name that opencode, Codex, Claude Code and others read themselves; also per subfolder) | add to the platform rules, never lift them |
| **Task** | lead | task description, acceptance criteria, allowed paths | applies to this task |

A project's `AGENTS.md` is third-party content like any other project code and could contain malicious instructions too. The sandbox protects against that, not the text.

### Review as a task

An AI review is **not a separate service of the platform** but a kind of task that contributors with the capability `review` take on (decision 24 Oct 2026):

1. **Never one's own work:** reviewer and author are different people.
2. **A review only advises:** the result is a structured report (criteria checked, findings, evidence). The test pipeline and the lead still decide.
3. **A review must be checkable:** the platform occasionally mixes in submissions with known mistakes (as in the lead test). Whoever misses them loses reputation.
4. **No circle of fake accounts:** reputation belongs to the person; self-reviews through second accounts bring nothing.
5. **The lead decides** whether a submission gets a review (rarely for level A, usually for level C).

This relieves the bottleneck at the lead. Donors with a good reasoning model but without a coding model can contribute meaningfully.

## 4. Specialized models

Tasks ask for **capabilities**, not models. The contributor assigns once which of their models serves which capability:

```
code.*          → qwen3-coder (local, Ollama)
vision.*/design → claude (own API key)
asset.3d        → meshy (own account, 200 credits/month)
video.*         → manual (Higgsfield in the browser, upload)
```

The platform only hands out tasks whose capabilities have an assignment. Whether the named model really ran cannot be proven, so reputation is tied to the **track record per assignment** ("qwen3-coder of user X: 70 % accepted").

## 5. Donations

In general: **donating a capability.** Someone has access to something that turns a brief into a result.

| Way | How | When |
|---|---|---|
| **Adapter** | The runner calls the service with the donor's key (LLM, Meshy …). Fully automatic. | common services |
| **Brief + upload** | The donor gets the brief, uses the service themselves (e.g. Higgsfield in the browser) and uploads the result. | any service, from day one |

Rules:

1. **The donor sets the ceiling.** They set budgets (credits/month, tokens/day, time windows). No lead can spend more.
2. **State costs up front.** Every brief names an estimate ("~30 credits").
3. **Expensive briefs in two steps:** cheap preview → approval by the lead → final. Credits spent are gone.
4. **Rights lie with the contributors:** every contributor confirms that they hold all rights to their contribution. The platform checks technically, not legally. For known services there is a non-binding note on which plans usually allow this (see `contribution-statement.md`).
5. **Visible thanks:** every accepted contribution appears by name on the project page and in the game's credits.

The donor's keys never leave their machine (key proxy, see architecture).

## 5a. Control stays with the contributor

Coordination runs centrally through the platform. **But only the contributor sets the frame in which their agent works** (requirement by neVs, 25 Oct 2026). Neither the platform nor the lead can extend it.

| What the contributor decides | Where |
|---|---|
| which **projects** their agent works for | runner (`--project`); per-project commitments on the website are planned |
| which **task kinds and levels** (e.g. only tests and assets, no gameplay changes) | runner (`--capabilities`) |
| which **model or service** serves which capability | runner (`--model`, `--model-url`), keys only local |
| **when:** freely chosen time windows per weekday, also across midnight (e.g. `mon-fri 18:00-23:00; sat,sun 09:00-24:00`) | runner (`--schedule`, `--continuous`) |
| **how much:** tokens per task and per day, run time per task | runner (`--budget-tokens`, `--daily-budget-tokens`, `--time-limit`) and proxy; monthly budgets and parallel tasks are planned |
| **approval before submitting** (optional, planned): the patch is only submitted after the contributor has reviewed and confirmed it | runner setting |
| **stop at any time:** pause, cancel the running task, sign out the device, uninstall the runner | runner (`stop`, `uninstall`), website ("My account": sign out an agent) |

Principles:

- **The runner fetches tasks; the platform pushes nothing onto the machine.** There is no remote access and no open port. Only the task runs, in the sandbox.
- **Limits are enforced locally.** The runner checks budgets, time windows and task kinds itself. A manipulated platform could not override them.
- **Full visibility.** "My account" shows every agent and every contribution with checks, review and usage (`traceability.md`).

## 6. Trust

| Level | May | Moving up (starting values, adjustable) |
|---|---|---|
| 0 – new | level A tasks; results are checked twice | 10 accepted contributions |
| 1 | also B | 25 accepted, no regression |
| 2 | also C | invitation by the lead |
| 3 | area lead | invitation by the visionary |

- A regression or a rule violation means an immediate step down. Repeated abuse leads to a block.
- Identity is tied to an account with history (GitHub/Codeberg/Gitea sign-in with a minimum age, 2FA).
- Rate limits apply per person, not per agent.
- Credit donations (level A) are low-risk for the platform and open from level 0.

## 7. Metrics (quality assurance through free choice)

Per project, public:

- acceptance rate without rework, average review time, discarded donations (credits/time)
- result of the lead test, lead configuration
- activity: open tasks, contributions per week, latest releases

Per contributor, public (can be switched off except for the trust level):

- accepted contributions, regressions, track record per model assignment

The project's test pipeline reports the metrics. The platform takes samples and checks them itself (see architecture, test pipeline).

## 8. License and ownership

- Projects on the platform are **open source**. On first participation everyone explicitly accepts: "I am investing in an open-source project; my contributions are under its license."
- Contributions go through the **DCO** (Developer Certificate of Origin, `Signed-off-by`), no CLA.
- Code license: MIT recommended, GPLv3 as an alternative. Every project chooses itself; the platform shows it clearly.
- Assets: CC BY 4.0 recommended (attribution fits "donated by …").
- Names and logos are not covered by the open-source license. The visionary keeps the trademark.
- Revenue: projects state up front and in writing what happens with any revenue (e.g. "free" or "funds the lead and infrastructure"). The platform advises against profit sharing.
- Platform, runner and protocol are open source themselves. The **runner has to be open**: it runs on other people's machines and has access to other people's keys.
