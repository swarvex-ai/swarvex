import adapter from "@sveltejs/adapter-node";

/** @type {import('@sveltejs/kit').Config} */
export default {
  kit: {
    adapter: adapter(),
    prerender: {
      // /projekte liefert (noch) das Statusboard der API aus, nicht diese App
      handleHttpError: ({ path, message }) => { if (path.startsWith("/projekte")) return; throw new Error(message); },
    },
  },
};
