-- Projekte aus dem Assistenten haben oft noch kein Repo; die Adresse kommt später (der Lead legt es mit dem Visionär an).
alter table projekte alter column repo_url drop not null;
