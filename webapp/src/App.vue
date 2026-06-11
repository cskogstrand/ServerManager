<script setup lang="ts">
import { ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { useServerStore } from "@/stores/server";
import Icon from "@/components/ui/Icon.vue";
import type { UserConfig } from "@/types/generated";

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const server = useServerStore();

// First-run setup: until config + install path are saved, no event can run
const setupNeeded = ref(false);

watch(
  () => auth.loggedIn,
  async (loggedIn) => {
    if (loggedIn) {
      void server.load();
      server.connect();
      try {
        const cfg = await api.get<UserConfig>("/api/config");
        setupNeeded.value = cfg.cfg_filled !== 1 || cfg.mod_filled !== 1;
      } catch {
        setupNeeded.value = false;
      }
    } else {
      server.disconnect();
    }
  },
  { immediate: true },
);

async function logout() {
  await auth.logout();
  await router.push({ name: "login" });
}

const navSections = [
  {
    label: "Operate",
    items: [
      { to: "/", label: "Dashboard", icon: "dashboard" },
      { to: "/events", label: "Events", icon: "events" },
      { to: "/queue", label: "Queue", icon: "queue" },
      { to: "/content", label: "Content", icon: "content" },
    ],
  },
  {
    label: "Presets",
    items: [
      { to: "/presets/classes", label: "Car Classes", icon: "car" },
      { to: "/presets/difficulty", label: "Difficulty", icon: "difficulty" },
      { to: "/presets/sessions", label: "Sessions", icon: "clock" },
      { to: "/presets/time", label: "Time & Weather", icon: "weather" },
    ],
  },
  {
    label: "Admin",
    items: [
      { to: "/settings", label: "Configuration", icon: "settings" },
      { to: "/settings/instances", label: "Instances", icon: "instances" },
      { to: "/preferences", label: "Preferences", icon: "user" },
      { to: "/about", label: "About", icon: "info" },
    ],
  },
];

const mobileNav = [
  navSections[0].items[0],
  navSections[0].items[1],
  navSections[0].items[2],
  navSections[0].items[3],
  navSections[2].items[0],
];
</script>

<template>
  <RouterView v-if="route.meta.public" />

  <div v-else class="min-h-screen overflow-x-hidden bg-bg text-text">
    <a
      href="#main-content"
      class="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-md focus:border focus:border-accent/50 focus:bg-surface focus:px-3 focus:py-2 focus:text-sm focus:text-accent"
    >
      Skip to main content
    </a>

    <div class="flex min-h-screen">
      <aside class="hidden w-64 shrink-0 flex-col border-r border-line bg-surface/95 px-3 py-4 md:flex">
        <div class="mb-6 flex items-center gap-3 px-2">
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

        <nav class="space-y-5">
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

        <div class="mt-auto border-t border-line px-2 pt-3">
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

      <main id="main-content" class="min-w-0 flex-1 px-4 py-4 pb-24 md:px-6 md:py-6 lg:px-8">
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
          v-if="setupNeeded"
          class="mb-4 flex gap-2 rounded-md border border-accent/40 bg-accent-dim px-3 py-2 text-sm text-text"
        >
          <Icon name="info" :size="17" class="mt-0.5 shrink-0 text-accent" />
          <span>
            Finish the first-run setup:
            <RouterLink to="/settings" class="font-semibold text-accent hover:underline">save the server configuration</RouterLink>
            and
            <RouterLink to="/content" class="font-semibold text-accent hover:underline">set the Assetto Corsa install path</RouterLink>.
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
    </nav>
  </div>
</template>
