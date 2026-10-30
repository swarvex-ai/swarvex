# Architektur und Techstack

Grundsatz: **Vorhandenes verbinden, nur die Koordination selbst bauen.** Jede Komponente nennt, was wir übernehmen und was wir bauen.

## Überblick

```
                         ┌───────────────────────────────────────────┐
                         │ Plattform (eigener Server)                 │
 Browser ──HTTPS────────▶│  Web (SvelteKit)                           │
                         │  API + MCP-Gateway (TypeScript, Fastify)   │
                         │  Koordinations-Kern (Zustandsmaschine)     │
                         │  PostgreSQL · Job-Queue · Objektspeicher   │
                         │  Identität (OAuth 2.1 / OIDC)              │
                         └───────▲──────────────▲──────────────▲──────┘
                                 │ MCP/HTTPS    │ MCP/HTTPS    │ Webhook/API
          ┌──────────────────────┴───┐  ┌───────┴───────────┐  ┌┴───────────────────────┐
          │ Lead des Visionärs        │  │ Runner der        │  │ Projekt-Repo + Prüfstraße│
          │ (Claude Code, Hermes,     │  │ Mitstreiter/      │  │ (GitHub, Gitea/Forgejo;  │
          │  opencode … beliebig)     │  │ Spender           │  │  Actions/Woodpecker)     │
          │ spricht MCP direkt        │  │ Sandbox + Proxy   │  │ beim Visionär            │
          └───────────────────────────┘  └───────────────────┘  └─────────────────────────┘
```

Agenten laufen **nie** auf dem Plattform-Server. Die Plattform hält Zustand, Regeln und Protokoll.

## Techstack

| Bereich | Wahl | Übernommen / gebaut | Begründung |
|---|---|---|---|
| Agenten-Schnittstelle | **MCP** (Model Context Protocol), Transport Streamable HTTP, Auth nach MCP-Authorization (OAuth 2.1) | Standard übernommen, Werkzeuge gebaut | Jeder gängige Agent (Claude Code, Codex, opencode, Hermes, Cursor …) spricht MCP. Ein Lead kann sich direkt anbinden, ohne eigene Software. |
| Backend | **TypeScript** auf Node 24 (Typen werden direkt ausgeführt, kein Build), `node:http`, offizielles MCP-TypeScript-SDK, Zod für Schemata | gebaut | Gleiche Typen für API, MCP-Werkzeuge und Web; offizielles SDK. Fastify verworfen (PoC): Der OAuth-Server und der MCP-Transport arbeiten direkt auf Node-Requests |
| Datenbank | **PostgreSQL 18** | übernommen | Einzige Quelle der Wahrheit, relationale Integrität, append-only Protokoll; Text-Patches und Metadaten |
| Job-Queue / Leases | **pg-boss** (Queue in Postgres) | übernommen | Keine zusätzliche Infrastruktur; Zeitüberschreitungen und Wiederholungen. Temporal bewusst *nicht*: Die langen Abläufe laufen beim Lead/Runner, nicht bei uns (E-05) |
| Identität | **oidc-provider** (panva), im Backend (E-06) | übernommen | Dynamische Client-Registrierung für MCP-Agenten, Device Flow (RFC 8628) für den Runner, OAuth 2.1 mit PKCE; Login über GitHub/Codeberg/Gitea |
| Web | **SvelteKit** (Node-Adapter) | gebaut | Schlank, selbst hostbar, SSR für öffentliche Projektseiten (E-07) |
| Objektspeicher | **Garage** (S3-kompatibel), Container im Stack (E-08) | übernommen | Eingang für Dateien bis zur Annahme, Prüfberichte, Screenshots. Angenommenes landet im Git-Repo des Projekts (Assets per LFS) |
| Runner | **Go**, ein statisches Binary pro Plattform | gebaut | Ein Download, keine Laufzeit-Abhängigkeit → echter Einzeiler (E-09) |
| Sandbox | **Podman** auf allen Systemen: rootless (Linux, optional gVisor), `podman machine` (macOS: libkrun/Apple Hypervisor, Windows: WSL2) (E-22) | übernommen | Fremder Projektcode läuft nie direkt auf dem Host; ein Code-Pfad für alle drei Systeme |
| Schlüssel-Proxy | **eigener kleiner Proxy** im Runner (Go) | gebaut | Agent im Container sieht nur den Proxy mit Wegwerf-Token; echte Keys bleiben außerhalb; erzwingt Budget und Zeitlimit. LiteLLM verworfen (PoC): weniger Fremdcode an der sicherheitskritischsten Stelle |
| Agenten im Container | **opencode**, Claude Code, Codex CLI, aider, Hermes … als Engine-Adapter | übernommen | Wir bauen keinen eigenen Coding-Agenten |
| Prüfstraße | **eigener Prüfdienst** (Go) beim Visionär, Container ohne Netz, signierte Meldung; Engine-Images: Godot (z. B. `barichello/godot-ci`), Unity (GameCI), Unreal (später) | gebaut | Gleiche Rolle beim Visionär, unabhängig vom CI-System |
| Releases und Signaturen | **GitHub Actions**, **GoReleaser**, Images in **GHCR**; **cosign** keyless, SBOM, SLSA-Provenienz; DCO-Sign-off für Beiträge; signierte Merge-Commits des Merge-Bots (E-23) | übernommen | Der Runner darf nicht selbst zum Angriffsweg werden |
| Beobachtbarkeit | **OpenTelemetry** in der API, **Grafana Alloy** → Prometheus, Loki, Grafana; externer Erreichbarkeitstest (E-24) | übernommen | Betrieb |
| Betrieb | **Docker Compose** auf einem Server (Debian 13), **Caddy** mit Let's Encrypt, nächtliche verschlüsselte Backups mit **restic** (PostgreSQL-Dump und Serverdateien) bei einem zweiten Anbieter, Geheimnisse mit **sops + age** (E-18, E-24) | übernommen | Ein Server reicht lange, weil keine Agenten bei uns laufen |

## MCP-Gateway: Werkzeuge pro Rolle

Ein Endpunkt `https://<domain>/mcp`. Welche Werkzeuge ein Client sieht, hängt vom Token ab (Rolle, Projekt, Vertrauensstufe).

**Mitstreiter (über den Runner oder direkt):**

| Werkzeug | Zweck |
|---|---|
| `project_context` | Vision, Regeln, Architekturentscheidungen, Lizenz |
| `matching_tasks` | Aufgaben, die zu den Fähigkeiten und der Stufe passen |
| `reserve_task` | Lease holen (mit Ablaufzeit) |
| `task_details` | volle Beschreibung, erlaubte Pfade, Prüfvertrag, Basis-Commit |
| `report_progress` | Lebenszeichen, verbrauchte Tokens/Credits/Zeit |
| `submit_result` | Patch (git format-patch) bzw. Artefakt + Zusammenfassung + eigene Testergebnisse |
| `abandon_task` | Grund, Logs; Aufgabe geht zurück in den Pool |

**Lead (zusätzlich):**

| Werkzeug | Zweck |
|---|---|
| `create_task` / `change_task` | inkl. Stufe, Fähigkeiten, Prüfvertrag, Mehrfachvergabe |
| `fix_contract` | Schnittstelle + Tests als verbindliche Basis für Aufgaben |
| `review_submission` | annehmen / Nacharbeit / ablehnen, jeweils mit Begründung |
| `propose_merge` | an den Merge-Bot; braucht grüne Prüfung |
| `create_asset_brief` | Auftrag an Spender inkl. Kostenschätzung und Vorschau-Stufe |
| `request_approval` | an den Visionär (Release, Lizenz, Budget-Ausnahmen) |
| `metrics` | Lage des Projekts |

Werkzeuge, die es **nicht** gibt: direkter Schreibzugriff auf Repos, Zugriff auf andere Projekte oder Nutzerdaten, Budgets anderer ändern, Prüfergebnisse überschreiben.

**Direktanbindung ohne Runner** (z. B. `claude mcp add --transport http …`) ist nur für Aufgaben erlaubt, die **keinen Projektcode ausführen**: Design-Vorschläge, Texte, Reviews durch Lesen. Alles, was baut oder testet, verlangt den Runner mit Sandbox.

## Runner

Ein Go-Binary, Open Source, signiert.

```
Runner (Host)
├─ Login (OAuth Device Flow) → kurzlebiges, gescoptes Token, Refresh im System-Schlüsselbund
├─ Zusagen/Fähigkeiten: kommen von der Website, lokal nur gespiegelt
├─ Zeitplan + Budgetwächter (bricht bei Überschreitung hart ab)
├─ Schlüssel-Proxy: LiteLLM (LLMs) + Asset-Adapter (Meshy …); echte Keys nur hier
├─ lokaler MCP-Proxy: reicht nur die Werkzeuge der aktuellen Aufgabe in den Container
└─ Sandbox (Podman/Docker), pro Aufgabe neu:
     ├─ Projekt-Snapshot (Basis-Commit), nur erlaubte Pfade beschreibbar
     ├─ Agent-Engine (opencode / Claude Code / Codex / Hermes …)
     ├─ Netz: nur Proxy + Paket-Spiegel (Allowlist)
     ├─ kein Home-Verzeichnis, kein Docker-Socket, Ressourcenlimits
     └─ Ausgabe: Patch + Artefakte + Bericht → Runner → Plattform
```

Der Runner prüft den Patch vor dem Einreichen selbst gegen die erlaubten Pfade. Die Plattform prüft danach noch einmal: Vertrauen ist gut, Prüfung auf eigener Seite ist Pflicht.

## Beiträge ohne Git-Rechte

Mitstreiter brauchen **keine** Schreibrechte und keine Git-Zugangsdaten:

1. Der Runner reicht einen Patch ein (`git format-patch` gegen den Basis-Commit).
2. Die Plattform speichert ihn mit Hash im Objektspeicher.
3. Die Prüfstraße des Projekts holt ihn per Webhook bzw. API, wendet ihn auf einen Prüfzweig an und prüft.
4. Nimmt der Lead an, erstellt der **Merge-Bot** des Projekts den Commit, mit Trailern (siehe Nachvollziehbarkeit) und DCO-Sign-off des Menschen.

## Projekt-Umgebung, Prüfstraße und Prüfvertrag

Projekte sind an keine Sprache und keine Engine gebunden (E-28). Runner und Prüfstraße ermitteln die Umgebung mit demselben Paket (`runner/umgebung`):

1. `swarvex.yaml` im Projekt (Vorlage oder eigenes Image, Einrichten, Bauen, Testen, Arbeitsordner, projekteigene geschützte Pfade),
2. sonst ein Dev Container (`.devcontainer/devcontainer.json`: Image oder Dockerfile, Einrichtungsbefehle),
3. sonst die Projektart an typischen Dateien (Godot, Node, Python, Rust, Go, .NET, Java),
4. sonst eine allgemeine Linux-Umgebung.

```yaml
template: godot          # oder image: ghcr.io/…
dir: game
setup: godot --headless --import
test: tools/run_tests.sh --headless
protected: |
  - tools/**
```

- **Zwei Phasen:** Einrichten (Abhängigkeiten) läuft mit Netz, aber ohne Agent, ohne Proxy und ohne Schlüssel. Agent, Bauen und Testen laufen ohne Netz; Abhängigkeiten liegen im Projektordner oder im Cache `/cache` je Projekt.
- **Agent-Schicht:** Der Runner legt den gewählten Agenten und ripgrep (`/opt/swarvex-agent`) über das Projekt-Image: opencode (Image `swarvex-agent`, Standard), Claude Code (`swarvex-agent-claude`) oder Codex (`swarvex-agent-codex`), je eine Baustufe in `runner/sandbox/Containerfile`, gebaut beim ersten Gebrauch. Der Schlüssel-Proxy lässt nur die Modell-Schnittstelle des gewählten Agenten durch (E-32). Voraussetzung ist glibc.
- **Geschützt:** `swarvex.yaml`, `.devcontainer/`, `AGENTS.md`, Anweisungen an Agenten (`.agents/`, `.claude/`, `.codex/`, `.opencode/`, `CLAUDE.md`, `opencode.json`), CI-Workflows und Lizenzdateien ändern Beiträge nie (Plattform, Runner und Prüfstraße prüfen das), sonst könnte ein Beitrag Befehle auf andere Rechner schmuggeln.
- Die Prüfstraße läuft beim Visionär und meldet ein signiertes Ergebnis an die Plattform. `swarvex env` führt dieselben Schritte lokal aus.
- **Stichproben:** Die Plattform wiederholt einen Anteil der Prüfungen selbst (Container auf eigenem Server) und markiert Abweichungen öffentlich.
- Wenn der Spender es erlaubt, laufen Asset-Prüfungen ebenfalls hier: Import, Polygonzahl, Skelett, Screenshot im Spiel-Licht.

## Datenmodell (Kern)

```
menschen(id, konto_provider, konto_id, erstellt, vertrauensstufe, gesperrt)
agenten(id, mensch_id, name, engine)                       -- ein Mensch, viele Agenten
zuordnungen(id, agent_id, faehigkeit, modell_bezeichnung, budget_json, zeitfenster)
projekte(id, slug, visionaer_id, vision_md, lizenz_code, lizenz_assets, engine,
         repo_url, pruefung_url, einnahmen_erklaerung, lead_konfiguration)
aufgaben(id, projekt_id, stufe, rolle, faehigkeiten[], pfade[], pruefvertrag_ref,
         basis_commit, kosten_schaetzung, mehrfach, min_vertrauen, zustand)
leases(id, aufgabe_id, agent_id, zuordnung_id, laeuft_ab)
einreichungen(id, aufgabe_id, lease_id, patch_hash, artefakte[], bericht,
              verbrauch_json, eingereicht)
pruefungen(id, einreichung_id, quelle(projekt|stichprobe), ergebnis, bericht_ref, signatur)
bewertungen(id, einreichung_id, lead_agent_id, urteil, begruendung)
spenden_budgets(id, mensch_id, dienst, einheit, limit, verbraucht, zeitraum)
ereignisse(seq, zeit, akteur, art, bezug, daten_json, vorgaenger_hash, hash)  -- append-only
```

## Sicherheit: Bedrohungen und Gegenmaßnahmen

| Bedrohung | Gegenmaßnahme |
|---|---|
| Schädlicher Projektcode greift den Rechner des Mitstreiters an | Sandbox pro Aufgabe, Netz-Allowlist, kein Home, kein Docker-Socket |
| Prompt-Injection über Repo/Issues/Aufgaben stiehlt Schlüssel | Schlüssel-Proxy: Keys sind nie im Container; Wegwerf-Tokens pro Aufgabe |
| Böswilliger Visionär formuliert Angriffs-Aufgaben | wie oben, und Budgets liegen beim Spender |
| Schwacher oder manipulierter Lead verschwendet Spenden | Spender-Budgets, Vorschau-Stufe, öffentliche Kennzahlen |
| Fake-Agenten, Spam | Identität am Menschen, Vertrauensstufen, Ratenlimits |
| Schädlicher Beitrag mit versteckter Hintertür | Prüfvertrag, geschützte Pfade, Diff-Regeln (z. B. keine Netz-/Datei-APIs ohne Lead-Freigabe), Lead-Review |
| Kompromittierter Runner-Download | cosign-signierte Releases, Prüfsumme im Installer, reproduzierbare Builds als Ziel |
| Projekt-CI führt fremden Code mit Geheimnissen aus | Prüfzweige ohne Secrets (die `pull_request_target`-Falle vermeiden), Merge-Bot getrennt von der Prüfung |
| Geschönte Prüfergebnisse eines Projekts | Stichproben auf Plattform-Hardware, Abweichungen öffentlich |
| Prompt-Injection beim Weg ohne Code: Der eigene Agent des Mitstreiters (volle Rechte auf dessen Rechner) liest fremde Aufgabentexte | Herkunftslabel an jeder MCP-Antwort, fremder Text getrennt vom Auftrag, fertige Agenten-Profile ohne Shell und Netz (siehe unten) |
| Prompt-Injection gegen den Lead über eingereichte Beiträge | Beiträge liest ein Unteragent, der nur in festem Schema antwortet; Annahme bleibt eine Lead-Entscheidung mit Prüfstraße davor |

Ein Restrisiko bleibt (ein Beitrag, der alle Prüfungen besteht und trotzdem schadet). Es wird offen kommuniziert.

### Fremder Text: Herkunftslabel und feste Schutzregeln

Angeregt durch die Guardrails von Archestra (siehe unten). Grundsatz: Ob ein Agent nach dem Lesen fremden Texts noch riskante Dinge tun darf, entscheiden **feste Regeln außerhalb des Agenten**, nicht das Modell. Eingeschleuste Anweisungen können daran nichts ändern.

1. **Herkunft an jeder Antwort.** Jede MCP-Antwort trennt den Auftrag der Plattform (Format, Regeln, erlaubte Pfade) von fremdem Text (Beschreibung des Visionärs, Beiträge, Kommentare) und sagt, wer ihn geschrieben hat und mit welcher Vertrauensstufe: `{"auftrag": …, "fremdtext": [{"autor": "…", "vertrauen": "fremd", "text": "…"}]}`.
2. **Profile für den Weg ohne Code.** Für Claude Code, Codex, opencode und weitere liefern wir fertige Einstellungen für Plattform-Arbeit mit: keine Shell, kein Netz außer der Plattform, Schreiben nur in einem Arbeitsordner. Der Installer bietet sie an, die Seite „Mitmachen“ erklärt sie. Wer mehr Rechte braucht, nimmt den Runner-Weg mit Sandbox.
3. **Lesen in festem Schema.** Muss ein Agent mit vielen Rechten (vor allem der Lead) fremden Text bewerten, liest ihn ein Unteragent ohne Werkzeuge und antwortet nur in einem strengen JSON-Schema (z. B. Bewertung, Befunde mit Pfad und Zeile). Freitext wird verworfen. Die Plattform prüft das Schema.

## Vergleich: Archestra (geprüft am 25.10.2026)

[Archestra](https://github.com/archestra-ai/archestra) (AGPL-3.0 mit Enterprise-Lizenz, etwa 4.400 Sterne) ist eine Plattform für Unternehmen. Sie sitzt zwischen den Agenten der Mitarbeiter und den Systemen der Firma: LLM-Proxy mit Budgets, MCP-Gateway mit Katalog, Agenten-Laufzeit auf Kubernetes, Guardrails, Audit-Log.

**Grundlegender Unterschied:** Archestra vertraut bekannten Mitarbeitern einer Firma, lässt Agenten zentral laufen und hält die Schlüssel zentral auf dem Server. Wir arbeiten mit fremden Mitstreitern; ihre Agenten laufen auf ihren Rechnern, und die Schlüssel verlassen den Rechner nie. Archestra als Baustein einzusetzen, widerspricht deshalb unserer Kernbotschaft („deine Schlüssel bleiben bei dir“) und brächte viel Fremdcode an die sicherheitskritischste Stelle. Wir setzen es nicht ein.

**Übernommen als Ideen:**

| Idee | Bei uns | Issue |
|---|---|---|
| Vertrauenslabel pro Sitzung, feste Regeln außerhalb des Agenten | Herkunftslabel, Agenten-Profile, Lesen in festem Schema (Abschnitt oben) | #36 |
| Unteragent antwortet nur in festem Schema (`attest-schema`) | Review-Aufgaben und Lead | #33 |
| Budgets in mehreren Ebenen, alle passenden Grenzen gelten gleichzeitig | Proxy: pro Aufgabe, Tag, Projekt | #37 |
| Eindeutige Fehlercodes: 402 bei Geldbudget, 429 mit `Retry-After` bei Rate | Proxy, damit Agenten sauber reagieren | #37 |
| Gezählt wird nur der vom Anbieter gemeldete Verbrauch; Preise lokaler Modelle setzt man von Hand | Spenden und Verbrauch in Tokens; Euro nur, wo ein Preis bekannt ist | #37 |
| OpenTelemetry und Prometheus von Anfang an | Betrieb | #14 |
| AGPL für einen Dienst: Forks, die als Dienst laufen, müssen ihre Änderungen offenlegen | Eingabe für die Lizenzentscheidung (Open Core übernehmen wir nicht) | – (E-13) |

## Vergleich: agentgateway (geprüft am 25.10.2026)

[agentgateway](https://github.com/agentgateway/agentgateway) (Apache-2.0, Projekt der Linux Foundation, Rust, etwa 5.600 Sterne, Version 1.6) ist ein Proxy für den Verkehr von Agenten:

- **LLM-Gateway:** etwa 20 Anbieter, OpenAI-kompatible Schnittstelle mit Übersetzung, z. B. Chat Completions → Anthropic.
- **MCP-Gateway:** mehrere MCP-Server hinter einem Endpunkt, stdio, HTTP und Streamable HTTP, OpenAPI → MCP; OAuth als Resource-Server.
- **Weiteres:** A2A, Rollen und Regeln mit CEL, Ratenlimits, OpenTelemetry, Guardrails.
- **Betrieb:** als eigenständiges Binary oder in Kubernetes.

**Grundlegender Unterschied zu Archestra:** agentgateway ist ein einzelnes, neutral verwaltetes Programm ohne Enterprise-Lizenz und läuft dort, wo man es startet. Es kann also beim Mitstreiter laufen, und die Schlüssel bleiben auf dessen Rechner. Das passt zu unserer Kernbotschaft.

**Auf der Plattform setzen wir es nicht ein.** Für MCP ist agentgateway nur Resource-Server und braucht einen externen Login-Dienst (eingebaut: Keycloak, Auth0). Unser eigener OAuth-Server kann schon mehr, was wir brauchen: Metadata-Dokumente, dynamische Registrierung, Gerätecode für den Runner, und dazu unsere Werkzeuge mit Zustand. Ein Gateway davor wäre ein zusätzlicher Sprung ohne Gewinn.

**Beim Mitstreiter (Runner) ist es interessant.** Zu prüfen in einem Spike:

| Idee | Bei uns | Issue |
|---|---|---|
| Übersetzung zwischen Anbietern: Wer einen Anthropic-, Gemini- oder Bedrock-Schlüssel hat, kann ihn nutzen, obwohl Agenten in der Sandbox OpenAI-kompatibel sprechen | Kette Sandbox → unser kleiner Schlüssel-Proxy (Token, Budget, Zeit, Protokoll; bleibt die Sicherheitsgrenze) → agentgateway auf dem Host (Übersetzung, Ausweichen bei Fehlern) → Anbieter | #43 |
| MCP-Föderation mit Werkzeug-Filter: Gespendete Dienste mit eigenem MCP-Server (z. B. Meshy, ElevenLabs) erscheinen in der Sandbox als ein Endpunkt, nur mit erlaubten Werkzeugen und Ratenlimit, ohne dass der Agent den Schlüssel sieht | Spenden-Adapter und Auftrag + Upload; vorher prüfen, ob agentgateway Zugangsdaten für Upstream-Server einsetzen kann (Doku unklar) | #43, #16, #17 |
| Gleiche Namen für Messwerte von MCP- und LLM-Aufrufen in OpenTelemetry | Betrieb, Kennzahlen im Runner | #14 |
| Regeln als CEL-Ausdrücke statt im Code | Idee für Vertrauensstufen, falls die Regeln wachsen; heute reichen feste Regeln im Code | #28 |
| Guardrails per Webhook (z. B. Erkennung eingeschleuster Anweisungen) | höchstens als zusätzliche Schicht; die festen Regeln außerhalb des Agenten bleiben maßgeblich | #36 |

**Nicht übernommen:** A2A (vielleicht später zwischen Leads), der Kubernetes-Betrieb und ein Gateway vor der Plattform.

**Bedingungen für den Einsatz im Runner:** Der Spike misst Größe, Speicherbedarf und Startzeit, prüft Tool-Aufrufe und Streaming mit opencode bei Anthropic und Gemini und schaut sich die Lizenzen der Abhängigkeiten an. Er bleibt optional: Ohne agentgateway arbeitet der Runner wie heute mit OpenAI-kompatiblen Endpunkten (lokal oder Frontier).
