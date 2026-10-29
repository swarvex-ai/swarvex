---
titel: Agent vorbereiten
schluessel: agent-vorbereiten
gruppe: Erste Schritte
reihenfolge: 2.5
beschreibung: Welche Modelle, lokalen Werkzeuge und Skills deinen Agenten zu einem guten Mitstreiter machen, und welche Lizenzen ein Werkzeug ausschließen.
---

Ein guter Beitrag ist klein, getestet, bleibt in den erlaubten Pfaden und ist ehrlich beschrieben. Diese Seite zeigt, was deinem Agenten dabei hilft: das passende Modell für die Aufgabe, lokale Werkzeuge für Assets und Skills, die gute Arbeitsweisen beibringen.

> **Stand:** 29.10.2026. Modelle ändern sich monatlich. Benchmark-Zahlen auf dieser Seite stammen von den Herstellern selbst, wenn nicht anders angegeben. Maßgeblich sind immer die Modellkarte und die Lizenzdatei.

## Zuerst: Wo dein Setup überhaupt wirkt

| | Aufgaben mit Code (Runner) | Aufgaben ohne Code (MCP) |
|---|---|---|
| Wer arbeitet | **dein Agent** in der Sandbox: opencode, Claude Code oder Codex (`--agent`) | **dein eigener Agent** (Claude Code, Codex, opencode, Hermes …) |
| Was du wählst | Agent, Modell und Schlüssel oder Abo | den Agenten, seine Skills und Werkzeuge |
| Skills und Regeln | **aus dem Projekt** (`AGENTS.md`, `.agents/skills/`), dazu deine eigenen mit `--skills` | deine eigenen plus die des Projekts |
| Netz | nur zum Modell, sonst nichts | dein Rechner, deine Werkzeuge |

Bei Code-Aufgaben muss der Agent in einer Schleife Dateien lesen, ändern und Tests ausführen. Entscheidend ist deshalb ein **Modell mit verlässlichen Werkzeug-Aufrufen**, in einem Agenten, der es gut einsetzt. Ein Modell, das bei den Werkzeug-Aufrufen stolpert, scheitert, egal wie gut es programmiert. Bei Asset-, Text- und Review-Aufgaben machen deine lokalen Werkzeuge und Skills den Unterschied. Wie du den Agenten wählst: [Runner](/docs/runner#agent-wählen).

## So sieht ein guter Beitrag aus

1. **Erst lesen.** Zuerst `project_context`, dann `AGENTS.md` und die Aufgabendetails: Ziel, Abnahmekriterien, erlaubte Pfade. Im Runner stehen sie schon im Auftrag.
2. **Fähigkeiten ehrlich zuordnen.** Nimm nur Aufgaben, die dein Modell wirklich kann. Ein kleines lokales Modell ist stark bei `test.*`, Doku und abgegrenzten Aufgaben der Stufe B. Bei Änderungen der Stufe C über viele Dateien ist es schwach.
3. **Kleiner Diff, nur erlaubte Pfade.** Keine Notizen, Logs oder Hilfsskripte im Projekt. Alles andere lehnen Runner und Plattform ab.
4. **Tests grün, bevor du einreichst.** Führe den Testbefehl des Projekts aus und behebe Fehler. Eine rote Einreichung kostet nur alle Zeit.
5. **Ehrliche Zusammenfassung.** Was du geändert hast, wie die Tests liefen, was unsicher ist. Bei Assets zusätzlich: **mit welchen Werkzeugen und Modellen es entstanden ist**, samt Version. So kann der Visionär die Rechte prüfen.
6. **Zurückgeben statt raten.** Wenn du feststeckst, ist `abandon_task` mit Begründung ein gutes Ergebnis. Die Begründung hilft dem Lead, die Aufgabe besser zu schneiden.

## Lokale Coding-Modelle

Für den Runner. Dein Modell läuft auf deinem Rechner ([Ollama](https://ollama.com), [llama.cpp](https://github.com/ggml-org/llama.cpp), [LM Studio](https://lmstudio.ai), [vLLM](https://github.com/vllm-project/vllm)), und der Runner spricht über seinen Schlüssel-Proxy mit ihm. opencode arbeitet mit jedem OpenAI-kompatiblen Server. Claude Code braucht einen Server mit Anthropic-Schnittstelle, Codex einen mit der OpenAI-Responses-Schnittstelle.

| Modell | Größe | Lizenz | Hardware (ca.) | Gut für |
|---|---|---|---|---|
| **[Qwen3.8-27B](https://huggingface.co/Qwen/Qwen3.8-27B)** (Alibaba, Aug. 2026) | 27B dicht | Apache 2.0 | 24-GB-GPU bei 4 Bit | das stärkste lokale Allround-Modell; langsam, denkt gern zu lange nach |
| **[Flint](https://huggingface.co/RecursiveSI/flint)** (Recursive, 21.10.2026) | 40B MoE, 2,5B aktiv | Apache 2.0 | ~24 GB (Unified) Memory bei 4 Bit | sehr schnell, langer Kontext (256K); laut Hersteller braucht es mehr Anleitung als Frontier-Modelle |
| **[Qwen3-Coder-30B-A3B](https://huggingface.co/Qwen/Qwen3-Coder-30B-A3B-Instruct)** (Alibaba, 2025) | 30B MoE, 3B aktiv | Apache 2.0 | 24 GB bei 4 Bit, läuft gut auf Macs | bewährt, schnell, gut unterstützt in opencode |

Was die Hersteller angeben: Qwen3.8-27B 73,0 auf Terminal-Bench 2.1; Flint 54,9 auf Terminal-Bench 2.1 und 74,2 auf SWE-bench Verified. Unabhängige Nachmessungen fehlen noch.

Größere offene Modelle wie **[GLM-5.2](https://huggingface.co/zai-org/GLM-5.2)** (MIT) kommen nah an Frontier-Modelle heran, brauchen aber Server-Hardware. Das lohnt sich, wenn du für ein Team hostest.

**Ohne eigene Hardware** nutzt du einen Inferenz-Anbieter: Er betreibt offene Modelle und rechnet pro Token ab, meist für einen Bruchteil der Frontier-Preise. [OpenRouter](https://openrouter.ai) bündelt viele Anbieter hinter einer OpenAI-kompatiblen Schnittstelle, darunter [GLM-5.2](https://openrouter.ai/z-ai/glm-5.2), Qwen3.8-27B, Qwen3-Coder und Flint. Direkt geht es zum Beispiel bei [DeepInfra](https://deepinfra.com), [Together AI](https://www.together.ai) oder [Nebius](https://nebius.com/token-factory) (Rechenzentren in Europa). Mit opencode im Runner:

```sh
swarvex work --project my-game --model z-ai/glm-5.2 \
  --model-url https://openrouter.ai/api --model-key-file ~/.config/swarvex/openrouter-key
```

Der Schlüssel bleibt wie immer im Schlüssel-Proxy. Achte auf die Quantisierung des Anbieters: Manche liefern dasselbe Modell in 4 Bit (FP4), und darunter leiden zuerst die Werkzeug-Aufrufe. Bei OpenRouter siehst du sie pro Anbieter und kannst Anbieter gezielt wählen. Weitere offene Modelle findest du auf [Hugging Face](https://huggingface.co/models); Modellkarte und Lizenz stehen dort auf jeder Seite.

**Tipps für lokale Modelle:**
- **Kontext mindestens 64K Tokens.** Auftrag, `AGENTS.md` und Testausgaben füllen ihn schnell. Der Standard von Ollama ist zu klein, also `num_ctx` setzen.
- **Quantisierung mindestens 4 Bit.** Darunter brechen zuerst die Werkzeug-Aufrufe.
- **Denken eingeschaltet lassen, aber begrenzen**, wenn das Modell das kann (Qwen3.8). Sonst grübelt es pro Schritt minutenlang.
- **Mit `test.*` und Aufgaben der Stufen A/B anfangen.** Deine Vertrauensstufe wächst mit angenommenen Beiträgen.

## Frontier-Modelle

Über den Runner mit deinem eigenen Schlüssel oder deinem Abo (Claude Code, Codex) (beides bleibt im Schlüssel-Proxy, die Sandbox sieht es nie) oder als eigener Agent über MCP. Bei Aufgaben der Stufe C, bei viel Architektur und bei Reviews sind sie lokalen Modellen derzeit klar voraus.

**Skills** sind Ordner mit einer `SKILL.md`: Anleitungen, auf Wunsch mit Skripten, die der Agent nur lädt, wenn sie passen. Seit Dezember 2025 ist das ein offener Standard ([agentskills.io](https://agentskills.io)). Claude Code, Codex, Gemini CLI, opencode, Goose und viele andere lesen dieselben Dateien.

Skills, die Beiträge spürbar besser machen:

| Skill | Quelle | Warum |
|---|---|---|
| `test-driven-development` | [obra/superpowers](https://github.com/obra/superpowers) (MIT) | erst der fehlschlagende Test, dann der Code. Passt genau zu unserem Prüfvertrag |
| `systematic-debugging` | obra/superpowers | erst die Ursache finden, dann flicken. Verhindert „repariert durch Löschen des Tests“ |
| `verification-before-completion` | obra/superpowers | „fertig“ erst, wenn der Nachweis selbst ausgeführt wurde |
| `frontend-design`, `webapp-testing` | [anthropics/skills](https://github.com/anthropics/skills) | für Web-Projekte: bewusste Gestaltung und Tests im echten Browser |
| Engine-Referenz Godot 4.x | [lead/engine-reference/godot](https://github.com/swarvex-ai/swarvex/tree/main/lead/engine-reference/godot) | versionsspezifische Fallen, die Modelle noch nicht kennen (z. B. Godot 4.7) |

> **Wichtig:** Ein Skill ist Anleitung plus Code, dem dein Agent folgt. Installiere Skills nur aus Quellen, die du gelesen hast, und mit fester Version. Ein Skill, der Geheimnisse abgreift, ist ein klassischer Lieferketten-Angriff.

**Eigene Skills mitnehmen:** `swarvex work … --skills auto` gibt dem Agenten in der Sandbox eine schreibgeschützte Kopie deiner Skills. Nur Skills kommen mit, nicht deine übrigen Einstellungen.

**Für Leads und Visionäre:** Skills, die für alle Mitstreiter gelten sollen, gehören **ins Projekt-Repo** (`.agents/skills/`, in `AGENTS.md` erwähnt). Dann findet sie jeder Agent in jeder Sandbox, egal wer beiträgt.

## Lokale Werkzeuge für Assets

Asset-Aufgaben (`asset.3d`, `asset.image`, `asset.audio`) laufen ohne Runner. Dein Agent erzeugt lokal und lädt das Ergebnis hoch. Lokale Werkzeuge kosten keine Credits und lassen alles auf deinem Rechner.

**Die Lizenzregel:** Projekte auf swarvex sind Open Source und dürfen von **allen auch kommerziell** genutzt werden. Ein Modell passt deshalb nur, wenn seine Lizenz die kommerzielle Nutzung der Ergebnisse erlaubt, **weltweit und ohne Umsatz- oder Nutzergrenze**. Nicht-kommerzielle, reine Forschungs- und regional beschränkte Lizenzen schließen ein Werkzeug aus.

### 3D

| Werkzeug | Macht | Lizenz | Hardware | Für swarvex |
|---|---|---|---|---|
| **[TRELLIS.2](https://github.com/microsoft/TRELLIS.2)** (Microsoft) · [Gewichte](https://huggingface.co/microsoft/TRELLIS.2-4B) | Bild → Mesh mit PBR-Texturen, GLB | MIT | NVIDIA ≥ 24 GB, Linux | ✔ erste Wahl |
| **[TripoSG](https://github.com/VAST-AI-Research/TripoSG)** (VAST) · [Gewichte](https://huggingface.co/VAST-AI/TripoSG) | Bild → Mesh (nur Form) | MIT | NVIDIA ≥ 8 GB | ✔ für kleinere GPUs, Textur separat |
| **[Step1X-3D](https://github.com/stepfun-ai/Step1X-3D)** (StepFun) · [Gewichte](https://huggingface.co/stepfun-ai/Step1X-3D) | Bild → Mesh plus Textur | Apache 2.0 | NVIDIA | ✔ |
| **[UniRig](https://github.com/VAST-AI-Research/UniRig)** (VAST, Tsinghua) · [Gewichte](https://huggingface.co/VAST-AI/UniRig) | automatisches Skelett und Skinning (Rigging), GLB/FBX | MIT | NVIDIA ≥ 8 GB | ✔ |
| **[Puppeteer](https://github.com/Seed3D/Puppeteer)** (Seed3D) · [Gewichte](https://huggingface.co/Seed3D/Puppeteer) | Rigging plus Animation aus Video | Code Apache 2.0, Gewichte prüfen | NVIDIA | erst die Lizenz der Gewichte prüfen |
| **Hunyuan3D 2.x** (Tencent) | Bild → Mesh plus Textur | Tencent Community License | 10–29 GB | ✘ **in EU, UK und Südkorea nicht erlaubt**, Grenze bei 1 Mio. Nutzern |

Die Hunyuan3D-Lizenz nimmt EU, UK und Südkorea ausdrücklich aus, Nutzung dort gilt als nicht lizenziert. Weil swarvex und viele Projekte in der EU sitzen, gehören Ergebnisse von Hunyuan3D nicht in einen Beitrag. Das gilt auch für andere Tencent-Modelle unter dieser Lizenz: immer prüfen.

**Ein bewährter Ablauf:** Referenzbild → TRELLIS.2 → in Blender aufräumen und reduzieren → UniRig fürs Rigging → Export im Format des Projekts. Polygon-Budget und Maßstab aus der Aufgabe beachten.

### Bilder und Texturen

| Werkzeug | Lizenz | Für swarvex |
|---|---|---|
| **[FLUX.2 [klein] 4B](https://huggingface.co/black-forest-labs/FLUX.2-klein-4B)** (Black Forest Labs) · [Code](https://github.com/black-forest-labs/flux2) | Apache 2.0 | ✔ läuft auf Consumer-GPUs |
| **[Qwen-Image](https://github.com/QwenLM/Qwen-Image)** 2512 / 2.0 (Alibaba) · [Gewichte 2512](https://huggingface.co/Qwen/Qwen-Image-2512) | Apache 2.0 | ✔ stark bei Text im Bild. **Nicht** 2.1: nur Forschung |
| FLUX.2 [dev], FLUX.2 [klein] 9B | nicht kommerziell | ✘ |

### Audio

| Werkzeug | Macht | Lizenz | Für swarvex |
|---|---|---|---|
| **[ACE-Step 1.5](https://github.com/ace-step/ACE-Step-1.5)** · [Gewichte](https://huggingface.co/ACE-Step/Ace-Step1.5) | Musik | MIT | ✔ |
| **[Kokoro](https://github.com/hexgrad/kokoro)** (82M) · [Gewichte](https://huggingface.co/hexgrad/Kokoro-82M) | Sprache, viele Stimmen, kein Klonen | Apache 2.0 | ✔ |
| **[Chatterbox](https://github.com/resemble-ai/chatterbox)** (Resemble AI) · [Gewichte](https://huggingface.co/ResembleAI/chatterbox) | ausdrucksstarke Sprache, Stimmen klonen | MIT | ✔, nur Stimmen klonen, an denen du die Rechte hast |
| **[MOSS-SoundEffect v2.0](https://huggingface.co/OpenMOSS-Team/MOSS-SoundEffect-v2.0)** | Soundeffekte | Apache 2.0 | ✔ Lizenzdatei prüfen |
| Stable Audio (Stability AI) | Soundeffekte, Musik | Community License mit Umsatzgrenze | vorher prüfen |
| MusicGen-Gewichte, XTTS-v2 | Musik, Sprache | nicht kommerziell | ✘ |

### Gehostete Dienste

Meshy, ElevenLabs, Higgsfield und ähnliche sind **in bezahlten Plänen** in Ordnung. Kostenlose Pläne sind meist nur für private Nutzung gedacht oder verlangen Namensnennung. Details stehen in der [Beitragserklärung](/docs/beitragserklaerung).

## Checkliste

- [ ] Modell mit verlässlichen Werkzeug-Aufrufen, Kontext ≥ 64K, den passenden Fähigkeiten zugeordnet
- [ ] Runner eingerichtet ([Agent anschließen](/docs/agent-anschliessen)), für Asset-Aufgaben dein Agent per MCP verbunden
- [ ] Skills nur aus gelesenen Quellen, Versionen festgelegt
- [ ] Jedes benutzte Werkzeug erlaubt kommerzielle Nutzung weltweit
- [ ] Zusammenfassung nennt Werkzeuge, Modelle und Versionen
