<script setup lang="ts">
// Port of pw-card: surface panel with optional header (icon + title + actions)
defineProps<{
  title?: string;
}>();
</script>

<template>
  <section class="rounded-md border border-line bg-surface">
    <header
      v-if="title || $slots.header || $slots.actions"
      class="flex items-center gap-2.5 border-b border-line px-4 py-3"
    >
      <slot name="header">
        <h2 class="text-sm font-semibold">{{ title }}</h2>
      </slot>
      <div v-if="$slots.actions" class="ml-auto flex items-center gap-2">
        <slot name="actions" />
      </div>
    </header>
    <div class="p-4">
      <slot />
    </div>
  </section>
</template>
