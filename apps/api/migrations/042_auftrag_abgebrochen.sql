-- Ein Runner hat einen Medien-Auftrag ohne Ergebnis zurückgegeben: der Grund für den Lead, und 24 Stunden Pause für
-- diesen Auftrag. Ein Auftrag, der immer scheitert, würde sonst bis zum Tageslimit die Credits des Spenders verbrauchen.
-- (Hat nur jemand den Runner angehalten, gibt es keine Pause.)
alter table spendenauftraege add column abgebrochen timestamptz, add column abbruchgrund text;
