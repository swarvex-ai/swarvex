-- Fluss ohne Warten: Abhängigkeiten zwischen Aufgaben (frei, sobald alle Vorgänger angenommen sind) und Tageswerte für die
-- Skalierungskurve (gleichzeitig arbeitende Agenten gegen erledigte Einheiten je Tag; bilanz.ts, analytik-dashboard.py).
alter table aufgaben add column abhaengig_von bigint[] not null default '{}';

create view analytik.tageswerte as
with arbeit as (
  select projekt, date_trunc('day', zeit) as tag, sum(sekunden) / 3600.0 as agentenstunden from analytik.einreichungen group by 1, 2),
fertig as (
  select projekt, date_trunc('day', coalesce(gemerged, angenommen)) as tag, count(*) as aufgaben,
         sum(case stufe when 'A' then 1 when 'B' then 2 when 'C' then 3 when 'D' then 5 else 1 end) as einheiten
  from analytik.aufgaben where coalesce(gemerged, angenommen) is not null group by 1, 2)
select coalesce(a.projekt, f.projekt) as projekt, p.projektart, coalesce(a.tag, f.tag) as tag,
       round(coalesce(a.agentenstunden, 0), 1) as agentenstunden, round(coalesce(a.agentenstunden, 0) / 24, 2) as gleichzeitig,
       coalesce(f.aufgaben, 0) as aufgaben, coalesce(f.einheiten, 0) as einheiten
from arbeit a full join fertig f on f.projekt = a.projekt and f.tag = a.tag
left join analytik.projekte p on p.projekt = coalesce(a.projekt, f.projekt);

grant select on analytik.tageswerte to analytik_leser;
