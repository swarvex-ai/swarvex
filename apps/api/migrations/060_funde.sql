-- Fundmeldungen (E-46, neVs 11.11.2026): Agenten melden beim Einreichen oder Zurückgeben Folgearbeit, die ihnen
-- auffällt (ein Fehler daneben, ein fehlender Test, eine Änderung außerhalb der erlaubten Pfade), statt sie zu übergehen
-- oder Dateien außerhalb ihrer Pfade zu ändern (die verwirft der Runner). Der Lead macht daraus mit einem Schritt eine
-- Aufgabe oder verwirft die Meldung mit Grund. Meldungen kommen von fremden Agenten: Lead und Bereichs-Leads sehen sie
-- als Daten; ausgewertet wird nur in Zahlen je Modell, öffentlich steht nichts. Eine gleiche Meldung (gleicher Titel oder gleiche Pfade) zählt bei der
-- offenen mit, statt eine zweite anzulegen.
create table funde (
  id              bigserial primary key,
  projekt_id      bigint not null references projekte(id),
  aufgabe_id      bigint not null references aufgaben(id),      -- bei dieser Aufgabe fiel es auf
  einreichung_id  bigint references einreichungen(id),          -- leer: bei einer Rückgabe gemeldet
  agent_id        bigint not null references agenten(id),
  modell          text,                                         -- für die Messung je Modell
  raum            text,                                         -- Clean Room: die Aufgabe daraus bleibt in diesem Raum
  titel           text not null check (length(titel) between 5 and 120),
  text            text not null default '' check (length(text) <= 1000),
  pfade           text[] not null default '{}',
  stufe           text check (stufe in ('A', 'B', 'C', 'D')),   -- Vorschlag des Agenten
  meldungen       int not null default 1,
  zustand         text not null default 'offen' check (zustand in ('offen', 'aufgabe', 'verworfen')),
  neue_aufgabe    bigint references aufgaben(id),
  entschieden_von bigint references menschen(id),
  entschieden     timestamptz,
  grund           text check (length(grund) <= 500),
  erstellt        timestamptz not null default now(),
  check (zustand <> 'aufgabe' or neue_aufgabe is not null)
);
create index funde_offen on funde(projekt_id) where zustand = 'offen';
create index funde_agent on funde(agent_id, erstellt);

-- Für die Auswertung (Grafana): wie viele Meldungen je Modell zu Aufgaben wurden, ohne Inhalte und ohne Menschen.
create view analytik.funde as
select p.slug as projekt, coalesce(p.projektart, 'unbekannt') as projektart, p.engine, coalesce(f.modell, 'unbekannt') as modell,
       date_trunc('day', f.erstellt) as tag, f.zustand, count(*) as funde, sum(f.meldungen) as meldungen
from funde f join projekte p on p.id = f.projekt_id
group by 1, 2, 3, 4, 5, 6;
grant select on analytik.funde to analytik_leser;
