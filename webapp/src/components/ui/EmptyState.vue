<script setup lang="ts">
// Centered empty state: icon, headline, supporting copy, optional CTA slot.
import Icon from "@/components/ui/Icon.vue";

withDefaults(
  defineProps<{
    icon?: string;
    title: string;
    message?: string;
  }>(),
  { icon: "info" },
);
</script>

<template>
  <div class="flex flex-col items-center justify-center rounded-md border border-dashed border-line bg-surface/40 px-6 py-10 text-center">
    <div class="mb-3 grid size-12 place-items-center rounded-full border border-line bg-surface-2 text-dim">
      <Icon :name="icon" :size="22" />
    </div>
    <h3 class="text-sm font-bold text-text">{{ title }}</h3>
    <p v-if="message" class="mt-1 max-w-sm text-sm text-muted">{{ message }}</p>
    <div v-if="$slots.default" class="mt-4 flex flex-wrap items-center justify-center gap-2">
      <slot />
    </div>
  </div>
</template>
