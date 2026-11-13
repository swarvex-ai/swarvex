-- Abgelaufene OIDC-Zeilen werden alle 15 Minuten gelöscht (main.ts, oidc-aufraeumen); der Index hält das billig.
create index oidc_payloads_laeuft_ab on oidc_payloads(laeuft_ab) where laeuft_ab is not null;
