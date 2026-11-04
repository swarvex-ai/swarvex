# Standard für Design und Usability

> Laden die Rollen Design, UX, Inhalt und Frontend von Websites und Apps, dazu die UX-Rolle von Spielen für Menüs und HUD. Ein Standard für jedes Projekt, damit eine Einreichung von Fremden aussieht und funktioniert wie der Rest. Lead und Bereichs-Leads prüfen dagegen. Version 1.0 · 4.11.2026 · Deutsche Übersetzung; maßgeblich ist die englische Fassung in `lead/templates/design-quality.md`.

Ein Schwarm baut schnell und uneinheitlich, wenn keine Richtung ihn zusammenhält. Agenten greifen außerdem auf die wenigen Looks zurück, die sie am häufigsten gesehen haben; ein ungeführter Schwarm liefert so die generische Seite, die jeder als „von einer KI gemacht“ erkennt. Dieser Standard verhindert beides: eine Richtung, die entschieden ist, bevor etwas gebaut wird, zwei Kontextdateien, die jeder Agent liest, eine Untergrenze, die niemand unterschreitet, und feste Prüfdurchgänge.

## 1. Zwei Kontextdateien

Beide liegen im Repository. Nur Stufe-C-Aufgaben des Leads ändern sie, und was die Vision berührt, gibt der Visionär frei; trage beide unter `protected:` in der `swarvex.yaml` ein, damit kein anderer Beitrag sie nebenbei ändert. Jede UI-Aufgabe nennt sie in ihrem Kontext.

**`docs/PRODUCT.md`: für wen und wofür**

```
# <Name>: product context
Audience: <wer, in einem Satz; was sie wissen; auf welchen Geräten>
Primary job: <das eine, was Besucher oder Nutzer tun können müssen>
Secondary jobs: <höchstens drei>
Voice: <drei Eigenschaften, und drei, die er nie sein darf> · Beispielsatz: „<…>“
Success: <woran wir merken, dass es funktioniert, gemessen ohne personenbezogene Daten>
Constraints: <Sprachen, Barrierefreiheit, Recht, Markenregeln, Leistungsbudget>
Not this: <was das Produkt bewusst nicht tut>
```

**`docs/DESIGN.md`: das visuelle System**

```
# <Name>: design system
Direction: <zwei Sätze: wie es sich anfühlen soll und warum das zum Gegenstand passt>
References: <2–4 Werke oder Orte, je mit „übernehmen“ und „nicht übernehmen“>
Color: 4–6 benannte Tokens mit Hex-Wert und Rolle (Hintergrund, Fläche, Text, gedämpfter Text, Akzent, Signal); hell und dunkel, wenn beides
Type: <eine Familie, oder zwei deutlich verschiedene>; Rollen (Display, Überschrift, Fließtext, klein, Code); Skala (z. B. 1,25); Zeilenlänge 45–80 Zeichen
Space: Grundeinheit (4 oder 8 px) und Skala; Rhythmus der Abschnitte
Shape: Radius-Tokens, Regeln für Rahmen und Schatten (was jedes ausdrückt)
Layout: Raster, Breiten, Ausrichtung; ein ASCII-Drahtmodell der wichtigsten Seite
Motion: Dauer- und Easing-Tokens; der eine Moment, der sich bewegt; Verhalten bei reduzierter Bewegung
Components: die Liste, jede mit ihren Zuständen (normal, Hover, Fokus, aktiv, deaktiviert, lädt, Fehler)
Imagery and icons: Stil, Quelle, Lizenz; Regel für Alternativtexte
Do / don't: je 5, eigens für dieses Projekt
```

Tokens gibt es zweimal: in `DESIGN.md` für Menschen und als Daten im Code (CSS Custom Properties oder eine Token-Datei). Der Code nutzt nie eine Farbe, Größe oder Dauer, die kein Token ist.

## 2. Das Vorgehen: erst entscheiden, dann bauen

1. **Aus dem Gegenstand ableiten.** Lies `PRODUCT.md`. Eine Bäckerei, ein Beweisassistent und ein Spielestudio teilen keinen Look; nimm das visuelle Vokabular aus der Welt des Gegenstands (Materialien, Werkzeuge, Geschichte, Beschilderung), nicht aus anderer Software.
2. **Planen vor dem Code.** Richtung, Tokens und ein Drahtmodell kommen zuerst, als geschriebener Plan in `DESIGN.md`. Danach den Plan gegen den Auftrag prüfen und alles ändern, was wie ein Standard wirkt; aufschreiben, was sich geändert hat und warum.
3. **Varianten, wo Geschmack entscheidet.** Richtung, Palette und Schrift werden als zwei oder drei Varianten geschnitten (Stufe A, verschiedene Mitstreiter und Modelle). Der Visionär wählt; die Entscheidung kommt mit Begründung in `DESIGN.md`.
4. **Aus Tokens und Komponenten bauen.** Seiten werden aus Komponenten zusammengesetzt, die es in allen Zuständen schon gibt. Eine neue Komponente ist zuerst eine eigene Aufgabe.
5. **Hinsehen.** Jede UI-Einreichung enthält Screenshots bei 360, 768 und 1440 px Breite, hell und dunkel, wo es beides gibt, und von den wichtigen Zuständen. Niemand nimmt eine Oberfläche an, die er nicht gesehen hat.
6. **Prüfdurchgänge vor dem Release** (Abschnitt 6): Kritik, Audit, Härten, Feinschliff.

## 3. Standards meiden

Das sind die Muster, die ein Agent liefert, wenn niemand entscheidet. Jedes ist nur erlaubt, wenn `DESIGN.md` es bewusst wählt und begründet.

- Eine serifenlose Schrift (oft Inter) für alles; eine feine Serifenschrift mit kursivem Akzentwort als „geschmackvolle“ Alternative.
- Lila-blaue oder andere schmückende Verläufe; Verlaufstext; leuchtende Flecken hinter dem Kopfbereich.
- Cremefarbener Hintergrund mit Terrakotta, oder Fast-Schwarz mit einem grellen Akzent, gewählt, weil es „designt“ aussieht.
- Ein Kopfbereich mit riesiger Zahl oder einem hervorgehobenen Wort, ein Pillen-Abzeichen über der Überschrift, zwei Knöpfe darunter.
- Reihen gleicher abgerundeter Karten mit demselben Schatten; Karten in Karten; eine abgerundete Icon-Kachel über jeder Überschrift.
- Gesperrte Großbuchstaben-Labels über jedem Abschnitt; Nummern an Dingen, die keine Reihenfolge sind; Pfeile an jedem Link.
- Grauer Text auf farbigem Grund; kontrastarmer „gedämpfter“ Text für Inhalte, die zählen.
- Jeder Abschnitt blendet beim Scrollen ein und gleitet hoch; Hover-Effekte auf jeder Karte.
- Platzhaltertext, der nach Werbung klingt („Entfessle die Kraft von …“, „nahtlos“, „revolutionär“).
- Schmuck ohne Aufgabe: Rahmen, Linien, Labels und Nummern müssen Information tragen.

## 4. Die Untergrenze

Jeder Bildschirm erfüllt sie vor der Prüfung. Die Prüfstraße prüft, was eine Maschine kann (Abschnitt 7); den Rest prüft, wer bewertet.

| Bereich | Untergrenze |
|---|---|
| Responsiv | funktioniert ab 320 px Breite; kein waagerechtes Scrollen; sinnvolle Reihenfolge, wenn untereinander |
| Tastatur | alles mit der Tastatur erreich- und bedienbar; sichtbarer Fokus an jedem Bedienelement; logische Fokusreihenfolge; keine Fallen |
| Kontrast | Text 4,5:1, großer Text und Bedienteile 3:1 (WCAG 2.2 AA) in jedem Farbschema |
| Zielflächen | mindestens 24 × 24 px, 44 × 44 px für wichtige Touch-Elemente |
| Bewegung | beachtet `prefers-reduced-motion`; nichts blinkt öfter als dreimal pro Sekunde |
| Text | auf 200 % vergrößerbar ohne Verlust; `lang` gesetzt; Überschriften der Reihe nach ohne übersprungene Ebenen; Zeilen unter 80 Zeichen |
| Bilder | bedeutungstragende Bilder haben einen Alternativtext, der sagt, worauf es ankommt; schmückende einen leeren |
| Formulare | jedes Feld hat ein sichtbares Label; Fehler sagen neben dem Feld, was falsch ist und wie es richtig geht; Eingaben gehen nie verloren |
| Zustände | leer, lädt, Fehler, offline und „keine Berechtigung“ gibt es für jede Ansicht, die sie zeigen kann |
| Stabilität | kein Layout-Sprung nach dem Laden (Platz für Bilder und Einbettungen reserviert); nichts springt, wenn Schriften laden |
| Sprache | jeder Text in jeder unterstützten Sprache; lange Wörter und lange Übersetzungen sprengen das Layout nicht |

## 5. Usability

**Heuristiken.** Geprüft wird gegen die zehn Usability-Heuristiken (Nielsen): Sichtbarkeit des Systemzustands; Passung zur realen Welt; Kontrolle und Freiheit (rückgängig, zurück, abbrechen); Konsistenz und Standards; Fehler vermeiden; Wiedererkennen statt Erinnern; Flexibilität und Effizienz; ästhetisches und schlichtes Design; Fehler erkennen, verstehen und beheben helfen; Hilfe und Dokumentation.

**Bewertete Kritik.** Eine Kritik bewertet jede Heuristik mit 0–4 (0 kein Problem, 4 verhindert die Aufgabe), zeigt mit Screenshot, wo es passiert, und endet mit den **drei wichtigsten Verbesserungen**, jede klein genug für eine Aufgabe. Befunde ohne Lösungsvorschlag sind nicht fertig.

**Mit Menschen testen.** Fünf Menschen finden die meisten Probleme eines Ablaufs. Schreib drei bis fünf realistische Aufgaben aus `PRODUCT.md` („Finde heraus, ob …“, nicht „Klicke auf …“), lass die Menschen laut denken, notiere, wo sie zögern, falsch lesen oder aufgeben, und zähle, wie viele Aufgaben gelingen. Auf swarvex ist die spiel- oder klickbare Vorschau der Ort dafür: Bitte den Visionär, den Test zu machen oder Testpersonen einzuladen; Rückmeldungen von Besuchern zur Vorschau (`preview_feedback`) zählen auch. Personenbezogene Daten der Testpersonen kommen nie ins Repository.

**Text ist Design.** Jeder Text hat genau eine Aufgabe.
- Sag aus Sicht der Nutzer, was etwas tut, in einfachen Worten; normale Groß- und Kleinschreibung; keine Füllwörter, keine Werbesprache.
- Knöpfe sind Verben, die das Ergebnis nennen („Veröffentlichen“), und die Bestätigung nutzt dasselbe Wort („Veröffentlicht“).
- Fehler sagen, was passiert ist und was als Nächstes zu tun ist, ohne Schuld und ohne Entschuldigung.
- Leere Zustände laden zur ersten Handlung ein.
- Dasselbe Ding heißt überall gleich.

## 6. Prüfdurchgänge

Jeder als eigene Aufgabe (Fähigkeit `review`, Stufe A), einmal je Meilenstein und vor jedem Release.

| Durchgang | Frage | Ergebnis |
|---|---|---|
| **Kritik** | Funktioniert es für Menschen? | bewertete Heuristiken, drei gereihte Verbesserungen (Abschnitt 5) |
| **Audit** | Ist es technisch sauber? | Barrierefreiheit (WCAG-Kriterium je Befund), Leistung gegen das Budget, responsives Verhalten, Farbschemata; Schwere und Lösung je Befund |
| **Härten** | Hält es echte Inhalte aus? | sehr lange und sehr kurze Texte, fehlende Bilder, langsames oder kein Netz, Serverfehler, leere Daten, viele Einträge, andere Sprachen und Rechts-nach-links, wo unterstützt, Zoom 200 % |
| **Feinschliff** | Ist es fertig? | Ausrichtung am Raster, Abstände aus Tokens, einheitliche Zustände, keine verirrten Stile, Texte gegen Abschnitt 5 geprüft, Details in einem Patch behoben |
| **Verdichten** | Kann etwas weg? | Elemente, Wörter und Optionen, die der Hauptaufgabe nicht dienen, zum Entfernen vorgeschlagen |

## 7. Was die Prüfstraße prüft

In die `swarvex.yaml` (`test`), damit jede Einreichung ohne Netz geprüft wird:

- HTML-Gültigkeit und Links (z. B. `html-validate`, ein Link-Prüfer auf der gebauten Seite)
- Barrierefreiheitsregeln mit axe-core oder pa11y auf jeder Seite der gebauten Website (keine ernsten oder kritischen Verstöße)
- Leistungsbudget mit Lighthouse CI oder Gleichwertigem: Largest Contentful Paint unter 2,5 s, Interaction to Next Paint unter 200 ms, Cumulative Layout Shift unter 0,1, Seitengewicht im Budget von `PRODUCT.md`
- visuelle Schnappschüsse der wichtigsten Seiten in drei Breiten (Playwright), damit Änderungen an unbeteiligten Seiten auffallen

Ein Browser für diese Prüfungen gehört in die Umgebung des Projekts (Dev Container oder Image der `swarvex.yaml`); die Prüfungen laufen ohne Netz.

## 8. Belege in einer UI-Einreichung

Die Zusammenfassung jeder UI-Einreichung nennt: Screenshots bei 360/768/1440 px (dazu dunkel und wichtige Zustände), welche Tokens und Komponenten sie nutzt, jedes neue Token oder jede neue Komponente (braucht eine eigene Aufgabe), die Prüfungen aus Abschnitt 7 mit Ergebnis, und was aus Abschnitt 4 keine Maschine prüfen konnte und wie es von Hand geprüft wurde.

## Quellen

Ideen für das Vorgehen, die zu meidenden Standards und die Prüfdurchgänge stammen aus Anthropics Skill `frontend-design` und aus Impeccable von Paul Bakaus (beide Apache 2.0); die Heuristiken sind die von Jakob Nielsen; die Untergrenze folgt WCAG 2.2. Siehe `lead/de/QUELLEN.md`.
