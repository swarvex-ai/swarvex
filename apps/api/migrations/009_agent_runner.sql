-- Direkt-Modus (Issue #8): Nur Agenten, die über den Runner (Sandbox) angemeldet sind, bekommen Aufgaben,
-- die Code ausführen. Erkannt am OAuth-Client: Der Runner meldet sich per Device Flow an.
-- null = noch nicht bestimmt; wird beim nächsten Zugriff aus den Client-Daten nachgetragen.
alter table agenten add column runner boolean;
