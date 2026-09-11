<script setup lang="ts">
import {clearDriverStreams} from "@/lib/useDriverStreams";
import { onBeforeUnmount, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { useServerStore } from "@/stores/server";
import Toaster from "@/components/ui/Toaster.vue";
import Sheet from "@/components/ui/Sheet.vue";
import Button from "@/components/ui/Button.vue";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
import BrandMark from "@/components/ui/BrandMark.vue";
import ThemeSwitch from "@/components/ui/ThemeSwitch.vue";
import type { UserConfig } from "@/types/generated";

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const server = useServerStore();

// First-run setup: until config + install path are saved, no event can run
const setupNeeded = ref(false);

// Low-rate recovery: re-pull live status for running servers so dropped
// one-time events (players/drivers) self-heal without heavy polling.
let recoveryTimer: ReturnType<typeof setInterval> | null = null;

function onVisible() {
  if (document.visibilityState === "visible" && auth.loggedIn) {
    void server.bootstrapLiveState();
  }
}

function startRecovery() {
  if (!recoveryTimer) {
    recoveryTimer = setInterval(() => {
      if (server.instanceList.some((i) => i.running)) void server.recoverRunning();
    }, 15000);
  }
  document.addEventListener("visibilitychange", onVisible);
}

function stopRecovery() {
  if (recoveryTimer) {
    clearInterval(recoveryTimer);
    recoveryTimer = null;
  }
  document.removeEventListener("visibilitychange", onVisible);
}

watch(
  () => auth.loggedIn,
  async (loggedIn) => {
    if (loggedIn) {
      // Hydrate full live state before opening SSE so the first snapshot/events
      // land on populated instances instead of being dropped.
      await server.bootstrapLiveState();
      server.connect();
      startRecovery();
      try {
        if(!auth.isAdmin){setupNeeded.value=false;return}
        const cfg = await api.get<UserConfig>("/api/config");
        setupNeeded.value = cfg.cfg_filled !== 1 || cfg.mod_filled !== 1;
      } catch {
        setupNeeded.value = false;
      }
    } else {
      server.disconnect();
      stopRecovery();
    }
  },
  { immediate: true },
);

onBeforeUnmount(stopRecovery);

async function logout() {
  await auth.logout();
 clearDriverStreams();
  await router.push({ name: "login" });
}

// Theme toggle — dark default; the pre-paint init lives in index.html.
const theme = ref<"dark" | "light">(
  (document.documentElement.dataset.theme as "dark" | "light") || "light",
);
function setTheme(value: "light" | "dark") {
  theme.value = value;
  document.documentElement.dataset.theme = theme.value;
  try { localStorage.setItem("theme", theme.value); } catch { /* Theme still works when browser storage is unavailable. */ }
}
const navigation = [
  { to: "/", label: "Today" }, { to: "/sessions", label: "Sessions" },
  { to: "/live", label: "Live" }, { to: "/drivers", label: "Drivers" },
];
const menuOpen = ref(false);
function active(path: string) {
  if (path === "/garage" && /^\/(settings|presets|maintenance|setup|admin|about)(\/|$)/.test(route.path)) return true;
  if (path === "/sessions" && /^\/(server|events|queue|history)(\/|$)/.test(route.path)) return true;
  if (path === "/drivers" && route.path.startsWith("/guest-drivers")) return true;
  if (path === "/live" && route.path === "/broadcast") return true;
  return route.path === path || (path !== "/" && route.path.startsWith(path + "/"));
}
watch(() => route.fullPath, () => {
  menuOpen.value = false;
  const raw = route.params.id && route.path.startsWith("/server/") ? route.params.id : route.query.instance;
  const id = Number(raw);
  if (id && server.instances[id]) server.selectInstance(id);
});
const refreshing = ref(false);
async function refreshConnection() {
  refreshing.value = true;
  try { await server.bootstrapLiveState(); }
  finally { refreshing.value = false; }
}
</script>

<template>
  <Toaster /><ConfirmDialog />
  <template v-if="route.meta.public || route.meta.bare">
    <div v-if="route.meta.public" class="absolute top-5 right-5 z-10"><ThemeSwitch :model-value="theme" @update:model-value="setTheme" /></div>
    <RouterView />
  </template>
  <div v-else class="pitlane-shell">
    <a href="#main-content" class="pitlane-skip">Skip to content</a>
    <header class="pitlane-header workspace-header">
      <RouterLink to="/" class="pitlane-brand" aria-label="Pitlane home"><BrandMark />Pitlane<span>.</span></RouterLink>
      <nav aria-label="Main navigation" class="pitlane-nav">
        <RouterLink v-for="item in navigation" :key="item.to" :to="item.to" :aria-current="active(item.to) ? 'page' : undefined">{{ item.label }}</RouterLink>
      </nav>
      <div class="pitlane-account">
        <RouterLink to="/garage" class="pitlane-garage" :aria-current="active('/garage') ? 'page' : undefined">Garage</RouterLink>
        <button class="workspace-account" aria-label="Account and preferences" :aria-expanded="menuOpen" @click="menuOpen = !menuOpen">{{ auth.user?.name?.slice(0,1).toUpperCase() || 'P' }}</button>
      </div>
    </header>
    <div v-if="server.loadError || !server.connected" class="pitlane-connection" role="status">
      <span>{{ server.loadError || 'Live updates reconnecting. Showing the last known state.' }}</span>
      <button :disabled="refreshing" @click="refreshConnection">{{ refreshing ? 'Refreshing…' : 'Refresh status' }}</button>
    </div>
    <div v-if="setupNeeded" class="pitlane-connection">
      <span>{{ auth.isAdmin ? 'A little setup before your first drive.' : 'An administrator needs to finish the installation before driving.' }}</span>
      <RouterLink v-if="auth.isAdmin" to="/setup">Finish setup →</RouterLink>
    </div>
    <main id="main-content" class="pitlane-content workspace-content" tabindex="-1"><RouterView /></main>
    <footer class="pitlane-footer"><span>Your club, connected. Built around the drive.</span><RouterLink to="/garage/advanced">Advanced tools →</RouterLink></footer>
    <nav aria-label="Mobile navigation" class="pitlane-mobile">
      <RouterLink v-for="item in navigation" :key="item.to" :to="item.to" :aria-current="active(item.to) ? 'page' : undefined">{{ item.label }}</RouterLink>
      <RouterLink to="/garage" :aria-current="active('/garage') ? 'page' : undefined">Garage</RouterLink>
    </nav>
    <Sheet :open="menuOpen" title="Your account" @close="menuOpen = false">
      <nav aria-label="Account navigation" class="pitlane-stack">
        <p class="text-lg">{{ auth.user?.name }}</p>
        <RouterLink to="/preferences" class="pitlane-button">Preferences & password</RouterLink>
        <RouterLink to="/about" class="pitlane-button">About ServerManager</RouterLink>
        <ThemeSwitch :model-value="theme" @update:model-value="setTheme" />
        <Button variant="dark" @click="logout">Sign out</Button>
      </nav>
    </Sheet>
  </div>
</template>
