<script setup lang="ts">
defineProps<{
  title: string;
  subtitle?: string;
  eyebrow?: string;
  icon?: string;
}>();
</script>

<template>
  <header class="pitlane-heading">
    <div v-if="$slots.prefix" class="pitlane-heading-back">
      <slot name="prefix" />
    </div>
    <div class="min-w-0 flex-1 basis-64">
      <span v-if="eyebrow" class="pitlane-eyebrow">{{ eyebrow }}</span>
      <h1 class="text-text">{{ title }}</h1>
      <p v-if="subtitle">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.actions" class="pitlane-actions">
      <slot name="actions" />
    </div>
  </header>
</template>
