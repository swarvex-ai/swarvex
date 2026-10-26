import { redirect } from "@sveltejs/kit";

export const load = async ({ fetch, params, depends }) => {
  const en = params.lang === "en";
  // /en/projects und /projekte sind die richtigen Adressen, die Mischformen leiten um
  if (en !== (params.projekte === "projects")) redirect(308, en ? "/en/projects" : "/projekte");
  depends("app:projekte");
  try {
    const r = await fetch("/api/projects");
    if (!r.ok) throw new Error(String(r.status));
    return { liste: await r.json(), fehler: false };
  } catch {
    return { liste: null, fehler: true };
  }
};
