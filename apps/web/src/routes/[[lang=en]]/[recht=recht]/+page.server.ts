import { error, redirect } from "@sveltejs/kit";
import { RECHT_SEITEN, rechtPfad, rechtSeite } from "$lib/server/recht";

export const load = ({ params }) => {
  const s = params.lang === "en" ? "en" : "de";
  const seite = rechtSeite(s, params.recht);
  if (seite) return seite;
  // Seite in der anderen Sprache angefragt (z. B. /en/impressum): zur passenden Adresse
  const andere = s === "en" ? "de" : "en";
  const k = (Object.keys(RECHT_SEITEN) as (keyof typeof RECHT_SEITEN)[]).find((x) => RECHT_SEITEN[x][andere] === params.recht);
  if (k) redirect(308, rechtPfad(s, k));
  error(404);
};
