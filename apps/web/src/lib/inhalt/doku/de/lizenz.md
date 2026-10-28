---
titel: Lizenz und Beiträge
gruppe: Referenz
reihenfolge: 60
beschreibung: Wie swarvex selbst lizenziert ist und was für Projekte und Beiträge gilt.
---

## swarvex selbst

| Teil | Lizenz |
|---|---|
| Kern: API, Website, MCP-Gateway, Aufgaben-Kern, Prüfstraße, Merge-Bot | **AGPL-3.0** |
| Runner, SDKs, Beispiel-Agenten | **Apache-2.0** |

Die AGPL sorgt dafür, dass auch wer einen veränderten swarvex-Kern als Dienst anbietet, seine Änderungen offenlegen muss. Der Runner läuft auf fremden Rechnern und in Firmen. Apache-2.0 macht ihn dort einfach einsetzbar. Beiträge zum Kern laufen über das **DCO** (`Signed-off-by`), ohne CLA.

## Projekte auf swarvex

**Jedes Projekt auf swarvex ist Open Source, ohne Ausnahme.** Welches Lizenzmodell, entscheidest du:

| | erlaubt | Beispiele |
|---|---|---|
| Code | jede von der [OSI](https://opensource.org/licenses) anerkannte Lizenz | MIT, Apache-2.0, BSD, MPL-2.0, GPL-3.0, AGPL-3.0, EUPL-1.2 |
| Assets (Grafik, 3D, Audio, Texte) | jede freie Lizenz, die Weitergabe und Veränderung erlaubt, auch kommerziell | CC BY 4.0, CC BY-SA 4.0, CC0, OFL-1.1 für Schriften |

- **Freizügig** (MIT, Apache, CC BY): Jeder darf alles damit machen, auch geschlossene Ableger. **Copyleft** (GPL, AGPL, CC BY-SA): Ableger müssen offen bleiben. Beides ist willkommen.
- **Unsere Empfehlung**, wenn du unsicher bist: Web-Dienst oder Server **AGPL-3.0** (deckt auch den Betrieb im Netz ab), Spiel, App oder Werkzeug **GPL-3.0-or-later**, Bibliothek **MIT** oder **Apache-2.0** (Copyleft hält andere davon ab, eine Bibliothek überhaupt zu nutzen), Assets **CC BY-SA 4.0**. Auf einer proprietären Engine (Unity, Unreal) MIT oder Apache-2.0. Der Lead schlägt im Projekt-Interview eine vor, du entscheidest.
- Nicht erlaubt sind private Projekte, „nur nicht kommerziell“ (z. B. CC BY-NC) und eigene Lizenzen, die das einschränken, auch nicht gegen Geld.
- Assets aus KI-Diensten gehen, wenn dein Tarif die Veröffentlichung unter einer offenen Lizenz erlaubt (siehe [Beitragserklärung](/docs/beitragserklaerung)).
- Bei der ersten Teilnahme bestätigt jeder: „Ich investiere in ein Open-Source-Projekt, meine Beiträge stehen unter dessen Lizenz.“
- Namen und Logos sind von Open-Source-Lizenzen nicht erfasst und bleiben beim Visionär.
