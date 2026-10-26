# Protokoll-Anker

Vorlage für ein **eigenes öffentliches Repo** (geplant: `swarvex-ai/protokoll-anker`), das den Kopf-Hash des swarvex-Ereignisprotokolls außerhalb der Plattform festhält.

Warum: Das Ereignisprotokoll ist hash-verkettet. Wer die Datenbank aber vollständig kontrolliert, könnte die ganze Kette neu berechnen, und sie wäre in sich wieder stimmig. Die Plattform signiert deshalb täglich ihren Kopf-Hash. Dieses Repo holt ihn ab und hält ihn in seiner Git-Historie fest, mit den Zeitstempeln von GitHub. Eine neu berechnete Kette passt nicht mehr zu den verankerten Köpfen, und die tägliche Action wird rot.

| Datei | Inhalt |
|---|---|
| `anker.mjs` | holt `/api/protokoll/kopf`, prüft die Signatur gegen `/jwks`, hängt an `koepfe.jsonl` an und vergleicht alle früheren Köpfe mit `/api/protokoll/hashes` |
| `.github/workflows/anker.yml` | täglich 03:45 UTC (die Plattform signiert um 03:17 UTC), Commit nur bei neuem Kopf |
| `koepfe.jsonl` | entsteht beim ersten Lauf: eine Zeile pro Kopf (`seq`, `hash`, `zeit`, `jws`) |

Selbst prüfen kann jeder, auch ohne dieses Repo: `BASIS=https://swarvex.ai node anker.mjs` in einem leeren Ordner legt eine eigene `koepfe.jsonl` an. Wer das regelmäßig laufen lässt, hat einen weiteren unabhängigen Anker.

Betreiber prüfen die Kette gegen den Anker mit:

```bash
docker compose exec -T api node src/cli.ts protokoll-pruefen https://raw.githubusercontent.com/swarvex-ai/protokoll-anker/main/koepfe.jsonl
```
