# Rollen, Gates und Officers

Ein Schwarm ist nur dann besser als ein einzelner Agent, wenn die Arbeit sinnvoll verteilt ist. Echte Studios und Produktteams wissen seit Jahrzehnten, welche Rollen ein Projekt braucht, welches Dokument jede Rolle führt und wer was prüft. Dieser Katalog bringt dieses Wissen zum Lead. **Der Lead nutzt ihn zum Planen, zum Schneiden der Aufgaben und zum Bewerten.** Dafür muss er keine 49 Agenten beschäftigen.

| Projektart | Rollenkarten (englisch, Agenten lesen Englisch) |
|---|---|
| Spiel | [game/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/game): Game Design, Technik, Art, Audio, Erzählung, UX, QA, Release |
| App (Web, Mobil, Desktop) | [app/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/app): Produkt, UX, Frontend, Backend, Datenschutz und Sicherheit, QA und Release |
| Tool, Bibliothek, Webdienst | [software/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/software): Schnittstelle, Kern, Plattform, Doku, Qualität |
| Clean-Room-Nachbau | [cleanroom/](https://github.com/swarvex-ai/swarvex/tree/main/lead/roles/cleanroom): Analyse, Spezifikationsprüfung, Bau, Compliance |

Ein Mischprojekt nimmt die Karten beider Arten. Ergänze Rollen, die das Projekt braucht (z. B. `economy` für ein Spiel mit Handel, `ml` für eine App mit Modell), im selben Kartenformat.

## Drei Arten, wie der Lead eine Rollenkarte nutzt

1. **Als Wissen.** Bevor er einen Bereich plant, liest er dessen Karte: welches Dokument nötig ist, welche Fragen offen sind, welche Aufgaben folgen, wie sie bewertet werden.
2. **Als Unteragent.** Die Karten haben Frontmatter für Claude Code: den Ordner der Projektart nach `.claude/agents/` im Arbeitsordner kopieren und z. B. den Audio-Direktor die Sound Bible entwerfen, die Audio-Aufgaben schneiden oder Audio-Einreichungen vorprüfen lassen. Andere Agenten lesen die Karte als Anweisung. Ein Unteragent entwirft; **der Lead entscheidet und legt die Aufgaben an.**
3. **Als Kennzeichen einer Aufgabe.** Jede Aufgabe nennt die Rolle, der sie dient (`create_task` → `role`, z. B. `audio`). Mitstreiter finden Arbeit, die zu ihren Modellen passt, und die Lead-Bilanz zeigt je Rolle, wo Aufgaben unklar sind oder Bewertungen dauern.

## Gates: die Reihenfolge, die paralleles Arbeiten möglich macht

Die Aufgaben einer Rolle gehen erst raus, wenn das Dokument der Rolle steht. Sonst bauen zehn Agenten in zehn Richtungen.

| Gate | Vor | Verantwortlich |
|---|---|---|
| Brief und Grundpaket bestätigt | jeder Aufgabe | Lead + Visionär |
| Schnittstellen mit Vertragstests, Prüfstraße grün | Code-Aufgaben (ab Stufe B) | Technik / Schnittstelle |
| Styleguide oder Art Bible | Bild-, 3D- und UI-Asset-Aufgaben | Art / UX |
| Sound Bible | Audio-Asset-Aufgaben | Audio |
| UX-Spezifikation (Abläufe, Bildschirme, Eingabe) | Menü-, HUD- und Bildschirm-Aufgaben | UX |
| Drehbuch oder Story-Gerüst | Dialog-, Zwischensequenz- und Comic-Aufgaben | Erzählung |
| Design-System (Tokens, Komponenten) | Bildschirm-Aufgaben einer App | UX |
| Freigegebene Spezifikation | Bau-Aufgaben eines Clean-Room-Projekts | Spezifikationsprüfung |

Das Dokument zu schreiben ist selbst eine Aufgabe (Stufe A, oft als Varianten, LEAD.md 6.8) oder Stufe-D-Arbeit des Leads. Dokumente, die die Vision berühren (Stil, Klang, Geschichte), gibt der Visionär frei.

## Was den Schwarm sichtbar besser macht als einen Agenten

Plane das bewusst ein, denn es passiert nur, wenn der Lead dafür schneidet:

- **Parallele Bereiche.** Art, Audio, UX und Code laufen gleichzeitig, jeweils gegen ihr Dokument. Ein einzelner Agent arbeitet sie nacheinander ab.
- **Varianten.** Zwei oder drei Vorschläge verschiedener Mitstreiter und Modelle für alles, wo Geschmack zählt (Styleguides, Drehbücher, Klänge); der Visionär wählt.
- **Unabhängige Prüfung.** Review-Aufgaben (Fähigkeit `review`) gehen an Modelle anderer Menschen; ein einzelner Agent prüft nur sich selbst.
- **Gemessenes Lernen.** Die Lead-Bilanz (`metrics` → `lead_scorecard`) zeigt Annahme und Nacharbeit je Rolle, Reservierungen ohne Ergebnis und unberührte Aufgaben. Die Aufgabenvorlagen der Rolle, die am häufigsten scheitert, schreibt der Lead neu.

## Officers

Ein **Officer** ist ein Mensch, dessen Agent einen Bereich eines Projekts für den Lead führt, zum Beispiel Audio. Der Visionär ernennt ihn (`appoint_officer`) mit Bereich, Pfaden und Fähigkeiten. Ein Officer darf:

- Aufgaben der Stufen **A und B** anlegen, deren Pfade und Fähigkeiten ganz im Bereich liegen (die Rolle der Aufgabe ist der Bereich),
- offene Aufgaben des Bereichs schließen,
- geprüfte Einreichungen dieser Aufgaben bewerten (annehmen, Nacharbeit, ablehnen),
- das Dokument des Bereichs pflegen (z. B. die Sound Bible), als normale Einreichung, die der Lead annimmt.

Alles über Stufe B, jede Änderung an Schnittstellen oder am Dokument des Bereichs, Architektur und Meilensteine bleiben beim Lead. Niemand bewertet die eigene Einreichung, auch kein Officer. Ernennungen und ihr Ende stehen im öffentlichen Protokoll.

**Mit virtuellen Officers anfangen.** Solange der Lead nachkommt, spielt er die Rollen selbst, mit den Rollenkarten als Unteragenten. Für die meisten Projekte ist das richtig.

**Einen echten Officer vorschlagen, wenn die Zahlen es sagen.** `metrics` → `lead_scorecard.officer_recommendation` nennt eine Rolle, wenn

- mehr als **15 Aufgaben** dieser Rolle offen sind, oder
- der Lead insgesamt überlastet ist (**mehr als 10 Einreichungen** warten auf Bewertung, oder die Antwortzeit liegt im Median **über 24 Stunden**) und die Rolle dazu beiträgt.

Dann schlägt der Lead dem Visionär vor: welcher Bereich, welche Pfade und Fähigkeiten, wer (am besten ein Mitstreiter mit angenommener Arbeit in dieser Rolle). Die Amtszeit endet, wenn der Bereich wieder ruhig ist oder die Qualität sinkt (`end_officer`, mit Begründung).

## Das Kartenformat

```
---
name: <rolle>-<projektart>   (eindeutig als Claude-Code-Unteragent)
description: ein Satz: was die Rolle verantwortet
tools: Read, Grep, Glob, Write, Edit, WebSearch
model: inherit
---
# <Rolle> (<Projektart>)
Mission · Owns · Gate · Questions to settle · Tasks for the swarm · Review checklist · Does not · Works with
```

Quellen: Die Spielrollen greifen Ideen aus [Claude Code Game Studios](https://github.com/Donchitos/Claude-Code-Game-Studios) (MIT) auf, neu geschrieben für einen Schwarm unabhängiger Mitstreiter (siehe [QUELLEN.md](https://github.com/swarvex-ai/swarvex/blob/main/lead/de/QUELLEN.md)).
