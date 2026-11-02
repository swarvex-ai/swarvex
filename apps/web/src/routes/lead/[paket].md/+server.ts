// Lead-Paket je Projektart als Markdown (lib/server/lead.ts), beim Bauen vorgerendert.
import { error } from "@sveltejs/kit";
import { paket, PAKETE } from "$lib/server/lead";

export const prerender = true;
export const entries = () => PAKETE.map((p) => ({ paket: p }));
export function GET({ params }) {
  if (!PAKETE.includes(params.paket)) error(404);
  return new Response(paket(params.paket), { headers: { "content-type": "text/markdown; charset=utf-8" } });
}
