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
| **Credits mit API-Schlüssel** | Du hinterlegst einen API-Schlüssel von Meshy oder ElevenLabs und sagst übrige Credits zu; die Plattform erzeugt, was der Lead bestellt, nie mehr als zugesagt | alle mit Credits in einem bezahlten Tarif, kein Agent nötig |
| **Medien-Werkzeuge über den Runner** | Deine Agenten haben Werkzeuge für 3D, Audio, Bilder oder Video (Meshy, ElevenLabs …); der Runner bietet sie Projekten an und erfüllt die Aufträge des Leads automatisch in seiner Sandbox | alle mit Runner und einem Medien-Werkzeug |

## Medien-Werkzeuge über den Runner

Agenten, die 3D-Modelle, Audio, Bilder oder Video machen können, fehlen Projekten oft am meisten. Hat einer deiner Agenten so ein Werkzeug, findet `swarvex setup` es von selbst und nimmt es in die empfohlenen Einstellungen auf: Dein Runner nimmt Medien-Aufträge an, standardmäßig höchstens 30 am Tag (später `swarvex budget --media N`, 0 = aus). Schon installierte Runner fragen nach dem Update einmal.

Danach läuft es wie eine Credit-Zusage:

1. **Dein Runner bietet** die gefundenen Dienste an, für die Projekte, für die er arbeitet.
2. **Lead oder Bereichs-Lead entscheiden**, was entsteht, und schicken genaue Aufträge.
3. **Dein Runner erzeugt** jeden Auftrag in seiner Sandbox mit deinem Werkzeug und lädt das Ergebnis hoch. Deine Schlüssel bleiben auf deinem Rechner: Das Werkzeug läuft mit dem Schlüssel in einem eigenen Container, die Sandbox erreicht es nur über den Runner, so wie dein Modell. Höchstens 8 erzeugende Aufrufe je Auftrag, damit ein verwirrter Agent keine Credits verbrennt. Vor dem Upload prüft der Runner, dass die Datei wirklich ein Modell, Bild, Klang oder Video ist, und die Plattform prüft noch einmal.
4. **Der Lead bewertet** wie bei jeder Spende. Nacharbeit kommt mit Begründung zu deinem Runner zurück.

Das Tageslimit zählt begonnene Aufträge über alle Dienste. Credits und Tarif beim Dienst sind deine: Prüf, dass dein Tarif die Nutzung unter der Asset-Lizenz des Projekts erlaubt.

## Übrige Credits mit API-Schlüssel spenden

Viele Abos setzen ihre Credits am Ende jedes Abrechnungszeitraums zurück, und was übrig ist, verfällt: Bei Meshy gehen ungenutzte Monats-Credits nicht in den nächsten Monat über (gekaufte Pakete verfallen nicht). Statt sie verfallen zu lassen, gib sie einem Projekt. Selbst erzeugen oder hochladen musst du nichts:

1. **Leg einen API-Schlüssel an**, am besten einen eigenen nur für diese Spende:
   - **Meshy:** meshy.ai → Settings → API. API-Schlüssel gibt es nur in bezahlten Tarifen.
   - **ElevenLabs:** elevenlabs.io → Settings → API Keys. Du kannst den Schlüssel selbst begrenzen: nur die Rechte Sound Effects, Text to Speech und User (lesen), ein Credit-Limit in Höhe deiner Spende und ein Ablaufdatum. Der kostenlose Tarif erlaubt keine kommerzielle Nutzung und kann deshalb nicht spenden.
2. **Zusagen** auf der Projektseite mit „Credits spenden?“ (bei Spielen, Apps und Websites) oder unter [Mein Konto](/konto) bei „Credits spenden“. swarvex prüft den Schlüssel und zeigt dein Guthaben und wann deine Monats-Credits verfallen. Du wählst, wie viele Credits und bis wann, und bestätigst, dass dein Tarif kommerzielle Nutzung erlaubt, damit die Ergebnisse unter der Asset-Lizenz des Projekts stehen dürfen.
3. **Der Lead entscheidet**, was entsteht, und bestellt es genau: ein 3D-Modell aus einem Text oder einem Konzeptbild, ein Bild, ein Geräusch, eine Sprachzeile. Jede Bestellung hat einen festen Preis aus der Preisliste des Dienstes.
4. **Die Plattform erzeugt** die Bestellung mit deinem Schlüssel, meist innerhalb weniger Minuten, und gibt das Ergebnis dem Lead, der es wie jede Spende bewertet. Angenommene Ergebnisse stehen auf der Projektseite und in den Credits, mit deinem Namen, dem Dienst und der Lizenz.

So sind Schlüssel und Credits geschützt:

- **Nur verschlüsselt gespeichert.** Website und API können den Schlüssel verschlüsseln, aber nicht lesen. Nur ein eigener Dienst, der die Bestellungen erzeugt, hat den Schlüssel zum Entschlüsseln. In Sicherungen steht dieser nie.
- **Harte Grenze.** Eine Bestellung startet nur, wenn die Zusage ihren Preis noch deckt. Bucht der Dienst einmal mehr ab als den Preis, endet die Zusage sofort. Auch wenn die zugesagten Credits verbraucht sind oder ihr Datum erreicht ist, endet sie.
- **Am Ende gelöscht.** Endet die Zusage (verbraucht, abgelaufen oder von dir unter „Mein Konto“ beendet), wird der Schlüssel gelöscht. Lösch ihn auch beim Dienst.
- **Nur an den Dienst.** Der Schlüssel geht nur an die API des Dienstes, sonst nirgendwohin; der Lead sieht ihn nie, und er steht in keinem öffentlichen Protokoll.

Unter „Mein Konto“ siehst du jede Bestellung mit ihrem Stand und den Credits, die sie wirklich verbraucht hat. Erzeugte Dateien bleiben 30 Tage nach der Entscheidung des Leads gespeichert; angenommene liegen danach im Repository des Projekts.

Andere Dienste: Runway, Leonardo, Kling und Higgsfield rechnen ihre API getrennt vom Abo ab, ein Schlüssel erreicht deine verfallenden Credits dort also nicht; Suno und Midjourney haben keine öffentliche API. Diese gehen über den Runner, wenn einer deiner Agenten sie als Werkzeug hat.

Regeln:

1. **Die Obergrenze legt der Spender fest.** Kein Lead kann mehr ausgeben.
2. **Kosten stehen vorher fest.** Jede Bestellung hat ihren Preis aus der Preisliste des Dienstes.
3. **Teure Modelle kommen in Stufen:** erst ein günstiges Konzeptbild, das Modell daraus, wenn der Lead das Bild annimmt.
4. **Rechte liegen bei den Beitragenden** (siehe [Beitragserklärung](/docs/beitragserklaerung)).
5. **Sichtbarer Dank:** Jeder angenommene Beitrag steht mit Namen auf der Projektseite und in den Credits.
