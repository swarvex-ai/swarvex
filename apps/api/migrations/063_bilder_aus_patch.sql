-- Szenenbilder aus dem Patch (12.11.2026): Ändert eine Einreichung Referenzbilder, liest der Prüfserver das alte Bild
-- aus dem Basis-Commit und das neue aus dem Patch, rechnet die Differenz aus und meldet hier, ob er jede geänderte Szene
-- liefern konnte. Erst dann darf ein fremder Runner die Einreichung gegenprüfen (bis dahin prüfte bei Referenzbildern
-- immer die Prüfstraße, weil nur sie Bilder lieferte). Die Prüfer sehen so genau das Bild, das gemergt würde, und kein
-- Runner kann Bilder schönen. „unvollstaendig“ (etwa ein Referenzbild in Git-LFS oder über 400 kB): Die Prüfstraße
-- prüft und rendert wie bisher.
alter table einreichungen add column bilder_aus_patch text check (bilder_aus_patch in ('vollstaendig', 'unvollstaendig'));
