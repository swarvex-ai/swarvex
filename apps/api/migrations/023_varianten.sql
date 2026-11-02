-- Varianten als eine Gruppe (create_task variants): wird eine angenommen, schließen sich die noch nicht begonnenen anderen.
alter table aufgaben add column variante_von bigint references aufgaben(id);
