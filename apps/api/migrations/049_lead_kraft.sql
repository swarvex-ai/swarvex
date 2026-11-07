-- Lead-Kraft (E-39, Entscheidung neVs 2026-11-07): mehr Lead für Projekte, die es brauchen, immer einer Person zugeordnet.

-- Haupt-Reservierung: Genau ein Lead-Dienst je Projekt führt die Hauptschleife (der Halter). Weitere gebundene Dienste
-- warten in Bereitschaft und übernehmen, wenn der Halter 15 Minuten schweigt. Dienste mit einem Bereich (Bereichs-Sitzung
-- des Leads oder Dienst eines Bereichs-Leads) halten ihren Bereich, nicht die Hauptschleife.
alter table projekte add column lead_halter bigint references agenten(id);

-- Lead-Gesuche: Lead oder Visionär (oder sein Lead-Dienst) sucht einen Bereichs-Lead. Bewerben kann sich jeder mit
-- eigenem Konto; der Visionär entscheidet. Je Bereich höchstens ein offenes Gesuch.
create table lead_gesuche (
  id            bigserial primary key,
  projekt_id    bigint not null references projekte(id),
  bereich       text not null check (bereich ~ '^[a-z][a-z0-9-]{1,30}$'),
  pfade         text[] not null check (cardinality(pfade) > 0),
  faehigkeiten  text[] not null check (cardinality(faehigkeiten) > 0),
  raum          text check (raum in ('analyse', 'pruefung', 'bau')),
  text          text not null,
  von_mensch    bigint not null references menschen(id),
  erstellt      timestamptz not null default now(),
  geschlossen   timestamptz
);
create unique index lead_gesuche_offen on lead_gesuche(projekt_id, bereich) where geschlossen is null;

-- Bewerbungen sind Bereichs-Lead-Vorschläge, die die Person selbst anstößt (gleich „zugesagt“): Der Visionär sieht sie
-- unter Mein Konto neben den Vorschlägen der Plattform. Er nimmt sie auch ohne Vertrauensstufe 2 an, denn er wählt die
-- Person selbst aus. Mehrere Bewerbungen je Bereich sind möglich, je Person eine laufende.
alter table bereichs_vorschlaege add column gesuch_id bigint references lead_gesuche(id);
drop index bereichs_vorschlag_laufend;
create unique index bereichs_vorschlag_laufend on bereichs_vorschlaege(projekt_id, bereich) where zustand in ('offen', 'zugesagt') and gesuch_id is null;
create unique index bereichs_bewerbung_laufend on bereichs_vorschlaege(gesuch_id, mensch_id) where zustand in ('offen', 'zugesagt') and gesuch_id is not null;

-- Lead-Kraft-Hinweis: Tagesstand der drei Signale. Halten sie drei Tage an, bekommen Lead und Visionär einen Vorschlag
-- (jemanden einladen oder selbst einen zweiten Lead-Agenten anbinden), höchstens einmal in sieben Tagen.
create table lead_kraft_tage (
  projekt_id       bigint not null references projekte(id),
  tag              date not null,
  backlog_leer     boolean not null,
  wartet_auf_lead  boolean not null,
  d_stau           boolean not null,
  primary key (projekt_id, tag)
);
alter table projekte add column lead_kraft_hinweis timestamptz;

-- Projektideen „Lead gesucht“ (ideen.ts): Wer eine Idee übernimmt, legt das Projekt an und wird sein Visionär und Lead.
-- Jede Idee höchstens einmal; ein Entwurf, der 14 Tage nicht veröffentlicht wird, gibt sie wieder frei.
alter table projekte add column idee text;
create unique index projekte_idee on projekte(idee) where idee is not null;
