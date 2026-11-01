-- Credit-Spenden (docs „Spenden“, Weg „Auftrag + Upload“): Wer bei einem Dienst Credits übrig hat, die bald verfallen,
-- sagt sie einem Projekt zu. Der Lead entscheidet über Verwendung und Reihenfolge und legt Aufträge an; der Spender
-- erzeugt sie mit seinem eigenen Konto und lädt das Ergebnis hoch. Kein Schlüssel verlässt das Konto des Spenders.
create table zusagen (
  id           bigserial primary key,
  mensch_id    bigint not null references menschen(id),
  projekt_id   bigint not null references projekte(id),
  dienst       text not null check (dienst ~ '^[a-z][a-z0-9-]{1,30}$'),
  credits      int not null check (credits between 1 and 1000000),
  verfaellt    date not null,
  notiz        text not null default '' check (length(notiz) <= 500),
  zustand      text not null default 'offen' check (zustand in ('offen', 'beendet')),
  erstellt     timestamptz not null default now()
);
create index zusagen_projekt on zusagen(projekt_id) where zustand = 'offen';

create table spendenauftraege (
  id            bigserial primary key,
  zusage_id     bigint not null references zusagen(id),
  projekt_id    bigint not null references projekte(id),
  von_mensch    bigint not null references menschen(id),          -- Lead, Visionär oder Officer
  titel         text not null check (length(titel) between 3 and 140),
  anleitung     text not null check (length(anleitung) between 10 and 8000),
  zielpfad      text not null check (zielpfad !~ '\.\.' and zielpfad !~ '^/'),
  dateitypen    text[] not null check (cardinality(dateitypen) > 0),
  max_mb        int not null default 200 check (max_mb between 1 and 500),
  credits       int not null check (credits between 1 and 1000000),
  zustand       text not null default 'offen' check (zustand in ('offen', 'hochgeladen', 'nacharbeit', 'angenommen', 'abgelehnt', 'zurueckgezogen')),
  datei         text,                                              -- Pfad im Upload-Speicher
  dateiname     text,
  sha256        text,
  bytes         bigint,
  hochgeladen   timestamptz,
  begruendung   text,
  entschieden   timestamptz,
  erstellt      timestamptz not null default now()
);
create index spendenauftraege_zusage on spendenauftraege(zusage_id);
