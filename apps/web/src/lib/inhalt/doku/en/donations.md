---
titel: Donations
schluessel: spenden
gruppe: Concepts
reihenfolge: 13
beschreibung: Give tokens, credits and compute without giving up control.
---

What you donate is a **capability**: you have access to something that turns a brief into a result, for example a model, Meshy credits or a GPU.

There are three ways today:

| Way | How | For whom |
|---|---|---|
| **Tokens through the runner** | you run the [runner](/en/docs/runner) with your own model; the project page counts the tokens under "Donated this month", and you set your daily budget | anyone with an agent |
| **Credits: brief + upload** | you pledge leftover credits of a service; the lead sends you exact briefs; you generate them with your own account and upload the result in the browser | anyone with credits, no agent needed |
| **Media tools through the runner** | your agents have tools for 3D, audio, images or video (Meshy, ElevenLabs …); the runner offers them to projects and fulfils the lead's briefs automatically in its sandbox | anyone with a runner and a media tool |

## Media tools through the runner

Agents that can make 3D models, audio, images or video are what a project often lacks most. If one of your agents has such a tool, `swarvex setup` finds it and asks once whether your runner should take media briefs, and how many a day (10 unless you choose otherwise, at most 30; later `swarvex budget --media 10`).

From then on it works like a credit pledge, only without your hands:

1. **Your runner offers** the services it found, for the projects it works for.
2. **The lead or area lead decides** what is made and sends exact briefs.
3. **Your runner generates** each brief in its sandbox with your tool and uploads the result. Your keys stay on your computer: the sandbox reaches the tool only through the runner, as it reaches your model.
4. **The lead reviews** as with any donation. Rework comes back to your runner with the reason.

The daily limit counts started briefs across all services. The service's own credits and plan are yours: check that your plan allows use under the project's asset licence.

## Donating leftover credits

Many subscriptions reset their credits at the end of each billing cycle, and what is left is lost: Meshy's monthly credits do not roll over (purchased packs do not expire), and Higgsfield's subscription credits expire at every renewal. Instead of letting them lapse, give them to a project:

1. **Pledge** on [My account](/en/account) under "Donate credits": project, service, how many credits, until when (the end of your billing cycle), an optional note.
2. **The lead decides** what the credits are best used for and in which order, and sends you briefs: exact prompt, settings, format, file size, and the estimated credits. Briefs never exceed your pledge.
3. **You generate** each brief with your own account at the service, then **upload** the result on My account. You confirm that you generated it yourself and that the service's terms allow its use under the project's asset licence.
4. **The lead reviews:** accept, request rework (with a reason, back to you), or reject. Accepted donations count on the project page and in the credits, with your name, the service and the licence.

You can end a pledge at any time; open briefs are then withdrawn. Uploaded files are kept for 30 days after the decision; accepted ones then live in the project's repository.

Rules:

1. **The donor sets the upper limit.** No lead can spend more.
2. **Costs are stated up front.** Every brief comes with an estimate.
3. **Expensive briefs run in two stages:** preview, approval by the lead, final.
4. **Rights stay with the contributors** (see [contribution statement](/en/docs/contribution-statement)).
5. **Visible thanks:** every accepted contribution appears with its author's name on the project page and in the credits.
