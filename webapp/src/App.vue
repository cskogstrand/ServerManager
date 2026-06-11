<script setup lang="ts">
import { watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useServerStore } from "@/stores/server";

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const server = useServerStore();

// Live data only while logged in
watch(
  () => auth.loggedIn,
  (loggedIn) => {
    if (loggedIn) {
      void server.load();
      server.connect();
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

const nav = [
  { to: "/", label: "Dashboard", icon: "▣" },
  { to: "/events", label: "Events", icon: "⚑" },
  { to: "/queue", label: "Queue", icon: "≡" },
  { to: "/content", label: "Content", icon: "▤" },
  { to: "/presets/classes", label: "Car Classes", icon: "⛟" },
  { to: "/presets/difficulty", label: "Difficulty", icon: "◔" },
  { to: "/presets/sessions", label: "Sessions", icon: "◷" },
  { to: "/presets/time", label: "Time & Weather", icon: "☼" },
  { to: "/settings", label: "Configuration", icon: "⚙" },
  { to: "/preferences", label: "Preferences", icon: "☺" },
  { to: "/about", label: "About", icon: "ℹ" },
];
</script>

<template>
  <RouterView v-if="route.meta.public" />

  <div v-else class="flex min-h-screen">
    <aside class="flex w-[228px] shrink-0 flex-col border-r border-line bg-surface px-3 py-4 max-md:hidden">
      <div class="mb-6 flex items-center gap-2 px-2">
        <span class="text-lg font-bold tracking-tight">Server Manager</span>
        <span
          class="ml-auto size-2 rounded-full"
          :class="server.connected ? 'bg-ok' : 'bg-danger'"
          :title="server.connected ? 'Live' : 'Disconnected'"
        />
      </div>
      <nav class="space-y-1">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="flex items-center gap-2.5 rounded-md px-3 py-2 text-sm text-muted hover:bg-surface-2 hover:text-text"
          active-class="bg-accent-dim !text-accent"
        >
          <span aria-hidden="true">{{ item.icon }}</span>
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="mt-auto border-t border-line px-2 pt-3">
        <div class="mb-2 truncate text-xs text-muted">{{ auth.user?.name }}</div>
        <button
          type="button"
          class="text-xs text-dim hover:text-text"
          @click="logout"
        >
          Sign out
        </button>
      </div>
    </aside>

    <main class="min-w-0 flex-1 p-6">
      <RouterView />
    </main>
  </div>
</template>
