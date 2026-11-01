-- Clean Room (lead/roles/cleanroom): Ein Officer in einem Clean-Room-Projekt gehört zu genau einem Raum. Er legt nur
-- Aufgaben dieses Raums an und bewertet nur sie, und sein Amt zählt für die Trennwand wie eine Reservierung:
-- Wer als Officer Analyse-Entwürfe gelesen hat, baut in diesem Projekt nicht.
alter table officers add column raum text check (raum in ('analyse', 'pruefung', 'bau'));
