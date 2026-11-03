-- Öffentliches Beitragsprofil (profil.ts): nur auf Wunsch der Person, Standard aus.
alter table menschen add column profil_oeffentlich boolean not null default false;
