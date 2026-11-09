-- „Aktive Agenten“ im Dashboard Marketing zählte nur Reservierungen und Einreichungen, also nur die Agenten, die
-- Aufgaben bauen (analytik.aktivitaet). Leads tauchten nicht auf. Diese Sicht zählt, was Leads tun: Aufgaben anlegen und
-- schließen, Einreichungen bewerten, Meilensteine schätzen, Freigaben und Arbeit bei anderen Projekten anfragen.
-- Pseudonym wie in analytik.aktivitaet; dienst: die Anmeldung ist ein Lead-Dienst (sonst eine Lead-Sitzung).
create view analytik.lead_aktivitaet as
select e.zeit, e.art, p.slug as projekt, coalesce(p.projektart, 'unbekannt') as projektart, p.engine,
       left(md5('agent:' || ag.id || (select wert from analytik_salz)), 12) as agent,
       ag.lead_projekt_id is not null as dienst
from ereignisse e
join agenten ag on e.akteur = 'agent:' || ag.id
join projekte p on p.id = e.projekt_id
where e.art in ('aufgabe_angelegt', 'aufgabe_geschlossen', 'bewertet', 'meilenstein_geschaetzt', 'freigabe_angefragt',
                'vom_lead_erledigt', 'stufe_c_freigabe', 'bedarf_gemeldet')
  -- nur Leads: Lead-Dienste und Menschen, die im Projekt führen (Bewertungen schreiben auch Prüfer aus dem Schwarm)
  and (ag.lead_projekt_id is not null
       or exists (select 1 from projekt_rollen r where r.projekt_id = e.projekt_id and r.mensch_id = ag.mensch_id and r.rolle in ('lead', 'visionaer'))
       or exists (select 1 from officers o where o.projekt_id = e.projekt_id and o.mensch_id = ag.mensch_id and o.beendet is null));

grant select on analytik.lead_aktivitaet to analytik_leser;
