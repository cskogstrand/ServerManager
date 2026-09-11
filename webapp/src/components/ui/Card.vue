<script setup lang="ts">
// Shared Pitlane panel with an optional header and actions.
defineProps<{
  title?: string;
  muted?: boolean;
}>();
</script>

<template>
  <section
    class="pitlane-panel"
    :class="muted ? 'bg-surface/70' : 'bg-surface'"
  >
    <header
      v-if="title || $slots.header || $slots.actions"
      class="pitlane-panel-heading"
    >
      <slot name="header">
        <h2 class="text-base font-semibold tracking-tight">{{ title }}</h2>
      </slot>
      <div v-if="$slots.actions" class="ml-auto flex flex-wrap items-center gap-2">
        <slot name="actions" />
      </div>
    </header>
    <div class="pitlane-panel-body">
      <slot />
    </div>
  </section>
</template>
