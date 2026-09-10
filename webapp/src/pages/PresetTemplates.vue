<script setup lang="ts">
import { computed } from "vue";
import { useAuthStore } from "@/stores/auth";
import PageHeader from "@/components/ui/PageHeader.vue";
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
    {
      to: "/presets/drift-scoring",
      label: "Drift Scoring",
      detail: "Reusable scoring modes, reset rules, multipliers and weights.",
      icon: "gauge",
      visible: auth.canOperate,
    },
  ].filter((t) => t.visible),
);
</script>

<template>
  <PageHeader
    title="Templates"
    subtitle="Reusable preset building blocks for race setups. Most changes affect every race setup using the template."
    icon="settings"
  />

  <nav class="workspace-tabs" aria-label="Race preparation"><RouterLink to="/events">Race setups</RouterLink><RouterLink to="/presets" aria-current="page">Templates</RouterLink><RouterLink to="/queue">Run plan</RouterLink></nav>
  <div class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
    <RouterLink v-for="(t, index) in templates" :key="t.to" :to="t.to" class="template-tile group">
      <div class="track-art flex h-28 items-center justify-between px-6"><span class="self-start pt-4 font-mono text-[10px] text-muted">{{ String(index + 1).padStart(2, '0') }} / PRESET</span><Icon :name="t.icon" :size="42" class="text-text/60" /></div>
      <div class="p-6"><h2 class="text-lg font-medium tracking-tight">{{ t.label }}</h2><p class="mt-2 min-h-12 text-sm leading-relaxed text-muted">{{ t.detail }}</p><span class="mt-5 inline-flex items-center gap-2 text-xs font-medium text-accent">Explore presets<Icon name="arrowUp" :size="14" class="rotate-90" /></span></div>
    </RouterLink>
  </div>
</template>
