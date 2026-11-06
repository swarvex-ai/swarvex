-- Gegenprüfung für alle Projekte: Runner ab 0.11 prüfen auch Einreichungen in Projekten ohne proprietäre Engine gegen
-- (eigener Lauf des Einreichers + Lauf eines anderen Menschen statt der Prüfstraße; die Prüfstraße entscheidet bei
-- Widerspruch oder wenn sich niemand findet). Wann ein Runner zuletzt danach gefragt hat: Nur wenn gerade einer da ist,
-- wartet eine Einreichung auf ihn, sonst prüft die Prüfstraße sofort wie bisher.
alter table agenten add column gegenpruefer_gesehen timestamptz;

-- Der eigene Lauf des Einreichers steht jetzt in allen Projekten als Prüfung „selbst“ an der Einreichung. Er ist keine
-- Prüfung durch Prüfstraße oder Schwarm und zählt deshalb nicht in der Sicherheitsübersicht mit.
create or replace view analytik.sicherheit_pruefungen as
select date_trunc('day', erstellt) as tag,
       count(*) as pruefungen,
       count(*) filter (where ergebnis = 'fehler') as fehlgeschlagen,
       count(*) filter (where bericht like '%changes a protected file%') as zweite_schranke
from pruefungen where quelle <> 'selbst' group by 1;
