# Clean-room specification: layout and formats

> For clean-room projects (LEAD.md section 9, "Clean-room rebuild"). The specification is the only thing that crosses the wall: analysts write it, reviewers release it, builders implement it, verifiers check the build against it. Formats adapted from REA's reconstruction obligation ledgers and Craft Anything's mechanics sheet (see `SOURCES.md`).

## Layout in the project repository

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

Never in the repository: the original itself, its files, screenshots or recordings of it, decompiled code, pseudocode, disassembly, strings, identifiers, comments, texts, graphics or sounds of the original, and REA or Craft Anything evidence bundles. They stay on the analyst's machine; the specification cites them by local evidence ID only.

## Obligations (`approved/obligations/<area>.json`)

An obligation is one claim the rebuild must make true, in our own words, with cases that prove it.

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

| Field | Meaning |
|---|---|
| `id`, `version` | stable ID (`<area>.<thing>.<detail>`) and a version that increases when the statement changes |
| `layer` | `file-format`, `protocol`, `cli`, `api`, `ui-flow`, `behaviour`, `value`, `performance` |
| `required` | required for the milestone's boundary, or optional |
| `statement` | what must hold, observable from outside, in your own words; never code structure or the original's names |
| `authority` | `observed` (seen directly), `derived` (computed from observations), `documented` (public documentation or standard), `estimated` (guessed; say why) |
| `evidence` | where it comes from: local evidence IDs and how they were obtained. Not the evidence itself |
| `cases` | fixtures: input and expected output, created by the analyst from their own files or runs |
| `depends_on` | IDs of obligations this one needs |
| `status` | `draft`, `approved` (released), `built` (cases pass in the pipeline), `verified` (a check round confirmed it against the original) |

## Values sheet (`approved/values.json`)

Every number or setting that shapes behaviour, so builders never guess:

```json
{ "values": [
  { "name": "autosave interval", "group": "saving", "value": 300, "unit": "s", "authority": "observed", "evidence": ["local ev_91aa: timed in 5 runs"] },
  { "name": "max undo steps", "group": "editing", "value": 100, "unit": "steps", "authority": "estimated", "evidence": ["stopped counting after 100 undos without limit"] }
] }
```

## Scenarios (`approved/scenarios/<name>.json`)

A user journey as steps and visible results, written as behaviour, not as the original's texts or layout:

```json
{ "scenario": "open-edit-save", "steps": [
  { "id": "open", "action": "open cases/save.header.version/v2.sav", "expect": "the document opens; the title shows the file name" },
  { "id": "edit", "action": "change the first value to 42", "expect": "the document is marked as changed" },
  { "id": "save", "action": "save", "expect": "the file is written; reopening shows 42; the header version is unchanged" }
] }
```

## Check rounds (`checks/round-<n>.md`)

Written by verifiers (analysis side) after comparing a build with the original:

```markdown
# Check round 3 (build 0.4.0, original 2.3)

| Obligation | Result | Difference, described as behaviour |
|---|---|---|
| save.header.version | pass | – |
| save.body.order | fail | the original keeps the order of entries after saving; the build sorts them |

New obligations: save.body.order-preserved (draft)
Coverage of the milestone boundary: 14 of 17 required obligations verified
```

A failed check becomes a draft obligation (or a new case for an existing one), goes through review and then into a build task. Never a code suggestion.
