<script setup lang="ts">
// Port of pw-form-row / pw-label / pw-hint
defineProps<{
  label: string;
  hint?: string;
  forId?: string;
}>();
</script>

<template>
  <div class="mb-3.5">
    <label
      :for="forId"
      class="mb-1.5 block text-xs font-bold tracking-wide text-muted uppercase"
    >
      {{ label }}
    </label>
    <slot />
    <p v-if="hint" class="mt-1 text-xs text-dim">{{ hint }}</p>
  </div>
</template>
