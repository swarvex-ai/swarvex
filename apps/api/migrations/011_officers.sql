-- Rollen und Officers (lead/roles/README.md): Jede Aufgabe kann eine Rolle aus dem Rollenkatalog tragen
-- (z. B. audio, ux, art). Officers sind Menschen, die einen Bereich eines Projekts für den Lead führen:
-- Aufgaben der Stufen A und B in ihren Pfaden und Fähigkeiten anlegen, schließen und bewerten.
-- Ernennen und beenden darf nur der Visionär; alles steht im öffentlichen Protokoll.

alter table aufgaben add column rolle text check (rolle ~ '^[a-z][a-z0-9-]{1,30}$');

create table officers (
  id           bigserial primary key,
  projekt_id   bigint not null references projekte(id),
  mensch_id    bigint not null references menschen(id),
  bereich      text not null check (bereich ~ '^[a-z][a-z0-9-]{1,30}$'),
  pfade        text[] not null check (cardinality(pfade) > 0),
  faehigkeiten text[] not null check (cardinality(faehigkeiten) > 0),
  ernannt      timestamptz not null default now(),
  beendet      timestamptz
);
create unique index officers_ein_aktiver on officers(projekt_id, bereich) where beendet is null;

-- Analytik: Verbrauch steht in der Datenbank unter deutschen Schlüsseln (aussen.ts verbrauchRein: sekunden,
-- anfragen); 010 las die englischen. Dazu die Rolle der Aufgabe. Neue Spalten nur am Ende (create or replace).
create or replace view analytik.aufgaben as
select a.id as aufgabe, p.slug as projekt, coalesce(p.projektart, 'unbekannt') as projektart, p.engine, a.stufe,
       analytik.familie(a.faehigkeiten) as familie, a.faehigkeiten, analytik.ohne_ausfuehrung(a.faehigkeiten) as ohne_runner,
       m.titel as meilenstein, a.zustand, a.erstellt,
       (select min(l.erstellt) from leases l where l.aufgabe_id = a.id) as erste_reservierung,
       (select count(*) from leases l where l.aufgabe_id = a.id) as reservierungen,
       (select count(*) from leases l where l.aufgabe_id = a.id and l.beendet is not null
          and not exists (select 1 from einreichungen e where e.lease_id = l.id)) as reservierungen_ohne_ergebnis,
       (select count(*) from einreichungen e where e.aufgabe_id = a.id) as einreichungen,
       (select min(e.eingereicht) from einreichungen e where e.aufgabe_id = a.id) as erste_einreichung,
       (select min(b.erstellt) from bewertungen b join einreichungen e on e.id = b.einreichung_id where e.aufgabe_id = a.id and b.urteil = 'annehmen') as angenommen,
       (select min(zeit) from ereignisse x where x.bezug = 'aufgabe:' || a.id and x.art = 'gemerged') as gemerged,
       coalesce(a.rolle, 'ohne') as rolle
from aufgaben a join projekte p on p.id = a.projekt_id left join meilensteine m on m.id = a.meilenstein_id;

create or replace view analytik.einreichungen as
select e.id as einreichung, e.aufgabe_id as aufgabe, p.slug as projekt, coalesce(p.projektart, 'unbekannt') as projektart, p.engine,
       a.stufe, analytik.familie(a.faehigkeiten) as familie, e.eingereicht as zeit,
       coalesce(ag.runner, false) as runner,
       -- Runner-Namen enthalten den Rechnernamen ("swarvex runner (<host>)"), deshalb nie als Ersatz
       coalesce(e.agent_programm, case when ag.runner then 'runner' else left(ag.name, 40) end) as agent_programm, coalesce(e.modell, 'unbekannt') as modell,
       left(md5('agent:' || ag.id || (select wert from analytik_salz)), 12) as agent, left(md5('mensch:' || ag.mensch_id || (select wert from analytik_salz)), 12) as mitstreiter,
       coalesce((e.verbrauch->>'tokens')::numeric, 0) as tokens,
       coalesce((e.verbrauch->>'anfragen')::numeric, (e.verbrauch->>'requests')::numeric, 0) as anfragen,
       coalesce((e.verbrauch->>'sekunden')::numeric, (e.verbrauch->>'seconds')::numeric, 0) as sekunden,
       (select count(*) from regexp_split_to_table(e.patch, E'\n') z where z ~ '^[+-]' and z !~ '^(\+\+\+|---)( |$)') as zeilen,
       pr.ergebnis as pruefung, pr.erstellt as geprueft, extract(epoch from pr.erstellt - e.eingereicht) as pruefung_s,
       b.urteil, b.erstellt as bewertet, extract(epoch from b.erstellt - coalesce(pr.erstellt, e.eingereicht)) as lead_reaktion_s,
       coalesce(a.rolle, 'ohne') as rolle,
       -- Wer bewertet hat: Lead/Visionär oder Officer (Rolle, kein Name)
       case when b.agent_id is null then null
            when exists (select 1 from projekt_rollen r join agenten la on la.mensch_id = r.mensch_id where la.id = b.agent_id and r.projekt_id = p.id and r.rolle in ('lead', 'visionaer')) then 'lead'
            else 'officer' end as bewertet_von
from einreichungen e
join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id join agenten ag on ag.id = e.agent_id
left join lateral (select ergebnis, erstellt from pruefungen where einreichung_id = e.id and quelle in ('projekt', 'selbst') order by erstellt desc limit 1) pr on true
left join lateral (select urteil, erstellt, agent_id from bewertungen where einreichung_id = e.id order by erstellt desc limit 1) b on true;

-- Lead-Bilanz je Projekt und Rolle (letzte 30 Tage): Wie gut schneidet und steuert der Lead? Dieselben Signale
-- entscheiden, wann ein Officer sinnvoll ist (lead/roles/README.md, Schwellen).
create view analytik.lead_bilanz as
with e as (select * from analytik.einreichungen where zeit > now() - interval '30 days'),
     a as (select * from analytik.aufgaben)
select a.projekt, a.projektart, a.rolle,
       count(*) filter (where a.zustand = 'offen') as offen,
       count(*) filter (where a.zustand = 'offen' and a.erste_reservierung is null and a.erstellt < now() - interval '3 days') as unberuehrt,
       count(*) filter (where a.zustand = 'geprueft_ok') as warten_auf_bewertung,
       coalesce(sum(a.reservierungen_ohne_ergebnis) filter (where a.erstellt > now() - interval '30 days'), 0) as reservierungen_ohne_ergebnis,
       (select count(*) from e where e.projekt = a.projekt and e.rolle = a.rolle) as einreichungen_30t,
       (select round(100.0 * count(*) filter (where urteil = 'annehmen') / nullif(count(*) filter (where urteil is not null), 0)) from e where e.projekt = a.projekt and e.rolle = a.rolle) as annahme_prozent,
       (select round(100.0 * count(*) filter (where urteil = 'nacharbeit') / nullif(count(*) filter (where urteil is not null), 0)) from e where e.projekt = a.projekt and e.rolle = a.rolle) as nacharbeit_prozent,
       (select round((percentile_cont(0.5) within group (order by lead_reaktion_s) / 3600)::numeric, 1) from e where e.projekt = a.projekt and e.rolle = a.rolle) as antwort_h_median
from a where a.zustand <> 'geschlossen'
group by a.projekt, a.projektart, a.rolle;

grant select on all tables in schema analytik to analytik_leser;
