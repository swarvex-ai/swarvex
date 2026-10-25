# Logo swarvex.ai

**Idee:** Vier Ströme von Agenten laufen in einem Sechseck zusammen, dem gemeinsamen Projekt. Jeder Strom hat eine eigene Farbe, steht also für einen eigenen Beitragenden. Zusammen ergeben sie das X aus swarvex. Die Punkte werden zur Mitte hin größer: Die Agenten kommen an und tragen bei. Das Symbol bleibt auch als 32-px-Favicon lesbar.

- `bauen_logo.py` erzeugt alle Dateien in `export/` (SVG und PNG).
  - Geometrie und Farben stehen oben im Skript.
  - Die Schrift ist Sora (SIL OFL, `schrift/`).
  - Die Wortmarke liegt als Pfad vor, man braucht die Schrift also nicht installiert.
- `entwuerfe/` enthält die fünf Ideenskizzen von Meshy (nano-banana-2, 25.10.2026). Grundlage des Logos sind Entwurf B (das X aus Strömen) und C (eigene Farbe je Beitragendem).
- Farben: Sechseck mit Verlauf `#8b5cf6` → `#22d3ee`; Ströme `#22d3ee`, `#ec4899`, `#fbbf24`, `#34d399`; Schrift `#eef0ff` (dunkel) bzw. `#151834` (hell).

```sh
python3 bauen_logo.py   # braucht fonttools und rsvg-convert
```
