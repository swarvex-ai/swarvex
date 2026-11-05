-- Zusage über den Runner: Ein Mitstreiter stellt die Medien-Werkzeuge seiner Agenten (Meshy, ElevenLabs …) über
-- den swarvex-Runner zur Verfügung. Lead und Bereichs-Leads beauftragen wie bei jeder Zusage (Spendenaufträge); der
-- Runner erzeugt in der Sandbox und lädt hoch. Statt Credits und Verfallsdatum gilt ein Tageslimit (Aufträge je Tag,
-- über alle Dienste des Agenten). Ohne Projekt gilt die Zusage für jedes Projekt (Runner auf „auto“).
alter table zusagen
  add column weg text not null default 'website' check (weg in ('website', 'runner')),
  add column agent_id bigint references agenten(id),
  add column tageslimit int check (tageslimit between 1 and 30),
  alter column projekt_id drop not null,
  alter column credits drop not null,
  alter column verfaellt drop not null;
alter table zusagen add constraint zusagen_weg check (
  (weg = 'website' and projekt_id is not null and credits is not null and verfaellt is not null and agent_id is null and tageslimit is null)
  or (weg = 'runner' and agent_id is not null and tageslimit is not null));
create unique index zusagen_runner_eine on zusagen(agent_id, dienst, coalesce(projekt_id, 0)) where zustand = 'offen' and weg = 'runner';

-- Der Runner hat mit einem Auftrag begonnen (Sperre gegen doppelte Arbeit; nach 3 Stunden verfallen)
alter table spendenauftraege add column begonnen timestamptz;
