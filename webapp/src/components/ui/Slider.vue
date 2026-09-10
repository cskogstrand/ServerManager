<script setup lang="ts">
import { inject } from "vue";
import { formFieldKey } from "@/lib/formField";
const field = inject(formFieldKey, null);
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    min: number;
    max: number;
    step?: number | string;
    decimals?: number;
    suffix?: string;
  }>(),
  { step: 1, decimals: 0, suffix: "" },
);

const model = defineModel<number | null | undefined>();

const rangeValue = computed(() => {
  const v = typeof model.value === "number" ? model.value : props.min;
  return Math.min(props.max, Math.max(props.min, v));
});

function clean(v: number) {
  return props.decimals > 0 ? Number(v.toFixed(props.decimals)) : Math.round(v);
}

function setValue(raw: string) {
  model.value = raw === "" ? null : clean(Number(raw));
}
</script>

<template>
  <div class="flex items-center gap-3">
    <input
      type="range"
      :id="field?.id.value"
      :aria-labelledby="field?.labelId"
      :aria-describedby="field?.describedBy.value"
      :min="min"
      :max="max"
      :step="step"
      :value="rangeValue"
      class="h-11 min-w-0 flex-1 cursor-pointer accent-accent"
      @input="setValue(($event.target as HTMLInputElement).value)"
    />
    <div class="flex w-28 shrink-0 items-center gap-1.5">
      <input
        type="number"
        :aria-labelledby="field?.labelId"
        :aria-describedby="field?.describedBy.value"
        :min="min"
        :max="max"
        :step="step"
        :value="model ?? ''"
        class="min-h-11 w-full rounded-md border border-control bg-input px-2 text-sm text-text outline-none transition-colors duration-200 hover:border-line-hi focus:border-accent focus:bg-input focus:ring-2 focus:ring-accent/20"
        @input="setValue(($event.target as HTMLInputElement).value)"
      />
      <span v-if="suffix" class="w-8 shrink-0 text-xs text-dim">{{ suffix }}</span>
    </div>
  </div>
</template>
