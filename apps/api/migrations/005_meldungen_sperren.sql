-- Meldungen (Rechtsverletzung, Missbrauch, Sicherheit) und Sperren (Issue #19, #29).
-- Persönliche Angaben der Meldenden stehen nur hier, nie im öffentlichen Ereignisprotokoll.
create table meldungen (
  id            bigserial primary key,
  art           text not null check (art in ('rechtsverletzung', 'missbrauch', 'sicherheit', 'sonstiges')),
  projekt_id    bigint references projekte(id),
  bezug         text not null default '',      -- Adresse, Aufgabe, Commit oder Datei
  beschreibung  text not null,
  name          text not null default '',
  email         text not null,
  rechteinhaber boolean not null default false, -- Inhaber oder bevollmächtigt (bei Rechtsverletzungen)
  zustand       text not null default 'offen' check (zustand in ('offen', 'in_pruefung', 'erledigt', 'abgelehnt')),
  ergebnis      text not null default '',
  erstellt      timestamptz not null default now(),
  erledigt      timestamptz
);

alter table menschen add column sperrgrund text;
alter table projekte add column gesperrt boolean not null default false;
alter table projekte add column sperrgrund text;
