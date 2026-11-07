import { defineConfig } from "vite";

// base "./": the build works under any path (playable previews of every submission live under their own path)
export default defineConfig({
  base: "./",
  // three.js is about 800 kB minified; split by dynamic import() once the game grows
  build: { target: "es2022", chunkSizeWarningLimit: 1500 },
  test: { include: ["tests/core/**/*.test.ts"] },
});
