# Clean-Room-Spezifikation: Aufbau und Formate

> Für Clean-Room-Projekte (LEAD.md Abschnitt 9, „Clean-Room-Nachbau“). Die Spezifikation ist das Einzige, was die Trennwand überquert: Analysierende schreiben sie, Prüfende geben sie frei, Bauende setzen sie um, Verifizierende prüfen den Bau gegen sie. Die Formate lehnen sich an die Pflichtenlisten (obligation ledgers) von REA und das Werteblatt (mechanics) von Craft Anything an (siehe `QUELLEN.md`). JSON-Felder bleiben englisch.

## Aufbau im Projekt-Repository

```
specification/
  README.md               index: the boundary of each milestone (which obligations it must close) and their status
  unknowns.md             open questions about the original's behaviour, each with what would settle it
  drafts/                 analysis output, not yet released (analysis room writes here)
    <area>/…              same layout as approved/
  approved/               released by the review room; the only thing the build room reads
    obligations/<area>.json
    values.json
    formats/<name>.md
    scenarios/<name>.json
    cases/<obligation-id>/  input files and expected outputs (made by analysts from their own files)
  checks/round-<n>.md     verification rounds: build compared with the original (analysis side writes)
```

Nie im Repository: das Vorbild selbst, seine Dateien, Screenshots oder Aufnahmen davon, dekompilierter Code, Pseudocode, Disassembly, Zeichenketten, Bezeichner, Kommentare, Texte, Grafiken oder Klänge des Vorbilds sowie Beleg-Bündel von REA oder Craft Anything. Sie bleiben auf dem Rechner der Analysierenden; die Spezifikation nennt sie nur über ihre lokale Beleg-ID.

## Pflichten (`approved/obligations/<area>.json`)

Eine Pflicht ist eine Aussage, die der Nachbau wahr machen muss, in eigenen Worten, mit Testfällen, die das belegen.

```json
{
  "area": "save-format",
  "obligations": [
    {
      "id": "save.header.version",
      "version": 1,
      "title": "A save file starts with a header that names the format version",
      "layer": "file-format",
      "required": true,
      "statement": "Bytes 0–3 hold the ASCII letters of the format tag, bytes 4–5 the version as an unsigned 16-bit little-endian integer. Files with an unknown version are refused with an error, not opened.",
      "authority": "observed",
      "evidence": ["local ev_7f2c: header of three save files I created with version 2.3 of the original"],
      "cases": ["cases/save.header.version/"],
      "depends_on": [],
      "status": "approved"
    }
  ]
}
```

| Feld | Bedeutung |
|---|---|
| `id`, `version` | stabile ID (`<bereich>.<ding>.<detail>`) und eine Version, die steigt, wenn sich die Aussage ändert |
| `layer` | `file-format`, `protocol`, `cli`, `api`, `ui-flow`, `behaviour`, `value`, `performance` |
| `required` | für die Grenze des Meilensteins erforderlich oder optional |
| `statement` | was gelten muss, von außen beobachtbar, in eigenen Worten; nie Code-Struktur oder Namen des Vorbilds |
| `authority` | `observed` (direkt gesehen), `derived` (aus Beobachtungen berechnet), `documented` (öffentliche Doku oder Norm), `estimated` (geschätzt; mit Begründung) |
| `evidence` | woher es stammt: lokale Beleg-IDs und wie sie entstanden sind. Nicht der Beleg selbst |
| `cases` | Testfälle: Eingabe und erwartete Ausgabe, von den Analysierenden aus eigenen Dateien oder Läufen erzeugt |
| `depends_on` | IDs der Pflichten, die diese voraussetzt |
| `status` | `draft`, `approved` (freigegeben), `built` (Testfälle grün in der Prüfstraße), `verified` (eine Prüfrunde hat es am Vorbild bestätigt) |

## Werteblatt (`approved/values.json`)

Jede Zahl und jede Einstellung, die das Verhalten prägt, damit Bauende nie raten:

```json
{ "values": [
  { "name": "autosave interval", "group": "saving", "value": 300, "unit": "s", "authority": "observed", "evidence": ["local ev_91aa: timed in 5 runs"] },
  { "name": "max undo steps", "group": "editing", "value": 100, "unit": "steps", "authority": "estimated", "evidence": ["stopped counting after 100 undos without limit"] }
] }
```

## Szenarien (`approved/scenarios/<name>.json`)

Ein Nutzungsablauf als Schritte und sichtbare Ergebnisse, beschrieben als Verhalten, nicht mit Texten oder Layout des Vorbilds:

```json
{ "scenario": "open-edit-save", "steps": [
  { "id": "open", "action": "open cases/save.header.version/v2.sav", "expect": "the document opens; the title shows the file name" },
  { "id": "edit", "action": "change the first value to 42", "expect": "the document is marked as changed" },
  { "id": "save", "action": "save", "expect": "the file is written; reopening shows 42; the header version is unchanged" }
] }
```

## Prüfrunden (`checks/round-<n>.md`)

Geschrieben von Verifizierenden (Analyse-Seite), nachdem sie einen Bau mit dem Vorbild verglichen haben:

```markdown
# Check round 3 (build 0.4.0, original 2.3)

| Obligation | Result | Difference, described as behaviour |
|---|---|---|
| save.header.version | pass | – |
| save.body.order | fail | the original keeps the order of entries after saving; the build sorts them |

New obligations: save.body.order-preserved (draft)
Coverage of the milestone boundary: 14 of 17 required obligations verified
```

Eine fehlgeschlagene Prüfung wird zu einem Pflicht-Entwurf (oder einem neuen Testfall für eine bestehende Pflicht), geht durch die Prüfung und dann in eine Bau-Aufgabe. Nie ein Code-Vorschlag.
