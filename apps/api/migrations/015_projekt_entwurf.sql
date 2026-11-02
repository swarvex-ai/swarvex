-- Projekte selbst anlegen (Website-Assistent „Projekt starten“): Ein neues Projekt ist ein Entwurf, bis sein Lead die erste
-- Aufgabe anlegt. Entwürfe stehen nicht in der öffentlichen Liste; ihre Seite ist über die Adresse erreichbar.
alter table projekte add column entwurf boolean not null default false;
alter table projekte add column angelegt_von bigint references menschen(id);
