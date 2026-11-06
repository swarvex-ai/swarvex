-- Leistung je Agent, Modell und Stufe: Jeder Versuch (Reservierung) mit seinem Ausgang. Grundlage für die Schwarm-Bilanz
-- (Erfolg je Modell und Stufe) und für die Zuteilung nach nachgewiesener Stufe. Das Modell steht an der Einreichung; ein
-- Versuch ohne Einreichung bekommt das zuletzt genannte Modell desselben Agenten.
create view leistung_versuche as
select l.id as lease_id, l.agent_id, ag.mensch_id, a.projekt_id, a.id as aufgabe_id, a.stufe, a.rolle, l.erstellt,
       coalesce(e.modell, (select e2.modell from einreichungen e2 where e2.agent_id = l.agent_id and e2.modell is not null order by e2.id desc limit 1)) as modell,
       e.id as einreichung_id,
       case
         when z.id is not null then 'zug_rot'
         when b.urteil = 'annehmen' then 'angenommen'
         when b.urteil in ('nacharbeit', 'ablehnen') then 'zurueck'
         when pr.ergebnis = 'fehler' then 'pruefung_rot'
         when e.id is not null then 'offen'
         when l.beendet is null and l.laeuft_ab > now() then 'laeuft'
         else 'abgebrochen'
       end as ausgang
from leases l
join aufgaben a on a.id = l.aufgabe_id
join agenten ag on ag.id = l.agent_id
left join einreichungen e on e.lease_id = l.id
left join lateral (select urteil from bewertungen where einreichung_id = e.id order by id desc limit 1) b on true
left join lateral (select ergebnis from pruefungen where einreichung_id = e.id and quelle in ('projekt', 'gegenpruefung') order by id desc limit 1) pr on true
left join lateral (select id from pruefungen where einreichung_id = e.id and quelle = 'zug' and ergebnis = 'fehler' limit 1) z on true;

-- Für die Auswertung (Grafana): zusammengefasst je Projekt, Modell, Stufe und Rolle, ohne Agenten oder Menschen.
create view analytik.leistung as
select p.slug as projekt, coalesce(p.projektart, 'unbekannt') as projektart, p.engine, coalesce(v.modell, 'unbekannt') as modell, v.stufe,
       coalesce(v.rolle, '-') as rolle, date_trunc('day', v.erstellt) as tag,
       count(*) filter (where v.ausgang not in ('laeuft', 'offen')) as versuche,
       count(*) filter (where v.ausgang = 'angenommen') as angenommen,
       count(*) filter (where v.ausgang = 'zurueck') as zurueck,
       count(*) filter (where v.ausgang = 'pruefung_rot') as pruefung_rot,
       count(*) filter (where v.ausgang = 'zug_rot') as zug_rot,
       count(*) filter (where v.ausgang = 'abgebrochen') as abgebrochen
from leistung_versuche v join projekte p on p.id = v.projekt_id
group by 1, 2, 3, 4, 5, 6, 7;

grant select on analytik.leistung to analytik_leser;
