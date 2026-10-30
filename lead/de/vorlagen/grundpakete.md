# Grundpakete je Projektart

> Für Phase 2 (LEAD.md, „Planung“, Schritt 0). Die Interview-Bäume klären **Entscheidungen**; die Grundpakete listen die **Bausteine**, die jedes Projekt einer Art braucht, ob jemand sie erwähnt hat oder nicht: Ein Spiel braucht ein Hauptmenü, eine App eine Fehlerseite, ein Kommandozeilen-Tool `--help`. Visionäre fragen selten danach, und genau deshalb werden sie vergessen.
> Die Pakete sind ein Minimum. Ergänze, was das konkrete Projekt zusätzlich braucht.

## So benutzt du sie

1. **Pakete wählen:** „Alle Projekte“ plus das Paket der Projektart. Ein Mischprojekt nimmt die Vereinigung (ein Spiel mit Level-Editor: Spiel + Tool). Ein Clean-Room-Nachbau nimmt das Paket der Art, die er nachbaut.
2. **Jeder Punkt bekommt eine Entscheidung**, keiner bleibt stumm:
   - **Mx**: in Meilenstein x (wird zu Epics und Aufgaben im Plan),
   - **später**: mit dem Meilenstein, in dem er fällig ist,
   - **nicht nötig**: mit einem Satz Begründung; er kommt zusätzlich in die Nicht-Ziele.
   Deine Empfehlung folgt der Spalte „Standard“.
3. **Ergebnis festhalten** im Projektbrief, Abschnitt „Grundpaket“ (eine Zeile je Punkt). Der Visionär bestätigt es zusammen mit dem Brief. Bei einem Projekt mit bereits bestätigtem Brief fragst du es als eine kleine Interview-Runde ab (LEAD.md, Phase 0, „Kleine Interviews zwischendurch“).
4. **Punkte zu Aufgaben machen** wie jede andere Arbeit: meist zuerst eine Gestaltungs- oder Spezifikationsaufgabe (`design.ui`, `text.en`), dann die Umsetzung. Baue eine Referenz (ein Menübildschirm, eine Einstellungsseite, ein Befehl) als Erweiterungspunkt, damit der Rest zu Stufe-B-Aufgaben wird.
5. **Bei jeder Meilenstein-Abnahme erneut prüfen** (LEAD.md 7.1): Ist alles erledigt, was in diesem Meilenstein fällig war?

**Spalte „Standard“:** **erster** = der erste spielbare oder benutzbare Meilenstein (Vertical Slice), **v1** = vor Version 1, **Release** = vor einer öffentlichen Veröffentlichung, **falls** = nur wenn die Bedingung im Punkt zutrifft.

## Alle Projekte

| ID | Baustein | Fertig, wenn | Standard |
|---|---|---|---|
| B1 | **README** | sagt, was es ist, zeigt einen Screenshot oder ein Beispiel, erklärt Installation oder Start und wie man mitmacht | erster |
| B2 | **Lizenzen und Credits** | `LICENSE`-Dateien für Code und Assets; `CREDITS.md` mit Beitragenden, genutzten Modellen und Diensten, Fremdlizenzen | erster |
| B3 | **Version und „Über“** | das Produkt zeigt seine Version; ein Über-Bildschirm, `--version` oder Fußzeile nennt Lizenz und Projektseite | v1 |
| B4 | **Changelog und Versionierung** | `CHANGELOG.md`, Versionsschema entschieden (z. B. SemVer), jedes Release hat einen Eintrag | v1 |
| B5 | **Release-Build** | reproduzierbarer Build je Zielplattform, mit Download- oder Installationsanleitung | v1 |
| B6 | **Sicherheitskontakt** | `SECURITY.md`: wie man eine Lücke meldet, ohne sie öffentlich zu machen | Release |
| B7 | **Weg für Fehlerberichte** | Issue-Vorlage mit Schritten, erwartetem und tatsächlichem Ergebnis, Version, System | v1 |
| B8 | **Name, Logo, Icon** | Name auf Konflikte geprüft, Logo und Icon in den Größen, die die Plattformen brauchen | v1 |
| B9 | **Material für die Projektseite** | Kurzbeschreibung, 3–5 Screenshots oder ein kurzer Clip für swarvex.ai und Stores | Release |

B6 und B7 sind für Repos in der Organisation `swarvex-ai` bereits abgedeckt: GitHub nimmt die Vorgaben aus `swarvex-ai/.github`, solange ein Repo keine eigenen Dateien hat. Eine Aufgabe nur anlegen, wenn das Projekt etwas Eigenes braucht, z. B. ein Fehlerformular für ein Spiel, das nach Version, System, Eingabegerät und Speicherstand fragt.

## Spiel

| ID | Baustein | Fertig, wenn | Standard |
|---|---|---|---|
| G-1 | **Start und Titel** | Splash mit Pflichtlogos (Engine, Lizenzen), Titelbild mit Name und Logo; vom Start bis zum Titel unter 5 s | erster |
| G-2 | **Hauptmenü** | Weiterspielen, neues Spiel, Optionen, Credits, Beenden; vollständig nur mit Gamepad oder Tastatur bedienbar, Fokus immer sichtbar | erster |
| G-3 | **Pausemenü** | Fortsetzen, Optionen, Neustart am Checkpoint, zurück zum Hauptmenü; Spiel und Klänge pausieren; öffnet sich bei Fokusverlust | erster |
| G-4 | **Optionen** | Audio (gesamt, Musik, Effekte, Stimmen), Anzeige (Fenstermodus, Auflösung, VSync, Bildratenbegrenzung), Steuerung (Tastenbelegung, Empfindlichkeit), Sprache; gespeichert und beim nächsten Start angewendet | erster (Tastenbelegung: v1) |
| G-5 | **Barrierefreiheit, Grundlagen** | Untertitel, Bildschirmwackeln und Blitzen abschaltbar, Blitzen ≤ 3 Hz, Textgröße, Gefahr nicht nur über Farbe erkennbar | erster |
| G-6 | **Eingabe** | Tastatur/Maus und Gamepad; Tastensymbole folgen dem zuletzt benutzten Gerät; Controller im Betrieb an- und absteckbar; keine Sackgasse, die eine Maus verlangt | erster |
| G-7 | **Speichern** | Checkpoints oder automatisches Speichern, Einstellungen getrennt vom Fortschritt, versioniertes Speicherformat, eine kaputte Speicherdatei bringt das Spiel nicht zum Absturz | erster |
| G-8 | **Einstieg** | die ersten Minuten bringen die Steuerung im Spiel bei (Hinweise, sicherer Raum), nicht über Textwände | erster |
| G-9 | **HUD und Rückmeldung** | Gesundheit und Ressourcen, Treffer-Feedback, Interaktionshinweise; lesbar bei 1080p und auf einem Handheld-Bildschirm | erster |
| G-10 | **Tod, Neuversuch, Ende** | Todes- oder Fehlschlag-Bildschirm, schneller Neuversuch, Endbildschirm mit Credits | erster |
| G-11 | **Menügefühl** | Übergänge und Animationen der Menüs, UI-Klänge (bewegen, bestätigen, zurück, Fehler), Menümusik; passend zum Styleguide | v1 |
| G-12 | **Laden** | Ladebildschirm oder -anzeige; kein Einfrieren über 1 s ohne Rückmeldung | v1 |
| G-13 | **Credits im Spiel** | Beitragende, Modelle und Dienste, Fremdlizenzen, erreichbar aus dem Hauptmenü | v1 |
| G-14 | **Bereit zur Übersetzung** | alle Spielertexte in einer Tabelle, keine Texte im Code, Schrift deckt die geplanten Sprachen ab | erster |
| G-15 | **Leistung und Qualitätsstufen** | Ziel-Bildrate auf Referenz-Hardware gemessen; Qualitätsvoreinstellungen (niedrig/mittel/hoch) | v1 |
| G-16 | **Fensterverhalten** | Fokusverlust pausiert, Auflösung und Fenstermodus wechseln ohne Neustart, sauberes Beenden | v1 |
| G-17 | **Logs und Abstürze** | Logdatei an dokumentiertem Ort, ein Absturz hinterlässt einen lesbaren Bericht, Spieler wissen, wie sie ihn schicken | v1 |
| G-18 | **Audiomischung** | Lautstärken ausgewogen, wichtige Klänge bleiben hörbar (Ducking) | v1 |
| G-19 | **Release-Material** | Store-Text, Screenshots, Trailer, Key Art, Pressemappe | Release |
| G-20 | **Erfolge, Cloud-Speicher, Plattformdienste** | nur wenn ein Store oder eine Plattform es verlangt oder der Visionär es will | falls |

## App (Web, Mobil, Desktop)

| ID | Baustein | Fertig, wenn | Standard |
|---|---|---|---|
| A-1 | **App-Rahmen und Navigation** | Hauptnavigation, Zurück verhält sich wie erwartet, Deep Links oder URLs je Bildschirm | erster |
| A-2 | **Erster Start** | kurzes Onboarding oder ein leerer Zustand, der den nächsten Schritt erklärt; optional Beispieldaten | erster |
| A-3 | **Zustände jedes Bildschirms** | Laden, leer, Fehler, offline und „keine Berechtigung“ gestaltet und umgesetzt, nicht nur der Normalfall | erster |
| A-4 | **Formulare** | Prüfung direkt am Feld, verständliche Fehlermeldungen, Warnung vor dem Verlust ungespeicherter Eingaben | erster |
| A-5 | **Rückmeldung und Sicherheit** | Bestätigungen, Rückgängig oder Rückfrage bei zerstörerischen Aktionen | erster |
| A-6 | **Einstellungen** | Sprache, helles/dunkles Design, Benachrichtigungen und alles, was der Brief Nutzern zu ändern erlaubt | v1 |
| A-7 | **Konten** | registrieren, anmelden, abmelden, Zugang wiederherstellen, Konto löschen | falls Nutzerkonten |
| A-8 | **Barrierefreiheit** | WCAG 2.2 AA: Kontrast, Tastatur, sichtbarer Fokus, Screenreader-Beschriftungen, reduzierte Bewegung beachtet | erster |
| A-9 | **Responsiv** | Telefon-, Tablet- und Desktop-Breiten, beide Ausrichtungen, wo relevant | erster |
| A-10 | **Bewegung und Mikro-Interaktionen** | Übergänge und Zustände (Hover, gedrückt, lädt) aus den Design-Tokens; aus, wenn reduzierte Bewegung eingestellt ist | v1 |
| A-11 | **Bereit zur Übersetzung** | Texte in einer Tabelle, Datum, Zahlen und Währungen je Sprache formatiert | erster |
| A-12 | **Rechtliche Seiten** | Datenschutzerklärung, Lizenzen und Credits; Impressum und Nutzungsbedingungen, wo nötig | Release |
| A-13 | **Rechte an eigenen Daten** | Export und Löschung der eigenen Daten | falls Nutzerdaten |
| A-14 | **Fehlerseiten und Fehlerberichte** | 404- und Fehlerseite mit Weg zurück; Fehlerberichte nur mit Einwilligung | v1 |
| A-15 | **Leistungsbudget** | Startzeit und Größe in der Prüfstraße gegen ein Budget gemessen | v1 |
| A-16 | **Icons und Vorschaubilder** | App-Icon, Favicon, Startbildschirm, Vorschaubilder für soziale Netze und Stores | v1 |
| A-17 | **Updates und Migration** | Hinweis auf Updates, lokale Daten überstehen ein Update | falls installierte App |
| A-18 | **Verteilung** | Store-Eintrag, PWA-Manifest oder Installer, Signatur | Release |
| A-19 | **Hilfe** | Hilfe oder FAQ zu den Hauptabläufen, Über-Bildschirm mit Version | v1 |

## Website (Landingpage, Dokumentation, Inhalte)

| ID | Baustein | Fertig, wenn | Standard |
|---|---|---|---|
| W-1 | **Layout und Navigation** | Kopf, Fuß, Navigation, die auf Telefonen funktioniert | erster |
| W-2 | **Startseite** | sagt auf einem Bildschirm, was es ist, und bietet einen klaren nächsten Schritt | erster |
| W-3 | **404-Seite** | mit Weg zurück und Suche oder Hauptlinks | erster |
| W-4 | **Responsiv und barrierefrei** | Telefon- bis Desktop-Breiten, WCAG 2.2 AA | erster |
| W-5 | **Suchmaschinen-Grundlagen** | Titel und Beschreibung je Seite, Sitemap, `robots.txt`, Vorschaubilder (Open Graph) | v1 |
| W-6 | **Rechtliche Seiten** | Datenschutzerklärung, Impressum, wo nötig; kein Cookie-Banner, solange nichts eine Einwilligung braucht | Release |
| W-7 | **Leistung** | optimierte Bilder, keine Layout-Verschiebung, Budget in der Prüfstraße gemessen | v1 |
| W-8 | **Kontakt** | ein Weg, die Menschen dahinter zu erreichen | v1 |
| W-9 | **Suche** | Volltextsuche | falls Dokumentation |
| W-10 | **Sprachen** | Sprachwechsel, übersetzte Seiten miteinander verknüpft | falls mehrere Sprachen |
| W-11 | **Statistik** | keine, oder datensparsam ohne personenbezogene Daten | falls gewünscht |
| W-12 | **Icons** | Favicon, Touch-Icon | erster |
| W-13 | **Bereitstellung** | automatischer Build und Vorschau je Änderung, Hosting dokumentiert | erster |

## Webdienst oder API (Server, Backend)

| ID | Baustein | Fertig, wenn | Standard |
|---|---|---|---|
| S-1 | **API-Vertrag** | maschinenlesbar (z. B. OpenAPI), versioniert, mit Beispielanfragen und -antworten | erster |
| S-2 | **Anmeldung und Berechtigungen** | wer was darf, ist getestet, nicht nur dokumentiert | falls Nutzer oder Clients |
| S-3 | **Eingabeprüfung und Fehlerformat** | jede Eingabe geprüft; ein Fehlerformat mit Code, Meldung und Hinweis | erster |
| S-4 | **Gesundheitsprüfung** | Health- und Readiness-Endpunkte für Überwachung und Orchestrierung | erster |
| S-5 | **Konfiguration** | über Umgebung oder Datei, dokumentiertes Beispiel, keine Geheimnisse im Repository | erster |
| S-6 | **Logs und Messwerte** | strukturierte Logs ohne personenbezogene Daten, Grundmesswerte (Anfragen, Fehler, Dauer) | v1 |
| S-7 | **Datenbank-Migrationen** | versionierte Migrationen, Startdaten für die Entwicklung | falls Datenbank |
| S-8 | **Grenzen** | Ratenbegrenzung, maximale Anfragegröße, Zeitlimits | v1 |
| S-9 | **Sicherung und Wiederherstellung** | Sicherung dokumentiert und eine Wiederherstellung tatsächlich geprobt | falls dauerhafte Daten |
| S-10 | **Bereitstellung** | Container-Image und ein Beispiel-Setup (z. B. Compose) mit Anleitung | v1 |
| S-11 | **Sicherheit** | TLS, Abhängigkeits-Updates, Sicherheits-Header, `SECURITY.md` | v1 |
| S-12 | **Datenschutz** | Aufbewahrungsfristen, Löschung, Übersicht der gespeicherten personenbezogenen Daten | falls personenbezogene Daten |
| S-13 | **Sauberes Herunterfahren** | laufende Anfragen werden beendet, keine halb geschriebenen Daten | v1 |
| S-14 | **Lokale Entwicklung** | startet mit einem Befehl, samt benötigter Dienste | erster |

## Tool (Kommandozeile)

| ID | Baustein | Fertig, wenn | Standard |
|---|---|---|---|
| C-1 | **Hilfe** | `--help` für das Tool und jeden Befehl, mit Beispielen; `--version` | erster |
| C-2 | **Fehlermeldungen** | sagen, was schiefging und was zu tun ist; auf stderr | erster |
| C-3 | **Exit-Codes** | dokumentiert und stabil; 0 nur bei Erfolg | erster |
| C-4 | **Konfiguration** | Datei, Umgebung und Optionen mit dokumentierter Rangfolge | v1 |
| C-5 | **Ausgabe für Skripte** | `--json` oder ein anderes stabiles Format, `--quiet` und `--verbose` | v1 |
| C-6 | **Farben** | nur im Terminal, `NO_COLOR` wird beachtet | v1 |
| C-7 | **Installation und Entfernung** | Pakete, Binärdateien oder Einzeiler; Entfernung dokumentiert | v1 |
| C-8 | **Shell-Vervollständigung** | bash, zsh, fish | Release |
| C-9 | **Referenz** | Manpage oder Referenzseite, aus den Hilfetexten erzeugt | Release |
| C-10 | **Plattformen** | Pfade, Zeilenenden und Kodierungen auf jedem unterstützten System korrekt | v1 |
| C-11 | **Abbruch** | Strg-C beendet sauber, keine halb geschriebenen Dateien | erster |
| C-12 | **Fortschritt** | Fortschrittsanzeige bei langen Vorgängen | v1 |
| C-13 | **Probelauf** | `--dry-run` für Vorgänge, die Daten ändern oder löschen | falls zerstörerische Vorgänge |

## Bibliothek oder SDK

| ID | Baustein | Fertig, wenn | Standard |
|---|---|---|---|
| L-1 | **Öffentliche Schnittstelle** | klar von Interna getrennt, jedes öffentliche Element dokumentiert, Referenz erzeugt | erster |
| L-2 | **Schnellstart** | von der Installation zum ersten funktionierenden Aufruf in unter 5 Minuten | erster |
| L-3 | **Beispiele** | ein Beispielordner, der in der Prüfstraße läuft | v1 |
| L-4 | **Versionierung und Abkündigung** | SemVer, Changelog, Abkündigungswarnungen mindestens eine Minor-Version vor dem Entfernen | v1 |
| L-5 | **Paket** | in der Registry der Sprache veröffentlicht, mit vollständigen Metadaten | Release |
| L-6 | **Unterstützte Versionen** | Laufzeit- und Sprachversionen als Matrix in der Prüfstraße getestet | v1 |
| L-7 | **Typen** | Typinformationen für die Sprachen, die sie nutzen | v1 |
| L-8 | **Fehler** | unterscheidbare Fehlertypen statt allgemeiner Ausnahmen oder Text | erster |
| L-9 | **Kein versteckter Zustand** | kein globaler Zustand; Threadsicherheit dokumentiert | erster |
| L-10 | **Abhängigkeiten** | so wenige wie möglich, Lizenzen verträglich | erster |
| L-11 | **Benchmarks** | reproduzierbare Messung in der Prüfstraße | falls Leistung wichtig |
| L-12 | **Migrationsanleitung** | für jede inkompatible Änderung | falls inkompatible Änderungen |

## Plugin oder Erweiterung (für ein Wirtsprogramm)

| ID | Baustein | Fertig, wenn | Standard |
|---|---|---|---|
| P-1 | **Kompatibilität** | unterstützte Versionen des Wirtsprogramms angegeben und getestet | erster |
| P-2 | **Installieren und Entfernen** | auf dem üblichen Weg des Wirtsprogramms; Entfernen hinterlässt nichts | erster |
| P-3 | **Einstellungen** | dort, wo das Wirtsprogramm Einstellungen hält, mit sinnvollen Standardwerten | v1 |
| P-4 | **Passt zum Wirtsprogramm** | folgt dessen UI-Richtlinien, Tastenkürzeln und Begriffen | v1 |
| P-5 | **Scheitert harmlos** | ein Fehler im Plugin reißt das Wirtsprogramm nie mit und wird verständlich gemeldet | erster |
| P-6 | **Marktplatz-Eintrag** | Beschreibung, Screenshots, Versionshinweise | Release |
