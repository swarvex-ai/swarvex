-- Gescheiterte Fremdprüfungen eines Runners (E-45, Nachtrag 11.11.2026). Bis Runner 0.15.0 erfuhr die Plattform nur von
-- zurückgegebenen Aufgaben, ob ein Runner noch arbeiten kann. Eine Fremdprüfung, die scheitert, weil der Modell-Anbieter
-- den Schlüssel ablehnt (am 11.11. das Ausgabelimit eines OpenRouter-Schlüssels), blieb auf dem Rechner des Betreibers;
-- gibt es gerade nur Prüfungen und keine Aufgaben, sah der Hinweis „Ein Runner braucht dich“ nie etwas davon.
-- Ab 0.15.1 meldet der Runner jede Fremdprüfung, die ab dem Sandbox-Schritt scheitert: mit Ursache, wenn das Modell
-- nicht arbeiten konnte, ohne, wenn es lief (dann beendet sie die Folge wie eine Rückgabe in der Sache). Nur für den
-- Betreiber selbst, nichts davon ist öffentlich; der Grund ist vom Runner schon bereinigt (Schlüssel, Adressen, Ordner).
create table fremdpruefung_abbrueche (
  id              bigserial primary key,
  einreichung_id  bigint not null references einreichungen(id),
  agent_id        bigint not null references agenten(id),
  ursache         text check (ursache in ('anbieter', 'runner', 'projekt', 'plattform', 'gestoppt')),
  grund           text not null,
  erstellt        timestamptz not null default now()
);
create index fremdpruefung_abbrueche_agent on fremdpruefung_abbrueche (agent_id, erstellt desc);
