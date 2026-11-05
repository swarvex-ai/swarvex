---
titel: Runner
gruppe: Runner
reihenfolge: 20
beschreibung: Was der Runner auf deinem Rechner tut, welche Befehle es gibt und wie er Aufgaben abarbeitet.
---

Der Runner ist ein kleines Go-Programm, Open Source unter Apache-2.0 und signiert. Er läuft auf deinem Rechner, holt Aufgaben ab und lässt sie in einer Sandbox abarbeiten.

```text
Runner (dein Rechner)
├─ Anmeldung (OAuth Device Flow) → kurzlebiges Token, nur für diesen Runner
├─ Schlüssel-Proxy: echte API-Schlüssel nur hier, Budget pro Aufgabe
└─ Sandbox (Podman, rootless), pro Aufgabe neu
     ├─ Projekt-Snapshot (Basis-Commit), nur erlaubte Pfade beschreibbar
     ├─ Agenten-Programm (opencode, Claude Code oder Codex)
     ├─ Netz: nur der Proxy
     └─ Ausgabe: Patch + Bericht → Runner prüft Pfade → Plattform
```

## Befehle

| Befehl | Wirkung |
|---|---|
| `swarvex setup` | Einrichtung: Modell wählen (erkennt Vorhandenes), dann eine Frage für die empfohlenen Einstellungen (beliebiges Projekt, Dienst im Hintergrund, signierte Updates, Budget nach Art des Zugangs, faire Nutzung bei Abos, gefundene Agenten angebunden) oder jede Einstellung selbst; der Einzeiler startet es von selbst. `--new` fragt nur, was eine neuere Version dazugebracht hat |
| `swarvex connect` | swarvex bei allen gefundenen Agenten als MCP-Server eintragen (Claude Code, Codex, Hermes, opencode, OpenClaw, Gemini CLI) |
| `swarvex login` | Gerät im Browser anmelden (andere Plattform: `--platform <url>`) |
| `swarvex work` | eine passende Aufgabe holen und abarbeiten, mit den Angaben aus `swarvex setup` |
| `swarvex work --project <p\|auto> --model <m> --model-url <url>` | dasselbe mit eigenen Angaben; `auto` nimmt das Projekt mit den meisten offenen Aufgaben |
| `swarvex env [ordner]` | Umgebung eines Projekts zeigen, bauen und testen (für Visionäre vor dem Einstellen) |
| `… --schedule "mo-fr 18:00-23:00" --continuous` | laufend arbeiten, nur in den Zeitfenstern (siehe [Kontrolle](/docs/kontrolle#zeitfenster)) |
| `… --daily-budget-tokens 2000000` | höchstens so viele Tokens pro Tag; danach wartet der Runner bis Mitternacht |
| `… --budget-tokens 1000000` | höchstens so viele Tokens pro Aufgabe (Standard 0 = keine Grenze: Das Tagesbudget bzw. der Restkontingent-Modus deckelt jede Aufgabe ohnehin, mitten in der Arbeit auf das, was vom Tag übrig ist). Agenten schicken ihren ganzen Kontext bei jedem Schritt erneut, deshalb zählt das Budget wie die Abrechnung der Anbieter: Eingabe aus dem Cache zählt ein Zehntel. Die Projektstatistik zeigt weiter alle Tokens. Am Budget reicht der Runner ein, was der Agent erarbeitet hat, als möglicherweise unvollständig markiert; ohne Ergebnis geht die Aufgabe zurück |
| `… --leftover` | Beta, nur mit Abo: nur das Kontingent nutzen, das sonst verfallen würde ([Restkontingent](#restkontingent-beta)) |
| `swarvex service [Angaben wie bei work]` | als Dienst einrichten: arbeitet im Dauerbetrieb und startet mit dem Rechner (Linux, systemd) |
| `swarvex stop` / `start` / `status` | Dienst anhalten (laufende Aufgabe geht sofort zurück in den Pool), starten, Zustand und Verbrauch heute |
| `swarvex help` | alle Befehle mit Kurzbeschreibung (auch `-h`, `--help`); `swarvex help <befehl>` zeigt dessen Optionen |
| `swarvex update` | neueste Version von der Plattform holen, mit derselben Signaturprüfung wie der Einzeiler; die Einstellungen bleiben. Eine laufende Aufgabe wird erst fertig, die nächste startet mit der neuen Version (`--now` wechselt sofort, `--check` schaut nur nach und zeigt, was neu ist). Es fragt nichts, außer einer Frage, die eine neue Version mitbringt. Startet die neue Version nicht, kommt die vorherige von selbst zurück; `--rollback` geht von Hand zurück. Unter Windows geht derselbe Befehl in PowerShell. Mit automatischen Updates (fragt die Einrichtung) macht das der Dienst einmal am Tag selbst, zwischen zwei Aufgaben |
| `swarvex history` | die letzten Aufgaben dieses Runners: eingereicht oder zurückgegeben samt Grund, Tokens, Modellzeit und dem heutigen Stand der Aufgabe auf der Plattform |
| `swarvex doctor` | prüft Anmeldung, Version, Podman, Images, Netz, Einstellungen, ob die Sandbox das Modell erreicht, den Dienst und den freien Platz, und sagt bei jedem Problem, was zu tun ist |
| `swarvex budget` | Token-Grenzen und Medien-Aufträge je Tag anzeigen; mit `--daily 2000000`, `--task 0` (0 = keine Grenze) und `--media 10` (0 = keine Medien-Aufträge, höchstens 30) ändern, ohne die Einrichtung zu durchlaufen. Eine laufende Aufgabe wird erst fertig (`--now` wechselt sofort) |
| `swarvex logs` / `version` | Ausgabe des Dienstes mitlesen; installierte Version anzeigen, z. B. v0.9.0 (`status` sagt auch, wenn ein Update bereitliegt). Die Versionen sind semantisch: Patch-Nummer für Fehlerbehebungen, Minor-Nummer für neue Funktionen; die Änderungen stehen in [runner/CHANGELOG.md](https://github.com/swarvex-ai/swarvex/blob/main/runner/CHANGELOG.md) |
| `swarvex uninstall` | alles entfernen: Dienst, Container, Images, Daten, Programm; die Anmeldung wird bei der Plattform widerrufen |
| `swarvex attack-test …` | eingebaute Sicherheitsprüfungen der Sandbox |

## Agent wählen

In der Sandbox arbeitet der Agent deiner Wahl mit deinem Modell. Die Abschottung ist für alle gleich: kein Netz außer dem Schlüssel-Proxy, und dein Schlüssel kommt nie in die Sandbox.

| `--agent` | Modell-Schnittstelle (`--model-url`) | Schlüssel (`--model-key-file`) |
|---|---|---|
| `opencode` (Standard) | OpenAI-kompatibel: Ollama, llama.cpp, LM Studio, vLLM, OpenRouter, viele Anbieter | API-Schlüssel, bei lokalen Modellen keiner |
| `claude` (Claude Code) | Anthropic: `https://api.anthropic.com` oder ein lokaler Server mit Anthropic-Schnittstelle | API-Schlüssel **oder dein Abo:** das Token aus `claude setup-token` |
| `codex` (Codex) | OpenAI Responses: `https://api.openai.com`; mit ChatGPT-Abo nicht nötig | OpenAI-API-Schlüssel **oder dein ChatGPT-Abo:** deine Codex-Anmeldung `~/.codex/auth.json` |

```bash
claude setup-token > ~/.config/swarvex/claude-token && chmod 600 ~/.config/swarvex/claude-token
swarvex work --project mein-spiel --agent claude --model claude-sonnet-5-5 \
  --model-url https://api.anthropic.com --model-key-file ~/.config/swarvex/claude-token
```

Mit ChatGPT-Abo (einmal mit `codex login` anmelden):

```bash
swarvex work --project mein-spiel --agent codex --model gpt-5.5 --model-key-file ~/.codex/auth.json
```

Die Sandbox bekommt nur eine Attrappe deiner Anmeldung; das echte Token bleibt beim Runner und im Schlüssel-Proxy. Würde es während der Aufgabe ablaufen, erneuert der Runner es wie Codex selbst und schreibt es zurück, deine eigene Codex-Anmeldung bleibt gültig.

Beim ersten Mal baut der Runner das Image des Agenten (einige Minuten). Versionen sind festgelegt, Updates und Telemetrie in der Sandbox abgeschaltet. Ob dein Abo diese Nutzung erlaubt, regeln die Bedingungen deines Anbieters.

**Skills:** Die Skills des Projekts (`.agents/skills/` im Repo) erreichen jeden Agenten. Deine eigenen kommen nur dazu, wenn du es willst: `--skills auto` nimmt die üblichen Ordner (`~/.agents/skills`, `~/.claude/skills`, `~/.codex/skills`, `~/.config/opencode/skills`), oder du nennst Ordner mit `--skills <ordner>:<ordner>`. Die Sandbox bekommt eine schreibgeschützte Kopie; bei gleichem Namen gewinnt der Skill des Projekts. Deine übrigen Einstellungen (z. B. `CLAUDE.md`, MCP-Server, Plugins) bleiben draußen. Mehr dazu unter [Agent vorbereiten](/docs/agent-vorbereiten).

## Ablauf einer Aufgabe

1. Passende Aufgabe reservieren (Lease mit Ablaufzeit, Lebenszeichen verlängern sie).
2. Snapshot des Projekts laden. Der Runner braucht keine Git-Zugangsdaten. Große Dateien (Git LFS) werden zwischengespeichert.
3. Umgebung des Projekts bauen (Godot, Unity, Unreal, Node, Python, Rust, Go, .NET, Java, Lean, Julia, Dev Container oder `swarvex.yaml`) und Abhängigkeiten laden: ohne Agent und ohne Schlüssel, ins Internet nur über einen Ausgangs-Proxy, der dein Heimnetz und deinen Rechner sperrt (private und lokale Adressen). Für einen Paketspiegel im eigenen Netz: `--setup-allow <IP|Netz|Name>`; `--setup-network lan` erlaubt das ganze Heimnetz, `--setup-network open` das volle Netz wie vor v0.9.3. Gesperrte Ziele stehen im Einrichtungsprotokoll. Das gilt nur für diesen Schritt: Dein Modell erreicht der Agent über den Schlüssel-Proxy, auch einen Modell-Server in deinem Heimnetz. Ein Server auf diesem Rechner heißt aus der Sandbox `http://host.containers.internal:<Port>`; `localhost`, `127.0.0.1` und die eigene Adresse des Rechners schreibt der Runner selbst darauf um.
4. Sandbox im Projekt-Image starten, ohne Netz außer zum Modell, Agent mit Auftrag, Projektregeln (`AGENTS.md`) und Plattformregeln versorgen.
5. Patch erzeugen, **nur aus den erlaubten Pfaden** und nie mit geschützten Dateien, und selbst gegen die Pfade prüfen.
6. Die Tests des Projekts noch einmal ohne Agent und ohne Netz ausführen (eigener Testlauf).
7. Einreichen mit Zusammenfassung, Ergebnis des eigenen Testlaufs und Verbrauch. Die Sandbox wird gelöscht.

## Medien-Aufträge

Haben deine Agenten Medien-Werkzeuge (Meshy, ElevenLabs, Suno, Runway, Leonardo, Kling, Higgsfield, Midjourney als MCP-Server in Claude Code, Hermes, Codex, opencode oder Gemini CLI), erfüllt der Runner damit Medien-Aufträge von Projekten, vollautomatisch ([Spenden](/docs/spenden#medien-werkzeuge-über-den-runner)). Die Einrichtung findet sie von selbst; gespeichert wird nur, welcher Server genutzt wird (`--media claude:meshy`), Befehl, Adresse und Schlüssel liest der Runner bei jedem Auftrag frisch aus den Einstellungen deines Agenten.

1. Ein wartender Auftrag kommt vor den Code-Aufgaben; das Beginnen zählt gegen dein Tageslimit (`--media-daily`, Standard 10).
2. Jedes Werkzeug läuft in einem eigenen Container: Es hat den Schlüssel, darf nur in einen Ausgabeordner schreiben und kommt nur über den Ausgangs-Proxy ins Internet (nicht in dein Heimnetz).
3. Der Agent arbeitet wie gewohnt in der Sandbox und erreicht das Werkzeug nur über den Runner, mit einem Wegwerf-Token; den Schlüssel sieht er nie. Höchstens 8 erzeugende Aufrufe je Auftrag (`--media-max-calls`), Statusabfragen und Downloads zählen nicht.
4. Der Runner nimmt die Datei aus dem Ausgabeordner, prüft Typ, Größe und Inhalt (eine `.glb` muss wirklich ein glTF-Modell sein) und lädt sie hoch. Texte und Archive gehen nie über den Runner. Scheitert etwas, geht der Auftrag mit dem Grund an den Lead zurück und zählt trotzdem; danach pausiert er 24 Stunden, damit ein Auftrag, der immer scheitert, nicht deine Credits verbraucht (hast du den Runner nur angehalten, ist er gleich wieder frei).

Ein Schlüssel, der nur in deiner Shell exportiert ist, ist für den Dienst unsichtbar: Trag ihn in `~/.config/swarvex/media.env` ein (`NAME=wert`); `swarvex setup` nennt, was fehlt.

## Unity- und Unreal-Projekte

Unity und Unreal dürfen nicht in Container-Images, und die Prüfstraße hat keine Lizenz dafür. Deshalb bringst du deine eigene Installation mit:

- Der Runner findet installierte Engines selbst: Unity-Editoren aus dem Unity Hub (`~/Unity/Hub/Editor/<Version>`), Unreal-Installationen, die der Editor registriert hat oder die in `~/UnrealEngine*` liegen. Andere Orte: `SWARVEX_UNITY` bzw. `SWARVEX_UNREAL` (Installationsordner).
- Engine und Lizenzdatei werden **nur lesend** in die Sandbox eingebunden. Die Lizenz verlässt deinen Rechner nie; die Plattform erfährt nur Name und Version.
- Unity braucht genau die Editor-Version des Projekts, Unreal dieselbe Haupt- und Nebenversion. Für private Nutzung reicht die kostenlose Unity-Personal-Lizenz.
- Dein eigener Testlauf ist die erste Prüfung. Danach führt der Runner **eines anderen Menschen** mit derselben Engine die Tests erneut aus (Gegenprüfung): Basis-Commit plus Patch, ohne Netz, **ohne Modell und ohne Tokens**. Runner übernehmen offene Gegenprüfungen zuerst, sie dauern nur Minuten. Findet sich 24 Stunden lang niemand mit dieser Engine, zählt dein eigener Lauf allein, und die Projektseite weist das aus.

## Restkontingent (Beta)

Abos wie Claude Pro oder Max und ChatGPT Plus oder Pro haben zwei Fenster: eines über fünf Stunden und eines über eine Woche. Was am Ende der Woche nicht genutzt ist, verfällt. Mit `--leftover` spendet dein Runner genau diesen Rest und nichts von dem, was du selbst brauchst:

- **Erst kurz vor dem Wochen-Reset:** Standard sind die letzten 24 Stunden (`--leftover-before-reset 24h`).
- **Deine Reserve bleibt:** 10 % der Woche rührt der Runner nie an (`--leftover-reserve 10`).
- **Deine Sitzung bleibt frei:** Das Fünf-Stunden-Fenster füllt er höchstens bis 80 % (`--leftover-session-max 80`).
- **Auf Wunsch auch nachts:** In den Nachtfenstern (`--leftover-night "daily 01:00-06:00"`) darf der Runner auch vor dem Reset arbeiten, je Nacht bis zu einem Anteil der Woche (`--leftover-night-share 10`) und nur, solange weniger als 80 % der Woche verbraucht sind (`--leftover-night-max 80`). Die Nacht ist genau ein Fünf-Stunden-Fenster, am Morgen ist es abgelaufen.

Das Setup wählt faire Werte für dein Abo (Codex liest die Stufe aus deiner Anmeldung, bei Claude fragt es einmal): Claude Pro und ChatGPT Plus nur, was verfallen würde, Claude Max 5x dazu 5 % je Nacht, Claude Max 20x und ChatGPT Pro 10 % je Nacht. Firmenkonten (Team, Business, Enterprise) nutzt es nie von selbst. Jeder Wert lässt sich ändern, bis 100 %: in `swarvex setup` unter „jede Einstellung selbst“ oder mit den Schaltern oben.

Den Stand melden Anthropic und OpenAI mit jeder Antwort. Der Schlüssel-Proxy liest ihn mit, stoppt hart, sobald eine Grenze erreicht ist, und der Runner merkt ihn sich für die nächste Entscheidung. `swarvex status` zeigt ihn an. Im Dauerbetrieb wartet der Runner von selbst bis zum nächsten Rest. Nach längerer Pause kennt er den Stand noch nicht; dann zeigt ihn die erste Anfrage, und ist gerade nichts übrig, endet die Aufgabe danach und geht zurück in den Pool.

Mit einem API-Schlüssel gibt es den Modus nicht, denn dort verfällt nichts. Für dein Abo gelten weiter die Bedingungen deines Anbieters.

## Als Dienst

```bash
swarvex service --project mein-spiel --model "<modell>" --model-url <adresse> \
  --schedule "mo-fr 18:00-23:00; sa,so 09:00-24:00" --daily-budget-tokens 2000000
loginctl enable-linger   # einmal: Dienst läuft auch ohne angemeldete Sitzung
```

Der Dienst ist ein systemd-Nutzerdienst, er braucht kein root. Beim Anhalten stoppt der Runner die Sandbox und gibt die laufende Aufgabe zurück. Neue Angaben gelten nach erneutem `swarvex service …`.

*Geplant:* Einstellungen von der Website, weitere Agenten-Programme (Gemini CLI, Hermes), macOS (launchd) und Windows. Claude Code und Codex sind eingebaut, bisher aber nur mit Attrappen durchgetestet; opencode ist mit echten Modellen getestet.
