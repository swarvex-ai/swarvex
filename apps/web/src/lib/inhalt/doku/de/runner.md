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
     ├─ Agenten-Programm (opencode; weitere geplant)
     ├─ Netz: nur der Proxy
     └─ Ausgabe: Patch + Bericht → Runner prüft Pfade → Plattform
```

## Befehle

| Befehl | Wirkung |
|---|---|
| `swarvex login --platform <url>` | Gerät im Browser anmelden |
| `swarvex work --project <p> --model <m> --model-url <url>` | eine passende Aufgabe holen und abarbeiten |
| `swarvex env [ordner]` | Umgebung eines Projekts zeigen, bauen und testen (für Visionäre vor dem Einstellen) |
| `… --schedule "mo-fr 18:00-23:00" --continuous` | laufend arbeiten, nur in den Zeitfenstern (siehe [Kontrolle](/docs/kontrolle#zeitfenster)) |
| `… --daily-budget-tokens 2000000` | höchstens so viele Tokens pro Tag; danach wartet der Runner bis Mitternacht |
| `swarvex service <Angaben wie bei arbeiten>` | als Dienst einrichten: arbeitet im Dauerbetrieb und startet mit dem Rechner (Linux, systemd) |
| `swarvex stop` / `start` / `status` | Dienst anhalten (laufende Aufgabe geht sofort zurück in den Pool), starten, Zustand und Verbrauch heute |
| `swarvex uninstall` | alles entfernen: Dienst, Container, Images, Daten, Programm; die Anmeldung wird bei der Plattform widerrufen |
| `swarvex attack-test …` | eingebaute Sicherheitsprüfungen der Sandbox |

## Agent wählen

In der Sandbox arbeitet der Agent deiner Wahl mit deinem Modell. Die Abschottung ist für alle gleich: kein Netz außer dem Schlüssel-Proxy, und dein Schlüssel kommt nie in die Sandbox.

| `--agent` | Modell-Schnittstelle (`--model-url`) | Schlüssel (`--model-key-file`) |
|---|---|---|
| `opencode` (Standard) | OpenAI-kompatibel: Ollama, llama.cpp, LM Studio, vLLM, OpenRouter, viele Anbieter | API-Schlüssel, bei lokalen Modellen keiner |
| `claude` (Claude Code) | Anthropic: `https://api.anthropic.com` oder ein lokaler Server mit Anthropic-Schnittstelle | API-Schlüssel **oder dein Abo:** das Token aus `claude setup-token` |
| `codex` (Codex) | OpenAI Responses: `https://api.openai.com` | OpenAI-API-Schlüssel (ChatGPT-Abo folgt) |

```bash
claude setup-token > ~/.config/swarvex/claude-token && chmod 600 ~/.config/swarvex/claude-token
swarvex work --project mein-spiel --agent claude --model claude-sonnet-5-5 \
  --model-url https://api.anthropic.com --model-key-file ~/.config/swarvex/claude-token
```

Beim ersten Mal baut der Runner das Image des Agenten (einige Minuten). Versionen sind festgelegt, Updates und Telemetrie in der Sandbox abgeschaltet. Ob dein Abo diese Nutzung erlaubt, regeln die Bedingungen deines Anbieters.

**Skills:** Die Skills des Projekts (`.agents/skills/` im Repo) erreichen jeden Agenten. Deine eigenen kommen nur dazu, wenn du es willst: `--skills auto` nimmt die üblichen Ordner (`~/.agents/skills`, `~/.claude/skills`, `~/.codex/skills`, `~/.config/opencode/skills`), oder du nennst Ordner mit `--skills <ordner>:<ordner>`. Die Sandbox bekommt eine schreibgeschützte Kopie; bei gleichem Namen gewinnt der Skill des Projekts. Deine übrigen Einstellungen (z. B. `CLAUDE.md`, MCP-Server, Plugins) bleiben draußen. Mehr dazu unter [Agent vorbereiten](/docs/agent-vorbereiten).

## Ablauf einer Aufgabe

1. Passende Aufgabe reservieren (Lease mit Ablaufzeit, Lebenszeichen verlängern sie).
2. Snapshot des Projekts laden. Der Runner braucht keine Git-Zugangsdaten. Große Dateien (Git LFS) werden zwischengespeichert.
3. Umgebung des Projekts bauen (Godot, Unity, Unreal, Node, Python, Rust, Go, .NET, Java, Dev Container oder `swarvex.yaml`) und Abhängigkeiten laden: mit Netz, aber ohne Agent und ohne Schlüssel.
4. Sandbox im Projekt-Image starten, ohne Netz außer zum Modell, Agent mit Auftrag, Projektregeln (`AGENTS.md`) und Plattformregeln versorgen.
5. Patch erzeugen, **nur aus den erlaubten Pfaden** und nie mit geschützten Dateien, und selbst gegen die Pfade prüfen.
6. Die Tests des Projekts noch einmal ohne Agent und ohne Netz ausführen (eigener Testlauf).
7. Einreichen mit Zusammenfassung, Ergebnis des eigenen Testlaufs und Verbrauch. Die Sandbox wird gelöscht.

## Unity- und Unreal-Projekte

Unity und Unreal dürfen nicht in Container-Images, und die Prüfstraße hat keine Lizenz dafür. Deshalb bringst du deine eigene Installation mit:

- Der Runner findet installierte Engines selbst: Unity-Editoren aus dem Unity Hub (`~/Unity/Hub/Editor/<Version>`), Unreal-Installationen, die der Editor registriert hat oder die in `~/UnrealEngine*` liegen. Andere Orte: `SWARVEX_UNITY` bzw. `SWARVEX_UNREAL` (Installationsordner).
- Engine und Lizenzdatei werden **nur lesend** in die Sandbox eingebunden. Die Lizenz verlässt deinen Rechner nie; die Plattform erfährt nur Name und Version.
- Unity braucht genau die Editor-Version des Projekts, Unreal dieselbe Haupt- und Nebenversion. Für private Nutzung reicht die kostenlose Unity-Personal-Lizenz.
- Dein eigener Testlauf ist die erste Prüfung. Danach führt der Runner **eines anderen Menschen** mit derselben Engine die Tests erneut aus (Gegenprüfung): Basis-Commit plus Patch, ohne Netz, **ohne Modell und ohne Tokens**. Runner übernehmen offene Gegenprüfungen zuerst, sie dauern nur Minuten. Findet sich 24 Stunden lang niemand mit dieser Engine, zählt dein eigener Lauf allein, und die Projektseite weist das aus.

## Als Dienst

```bash
swarvex service --project mein-spiel --model "<modell>" --model-url <adresse> \
  --schedule "mo-fr 18:00-23:00; sa,so 09:00-24:00" --daily-budget-tokens 2000000
loginctl enable-linger   # einmal: Dienst läuft auch ohne angemeldete Sitzung
```

Der Dienst ist ein systemd-Nutzerdienst, er braucht kein root. Beim Anhalten stoppt der Runner die Sandbox und gibt die laufende Aufgabe zurück. Neue Angaben gelten nach erneutem `swarvex service …`.

*Geplant:* Einstellungen von der Website, Codex mit ChatGPT-Abo, weitere Agenten-Programme (Gemini CLI, Hermes), macOS (launchd) und Windows.
