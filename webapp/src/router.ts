import { createRouter, createWebHistory } from "vue-router";

// Routes fill in over Phases 2-5; Dashboard proves the pipeline.
const router = createRouter({
  history: createWebHistory("/app/"),
  routes: [
    { path: "/", name: "dashboard", component: () => import("@/pages/Dashboard.vue") },
  ],
});

export default router;
