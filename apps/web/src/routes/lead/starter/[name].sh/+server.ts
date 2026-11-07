// Startvorlagen je Stack (lead/templates/<name>/) als Shell-Skript: `curl -fsSL https://swarvex.ai/lead/starter/threejs.sh | sh`
// schreibt die Dateien in den aktuellen Ordner und überschreibt nie etwas. Beim Bauen vorgerendert. Der Lead-Dienst holt
// es über seinen Proxy ($SWARVEX_PROXY/lead/starter/threejs.sh mit $SWARVEX_TOKEN).
import { error } from "@sveltejs/kit";
import { starterSkript, STARTER } from "$lib/server/lead";

export const prerender = true;
export const entries = () => STARTER.map((name) => ({ name }));
export function GET({ params }) {
  if (!STARTER.includes(params.name)) error(404);
  return new Response(starterSkript(params.name), { headers: { "content-type": "text/x-shellscript; charset=utf-8" } });
}
