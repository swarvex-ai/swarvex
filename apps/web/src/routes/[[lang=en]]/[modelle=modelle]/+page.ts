import { redirect } from "@sveltejs/kit";

export const load = async ({ fetch, params }) => {
  const en = params.lang === "en";
  // /en/models und /modelle sind die richtigen Adressen, die Mischformen leiten um
  if (en !== (params.modelle === "models")) redirect(308, en ? "/en/models" : "/modelle");
  try {
    const r = await fetch("/api/models");
    if (!r.ok) throw new Error(String(r.status));
    return { rangliste: await r.json(), fehler: false };
  } catch {
    return { rangliste: null, fehler: true };
  }
};
