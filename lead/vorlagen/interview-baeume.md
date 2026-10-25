# Startbäume für das Projekt-Interview

> Für Phase 0 (LEAD.md, „Das Projekt-Interview“). Jeder Knoten ist eine Entscheidung. **„nach“** nennt die Vorbedingungen: Erst wenn diese geklärt sind, gehört der Knoten zur Front.
> **P** = Pflichtzweig (muss vor dem Fundament entschieden sein), **S** = späterer Zweig (darf mit vorläufigem Standardwert offen bleiben).
> Die Bäume sind ein Mindestumfang. Ergänze alles, was die konkrete Idee zusätzlich aufwirft.
> Fakten (vorhandenes Repo, Versionen, Lizenzen) recherchierst du selbst, statt zu fragen.

## Gemeinsamer Stamm (alle Projektarten)

| ID | Entscheidung | P/S | nach | Hinweis für die Empfehlung |
|---|---|---|---|---|
| G1 | **Art:** Spiel, Tool/CLI/Bibliothek, App, Mischform? | P | – | bestimmt den Fachbaum unten |
| G2 | **Kern in einem Satz:** Was ist es, für wen, welches Erlebnis oder Problem? | P | – | Formulierungsvorschlag aus der Idee machen |
| G3 | **Zielgruppe:** Wer genau, welches Vorwissen, welches Gerät, wie lange am Stück? | P | G2 | eng fassen; „alle“ ist keine Zielgruppe |
| G4 | **Säulen:** 3–5 Eigenschaften, die immer stimmen müssen | P | G2, G3 | jede Säule mit „woran merkt man das?“ |
| G5 | **Nicht-Ziele:** Was ist es ausdrücklich nicht? | P | G4 | mindestens 3; schützt vor Ausufern |
| G6 | **Erfolgskriterien Version 1:** messbar oder eindeutig beobachtbar | P | G4 | „fertig, wenn …“ |
| G7 | **Plattformen** | P | G1, G3 | wenige zuerst; jede Plattform kostet Prüfaufwand |
| G8 | **Stack / Engine / Sprache** | P | G1, G7 | Recherche: was können Mitstreiter-Modelle gut (verbreitete Sprachen)? |
| G9 | **Lizenz Code / Assets** | P | – | Empfehlung MIT bzw. CC BY 4.0; Projekte auf swarvex sind Open Source |
| G10 | **Projektsprache** (Aufgaben, Doku) und Sprachen des Produkts | P | G3 | Code-Bezeichner meist Englisch |
| G11 | **Qualitätsniveau:** Prototyp / Indie / poliert | P | G6 | bestimmt Prüfaufwand und Asset-Anspruch |
| G12 | **Referenzen und Stil:** „wie X, aber Y“, Stimmung, Palette | P | G4 | 2–3 Referenzen mit „übernehmen / nicht übernehmen“ |
| G13 | **Bestand:** Was existiert schon (Repo, Code, Assets, Marke)? | P | – | selbst nachsehen, nur Lücken fragen |
| G14 | **Unveränderliches:** Was darf nie geändert werden? | P | G13 | z. B. Kernsteuerung, Markenelemente |
| G15 | **Ressourcen:** erwartete Schwarmgröße, Asset-Bedarf, Budget für Spenden-Aufträge | P | G11 | Budget pro Meilenstein ohne Rückfrage festlegen |
| G16 | **Barrierefreiheit** | S | G3, G7 | Mindeststandard vorschlagen |
| G17 | **Datenschutz / Daten von Nutzern** | P, wenn Nutzerdaten anfallen, sonst S | G1 | so wenig Daten wie möglich |
| G18 | **Release-Ziel:** wo veröffentlichen (itch.io, Store, Paketregister, Web), wann ungefähr | S | G7, G9 | Freigabe bleibt beim Visionär |
| G19 | **Einnahmen des Projekts** (falls je): z. B. „kostenlos“ oder „finanziert Lead und Infrastruktur“ | S | G9 | vorab schriftlich erklären (konzept.md §8) |
| G20 | **Rolle des Visionärs im Alltag:** Wie oft ist er erreichbar, was will er selbst abnehmen? | P | – | bestimmt, was du delegiert bekommst |

## Fachbaum: Spiel

| ID | Entscheidung | P/S | nach | Hinweis |
|---|---|---|---|---|
| S1 | **Genre und Perspektive** (2D/3D, Kamera) | P | G2, G12 | |
| S2 | **Kern-Spielschleife:** was tut der Spieler alle 10 s, 1 min, 10 min? | P | S1 | wichtigste Frage des Baums |
| S3 | **Steuerung und Eingabegeräte** (Tastatur/Maus, Controller, Touch) | P | G7, S2 | frei belegbar? |
| S4 | **Sitzungslänge und Fortschritt:** Läufe, Speichern, Meta-Fortschritt | P | G3, S2 | |
| S5 | **Gefühl:** Reaktionszeiten, Trefferfeedback, Lesbarkeit der Gefahr | P | S2, G4 | als messbare Kriterien formulieren |
| S6 | **Schwierigkeit und Fairness** | S | S2, G3 | Schwierigkeitsgrade? |
| S7 | **Inhaltsmenge Version 1:** Level, Gegner, Waffen, Gegenstände | P | G6, S2 | klein anfangen: 1 Level vollständig |
| S8 | **Inhalte als Daten:** welche Inhaltstypen sollen Mitstreiter ergänzen können? | P | S7, G8 | bestimmt die Erweiterungspunkte im Fundament |
| S9 | **Asset-Stil und -Pipeline:** 2D/3D, Auflösung, Polygon- und Texturgrenzen, Animationen | P | S1, G12 | |
| S10 | **Audio:** Musik, Effekte, Sprache | S | G12 | |
| S11 | **Erzählung/Welt** (falls relevant) | S | S1 | |
| S12 | **Mehrspieler** | P, wenn ja, sonst S | S2, G7 | verändert die Architektur stark |
| S13 | **Balancing-Methode:** Simulationen, Kennzahlen | S | S2, S7 | |
| S14 | **Playtests:** wer, wie oft, welche Fragen | S | G6 | |

## Fachbaum: Tool / CLI / Bibliothek

| ID | Entscheidung | P/S | nach | Hinweis |
|---|---|---|---|---|
| T1 | **Nutzungsform:** CLI, Bibliothek/API, Dienst, Plugin für X | P | G2 | |
| T2 | **Kernabläufe:** die 3–5 häufigsten Aufgaben der Nutzer, Schritt für Schritt | P | T1, G3 | |
| T3 | **Schnittstelle:** Befehle/Funktionen, Optionen, Standardwerte | P | T2 | vor dem Bauen dokumentieren |
| T4 | **Ein- und Ausgabeformate** (Dateien, Streams, Formate, Kodierungen) | P | T2 | Beispiele sammeln (Golden Tests) |
| T5 | **Fehlerverhalten:** Meldungen, Exit-Codes, Wiederaufnahme | P | T3 | |
| T6 | **Konfiguration:** Dateien, Umgebungsvariablen, Profile | S | T3 | |
| T7 | **Kompatibilität:** Betriebssysteme, Sprach-/Laufzeitversionen, Abwärtskompatibilität | P | G7, G8 | |
| T8 | **Leistung und Grenzen:** Datenmengen, Zeitvorgaben, Speicher | P | T2 | messbar machen |
| T9 | **Abhängigkeiten:** erlaubt, verboten, Lizenzverträglichkeit | P | G8, G9 | Recherche selbst |
| T10 | **Sicherheit:** Umgang mit fremden Eingaben, Rechten, Geheimnissen | P | T4 | |
| T11 | **Verteilung:** Paketregister, Binärdateien, Container | S | G18 | |
| T12 | **Doku:** `--help`, README, Beispiele, Referenz | P | T3 | Teil jeder Aufgabe |
| T13 | **Erweiterbarkeit:** Plugins, Formate, Befehle von Dritten | S | T3 | bestimmt Erweiterungspunkte |

## Fachbaum: App (Web / Mobil / Desktop)

| ID | Entscheidung | P/S | nach | Hinweis |
|---|---|---|---|---|
| A1 | **App-Typ:** Web, mobil (iOS/Android), Desktop, plattformübergreifend | P | G7 | |
| A2 | **Hauptbenutzerflüsse:** 3–5 Wege, die die App tragen | P | G2, G3 | als Ablauf skizzieren |
| A3 | **Bildschirme** je Fluss | P | A2 | `design.ui`-Aufgaben folgen daraus |
| A4 | **Datenmodell:** welche Dinge, Beziehungen, wer besitzt welche Daten | P | A2 | |
| A5 | **Konten und Anmeldung** | P, wenn Nutzerdaten, sonst S | A4, G17 | |
| A6 | **Backend:** keins / eigenes / fertiger Dienst; API-Vertrag | P | A4, G8 | Mock-Server für parallele Arbeit |
| A7 | **Offline-Fähigkeit und Synchronisation** | S | A1, A4 | |
| A8 | **Design-System:** Tokens (Farben, Abstände, Schrift), Komponenten | P | G12 | steht vor den Bildschirmen |
| A9 | **Responsivität / Bildschirmgrößen** | P | A1 | |
| A10 | **Barrierefreiheit** (Kontrast, Tastatur, Screenreader) | P | A3 | bei Apps Pflicht |
| A11 | **Benachrichtigungen, Hintergrundaktivität** | S | A1, A2 | |
| A12 | **Analyse/Telemetrie** | S | G17 | nur mit Einwilligung, minimal |
| A13 | **Store-Anforderungen** (Richtlinien, Konten, Gebühren) | S | A1, G18 | |

## Abschluss

Die Pflicht-Front ist leer, wenn alle **P**-Knoten des Stamms und des passenden Fachbaums im Projektbrief stehen, als *entschieden*, *Empfehlung angenommen* oder *delegiert*. Dann folgt die ausdrückliche Bestätigung durch den Visionär (LEAD.md, Phase 0, Schritt 6).

## Zusatzbaum: Clean-Room-Neubau

Nur wenn ein vorhandenes Programm neu gebaut werden soll. Jede Frage mit Empfehlung, wie gewohnt.

1. **Vorbild:** Welches Programm, welche Version, von wem? *(Empfehlung: eine konkrete Version festhalten, damit die Spezifikation einen Bezug hat.)*
2. **Erwerb:** Hast du eine eigene, rechtmäßige Lizenz? Wer analysiert, braucht ebenfalls eine eigene. *(Ohne das: kein Projekt.)*
3. **Schutzmaßnahmen:** Gibt es Kopierschutz, Lizenzprüfung, Online-Zwang oder Anti-Cheat? *(Wenn ja: Analyse nur ohne Umgehung möglich; sonst kein Projekt.)*
4. **Ziel:** Kompatibilität (Dateien, Protokolle), Ersatz für ein eingestelltes Produkt, Erhalt, Lernen? *(Empfehlung: Kompatibilität mit offenen Dateiformaten als messbares Ziel.)*
5. **Abgrenzung:** Neuer Name, eigenes Aussehen, eigene Assets? *(Pflicht. Marken, Logos, Grafiken, Texte und Töne des Vorbilds werden nie übernommen.)*
6. **Umfang:** Welche Funktionen zuerst? *(Empfehlung: die kleinste Kette, mit der man eine echte Datei des Vorbilds öffnen und speichern kann.)*
7. **Rechtsgrundlage:** Ein Satz, warum die Analyse erlaubt ist (z. B. „eigene Lizenz, Beobachtung des Programmverhaltens nach § 69d Abs. 3 UrhG, Ziel Interoperabilität“). *(Wird beim Projekt als Rechtsgrundlage hinterlegt und ist öffentlich einsehbar.)*
