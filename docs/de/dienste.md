# Dienste: Rechte an Ergebnissen

Hintergrundrecherche für den unverbindlichen Tarif-Hinweis in `beitragserklaerung.md` . **Die Verantwortung für Rechte liegt bei den Beitragenden, nicht bei der Plattform** (Entscheidung neVs, 24.10.2026). Diese Seite ist keine Prüfpflicht. Pro Dienst und Tarif: Wem gehört das Ergebnis, und darf es in ein Open-Source-Projekt? Jede Zeile nennt Quelle und Prüfdatum. **Keine Rechtsberatung.** Vor dem öffentlichen Start sollte eine Fachperson drüberschauen.

## Grundregel: „nicht kommerziell“ passt nie in ein Open-Source-Projekt

Auch wenn ein Projekt selbst nichts verkauft: MIT und CC BY erlauben **jedem** die kommerzielle Nutzung. Ein Ergebnis, das nur „privat/nicht kommerziell“ genutzt werden darf, kann deshalb nicht unter eine offene Lizenz gestellt werden. Praktisch heißt das: **Gratis-Tarife fast aller Dienste scheiden aus.** Ausnahmen sind Dienste, die Gratis-Ergebnisse selbst unter eine offene Lizenz stellen (Meshy: CC BY 4.0).

## Meshy

Quelle: Terms of Service, Stand „Last Updated: September 19, 2026“, geprüft am 24.10.2026 (https://www.meshy.ai/terms-of-use)

| Tarif | Eigentum am Ergebnis | Lizenz | Für Open-Source-Projekte |
|---|---|---|---|
| Free | **Meshy** besitzt das Ergebnis (§3.2) | CC BY 4.0, Namensnennung „Meshy“ Pflicht | **ja**, mit Namensnennung im Asset-Manifest und in den Credits |
| Bezahlt | **Kunde** besitzt das Ergebnis (§3.2) | frei lizenzierbar | **ja** |
| Community-Seite | gepostete Ergebnisse unter CC0 (§3.3) | gemeinfrei | ja |

Zusätzliche Punkte:

- **Service Assets (§2.3/§3.1):** Animationen, Rigs, vorberechnete Texturen aus der Meshy-Bibliothek sind nur lizenziert, *soweit sie im Ergebnis stecken und für dessen Nutzung nötig sind*. Als eigenständige Dateien unter einer offenen Lizenz weiterzugeben, ist damit nicht gedeckt. **Folge:** Herausgelöste Bibliotheks-Animationen (z. B. retargetierte Clips) gehören nicht pauschal unter die Projektlizenz, sondern unter einen Sonderhinweis. Alternativ erzeugt man sie über Text-to-Motion als eigenes Ergebnis neu. Das ist eine Auslegungsfrage, die fachlich geklärt werden sollte.
- **KI-Kennzeichnung (§2.4):** Metadaten und Wasserzeichen, die Meshy in Ergebnisse einbettet, dürfen nicht entfernt oder verändert werden. **Folge:** Pipelines (Retarget, Re-Export) müssen sie erhalten. Das Asset-Manifest kennzeichnet KI-Herkunft ohnehin.
- **Training (§2.6):** Ergebnisse dürfen nicht zum Training von Modellen genutzt werden, die mit Meshy konkurrieren. Das bindet den Erzeuger, nicht das Projekt. Trotzdem gehört es als Hinweis ins Manifest.
- **API (§2.5):** API-Ergebnisse löscht Meshy nach 3 Tagen. Der Runner muss sie also sofort sichern.
- **Training durch Meshy (§2.9):** Meshy darf Ein- und Ausgaben von Nicht-Enterprise-Kunden zum Training nutzen. Spender sollten das wissen.

## Higgsfield

Quellen: Hilfeartikel „Who owns my generations and can I use them commercially?“ und Zusammenfassung der Terms-Änderung (wirksam ab 27.08.2026), geprüft am 24.10.2026. Der Volltext der Terms (https://higgsfield.ai/terms-of-use-agreement) war beim Abruf nicht direkt lesbar.

| Tarif | Eigentum am Ergebnis | Kommerzielle Nutzung | Für Open-Source-Projekte |
|---|---|---|---|
| alle | **Erzeuger** (§4.4); Higgsfield beansprucht kein Eigentum, Rechte bleiben auch nach Kündigung | laut Terms und Hilfe nicht eingeschränkt, nicht an einen Tarif gebunden | **ja**, rechtlich |
| Free | wie oben | Ein Drittanbieter (Krea-Blog) berichtet, die Preistabelle führe „Commercial use“ beim Free-Tarif als *nicht enthalten*. Das ist ein Widerspruch zu den Terms und nicht selbst geprüft | **unklar** |
| Free | – | Ergebnisse tragen ein **Wasserzeichen**; ohne Wasserzeichen nur in bezahlten Tarifen | für Intros/Trailer praktisch ungeeignet |

Zusätzliche Punkte:

- **Training (§5.1(iii), §5.2(iv)):** Ergebnisse dürfen nicht zum Training von KI-Modellen genutzt werden. Das bindet den Erzeuger. Hinweis ins Manifest.
- **Training durch Higgsfield (§4.4):** Higgsfield nutzt Inhalte zum Training (Enterprise ausgenommen).
- **Fremde Rechte (§5.1(ii)(b), §5.3):** Der Erzeuger haftet für Rechte Dritter (Marken, Gesichter, Urheberrecht). Das betrifft vor allem Referenzbilder.

## Suno (Musik)

Quelle: Terms of Service, „Last revised August 10, 2026“, wirksam ab 03.09.2026, geprüft am 24.10.2026 (https://suno.com/terms)

| Tarif | Eigentum | Für Open-Source-Projekte |
|---|---|---|
| Free (Basic) | Nutzung nur „lawful, personal and non-commercial“ | **nein** |
| Pro / Premier | Suno überträgt seine Rechte am Ergebnis an den Nutzer. Aber: Das Ergebnis bleibt „subject to these Terms … including any applicable commercial use restrictions“, und Suno garantiert nicht, dass überhaupt Urheberrecht entsteht | **unklar → Fachperson** |

Zusätzliche Punkte:

- **Kommerzielle Nutzung nur nach einem erlaubten Download** über Suno, mit monatlichem Kontingent. Die Rechte daran bleiben nach Kündigung oder Downgrade bestehen.
- **Remixes** sind unabhängig vom Tarif nur nicht-kommerziell nutzbar, also nie für Open Source.
- **Wasserzeichen und Metadaten** dürfen nicht entfernt werden. Trainingsverbot für Ergebnisse.
- Ob ein Pro-Nutzer ein Lied unter CC BY stellen darf, das *Dritten* jede kommerzielle Nutzung erlaubt, lassen die Bedingungen offen („distribute, license … beyond the permitted Download use“ ist untersagt). Bis zur Klärung: **Suno nicht in der Lizenzliste.**

## ElevenLabs (Stimmen, Soundeffekte, Musik)

Quelle: offizieller Hilfeartikel „Can I publish the content I generate on the platform?“, geprüft am 24.10.2026

| Tarif | Kommerziell | Für Open-Source-Projekte |
|---|---|---|
| Free / nicht angemeldet | „does not include a commercial license“, Namensnennung „elevenlabs.io“ Pflicht | **nein** |
| Bezahlt | kommerzielle Lizenz, zeitlich unbegrenzt für Inhalte, die während des Abos erzeugt wurden; **nicht** für Beta-Dienste | **ja, voraussichtlich** (Service-Specific Terms für Soundeffekte/Musik noch lesen) |

## Ohne offizielle Quelle geprüft (nur Sekundärquellen, nicht in die Lizenzliste)

| Dienst | Laut Sekundärquellen | Status |
|---|---|---|
| **Udio** (Musik) | Eigentum bei Udio bzw. Lizenzgebern; Standard „persönlich, nicht kommerziell“ | Offizielle Terms beim Abruf nicht lesbar. **Vorerst nein** |
| **MiniMax / Hailuo** (Video, Musik) | Free: Wasserzeichen, nicht kommerziell. Bezahlt: kommerziell, Eigentum beim Nutzer. Offene H3-Gewichte: Lizenz schließt EU, UK, Südkorea und USA aus, auch für Ergebnisse | Hosted-Dienst bezahlt: offizielle Terms lesen. **Lokales MiniMax mit offenen Gewichten: in der EU laut Sekundärquelle nicht erlaubt** – unbedingt prüfen |
| **Kling** (Video) | Free: nicht kommerziell, Wasserzeichen. Bezahlt: kommerziell. Breite Lizenz an Kling/Kuaishou zur Nutzung der Inhalte | offizielle Terms lesen |
| **Tripo** (3D) | Widersprüchlich: Blog sagt Free = CC BY 4.0, Hilfeseite sagt Free = Tripo behält alle Rechte (Terms 5.2.1). Bezahlt: Rechte beim Nutzer (5.2.2) | **Free nein**, bezahlt nach Prüfung der Terms |

## Folgen für die Plattform

Seit der Entscheidung vom 24.10.2026 gibt es keine Lizenzliste mit Durchsetzung mehr. Es gibt nur noch:

1. die Beitragserklärung, mit der der Beitragende die Rechte versichert
2. einen unverbindlichen Tarif-Hinweis (Meshy, Higgsfield, ElevenLabs, jeweils bezahlte Tarife)
3. ein Meldeformular und einen Takedown-Ablauf

Siehe `beitragserklaerung.md`.
