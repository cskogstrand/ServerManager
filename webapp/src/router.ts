import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/stores/auth";

// Routes fill in over Phases 2-5.
const router = createRouter({
  history: createWebHistory("/"),
  routes: [
    { path: "/login", name: "login", component: () => import("@/pages/Login.vue"), meta: { public: true } },
    { path: "/", name: "dashboard", component: () => import("@/pages/Dashboard.vue") },
    { path: "/events", name: "events", component: () => import("@/pages/Events.vue") },
    { path: "/queue", name: "queue", component: () => import("@/pages/Queue.vue") },
    { path: "/content", name: "content", component: () => import("@/pages/Content.vue") },
    { path: "/presets/difficulty", name: "preset-difficulty", component: () => import("@/pages/PresetDifficulty.vue") },
    { path: "/presets/sessions", name: "preset-sessions", component: () => import("@/pages/PresetSession.vue") },
    { path: "/presets/time", name: "preset-time", component: () => import("@/pages/PresetTime.vue") },
    { path: "/presets/classes", name: "preset-classes", component: () => import("@/pages/PresetClass.vue") },
    { path: "/settings", name: "settings", component: () => import("@/pages/SettingsConfig.vue") },
    { path: "/settings/instances", name: "instances", component: () => import("@/pages/SettingsInstances.vue") },
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
