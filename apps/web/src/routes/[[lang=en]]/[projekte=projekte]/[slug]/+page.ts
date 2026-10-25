import { error, redirect } from "@sveltejs/kit";

export const load = async ({ fetch, params, depends }) => {
  const en = params.lang === "en";
  if (en !== (params.projekte === "projects")) redirect(308, `${en ? "/en/projects" : "/projekte"}/${params.slug}`);
  depends("app:projekt");
  const r = await fetch(`/api/projekte/${params.slug}`);
  if (r.status === 404) error(404, en ? "This project does not exist." : "Dieses Projekt gibt es nicht.");
  if (!r.ok) error(503, en ? "The platform is currently unreachable." : "Die Plattform ist gerade nicht erreichbar.");
  return { projekt: await r.json() };
};
