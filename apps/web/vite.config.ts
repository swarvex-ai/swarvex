import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vite";

// In der Entwicklung holt die Seite /api von der Labor-Plattform (Caddy mit interner CA).
const API = process.env.API_URL ?? "http://localhost:3000";

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    fs: { allow: ["../.."] },  // Doku-Quellen liegen im Repo außerhalb von apps/web
    proxy: { "/api": { target: API, changeOrigin: true, secure: false } },
  },
  preview: { proxy: { "/api": { target: API, changeOrigin: true, secure: false } } },
});
