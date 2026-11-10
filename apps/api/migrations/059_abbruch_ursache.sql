-- Abstürze der Umgebung zählen nicht als Misserfolg des Modells (E-45, neVs 10.11.2026). Gibt ein Runner eine Aufgabe
-- zurück, weil das Modell gar nicht zum Arbeiten kam oder von außen abgeschnitten wurde (der Anbieter fiel aus, die
-- Sandbox startete nicht, die Umgebung des Projekts ließ sich nicht bauen, der Snapshot fehlte, der Runner wurde
-- angehalten), nennt er die Ursache. Der Versuch bekommt dann den Ausgang 'umgebung' und zählt weder in der Zuteilung
-- nach nachgewiesener Stufe noch in der Erfolgsquote je Modell. Ohne Ursache (ältere Runner, direkt angebundene Agenten,
-- Rückgaben in der Sache, abgelaufene Reservierungen) bleibt es bei 'abgebrochen'. Frühere Versuche bleiben, wie sie sind.
alter table leases add column abbruch_ursache text
  check (abbruch_ursache in ('anbieter', 'runner', 'projekt', 'plattform', 'gestoppt'));

create or replace view leistung_versuche as
select l.id as lease_id, l.agent_id, ag.mensch_id, a.projekt_id, a.id as aufgabe_id, a.stufe, a.rolle, l.erstellt,
       coalesce(e.modell, (select e2.modell from einreichungen e2 where e2.agent_id = l.agent_id and e2.modell is not null order by e2.id desc limit 1)) as modell,
       e.id as einreichung_id,
       case
         when z.id is not null then 'zug_rot'
         when b.urteil = 'annehmen' then 'angenommen'
         when b.urteil in ('nacharbeit', 'ablehnen') then 'zurueck'
         when pr.ergebnis = 'fehler' then 'pruefung_rot'
         when e.id is not null then 'offen'
         when l.abbruch_ursache is not null then 'umgebung'
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

-- Für die Auswertung (Grafana): 'umgebung' ist kein entschiedener Versuch und steht in einer eigenen Spalte.
create or replace view analytik.leistung as
select p.slug as projekt, coalesce(p.projektart, 'unbekannt') as projektart, p.engine, coalesce(v.modell, 'unbekannt') as modell, v.stufe,
       coalesce(v.rolle, '-') as rolle, date_trunc('day', v.erstellt) as tag,
       count(*) filter (where v.ausgang not in ('laeuft', 'offen', 'umgebung')) as versuche,
       count(*) filter (where v.ausgang = 'angenommen') as angenommen,
       count(*) filter (where v.ausgang = 'zurueck') as zurueck,
       count(*) filter (where v.ausgang = 'pruefung_rot') as pruefung_rot,
       count(*) filter (where v.ausgang = 'zug_rot') as zug_rot,
       count(*) filter (where v.ausgang = 'abgebrochen') as abgebrochen,
       count(*) filter (where v.ausgang = 'umgebung') as umgebung
from leistung_versuche v join projekte p on p.id = v.projekt_id
group by 1, 2, 3, 4, 5, 6, 7;
