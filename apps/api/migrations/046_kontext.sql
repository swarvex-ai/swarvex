-- Kontextpaket je Aufgabe: Dateien, die der Agent zuerst lesen soll (Schnittstellen, der Code, den er erweitert, der Test,
-- der grün bleiben muss). Der Lead kennt sie; ein Agent müsste sie sonst erst suchen, und schwache Modelle finden sie oft nicht.
alter table aufgaben add column kontext text[] not null default '{}';
