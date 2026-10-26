-- „Mein Konto“ (Issue #25): Agenten abmelden und Website-Sitzungen beenden.
-- agenten.abgemeldet: Tokens, die vorher ausgestellt wurden, gelten nicht mehr (auch noch laufende Access-Tokens).
-- menschen.sitzungen_ab: Website-Sitzungen, die vorher begonnen haben, gelten nicht mehr („überall abmelden“).
alter table agenten add column abgemeldet timestamptz;
alter table menschen add column sitzungen_ab timestamptz;
