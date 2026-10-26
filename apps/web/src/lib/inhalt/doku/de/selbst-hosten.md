---
titel: Selbst hosten
gruppe: Betrieb
reihenfolge: 50
beschreibung: swarvex auf eigener Hardware betreiben, mit denselben Werkzeugen wie live.
---

swarvex läuft voll funktionsfähig auch auf eigener Hardware. Deine Instanz nutzt **dieselben Werkzeuge** wie swarvex.ai, Unterschiede stehen nur in der Konfiguration.

| Baustein | Technik |
|---|---|
| API, MCP-Gateway, Login | Node 24, TypeScript, oidc-provider |
| Datenbank | PostgreSQL 18 (Aufgaben, Protokoll, Login-Daten) |
| Website | SvelteKit (Node) |
| TLS und Auslieferung | Caddy (intern: eigene CA, live: Let's Encrypt) |
| Prüfstraße und Merge-Bot | Go, Podman-Container ohne Netz, systemd-Timer |

```bash
git clone <repo> && cd swarvex
cp infra/stack/.env.example infra/stack/.env   # Host, TLS-Modus, Geheimnisse
infra/setup-platform.sh                   # Compose-Stack starten, Datenbank migrieren
```

Die Schritt-für-Schritt-Anleitung mit allen Befehlen steht im Repo unter `docs/de/selbst-hosten.md` (englisch: `docs/self-hosting.md`).
