<script setup lang="ts">
// Port of pw-card: surface panel with optional header (icon + title + actions)
defineProps<{
  title?: string;
  muted?: boolean;
}>();
</script>

<template>
  <section
    class="rounded-md border border-line shadow-sm"
    :class="muted ? 'bg-surface/70' : 'bg-surface'"
  >
    <header
      v-if="title || $slots.header || $slots.actions"
      class="flex min-h-12 flex-wrap items-center gap-2.5 border-b border-line bg-surface-2/35 px-4 py-3"
    >
      <slot name="header">
        <h2 class="text-base font-semibold tracking-tight">{{ title }}</h2>
      </slot>
      <div v-if="$slots.actions" class="ml-auto flex flex-wrap items-center gap-2">
        <slot name="actions" />
      </div>
    </header>
    <div class="p-4">
      <slot />
    </div>
  </section>
</template>
