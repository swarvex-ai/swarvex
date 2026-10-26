import type { Handle, HandleFetch } from "@sveltejs/kit";
import { env } from "$env/dynamic/private";

// <html lang> passend zur URL (auch beim Vorrendern)
export const handle: Handle = ({ event, resolve }) => {
  const lang = event.url.pathname === "/en" || event.url.pathname.startsWith("/en/") ? "en" : "de";
  return resolve(event, { transformPageChunk: ({ html }) => html.replace("%lang%", lang) });
};

// Beim serverseitigen Rendern geht /api/… direkt an den API-Dienst (im Compose-Stack http://api:3000),
// im Browser über Caddy. So sind Projektseiten schon im ausgelieferten HTML vollständig.
export const handleFetch: HandleFetch = ({ request, fetch }) => {
  const url = new URL(request.url);
  if (url.pathname.startsWith("/api/") && env.API_INTERNAL) {
    return fetch(new Request(env.API_INTERNAL + url.pathname + url.search, request));
  }
  return fetch(request);
};
