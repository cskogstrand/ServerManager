<script setup lang="ts">
withDefaults(
  defineProps<{
    type?: string;
    id?: string;
    placeholder?: string;
    min?: number;
    max?: number;
    autocomplete?: string;
  }>(),
  { type: "text" },
);

const model = defineModel<string | number | null>();
</script>

<template>
  <input
    :id="id"
    v-model="model"
    :type="type"
    :placeholder="placeholder"
    :min="min"
    :max="max"
    :autocomplete="autocomplete"
    class="w-full rounded-md border border-line bg-surface-2 px-3 py-1.5 text-sm outline-none placeholder:text-dim focus:border-accent"
  />
</template>
