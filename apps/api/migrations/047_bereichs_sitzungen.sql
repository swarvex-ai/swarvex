-- Bereichs-Sitzungen (Entscheidung neVs 2026-11-06): Weitere Sitzungen von Lead oder Visionär übernehmen einen Bereich,
-- ohne Vertrauensstufe 2, denn es ist derselbe Mensch mit denselben Rechten. Die Plattform teilt nur die Arbeit auf: Die
-- Bereichs-Sitzung sieht die Warteschlange ihres Bereichs (Stufen A bis C), die Hauptsitzung den Rest (D, Architektur und
-- alles außerhalb). Meldet sich eine Bereichs-Sitzung zwei Stunden nicht, fällt ihr Bereich an die Hauptsitzung zurück.
create table bereichs_sitzungen (
  id           bigserial primary key,
  projekt_id   bigint not null references projekte(id),
  agent_id     bigint not null references agenten(id),
  bereich      text not null,
  pfade        text[] not null,
  faehigkeiten text[] not null,
  gesehen      timestamptz not null default now(),
  erstellt     timestamptz not null default now(),
  beendet      timestamptz
);
create unique index bereichs_sitzungen_bereich on bereichs_sitzungen(projekt_id, bereich) where beendet is null;
create unique index bereichs_sitzungen_agent on bereichs_sitzungen(projekt_id, agent_id) where beendet is null;

-- Bereichs-Leads dürfen Stufe C anlegen und abnehmen, wenn der Lead den Bereich dafür freigibt (Entscheidung neVs 2026-11-06)
alter table officers add column stufe_c boolean not null default false;
