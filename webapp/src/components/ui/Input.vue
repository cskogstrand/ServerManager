<script setup lang="ts">
const props = withDefaults(
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
    :id="id"
    :value="model ?? ''"
    :type="type"
    :placeholder="placeholder"
    :min="min"
    :max="max"
    :autocomplete="autocomplete"
    class="w-full rounded-md border border-line bg-surface-2 px-3 py-1.5 text-sm outline-none placeholder:text-dim focus:border-accent"
    @input="onInput"
  />
</template>
