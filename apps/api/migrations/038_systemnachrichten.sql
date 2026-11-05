-- Nachrichten der Plattform selbst (ohne Absender-Menschen): z. B. die Prüfstraße meldet dem Lead, dass die Tests
-- des Projekts schon ohne einen Beitrag rot sind (Gegenprobe). Absender ist dann „swarvex“.
alter table nachrichten alter column von_mensch drop not null;

-- Moderations-Signale (Grafana „Leads und Moderation“): rote Projekt-Tests mitzählen
create or replace view analytik.signale as
select date_trunc('day', zeit) as tag, art, count(*) as anzahl
from ereignisse
where art in ('einreichung_abgewiesen', 'pruefstrasse_abgewiesen', 'clean_room_trennung', 'reservierung_abgelaufen', 'abgebrochen',
              'meldung_eingegangen', 'mensch_gesperrt', 'projekt_gesperrt', 'zurueck_in_pool', 'pruefung_beendet', 'aufgabe_geschlossen',
              'projekt_tests_rot')
group by 1, 2;
