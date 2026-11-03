-- Vorschau je Einreichung (pipeline/vorschau.go): geprüfte Einreichungen werden vor der Bewertung gebaut und unter
-- review.<Vorschau-Domain>/<id>/ ausprobierbar; hier steht, ob und wann das gelang.
alter table einreichungen add column vorschau jsonb;
