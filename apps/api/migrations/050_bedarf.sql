-- Projektübergreifende Aufgaben (Bedarf, E-39, Entscheidung neVs 2026-11-07): Ein Projekt meldet, dass es etwas aus
-- einem anderen braucht (Grundriss braucht einen Ohrensessel aus Formwerk). Lead oder Bereichs-Lead des Zielprojekts
-- nimmt an und verknüpft eine eigene Aufgabe; die wartende Aufgabe im anfragenden Projekt ist blockiert, bis jene
-- angenommen ist. Wird die verknüpfte Aufgabe geschlossen, ist der Bedarf wieder offen und die Blockade weg.
create table bedarf (
  id           bigserial primary key,
  von_projekt  bigint not null references projekte(id),
  an_projekt   bigint not null references projekte(id) check (an_projekt <> von_projekt),
  von_aufgabe  bigint references aufgaben(id),
  titel        text not null,
  text         text not null,
  von_mensch   bigint not null references menschen(id),
  zustand      text not null default 'offen' check (zustand in ('offen', 'angenommen', 'abgelehnt', 'erledigt', 'zurueckgezogen')),
  aufgabe_id   bigint references aufgaben(id),
  antwort      text,
  erstellt     timestamptz not null default now(),
  entschieden  timestamptz
);
create index bedarf_an on bedarf(an_projekt, zustand);
create index bedarf_von on bedarf(von_projekt, zustand);
