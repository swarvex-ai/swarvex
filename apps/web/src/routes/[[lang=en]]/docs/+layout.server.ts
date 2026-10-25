import { navigation } from "$lib/server/doku";

export const prerender = true;
export const load = ({ params }) => ({ nav: navigation(params.lang === "en" ? "en" : "de") });
