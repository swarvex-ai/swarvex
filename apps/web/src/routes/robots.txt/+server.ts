import { env } from "$env/dynamic/private";
import { SEITE } from "$lib/seo";

// Live (INDEXIEREN=ja): alles erlaubt, auch KI-Suchen und Training; Projekte sind ohnehin Open Source.
// Labor: nichts indexieren. Gleicher Code, Unterschied nur in der Konfiguration (Labor = Live).
export const GET = () => {
  const offen = env.INDEXIEREN === "ja";
  const text = offen ? `User-agent: *\nAllow: /\n\nSitemap: ${SEITE}/sitemap.xml\n` : "User-agent: *\nDisallow: /\n";
  return new Response(text, { headers: { "content-type": "text/plain; charset=utf-8" } });
};
