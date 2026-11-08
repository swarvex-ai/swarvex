-- Fortschritt ehrlich (Entscheidung neVs 2026-11-08, E-41): Leads planen rollierend, die angelegten Aufgaben sind nur ein
-- Teil eines Meilensteins. Der Lead schätzt deshalb je Meilenstein, wie viele Aufgaben er insgesamt braucht; die
-- Projektseite zeigt den Fortschritt gegen diese Schätzung und ohne Schätzung nur „erledigt von geplant“.
alter table meilensteine
  add column ziel text,
  add column schaetzung int check (schaetzung between 1 and 10000),
  add column geschaetzt timestamptz;
