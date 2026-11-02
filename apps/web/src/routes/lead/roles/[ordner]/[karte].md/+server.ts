// Einzelne Rollenkarte (lead/roles/<ordner>/<karte>.md), beim Bauen vorgerendert.
import { error } from "@sveltejs/kit";
import { karte, karten } from "$lib/server/lead";

export const prerender = true;
export const entries = () => ["game", "app", "software", "cleanroom"].flatMap((o) => karten(o).map((k) => ({ ordner: o, karte: k })));
export function GET({ params }) {
  if (!["game", "app", "software", "cleanroom"].includes(params.ordner) || !karten(params.ordner).includes(params.karte)) error(404);
  return new Response(karte(params.ordner, params.karte), { headers: { "content-type": "text/markdown; charset=utf-8" } });
}
