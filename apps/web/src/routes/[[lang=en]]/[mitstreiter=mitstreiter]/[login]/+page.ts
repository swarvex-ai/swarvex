import { error, redirect } from "@sveltejs/kit";

export const load = async ({ fetch, params }) => {
  const en = params.lang === "en";
  if (en !== (params.mitstreiter === "contributors")) redirect(308, `${en ? "/en/contributors" : "/mitstreiter"}/${params.login}`);
  const r = await fetch(`/api/people/${encodeURIComponent(params.login)}`);
  // Ohne Zustimmung der Person gibt es keine Seite (404, wie bei einem unbekannten Namen)
  if (r.status === 404) error(404, "no public profile");
  if (!r.ok) error(502, "profile unavailable");
  return { profil: await r.json() };
};
