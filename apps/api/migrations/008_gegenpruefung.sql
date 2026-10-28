-- Proprietäre Engines (Unity, Unreal; E-31): Die Prüfstraße kann sie nicht ausführen. Stattdessen zählt
-- der Testlauf des einreichenden Runners („selbst“), und der Runner eines anderen Menschen mit derselben
-- Engine prüft gegen („gegenpruefung“): Basis-Commit + Patch, Tests ohne Netz, ohne Modell.
alter table pruefungen drop constraint pruefungen_quelle_check;
alter table pruefungen add constraint pruefungen_quelle_check check (quelle in ('projekt', 'stichprobe', 'selbst', 'gegenpruefung'));

create table gegenpruefungen (
  id             bigserial primary key,
  einreichung_id bigint not null references einreichungen(id),
  agent_id       bigint not null references agenten(id),
  laeuft_ab      timestamptz not null,
  beendet        timestamptz,
  ergebnis       text check (ergebnis in ('ok', 'fehler', 'nicht_moeglich')),
  erstellt       timestamptz not null default now()
);
create index gegenpruefungen_offen on gegenpruefungen(einreichung_id) where beendet is null;
