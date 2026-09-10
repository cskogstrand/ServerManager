<script setup lang="ts">
import PageHeader from "@/components/ui/PageHeader.vue";
import Icon from "@/components/ui/Icon.vue";

const adminItems = [
  { to: "/setup", label: "Server Setup", detail: "First-run setup, setup health, and run readiness.", icon: "settings" },
  { to: "/settings/installation", label: "Installation", detail: "Assetto Corsa path and CSP requirements.", icon: "folder" },
  { to: "/content", label: "Content Library", detail: "Cars, tracks, cache rebuilds, and content checks.", icon: "content" },
  { to: "/settings", label: "Server Configuration", detail: "Lobby identity, access, engine, and server limits.", icon: "settings" },
  { to: "/settings/instances", label: "Server Instances", detail: "Server processes, ports, boot behavior, and queues.", icon: "instances" },
  { to: "/settings/streaming", label: "Streaming & Capture", detail: "Driver streams, spectator streams, and highlights.", icon: "broadcast" },
  { to: "/settings/streams", label: "Stream Diagnostics", detail: "Recorder state, probes, buffers, and stream logs.", icon: "activity" },
  { to: "/maintenance", label: "Backup & Restore", detail: "Database backups, restore points, and support exports.", icon: "content" },
  { to: "/settings/users", label: "Users & Roles", detail: "Accounts, roles, passwords, and access control.", icon: "users" },
];
</script>

<template>
  <PageHeader
    title="Your workspace, configured."
    subtitle="Server setup, content, and the people who keep your race nights running."
    icon="lock"
  />

  <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
    <RouterLink v-for="item in adminItems" :key="item.to" :to="item.to" class="group rounded-md border border-line bg-surface p-6 transition-colors hover:border-accent/60 hover:bg-surface-2/40">
      <div class="mb-5 flex items-center justify-between"><span class="grid size-11 place-items-center rounded-md bg-accent-dim text-accent"><Icon :name="item.icon" :size="20" /></span><Icon name="arrowUp" :size="16" class="rotate-90 text-dim group-hover:text-accent" /></div>
      <h2 class="text-base font-medium">{{ item.label }}</h2>
      <p class="mt-2 text-sm leading-relaxed text-muted">{{ item.detail }}</p>
    </RouterLink>
  </div>
</template>
