-- Zusage mit API-Schlüssel (Entscheidung neVs 2026-11-17): Wer übrige Credits spenden will, ohne selbst Aufträge zu
-- erzeugen und hochzuladen, hinterlegt einen API-Schlüssel des Dienstes (Meshy, ElevenLabs). Beim Hinterlegen prüft
-- die Plattform Schlüssel und Guthaben; gespeichert wird er nur verschlüsselt (RSA-OAEP mit dem öffentlichen Schlüssel,
-- src/spendenschluessel.ts). Entschlüsseln kann nur das Spendenwerk (eigener Container, src/spendenwerk.ts): Es erzeugt
-- die Aufträge des Leads und hält die Zusage als harte Grenze ein. Endet die Zusage, wird der Schlüssel gelöscht.
alter table zusagen drop constraint zusagen_weg_check;
alter table zusagen add constraint zusagen_weg_check check (weg in ('website', 'runner', 'schluessel'));
alter table zusagen drop constraint zusagen_weg;
alter table zusagen add constraint zusagen_weg check (
  (weg in ('website', 'schluessel') and projekt_id is not null and credits is not null and verfaellt is not null and agent_id is null and tageslimit is null)
  or (weg = 'runner' and agent_id is not null and tageslimit is not null));
alter table zusagen
  add column schluessel bytea,                         -- verschlüsselter API-Schlüssel; null, sobald die Zusage endet
  add column schluessel_ende text check (schluessel_ende ~ '^[A-Za-z0-9_-]{0,4}$'),  -- letzte Zeichen, zum Wiedererkennen
  add column guthaben_geprueft int,                    -- Guthaben beim Dienst, als der Schlüssel geprüft wurde
  add column verbraucht int not null default 0 check (verbraucht >= 0),  -- tatsächlich verbrauchte Credits (Weg schluessel)
  add column endgrund text check (length(endgrund) <= 300);
alter table zusagen add constraint zusagen_schluessel check (weg = 'schluessel' or schluessel is null);
alter table zusagen add constraint zusagen_schluessel_offen check (weg <> 'schluessel' or zustand <> 'offen' or schluessel is not null);

-- Aufträge an eine Schlüssel-Zusage beschreiben die Erzeugung genau (Dienst-Aufruf, src/spendendienste.ts); die
-- Credits sind dann der Preis laut Preisliste, und verbraucht ist, was der Dienst wirklich abgebucht hat.
alter table spendenauftraege
  add column erzeugung jsonb,
  add column verbraucht int check (verbraucht >= 0);
create index spendenauftraege_werk on spendenauftraege(erstellt) where erzeugung is not null and zustand = 'offen';
