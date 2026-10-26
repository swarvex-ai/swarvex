# swarvex selbst hosten

Die Plattform läuft vollständig auf eigener Hardware. Die öffentliche Instanz swarvex.ai nutzt dieselben Werkzeuge und Skripte; Unterschiede stehen nur in der Konfiguration (`.env`).

## 1. Bausteine

| Baustein | Wo | Technik |
|---|---|---|
| Plattform | ein Server (Debian 13, ab 4 Kerne, 8 GB) | Docker Compose: PostgreSQL 18, API + MCP + OAuth (Node 24), Website (SvelteKit), Caddy (TLS) |
| Prüfstraße und Merge-Bot | beim Visionär bzw. auf dem Server, als systemd-Timer | `swarvex-pruefstrasse` (Go), Prüf-Image mit der Engine des Projekts |
| Mitstreiter | eigener Rechner (Linux) | `swarvex` (Go), rootless Podman, Sandbox- und Proxy-Image |
| Konto-Anbieter | GitHub, Codeberg oder eigenes Gitea/Forgejo | OAuth-App für den Login |
| Projekt-Repo | beliebiger Git-Server mit SSH | Deploy-Keys |

## 2. Code auf den Server

```bash
git clone https://github.com/swarvex-ai/swarvex.git
rsync -az --exclude node_modules --exclude .git swarvex/ root@<server>:/opt/swarvex/
```

## 3. Konto-Anbieter und Projekt-Repo

1. **OAuth-App** beim Konto-Anbieter (mindestens einer):
   - Rücksprung-Adresse: `https://<BASIS_HOST>/interaction/callback`
   - GitHub: *Settings → Developer settings → OAuth Apps*; Codeberg/Gitea/Forgejo: *Einstellungen → Anwendungen → OAuth2* (vertraulich)
   - Client-ID und Secret kommen in die `.env` (`GITHUB_*`, `CODEBERG_*` oder `GITEA_*`), nirgends sonst hin.
2. **Projekt-Repo** anlegen und den Stand hineinschieben (inklusive Git-LFS).
3. **Deploy-Keys** im Projekt-Repo eintragen. Die drei öffentlichen Schlüssel gibt `einrichten-plattform.sh` am Ende aus:
   - Snapshot (Plattform): **nur lesen**
   - Prüfstraße: **nur lesen**
   - Merge-Bot: **schreiben**. Er ist der einzige Schlüssel mit Schreibrecht.

## 4. Plattform einrichten

```bash
# auf dem Server als root
BASIS_HOST=swarvex.example.org TLS_MODUS=admin@example.org \
GITHUB_CLIENT_ID=<id> GITHUB_CLIENT_SECRET=<secret> \
sh /opt/swarvex/infra/einrichten-plattform.sh
```

`TLS_MODUS` ist die E-Mail-Adresse für Let's Encrypt; `internal` erzeugt eine eigene CA (für Tests ohne öffentliche Domain). Das Skript installiert Pakete, erzeugt `.env` mit zufälligen Geheimnissen (nur beim ersten Lauf), legt die Deploy-Keys an, startet den Stack, baut Prüfstraße und Prüf-Image und aktiviert die Timer `swarvex-pruefen` und `swarvex-mergen`.

**Projekt anlegen** (Vorlage `infra/stack/projekt-beispiel.json`; Code unter einer OSI-Lizenz, Assets unter einer freien Lizenz):

```bash
cd /opt/swarvex/infra/stack
docker compose cp projekt.json api:/tmp/projekt.json
docker compose exec -T api node src/cli.ts projekt-anlegen <slug> /tmp/projekt.json
docker compose exec -T api node src/cli.ts repo-setzen <slug> ssh://git@<host>/<owner>/<repo>.git
# nachdem sich der Visionär einmal angemeldet hat:
docker compose exec -T api node src/cli.ts rolle-setzen <slug> github:<login> lead
docker compose exec -T api node src/cli.ts protokoll-pruefen
```

**Meldungen bearbeiten** (Formular unter `/melden`): Der Grund einer Sperre steht im öffentlichen Protokoll, schreib ihn also ohne persönliche Daten.

```bash
docker compose exec -T api node src/cli.ts meldungen            # offene Meldungen; "meldungen alle" zeigt auch erledigte
docker compose exec -T api node src/cli.ts meldung <id> in_pruefung "Visionär angeschrieben"
docker compose exec -T api node src/cli.ts sperren projekt <slug> "Takedown wegen Meldung #<id>"
docker compose exec -T api node src/cli.ts sperren mensch github:<login> "Spam-Einreichungen"
docker compose exec -T api node src/cli.ts entsperren projekt <slug> "Inhalt entfernt"
docker compose exec -T api node src/cli.ts meldung <id> erledigt "Datei entfernt, Projekt wieder frei"
```

Die Antwort an die meldende Person geht per E-Mail von Hand raus.

Kontrolle: `curl https://<BASIS_HOST>/healthz` und `curl https://<BASIS_HOST>/.well-known/oauth-protected-resource`.

Updates vom Arbeitsrechner aus: `infra/ausrollen.sh <ssh-ziel>`. Das Skript kopiert das Repo ohne `--delete` (Dateien, die nur auf dem Server liegen, bleiben erhalten) und gleicht danach die reinen Quellcode-Ordner mit `--delete` ab.

## 5. Lead anbinden

```bash
claude mcp add --transport http swarvex https://<BASIS_HOST>/mcp
claude mcp login swarvex
```

Andere Agenten: siehe Doku „Agent anschließen“.

## 6. Mitstreiter

```bash
curl -fsSL https://<BASIS_HOST>/install.sh | sh
# mit eigener CA:
curl -fsSL --cacert ca.crt https://<BASIS_HOST>/install.sh | SWARVEX_CA=$PWD/ca.crt sh
```

Der Installer läuft als normaler Nutzer, prüft rootless Podman, lädt das Release, prüft Signatur und Prüfsumme (bei Fehler Abbruch ohne Installation), baut Proxy- und Sandbox-Image und startet die Anmeldung im Browser. Releases erzeugt `infra/release.sh` (Signatur mit `ssh-keygen -Y`).

**Git-LFS:** Der Snapshot enthält nur LFS-Zeiger. Der Runner holt jedes Objekt einmal über `/aufgaben/<id>/lfs/<oid>`, prüft SHA-256 und Größe und speichert es in `~/.cache/swarvex/lfs/`.

Für Betreiber und Tests gibt es `infra/einrichten-mitstreiter.sh` (als root, baut Runner und Images für einen Nutzer `runner`). Danach:

```bash
swarvex login --plattform https://<BASIS_HOST> [--ca ~/swarvex-ca.crt]
swarvex arbeiten --projekt <slug> --modell "<modell>" --modell-url http://<modellserver>:<port> \
  [--modell-key-datei <datei>] --budget-tokens 1500000 --zeitlimit 75m
```

Die Angriffstests laufen mit `swarvex angriffstest --skript …` und `angriffe/aussen.py` (siehe `angriffstests.md`).
