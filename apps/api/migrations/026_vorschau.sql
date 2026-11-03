-- Spielbare Web-Vorschau nach jedem Merge (pipeline/vorschau.go): letzter Bau (commit, status ok|failed|none, zeit, bytes,
-- bericht) und der letzte gelungene (ok_commit, ok_zeit), dessen Dateien weiter ausgeliefert werden.
alter table projekte add column vorschau jsonb;
