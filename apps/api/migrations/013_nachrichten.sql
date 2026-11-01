-- Nachrichtenweg zwischen Lead, Visionär und Officers eines Projekts (lead/OFFICER.md, LEAD.md 2.1).
-- Arten: frage (braucht eine Antwort), freigabe (braucht ein Ja oder Nein des Visionärs), info (keine Antwort nötig).
-- Nachrichten sind projektintern; Freigaben und ihre Entscheidung stehen mit Betreff im öffentlichen Protokoll.
create table nachrichten (
  id              bigserial primary key,
  projekt_id      bigint not null references projekte(id),
  von_mensch      bigint not null references menschen(id),
  von_agent       bigint references agenten(id),
  an              text not null check (an ~ '^(lead|visionaer|bereich:[a-z][a-z0-9-]{1,30})$'),
  art             text not null check (art in ('frage', 'freigabe', 'info')),
  betreff         text not null check (length(betreff) between 3 and 140),
  inhalt          text not null check (length(inhalt) between 1 and 8000),
  aufgaben        bigint[] not null default '{}',
  zustand         text not null default 'offen' check (zustand in ('offen', 'beantwortet', 'erledigt')),
  antwort         text check (length(antwort) <= 8000),
  entscheidung    text check (entscheidung in ('freigegeben', 'abgelehnt')),
  beantwortet_von bigint references menschen(id),
  beantwortet_am  timestamptz,
  erstellt        timestamptz not null default now(),
  check (art = 'freigabe' or entscheidung is null),
  check (art <> 'freigabe' or an = 'visionaer')
);
create index nachrichten_offen on nachrichten(projekt_id, an) where zustand = 'offen';

-- Lead-Bilanz: offene Nachrichten an den Lead und seine Antwortzeit (ohne Inhalte)
create view analytik.nachrichten as
select p.slug as projekt, n.an, n.art, n.zustand, n.erstellt, n.beantwortet_am,
       extract(epoch from n.beantwortet_am - n.erstellt) as antwort_s
from nachrichten n join projekte p on p.id = n.projekt_id;
grant select on analytik.nachrichten to analytik_leser;
