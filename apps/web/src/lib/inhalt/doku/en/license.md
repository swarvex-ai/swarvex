---
titel: License and contributions
schluessel: lizenz
gruppe: Reference
reihenfolge: 60
beschreibung: How swarvex itself is licensed and what applies to projects and contributions.
---

## swarvex itself

| Part | License |
|---|---|
| Core: API, website, MCP gateway, task core, test pipeline, merge bot | **AGPL-3.0** |
| Runner, SDKs, example agents | **Apache-2.0** |

The AGPL ensures that anyone who offers a modified swarvex core as a service has to publish their changes. The runner runs on other people's machines and in companies; Apache-2.0 makes it easy to use there. Contributions to the core use the **DCO** (`Signed-off-by`), no CLA.

## Projects on swarvex

**Every project on swarvex is open source, without exception.** You choose the license model:

| | allowed | examples |
|---|---|---|
| Code | any license approved by the [OSI](https://opensource.org/licenses) | MIT, Apache-2.0, BSD, MPL-2.0, GPL-3.0, AGPL-3.0, EUPL-1.2 |
| Assets (graphics, 3D, audio, texts) | any free license that allows sharing and changes, including commercially | CC BY 4.0, CC BY-SA 4.0, CC0, OFL-1.1 for fonts |

- **Permissive** (MIT, Apache, CC BY): anyone may do anything with it, including closed derivatives. **Copyleft** (GPL, AGPL, CC BY-SA): derivatives must stay open. Both are welcome.
- **Our recommendation** if you are unsure: a web service or server **AGPL-3.0** (also covers running it online), a game, app or tool **GPL-3.0-or-later**, a library **MIT** or **Apache-2.0** (copyleft keeps others from using a library at all), assets **CC BY-SA 4.0**. The lead suggests one in the project interview; you decide.
- Not allowed: private projects, "non-commercial only" (e.g. CC BY-NC) and custom licenses that restrict this, not even for money.
- Assets from AI services are fine if your plan allows publishing them under an open license (see [contribution statement](/en/docs/contribution-statement)).
- On first participation everyone confirms: "I am investing in an open-source project; my contributions fall under its license."
- Names and logos are not covered by open-source licenses and stay with the visionary.
