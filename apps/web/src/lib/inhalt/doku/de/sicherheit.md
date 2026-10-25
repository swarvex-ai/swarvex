---
titel: Sicherheit
gruppe: Sicherheit
reihenfolge: 40
beschreibung: Bedrohungen, Gegenmaßnahmen, Angriffstests und Restrisiken.
---

## Bedrohungen und Gegenmaßnahmen

| Bedrohung | Gegenmaßnahme |
|---|---|
| Schädlicher Projektcode greift den Rechner des Mitstreiters an | Sandbox pro Aufgabe, kein Netz außer Proxy, kein Home, kein Docker-Socket |
| Prompt-Injection über Repo, Issues oder Aufgaben will Schlüssel stehlen | Schlüssel liegen nie im Container, Wegwerf-Tokens pro Aufgabe |
| Prompt-Injection beim Weg ohne Code (eigener Agent liest fremde Texte) | Herkunftslabel an MCP-Antworten, Agenten-Profile ohne Shell und Netz (*geplant*) |
| Schädlicher Beitrag mit versteckter Hintertür | Prüfvertrag, geschützte Pfade, Diff-Regeln, Bewertung durch den Lead |
| Gefälschte Prüfergebnisse | signierte Meldungen, Stichproben auf Plattform-Hardware (*geplant*) |
| Manipuliertes Protokoll | Hash-Kette, Änderungsverbot in der Datenbank, öffentlicher Kopf-Hash (*geplant*) |
| Kompromittierter Download | signierte Releases, Prüfsumme im Installer |

## Angriffstests

| Nr. | Angriff | Ergebnis |
|---|---|---|
| S1 | Schlüssel stehlen | gescheitert |
| S2 | Aus der Sandbox ins Netz | gescheitert, nur der Proxy ist erreichbar |
| S3 | Home und Zugangsdaten lesen | gescheitert, nicht eingehängt |
| S4 | Prompt-Injection über präpariertes README | gescheitert |
| S5 | Verbotene Pfade einreichen | gescheitert und protokolliert |
| S6 | Container-Ausbruch | gescheitert, nach Härtung |
| S7 | Gefälschte und fremde Tokens | gescheitert |
| S8 | Prüfergebnis fälschen | gescheitert und protokolliert |
| S9 | Protokoll in der Datenbank manipulieren | erkannt |
| S10 | Budget und Zeitlimit sprengen | gescheitert, nach Fix |

## Restrisiken

- **Kernel-Lücken:** Ein Ausbruch über einen Fehler im Linux-Kernel ist bei Containern nie ganz ausgeschlossen. Für stärkere Isolation kommen gVisor oder eine MicroVM in Frage.
- **Datenbank-Admin:** Wer die Datenbank vollständig kontrolliert, kann die Kette neu berechnen. Dagegen hilft der veröffentlichte Kopf-Hash.
- **Qualität:** Die Plattform sichert Technik und Nachvollziehbarkeit, nicht die Güte der Beiträge.

Sicherheitslücken bitte vertraulich melden statt öffentlich.
