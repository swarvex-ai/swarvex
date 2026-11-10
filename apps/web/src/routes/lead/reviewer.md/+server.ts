// Prüfer-Handbuch (lead/REVIEWER.md) als Markdown: Der Runner legt es seinem Prüfmodell bei Fremdbewertungen vor
// (runner/fremdbewertung.go). Beim Bauen vorgerendert, eine Quelle: lead/ im Repo.
import { prueferHandbuch } from "$lib/server/lead";

export const prerender = true;
export function GET() {
  return new Response(prueferHandbuch(), { headers: { "content-type": "text/markdown; charset=utf-8" } });
}
