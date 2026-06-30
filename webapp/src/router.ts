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
    { path: "/sessions", name: "session-search", component: () => import("@/pages/SessionSearch.vue") },
    { path: "/guest-drivers", name: "guest-drivers", component: () => import("@/pages/GuestDrivers.vue"), meta: { operate: true } },
    { path: "/guest-drivers/:id", name: "guest-driver-detail", component: () => import("@/pages/GuestDriverDetail.vue"), meta: { operate: true } },
    { path: "/content", name: "content", component: () => import("@/pages/Content.vue"), meta: { admin: true } },
    { path: "/presets/difficulty", name: "preset-difficulty", component: () => import("@/pages/PresetDifficulty.vue"), meta: { admin: true } },
    { path: "/presets/sessions", name: "preset-sessions", component: () => import("@/pages/PresetSession.vue"), meta: { admin: true } },
    { path: "/presets/time", name: "preset-time", component: () => import("@/pages/PresetTime.vue"), meta: { admin: true } },
    { path: "/presets/classes", name: "preset-classes", component: () => import("@/pages/PresetClass.vue"), meta: { operate: true } },
    { path: "/setup", name: "setup", component: () => import("@/pages/SetupWorkbench.vue"), meta: { admin: true } },
    { path: "/settings", name: "settings", component: () => import("@/pages/SettingsConfig.vue"), meta: { admin: true } },
    { path: "/settings/instances", name: "instances", component: () => import("@/pages/SettingsInstances.vue"), meta: { admin: true } },
    { path: "/settings/streams", name: "streams-debug", component: () => import("@/pages/StreamsDebug.vue"), meta: { admin: true } },
    { path: "/maintenance", name: "maintenance", component: () => import("@/pages/Maintenance.vue"), meta: { admin: true } },
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

// Stale-chunk recovery: after a redeploy the old hashed route chunks no longer
// exist, so lazy import() rejects and navigation hangs. Reload the target once
// to pull the fresh build. The timestamp guard stops a reload loop if the chunk
// is genuinely missing (e.g. mid-deploy).
router.onError((err, to) => {
  if (!/dynamically imported module|module script failed/i.test(String(err))) return;
  const last = Number(sessionStorage.getItem("chunk-reload-ts") || 0);
  if (Date.now() - last > 10_000) {
    sessionStorage.setItem("chunk-reload-ts", String(Date.now()));
    window.location.assign(to.fullPath);
  }
});

export default router;
