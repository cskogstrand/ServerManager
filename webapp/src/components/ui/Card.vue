<script setup lang="ts">
// Shared Paddock surface with an optional header and actions.
defineProps<{
  title?: string;
  muted?: boolean;
}>();
</script>

<template>
  <section
    class="paddock-card"
    :class="muted ? 'bg-surface/70' : 'bg-surface'"
  >
    <header
      v-if="title || $slots.header || $slots.actions"
      class="paddock-card-header"
    >
      <slot name="header">
        <h2 class="text-base font-semibold tracking-tight">{{ title }}</h2>
      </slot>
      <div v-if="$slots.actions" class="ml-auto flex flex-wrap items-center gap-2">
        <slot name="actions" />
      </div>
    </header>
    <div class="paddock-card-body">
      <slot />
    </div>
  </section>
</template>
