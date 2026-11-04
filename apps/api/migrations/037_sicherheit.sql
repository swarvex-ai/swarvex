-- Sicherheits-Dashboard (Grafana, Bereich Betrieb): nur Zahlen, keine Namen, keine Adressen.

-- Welches Agent-Programm in welcher Version sich verbindet (MCP initialize → clientInfo), je Agent der letzte Stand
alter table agenten add column programm text, add column programm_version text, add column programm_seit timestamptz;

-- Sicherheitsrelevante Ereignisse je Tag
create view analytik.sicherheit_ereignisse as
select date_trunc('day', zeit) as tag,
       case art
         when 'mensch_registriert' then 'neue Menschen'
         when 'anmeldung_abgewiesen' then 'Anmeldung abgewiesen (Konto zu neu)'
         when 'anmeldung_ausnahme' then 'Anmeldung mit Ausnahme'
         when 'agent_angebunden' then 'Agent angebunden'
         when 'agent_abgemeldet' then 'Agent abgemeldet'
         when 'pruefstrasse_abgewiesen' then 'Prüfstraße: falsche Signatur'
         when 'meldung_eingegangen' then 'Meldung eingegangen'
         when 'mensch_gesperrt' then 'Mensch gesperrt'
         when 'projekt_gesperrt' then 'Projekt gesperrt'
         when 'regression_gemeldet' then 'Regression gemeldet'
         when 'clean_room_trennung' then 'Clean Room: Trennwand gegriffen'
         when 'officer_ernannt' then 'Bereichs-Lead ernannt'
         when 'officer_beendet' then 'Bereichs-Lead beendet'
         when 'vertrauen_geaendert' then case when (daten->>'nach')::int > (daten->>'von')::int then 'Vertrauen gestiegen' else 'Vertrauen gesunken' end
       end as art,
       count(*) as anzahl
from ereignisse
where art in ('mensch_registriert', 'anmeldung_abgewiesen', 'anmeldung_ausnahme', 'agent_angebunden', 'agent_abgemeldet', 'pruefstrasse_abgewiesen',
              'meldung_eingegangen', 'mensch_gesperrt', 'projekt_gesperrt', 'regression_gemeldet', 'clean_room_trennung', 'officer_ernannt', 'officer_beendet',
              'vertrauen_geaendert')
group by 1, 2;

-- Abgewiesene Einreichungen nach Grund (die erste Schranke: Pfadprüfung der Plattform)
create view analytik.sicherheit_abweisungen as
select date_trunc('day', x.zeit) as tag,
       case when f like 'protected file%' then 'geschützte Datei'
            when f like 'outside the allowed paths%' then 'außerhalb der erlaubten Pfade'
            when f like 'invalid path%' then 'ungültiger Pfad'
            when f like 'git cannot read%' then 'für git unlesbar'
            when f like 'too many%' then 'zu groß'
            else 'anderes' end as grund,
       count(*) as anzahl
from ereignisse x, jsonb_array_elements_text(x.daten->'fehler') f
where x.art = 'einreichung_abgewiesen'
group by 1, 2;

-- Prüfstraße je Tag: Prüfungen, Fehlschläge und die zweite Schranke (geschützte Datei erst nach dem Anwenden entdeckt)
create view analytik.sicherheit_pruefungen as
select date_trunc('day', erstellt) as tag,
       count(*) as pruefungen,
       count(*) filter (where ergebnis = 'fehler') as fehlgeschlagen,
       count(*) filter (where bericht like '%changes a protected file%') as zweite_schranke
from pruefungen group by 1;

-- Prüfschlange jetzt: wartende Einreichungen und die älteste
create view analytik.pruefschlange as
select count(*) filter (where a.zustand = 'eingereicht') as wartend,
       count(*) filter (where a.zustand = 'pruefung') as laufend,
       coalesce(round((extract(epoch from now() - min(e.eingereicht) filter (where a.zustand = 'eingereicht')) / 3600)::numeric, 1), 0) as aelteste_h
from aufgaben a join einreichungen e on e.id = (select max(id) from einreichungen where aufgabe_id = a.id)
where a.zustand in ('eingereicht', 'pruefung');

-- Manipulationsschutz des Protokolls: wie alt der letzte signierte Kopf-Hash ist (täglich 03:17 UTC)
create view analytik.sicherheit_kopf as
select max(seq) as seq, max(zeit) as zeit, round((extract(epoch from now() - max(zeit)) / 3600)::numeric, 1) as alter_h from koepfe;

-- OAuth-Clients je Tag nach Art: Runner (Geräte-Anmeldung), lokaler Agent (Loopback), Web-Client (Rücksprung auf eine Website)
create view analytik.oauth_clients as
select date_trunc('day', to_timestamp((payload->>'client_id_issued_at')::bigint)) as tag,
       case when payload->'grant_types' ? 'urn:ietf:params:oauth:grant-type:device_code' then 'Runner (Gerät)'
            when payload->>'redirect_uris' ~ 'https://' then 'Web-Client'
            else 'lokaler Agent' end as art,
       count(*) as anzahl
from oidc_payloads where typ = 'Client' and payload ? 'client_id_issued_at'
group by 1, 2;

-- Stand jetzt: Sitzungen, Grants, Sperren, offene Meldungen, Vertrauensstufen
create view analytik.sicherheit_stand as
select (select count(*) from oidc_payloads where typ = 'Session' and (laeuft_ab is null or laeuft_ab > now())) as sitzungen,
       (select count(*) from oidc_payloads where typ = 'Grant' and (laeuft_ab is null or laeuft_ab > now())) as grants,
       (select count(*) from menschen where gesperrt) as menschen_gesperrt,
       (select count(*) from projekte where gesperrt) as projekte_gesperrt,
       (select count(*) from meldungen where zustand in ('offen', 'in_pruefung')) as meldungen_offen,
       (select count(*) from meldungen where zustand in ('offen', 'in_pruefung') and art = 'sicherheit') as sicherheitsmeldungen_offen,
       (select count(*) from officers where beendet is null) as bereichs_leads;

create view analytik.vertrauen_verteilung as
select vertrauensstufe as stufe, count(*) as menschen from menschen where not gesperrt group by 1;

-- Agent-Programme und Versionen der letzten 30 Tage (ohne Namen der Menschen)
create view analytik.agent_versionen as
select coalesce(programm, 'unbekannt') as programm, coalesce(programm_version, '?') as version,
       count(*) as agenten, bool_or(runner) as runner
from agenten where abgemeldet is null and programm_seit > now() - interval '30 days'
group by 1, 2;

grant select on all tables in schema analytik to analytik_leser;
