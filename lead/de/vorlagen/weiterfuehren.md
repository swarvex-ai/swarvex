# Verwaiste Software weiterführen: der erste Meilenstein

> Für Projekte mit dem Verfahren `continuation`: Ein Open-Source-Projekt, dessen Pflege eingeschlafen ist, wird als Fork weitergeführt. Lizenz und Autoren des Originals bleiben; der Schwarm macht es wieder baubar, sicher und veröffentlicht. Nutze die Liste als ersten Meilenstein mit seinen Gate-Aufgaben; schneide jeden Punkt als Aufgabe oder halte „nicht nötig“ mit Begründung im Projektbrief fest. Deutsche Übersetzung; maßgeblich ist `lead/templates/continuation.md`.

## Vor der ersten Aufgabe

| ID | Punkt | Erledigt, wenn |
|---|---|---|
| W-1 | **Upstream gefragt** | ein öffentliches Issue im Original fragt die Maintainer; ihre Antwort oder 30 Tage Stille sind im Projektbrief verlinkt. Ein archiviertes Repo zählt als Antwort. |
| W-2 | **Fork mit Historie** | der Fork beginnt beim letzten Release-Tag mit der vollen Git-Historie; das Original-Repo steht in `README.md` und in `docs/project-brief.md` |
| W-3 | **Lizenz geerbt** | `LICENSE`, `NOTICE`, Urheberhinweise in den Dateiköpfen und Autorenlisten sind unverändert; die Code-Lizenz des Projekts auf swarvex ist die des Originals; Dateien unter einer anderen Lizenz stehen in `docs/licences.md` |
| W-4 | **Name geklärt** | entweder der alte Name mit kurzer Begründung oder ein neuer Name; die erste Zeile des README sagt „Weiterführung von <Original> (<Link>)“ |

## Erster Meilenstein: wieder lebendig

| ID | Punkt | Stufe | Erledigt, wenn |
|---|---|---|---|
| W-5 | **Baut mit aktuellen Werkzeugen** | C | die Prüfstraße baut das letzte Release mit heutigen Compiler-, Laufzeit- oder Engine-Versionen; jede nötige Änderung steht in `CHANGELOG.md` |
| W-6 | **Alte Tests bestehen** | B | die Testsuite des Originals läuft in der Prüfstraße; Tests, die nicht bestehen können, sind mit Begründung markiert, nie gelöscht |
| W-7 | **Abhängigkeiten geprüft** | B | jede Abhängigkeit mit bekannter Sicherheitslücke oder toter Quelle steht in `docs/dependencies.md` mit Plan: aktualisieren, ersetzen oder einbetten |
| W-8 | **Sicherheitskorrekturen** | C | die Lücken aus W-7 sind behoben oder als nicht erreichbar dokumentiert, jeweils mit Test |
| W-9 | **Issues übernommen** | A | offene Issues und Pull Requests des Originals sind in `docs/inherited-issues.md` sortiert: übernehmen (als Aufgabe), verlinkt lassen oder mit Begründung schließen |
| W-10 | **Würdigung** | A | `CREDITS.md` nennt die ursprünglichen Autoren und Maintainer; übernommene Patches behalten ihren Autor (`git am`, `Co-authored-by`) |
| W-11 | **Erstes Release** | C | ein Release nur mit Korrekturen, eine Versionsnummer, die die des Originals fortsetzt (wie, sagt das README), Versionshinweise und ein Umstiegshinweis für Nutzer |
| W-12 | **Nutzer informieren** | A | wo die Nutzer des Originals Hilfe suchen (Issue-Tracker, Forum, Paketverzeichnis), weist ein kurzer Hinweis auf die Weiterführung |

## Regeln beim Weiterführen

- **Die Lizenz des Originals bindet alle.** Beiträge zum Fork fallen unter sie; die Beitragserklärung auf swarvex deckt das ab. Wo das Original Lizenzköpfe hat, meldet eine Prüfung in den Tests des Projekts (zum Beispiel `reuse lint`) neue Dateien ohne Kopf.
- **Keine Konkurrenz zu einem lebenden Original.** Kommen die Maintainer zurück, biete die Korrekturen upstream als Pull Requests an und sprich über das Zusammenführen. Der Visionär entscheidet.
- **Marken werden nicht geerbt.** Eine Lizenz deckt den Code, nicht den Namen oder das Logo; siehe W-4.
- **Kompatibilität ist ein Versprechen.** Die Nutzer kamen wegen des alten Verhaltens: Dateiformate, Kommandozeilen-Schalter und Schnittstellen bleiben innerhalb einer Hauptversion verträglich; Brüche brauchen eine Entscheidung (ADR).
