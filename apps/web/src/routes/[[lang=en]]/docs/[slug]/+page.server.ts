import { error } from "@sveltejs/kit";
import { REPO_WEB, alternativen, rendern, seiten } from "$lib/server/doku";
import type { EntryGenerator } from "./$types";

export const entries: EntryGenerator = () => [
  ...seiten("de").map((s) => ({ lang: undefined, slug: s.slug })),
  ...seiten("en").map((s) => ({ lang: "en" as const, slug: s.slug })),
];

export const load = async ({ params }) => {
  const sprache = params.lang === "en" ? "en" : "de";
  const alle = seiten(sprache);
  const i = alle.findIndex((s) => s.slug === params.slug);
  if (i < 0) error(404, sprache === "en" ? "This page does not exist in the docs." : "Diese Seite gibt es in der Doku nicht.");
  const s = alle[i];
  const { html, ueberschriften } = await rendern(s);
  const nachbar = (j: number) => (alle[j] ? { slug: alle[j].slug, titel: alle[j].titel } : null);
  return {
    slug: s.slug, titel: s.titel, gruppe: s.gruppe, beschreibung: s.beschreibung, html, ueberschriften,
    zurueck: nachbar(i - 1), weiter: nachbar(i + 1), alternativen: alternativen(s),
    bearbeiten: `${REPO_WEB.replace("/blob/", "/edit/")}/${s.datei}`,
    eingebunden: s.quelle ?? null,
  };
};
