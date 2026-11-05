-- Stand von main je Projekt, gemeldet von der Prüfstraße (pipeline/vorschau.go, alle fünf Minuten). Beim Reservieren
-- wird er zur Basis der Aufgabe: Jeder Versuch arbeitet auf dem neuesten Stand, mit Fixes und angenommenen Vorgängern.
alter table projekte add column kopf_commit text check (kopf_commit ~ '^[0-9a-f]{40}$');
update projekte set kopf_commit = vorschau->>'commit' where vorschau->>'commit' ~ '^[0-9a-f]{40}$';
