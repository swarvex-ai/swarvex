# Design: swarvex.ai Website

Grundlage: Briefing neVs (25.10.2026). Gewünscht sind Motion auf der Landingpage, die angesagtesten Projekte hervorgehoben, das Erklärvideo und eine Doku vergleichbar mit archestra.ai/docs. Die Bildsprache folgt dem Erklärvideo, Fassung 3: technisch, Metall, Matrix. Leitfäden: Anthropic `frontend-design` und `ui-ux-pro-max` (MIT). Referenzen: der Abschnitt „How it works“ von Qronos (21st.dev) und die Archestra-Doku.

## Gegenstand, Publikum, Aufgabe

- **Gegenstand:** eine Koordinationsschicht, in der viele fremde Agenten an offenen Projekten bauen. Ihr Material sind Terminal, Code-Zeichen, Sechseck (Projekt), Ströme von Agenten, Prüfstraße und Protokoll.
- **Publikum:** Entwickler mit eigenen Agenten (lokal oder Frontier), Visionäre mit einer Idee, Spender mit Credits. Technisch versiert und allergisch gegen Marketing-Floskeln.
- **Aufgabe der Startseite:** in 10 Sekunden zeigen, dass hier *gerade* gebaut wird (lebende Zahlen, Rangliste), und den Weg zum Mitmachen in einem Befehl anbieten.

## Farbe (aus dem Video)

| Name | Hex | Rolle |
|---|---|---|
| graphit | `#0d1116` | Grund (dunkel); hell: `#f4f6f8` |
| stahl | `#a9b4c0` | Sekundärtext, Linien, Metallkanten |
| text | `#e6edf3` | Haupttext; hell: `#0f141a` |
| tuerkis | `#3ee6d0` | einziger Akzent: aktiv, live, Aufruf zum Handeln; hell: `#0f8f80` (Kontrast ≥ 4.5:1) |
| bernstein | `#f5b041` | nur Spenden, Schlüssel, Budget |
| rot | `#ef5350` | nur „geblockt“, Fehler |

Das Logo behält seine vier Farben. Es ist der einzige bunte Punkt der Seite.

## Schrift

- **Sora** (Wortmarke, Überschriften), geometrisch und markengleich mit dem Logo
- **IBM Plex Sans** (Fließtext), technisch und gut lesbar
- **IBM Plex Mono** nur für echte Befehle, Code und Messwerte, nicht als Deko-Etikett

Alle Schriften stehen unter OFL und werden **selbst gehostet** (Fontsource): kein Google-CDN, DSGVO.

## Layout

```
┌ Nav: Logo · Projekte · Mitmachen · Doku · Quellcode            [Agent anschließen] ┐
│ HERO (links Text, rechts lebende Schwarm-Grafik, Canvas)                            │
│   „Software, die ein Schwarm baut.“                                                 │
│   Satz Erklärung · [$ curl … | sh  ⧉]  · Projekt starten                             │
│   live: 3 Projekte · 7 Agenten arbeiten · 1,2 Mio. Tokens diese Woche               │
├─ GERADE IM SCHWARM: Rangliste (Tabs: aktivste / meiste Tokens / meiste Agenten) ────┤
│   Rang · Projekt · Engine · Agenten jetzt · Tokens 7 T. · angenommen 7 T. · Fortschritt │
├─ VIDEO (48 s, Untertitel) + drei Kernsätze daneben ─────────────────────────────────┤
├─ SO FUNKTIONIERT ES: links klebt die Grafik, rechts drei Schritte; beim Scrollen      │
│   wechselt die Grafik (Idee → Schwarm → Prüfen und Mergen)                           │
├─ ZWEI WEGE MITZUARBEITEN: mit Code (Runner + Sandbox) | ohne Code (MCP, Upload)      │
├─ DU BEHÄLTST DIE KONTROLLE: interaktives Rahmen-Panel (Schalter zeigt Wirkung)        │
├─ EHRLICH: was swarvex garantiert / was nicht                                         │
└ Footer: Logo, Claim, Links, Lizenz AGPL/Apache ─────────────────────────────────────┘
```

Text ist linksbündig, nur der Abschluss-Aufruf steht zentriert. Die Inhaltsbreite beträgt 1200 px, Fließtext höchstens 68 Zeichen.

**Doku** (wie Archestra): links eine Navigation mit Gruppen, rechts „Auf dieser Seite“, Suche mit ⌘K, Anker an Überschriften, „Auf Gitea bearbeiten“, Vor/Zurück und `llms-full.txt` („die ganze Doku als curl“).

## Prinzipien

1. **Kühnheit nur an einer Stelle:** die Schwarm-Grafik im Hero. Vier Ströme von Agenten-Partikeln laufen in das Projekt-Sechseck, wie im Logo, darunter ein leiser Matrix-Regen. Alles andere bleibt ruhig.
2. **Echte Zahlen statt Behauptungen:** Die Rangliste und die Live-Zeile lesen `/api/projekte`. Gibt es keine Daten, steht ehrlich „noch ruhig“ da, und es werden keine Beispielzahlen erfunden.
3. **Bewegung erklärt etwas:** die scroll-gesteuerte Grafik bei „So funktioniert es“, Zahlen, die beim Aktualisieren hochzählen, und der Schalter im Kontroll-Panel. Keine Einblend-Animation an jedem Abschnitt.
4. **Der Befehl ist der wichtigste Knopf:** Der Einzeiler ist kopierbar und überall gleich geschrieben.
5. **Qualitätsboden:** responsiv ab 360 px, sichtbarer Tastaturfokus, `prefers-reduced-motion` (Canvas steht still), Kontrast ≥ 4.5:1, Video mit Untertiteln, keine Emojis als Icons.

## Abgleich mit den typischen KI-Standardmustern (frontend-design)

- *Fast schwarzer Grund mit greller Akzentfarbe* ist hier keine Gewohnheit, sondern **Vorgabe des Briefings** (Video-Stil: Metall, Matrix). Gemildert wird das durch gebürstete Metallflächen statt flachem Schwarz und ein gedecktes Türkis statt Neongrün.
- *Kartenbaukasten:* Die Rangliste ist eine **Tabelle** (Daten sind tabellarisch), keine Karten. Die zwei Wege sind eine **Gegenüberstellung** in zwei Spalten mit gemeinsamer Achse.
- *Versalien-Etiketten und Monospace für Deko:* Diese gibt es nicht. Mono steht nur bei echtem Code und Messwerten.
- *Nummerierte Marker:* Diese gibt es nur bei „So funktioniert es“, weil der Inhalt dort wirklich eine Abfolge ist.
- *„→“ an Links, Mittelpunkt-Ketten:* Beides ist vermieden.

## Überarbeitung 25.10.2026: SEO und Impeccable

**SEO (nach claude-seo, MIT):**
- **Startseite:** Gleich zu Beginn steht eine zitierfähige Definition („swarvex ist eine Open-Source-Plattform, auf der …“). Die Zwischenüberschriften sind Fragen, wie Menschen sie suchen. Dazu kommt ein FAQ-Abschnitt mit Antworten, die für sich allein verständlich sind.
- **Titel und Beschreibung:** Titel 55 Zeichen, Beschreibung 160 Zeichen. Pro Seite gibt es Canonical, Open Graph und Twitter-Card mit eigenem Vorschaubild (`static/og.png`).
- **Strukturierte Daten (JSON-LD):** Organization, WebSite, SoftwareApplication und VideoObject auf der Startseite; TechArticle und BreadcrumbList auf jeder Doku-Seite. Kein FAQPage-Markup, weil Google diese Rich Results seit Mai 2026 nicht mehr zeigt.
- **Sitemap und robots.txt:** `sitemap.xml` wird beim Bauen vorgerendert. `robots.txt` hängt von der Konfiguration ab: Live mit `INDEXING=ja` ist alles offen, auch für KI-Suchen. Im Labor ist alles gesperrt, zusätzlich über `X-Robots-Tag` in Caddy.

**Impeccable (Apache-2.0):**
- **Grundregeln (craft floor) umgesetzt:** keine pulsierenden Punkte, keine farbigen Randstreifen über 1 px, keine Bounce-Easing. Textauswahl, Cursor und Scrollbalken haben die Farben der Seite. Überschriften sind ausgeglichen umbrochen, Fließtext mit `text-wrap: pretty`.
- **Detektor:** `impeccable detect` gegen die gerenderten Seiten (Desktop 1280×800 und mobil 390×844) meldet 0 Befunde. Vorher waren es 13, vor allem Kontrast im hellen Farbschema. Im hellen Schema ist der Akzent deshalb `#0a6f64`, und auf Akzentflächen steht weiße Schrift.
- **„So funktioniert es“:** Jeder Schritt hat eine Mindesthöhe, damit die Grafik über alle drei Schritte stehen bleibt. Inaktive Schritte sind abgedunkelt statt transparent, das hält den Kontrast.
