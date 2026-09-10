<script setup lang="ts">
import { inject } from "vue";
import { formFieldKey } from "@/lib/formField";
const field = inject(formFieldKey, null);
const props = withDefaults(
  defineProps<{
    type?: string;
    id?: string;
    placeholder?: string;
    min?: number;
    max?: number;
    step?: number | string;
    autocomplete?: string;
    required?: boolean;
  }>(),
  { type: "text" },
);

const model = defineModel<string | number | null | undefined>();

// type="number" must produce real numbers in the model — the Go API binds
// into *int and rejects quoted values.
function onInput(e: Event) {
  const v = (e.target as HTMLInputElement).value;
  model.value = props.type === "number" ? (v === "" ? null : Number(v)) : v;
}
</script>

<template>
  <input
    :id="id ?? field?.id.value"
    :aria-labelledby="field?.labelId"
    :aria-describedby="field?.describedBy.value"
    :aria-invalid="field?.invalid.value || undefined"
    :value="model ?? ''"
    :type="type"
    :placeholder="placeholder"
    :min="min"
    :max="max"
    :step="step"
    :autocomplete="autocomplete"
    :required="required"
    class="min-h-11 w-full rounded-md border border-control bg-surface-2 px-3 text-sm text-text outline-none transition-colors duration-200 placeholder:text-muted hover:border-accent focus:border-accent focus:bg-surface-3 focus:ring-2 focus:ring-accent/20"
    @input="onInput"
  />
</template>
