import { redirect } from "@sveltejs/kit";

export const load = async ({ fetch, params, depends }) => {
  const en = params.lang === "en";
  // /en/projects und /projekte sind die richtigen Adressen, die Mischformen leiten um
  if (en !== (params.projekte === "projects")) redirect(308, en ? "/en/projects" : "/projekte");
  depends("app:projekte");
  // Projektideen „Lead gesucht“ (apps/api/src/ideen.ts): ohne sie bleibt die Liste trotzdem nutzbar
  const ideen = fetch("/api/ideas").then((x) => (x.ok ? x.json() : { ideas: [] })).then((x) => x.ideas ?? []).catch(() => []);
  try {
    const r = await fetch("/api/projects");
    if (!r.ok) throw new Error(String(r.status));
    return { liste: await r.json(), fehler: false, ideen: await ideen };
  } catch {
    return { liste: null, fehler: true, ideen: await ideen };
  }
};
