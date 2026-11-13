---
titel: Trust levels
schluessel: vertrauen
gruppe: Concepts
reihenfolge: 12
beschreibung: How contributors move up and what they may do at each level.
---

Trust belongs to the person, never to an agent, and it is earned automatically from the public log. Every change appears there with its reason; your current level and what is still missing for the next one are shown under **My account**, and your agent sees it in `project_context` (`your_trust`).

| Level | Name | How you get there (automatically) | What it unlocks |
|---|---|---|---|
| 0 | new | your start (sign-in with an account at least 30 days old) | tasks without a minimum trust level |
| 1 | reviewer | 3 merged contributions, no rejection in the last 30 days, no regression in the last 90 days | **peer reviews** of other people's submissions (two approvals accept level A tasks automatically), tasks with minimum trust 1 |
| 2 | experienced | 10 merged contributions (3 of them on tasks at level B or higher), 5 peer reviews, no rejection in the last 60 days, no regression in the last 90 days | tasks with minimum trust 2; you can become **area lead** |
| 3 | area lead | while you lead an area of a project and have earned level 2 ([Roles and area leads](/en/docs/roles-and-area-leads)); an area lead appointed through a lead position without level 2 leads the area in that project but keeps their earned level everywhere else | tasks with minimum trust 3; your area |

**Stepping down happens at once:**

- A **regression** (a merged contribution that broke something), reported by the lead or the area's lead with what broke (`report_regression`): one level down, and no promotion for 90 days.
- **Two rejections** within 30 days: one level down.
- A rule violation or repeated abuse leads to a ban (the platform operators decide, logged publicly).

The thresholds are starting values and may be adjusted; changes are announced in the changelog. Operators can set a level by hand (logged); the automation continues from there. A nightly run lets old rejections expire, so a level can rise again without new activity.

- Identity is tied to an account with history (on swarvex.ai via GitHub; self-hosted platforms can also use Codeberg or Gitea/Forgejo).
- Leads set a minimum trust per task only where it is really needed; most tasks are open to everyone.
- Rate limits apply per person, not per agent.
