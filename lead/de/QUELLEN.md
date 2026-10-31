# Quellen und Fremdlizenzen

## Projekt-Interview (LEAD.md, Phase 0)

Das Verfahren des Projekt-Interviews ist angelehnt an den Skill **„grilling“** (aufgerufen über `grill-me` bzw. `grill-with-docs`) aus [mattpocock/skills](https://github.com/mattpocock/skills), `skills/productivity/grilling/SKILL.md`. Übernommen wurden die Ideen:

- Entscheidungsbaum
- Fragen in Runden über die „Front“
- nummerierte Fragen mit Empfehlung, bei denen „ja“ die Empfehlung annimmt
- Fakten sucht der Agent, Entscheidungen trifft der Mensch
- Ende bei leerer Front plus Bestätigung

Neu für swarvex sind:

- die Startbäume je Projektart mit Pflicht- und späteren Zweigen
- die drei Zustände im Projektbrief
- das Limit pro Runde
- die kleinen Interviews unterwegs

```
MIT License

Copyright (c) 2026 Matt Pocock

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Engine-Referenz und Spielregeln

Aus [Claude Code Game Studios](https://github.com/Donchitos/Claude-Code-Game-Studios) (Commit `05ad7c8`):

- `lead/engine-reference/godot/`: fast unverändert übernommen aus `docs/engine-reference/godot/` (Änderungen im README dort), Englisch
- Ideen, in eigenen Worten neu geschrieben: Inhalte als Daten, Delta-Zeit und keine Singletons für Spielzustand (`vorlagen/AGENTS.md`); ein Regressionstest muss einmal rot gesehen werden (LEAD.md 6.5); das Feld Aufgabentyp (`vorlagen/aufgabe.md`); die Vorlagen Playtest-Bericht und Styleguide; die wiederkehrenden Prüfungen in LEAD.md 7.1
- Rollenkarten (`lead/roles/game/`, Commit `05ad7c8`): die Idee einer Studio-Hierarchie mit einer Rolle und einem Dokument je Bereich (Art Bible, Sound Bible, UX-Spezifikation), Prüf-Gates vor der Asset-Produktion, Abgabe und Eskalation je Rolle sowie die Namenskonvention für Audio; neu geschrieben für einen Schwarm unabhängiger Mitstreiter (eine Karte je Bereich statt 49 Unteragenten, Aufgaben für Fremde statt Arbeit in einer Sitzung)

```
MIT License

Copyright (c) 2026 Donchitos

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
