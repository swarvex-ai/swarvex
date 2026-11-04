-- Besucherstatistik der Website ohne Cookies und ohne Zugriffsprotokoll (Datenschutzerklärung, Abschnitt 2):
-- gezählt werden nur Seitenaufrufe je Tag, Seite, Herkunft (Domain) und Gerätetyp. Eindeutige Besucher je Tag
-- über einen Hash aus IP-Adresse, Browserkennung und einem Tagesgeheimnis; Geheimnis und Hashes werden nach
-- Tagesende gelöscht (besucheAbschliessen), danach bleibt nur die Zahl. IP-Adressen werden nie gespeichert.
create table besuche (
  tag date not null, pfad text not null, quelle text not null, geraet text not null,
  aufrufe integer not null default 0,
  primary key (tag, pfad, quelle, geraet)
);
create table besuch_salz (tag date primary key, wert bytea not null);
create table besucher_heute (tag date not null, kennung bytea not null, primary key (tag, kennung));
create table besucher_tage (tag date primary key, besucher integer not null);
revoke all on besuch_salz, besucher_heute from public;

create view analytik.besuche as
select tag, pfad, case when pfad = '/en' or pfad like '/en/%' then 'en' else 'de' end as sprache, quelle, geraet, aufrufe from besuche;
create view analytik.besucher as
select tag, besucher from besucher_tage
union all
select tag, count(*)::integer from besucher_heute where tag not in (select tag from besucher_tage) group by tag;
grant select on analytik.besuche, analytik.besucher to analytik_leser;
