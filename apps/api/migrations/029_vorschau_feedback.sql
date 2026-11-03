-- Rückmeldungen zur spielbaren Vorschau (feedback.ts): angemeldete Besucher sagen, was gut war und was hakt; Lead und
-- Officers machen daraus Aufgaben. Der Inhalt bleibt projektintern, im Protokoll steht nur, dass es eine Rückmeldung gab.
create table vorschau_feedback (
  id          bigserial primary key,
  projekt_id  bigint not null references projekte(id),
  mensch_id   bigint not null references menschen(id),
  commit      text check (length(commit) <= 64),
  gut         text not null default '' check (length(gut) <= 2000),
  hakt        text not null default '' check (length(hakt) <= 2000),
  geraet      text check (length(geraet) <= 120),
  zustand     text not null default 'neu' check (zustand in ('neu', 'gesehen', 'aufgabe')),
  aufgabe_id  bigint references aufgaben(id),
  erstellt    timestamptz not null default now(),
  check (length(gut) + length(hakt) > 0)
);
create index vorschau_feedback_offen on vorschau_feedback(projekt_id, zustand);
