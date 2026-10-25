-- Client-ID-Metadata-Dokumente (MCP-Spezifikation 2025-11-25): Die Client-ID ist eine URL, die alle
-- Installationen eines Agenten teilen (z. B. alle Claude-Code-Nutzer). Ein Agent ist deshalb
-- (Mensch, Client) statt nur Client.
alter table agenten drop constraint agenten_client_id_key;
alter table agenten add constraint agenten_mensch_client_key unique (mensch_id, client_id);
