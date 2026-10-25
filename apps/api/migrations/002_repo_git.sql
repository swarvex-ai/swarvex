-- Git-Adresse, über die die Plattform (Deploy-Key, nur lesend) Snapshots für Runner erzeugt.
alter table projekte add column repo_git text;
