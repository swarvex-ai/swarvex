-- Projektseiten schneller (11.11.2026). Gemessen auf live: /api/projects/<slug> 1,2–1,3 s, /api/projects 0,4 s und mit
-- jedem Projekt mehr. Zwei Ursachen liegen in der Datenbank:
--
-- 1. JIT: Der Planer schätzt die rekursive Abfrage der Folgeaufgaben (aufgaben.ts, folgeaufgaben) um Größenordnungen zu
--    teuer (Kosten 2·10⁸ bei 13 Zeilen) und übersetzt sie deshalb bei jedem Aufruf in Maschinencode: 743 ms Übersetzung,
--    8 ms Ausführung. Für viele kleine Abfragen lohnt JIT nie; aus für diese Datenbank (gilt für neue Verbindungen).
do $$ begin execute format('alter database %I set jit = off', current_database()); end $$;

-- 2. analytik.lead_bilanz las für die letzten 30 Tage alle Spalten von analytik.einreichungen, auch „zeilen“, die jeden
--    Patch mit einem regulären Ausdruck in Zeilen zerlegt (Referenzbilder als Binärdaten eingeschlossen): rund 190 ms je
--    Projekt, und die Projektliste fragt je Projekt. Gebraucht werden nur Projekt, Rolle, Urteil und Reaktionszeit; mit
--    benannten Spalten berechnet Postgres den Rest nicht. Ergebnis und Spalten bleiben gleich.
create or replace view analytik.lead_bilanz as
with e as (select projekt, rolle, urteil, lead_reaktion_s from analytik.einreichungen where zeit > now() - interval '30 days'),
     a as (select projekt, projektart, rolle, zustand, erste_reservierung, erstellt, reservierungen_ohne_ergebnis from analytik.aufgaben)
select a.projekt, a.projektart, a.rolle,
       count(*) filter (where a.zustand = 'offen') as offen,
       count(*) filter (where a.zustand = 'offen' and a.erste_reservierung is null and a.erstellt < now() - interval '3 days') as unberuehrt,
       count(*) filter (where a.zustand = 'geprueft_ok') as warten_auf_bewertung,
       coalesce(sum(a.reservierungen_ohne_ergebnis) filter (where a.erstellt > now() - interval '30 days'), 0) as reservierungen_ohne_ergebnis,
       (select count(*) from e where e.projekt = a.projekt and e.rolle = a.rolle) as einreichungen_30t,
       (select round(100.0 * count(*) filter (where urteil = 'annehmen') / nullif(count(*) filter (where urteil is not null), 0)) from e where e.projekt = a.projekt and e.rolle = a.rolle) as annahme_prozent,
       (select round(100.0 * count(*) filter (where urteil = 'nacharbeit') / nullif(count(*) filter (where urteil is not null), 0)) from e where e.projekt = a.projekt and e.rolle = a.rolle) as nacharbeit_prozent,
       (select round((percentile_cont(0.5) within group (order by lead_reaktion_s) / 3600)::numeric, 1) from e where e.projekt = a.projekt and e.rolle = a.rolle) as antwort_h_median
from a where a.zustand <> 'geschlossen'
group by a.projekt, a.projektart, a.rolle;
