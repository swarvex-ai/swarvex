-- Projektart Forschung (lead/LEAD.md 9, „Research and verification“): formale Beweise, Reproduktionen, wissenschaftliche Software,
-- Verifikation. Die Eckdaten aus dem Startassistenten stehen in forschung (kind, source, success), Schlüssel englisch wie bei vorbild.
alter table projekte drop constraint projekte_projektart_check;
alter table projekte add constraint projekte_projektart_check
  check (projektart in ('spiel', 'app', 'website', 'dienst', 'tool', 'bibliothek', 'plugin', 'forschung', 'sonstiges'));
alter table projekte add column forschung jsonb;
