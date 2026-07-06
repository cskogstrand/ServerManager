<script setup lang="ts">
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
      :min="min"
      :max="max"
      :step="step"
      :value="rangeValue"
      class="h-2 min-w-0 flex-1 cursor-pointer accent-accent"
      @input="setValue(($event.target as HTMLInputElement).value)"
    />
    <div class="flex w-28 shrink-0 items-center gap-1.5">
      <input
        type="number"
        :min="min"
        :max="max"
        :step="step"
        :value="model ?? ''"
        class="min-h-9 w-full rounded-md border border-line bg-surface-2 px-2 text-sm text-text outline-none transition-colors duration-200 hover:border-line-hi focus:border-accent focus:bg-surface-3 focus:ring-2 focus:ring-accent/20"
        @input="setValue(($event.target as HTMLInputElement).value)"
      />
      <span v-if="suffix" class="w-8 shrink-0 text-xs text-dim">{{ suffix }}</span>
    </div>
  </div>
</template>
