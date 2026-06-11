import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";

// The SPA is served by the Go binary at /app (embedded webapp/dist).
// In dev, Vite serves it at http://localhost:5173/app/ and proxies API
// calls to the Go server on :3030 — log in once on :3030 and the cookies
// work here too (cookies are not port-scoped).
export default defineConfig({
  base: "/app/",
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/api": "http://localhost:3030",
      "/static": "http://localhost:3030",
    },
  },
  test: {
    environment: "jsdom",
  },
});
