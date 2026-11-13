import type { Handle, HandleFetch } from "@sveltejs/kit";
import { env } from "$env/dynamic/private";

// <html lang> passend zur URL (auch beim Vorrendern)
export const handle: Handle = ({ event, resolve }) => {
  const lang = event.url.pathname === "/en" || event.url.pathname.startsWith("/en/") ? "en" : "de";
  return resolve(event, { transformPageChunk: ({ html }) => html.replace("%lang%", lang) });
};

// Beim serverseitigen Rendern geht /api/… direkt an den API-Dienst (im Compose-Stack http://api:3000),
// im Browser über Caddy. So sind Projektseiten schon im ausgelieferten HTML vollständig.
// Die Adresse des Besuchers geht mit (X-Forwarded-For): Die API bremst je Besucher, nicht alle Seitenaufrufe zusammen
// als einen (Sicherheitsaudit 12.11.2026). adapter-node liest sie aus dem Kopf, den Caddy setzt (ADDRESS_HEADER).
export const handleFetch: HandleFetch = ({ event, request, fetch }) => {
  const url = new URL(request.url);
  if (url.pathname.startsWith("/api/") && env.API_INTERNAL) {
    const weiter = new Request(env.API_INTERNAL + url.pathname + url.search, request);
    try {
      weiter.headers.set("x-forwarded-for", event.getClientAddress());
    } catch {
      // beim Vorrendern gibt es keinen Besucher
    }
    return fetch(weiter);
  }
  return fetch(request);
};
