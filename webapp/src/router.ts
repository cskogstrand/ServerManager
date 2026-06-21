import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/stores/auth";

// Routes fill in over Phases 2-5.
const router = createRouter({
  history: createWebHistory("/"),
  routes: [
    { path: "/login", name: "login", component: () => import("@/pages/Login.vue"), meta: { public: true } },
    { path: "/", name: "dashboard", component: () => import("@/pages/Dashboard.vue") },
    { path: "/server/:id", name: "server-detail", component: () => import("@/pages/ServerDetail.vue") },
    { path: "/server/:id/broadcast", name: "server-broadcast", component: () => import("@/pages/Broadcast.vue"), meta: { bare: true } },
    { path: "/broadcast", name: "broadcast-auto", component: () => import("@/pages/Broadcast.vue"), meta: { bare: true } },
    { path: "/leaderboard", name: "leaderboard", component: () => import("@/pages/Leaderboard.vue"), meta: { bare: true } },
    { path: "/events", name: "events", component: () => import("@/pages/Events.vue") },
    { path: "/queue", name: "queue", component: () => import("@/pages/Queue.vue") },
    { path: "/drivers", name: "drivers", component: () => import("@/pages/DriverStats.vue") },
    { path: "/drivers/:guid", name: "driver-detail", component: () => import("@/pages/DriverDetail.vue") },
    { path: "/guest-drivers", name: "guest-drivers", component: () => import("@/pages/GuestDrivers.vue"), meta: { operate: true } },
    { path: "/guest-drivers/:id", name: "guest-driver-detail", component: () => import("@/pages/GuestDriverDetail.vue") },
    { path: "/content", name: "content", component: () => import("@/pages/Content.vue") },
    { path: "/presets/difficulty", name: "preset-difficulty", component: () => import("@/pages/PresetDifficulty.vue"), meta: { admin: true } },
    { path: "/presets/sessions", name: "preset-sessions", component: () => import("@/pages/PresetSession.vue"), meta: { admin: true } },
    { path: "/presets/time", name: "preset-time", component: () => import("@/pages/PresetTime.vue"), meta: { admin: true } },
    { path: "/presets/classes", name: "preset-classes", component: () => import("@/pages/PresetClass.vue"), meta: { operate: true } },
    { path: "/setup", name: "setup", component: () => import("@/pages/SetupWorkbench.vue") },
    { path: "/settings", name: "settings", component: () => import("@/pages/SettingsConfig.vue") },
    { path: "/settings/instances", name: "instances", component: () => import("@/pages/SettingsInstances.vue") },
    { path: "/settings/streams", name: "streams-debug", component: () => import("@/pages/StreamsDebug.vue"), meta: { admin: true } },
    { path: "/maintenance", name: "maintenance", component: () => import("@/pages/Maintenance.vue") },
    { path: "/settings/users", name: "users", component: () => import("@/pages/Users.vue"), meta: { admin: true } },
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
  if (to.meta.admin && !auth.isAdmin) {
    return { name: "dashboard" };
  }
  if (to.meta.operate && !auth.canOperate) {
    return { name: "dashboard" };
  }
  return true;
});

export default router;
