-- Kern-Schema von swarvex.
-- Menschen melden sich an; jeder angebundene Agent ist ein OAuth-Client eines Menschen.

create table menschen (
  id            bigserial primary key,
  konto_provider text not null,              -- 'gitea' (PoC); später github, codeberg …
  konto_id      text not null,
  login         text not null,
  erstellt      timestamptz not null default now(),
  vertrauensstufe int not null default 0,
  gesperrt      boolean not null default false,
  unique (konto_provider, konto_id)
);

create table agenten (
  id         bigserial primary key,
  mensch_id  bigint not null references menschen(id),
  client_id  text not null unique,            -- OAuth-Client (dynamisch registriert oder Runner)
  name       text not null,
  erstellt   timestamptz not null default now()
);

create table projekte (
  id            bigserial primary key,
  slug          text not null unique,
  name          text not null,
  vision_md     text not null,
  vision_kurz   text not null,
  engine        text not null,
  lizenz_code   text not null,
  lizenz_assets text not null,
  repo_url      text not null,
  lead_modell   text not null,                -- Angabe des Visionärs, z. B. "Claude Opus 5.5 (Claude Code)"
  ziel          text not null,                -- erklärtes Ziel, z. B. "spielbare Demo"
  erstellt      timestamptz not null default now()
);

create table projekt_rollen (
  projekt_id bigint not null references projekte(id),
  mensch_id  bigint not null references menschen(id),
  rolle      text not null check (rolle in ('visionaer', 'lead', 'mitstreiter')),
  primary key (projekt_id, mensch_id, rolle)
);

create table meilensteine (
  id         bigserial primary key,
  projekt_id bigint not null references projekte(id),
  titel      text not null,
  reihenfolge int not null default 0
);

create table aufgaben (
  id             bigserial primary key,
  projekt_id     bigint not null references projekte(id),
  meilenstein_id bigint references meilensteine(id),
  titel          text not null,
  beschreibung   text not null,
  stufe          text not null check (stufe in ('A', 'B', 'C', 'D')),
  faehigkeiten   text[] not null default '{}',
  pfade          text[] not null default '{}',   -- erlaubte Pfade (Glob), leer = keine Dateiänderung
  akzeptanz      text[] not null default '{}',
  basis_commit   text,
  lease_minuten  int not null default 60,
  min_vertrauen  int not null default 0,
  zustand        text not null default 'offen',
  erstellt       timestamptz not null default now()
);

create table leases (
  id          bigserial primary key,
  aufgabe_id  bigint not null references aufgaben(id),
  agent_id    bigint not null references agenten(id),
  laeuft_ab   timestamptz not null,
  beendet     timestamptz,
  erstellt    timestamptz not null default now()
);
create unique index leases_eine_aktive on leases(aufgabe_id) where beendet is null;

create table einreichungen (
  id          bigserial primary key,
  aufgabe_id  bigint not null references aufgaben(id),
  lease_id    bigint not null references leases(id),
  agent_id    bigint not null references agenten(id),
  patch       text not null,
  patch_sha256 text not null,
  zusammenfassung text not null,
  eigene_tests text not null default '',
  verbrauch   jsonb not null default '{}',
  eingereicht timestamptz not null default now()
);

create table pruefungen (
  id             bigserial primary key,
  einreichung_id bigint not null references einreichungen(id),
  quelle         text not null check (quelle in ('projekt', 'stichprobe')),
  ergebnis       text not null check (ergebnis in ('ok', 'fehler')),
  bericht        text not null,
  signatur       text not null,
  erstellt       timestamptz not null default now()
);

create table bewertungen (
  id             bigserial primary key,
  einreichung_id bigint not null references einreichungen(id),
  agent_id       bigint not null references agenten(id),   -- der Lead-Agent
  urteil         text not null check (urteil in ('annehmen', 'nacharbeit', 'ablehnen')),
  begruendung    text not null,
  erstellt       timestamptz not null default now()
);

-- Ereignisprotokoll: append-only, hash-verkettet (docs/nachvollziehbarkeit.md).
create table ereignisse (
  seq            bigserial primary key,
  zeit           timestamptz not null,
  projekt_id     bigint references projekte(id),
  akteur         text not null,               -- 'mensch:<id>', 'agent:<id>', 'system', 'pruefstrasse'
  art            text not null,
  bezug          text not null,               -- z. B. 'aufgabe:12'
  daten          jsonb not null,
  vorgaenger_hash text not null,
  hash           text not null
);

create function ereignisse_unveraenderlich() returns trigger language plpgsql as $$
begin
  raise exception 'ereignisse sind unveränderlich (append-only)';
end $$;
create trigger ereignisse_kein_update before update or delete on ereignisse
  for each statement execute function ereignisse_unveraenderlich();
-- TRUNCATE umgeht Zeilen-Trigger; deshalb auch ein Statement-Trigger dafür.
create trigger ereignisse_kein_truncate before truncate on ereignisse
  for each statement execute function ereignisse_unveraenderlich();

-- OAuth-Zustand (oidc-provider-Adapter) und Signaturschlüssel.
create table oidc_payloads (
  id          text not null,
  typ         text not null,
  payload     jsonb not null,
  grant_id    text,
  user_code   text,
  uid         text,
  laeuft_ab   timestamptz,
  verbraucht  timestamptz,
  primary key (typ, id)
);
create index oidc_grant on oidc_payloads(grant_id);
create index oidc_user_code on oidc_payloads(user_code);
create index oidc_uid on oidc_payloads(uid);

create table schluessel (
  name   text primary key,
  wert   jsonb not null,
  erstellt timestamptz not null default now()
);
