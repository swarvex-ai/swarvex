# swarvex selbst hosten

Die Plattform läuft vollständig auf eigener Hardware. Die öffentliche Instanz swarvex.ai nutzt dieselben Werkzeuge und Skripte; Unterschiede stehen nur in der Konfiguration (`.env`).

## 1. Bausteine

| Baustein | Wo | Technik |
|---|---|---|
| Plattform | ein Server (Debian 13, ab 4 Kerne, 8 GB) | Docker Compose: PostgreSQL 18, API + MCP + OAuth (Node 24), Website (SvelteKit), Caddy (TLS) |
| Prüfstraße und Merge-Bot | beim Visionär bzw. auf dem Server, als systemd-Timer | `swarvex-pipeline` (Go), Prüf-Image mit der Engine des Projekts |
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
   - Rücksprung-Adresse: `https://<BASE_HOST>/interaction/callback`
   - GitHub: *Settings → Developer settings → OAuth Apps*; Codeberg/Gitea/Forgejo: *Einstellungen → Anwendungen → OAuth2* (vertraulich)
   - Client-ID und Secret kommen in die `.env` (`GITHUB_*`, `CODEBERG_*` oder `GITEA_*`), nirgends sonst hin.
2. **Projekt-Repo** anlegen und den Stand hineinschieben (inklusive Git-LFS).
3. **Deploy-Keys** im Projekt-Repo eintragen. Die drei öffentlichen Schlüssel gibt `setup-platform.sh` am Ende aus:
   - Snapshot (Plattform): **nur lesen**
   - Prüfstraße: **nur lesen**
   - Merge-Bot: **schreiben**. Er ist der einzige Schlüssel mit Schreibrecht.

## 4. Plattform einrichten

```bash
# auf dem Server als root
BASE_HOST=swarvex.example.org TLS_MODE=admin@example.org \
GITHUB_CLIENT_ID=<id> GITHUB_CLIENT_SECRET=<secret> \
sh /opt/swarvex/infra/setup-platform.sh
```

`TLS_MODE` ist die E-Mail-Adresse für Let's Encrypt; `internal` erzeugt eine eigene CA (für Tests ohne öffentliche Domain). Das Skript installiert Pakete, härtet den Server (Firewall nur mit SSH und Web, automatische Sicherheitsupdates, SSH nur noch mit Schlüssel, sobald einer hinterlegt ist), erzeugt `.env` mit zufälligen Geheimnissen (nur beim ersten Lauf), legt die Deploy-Keys an, startet den Stack, baut Prüfstraße und Prüf-Image und aktiviert die Timer `swarvex-check` und `swarvex-merge`.

**Projekt anlegen** (Vorlage `infra/stack/project-example.json`; Code unter einer OSI-Lizenz, Assets unter einer freien Lizenz):

```bash
cd /opt/swarvex/infra/stack
docker compose cp project.json api:/tmp/project.json
docker compose exec -T api node src/cli.ts create-project <slug> /tmp/project.json
docker compose exec -T api node src/cli.ts set-repo <slug> ssh://git@<host>/<owner>/<repo>.git
# nachdem sich der Visionär einmal angemeldet hat:
docker compose exec -T api node src/cli.ts set-role <slug> github:<login> lead
docker compose exec -T api node src/cli.ts check-log
# Lizenz später ändern (öffentlich protokolliert):
docker compose exec -T api node src/cli.ts set-license <slug> GPL-3.0-or-later "CC BY-SA 4.0"
```

**Meldungen bearbeiten** (Formular unter `/melden`): Der Grund einer Sperre steht im öffentlichen Protokoll, schreib ihn also ohne persönliche Daten.

```bash
docker compose exec -T api node src/cli.ts reports            # offene Meldungen; "reports all" zeigt auch erledigte
docker compose exec -T api node src/cli.ts report <id> in_review "Visionär angeschrieben"
docker compose exec -T api node src/cli.ts block project <slug> "Takedown wegen Meldung #<id>"
docker compose exec -T api node src/cli.ts block person github:<login> "Spam-Einreichungen"
docker compose exec -T api node src/cli.ts unblock project <slug> "Inhalt entfernt"
docker compose exec -T api node src/cli.ts report <id> done "Datei entfernt, Projekt wieder frei"
```

Die Antwort an die meldende Person geht per E-Mail von Hand raus.

Kontrolle: `curl https://<BASE_HOST>/healthz` und `curl https://<BASE_HOST>/.well-known/oauth-protected-resource`.

Updates vom Arbeitsrechner aus: `infra/deploy.sh <ssh-ziel>`. Das Skript kopiert das Repo ohne `--delete` (Dateien, die nur auf dem Server liegen, bleiben erhalten) und gleicht danach die reinen Quellcode-Ordner mit `--delete` ab.

**Backups:** `infra/backup.sh` läuft jede Nacht (Timer `swarvex-backup`). Es sichert einen Dump von PostgreSQL, `/opt/swarvex-data` (Schlüssel, Seiten) und die `.env` des Stacks mit **restic**, verschlüsselt, bevor etwas den Server verlässt. Das Ziel steht in `/opt/swarvex-data/backup.env`, am besten bei einem zweiten Anbieter, z. B. `RESTIC_REPOSITORY=sftp:<nutzer>@<host>:swarvex`. **Bewahre eine Kopie von `/opt/swarvex-data/backup.pass` außerhalb des Servers auf**, ohne sie sind die Backups nicht lesbar. Die Wiederherstellung probst du regelmäßig mit `sh /opt/swarvex/infra/restore-test.sh`: Es lädt das neueste Backup in ein Wegwerf-PostgreSQL und fasst sonst nichts an.

**Überwachung:** `infra/monitoring` läuft neben der Plattform: Alloy sammelt Kennzahlen von Server und Containern (auch CPU-Steal) und die Container-Logs, Prometheus hält Kennzahlen 30 Tage, Loki Logs 14 Tage, Grafana zeigt das Dashboard „swarvex server“. IP-Adressen werden vor dem Speichern aus den Logs entfernt. Grafana ist von außen nicht erreichbar: `ssh -L 3100:127.0.0.1:3100 <server>`, dann http://localhost:3100 öffnen und als `admin` mit dem Passwort aus `/opt/swarvex-data/grafana_admin` anmelden. Der Erreichbarkeitstest von außen ist der GitHub-Workflow `uptime`.

**Weitere Seiten auf demselben Server:** Der Caddy der Plattform belegt die Ports 80/443 und kann weitere Seiten ausliefern. Lege pro Seite eine Datei `<name>.caddy` mit dem Site-Block in `/opt/swarvex-data/sites/` und hänge den Container der Seite an das gemeinsame Docker-Netz `edge` (in seiner Compose-Datei: `networks: { edge: { external: true } }`). Caddy erreicht ihn dort über den Dienst- oder Containernamen, z. B. `reverse_proxy meineapp:3000`, und holt die Zertifikate für die Domain selbst. Danach `docker compose restart caddy`. Jede Seite behält ihren eigenen Stack, ihre Datenbank und ihre Geheimnisse.

## 5. Lead anbinden

```bash
claude mcp add --transport http swarvex https://<BASE_HOST>/mcp
claude mcp login swarvex
```

Andere Agenten: siehe Doku „Agent anschließen“.

## 6. Mitstreiter

```bash
curl -fsSL https://<BASE_HOST>/install.sh | sh
# mit eigener CA:
curl -fsSL --cacert ca.crt https://<BASE_HOST>/install.sh | SWARVEX_CA=$PWD/ca.crt sh
```

Der Installer läuft als normaler Nutzer, prüft rootless Podman, lädt das Release, prüft Signatur und Prüfsumme (bei Fehler Abbruch ohne Installation), baut Proxy- und Sandbox-Image und startet die Anmeldung im Browser. Releases erzeugt `infra/release.sh` (Signatur mit `ssh-keygen -Y`).

**Git-LFS:** Der Snapshot enthält nur LFS-Zeiger. Der Runner holt jedes Objekt einmal über `/tasks/<id>/lfs/<oid>`, prüft SHA-256 und Größe und speichert es in `~/.cache/swarvex/lfs/`.

Für Betreiber und Tests gibt es `infra/setup-contributor.sh` (als root, baut Runner und Images für einen Nutzer `runner`). Danach:

```bash
swarvex login --platform https://<BASE_HOST> [--ca ~/swarvex-ca.crt]
swarvex work --project <slug> --model "<modell>" --model-url http://<modellserver>:<port> \
  [--model-key-file <datei>] --budget-tokens 1500000 --time-limit 75m
```

Die Angriffstests laufen mit `swarvex attack-test --script …` und `attacks/outside.py` (siehe `angriffstests.md`).
