<script setup lang="ts">
// Port of pw-toggle: labelled switch bound via v-model
defineProps<{
  label?: string;
  disabled?: boolean;
}>();

const model = defineModel<boolean>({ required: true });
</script>

<template>
  <label
    class="flex cursor-pointer items-center gap-2.5 select-none"
    :class="disabled && 'cursor-not-allowed opacity-50'"
  >
    <button
      type="button"
      role="switch"
      :aria-checked="model"
      :disabled="disabled"
      class="relative h-5 w-9 shrink-0 cursor-pointer rounded-full border transition-colors duration-200 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:cursor-not-allowed"
      :class="model ? 'border-ok/60 bg-ok-glow' : 'border-line bg-surface-2'"
      @click="model = !model"
    >
      <span
        class="absolute top-0.5 size-3.5 rounded-full transition-all duration-200"
        :class="model ? 'left-[18px] bg-ok shadow-[0_0_12px_rgba(79,216,132,0.35)]' : 'left-0.5 bg-muted'"
      />
    </button>
    <span v-if="label" class="text-sm text-text">{{ label }}</span>
  </label>
</template>
