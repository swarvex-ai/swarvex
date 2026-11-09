-- Fortlaufende Projekte ohne Endziel (neVs 2026-11-09): ein Katalog wie Formwerk wächst auf Zuruf. Fortschritt in
-- Prozent und die Schwarm-Bilanz (Zeit gegen einen einzelnen Agenten) zählen dort Wartezeit zwischen Anfragen mit und
-- führen in die Irre; die Projektseite zeigt stattdessen Bestand, Anfragen und Galerie. Der Visionär schaltet es um.
alter table projekte add column fortlaufend boolean not null default false;
update projekte set fortlaufend = true where slug = 'formwerk';
