<script setup lang="ts">
import { computed } from "vue";
import { useAuthStore } from "@/stores/auth";
import PageHeader from "@/components/ui/PageHeader.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";

const auth = useAuthStore();

const templates = computed(() =>
  [
    {
      to: "/presets/classes",
      label: "Car Classes",
      detail: "Reusable grids: cars, skins, ballast and counts.",
      icon: "car",
      visible: auth.canOperate,
    },
    {
      to: "/presets/sessions",
      label: "Sessions",
      detail: "Booking, practice, qualify, race length and race rules.",
      icon: "clock",
      visible: auth.isAdmin,
    },
    {
      to: "/presets/time",
      label: "Time & Weather",
      detail: "Weather panels, temperatures, wind and CSP time/date.",
      icon: "weather",
      visible: auth.isAdmin,
    },
    {
      to: "/presets/difficulty",
      label: "Difficulty",
      detail: "Assists, realism, penalties, dynamic track and voting.",
      icon: "difficulty",
      visible: auth.isAdmin,
    },
  ].filter((t) => t.visible),
);
</script>

<template>
  <PageHeader
    title="Advanced Templates"
    subtitle="Reusable preset building blocks for race setups. Most changes affect every race setup using the template."
    icon="settings"
  />

  <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
    <Card v-for="t in templates" :key="t.to">
      <template #header>
        <Icon :name="t.icon" :size="16" class="text-accent" />
        <h2 class="text-sm font-bold">{{ t.label }}</h2>
      </template>
      <p class="mb-3 text-sm text-muted">{{ t.detail }}</p>
      <RouterLink :to="t.to">
        <Button variant="dark" size="sm">
          Open
          <Icon name="arrowUp" :size="14" class="rotate-90" />
        </Button>
      </RouterLink>
    </Card>
  </div>
</template>
