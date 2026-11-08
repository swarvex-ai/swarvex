-- Der Asset-Katalog vor jeder Generierung (Entscheidung neVs 2026-11-08, src/katalog.ts): Spendenaufträge und Anfragen
-- an Formwerk merken sich, welche Katalog-Assets beim Anlegen passten und warum keines genommen wurde.
-- katalog_treffer: null = ungeprüft (Katalog noch nicht veröffentlicht), leer = geprüft ohne Treffer, sonst „art:id“.
alter table spendenauftraege add column katalog_treffer text[], add column katalog_begruendung text;
alter table bedarf add column katalog_treffer text[], add column katalog_begruendung text;

-- Für die Auswertung (Grafana, Bereich Betrieb): wie oft trotz passender Katalog-Assets generiert oder angefragt
-- wurde. Ohne Texte und ohne Menschen, nur Zählungen je Tag und Projekt.
create view analytik.katalog_pruefung as
select date_trunc('day', a.erstellt) as tag, 'spendenauftrag'::text as art, p.slug as projekt, coalesce(p.projektart, 'unbekannt') as projektart, p.engine,
       a.katalog_treffer is not null as geprueft, coalesce(cardinality(a.katalog_treffer), 0) > 0 as trotz_treffer
from spendenauftraege a join projekte p on p.id = a.projekt_id
union all
select date_trunc('day', b.erstellt), 'anfrage_formwerk', p.slug, coalesce(p.projektart, 'unbekannt'), p.engine,
       b.katalog_treffer is not null, coalesce(cardinality(b.katalog_treffer), 0) > 0
from bedarf b join projekte p on p.id = b.von_projekt join projekte z on z.id = b.an_projekt
where z.slug = 'formwerk';

grant select on analytik.katalog_pruefung to analytik_leser;
