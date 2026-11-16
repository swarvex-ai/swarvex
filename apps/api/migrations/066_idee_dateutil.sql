-- Der Start-Assistent verwarf eine übernommene Projektidee, wenn die Anmeldung ohne ?idea= nach /starten zurückführte,
-- behielt aber die Felder, die sie vorbelegt hatte. So entstand dateutil-absichern (15.11.2026) aus der gleichnamigen
-- Idee, aber ohne Verbindung zu ihr, und die Projektliste bot die Idee weiter mit „Als Lead übernehmen“ an.
-- Name und Kurzvision des Projekts sind der Text der Idee. Der Assistent ist korrigiert; hier bekommt das Projekt
-- seine Idee, sofern sie niemand anderes übernommen hat.
update projekte set idee = 'dateutil-absichern'
 where slug = 'dateutil-absichern' and idee is null
   and not exists (select 1 from projekte where idee = 'dateutil-absichern');
