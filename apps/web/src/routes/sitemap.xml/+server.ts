import { alternativen, seiten } from "$lib/server/doku";
import { SEITE, STAND } from "$lib/seo";

// Sitemap mit Sprachvarianten (hreflang) für Startseite und alle Doku-Seiten
export const prerender = true;
export const GET = () => {
  const gruppen: Record<string, string>[] = [{ de: "/", en: "/en" }, { de: "/projekte", en: "/en/projects" },
    { de: "/open-source", en: "/en/open-source" }, { de: "/melden", en: "/en/report" }, { de: "/kontakt", en: "/en/contact" }, { de: "/datenschutz", en: "/en/privacy" }, { de: "/nutzungsbedingungen", en: "/en/terms" }, ...seiten("de").map(alternativen)];
  for (const s of seiten("en")) { const a = alternativen(s); if (!a.de) gruppen.push(a); }
  const eintraege = gruppen.flatMap((a) => Object.values(a).map((p) => {
    const alt = Object.entries(a).map(([sp, q]) => `<xhtml:link rel="alternate" hreflang="${sp}" href="${SEITE}${q}"/>`).join("")
      + (a.en ? `<xhtml:link rel="alternate" hreflang="x-default" href="${SEITE}${a.en}"/>` : "");
    return `  <url><loc>${SEITE}${p}</loc><lastmod>${STAND}</lastmod>${alt}</url>`;
  }));
  const xml = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">\n${eintraege.join("\n")}\n</urlset>\n`;
  return new Response(xml, { headers: { "content-type": "application/xml; charset=utf-8" } });
};
