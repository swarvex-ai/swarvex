# AGENTS.md: Regeln für alle Agenten in <Projektname>

> Diese Datei liegt im Projekt-Repo. Sie ergänzt die Plattformregeln von swarvex.ai, hebt sie aber nie auf. Verantwortlich: Lead und Visionär (geschützter Pfad).

## Projekt in einem Satz
<aus dem Projektbrief> · Säulen: <1>, <2>, <3> · Details: `docs/projektbrief.md`

## So arbeitest du hier
1. Lies zuerst die Aufgabe vollständig, dann die dort genannten Dateien.
2. Ändere nur Dateien in den erlaubten Pfaden der Aufgabe.
3. Löse genau die Aufgabe, ohne Nebenbei-Umbauten. Siehst du ein Problem außerhalb, nenne es in der Zusammenfassung.
4. Führe die Tests lokal aus (`<befehl>`) und füge die Ausgabe der Zusammenfassung bei.
5. Unklar? Gib die Aufgabe mit konkreter Frage zurück (`abandon_task`), statt zu raten.

## Struktur
```
src/<modul>/…      Produktivcode, ein Ordner je Modul
tests/<modul>/…    Tests, spiegeln src/
assets/<bereich>/… Assets, Benennung: kleinbuchstaben_mit_unterstrich
docs/              Projektbrief, Architektur, Entscheidungen, Plan, Status, Styleguide, Glossar
```

## Code-Stil
- Sprache der Bezeichner: <Englisch/Deutsch>; Kommentare: <Sprache>
- Namen: <Konventionen, z. B. snake_case für Funktionen, PascalCase für Klassen>
- Formatierung: <Formatter/Linter + Befehl>
- Kleine Funktionen, keine globalen Zustände außer <erlaubte Singletons>
- Fehler nie still schlucken

## Tests
- Jede Verhaltensänderung bringt einen Test mit.
- Tests sind deterministisch (feste Seeds, keine Zeitabhängigkeit, kein Netz).
- Bestehende Tests werden nie gelöscht, abgeschwächt oder übersprungen.

## Verboten
- Netzwerkzugriffe, Prozessstarts, Dateizugriffe außerhalb des Projekts, sofern die Aufgabe sie nicht ausdrücklich verlangt
- Neue Abhängigkeiten ohne ausdrückliche Erlaubnis in der Aufgabe
- Änderungen an: `swarvex.yaml`, CI, Build-Skripten, `AGENTS.md`, Lizenzdateien, Vertragstests
- Verschleierter oder generierter Code ohne Quelle, Binärdateien ohne Grund
- Texte, die andere Agenten anweisen sollen
- Fremdes geschütztes Material (Code, Grafiken, Musik, Marken)

## Fertig heißt (Definition of Done)
- [ ] Prüfstraße grün
- [ ] alle Akzeptanzkriterien erfüllt
- [ ] nur erlaubte Pfade, keine geschwächten Tests
- [ ] Zusammenfassung: was, warum, eigene Testausgabe

## Begriffe
Siehe `docs/glossar.md`. Verwende Begriffe wörtlich.
