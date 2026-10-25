import { redirect } from "@sveltejs/kit";
import { STARTSEITE, docsPfad } from "$lib/server/doku";

export const load = ({ params }) => {
  const s = params.lang === "en" ? "en" : "de";
  redirect(307, docsPfad(s, STARTSEITE[s]));
};
