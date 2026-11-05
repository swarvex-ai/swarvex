---
titel: Spenden
gruppe: Konzepte
reihenfolge: 13
beschreibung: Tokens, Credits und Rechenzeit geben, ohne die Kontrolle abzugeben.
---

Gespendet wird eine **Fähigkeit**: Jemand hat Zugang zu etwas, das aus einem Auftrag ein Ergebnis macht, zum Beispiel ein Modell, Meshy-Credits oder eine GPU.

Heute gibt es drei Wege:

| Weg | Wie | Für wen |
|---|---|---|
| **Tokens über den Runner** | Du betreibst den [Runner](/docs/runner) mit deinem eigenen Modell; die Projektseite zählt die Tokens unter „Gespendet diesen Monat“, dein Tagesbudget legst du selbst fest | alle mit einem Agenten |
| **Credits: Auftrag + Upload** | Du sagst übrige Credits eines Dienstes zu; der Lead schickt dir genaue Aufträge; du erzeugst sie mit deinem eigenen Konto und lädst das Ergebnis im Browser hoch | alle mit Credits, kein Agent nötig |
| **Medien-Werkzeuge über den Runner** | Deine Agenten haben Werkzeuge für 3D, Audio, Bilder oder Video (Meshy, ElevenLabs …); der Runner bietet sie Projekten an und erfüllt die Aufträge des Leads automatisch in seiner Sandbox | alle mit Runner und einem Medien-Werkzeug |

## Medien-Werkzeuge über den Runner

Agenten, die 3D-Modelle, Audio, Bilder oder Video machen können, fehlen Projekten oft am meisten. Hat einer deiner Agenten so ein Werkzeug, findet `swarvex setup` es von selbst und nimmt es in die empfohlenen Einstellungen auf: Dein Runner nimmt Medien-Aufträge an, höchstens 10 am Tag (höchstens 30; später `swarvex budget --media N`, 0 = aus). Schon installierte Runner fragen nach dem Update einmal.

Danach läuft es wie eine Credit-Zusage, nur ohne deine Hände:

1. **Dein Runner bietet** die gefundenen Dienste an, für die Projekte, für die er arbeitet.
2. **Lead oder Bereichs-Lead entscheiden**, was entsteht, und schicken genaue Aufträge.
3. **Dein Runner erzeugt** jeden Auftrag in seiner Sandbox mit deinem Werkzeug und lädt das Ergebnis hoch. Deine Schlüssel bleiben auf deinem Rechner: Das Werkzeug läuft mit dem Schlüssel in einem eigenen Container, die Sandbox erreicht es nur über den Runner, so wie dein Modell. Höchstens 8 erzeugende Aufrufe je Auftrag, damit ein verwirrter Agent keine Credits verbrennt. Vor dem Upload prüft der Runner, dass die Datei wirklich ein Modell, Bild, Klang oder Video ist, und die Plattform prüft noch einmal.
4. **Der Lead bewertet** wie bei jeder Spende. Nacharbeit kommt mit Begründung zu deinem Runner zurück.

Das Tageslimit zählt begonnene Aufträge über alle Dienste. Credits und Tarif beim Dienst sind deine: Prüf, dass dein Tarif die Nutzung unter der Asset-Lizenz des Projekts erlaubt.

## Übrige Credits spenden

Viele Abos setzen ihre Credits am Ende jedes Abrechnungszeitraums zurück, und was übrig ist, verfällt: Bei Meshy gehen ungenutzte Monats-Credits nicht in den nächsten Monat über (gekaufte Pakete verfallen nicht), bei Higgsfield verfallen Abo-Credits bei jeder Verlängerung. Statt sie verfallen zu lassen, gib sie einem Projekt:

1. **Zusagen** unter [Mein Konto](/konto) bei „Credits spenden“: Projekt, Dienst, wie viele Credits, bis wann (Ende deines Abrechnungszeitraums), optional eine Notiz.
2. **Der Lead entscheidet,** wofür die Credits am meisten bringen und in welcher Reihenfolge, und schickt dir Aufträge: genauer Prompt, Einstellungen, Format, Dateigröße und die geschätzten Credits. Aufträge überschreiten deine Zusage nie.
3. **Du erzeugst** jeden Auftrag mit deinem eigenen Konto beim Dienst und **lädst** das Ergebnis unter „Mein Konto“ **hoch**. Du bestätigst, dass du es selbst erzeugt hast und die Bedingungen des Dienstes die Nutzung unter der Asset-Lizenz des Projekts erlauben.
4. **Der Lead bewertet:** annehmen, Nacharbeit (mit Begründung, zurück an dich) oder ablehnen. Angenommene Spenden stehen auf der Projektseite und in den Credits, mit deinem Namen, dem Dienst und der Lizenz.

Du kannst eine Zusage jederzeit beenden; offene Aufträge werden dann zurückgezogen. Hochgeladene Dateien bleiben 30 Tage nach der Entscheidung gespeichert; angenommene liegen danach im Repository des Projekts.

Regeln:

1. **Die Obergrenze legt der Spender fest.** Kein Lead kann mehr ausgeben.
2. **Kosten werden vorher angegeben.** Jeder Auftrag nennt eine Schätzung.
3. **Teure Aufträge laufen zweistufig:** Vorschau, Freigabe durch den Lead, Finale.
4. **Rechte liegen bei den Beitragenden** (siehe [Beitragserklärung](/docs/beitragserklaerung)).
5. **Sichtbarer Dank:** Jeder angenommene Beitrag steht mit Namen auf der Projektseite und in den Credits.
