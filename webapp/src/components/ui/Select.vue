<script setup lang="ts">
defineProps<{
  id?: string;
  options: { value: string | number; label: string }[];
}>();

const model = defineModel<string | number | null>();
</script>

<template>
  <select
    :id="id"
    v-model="model"
    class="min-h-9 w-full cursor-pointer rounded-md border border-line bg-surface-2 px-3 text-sm text-text outline-none transition-colors duration-200 hover:border-line-hi focus:border-accent focus:bg-surface-3 focus:ring-2 focus:ring-accent/20"
  >
    <option v-for="opt in options" :key="opt.value" :value="opt.value">
      {{ opt.label }}
    </option>
  </select>
</template>
