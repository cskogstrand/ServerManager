<script setup lang="ts">
import { inject, useId } from "vue";
import { formFieldKey } from "@/lib/formField";
const props = defineProps<{ label?: string; disabled?: boolean }>();
const model = defineModel<boolean>({ required: true });
const field = inject(formFieldKey, null);
const generatedId = useId();
</script>

<template>
  <label :for="field?.id.value ?? generatedId" class="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-text" :class="disabled && 'cursor-not-allowed opacity-50'">
    <input :id="field?.id.value ?? generatedId" v-model="model" type="checkbox" role="switch" :disabled="disabled"
      :aria-label="props.label" :aria-labelledby="props.label ? undefined : field?.labelId" :aria-describedby="field?.describedBy.value"
      class="peer sr-only" />
    <span aria-hidden="true" class="flex h-6 w-11 shrink-0 items-center rounded-full border border-control bg-surface-4 p-0.5 transition-colors peer-checked:border-primary peer-checked:bg-primary peer-focus-visible:outline-2 peer-focus-visible:outline-offset-4 peer-focus-visible:outline-accent"><span class="size-4 rounded-full bg-text shadow-sm transition-transform" :class="model ? 'translate-x-5 !bg-on-primary' : 'translate-x-0'"></span></span>
    <span v-if="label">{{ label }}</span>
  </label>
</template>
