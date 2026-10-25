import { json } from "@sveltejs/kit";
import { suchindex } from "$lib/server/doku";

export const prerender = true;
export const entries = () => [{ lang: undefined }, { lang: "en" as const }];
export const GET = ({ params }) => json(suchindex(params.lang === "en" ? "en" : "de"));
