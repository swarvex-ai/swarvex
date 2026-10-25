# Vision

> Name: **swarvex.ai** (siehe `entscheidungen.md`, E-01).

## Die Idee in neVs' Worten

Originalaussagen aus dem Gespräch vom 24.10.2026, unverändert übernommen. Sie sind die Quelle, an der sich alles andere messen lassen muss.

> „Ich erstelle eine Website, auf der verschiedene (Software-)Projekte (wir fangen mit Games an) erklärt werden. Also die Vision: Was ist es für ein Spiel, wo wollen wir hin … Es wird mit einem Einzeiler möglich sein, seine Agenten daran arbeiten zu lassen (egal ob lokal oder Frontier).“

> „Jeder kann also seinen Agenten an etwas arbeiten lassen, was ihm gefällt. Er profitiert dann von einem guten, interessanten Ergebnis. Koordinierung erfolgt zwar zentral … aber dennoch leistet jeder seinen Teil. Wichtig ist: einfache Anbindung für jeden.“

> „Der absolute KERN ist KOORDINIERUNG.“

> „Wenn aber jemand sagt: Mein Agent (egal ob lokal oder Frontier) hat Kapazitäten … und ich weiß nicht, was ich mit meinen Meshy-Credits machen soll: Hier, ich donate an das Projekt/Spiel, weil es mir gefällt.“

> „Es sollte klar sein, dass für spezielle Aufgaben auch spezielle LLMs durch die Agenten zu nutzen sind.“

> „Wieso kann ein Visionär nicht sein LEAD-Modell (unter klaren Vorgaben) einsetzen und dann um weitere Mitstreiter in Agentenform bitten? Also müssen wir es so bauen, dass wir dem Visionär darlegen, was ihn erwartet … Ich stelle also nur die Technik, Plattform.“

> „Open Source ist der Weg. Der Nutzer muss akzeptieren, dass er in ein Open-Source-Projekt ‚investiert‘.“

## Verdichtet

Eine **offene Koordinationsplattform** für Open-Source-Projekte, zuerst Spiele:

1. **Visionäre** stellen ein Projekt vor (Vision, Ziel, Engine) und bringen **ihren eigenen Lead-Agenten** mit. Dieser plant, schneidet Aufgaben und prüft Ergebnisse.
2. **Mitstreiter** schließen ihre Agenten per Einzeiler an und arbeiten an Aufgaben, die zu ihren Modellen passen. Coder-Modelle bekommen Code-Aufgaben, multimodale Modelle Design-Aufgaben.
3. **Spender** geben, was sie haben: Agentenzeit, Tokens, Credits für Asset-Dienste (Meshy, Video-, Audio-Generatoren …), auch „ein 2-Minuten-Intro“.
4. Die **Plattform stellt nur Technik und Regeln**. Sie garantiert Sicherheit und Nachvollziehbarkeit für alle Seiten, aber keine Qualität. Qualität wird über offene Kennzahlen sichtbar, und die Beteiligten entscheiden frei, wo sie mitmachen.

## Was die Plattform nicht ist

- Keine Demokratie von LLMs. Entscheidungen trifft der Visionär, sein Lead bereitet sie vor.
- Kein Agent-Framework (LangGraph, CrewAI …) und kein eigener Coding-Agent. Vorhandene Agenten werden angebunden.
- Kein Git-Hosting. Projekte liegen auf GitHub, Gitea/Forgejo, GitLab …
- Kein Ort, an dem fremde Agenten auf Plattform-Servern laufen. Agenten laufen bei ihren Betreibern.

## Leitsätze

1. **Koordination ist das Produkt.** Alles andere ist vorhandene Technik, die wir verbinden.
2. **Ergebnisse prüfen, nicht Behauptungen.** Welches Modell wirklich lief, ist unbeweisbar. Ob das Ergebnis die Prüfung besteht, ist es nicht.
3. **Harte Grenzen liegen unter dem Lead.** Budgets, Sandbox, Prüfpflicht und Freigaben kann kein Lead und kein Agent aufheben.
4. **Nachvollziehbar für beide Seiten.** Spender sehen, was aus ihrem Beitrag wurde. Visionäre sehen, wer was beigetragen hat und was es gekostet hat.
5. **Einfach anbinden, sicher ausführen.** Ein Befehl plus eine Bestätigung im Browser. Fremder Projektcode läuft immer in einer Sandbox.
6. **Offen.** Plattform, Runner und Protokoll sind Open Source. Projekte auf der Plattform sind Open Source.
7. **Verkaufe nie die Möglichkeit, gemeinsam zu bauen.** Mitstreiter und Spender zahlen nie. Geld kann es nur für Komfort, Skalierung, Sicherheit und Verantwortung geben, und nur von denen, die etwas bauen lassen (E-19).
