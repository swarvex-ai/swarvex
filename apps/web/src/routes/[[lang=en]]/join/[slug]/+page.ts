// Teilen-Link eines Projekts (E-39): swarvex.ai/join/<projekt> und /en/join/<projekt>. Er gibt keine Rechte.
import { error } from "@sveltejs/kit";

export const load = async ({ fetch, params }) => {
  const en = params.lang === "en";
  const r = await fetch(`/api/projects/${params.slug}`);
  if (r.status === 404) error(404, en ? "This project does not exist." : "Dieses Projekt gibt es nicht.");
  if (!r.ok) error(503, en ? "The platform is currently unreachable." : "Die Plattform ist gerade nicht erreichbar.");
  return { projekt: await r.json() };
};
