<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { useServerStore } from "@/stores/server";
import Icon from "@/components/ui/Icon.vue";
import Toaster from "@/components/ui/Toaster.vue";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
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

// Build is preset/setup work (admin only); most of Admin is admin only.
// Operate is open to every role (viewers see, stewards operate).
const allSections = [
  {
    label: "Operate",
    items: [
      { to: "/", label: "Dashboard", icon: "dashboard" },
      { to: "/events", label: "Race Setups", icon: "events" },
      { to: "/queue", label: "Run Plan", icon: "queue" },
      { to: "/drivers", label: "Driver Stats", icon: "trophy" },
      { to: "/sessions", label: "Session Search", icon: "search" },
      { to: "/leaderboard", label: "Leaderboard", icon: "flag" },
      { to: "/guest-drivers", label: "Guest Drivers", icon: "users", operate: true },
    ],
  },
  {
    label: "Build",
    items: [
      { to: "/presets/classes", label: "Car Classes", icon: "car", operate: true },
      { to: "/presets/difficulty", label: "Difficulty", icon: "difficulty", admin: true },
      { to: "/presets/sessions", label: "Sessions", icon: "clock", admin: true },
      { to: "/presets/time", label: "Time & Weather", icon: "weather", admin: true },
    ],
  },
  {
    label: "Admin",
    items: [
      { to: "/setup", label: "Server Setup", icon: "settings", admin: true },
      { to: "/settings", label: "Configuration", icon: "settings", admin: true },
      { to: "/settings/instances", label: "Instances", icon: "instances", admin: true },
      { to: "/settings/streams", label: "Stream Debug", icon: "broadcast", admin: true },
      { to: "/content", label: "Content", icon: "content", admin: true },
      { to: "/maintenance", label: "Backup & Restore", icon: "content", admin: true },
      { to: "/settings/users", label: "Users & Roles", icon: "users", admin: true },
      { to: "/preferences", label: "Preferences", icon: "user" },
      { to: "/about", label: "About", icon: "info" },
    ],
  },
] as const;

type NavItem = { to: string; label: string; icon: string; admin?: boolean; operate?: boolean };

// Item visibility: admin items need admin; operate items need steward-or-admin;
// everything else is open to any role.
const canSee = (it: NavItem) => (it.admin ? auth.isAdmin : it.operate ? auth.canOperate : true);

const navSections = computed(() =>
  allSections
    .filter((s) => !("admin" in s && s.admin) || auth.isAdmin)
    .map((s) => ({ label: s.label, items: (s.items as readonly NavItem[]).filter(canSee) }))
    .filter((s) => s.items.length > 0),
);

// Bottom-tab nav (mobile): the four daily-operations items; the fifth slot is a
// hamburger that opens a drawer with every remaining (role-visible) item.
// Capped at four so the grid stays 4 tabs + hamburger — extra Operate items
// (e.g. Leaderboard) are reachable from the "More" drawer.
const operate = allSections[0].items as readonly NavItem[];
const mobileNav = computed<NavItem[]>(() =>
  (operate as NavItem[]).filter(canSee).slice(0, 4), // Dashboard, Events, Queue, Driver Stats
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

  <!-- Public (login) and bare (full-screen broadcast) routes skip the app shell. -->
  <RouterView v-if="route.meta.public || route.meta.bare" />

  <div v-else class="min-h-screen overflow-x-hidden bg-bg text-text">
    <a
      href="#main-content"
      class="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-md focus:border focus:border-accent/50 focus:bg-surface focus:px-3 focus:py-2 focus:text-sm focus:text-accent"
    >
      Skip to main content
    </a>

    <div class="flex h-screen overflow-hidden">
      <aside class="hidden h-full w-64 shrink-0 flex-col overflow-hidden border-r border-line bg-surface/95 px-3 py-4 md:flex">
        <div class="mb-6 flex shrink-0 items-center gap-3 px-2">
          <div class="grid size-9 place-items-center rounded-md border border-accent/30 bg-accent-dim text-sm font-black text-accent">
            SM
          </div>
          <div class="min-w-0">
            <div class="truncate text-sm font-bold tracking-tight text-text">Server Manager</div>
            <div class="text-xs text-dim">Race operations</div>
          </div>
          <span
            class="ml-auto size-2 rounded-full"
            :class="server.connected ? 'bg-ok shadow-[0_0_14px_rgba(79,216,132,0.55)]' : 'bg-danger'"
            :title="server.connected ? 'Live' : 'Disconnected'"
          />
        </div>

        <nav class="-mr-1 flex-1 space-y-5 overflow-y-auto pr-1">
          <section v-for="section in navSections" :key="section.label">
            <h2 class="mb-1.5 px-3 text-[11px] font-bold tracking-wide text-dim uppercase">
              {{ section.label }}
            </h2>
            <div class="space-y-1">
              <RouterLink
                v-for="item in section.items"
                :key="item.to"
                :to="item.to"
                class="flex min-h-9 items-center gap-2.5 rounded-md px-3 text-sm font-medium text-muted transition-colors duration-200 hover:bg-surface-2 hover:text-text"
                active-class="bg-accent-dim !text-accent"
              >
                <Icon :name="item.icon" :size="17" />
                <span class="truncate">{{ item.label }}</span>
              </RouterLink>
            </div>
          </section>
        </nav>

        <div class="mt-3 shrink-0 border-t border-line px-2 pt-3">
          <div class="mb-2 flex items-center gap-2">
            <div class="grid size-7 place-items-center rounded-md bg-surface-2 text-xs font-bold text-muted">
              {{ auth.user?.name?.charAt(0)?.toUpperCase() ?? "?" }}
            </div>
            <div class="min-w-0">
              <div class="truncate text-xs font-semibold text-text">{{ auth.user?.name }}</div>
              <div class="text-[11px] text-dim">Signed in</div>
            </div>
          </div>
          <button
            type="button"
            class="flex min-h-8 w-full cursor-pointer items-center gap-2 rounded-md px-2 text-xs font-semibold text-muted transition-colors hover:bg-surface-2 hover:text-text"
            @click="logout"
          >
            <Icon name="logOut" :size="15" />
            Sign out
          </button>
        </div>
      </aside>

      <main id="main-content" class="min-w-0 flex-1 overflow-y-auto px-4 py-4 pb-24 md:px-6 md:py-6 md:pb-6 lg:px-8">
        <div class="mb-4 flex items-center gap-3 md:hidden">
          <div class="grid size-9 place-items-center rounded-md border border-accent/30 bg-accent-dim text-sm font-black text-accent">
            SM
          </div>
          <div class="min-w-0">
            <div class="text-sm font-bold">Server Manager</div>
            <div class="text-xs text-dim">{{ server.connected ? "Live connection" : "Disconnected" }}</div>
          </div>
          <span
            class="ml-auto size-2 rounded-full"
            :class="server.connected ? 'bg-ok' : 'bg-danger'"
          />
        </div>

        <p
          v-if="setupNeeded && auth.isAdmin"
          class="mb-4 flex gap-2 rounded-md border border-accent/40 bg-accent-dim px-3 py-2 text-sm text-text"
        >
          <Icon name="info" :size="17" class="mt-0.5 shrink-0 text-accent" />
          <span>
            First-run setup isn't finished yet.
            <RouterLink to="/setup" class="font-semibold text-accent hover:underline">Open Server Setup</RouterLink>
            to see what's left and fix each step.
          </span>
        </p>
        <RouterView />
      </main>
    </div>

    <nav class="fixed inset-x-0 bottom-0 z-30 grid grid-cols-5 border-t border-line bg-surface/95 px-1 py-1.5 backdrop-blur md:hidden">
      <RouterLink
        v-for="item in mobileNav"
        :key="item.to"
        :to="item.to"
        class="flex min-h-12 flex-col items-center justify-center gap-1 rounded-md text-[11px] font-semibold text-muted transition-colors hover:bg-surface-2 hover:text-text"
        active-class="bg-accent-dim !text-accent"
      >
        <Icon :name="item.icon" :size="18" />
        <span>{{ item.label }}</span>
      </RouterLink>
      <button
        type="button"
        class="flex min-h-12 flex-col items-center justify-center gap-1 rounded-md text-[11px] font-semibold text-muted transition-colors hover:bg-surface-2 hover:text-text"
        :class="mobileMenuOpen ? 'bg-accent-dim !text-accent' : ''"
        aria-label="More menu"
        @click="mobileMenuOpen = true"
      >
        <Icon name="menu" :size="18" />
        <span>More</span>
      </button>
    </nav>

    <!-- Mobile "more" drawer: every role-visible nav item -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition-opacity duration-200"
        leave-active-class="transition-opacity duration-200"
        enter-from-class="opacity-0"
        leave-to-class="opacity-0"
      >
        <div
          v-if="mobileMenuOpen"
          class="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm md:hidden"
          @click="mobileMenuOpen = false"
        />
      </Transition>
      <Transition
        enter-active-class="transition-transform duration-200 ease-out"
        leave-active-class="transition-transform duration-200 ease-in"
        enter-from-class="translate-y-full"
        leave-to-class="translate-y-full"
      >
        <div
          v-if="mobileMenuOpen"
          class="fixed inset-x-0 bottom-0 z-50 max-h-[80vh] overflow-y-auto rounded-t-2xl border-t border-line bg-surface px-3 pt-3 pb-6 md:hidden"
        >
          <div class="mb-3 flex items-center justify-between px-2">
            <h2 class="text-sm font-bold text-text">Menu</h2>
            <button
              type="button"
              class="grid size-8 place-items-center rounded-md text-muted transition-colors hover:bg-surface-2 hover:text-text"
              aria-label="Close menu"
              @click="mobileMenuOpen = false"
            >
              <Icon name="x" :size="18" />
            </button>
          </div>
          <nav class="space-y-4">
            <section v-for="section in navSections" :key="section.label">
              <h3 class="mb-1.5 px-3 text-[11px] font-bold tracking-wide text-dim uppercase">
                {{ section.label }}
              </h3>
              <div class="space-y-1">
                <RouterLink
                  v-for="item in section.items"
                  :key="item.to"
                  :to="item.to"
                  class="flex min-h-10 items-center gap-2.5 rounded-md px-3 text-sm font-medium text-muted transition-colors hover:bg-surface-2 hover:text-text"
                  active-class="bg-accent-dim !text-accent"
                >
                  <Icon :name="item.icon" :size="17" />
                  <span class="truncate">{{ item.label }}</span>
                </RouterLink>
              </div>
            </section>
          </nav>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
