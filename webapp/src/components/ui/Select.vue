<script setup lang="ts">
import { inject } from "vue";
import { formFieldKey } from "@/lib/formField";
const field = inject(formFieldKey, null);
defineProps<{
  id?: string;
  placeholder?: string;
  options: { value: string | number; label: string }[];
}>();

const model = defineModel<string | number | null>();
</script>

<template>
  <select
    :id="id ?? field?.id.value"
    :aria-labelledby="field?.labelId"
    :aria-describedby="field?.describedBy.value"
    :aria-invalid="field?.invalid.value || undefined"
    v-model="model"
    class="min-h-11 w-full cursor-pointer rounded-md border border-control bg-surface-2 px-3 text-sm text-text outline-none transition-colors duration-200 hover:border-accent focus:border-accent focus:bg-surface-3 focus:ring-2 focus:ring-accent/20"
  >
    <option v-if="model == null" :value="model" disabled>{{ placeholder ?? "Choose an option…" }}</option>
    <option v-for="opt in options" :key="opt.value" :value="opt.value">
      {{ opt.label }}
    </option>
  </select>
</template>
