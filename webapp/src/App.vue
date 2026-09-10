<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { useServerStore } from "@/stores/server";
import Icon from "@/components/ui/Icon.vue";
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
  await router.push({ name: "login" });
}

// Theme toggle — dark default; the pre-paint init lives in index.html.
const theme = ref<"dark" | "light">(
  (document.documentElement.dataset.theme as "dark" | "light") || "dark",
);
function setTheme(value: "light" | "dark") {
  theme.value = value;
  document.documentElement.dataset.theme = theme.value;
  try { localStorage.setItem("theme", theme.value); } catch { /* Theme still works when browser storage is unavailable. */ }
}
const navExpanded = ref(false);

// Navigation follows the user's jobs: operate live servers, build reusable race
// setups, and review driver history. Admin lives behind the account footer.
const allSections = [
  {
    label: "Operate",
    items: [
      { to: "/", label: "Dashboard", icon: "dashboard" },
      { key: "race-control", label: "Race Control", icon: "activity" },
      { to: "/queue", label: "Run Plan", icon: "queue" },
      { to: "/broadcast", label: "Broadcast Display", icon: "broadcast" },
    ],
  },
  {
    label: "Build",
    items: [
      { to: "/events", label: "Race Setups", icon: "events" },
      { to: "/presets", label: "Templates", icon: "settings", operate: true },
    ],
  },
  {
    label: "Drivers & History",
    items: [
      { to: "/drivers", label: "Driver Stats", icon: "trophy" },
      { to: "/sessions", label: "Session Search", icon: "search" },
      { to: "/guest-drivers", label: "Guest Drivers", icon: "users", operate: true },
      { to: "/history", label: "Results & History", icon: "trophy" },
    ],
  },
] as const;

type NavItem = { to?: string; key?: "race-control"; label: string; icon: string; admin?: boolean; operate?: boolean };
type AccountItem = { to: string; label: string; icon: string };

const accountItems = computed<AccountItem[]>(() => {
  const items: AccountItem[] = [
    { to: "/preferences", label: "Preferences", icon: "user" },
    { to: "/about", label: "About", icon: "info" },
  ];
  if (auth.isAdmin) items.splice(1, 0, { to: "/admin", label: "Admin", icon: "lock" });
  return items;
});

// Item visibility: admin items need admin; operate items need steward-or-admin;
// everything else is open to any role.
const canSee = (it: NavItem) => (it.admin ? auth.isAdmin : it.operate ? auth.canOperate : true);

const navSections = computed(() =>
  allSections
    .filter((s) => !("admin" in s && s.admin) || auth.isAdmin)
    .map((s) => ({ label: s.label, items: (s.items as readonly NavItem[]).filter(canSee) }))
    .filter((s) => s.items.length > 0),
);

watch([() => route.fullPath, () => server.loaded], () => {
  const raw = route.name === "server-detail" ? route.params.id : route.query.instance;
  const id = typeof raw === "string" ? Number(raw) : null;
  if (id && server.instances[id]) server.selectInstance(id);
}, { immediate: true });

const raceControlTo = computed(() => {
  const id = server.selectedInstanceId ?? (server.instanceList.length === 1 ? server.instanceList[0].id : null);
  return id ? `/server/${id}` : "/?choose=server";
});
function navTo(item: NavItem): string {
  if (item.key === "race-control") return raceControlTo.value;
  if (item.to === "/queue" && server.selectedInstanceId) return `/queue?instance=${server.selectedInstanceId}`;
  return item.to ?? "/";
}
function navActive(item: NavItem): boolean {
  if (item.key === "race-control") return route.name === "server-detail";
  return !!item.to && (route.path === item.to || (item.to !== "/" && route.path.startsWith(item.to + "/")));
}
const workspaceLabel = computed(() => {
  return navSections.value.find(section => section.items.some(navActive))?.label ?? (route.meta.admin ? "Administration" : "Workspace");
});
function mobileLabel(item: NavItem): string {
  return item.key === "race-control" ? "Control" : item.to === "/broadcast" ? "Broadcast" : item.to === "/" ? "Overview" : item.label;
}
const refreshing = ref(false);
async function refreshConnection() {
  refreshing.value = true;
  try { await server.bootstrapLiveState(); }
  finally { refreshing.value = false; }
}
// Bottom-tab nav (mobile): the four daily-operation items; the fifth slot opens
// the drawer with everything else.
const operate = allSections[0].items as readonly NavItem[];
const mobileNav = computed<NavItem[]>(() =>
  (operate as NavItem[]).filter(canSee).slice(0, 4),
);

// Mobile "more" drawer: all nav sections, closed on navigation.
const mobileMenuOpen = ref(false);
watch(
  () => route.fullPath,
  () => {
    mobileMenuOpen.value = false;
  },
);
</script>

<template>
  <Toaster />
  <ConfirmDialog />

  <template v-if="route.meta.public || route.meta.bare">
    <div v-if="route.meta.public" class="absolute top-5 right-5 z-10"><ThemeSwitch :model-value="theme" @update:model-value="setTheme" /></div>
    <RouterView />
  </template>

  <div v-else class="min-h-screen bg-bg text-text">
    <a href="#main-content" class="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-md focus:border focus:border-accent focus:bg-surface focus:px-3 focus:py-2 focus:text-sm focus:text-accent">Skip to main content</a>
    <div class="paddock-shell">
      <aside id="workspace-navigation" class="workspace-rail" :class="{ expanded: navExpanded }" aria-label="Workspace navigation">
        <RouterLink to="/" class="rail-brand" aria-label="Server Manager overview"><BrandMark /><span class="rail-link-label text-sm font-semibold">Server Manager</span></RouterLink>
        <nav class="rail-nav" aria-label="Main navigation">
          <section v-for="section in navSections" :key="section.label" class="rail-section">
            <h2 class="rail-label">{{ section.label }}</h2>
            <RouterLink v-for="item in section.items" :key="item.to ?? item.key" :to="navTo(item)"
              class="rail-link" active-class="" :class="{ 'is-active': navActive(item) }"
              :aria-label="item.label" :title="item.label" :aria-current="navActive(item) ? 'page' : undefined">
              <Icon :name="item.icon" :size="19" /><span class="rail-link-label">{{ item.label }}</span>
            </RouterLink>
          </section>
        </nav>
        <div class="rail-footer">
          <RouterLink v-if="auth.isAdmin" to="/admin" class="rail-link" active-class="" :class="{ 'is-active': workspaceLabel === 'Administration' }" :aria-current="workspaceLabel === 'Administration' ? 'page' : undefined" aria-label="Admin" title="Administration"><Icon name="settings" :size="19" /><span class="rail-link-label">Administration</span></RouterLink>
          <button type="button" class="rail-link w-full" aria-label="Open account and navigation" title="Account and navigation" @click="mobileMenuOpen = true"><span class="grid size-7 shrink-0 place-items-center rounded-full bg-accent-dim text-xs font-semibold text-accent">{{ auth.user?.name?.charAt(0)?.toUpperCase() ?? '?' }}</span><span class="rail-link-label truncate">{{ auth.user?.name }}</span></button>
        </div>
      </aside>

      <div class="workspace-body">
        <header class="workspace-header">
          <div class="workspace-heading">
            <button type="button" class="hidden size-11 shrink-0 place-items-center rounded-md text-muted hover:bg-surface-2 hover:text-text md:grid" :aria-label="navExpanded ? 'Collapse navigation' : 'Expand navigation'" :aria-expanded="navExpanded" aria-controls="workspace-navigation" @click="navExpanded = !navExpanded"><Icon :name="navExpanded ? 'arrowLeft' : 'menu'" :size="18" /></button>
            <BrandMark class="md:hidden" />
            <div class="min-w-0"><div class="text-sm font-semibold tracking-tight">Server Manager</div><div class="mt-1 text-[10px] text-dim">{{ workspaceLabel }}</div></div>
          </div>
          <div class="workspace-tools">
            <div class="workspace-connection" :class="{ disconnected: !server.connected }"><span class="size-1.5 shrink-0 rounded-full bg-current" />{{ server.connected ? 'Live connection' : 'Updates disconnected' }}</div>
            <ThemeSwitch :model-value="theme" @update:model-value="setTheme" />
            <button type="button" class="workspace-account" :aria-label="`Account menu for ${auth.user?.name ?? 'user'}`" @click="mobileMenuOpen = true">{{ auth.user?.name?.charAt(0)?.toUpperCase() ?? '?' }}</button>
          </div>
        </header>
        <main id="main-content" tabindex="-1" class="workspace-content">
          <p v-if="setupNeeded && auth.isAdmin" class="mb-5 flex gap-2 rounded-md border border-accent/40 bg-accent-dim px-4 py-3 text-sm text-text">
            <Icon name="info" :size="17" class="mt-0.5 shrink-0 text-accent" />
            <span>First-run setup isn't finished yet. <RouterLink to="/setup" class="font-semibold text-accent hover:underline">Open Server Setup</RouterLink> to see what's left and fix each step.</span>
          </p>
          <div v-if="!server.connected || server.loadError || Object.keys(server.statusErrors).length" role="status" class="mb-5 flex flex-wrap items-center gap-3 rounded-md border border-warn/40 bg-warn-glow px-4 py-3 text-sm">
            <Icon name="alert" :size="18" class="shrink-0 text-warn" />
            <div class="min-w-0 flex-1"><p class="font-semibold text-warn">{{ server.loadError ? 'Server status unavailable' : !server.connected ? 'Live updates disconnected' : 'Some server status is out of date' }}</p><p class="text-muted">{{ server.loadError || 'Showing the last known state. This does not mean your race servers have stopped.' }}<span v-if="server.lastUpdatedAt"> Last update {{ new Date(server.lastUpdatedAt).toLocaleTimeString() }}.</span></p></div>
            <Button variant="dark" size="sm" :disabled="refreshing" @click="refreshConnection">{{ refreshing ? 'Refreshing…' : 'Retry status' }}</Button>
          </div>
          <RouterView />
        </main>
      </div>
    </div>

    <nav aria-label="Mobile navigation" class="fixed inset-x-0 bottom-0 z-30 grid grid-cols-5 border-t border-line bg-rail px-1 pt-1.5 pb-[max(0.375rem,env(safe-area-inset-bottom))] md:hidden">
      <RouterLink v-for="item in mobileNav" :key="item.to ?? item.key" :to="navTo(item)" class="flex min-h-12 flex-col items-center justify-center gap-1 rounded-md text-[11px] font-medium text-muted transition-colors hover:bg-surface-2 hover:text-text" active-class="" :class="{ 'bg-accent-dim !text-accent': navActive(item) }" :aria-current="navActive(item) ? 'page' : undefined"><Icon :name="item.icon" :size="18" /><span class="max-w-[72px]">{{ mobileLabel(item) }}</span></RouterLink>
      <button type="button" class="flex min-h-12 flex-col items-center justify-center gap-1 rounded-md text-[11px] font-medium text-muted hover:bg-surface-2 hover:text-text" :class="mobileMenuOpen ? 'bg-accent-dim !text-accent' : ''" aria-label="More menu" :aria-expanded="mobileMenuOpen" @click="mobileMenuOpen = true"><Icon name="menu" :size="18" /><span>More</span></button>
    </nav>

    <Sheet :open="mobileMenuOpen" title="Menu" @close="mobileMenuOpen = false">
      <nav aria-label="All navigation" class="space-y-6">
        <section v-for="section in navSections" :key="section.label">
          <h3 class="mb-2 font-mono text-[10px] tracking-wider text-dim uppercase">{{ section.label }}</h3>
          <RouterLink v-for="item in section.items" :key="item.to ?? item.key" :to="navTo(item)" class="flex min-h-11 items-center gap-3 rounded-md px-3 text-sm text-text hover:bg-surface-2" active-class="" :class="{ 'bg-accent-dim !text-accent': navActive(item) }" :aria-current="navActive(item) ? 'page' : undefined"><Icon :name="item.icon" :size="18" />{{ item.label }}</RouterLink>
        </section>
        <section class="border-t border-line pt-4">
          <h3 class="mb-2 text-xs font-medium text-muted">{{ auth.user?.name }} · Account</h3>
          <RouterLink v-for="item in accountItems" :key="item.to" :to="item.to" class="flex min-h-11 items-center gap-3 rounded-md px-3 text-sm hover:bg-surface-2"><Icon :name="item.icon" :size="18" />{{ item.label }}</RouterLink>
          <ThemeSwitch :model-value="theme" class="my-3 w-fit" @update:model-value="setTheme" />
          <button type="button" class="flex min-h-11 w-full items-center gap-3 rounded-md px-3 text-sm hover:bg-surface-2" @click="logout"><Icon name="logOut" :size="18" />Sign out</button>
        </section>
      </nav>
    </Sheet>
  </div>
</template>
