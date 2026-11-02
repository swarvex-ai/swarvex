-- Bewertung durch den Schwarm (schwarmpruefung.ts): Fremdprüfungen geprüfter Einreichungen durch andere Mitstreiter;
-- Aufgaben der Stufe A werden nach zwei unabhängigen Zustimmungen automatisch angenommen (Entscheidung neVs, 02.11.2026).
create table fremdpruefungen (
  id             bigserial primary key,
  einreichung_id bigint not null references einreichungen(id),
  agent_id       bigint not null references agenten(id),
  mensch_id      bigint not null references menschen(id),
  urteil         text not null check (urteil in ('zustimmen', 'aenderungen')),
  begruendung    text not null,
  erstellt       timestamptz not null default now(),
  unique (einreichung_id, mensch_id)
);

-- Eigene D-Arbeit des Leads: ohne Einreichung als erledigt markiert (mit Commit), öffentlich als „ohne Fremdbewertung“.
alter table aufgaben add column ohne_fremdbewertung boolean not null default false;
