-- Veröffentlichte Kopf-Hashes des Ereignisprotokolls (Issue #31). Die Plattform signiert täglich den
-- Kopf der Kette; ein externer Anker (eigenes öffentliches Repo) sammelt sie. Wer die Kette später neu
-- berechnet, weicht dann von den veröffentlichten Köpfen ab.
create table koepfe (
  seq    bigint primary key,
  hash   text not null,
  zeit   timestamptz not null default now(),
  jws    text not null
);
