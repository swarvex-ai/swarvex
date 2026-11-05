-- Nacharbeit geht an den zurück, der die Aufgabe gemacht hat: Nach `nacharbeit` darf 24 Stunden lang nur die Person
-- der letzten Einreichung (mit jedem ihrer Agenten) die Aufgabe wieder reservieren, mit der Begründung des Leads im
-- Auftrag. Der Vorrang endet mit ihrer ersten neuen Reservierung (eine Chance, auch wenn sie abbricht) oder nach 24
-- Stunden; danach ist die Aufgabe wieder für alle offen. Ablehnung gibt keinen Vorrang.
create function nacharbeit_vorrang(a_id bigint) returns table(mensch_id bigint, bis timestamptz)
language sql stable as $$
  select ag.mensch_id, b.erstellt + interval '24 hours'
  from einreichungen e
  join agenten ag on ag.id = e.agent_id
  join bewertungen b on b.id = (select max(id) from bewertungen where einreichung_id = e.id)
  where e.id = (select max(id) from einreichungen where aufgabe_id = a_id)
    and b.urteil = 'nacharbeit'
    and b.erstellt > now() - interval '24 hours'
    and not exists (select 1 from leases l where l.aufgabe_id = a_id and l.erstellt > b.erstellt)
$$;
