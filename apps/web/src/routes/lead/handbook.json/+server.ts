import { handbuch } from "$lib/server/lead";

// Alles, was das swarvex-Werkzeug handbook ausliefert (E-48): Vorlagen, Rollenkarten, Engine-Referenzen, Handbücher und
// je Lead-Paket die zugeschnittenen Dateien. Beim Bauen erzeugt; die API holt es intern (apps/api/src/handbuch.ts).
export const prerender = true;
export const GET = () => new Response(JSON.stringify(handbuch()), { headers: { "content-type": "application/json; charset=utf-8" } });
