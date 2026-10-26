# Vorlage: Asset-Auftrag (Design, 3D, Bild, Audio, Video)

> Stufe A, Fähigkeit `asset.*` bzw. `design.*`. Kostet fremde Credits, deshalb besonders sorgfältig. **Immer zweistufig:** Vorschau → deine Freigabe → Finale.

```markdown
## Ziel
<Was entsteht, wofür, wo im Projekt?> Damit <Wert>.

## Einsatz
- Ort: <Szene/Bildschirm/Level>
- Größe im Bild: <z. B. ca. 96 px hoch bei 1080p; Kamera schräg von oben 35°>
- Kontext: <was daneben zu sehen ist, Hintergrundfarbe, Licht>

## Stil
- Styleguide: `docs/styleguide.md#<abschnitt>`
- Referenzen (1–3): <vorhandene Assets/Bilder im Repo>
- Palette: <Farbwerte>
- Stimmung: <3 Adjektive>
- Negativ: <kein Text, keine Logos, keine Wasserzeichen, kein Realismus …>

## Technische Vorgaben
| | |
|---|---|
| Format | z. B. GLB (glTF 2.0) / PNG mit Alpha / OGG Vorbis 44,1 kHz |
| Maße/Maßstab | z. B. 1 Einheit = 1 m, Höhe 1,2 m, Blick in −Z, Pivot am Boden mittig |
| Grenzen | z. B. ≤ 5.000 Dreiecke, eine Textur 1024², Datei ≤ 2 MB |
| Rig/Animation | z. B. humanoides Rig; Animationen `idle`, `lauf`, `angriff` |
| Benennung/Ordner | z. B. `assets/gegner/schleim/schleim.glb` |

## Prompt-Vorschlag (für den Dienst)
> <Englischer Prompt, konkret, mit Stil, Ansicht, Hintergrund; Negativliste>

## Ablauf und Kosten
1. Vorschau (z. B. Meshy-Vorschau ohne Textur, ~20 Credits). Abgabe: Vorschaubilder.
2. Freigabe durch den Lead (Begründung bei Ablehnung, max. 1 weitere Vorschau).
3. Finale (z. B. Textur/Refine, ~10 Credits). Abgabe: Datei + Screenshot.
Gesamt geschätzt: <x> Credits. Nicht mehrfach vergeben.

## Abnahme
- [ ] Import in der Prüfstraße ohne Fehler/Warnungen
- [ ] Grenzen eingehalten (Dreiecke, Textur, Dateigröße)
- [ ] Screenshot in Projektbeleuchtung: Silhouette lesbar, Stil passt zur Referenz
- [ ] Benennung/Ordner korrekt
- [ ] Rechte: Beitragserklärung bestätigt; Lizenz des Projekts (z. B. CC BY 4.0) gilt
```

## Beispiel: 3D-Gegner

- **titel:** 3D-Modell Schleim (Vorschau → Finale)
- **faehigkeiten:** `["asset.3d"]`, **pfade:** `["assets/gegner/schleim/**"]`
- Einsatz: Seitenansicht, ca. 90 px hoch, dunkle Höhle mit kühlem Licht
- Stil: wie `assets/gegner/fledermaus/fledermaus.glb`, Low-Poly, flache Farbflächen, Palette `#5cc28d`, `#2f7a55`, Glanzpunkt `#e4ffe9`
- Prompt: *"cute low-poly slime monster, translucent green jelly body with two big eyes, flat shading, game asset, side view, plain background, no text"*
- Grenzen: ≤ 3.000 Dreiecke, 1 Textur 512², Pivot unten mittig, Höhe 0,8 Einheiten
- Kosten: Vorschau ~20, Finale ~10 Credits
