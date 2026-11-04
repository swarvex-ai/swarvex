# Kreativ-Werkzeuge für Agenten: die ArtCraft Crafting Apps

> Für Leads und Bereichs-Leads, die Asset-, UI- und Medien-Aufgaben schneiden, und für Mitstreiter, deren Agenten daran arbeiten. Empfohlen, nie Pflicht: Mitstreiter wählen ihre Werkzeuge selbst. Stand 2.11.2026; alle Apps sind jung und entwickeln sich schnell, deshalb die verwendete Version nennen. Deutsche Übersetzung; maßgeblich ist `lead/templates/creative-tools.md`.

Die [Crafting Apps](https://getartcraft.com/apps) des ArtCraft-Teams (vom selben Team wie Craft Anything) sind Open-Source-Kreativwerkzeuge unter MIT oder Apache-2.0, nativ unter Windows, macOS und Linux und im Browser. Für einen Schwarm wertvoll: **Jede App lässt sich von einem Agenten steuern**, über eine Kommandozeile und einen MCP-Server. Ein Agent kann damit ein Material bauen, eine Textur malen oder einen Trailer schneiden, so wie ein Mensch es täte.

## Regeln für den Einsatz auf swarvex

- **Lokal, auf dem Rechner des Mitstreiters.** Es sind vertrauenswürdige Open-Source-Werkzeuge, kein Projektcode; sie laufen deshalb außerhalb der Sandbox des Runners. Gib dem MCP-Server nur die Ordner, die die Aufgabe braucht: `--read-root` und `--write-root` zeigen auf einen Arbeitsordner, nie auf dein Home-Verzeichnis.
- **Dateien aus dem Repo sind fremde Eingaben**, für diese Apps wie für jedes Werkzeug: Öffne nur, was die Aufgabe nennt.
- **Das Ergebnis gehört dem Projekt.** Was aus den eigenen prozeduralen Knoten der Apps, aus eigenen Bildern oder aus Projekt-Assets entsteht, fällt unter die Asset-Lizenz des Projekts. Fremdes, das hineinkommt (ein Foto, eine Schrift, ein Pinsel), muss selbst die Weitergabe erlauben (Art-Rollenkarte, „Where assets come from“).
- **Die Quelle mit dem Export einreichen.** Die Projektdatei (`.mcraft`, `.svg`, `.csv`, die Projektdatei der App) gehört neben die exportierten Dateien, damit ein späterer Agent einen Regler ändern kann, statt neu anzufangen. App und Version stehen in `CREDITS.md`.
- **Der Reifegrad ist eine Selbsteinschätzung** der Macher. Behandle einen Fehler in einer App wie bei jedem Werkzeug: beim Projekt melden, umgehen, nie einen kaputten Export einreichen.

## Welche App wofür

| App | Macht | Rollen und Aufgaben | Quelle im Repo | Stand (Einschätzung der Macher) |
|---|---|---|---|---|
| [**TextureCraft**](https://github.com/storytold/texturecraft) | Texturen auf 3D-Modelle malen: Ebenen, Masken, Smart Materials, Baking, PBR-Export, UDIM | art: ein Modell aus Meshy, TRELLIS oder dem Schwarm texturieren; Nähte reparieren; Maps backen | ihre Projektdatei plus die exportierten Maps | ≈ 93 % der täglichen Arbeit eines Textur-Artists |
| [**MaterialCraft**](https://github.com/storytold/materialcraft) | prozedurale, kachelbare PBR-Materialien aus Knotengraphen | art: Materialien nach der Art Bible (Wände, Böden, Metall, Untergrund), Varianten über Regler (z. B. Abnutzung) | `.mcraft` plus exportierte Maps | ≈ 67 % der täglichen Arbeit eines Material-Artists |
| [**VectorCraft**](https://github.com/storytold/vectorcraft) | Vektorgrafik: Icons, Logos, UI-Grafik, SVG, PDF | ux: Icon-Sets, HUD- und Menügrafik, App-Icons, Logos | `.svg` (Text: jeder Patch zeigt die Änderung) | ≈ 65 % bereit für echte Arbeit |
| [**PhotoCraft**](https://github.com/storytold/photocraft) | Bildbearbeitung: Ebenen, Masken, Einstellungsebenen, echte PSD-Dateien | art, ux: Sprites, Konzepte aufräumen, Texturen ausbessern, Store-Bilder, Screenshots | die Ebenen-Datei (`.psd`) plus der Export | die beliebteste App der Familie auf GitHub |
| [**EffectCraft**](https://github.com/storytold/effectcraft) | Motion Graphics und Compositing; Export als Video und **Lottie** | ux: UI- und Menü-Animationen als Lottie (JSON, für Apps und Websites); release: Titel und Effekte im Trailer | die Projektdatei; Lottie-`.json` ist selbst Text | jung, schon nutzbar |
| [**FilmCraft**](https://github.com/storytold/filmcraft) | Videoschnitt, Farbe und Ton | release: Trailer, Devlogs, Store-Videos aus aufgenommenem Gameplay | die Projektdatei (oder eine `.otio`-Zeitleiste) plus das Video | Alpha, ≈ 56 % bereit für echte Arbeit |
| [**GridCraft**](https://github.com/storytold/gridcraft) | Tabellen mit Formeln und Diagrammen | game-design: Balancing- und Wertetabellen, als `.csv` im Repo, damit das Spiel sie liest und Patches lesbar bleiben | `.csv` (Text) | Clean-Room-Tabellenkalkulation, MCP-Server |

Weniger nützlich für unsere Projekte, genannt, damit niemand erneut suchen muss: **DesignCraft** (Seitenlayout: höchstens ein Pressekit oder ein gedrucktes Handbuch), **LightCraft** (Raw-Entwicklung von Fotos: nur für fotografierte Referenzen), **PdfCraft** (PDF-Bearbeitung) und **ArtCraft** selbst (ein KI-Studio für Bild und Video; für seine Ergebnisse gilt die Regel für generierte Assets: Die Bedingungen des genutzten Modells müssen die Asset-Lizenz des Projekts erlauben).

## An einen Agenten anbinden

Jede App bringt ein `…-cli` mit dem Befehl `mcp` mit. Für Claude Code, mit den Ordnern auf den Arbeitsordner der Aufgabe beschränkt:

```bash
claude mcp add materialcraft -- materialcraft-cli mcp --read-root ~/swarvex-work/materials --write-root ~/swarvex-work/materials
claude mcp add texturecraft  -- texturecraft-cli mcp --read-root ~/swarvex-work/textures --write-root ~/swarvex-work/textures
claude mcp add vectorcraft   -- vectorcraft-cli mcp
claude mcp add gridcraft     -- gridcraft-cli mcp
```

Andere Agenten tragen denselben Befehl als stdio-MCP-Server ein. Die genauen Schalter stehen in der README der jeweiligen App; manche lassen sich auch über einen Steuerkanal in der laufenden Desktop-App bedienen.

## Lesbare Änderungen bei gepackten Quellen

Ein `.mcraft`-Paket ist ein ZIP-Archiv mit einer `package.json` darin; Git zeigt es deshalb als Binärdatei. Mit zwei Zeilen wird die JSON zum Vergleich:

```bash
# im Projekt-Repo, einmal committet (der Lead legt das mit dem Grundpaket an)
echo '*.mcraft diff=mcraft' >> .gitattributes
# auf jedem Rechner, der Reviews macht
git config diff.mcraft.textconv 'unzip -p "$1" package.json #'
```
