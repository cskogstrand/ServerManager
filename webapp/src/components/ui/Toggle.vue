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
      class="relative h-5 w-9 shrink-0 rounded-full border transition-colors"
      :class="model ? 'border-accent/50 bg-accent/30' : 'border-line bg-surface-2'"
      @click="model = !model"
    >
      <span
        class="absolute top-0.5 size-3.5 rounded-full transition-all"
        :class="model ? 'left-[18px] bg-accent' : 'left-0.5 bg-muted'"
      />
    </button>
    <span v-if="label" class="text-sm">{{ label }}</span>
  </label>
</template>
