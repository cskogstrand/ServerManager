import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/stores/auth";

// Routes fill in over Phases 2-5.
const router = createRouter({
  history: createWebHistory("/app/"),
  routes: [
    { path: "/login", name: "login", component: () => import("@/pages/Login.vue"), meta: { public: true } },
    { path: "/", name: "dashboard", component: () => import("@/pages/Dashboard.vue") },
    { path: "/settings", name: "settings", component: () => import("@/pages/SettingsConfig.vue") },
    { path: "/preferences", name: "preferences", component: () => import("@/pages/SettingsUser.vue") },
    { path: "/about", name: "about", component: () => import("@/pages/About.vue") },
  ],
});

router.beforeEach(async (to) => {
  if (to.meta.public) return true;
  const auth = useAuthStore();
  await auth.ensureChecked();
  if (!auth.loggedIn) {
    return { name: "login", query: { redirect: to.fullPath } };
  }
  return true;
});

export default router;
