# Nachvollziehbarkeit

Beide Seiten müssen jederzeit sehen können, was passiert ist: der Visionär, wer was beigetragen hat, und der Spender oder Mitstreiter, was aus seinem Beitrag wurde.

## Ereignisprotokoll

- Jede Zustandsänderung ist ein **Ereignis** in einer append-only Tabelle: Aufgabe angelegt, reserviert, eingereicht, geprüft, bewertet, gemerged; Budget verbraucht; Vertrauensstufe geändert; Freigabe erteilt.
- Die Ereignisse sind **hash-verkettet**: Jedes trägt den Hash seines Vorgängers. Nachträgliche Änderungen fallen auf.
- Der Kopf-Hash der Kette wird täglich signiert veröffentlicht (`/api/protokoll/kopf`, Projektseite) und von einem externen öffentlichen Repo verankert (`infra/log-anchor`). So kann auch die Plattform die Kette nicht unbemerkt umschreiben.
- Das Protokoll ist öffentlich, mit Ausnahme personenbezogener Daten, die abschaltbar sind.

## Herkunft im Git-Verlauf

Jeder Merge-Commit des Merge-Bots trägt Trailer:

```
Aufgabe: <projekt>#<aufgaben-id>
Beitrag-von: <mensch> (<agent>)
Modell-Zuordnung: code.gdscript=qwen3-coder (angegeben)
Pruefung: <link zum Prüfbericht>
Bewertet-von: <lead-agent> — <kurzbegründung>
Signed-off-by: <mensch> <mail>
```

„Angegeben“ heißt: Das Modell ist eine Selbstauskunft und nicht bewiesen. Für Assets steht dieselbe Herkunft in einer `CREDITS`-Datei bzw. einem Manifest pro Asset (Dienst, Tarif, Spender, Auftrag, Lizenz).

## Sicht des Spenders/Mitstreiters

Dashboard „Meine Beiträge“:

- jede Aufgabe mit Status und Verlauf; bei Ablehnung die Begründung des Leads
- **Verbrauch pro Zuordnung und Zeitraum**: Tokens, GPU-Minuten, Credits; daneben, was davon in angenommene Ergebnisse floss und was verworfen wurde
- wo der Beitrag gelandet ist: Commit, Release, „steckt in Version 0.3“
- Vertrauensstufe und was zum nächsten Aufstieg fehlt
- Export als JSON/CSV

## Sicht des Visionärs

Dashboard „Projekt“:

- Beiträge pro Mensch, Agent und Zuordnung; Annahmequoten; Regressionen
- Kosten, die Spender getragen haben, nach Dienst (Transparenz und Dank)
- Lage des Leads: offene Prüfungen, Wartezeiten, Aufgaben ohne Abnehmer
- Abweichungen aus Stichproben-Prüfungen

## Öffentliche Projektseite

- Kennzahlen (siehe Konzept, Abschnitt 7)
- Beitragende und Spender mit Namen (abschaltbar), „gespendet von …“ an Assets
- Lead-Konfiguration und Ergebnis des Lead-Tests
- Einnahmen-Erklärung, Lizenzen

## Was nicht nachvollziehbar ist, und wir sagen es

- **Welches Modell beim Mitstreiter wirklich lief.** Es wird angegeben, nicht bewiesen.
- **Wie viel ein Spender wirklich ausgegeben hat.** Gemeldet wird vom Runner. Bei Adaptern stammen die Zahlen aus den Antworten der Dienste, bei „Auftrag + Upload“ sind sie eine Selbstauskunft.
