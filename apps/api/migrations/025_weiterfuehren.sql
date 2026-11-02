-- Verfahren continuation (lead/LEAD.md 9, „Continuing abandoned software“): ein verwaistes Open-Source-Projekt als Fork weiterführen.
-- Das Original steht in upstream (repo, status, contact), Schlüssel englisch wie bei vorbild.
alter table projekte drop constraint projekte_verfahren_check;
alter table projekte add constraint projekte_verfahren_check check (verfahren in ('standard', 'clean_room', 'continuation'));
alter table projekte add column upstream jsonb;
