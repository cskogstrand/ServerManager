import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";

// The SPA is the UI, served by the Go binary at / from src/embed/webapp/dist.
// In dev, Vite serves it at http://localhost:5173/ and proxies API calls
// to the Go server on :3030 — cookies are not port-scoped, so the login
// session works on both.
export default defineConfig({
  base: "/",
  plugins: [vue(), tailwindcss()],
  build: {
    outDir: "../src/embed/webapp/dist",
    emptyOutDir: true,
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/api": process.env.PITLANE_API_URL || "http://localhost:3030",
      "/static": process.env.PITLANE_API_URL || "http://localhost:3030",
      "/dl": process.env.PITLANE_API_URL || "http://localhost:3030",
    },
  },
  test: {
    environment: "jsdom",
  },
});
