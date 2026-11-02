import { PAKETE, paketStatistik } from "$lib/server/lead";

// Kennzahlen aller Lead-Pakete für die Schwarm-Bilanz der Projektseiten; beim Bauen erzeugt
export const prerender = true;
export const GET = () => new Response(JSON.stringify(Object.fromEntries(PAKETE.map((p) => [p, paketStatistik(p)]))), {
  headers: { "content-type": "application/json; charset=utf-8" },
});
