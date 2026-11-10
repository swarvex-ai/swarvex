-- Fremdbewertung durch den Runner (neVs 10.11.2026): Damit die eigene Arbeit eines Leads nicht liegen bleibt, bewerten
-- die Runner anderer Mitstreiter mit. Vier Leitplanken halten die Qualität: ein anderer Mensch und eine andere
-- Modellfamilie als die Einreichung, zustimmen bei sichtbaren Änderungen nur nach Blick auf die Szenenbilder, Prüfpunkte
-- des Leads (Fragen, kein Urteil), die der Prüfer einzeln beantwortet, und eine Messung, welche Zustimmungen sich später
-- als falsch erweisen (Regression gemeldet oder eine Aufgabe behebt die angenommene).
alter table einreichungen add column pruefpunkte text[];
alter table fremdpruefungen add column antworten text[], add column bilder_gesehen boolean not null default false;
alter table aufgaben add column behebt bigint references aufgaben(id);
create index aufgaben_behebt on aufgaben(behebt) where behebt is not null;
