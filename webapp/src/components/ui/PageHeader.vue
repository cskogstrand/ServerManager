<script setup lang="ts">
import Icon from "@/components/ui/Icon.vue";

defineProps<{
  title: string;
  subtitle?: string;
  icon?: string;
}>();
</script>

<template>
  <header class="mb-5 flex flex-wrap items-start gap-3">
    <div v-if="$slots.prefix" class="shrink-0">
      <slot name="prefix" />
    </div>
    <div
      v-if="icon"
      class="grid size-10 shrink-0 place-items-center rounded-md border border-line bg-surface-2 text-accent"
    >
      <Icon :name="icon" :size="19" />
    </div>
    <div class="min-w-0 flex-1 basis-64">
      <h1 class="text-2xl font-bold tracking-tight text-text">{{ title }}</h1>
      <p v-if="subtitle" class="mt-0.5 max-w-3xl text-sm text-muted">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.actions" class="ml-auto flex flex-wrap items-center justify-end gap-2">
      <slot name="actions" />
    </div>
  </header>
</template>
