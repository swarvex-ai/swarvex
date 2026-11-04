// Einzelne Rollenkarte (lead/roles/<ordner>/<karte>.md), beim Bauen vorgerendert.
import { error } from "@sveltejs/kit";
import { KARTEN_ORDNER, karte, karteMaengel, karten } from "$lib/server/lead";

export const prerender = true;
export const entries = () => KARTEN_ORDNER.flatMap((o) => karten(o).map((k) => ({ ordner: o, karte: k })));
export function GET({ params }) {
  if (!KARTEN_ORDNER.includes(params.ordner) || !karten(params.ordner).includes(params.karte)) error(404);
  const maengel = karteMaengel(params.ordner, params.karte);
  if (maengel.length) throw new Error(`role card ${params.ordner}/${params.karte}.md: ${maengel.join("; ")}`);
  return new Response(karte(params.ordner, params.karte), { headers: { "content-type": "text/markdown; charset=utf-8" } });
}
