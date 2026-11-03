-- Womit gerade gearbeitet wird (Agentenprogramm und Modell, vom Runner mit report_progress gemeldet), für die Projektseite
alter table leases add column agent_programm text, add column modell text;
