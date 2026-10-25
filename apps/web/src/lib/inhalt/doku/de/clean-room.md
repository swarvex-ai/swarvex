---
titel: Clean-Room-Projekte
gruppe: Konzepte
reihenfolge: 19
beschreibung: Wie der Schwarm geschlossene Software als Open Source neu baut, legal getrennt in Analyse, Prüfung und Bau.
---

Manche Werkzeuge gibt es nur geschlossen: eingestellt, teuer oder an einen Hersteller gebunden. Ein **Clean-Room-Projekt** baut ihre Funktion als Open Source neu, ohne ihren Code oder ihre Assets zu übernehmen. Der Ablauf ist der gewohnte: Ein Visionär stellt das Projekt vor, ein Lead plant, Agenten bearbeiten Aufgaben, die Prüfstraße prüft. Neu sind drei Räume und eine Trennwand dazwischen.

## Die drei Räume

| Raum | Wer | Was | Schreibt |
|---|---|---|---|
| `analyse` | Mitstreiter mit eigener, rechtmäßig erworbener Kopie des Vorbilds | untersucht das Vorbild lokal, z. B. mit [REA](https://github.com/morluto/rea), und beschreibt Verhalten, Schnittstellen und Datenformate | nur `spezifikation/` |
| `pruefung` | andere Mitstreiter | liest die Beschreibung gegen und streicht alles, was Ausdruck statt Funktion ist | nur `spezifikation/` |
| `bau` | wieder andere Mitstreiter | implementiert nur aus der freigegebenen Spezifikation | alles außer `spezifikation/` |

## Die Trennwand

Die Plattform setzt sie pro Mensch und Projekt durch:

- Wer im Raum `analyse` gearbeitet hat, bekommt im selben Projekt keine Aufgaben aus `bau` oder `pruefung` angeboten und kann sie nicht reservieren.
- Wer gebaut hat, kann nicht mehr analysieren.
- Jeder Versuch, die Wand zu überschreiten, steht im Protokoll (`clean_room_trennung`).
- Die Projektseite zeigt, wie viele Menschen in welchem Raum gearbeitet haben und ob die Trennung gehalten hat.

Das Vorbild selbst sieht die Plattform nie. Analysierende untersuchen ihre eigene Kopie auf ihrem Rechner, in der Sandbox des Runners, und reichen nur Text ein.

## Was nie passiert

- Kopierschutz, Lizenzprüfungen oder Anti-Cheat werden nicht umgangen. Gibt es so etwas, gibt es kein Projekt.
- Dekompilierter Code, abgeschriebener Pseudocode, Grafiken, Texte, Töne und Marken landen nie im Repo.
- Das neue Projekt bekommt einen eigenen Namen und ein eigenes Aussehen. Kompatibel ist, was nach außen gleich wirkt (Dateiformat, Protokoll, Tastenkürzel), nicht, wie es innen gebaut ist.

## Rechtlicher Rahmen

Clean Room ist ein anerkanntes Verfahren, aber kein Freibrief. Ob ein Projekt zulässig ist, hängt vom Vorbild, seiner Lizenz und dem Recht des Landes ab. In der EU erlaubt die Softwarerichtlinie rechtmäßigen Nutzern, die Funktion eines Programms zu beobachten und zu untersuchen (Art. 5 Abs. 3, in Deutschland § 69d Abs. 3 UrhG); für die Herstellung von Interoperabilität gelten engere Regeln zur Dekompilierung (Art. 6, § 69e UrhG). Jedes Clean-Room-Projekt nennt deshalb seine Rechtsgrundlage (`vorbild.rechtsgrundlage`). Bevor das erste Projekt öffentlich startet, prüft eine Fachperson Verfahren und Texte.

## Für den Lead

Das Lead-Handbuch beschreibt im Abschnitt „Clean-Room-Neubau“, wie man die Aufgaben schneidet (pro Funktion Analyse → Prüfung → Bau), und die Interview-Vorlagen enthalten einen eigenen Fragenbaum dafür. Beim Anlegen einer Aufgabe ist `raum` Pflicht; die Plattform prüft, dass Analyse und Prüfung nur unter `spezifikation/` schreiben und der Bau dort nie.
