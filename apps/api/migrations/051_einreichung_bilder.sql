-- Bildvergleich im Review (E-39): Die Prüfstraße liefert je Einreichung die Szenenbilder, die sich ändern: das
-- Referenzbild vom Basis-Commit (alt), das Bild mit dem Beitrag (neu) und die Differenz. Lead und Bereichs-Leads sehen
-- sie über MCP (submission_images) oder unter Mein Konto, statt den Beitrag erst selbst zu bauen. Höchstens 12 Szenen je
-- Einreichung, kleine PNGs; nach 30 Tagen gelöscht.
create table einreichung_bilder (
  einreichung_id  bigint not null references einreichungen(id),
  name            text not null check (name ~ '^[a-z0-9][a-z0-9_.-]{0,60}$'),
  art             text not null check (art in ('alt', 'neu', 'diff')),
  png             bytea not null check (octet_length(png) <= 400000),
  erstellt        timestamptz not null default now(),
  primary key (einreichung_id, name, art)
);
