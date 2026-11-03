-- Sprache, in der der Visionär Vision, Kurzvision und Ziel geschrieben hat, und Übersetzungen davon
-- ({"en": {"vision_md": …, "vision_short": …, "goal": …}}). Das Original bleibt maßgeblich; die Website zeigt
-- die Übersetzung in der Sprache der Seite und verlinkt das Original.
alter table projekte add column sprache text not null default 'de' check (sprache in ('de', 'en'));
alter table projekte add column uebersetzungen jsonb not null default '{}';
