<script setup lang="ts">
import { computed, provide, useId } from "vue";
import { formFieldKey } from "@/lib/formField";

const props = defineProps<{ label: string; hint?: string; forId?: string; error?: string; required?: boolean }>();
const generatedId = useId();
const id = computed(() => props.forId || generatedId);
const labelId = `${generatedId}-label`;
const hintId = `${generatedId}-hint`;
const errorId = `${generatedId}-error`;
const describedBy = computed(() => [props.hint && hintId, props.error && errorId].filter(Boolean).join(" ") || undefined);
provide(formFieldKey, { id, labelId, describedBy, invalid: computed(() => !!props.error) });
</script>

<template>
  <div class="mb-5">
    <label :id="labelId" :for="id" class="mb-2 block text-sm font-medium text-text">
      {{ label }}<span v-if="required" class="ml-1 text-muted">(required)</span>
    </label>
    <slot :id="id" :described-by="describedBy" />
    <p v-if="hint" :id="hintId" class="mt-2 text-xs leading-relaxed text-muted">{{ hint }}</p>
    <p v-if="error" :id="errorId" role="alert" class="mt-2 text-sm text-danger">{{ error }}</p>
  </div>
</template>
