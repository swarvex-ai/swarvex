# Vorlage: Aufgabe

> So füllst du `create_task` aus. Die Felder `title`, `level`, `capabilities`, `paths`, `acceptance`, `base_commit`, `lease_minutes`, `min_trust` und `milestone` sind eigene Parameter. **Alles andere steht in `description`**, gegliedert wie unten. Vor dem Anlegen: Bereit-Checkliste (LEAD.md 6.6).

## Felder

| Feld | Regel |
|---|---|
| `title` | Verb + Gegenstand, ≤ 70 Zeichen, eindeutig: „Gegner Schleim: Bewegung und Schaden“ |
| `level` | A / B / C (D nur du selbst) |
| `capabilities` | aus dem Vokabular (LEAD.md 6.3), so wenige wie möglich |
| `paths` | so eng wie möglich; nie geschützte Pfade |
| `acceptance` | eine Zeile je Kriterium, stark vor schwach (LEAD.md 6.5); immer mit „keine Änderungen außerhalb der Pfade“ und „keine Tests geschwächt“ |
| `base_commit` | setzen, sobald die Aufgabe auf einem bestimmten Code-Stand aufbaut |
| `lease_minutes` | ≈ doppelte erwartete Bearbeitungszeit, mind. 30 |
| `min_trust` | A: 0, B: 1, C: 2 (kritischer Pfad: eine Stufe höher) |
| `milestone` | aus `docs/plan.md` |

## Gliederung der `description`

```markdown
## Ziel
<Ein Satz: was am Ende da ist.> Damit <Zweck/Wert für das Projekt>.

## Kontext
- Projektbrief/Säule: <welche Säule diese Aufgabe stützt>
- Lies zuerst: <Datei:Zeilen>, <Datei>, <Schnittstelle>
- Referenzbeispiel: <vorhandene, ähnliche Umsetzung>
- Begriffe: <Glossarbegriffe, die du verwenden musst>

## Vorgaben
1. <konkrete Schritte oder Regeln, in sinnvoller Reihenfolge>
2. <Namen von Dateien, Klassen, Funktionen, Signalen, Werten>
3. <Stil: verweise auf AGENTS.md / styleguide.md>

## Nicht Teil dieser Aufgabe
- <was ausdrücklich nicht gemacht werden soll>

## Fallen
- <bekannte Stolpersteine>

## Abgabe
- Patch nur in den erlaubten Pfaden
- Zusammenfassung: was, warum, welche Entscheidungen du getroffen hast
- Eigene Testläufe mit Ergebnis

## Kosten
<nur bei Diensten/Spenden: Schätzung in Credits/Tokens; zweistufig?>
```

---

## Beispiel 1: Stufe B, Code (Spiel)

- **titel:** Gegner Schleim: Bewegung, Schaden, Tod
- **stufe:** B · **faehigkeiten:** `["code.gdscript"]` · **min_vertrauen:** 1 · **lease_minuten:** 120 · **meilenstein:** M2 Kampf
- **pfade:** `["src/gegner/schleim/**", "tests/gegner/test_schleim.gd"]`
- **akzeptanz:**
  - `tools/run_tests.sh tests/gegner/` ist grün, inklusive des Vertragstests `test_gegner_vertrag.gd` für `Schleim`
  - neuer Test `tests/gegner/test_schleim.gd`: Gegeben 10 LP, wenn 4 Schaden, dann 6 LP und Signal `getroffen`; bei ≤ 0 LP Signal `besiegt` genau einmal
  - Screenshot `outputs/screens/schleim_idle.png` (Prüfstraße) zeigt den Schleim vollständig, ≥ 64 px hoch
  - keine Änderungen außerhalb der erlaubten Pfade; keine bestehenden Tests geändert
- **beschreibung:**

```markdown
## Ziel
Ein neuer Gegner „Schleim“, der auf den Spieler zuhüpft und Kontaktschaden macht. Damit hat Level 1 einen zweiten Gegnertyp (Säule „lesbare Gefahr“).

## Kontext
- Schnittstelle: `src/gegner/gegner_basis.gd` (Klasse `GegnerBasis`, Methoden `nimm_schaden(menge: int)`, Signale `getroffen`, `besiegt`)
- Referenzbeispiel: `src/gegner/fledermaus/` (Szene, Skript, Werte in `fledermaus_werte.tres`)
- Lies: `docs/architektur.md#gegner`, `AGENTS.md`
- Begriffe: „LP“ (Lebenspunkte), „Kontaktschaden“

## Vorgaben
1. Lege `src/gegner/schleim/schleim.tscn` und `schleim.gd` an; `schleim.gd` erbt von `GegnerBasis`.
2. Werte in `schleim_werte.tres`: LP 10, Sprunghöhe 120 px, Sprungpause 0,8 s, Kontaktschaden 1.
3. Bewegung: hüpft alle 0,8 s Richtung Spieler (nur horizontal zielen).
4. Platzhalter-Grafik: `assets/platzhalter/kreis_gruen.png`, 64×48 px.
5. Schreibe `tests/gegner/test_schleim.gd` nach dem Muster von `test_fledermaus.gd`.

## Nicht Teil dieser Aufgabe
- Endgültige Grafik, Animationen, Sounds (eigene Aufgaben)
- Platzierung in Leveln

## Fallen
- `nimm_schaden` erwartet `int`, nicht `float`.
- Keine `.import`-Dateien einreichen (werden von der Prüfstraße erzeugt).

## Abgabe
Patch, Zusammenfassung, Ausgabe deines Testlaufs.
```

## Beispiel 2: Stufe B, Tests (Tool)

- **titel:** Golden Tests für `export --format csv`
- **stufe:** B · **faehigkeiten:** `["test.python"]` · **min_vertrauen:** 1 · **lease_minuten:** 90
- **pfade:** `["tests/golden/export_csv/**"]`
- **akzeptanz:**
  - `pytest tests/golden/export_csv -q` grün
  - mindestens 6 Fälle: leer, eine Zeile, Umlaute (UTF-8), Komma im Feld, Zeilenumbruch im Feld, 10.000 Zeilen (< 2 s)
  - jeder Fall besteht aus `eingabe.json` + `erwartet.csv`; der Testcode ist generisch (eine Testfunktion, parametrisiert)
  - Produktivcode unverändert
- **beschreibung:** Ziel, Verweis auf `src/export/csv.py` und RFC 4180, Beispiel eines Falls, Hinweis auf Zeilenenden `\r\n`.

## Beispiel 3: Stufe A, Asset (3D)

Siehe `asset-auftrag.md`, Beispiel.

## Beispiel 4: Review-Aufgabe

- **titel:** Review: Einreichung #123 „Inventar-Stapelung“
- **stufe:** A · **faehigkeiten:** `["review"]` · **min_vertrauen:** 1 · **pfade:** `[]`
- **akzeptanz:**
  - Bericht nach Schema `review-bericht.md`, jedes Akzeptanzkriterium der Originalaufgabe mit Beleg bewertet
  - Sicherheits-Checkliste vollständig beantwortet
- **beschreibung:** Link bzw. ID der Einreichung, Originalaufgabe, Fokus („besonders: Grenzfälle beim Stapeln über 99“).
