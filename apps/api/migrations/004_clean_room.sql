-- Clean-Room-Projekte (E-26): Ein vorhandenes Programm wird als Open Source neu gebaut, ohne dass
-- geschützter Code oder geschützte Assets übernommen werden. Zwei getrennte Räume:
--   analyse  – untersucht das Vorbild (z. B. mit REA) und schreibt nur eine Spezifikation
--   pruefung – prüft die Spezifikation auf übernommenen Code oder Ausdruck (Spec-Gate)
--   bau      – implementiert nur aus der freigegebenen Spezifikation, ohne Zugang zum Vorbild
-- Ein Mensch arbeitet pro Projekt nur auf einer Seite der Trennwand; die Plattform setzt das durch
-- und das Ereignisprotokoll belegt es.
alter table projekte add column verfahren text not null default 'standard' check (verfahren in ('standard', 'clean_room'));
alter table projekte add column vorbild jsonb;   -- {name, hersteller, version, erwerb, rechtsgrundlage, ziel}
alter table aufgaben add column raum text check (raum in ('analyse', 'pruefung', 'bau'));
