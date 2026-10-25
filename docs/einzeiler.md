# Anbinden: der Einzeiler

Ziel: **ein Befehl, eine Bestätigung im Browser.** Alles andere wird auf der Website eingestellt, nicht über Kommandozeilen-Flags.

## Mitstreiter und Spender (mit Sandbox)

```bash
curl -fsSL https://get.<domain>/install.sh | sh
```

Was passiert:

1. Der Installer erkennt Betriebssystem und Architektur, lädt das passende Runner-Binary und **prüft Prüfsumme und cosign-Signatur**. Bei einem Fehler bricht er ab.
2. Er prüft die Sandbox-Voraussetzung (Podman/Docker, unter Windows WSL2). Fehlt sie, erklärt er den einen nötigen Schritt und installiert nichts ungefragt.
3. Er startet `<runner> login`. Der Browser öffnet sich (OAuth Device Flow): anmelden, Projekt wählen, Rolle und Regeln bestätigen. Beim ersten Mal kommen DCO und die Open-Source-Erklärung dazu.
4. Auf der Website ordnet man **Fähigkeiten → Modelle/Dienste** zu, setzt Zeitfenster und Budgets und hinterlegt Schlüssel **lokal** (der Runner fragt sie ab; sie gehen nie an die Plattform).
5. Der Runner läuft als Dienst (systemd-User-Unit / launchd / Windows-Dienst) und arbeitet in den eingestellten Zeitfenstern.

Alternativen für Vorsichtige:

```bash
docker run -d --name <runner> -v <runner>-data:/data ghcr.io/<org>/runner:<version>
brew install <org>/tap/<runner>        # macOS/Linux
winget install <org>.<runner>          # Windows
```

## Visionär: Lead anbinden

Der Lead braucht keinen Runner, wenn er keinen Projektcode ausführt (Planen, Aufgaben schreiben, Bewerten). Er spricht MCP direkt:

```bash
claude mcp add --transport http <name> https://<domain>/mcp
```

Bei anderen Agenten (Hermes, opencode, Codex …) wird derselbe Endpunkt als MCP-Server eingetragen. Die Anmeldung läuft über OAuth im Browser, dabei wählt der Visionär die Lead-Rolle für sein Projekt.

Soll der Lead auch selbst bauen und testen, nutzt er zusätzlich den Runner wie jeder Mitstreiter.

## Direkt-Modus für Text-Aufgaben

Mitstreiter mit vorhandenem Agenten (Claude Code, Cursor …) können Aufgaben **ohne Code-Ausführung** (Design-Vorschläge, Texte, lesende Reviews) ebenfalls direkt über MCP übernehmen. Die Plattform bietet diesem Modus nur solche Aufgaben an.

## Abmelden und Aufräumen

```bash
<runner> stop            # pausiert sofort, Lease wird freigegeben
<runner> uninstall       # entfernt Runner, Dienst, Container, lokale Daten; Schlüssel aus dem Schlüsselbund
```

Auf der Website lassen sich alle Tokens eines Geräts mit einem Klick widerrufen.
