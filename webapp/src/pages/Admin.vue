<script setup lang="ts">
import PageHeader from "@/components/ui/PageHeader.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
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
    title="Admin"
    subtitle="System setup, configuration, diagnostics, users, and maintenance."
    icon="lock"
  />

  <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
    <Card v-for="item in adminItems" :key="item.to">
      <template #header>
        <Icon :name="item.icon" :size="16" class="text-accent" />
        <h2 class="min-w-0 truncate text-sm font-bold">{{ item.label }}</h2>
      </template>
      <p class="mb-3 text-sm text-muted">{{ item.detail }}</p>
      <RouterLink :to="item.to">
        <Button variant="dark" size="sm">
          Open
          <Icon name="arrowUp" :size="14" class="rotate-90" />
        </Button>
      </RouterLink>
    </Card>
  </div>
</template>
