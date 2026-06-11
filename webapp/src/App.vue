<script setup lang="ts">
import { onMounted } from "vue";
import { useServerStore } from "@/stores/server";

const server = useServerStore();

onMounted(() => {
  void server.load();
  server.connect();
});

const nav = [
  { to: "/", label: "Dashboard", icon: "▣" },
  // Events, Content, Presets, Settings arrive in Phases 2-5
];
</script>

<template>
  <div class="flex min-h-screen">
    <aside class="w-[228px] shrink-0 border-r border-line bg-surface px-3 py-4 max-md:hidden">
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
    </aside>

    <main class="min-w-0 flex-1 p-6">
      <RouterView />
    </main>
  </div>
</template>
