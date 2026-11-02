-- Schwarm-Bilanz (bilanz.ts): Schätzung des Leads, wie lange ein Mensch für eine Aufgabe bräuchte (für den Vergleich
-- mit einem menschlichen Team), und ein gemessener Vergleichswert je Projekt (z. B. Rend&Run: ein Agent allein, 24 Tage).
alter table aufgaben add column menschen_stunden numeric check (menschen_stunden > 0 and menschen_stunden <= 2000);
alter table projekte add column vergleich jsonb;
