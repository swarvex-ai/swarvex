-- Keine geschätzten Menschenstunden mehr (Entscheidung neVs, 02.11.2026): öffentlich zählt nur, was gemessen oder im Repo
-- belegt ist. Der Vergleich mit einem Solo-Bau läuft über Aufgaben-Einheiten (Stufen-Gewichte, bilanz.ts).
alter table aufgaben drop column if exists menschen_stunden;
