# Agenten-Kompatibilitätstest

Wir leben davon, dass sich möglichst viele Agenten möglichst einfach anbinden lassen. Dieser Test meldet **echte Agenten-Programme** per MCP-OAuth an swarvex an und prüft, dass sie danach die Werkzeuge sehen.

| Datei | Inhalt |
|---|---|
| `compose.yaml` | Testinstanz: eigene Datenbank (im Arbeitsspeicher), API, Attrappe eines Konto-Anbieters |
| `konto_attrappe.py` | Gitea-artiger Konto-Anbieter, der jede Anmeldung sofort für das Testkonto „kompat“ genehmigt |
| `browser.py` | Ersatz-Browser: folgt der Anmelde-URL wie ein Mensch bis zum Rücksprung an den Agenten |
| `agenten.py` | Testlauf: pro Agent leeres Konfig-Verzeichnis, Einrichtung, Anmeldung, Prüfung; Ergebnis in `ergebnis.md` |

## Ablauf

```sh
# Testinstanz auf der Plattform-VM starten (getrennt vom Labor-Stack)
ssh swarvex 'cd /opt/swarvex && docker compose -f kompatibilitaet/compose.yaml up -d --build'
# Tunnel: Agenten sehen die Instanz als http://localhost:3800 (MCP erlaubt HTTP nur auf localhost)
ssh -f -N -L 3800:localhost:3800 -L 3999:localhost:3999 swarvex
# optional: weitere Agenten nur hier installieren (ignoriert von Git)
(cd kompatibilitaet/werkzeuge && npm i opencode-ai openclaw @google/gemini-cli)
kompatibilitaet/agenten.py            # oder einzelne: agenten.py claude codex-dcr
# danach aufräumen
ssh swarvex 'cd /opt/swarvex && docker compose -f kompatibilitaet/compose.yaml down'
```

Die echte Einrichtung des Rechners bleibt unberührt: Jeder Agent bekommt ein eigenes `HOME` und eigene Konfig-Verzeichnisse unter `ausgabe/lauf-*`.

## Stand 25.10.2026

| Agent | Version | Registrierung | Ergebnis |
|---|---|---|---|
| Claude Code | 2.1.310 | Client-ID-Metadata-Dokument | ✔ verbunden, Werkzeuge sichtbar |
| Codex | 0.154.0 | Metadata-Dokument (Standard) und dynamisch (`--oauth-client-registration dcr`) | ✔ beide Wege |
| Hermes | 0.20.6 | Metadata-Dokument | ✔ 8 Werkzeuge |
| opencode | 1.19.4 | dynamisch | ✔ verbunden |
| OpenClaw | 2026.10.5 | dynamisch | ✔ 8 Werkzeuge (nur mit `--transport streamable-http`) |
| Gemini CLI | 0.65.0 | – | Einrichtung ✔; Anmeldung nur interaktiv (`/mcp auth`), nicht automatisierbar |

Gefunden und behoben beim ersten Lauf:

- Claude Code nennt im Metadata-Dokument `http://localhost/callback` ohne Port und nutzt dann einen zufälligen Port. Nach RFC 8252 ist das für lokale Programme richtig; der Server behandelt Clients mit reinem Loopback-Rücksprung deshalb als „native“.
- Ohne Unterstützung für Metadata-Dokumente hätten sich Claude Code und Hermes nicht wie vorgesehen angemeldet; der Agent ist jetzt das Paar (Mensch, Client), weil sich alle Nutzer eines Programms eine Client-ID teilen.
