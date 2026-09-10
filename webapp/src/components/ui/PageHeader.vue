<script setup lang="ts">
import Icon from "@/components/ui/Icon.vue";

defineProps<{
  title: string;
  subtitle?: string;
  icon?: string;
}>();
</script>

<template>
  <header class="page-header">
    <div v-if="$slots.prefix" class="shrink-0">
      <slot name="prefix" />
    </div>
    <div class="min-w-0 flex-1 basis-64">
      <div class="flex items-center gap-3"><h1 class="text-text">{{ title }}</h1><Icon v-if="icon" :name="icon" :size="20" class="shrink-0 text-dim" /></div>
      <p v-if="subtitle">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.actions" class="ml-auto flex flex-wrap items-center justify-end gap-2">
      <slot name="actions" />
    </div>
  </header>
</template>
