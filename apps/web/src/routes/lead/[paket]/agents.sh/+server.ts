// Skript, das die Rollenkarten eines Pakets als Claude-Code-Unteragenten einrichtet (lib/server/lead.ts).
import { error } from "@sveltejs/kit";
import { agentenSkript, PAKETE } from "$lib/server/lead";

export const prerender = true;
export const entries = () => PAKETE.map((p) => ({ paket: p }));
export function GET({ params }) {
  if (!PAKETE.includes(params.paket)) error(404);
  return new Response(agentenSkript(params.paket), { headers: { "content-type": "text/x-shellscript; charset=utf-8" } });
}
