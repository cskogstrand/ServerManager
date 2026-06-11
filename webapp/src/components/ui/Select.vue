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
    class="w-full rounded-md border border-line bg-surface-2 px-3 py-1.5 text-sm outline-none focus:border-accent"
  >
    <option v-for="opt in options" :key="opt.value" :value="opt.value">
      {{ opt.label }}
    </option>
  </select>
</template>
