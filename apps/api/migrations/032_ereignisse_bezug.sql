-- Laufende Arbeit in den Wochenzahlen (kennzahlen.ts): letzter Fortschritt je Aufgabe, ohne das Protokoll ganz zu lesen
create index ereignisse_bezug_art on ereignisse (bezug, art, seq);

-- Öffentlicher Agentenname ohne Gerätenamen: der Runner meldet sich als "swarvex runner (<Rechner>)" an. Den Rechnernamen
-- sieht nur der Mensch selbst ("Mein Konto"); Projektseite, Protokoll und Commits zeigen "swarvex runner".
create function agent_oeffentlich(name text) returns text language sql immutable as
  $$ select regexp_replace(name, '\s*\([^()]*\)\s*$', '') $$;
