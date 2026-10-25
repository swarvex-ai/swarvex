"""Baut das swarvex-Logo als SVG (Symbol, Wortmarke, Varianten) und PNG-Exporte.

Idee: Vier Ströme von Agenten (Punkte, jeder Strom eine eigene Farbe = ein eigener Beitragender)
laufen in einem Sechseck zusammen, dem gemeinsamen Projekt. Zusammen ergeben sie das X aus swarvex.
Die Punkte werden zur Mitte hin größer: Die Agenten kommen an und tragen bei.

Aufruf: python3 bauen_logo.py   (braucht fonttools und rsvg-convert)
Schrift: Sora (SIL Open Font License, schrift/OFL.txt).
"""
import math, subprocess
from pathlib import Path

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont

HIER = Path(__file__).parent
AUS = HIER / "export"
AUS.mkdir(exist_ok=True)

VIOLETT, CYAN, MAGENTA, AMBER, GRUEN = "#8b5cf6", "#22d3ee", "#ec4899", "#fbbf24", "#34d399"
HELL, DUNKEL, GEDAEMPFT_D, GEDAEMPFT_H = "#eef0ff", "#151834", "#8b90b8", "#5b6085"
STROEME = [(45, CYAN), (135, MAGENTA), (225, AMBER), (315, GRUEN)]  # Winkel in SVG-Koordinaten (y nach unten)
ABSTAENDE = [25, 33.5, 41, 47.5]
RADIEN = [5.2, 4.2, 3.3, 2.5]


def symbol_elemente(x0=0, y0=0, groesse=100, id_="v"):
    """SVG-Elemente des Symbols in einem Quadrat groesse×groesse ab (x0, y0)."""
    s = groesse / 100
    mx, my = x0 + 50 * s, y0 + 50 * s
    ecken = " ".join(f"{mx + 14.5 * s * math.cos(math.radians(30 + 60 * i)):.2f},{my + 14.5 * s * math.sin(math.radians(30 + 60 * i)):.2f}"
                     for i in range(6))
    teile = [f'<defs><linearGradient id="{id_}" x1="0" y1="1" x2="1" y2="0">'
             f'<stop offset="0" stop-color="{VIOLETT}"/><stop offset="1" stop-color="{CYAN}"/></linearGradient></defs>',
             f'<polygon points="{ecken}" fill="url(#{id_})"/>']
    for winkel, farbe in STROEME:
        for d, r in zip(ABSTAENDE, RADIEN):
            x = mx + d * s * math.cos(math.radians(winkel))
            y = my + d * s * math.sin(math.radians(winkel))
            teile.append(f'<circle cx="{x:.2f}" cy="{y:.2f}" r="{r * s:.2f}" fill="{farbe}"/>')
    return "\n".join(teile)


def text_pfad(text, schrift, groesse, x, grundlinie):
    """Text als SVG-Pfad (Glyphen-Umrisse, keine Schrift nötig). Gibt Pfad und Breite zurück."""
    font = TTFont(HIER / "schrift" / schrift)
    glyphen, cmap, hmtx = font.getGlyphSet(), font.getBestCmap(), font["hmtx"]
    faktor = groesse / font["head"].unitsPerEm
    pfade, cursor = [], x
    for zeichen in text:
        name = cmap[ord(zeichen)]
        pen = SVGPathPen(glyphen)
        glyphen[name].draw(TransformPen(pen, (faktor, 0, 0, -faktor, cursor, grundlinie)))
        pfade.append(pen.getCommands())
        cursor += hmtx[name][0] * faktor
    return " ".join(pfade), cursor - x


def svg(breite, hoehe, inhalt, hintergrund=None):
    hg = f'<rect width="100%" height="100%" fill="{hintergrund}"/>' if hintergrund else ""
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{breite:.0f}" height="{hoehe:.0f}" '
            f'viewBox="0 0 {breite:.2f} {hoehe:.2f}">{hg}{inhalt}</svg>\n')


def schriftzug(x, grundlinie, groesse, farbe_name, farbe_ai):
    p1, b1 = text_pfad("swarvex", "Sora-SemiBold.ttf", groesse, x, grundlinie)
    p2, b2 = text_pfad(".ai", "Sora-Regular.ttf", groesse, x + b1 + groesse * 0.02, grundlinie)
    return f'<path d="{p1}" fill="{farbe_name}"/><path d="{p2}" fill="{farbe_ai}"/>', b1 + b2 + groesse * 0.02


def baue():
    dateien = {}
    dateien["swarvex_symbol.svg"] = svg(100, 100, symbol_elemente())
    dateien["swarvex_app-icon.svg"] = svg(100, 100, f'<rect width="100" height="100" rx="22" fill="{DUNKEL}"/>' + symbol_elemente(8, 8, 84))

    for variante, (name_farbe, ai_farbe, claim_farbe) in {"dunkel": (HELL, GEDAEMPFT_D, GEDAEMPFT_D),
                                                          "hell": (DUNKEL, GEDAEMPFT_H, GEDAEMPFT_H)}.items():
        zug, breite = schriftzug(100, 72, 62, name_farbe, ai_farbe)
        dateien[f"swarvex_logo_{variante}.svg"] = svg(100 + breite + 4, 100, symbol_elemente(0, 0, 100, f"v{variante}") + zug)
        claim, cb = text_pfad("the coordination layer for agent-built software", "Sora-Regular.ttf", 17.5, 102, 98)
        dateien[f"swarvex_logo_claim_{variante}.svg"] = svg(100 + max(breite, cb + 2) + 4, 108,
                                                           symbol_elemente(0, 4, 100, f"c{variante}") + zug
                                                           + f'<path d="{claim}" fill="{claim_farbe}"/>')
        wort, wb = schriftzug(0, 62, 62, name_farbe, ai_farbe)
        dateien[f"swarvex_wortmarke_{variante}.svg"] = svg(wb + 2, 78, wort)
    for name, inhalt in dateien.items():
        (AUS / name).write_text(inhalt)

    pngs = [("swarvex_symbol.svg", "swarvex_symbol_1024.png", ["-w", "1024"]),
            ("swarvex_app-icon.svg", "swarvex_app-icon_512.png", ["-w", "512"]),
            ("swarvex_app-icon.svg", "favicon_32.png", ["-w", "32"]),
            ("swarvex_logo_dunkel.svg", "swarvex_logo_dunkel_2000.png", ["-w", "2000"]),
            ("swarvex_logo_hell.svg", "swarvex_logo_hell_2000.png", ["-w", "2000"]),
            ("swarvex_logo_claim_dunkel.svg", "swarvex_logo_claim_dunkel_2400.png", ["-w", "2400"])]
    for quelle, ziel, groesse in pngs:
        subprocess.run(["rsvg-convert", *groesse, "-o", AUS / ziel, AUS / quelle], check=True)
    print("\n".join(sorted(p.name for p in AUS.iterdir())))


if __name__ == "__main__":
    baue()
