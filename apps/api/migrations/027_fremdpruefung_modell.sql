-- Fremdprüfungen bevorzugt durch ein anderes Modell als das des Autors (schwarmpruefung.ts): Ein zweites Modell findet
-- andere Fehler. Das prüfende Modell wird mit der Fremdprüfung festgehalten, wenn der Agent es nennt.
alter table fremdpruefungen add column modell text;
