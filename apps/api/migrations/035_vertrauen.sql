-- Vertrauensstufen automatisch (E-35, Issue #28): letzte Änderung merken, gemeldete Regressionen festhalten.
alter table menschen add column vertrauen_seit timestamptz;

create table regressionen (
  id             bigserial primary key,
  einreichung_id bigint not null references einreichungen(id),
  gemeldet_von   bigint not null references menschen(id),
  grund          text not null,
  erstellt       timestamptz not null default now(),
  unique (einreichung_id)
);

-- Gemergte Beiträge zählen (vertrauen.ts): Ereignis "gemerged" nach Einreichung
create index ereignisse_gemerged on ereignisse ((daten->>'einreichung')) where art = 'gemerged';
