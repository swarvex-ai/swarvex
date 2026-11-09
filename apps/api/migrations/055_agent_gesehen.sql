-- Wann eine Anmeldung (Runner, Lead-Dienst, direkt angebundener Agent) zuletzt da war: jeder Aufruf mit gültigem Token
-- (auth/zugang.ts, höchstens einmal in der Minute). „Zuletzt aktiv“ auf der Konto-Seite zählte bisher nur Reservierungen
-- und Einreichungen, Lead-Dienste und Lead-Sitzungen standen dort immer auf „noch nie“.
alter table agenten add column gesehen timestamptz;

-- Nachtragen aus dem, was es schon gibt: eigene Ereignisse, Gegenprüfungs-Abfragen, Reservierungen, Einreichungen
-- und für Lead-Dienste die Anwesenheit ihres Projekts.
update agenten a set gesehen = greatest(
  (select max(e.zeit) from ereignisse e where e.akteur = 'agent:' || a.id),
  a.gegenpruefer_gesehen,
  (select max(l.erstellt) from leases l where l.agent_id = a.id),
  (select max(x.eingereicht) from einreichungen x where x.agent_id = a.id),
  (select p.lead_gesehen from projekte p where p.id = a.lead_projekt_id));
