-- Interne Auswertung (Grafana, Marketing): Projektart, Agentenprogramm und Modell je Einreichung,
-- dazu Sichten im Schema analytik. Die Sichten enthalten keine personenbezogenen Daten: keine Logins,
-- keine Mensch-IDs, keine Texte; Menschen nur als nicht rückführbares Pseudonym (für eindeutige Zählungen).
-- Die Rolle analytik_leser darf nur diese Sichten lesen; Passwort und LOGIN setzt infra/analytics-setup.sh.

alter table projekte add column projektart text
  check (projektart in ('spiel', 'app', 'website', 'dienst', 'tool', 'bibliothek', 'plugin', 'sonstiges'));
alter table einreichungen add column agent_programm text, add column modell text;

create schema analytik;
do $$ begin
  if not exists (select 1 from pg_roles where rolname = 'analytik_leser') then create role analytik_leser nologin; end if;
end $$;

-- Pseudonym: Hash mit einem Geheimnis, das nur in einer Tabelle außerhalb von analytik liegt. Die Sichten lesen es
-- mit den Rechten ihres Besitzers; der Leser kommt weder an das Geheimnis noch an die Tabellen dahinter.
create table analytik_salz (wert text not null);
insert into analytik_salz select md5(random()::text || clock_timestamp()::text);

-- Fähigkeitsfamilie einer Aufgabe: code, test, text, design, asset, review, daten, simulation; mehrere = gemischt
create function analytik.familie(f text[]) returns text language sql immutable as $$
  select case when cardinality(f) = 0 then 'keine'
              when (select count(distinct split_part(x, '.', 1)) from unnest(f) x) = 1 then split_part(f[1], '.', 1)
              else 'gemischt' end $$;

-- Läuft ohne Runner (Direkt-Modus, aufgaben.ts OHNE_AUSFUEHRUNG)
create function analytik.ohne_ausfuehrung(f text[]) returns boolean language sql immutable as $$
  select cardinality(f) > 0 and not exists (select 1 from unnest(f) x where x !~ '^(text\.|design\.|asset\.)' and x not in ('review', 'daten')) $$;

create view analytik.projekte as
select p.slug as projekt, p.name, coalesce(p.projektart, 'unbekannt') as projektart, p.engine, p.verfahren,
       (select min(zeit) from ereignisse e where e.projekt_id = p.id and e.art = 'vision_veroeffentlicht') as veroeffentlicht,
       count(a.id) filter (where a.zustand <> 'geschlossen') as aufgaben,
       count(a.id) filter (where a.zustand = 'offen') as offen,
       count(a.id) filter (where a.zustand in ('reserviert', 'in_arbeit')) as in_arbeit,
       count(a.id) filter (where a.zustand in ('eingereicht', 'pruefung', 'geprueft_ok', 'geprueft_fehler')) as in_pruefung,
       count(a.id) filter (where a.zustand in ('angenommen', 'gemerged')) as erledigt,
       p.gesperrt
from projekte p left join aufgaben a on a.projekt_id = p.id
group by p.id;

create view analytik.aufgaben as
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
       (select min(zeit) from ereignisse x where x.bezug = 'aufgabe:' || a.id and x.art = 'gemerged') as gemerged
from aufgaben a join projekte p on p.id = a.projekt_id left join meilensteine m on m.id = a.meilenstein_id;

create view analytik.einreichungen as
select e.id as einreichung, e.aufgabe_id as aufgabe, p.slug as projekt, coalesce(p.projektart, 'unbekannt') as projektart, p.engine,
       a.stufe, analytik.familie(a.faehigkeiten) as familie, e.eingereicht as zeit,
       coalesce(ag.runner, false) as runner,
       -- Runner-Namen enthalten den Rechnernamen ("swarvex runner (<host>)"), deshalb nie als Ersatz
       coalesce(e.agent_programm, case when ag.runner then 'runner' else left(ag.name, 40) end) as agent_programm, coalesce(e.modell, 'unbekannt') as modell,
       left(md5('agent:' || ag.id || (select wert from analytik_salz)), 12) as agent, left(md5('mensch:' || ag.mensch_id || (select wert from analytik_salz)), 12) as mitstreiter,
       coalesce((e.verbrauch->>'tokens')::numeric, 0) as tokens, coalesce((e.verbrauch->>'requests')::numeric, 0) as anfragen,
       coalesce((e.verbrauch->>'seconds')::numeric, 0) as sekunden,
       (select count(*) from regexp_split_to_table(e.patch, E'\n') z where z ~ '^[+-]' and z !~ '^(\+\+\+|---)( |$)') as zeilen,
       pr.ergebnis as pruefung, pr.erstellt as geprueft, extract(epoch from pr.erstellt - e.eingereicht) as pruefung_s,
       b.urteil, b.erstellt as bewertet, extract(epoch from b.erstellt - coalesce(pr.erstellt, e.eingereicht)) as lead_reaktion_s
from einreichungen e
join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id join agenten ag on ag.id = e.agent_id
left join lateral (select ergebnis, erstellt from pruefungen where einreichung_id = e.id and quelle in ('projekt', 'selbst') order by erstellt desc limit 1) pr on true
left join lateral (select urteil, erstellt from bewertungen where einreichung_id = e.id order by erstellt desc limit 1) b on true;

-- Jede Arbeitsspur als Zeile: Reservierung, Einreichung. Für eindeutige Zählungen von Agenten und Mitstreitern im Zeitraum.
create view analytik.aktivitaet as
select l.erstellt as zeit, 'reservierung' as art, p.slug as projekt, coalesce(p.projektart, 'unbekannt') as projektart, p.engine,
       analytik.familie(a.faehigkeiten) as familie, coalesce(ag.runner, false) as runner,
       left(md5('agent:' || ag.id || (select wert from analytik_salz)), 12) as agent, left(md5('mensch:' || ag.mensch_id || (select wert from analytik_salz)), 12) as mitstreiter, 0::numeric as tokens
from leases l join aufgaben a on a.id = l.aufgabe_id join projekte p on p.id = a.projekt_id join agenten ag on ag.id = l.agent_id
union all
select zeit, 'einreichung', projekt, projektart, engine, familie, runner, agent, mitstreiter, tokens from analytik.einreichungen;

-- Was ist gerade angesagt: letzte 7 Tage gegen die 7 davor, je Projekt
create view analytik.trend as
with w as (
  select projekt, projektart, engine,
         count(*) filter (where zeit > now() - interval '7 days') as aktionen_7t,
         count(*) filter (where zeit <= now() - interval '7 days' and zeit > now() - interval '14 days') as aktionen_vorher,
         count(distinct agent) filter (where zeit > now() - interval '7 days') as agenten_7t,
         count(distinct agent) filter (where zeit <= now() - interval '7 days' and zeit > now() - interval '14 days') as agenten_vorher,
         count(distinct mitstreiter) filter (where zeit > now() - interval '7 days') as mitstreiter_7t,
         sum(tokens) filter (where zeit > now() - interval '7 days') as tokens_7t
  from analytik.aktivitaet group by 1, 2, 3)
select *, case when aktionen_vorher = 0 then null else round(100.0 * (aktionen_7t - aktionen_vorher) / aktionen_vorher) end as wachstum_prozent
from w;

-- Schwarm gegenüber einem einzelnen Agenten, je Projekt (Definitionen im Dashboard)
create view analytik.schwarm as
with e as (select * from analytik.einreichungen),
     l as (select a.projekt_id, l.erstellt, coalesce(l.beendet, least(l.laeuft_ab, now())) as ende
           from leases l join aufgaben a on a.id = l.aufgabe_id),
     spitze as (select l1.projekt_id, max((select count(*) from l l2 where l2.projekt_id = l1.projekt_id and l2.erstellt <= l1.erstellt and l2.ende > l1.erstellt)) as max_parallel
                from l l1 group by 1)
select p.slug as projekt, coalesce(p.projektart, 'unbekannt') as projektart,
       round(coalesce(sum(e.sekunden), 0) / 3600.0, 1) as agenten_stunden,
       round(coalesce(sum(e.sekunden) filter (where e.urteil = 'annehmen'), 0) / 3600.0, 1) as einzel_agent_stunden,
       round(extract(epoch from (max(e.bewertet) - (select min(erstellt) from l where l.projekt_id = p.id))) / 3600.0, 1) as kalender_stunden,
       round(coalesce(sum(e.sekunden), 0) / nullif(extract(epoch from (max(e.bewertet) - (select min(erstellt) from l where l.projekt_id = p.id))), 0), 2) as schwarm_faktor,
       coalesce(s.max_parallel, 0) as max_parallel,
       count(distinct e.agent) as agenten, count(distinct e.mitstreiter) as mitstreiter,
       coalesce(sum(e.tokens), 0) as tokens, coalesce(sum(e.tokens) filter (where e.urteil = 'annehmen'), 0) as tokens_angenommen,
       round(100.0 * coalesce(sum(e.tokens) filter (where e.urteil = 'annehmen'), 0) / nullif(sum(e.tokens), 0)) as token_effizienz_prozent
from projekte p left join e on e.projekt = p.slug left join spitze s on s.projekt_id = p.id
group by p.id, s.max_parallel;

-- Vom Anbinden bis zum angenommenen Beitrag (Agenten, Mitstreiter)
create view analytik.trichter as
with ag as (
  select ag.id, ag.mensch_id, ag.erstellt, coalesce(ag.runner, false) as runner,
         exists (select 1 from leases l where l.agent_id = ag.id) as reserviert,
         exists (select 1 from einreichungen e where e.agent_id = ag.id) as eingereicht,
         exists (select 1 from einreichungen e join bewertungen b on b.einreichung_id = e.id where e.agent_id = ag.id and b.urteil = 'annehmen') as angenommen,
         (select count(distinct e.aufgabe_id) from einreichungen e where e.agent_id = ag.id) as aufgaben,
         (select min(l.erstellt) from leases l where l.agent_id = ag.id) as erste_reservierung
  from agenten ag
  where not exists (select 1 from projekt_rollen r where r.mensch_id = ag.mensch_id and r.rolle in ('lead', 'visionaer')))
select runner, count(*) as angebunden, count(*) filter (where reserviert) as reserviert, count(*) filter (where eingereicht) as eingereicht,
       count(*) filter (where angenommen) as angenommen, count(*) filter (where aufgaben >= 2) as wiederkehrend,
       round(extract(epoch from percentile_cont(0.5) within group (order by erste_reservierung - erstellt)) / 60) as minuten_bis_erste_reservierung_median
from ag group by runner;

-- Prüfstraße und Sicherheitssignale je Tag
create view analytik.signale as
select date_trunc('day', zeit) as tag, art, count(*) as anzahl
from ereignisse
where art in ('einreichung_abgewiesen', 'pruefstrasse_abgewiesen', 'clean_room_trennung', 'reservierung_abgelaufen', 'abgebrochen',
              'meldung_eingegangen', 'mensch_gesperrt', 'projekt_gesperrt', 'zurueck_in_pool', 'pruefung_beendet', 'aufgabe_geschlossen')
group by 1, 2;

grant usage on schema analytik to analytik_leser;
grant select on all tables in schema analytik to analytik_leser;
revoke all on analytik_salz from public;
