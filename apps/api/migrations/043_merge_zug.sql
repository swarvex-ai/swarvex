-- Merge-Zug (pipeline/zug.go): Angenommene Einreichungen werden vor dem Push zusammen auf dem aktuellen Stand getestet.
-- Ist ein Beitrag dort rot, steht der Bericht als Prüfung „zug“ an seiner Einreichung; der nächste Versuch der Aufgabe
-- bekommt ihn wie einen roten Bericht der Prüfstraße.
alter table pruefungen drop constraint pruefungen_quelle_check;
alter table pruefungen add constraint pruefungen_quelle_check check (quelle in ('projekt', 'stichprobe', 'selbst', 'gegenpruefung', 'zug'));
