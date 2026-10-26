# Konzept

Wie die Koordination funktioniert: Rollen, Aufgaben, Spenden, Vertrauen, Erwartungen. Technik steht in `architektur.md`, offene Entscheidungen stehen in `entscheidungen.md`.

## 1. Rollen

| Rolle | Wer | Läuft wo | Darf |
|---|---|---|---|
| **Visionär** | Mensch, Projekteigner | – | Vision und Regeln festlegen, Lead bestimmen, Releases und Lizenzfragen freigeben, Architekturentscheidungen treffen |
| **Lead** | Agent des Visionärs (beliebiges Modell) | Hardware/Abo des Visionärs | Aufgaben anlegen, Verträge und Tests festschreiben, Ergebnisse annehmen oder ablehnen, Merges vorschlagen, Asset-Aufträge stellen |
| **Bereichs-Lead** | Agent eines Mitstreiters auf Vertrauensstufe 3 | dessen Hardware | wie Lead, beschränkt auf einen Bereich (z. B. „Gegner“) |
| **Mitstreiter** | Mensch mit einem oder mehreren Agenten | eigene Hardware | Aufgaben passend zu Stufe und Fähigkeiten übernehmen |
| **Spender** | Mensch mit Credits/Kapazität | eigene Konten/Hardware | Aufträge mit eigenem Budget ausführen lassen |
| **Plattform** | wir | eigener Server | Regeln durchsetzen, vermitteln, protokollieren – entscheidet nichts Inhaltliches |

Mitstreiter und Spender sind dieselben Menschen in unterschiedlichen Rollen. Ein Konto kann beides sein.

**Ruf und Stimmen hängen am Menschen, nie am Agenten.** 50 registrierte Agenten einer Person bringen ihr keinen Vorteil.

## 2. Die Lead-Rolle

Der Visionär bringt seinen Lead mit: Claude über ein Abo oder die API, Qwen über OpenRouter mit Hermes, ein lokales Modell, was er will. Die Plattform beschreibt die Lead-Rolle als **Schnittstelle** (MCP-Werkzeuge, siehe Architektur). Wer sie bedient, ist frei.

Wie der Lead die Rolle ausfüllt, steht im **Lead-Handbuch** (`lead/LEAD.md` mit Vorlagen). Das umfasst Ideenaufnahme, Fundament, Planung, Aufgaben schneiden, Steuern, Bewerten und Release. Jeder Lead lädt es als Rollenanweisung.

### Was den Visionär erwartet (Erwartungsrechner)

Beim Anlegen eines Projekts zeigt die Plattform die zu erwartende Lead-Last:

- Aufgaben pro Tag, die geschnitten werden müssen (abhängig von der Zielgröße des Schwarms)
- Ergebnisse pro Tag, die zu prüfen sind
- Architekturarbeit pro Woche (Verträge, Schnittstellen, Umbauten)
- welche Lead-Konfigurationen eine vergleichbare Last in anderen Projekten getragen haben

Am Anfang beruhen diese Zahlen auf dem Pilotprojekt Rend&Run, später auf echten Projektdaten. **Die Plattform empfiehlt keine Abos und verspricht nichts.** Sie zeigt gemessene Last. Ob ein Abo reicht und ob dessen Nutzungsbedingungen den Einsatz erlauben, beurteilt der Visionär selbst.

### Lead-Test

Ein standardisierter Satz Prüfaufgaben mit sichtbarem Ergebnis auf der Projektseite:

1. Ein Feature in Aufgaben zerlegen (Umfang, Akzeptanzkriterien, Abhängigkeiten).
2. Fünf Beiträge prüfen, in drei davon sind Fehler versteckt.
3. Eine Schnittstelle für ein neues Modul entwerfen (z. B. einen Gegnertyp).
4. Eine Asset-Auftragsbeschreibung schreiben, die ein schwaches Modell korrekt ausführen kann.

Der Test verbietet nichts. Er macht sichtbar, was ein Lead kann. Kombinationen sind erlaubt, etwa ein günstiges Modell für Routine und ein starkes für Architektur.

## 3. Aufgaben

### Stufen

| Stufe | Art | Beispiel | Geeignet für |
|---|---|---|---|
| **A** | automatisch prüfbar, ohne Projektcode-Änderung | Asset erzeugen, Simulation laufen lassen, Balancing-Lauf, Screenshot-Regression | alle, auch schwache lokale Modelle und reine Credit-Spender |
| **B** | isoliertes Modul hinter fertiger Schnittstelle | neuer Effekt, HUD-Element, Gegner-Variante, Tests für bestehenden Code | Coder-Modelle |
| **C** | Änderung innerhalb eines Systems, abgesichert durch Vertragstests | neuer Angriff für bestehenden Gegner, Waffen-Mod | starke Coder-Modelle |
| **D** | Querschnitt, Architektur | Schadenssystem umbauen | nur Lead/Bereichs-Lead |

**Die eigentliche Lead-Arbeit:** Code so schneiden, dass möglichst viel Arbeit von D nach B rutscht, also Erweiterungspunkte statt Monolithen, Verträge und Tests zuerst. Ohne diese Architekturarbeit gibt es keinen Schwarm.

### Aufgabenbeschreibung (Kurzfassung, Schema in `architektur.md`)

- Ziel, Kontext und Verweise auf Vision und Architekturentscheidungen
- **Rolle und Fähigkeiten** (z. B. `code.gdscript`, `vision.image`, `asset.3d`)
- erlaubte Pfade bzw. Ausgabeformat
- Akzeptanzkriterien, möglichst als ausführbarer Prüfvertrag
- Zeit- und Kostenrahmen
- Stufe A–D und Mindest-Vertrauensstufe

### Wettbewerb bei Überkapazität

Gibt es mehr Agenten als Aufgaben, darf der Lead eine Aufgabe **mehrfach vergeben**. Die beste Lösung, die die Prüfung besteht, gewinnt. Spenden sind so nie verschwendet, und schwache Modelle bremsen nichts.

### Lebenszyklus

```
entwurf → offen → reserviert → in_arbeit → eingereicht → prüfung
   → (geprüft_ok → angenommen → gemerged) | (geprüft_fehler | abgelehnt → offen/geschlossen)
reserviert/in_arbeit ohne Lebenszeichen bis Lease-Ende → offen (zurück in den Pool)
```

Jeder Übergang ist ein protokolliertes Ereignis (siehe `nachvollziehbarkeit.md`).

### Regelebenen für Agenten

| Ebene | Wer schreibt sie | Wie sie zum Agenten kommt | Verbindlichkeit |
|---|---|---|---|
| **Plattformregeln** | Plattform | der Runner setzt sie in jeden Auftrag (nicht aus dem Repo) | gelten immer; die harten Teile (Pfade, Netz, Budget, Prüfpflicht) setzt zusätzlich die Technik durch |
| **Projektregeln** | Visionär/Lead | `AGENTS.md` im Projekt (Standardname, den opencode, Codex, Claude Code u. a. selbst lesen; auch pro Unterordner) | ergänzen die Plattformregeln, heben sie nie auf |
| **Aufgabe** | Lead | Aufgabenbeschreibung, Akzeptanzkriterien, erlaubte Pfade | gilt für diese Aufgabe |

Die `AGENTS.md` eines Projekts ist fremder Inhalt wie jeder andere Projektcode und könnte auch bösartige Anweisungen enthalten. Dagegen schützt die Sandbox, nicht der Text.

### Review als Aufgabe

Ein KI-Review ist **kein eigener Dienst der Plattform**, sondern eine Aufgabenart, die Mitstreiter mit der Fähigkeit `review` übernehmen (Entscheidung 24.10.2026):

1. **Nie die eigene Arbeit:** Reviewer und Autor sind verschiedene Menschen.
2. **Ein Review berät nur:** Ergebnis ist ein strukturierter Bericht (geprüfte Kriterien, Befunde, Belege). Entscheiden tun weiterhin Prüfstraße und Lead.
3. **Ein Review muss überprüfbar sein:** Die Plattform mischt gelegentlich Einreichungen mit bekannten Fehlern unter (wie beim Lead-Test). Wer sie übersieht, verliert Ansehen.
4. **Kein Kreis aus Fake-Konten:** Das Ansehen hängt am Menschen, Selbst-Reviews über Zweitkonten bringen nichts.
5. **Der Lead entscheidet**, ob eine Einreichung ein Review bekommt (Stufe A selten, Stufe C in der Regel).

So entlastet das Review den Engpass beim Lead. Spender mit einem guten Reasoning-Modell, aber ohne Coder-Modell, können damit sinnvoll beitragen.

## 4. Spezialisierte Modelle

Aufgaben verlangen **Fähigkeiten**, keine Modelle. Der Mitstreiter ordnet auf der Website einmal zu, welches seiner Modelle welche Fähigkeit bedient:

```
code.*          → qwen3-coder (lokal, Ollama)
vision.*/design → claude (eigener API-Key)
asset.3d        → meshy (eigenes Konto, 200 Credits/Monat)
video.*         → manuell (Higgsfield im Browser, Upload)
```

Die Plattform vermittelt nur Aufgaben, deren Fähigkeiten eine Zuordnung haben. Ob wirklich das genannte Modell lief, ist nicht beweisbar. Der Ruf hängt deshalb an der **Bilanz pro Zuordnung** („qwen3-coder von Nutzer X: 70 % angenommen“).

## 5. Spenden

Allgemein gilt: **Fähigkeiten-Spende.** Jemand hat Zugang zu etwas, das aus einem Auftrag ein Ergebnis macht.

| Weg | Wie | Wann |
|---|---|---|
| **Adapter** | Der Runner ruft den Dienst mit dem Schlüssel des Spenders auf (LLM, Meshy …). Vollautomatisch. | häufige Dienste |
| **Auftrag + Upload** | Der Spender bekommt die Auftragsbeschreibung, bedient den Dienst selbst (z. B. Higgsfield im Browser) und lädt das Ergebnis hoch. | jeder Dienst, ab Tag 1 |

Regeln:

1. **Die Obergrenze liegt beim Spender.** Er legt Budgets fest (Credits/Monat, Tokens/Tag, Zeitfenster). Kein Lead kann mehr ausgeben.
2. **Kosten vorher angeben.** Jeder Auftrag nennt eine Schätzung („~30 Credits“).
3. **Teure Aufträge in zwei Schritten:** günstige Vorschau → Freigabe durch den Lead → Finale. Ausgegebene Credits sind weg.
4. **Rechte liegen bei den Beitragenden:** Jeder Beitragende versichert, alle Rechte an seinem Beitrag zu halten. Die Plattform prüft technisch, nicht rechtlich. Für bekannte Dienste gibt es einen unverbindlichen Hinweis, welche Tarife das in der Regel erfüllen (siehe `beitragserklaerung.md`).
5. **Sichtbarer Dank:** Jeder angenommene Beitrag erscheint mit Namen auf der Projektseite und in den Credits des Spiels.

Schlüssel des Spenders verlassen nie seinen Rechner (Schlüssel-Proxy, siehe Architektur).

## 5a. Kontrolle beim Beitragenden

Die Koordination läuft zentral über die Plattform. **Den Rahmen, in dem ein Agent mitarbeitet, legt aber allein der Beitragende fest** (Vorgabe neVs, 25.10.2026). Die Plattform und der Lead können diesen Rahmen nicht erweitern.

| Was der Beitragende bestimmt | Wo |
|---|---|
| Für welche **Projekte** sein Agent arbeitet | Website (Zusage pro Projekt) |
| Welche **Aufgabenarten und Stufen** (z. B. nur Tests und Assets, keine Gameplay-Änderungen) | Website, im Runner gespiegelt |
| Welches **Modell bzw. welcher Dienst** welche Fähigkeit bedient | Website (Zuordnungen), Schlüssel nur lokal |
| **Wann:** frei wählbare Zeitfenster pro Wochentag, auch über Mitternacht (z. B. `mo-fr 18:00-23:00; sa,so 09:00-24:00`) | Website, durchgesetzt im Runner (`--zeitfenster`, `--dauerbetrieb`) |
| **Wie viel:** Token-, Credit- und Zeitbudget pro Aufgabe, Tag und Monat; Anzahl paralleler Aufgaben | Website, durchgesetzt im Runner und im Proxy |
| **Freigabe vor dem Einreichen** (optional): Der Patch wird erst nach Durchsicht und Bestätigung des Beitragenden eingereicht | Runner-Einstellung |
| **Jederzeit stoppen:** pausieren, laufende Aufgabe abbrechen, Gerät abmelden, Runner deinstallieren | Runner (`stop`, `uninstall`), Website („Mein Konto“: Agent abmelden) |

Grundsätze:

- **Der Runner holt Aufgaben ab, die Plattform schiebt nichts auf den Rechner.** Es gibt keinen Fernzugriff und keinen offenen Port. Ausgeführt wird nur die Aufgabe, in der Sandbox.
- **Grenzen werden lokal durchgesetzt.** Budgets, Zeitfenster und Aufgabenarten prüft der Runner selbst. Eine manipulierte Plattform könnte sie nicht aushebeln.
- **Volle Einsicht.** Das Dashboard „Meine Beiträge“ zeigt jede Aufgabe, jeden Verbrauch und jede Bewertung (`nachvollziehbarkeit.md`).

## 6. Vertrauen

| Stufe | Darf | Aufstieg (Startwerte, justierbar) |
|---|---|---|
| 0 – neu | Aufgaben der Stufe A; Ergebnisse werden doppelt geprüft | 10 angenommene Beiträge |
| 1 | zusätzlich B | 25 angenommen, keine Regression |
| 2 | zusätzlich C | Einladung durch den Lead |
| 3 | Bereichs-Lead | Einladung durch den Visionär |

- Eine Regression oder ein Regelverstoß bedeutet sofortigen Abstieg. Wiederholter Missbrauch führt zur Sperre.
- Die Identität ist an ein Konto mit Historie gebunden (GitHub/Codeberg/Gitea-Login mit Mindestalter, 2FA).
- Ratenlimits gibt es pro Mensch, nicht pro Agent.
- Credit-Spenden (Stufe A) sind für die Plattform risikoarm und ab Stufe 0 offen.

## 7. Kennzahlen (die Qualitätssicherung über freie Wahl)

Pro Projekt, öffentlich:

- Annahmequote ohne Nacharbeit, mittlere Prüfzeit, verworfene Spenden (Credits/Zeit)
- Ergebnis des Lead-Tests, Lead-Konfiguration
- Aktivität: offene Aufgaben, Beiträge pro Woche, letzte Releases

Pro Mitstreiter, öffentlich (abschaltbar bis auf die Vertrauensstufe):

- angenommene Beiträge, Regressionen, Bilanz pro Modell-Zuordnung

Die Kennzahlen meldet die Prüfstraße des Projekts. Die Plattform zieht Stichproben und prüft sie selbst nach (siehe Architektur, Prüfstraße).

## 8. Lizenz und Eigentum

- Projekte auf der Plattform sind **Open Source**. Bei der ersten Teilnahme akzeptiert jeder ausdrücklich: „Ich investiere in ein Open-Source-Projekt; meine Beiträge stehen unter dessen Lizenz.“
- Beiträge laufen über das **DCO** (Developer Certificate of Origin, `Signed-off-by`), kein CLA.
- Code-Lizenz: Empfehlung MIT, Alternative GPLv3. Das wählt jedes Projekt selbst, die Plattform zeigt es deutlich an.
- Assets: CC BY 4.0 als Empfehlung (Namensnennung passt zu „gespendet von …“).
- Namen und Logos sind von der Open-Source-Lizenz nicht erfasst. Der Visionär behält die Marke.
- Einnahmen: Projekte erklären vorab und schriftlich, was mit Einnahmen passiert (z. B. „kostenlos“ oder „finanziert Lead und Infrastruktur“). Die Plattform rät von Gewinnbeteiligungen ab.
- Plattform, Runner und Protokoll sind selbst Open Source. Der **Runner muss offen sein**: Er läuft auf fremden Rechnern und hat Zugang zu fremden Schlüsseln.
