-- Lead-Dienst (E-38, Entscheidung neVs 2026-11-06): `swarvex project lead` lässt den Lead als Dienst in einer eigenen
-- Sandbox laufen. Er meldet sich als eigenes Gerät an und bindet diese Anmeldung an genau ein Projekt: Sie sieht nur die
-- Lead-Werkzeuge dieses Projekts, entscheidet keine Freigaben und holt Git-Zugang nur für dessen Repository.
alter table agenten add column lead_projekt_id bigint references projekte(id);
create index agenten_lead_projekt on agenten(lead_projekt_id) where lead_projekt_id is not null;

-- Anwesenheit: gesehen = letzter Aufruf eines Lead-Dienstes (auch sein Warten auf Arbeit), runde = letzte beendete Runde.
-- alarm = wann der Visionär zuletzt die Nachricht „Lead nicht erreichbar“ bekam (eine je Ausfall).
alter table projekte add column lead_gesehen timestamptz, add column lead_runde timestamptz, add column lead_alarm timestamptz;

-- Runden des Lead-Dienstes: Dauer, Verbrauch und Ausgang (für den Visionär und die Bilanz, nicht öffentlich im Einzelnen).
create table lead_runden (
  id          bigserial primary key,
  projekt_id  bigint not null references projekte(id),
  agent_id    bigint not null references agenten(id),
  beginn      timestamptz not null,
  sekunden    int not null,
  tokens      bigint not null default 0,
  anlass      text not null,
  ergebnis    text not null,
  erstellt    timestamptz not null default now()
);
create index lead_runden_projekt on lead_runden(projekt_id, erstellt desc);
