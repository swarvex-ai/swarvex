-- Lebenszeichen während einer Runde des Lead-Dienstes (Entscheidung neVs 2026-11-20): Eine Runde kann lange ohne
-- Plattform-Aufruf laufen (der Lead baut, testet, prüft lokal). Der Runner meldet deshalb alle paar Minuten, dass die
-- Runde noch läuft (report_lead_cycle_running); Mein Konto und die Projektseite zeigen dann „in einer Runde seit …“
-- statt „nicht erreichbar“. Die Meldung der beendeten Runde (report_lead_cycle) setzt das Feld zurück.
alter table projekte add column lead_runde_seit timestamptz;
