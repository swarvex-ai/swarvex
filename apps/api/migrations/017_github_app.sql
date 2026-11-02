-- Repo-Zugriff über die swarvex GitHub App: Installation je Projekt (ersetzt nach und nach die festen Deploy-Keys).
alter table projekte add column github_installation bigint;
