-- Bereichs-Leads automatisch vorschlagen (E-35): Kandidat sagt zu, Visionär gibt frei (oder automatisch je Projekt).
create table bereichs_vorschlaege (
  id           bigserial primary key,
  projekt_id   bigint not null references projekte(id),
  bereich      text not null check (bereich ~ '^[a-z][a-z0-9-]{1,30}$'),
  mensch_id    bigint not null references menschen(id),
  pfade        text[] not null check (cardinality(pfade) > 0),
  faehigkeiten text[] not null check (cardinality(faehigkeiten) > 0),
  gruende      jsonb not null default '[]',
  zustand      text not null default 'offen' check (zustand in ('offen', 'zugesagt', 'abgelehnt', 'ernannt', 'verworfen', 'abgelaufen')),
  erstellt     timestamptz not null default now(),
  entschieden  timestamptz
);
-- Je Bereich höchstens ein laufender Vorschlag; wer einmal abgesagt hat, wird für diesen Bereich nicht erneut gefragt
create unique index bereichs_vorschlag_laufend on bereichs_vorschlaege(projekt_id, bereich) where zustand in ('offen', 'zugesagt');

-- Visionär: Bereichs-Leads ernennen, sobald die Person zusagt, ohne eigene Freigabe
alter table projekte add column bereichs_leads_automatisch boolean not null default false;
