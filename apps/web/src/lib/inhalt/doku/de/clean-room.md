---
titel: Clean-Room-Projekte
gruppe: Konzepte
reihenfolge: 19
beschreibung: Wie der Schwarm geschlossene Software als Open Source neu baut: studieren, freigeben, bauen und prüfen, getrennt in Räume.
---

Manche Werkzeuge gibt es nur geschlossen: eingestellt, teuer oder an einen Hersteller gebunden. Ein **Clean-Room-Projekt** baut ihre Funktion als Open Source neu, ohne ihren Code oder ihre Assets zu übernehmen. Der Ablauf ist der gewohnte: Ein Visionär stellt das Projekt vor, ein Lead plant, Agenten bearbeiten Aufgaben, die Prüfstraße prüft. Neu sind Räume, eine Trennwand dazwischen und eine Spezifikation, die als Einziges die Wand überquert.

Werkzeuge wie [Craft Anything](https://github.com/storytold/craft-anything) und [REA](https://github.com/morluto/rea) zeigen das Verfahren für einen einzelnen Agenten: das Vorbild studieren, bauen, gegen das Vorbild prüfen, nachbessern. swarvex fährt dieselbe Schleife mit **getrennten Menschen**: Wer das Vorbild studiert hat, baut nie, und wer baut, sieht das Vorbild nie.

## Die Schleife über die Räume

| Schritt | Raum | Wer | Schreibt |
|---|---|---|---|
| Studieren | `analysis` | Mitstreiter mit eigener, rechtmäßig erworbener Kopie | Entwürfe unter `specification/drafts/`: Pflichten mit Testfällen, Werte, Formate, Szenarien |
| Freigeben | `review` | andere Mitstreiter | alles streichen, was Ausdruck ist, nach `specification/approved/` freigeben |
| Bauen | `build` | wieder andere Mitstreiter | Code, der die freigegebenen Testfälle erfüllt; nie `specification/` |
| Prüfen | `analysis` | Verifizierende von der Analyse-Seite | `specification/checks/`: der Bau mit dem Vorbild verglichen; Abweichungen werden neue Entwürfe |

Im Mittelpunkt der Spezifikation steht die **Pflicht**: eine Aussage in eigenen Worten („eine Speicherdatei beginnt mit einem Kopf, der die Formatversion nennt“), mit Eingabedateien und erwarteter Ausgabe und einer Angabe, woher sie bekannt ist: beobachtet, abgeleitet, dokumentiert oder geschätzt. Ein Meilenstein ist fertig, wenn alle seine erforderlichen Pflichten am Vorbild bestätigt sind. Das Format steht im Lead-Handbuch (`lead/de/vorlagen/clean-room-spezifikation.md`).

## Zwei Tiefen der Analyse

- **observe** (Standard): nur, was ein Nutzer von außen sieht: das Programm benutzen, Szenarien aufzeichnen, messen und Dateien, die die Analysierenden selbst damit erzeugt haben. So arbeiten die Aufnahme-Werkzeuge von REA und Craft Anything im Clean-room-Modus.
- **inspect**: hineinsehen (dekompilieren mit REA oder Ghidra), nur für Interoperabilität, wo Beobachtung nicht reicht (Dateiformate, Protokolle), nur wo das Interview des Projekts es erlaubt hat und nur für Mitstreiter mit höherer Vertrauensstufe. Das Ergebnis ist auch dann nur eine Beschreibung des Verhaltens.

Der Deep-Modus von Craft Anything und ein Nachbau durch dieselbe Person werden für swarvex-Ergebnisse nie verwendet.


Die Tiefe legt der Visionär beim Anlegen im Assistenten fest; sie ist die Obergrenze für alle Analyse-Aufgaben des Projekts. Ein Projekt, das nur beobachtet, bekommt nie Aufgaben zum Hineinsehen.

## Werkzeuge für die Analyse

Der Lead eines Clean-Room-Projekts bekommt einen Werkzeugkasten (`lead/roles/cleanroom/toolkit.md`), sortiert nach der Art des Originals und der Tiefe. [REA](https://github.com/morluto/rea) ist der Einstieg: Es bindet die meisten Werkzeuge selbst ein und liefert jedes Ergebnis als Beleg mit Herkunft, genau das, was die Pflichten und Prüfrunden brauchen. Eine Erklärung eines Agenten ist ein Hinweis, kein Beleg.

| Original | observe | inspect (wenn erlaubt) |
|---|---|---|
| Windows-Programm | benutzen und mitschneiden, [Detect It Easy](https://github.com/horsicq/Detect-It-Easy) („womit gebaut?“) | [Ghidra](https://github.com/NationalSecurityAgency/ghidra) über REA, [mcp-windbg](https://github.com/svnscha/mcp-windbg) für Laufzeitzustand |
| Linux- oder macOS-Programm | Prozess- und Oberflächen-Mitschnitte | Ghidra über REA, [pwndbg](https://github.com/pwndbg/pwndbg) |
| Android-App | Emulator, Bildschirm- und Log-Mitschnitte | [JADX](https://github.com/skylot/jadx) und [Apktool](https://github.com/iBotPeaches/Apktool) über REA |
| Web- oder Electron-App | Browser-Szenarien, Netzwerk-Mitschnitte | ausgelieferte Bundles über REA |
| Spiel | spielen, aufzeichnen, eigene Spielstände, gemessene Werte | Unity- und Godot-Pakete über REA, Ghidra |
| Firmware oder Gerät | sichtbares Verhalten, Protokoll auf der Leitung | [unblob](https://github.com/onekey-sec/unblob), dann [Binwalk](https://github.com/ReFirmLabs/binwalk) über REA |
| Dateiformat oder Protokoll | viele selbst erzeugte Dateien, je eine Einstellung verändert | Lese- und Schreibcode nur für offene Details |

Alles läuft nur auf dem Rechner der Analysierenden, nie in der Sandbox des Runners und nie im Repo. Nicht verwendet werden Exploit-Werkzeuge, Kernel-Debugger und alles, was Schutzmaßnahmen umgeht oder Packer entpackt.

## Die Trennwand

Die Plattform setzt sie pro Mensch und Projekt durch:

- Wer in `analysis` gearbeitet hat (auch beim Prüfen), bekommt im selben Projekt keine Aufgaben aus `build` oder `review` angeboten und kann sie nicht reservieren. Wer gebaut hat, kann nicht mehr analysieren.
- Officers gehören zu einem Raum; ihr Amt zählt für die Trennwand wie eine Reservierung.
- Bauende bekommen nur die freigegebene Spezifikation: Entwürfe, Prüfrunden und offene Fragen fehlen in ihrem Arbeitsstand.
- Jeder Versuch, die Wand zu überschreiten, steht im Protokoll (`clean_room_separation`). Die Projektseite zeigt, wie viele Menschen in welchem Raum gearbeitet haben und ob die Trennung gehalten hat.

Die Plattform sieht das Vorbild selbst nie. Analysierende untersuchen ihre eigene Kopie auf ihrem eigenen Rechner und reichen nur Text ein. Ihre Belege (Bündel von REA oder Craft Anything, Screenshots, Aufnahmen) bleiben bei ihnen; die Spezifikation nennt sie nur über eine lokale ID.

## Was nie passiert

- Kopierschutz, Lizenzprüfungen oder Anti-Cheat werden nicht umgangen. Gibt es sie, gibt es kein Projekt.
- Dekompilierter Code, abgeschriebener Pseudocode, Zeichenketten, Bezeichner, Grafiken, Texte, Klänge, Screenshots und Marken des Vorbilds kommen nie ins Repo.
- Das neue Projekt bekommt einen eigenen Namen und ein eigenes Aussehen. Kompatibel heißt, was von außen gleich wirkt (Dateiformat, Protokoll, Tastenkürzel, Arbeitsabläufe), nicht wie es innen gebaut ist oder aussieht.

## Rechtlicher Rahmen

Clean Room ist ein anerkanntes Verfahren, kein Freibrief. Ob ein Projekt zulässig ist, hängt vom Vorbild, seiner Lizenz und dem Recht des Landes ab. In der EU erlaubt die Software-Richtlinie rechtmäßigen Nutzern, die Funktionsweise eines Programms zu beobachten und zu untersuchen (Art. 5 Abs. 3, in Deutschland § 69d Abs. 3 UrhG); für das Dekompilieren zur Interoperabilität gelten engere Regeln (Art. 6, § 69e UrhG). Jedes Clean-Room-Projekt nennt deshalb, wie das Original rechtmäßig erworben wurde (`original.acquisition`); wer hineinsehen will (`original.depth` inspect), nennt zusätzlich die Rechtsgrundlage (`original.legal_basis`) und welche Teile von innen angesehen werden dürfen (`original.inspect`). Bevor das erste Projekt öffentlich wird, prüft eine Fachperson das Verfahren und die Texte.

## Für den Lead

Das Lead-Handbuch beschreibt die Schleife unter „Clean-Room-Nachbau“, die Rollenkarten in `lead/roles/cleanroom/` (Analyse, Spezifikationsprüfung, Bau, Verifikation, Compliance) geben jeder Rolle ihre Aufgaben, ihre Prüfliste und ihr Officer-Mandat, und das Interview hat einen eigenen Fragenbaum. Beim Anlegen einer Aufgabe ist `room` Pflicht; die Plattform prüft, dass Analyse und Prüfung nur unter `specification/` schreiben und der Bau nie.
