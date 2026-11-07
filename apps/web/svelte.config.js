import adapter from "@sveltejs/adapter-node";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";

// Hashes der Inline-Skripte in app.html (Farbschema vor dem ersten Bild), aus der Datei selbst berechnet
const appSkripte = [...readFileSync(new URL("./src/app.html", import.meta.url), "utf8").matchAll(/<script>([\s\S]*?)<\/script>/g)]
  .map((m) => `sha256-${createHash("sha256").update(m[1]).digest("base64")}`);

/** @type {import('@sveltejs/kit').Config} */
export default {
  kit: {
    adapter: adapter(),
    // Neue Version nach einem Deploy erkennen: offene Tabs laden beim nächsten Klick neu (+layout.svelte), statt mit
    // altem Code weiterzunavigieren und neue Inhalte nicht zu zeigen
    version: { pollInterval: 300_000 },
    // Content-Security-Policy: nur eigene Quellen. SvelteKit setzt für seine Skripte Nonces (gerendert) bzw. Hashes
    // (vorgerendert); die Skripte aus app.html kommen über appSkripte dazu.
    csp: {
      mode: "auto",
      directives: {
        "default-src": ["self"],
        "script-src": ["self", ...appSkripte],
        "style-src": ["self", "unsafe-inline"],
        "img-src": ["self", "data:", "blob:"],
        "media-src": ["self", "blob:"],
        "font-src": ["self", "data:"],
        "connect-src": ["self"],
        "object-src": ["none"],
        "base-uri": ["self"],
        "form-action": ["self"],
      },
    },
    prerender: {
      // /projekte liefert (noch) das Statusboard der API aus, nicht diese App
      handleHttpError: ({ path, message }) => { if (path.startsWith("/projekte")) return; throw new Error(message); },
    },
  },
};
