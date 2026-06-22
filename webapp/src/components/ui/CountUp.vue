<script setup lang="ts">
// Animates an integer metric toward `value` with an ease-out cubic and renders
// it with locale thousands separators. Honors prefers-reduced-motion by
// snapping straight to the value. Used for the driver-detail hero KPIs.
import { onUnmounted, ref, toRef, watch } from "vue";

const props = defineProps<{ value: number }>();
const target = toRef(props, "value");
const durationMs = 850;
const display = ref(0);
const reduce =
  typeof window !== "undefined" && typeof window.matchMedia === "function"
    ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
    : false;

let raf = 0;
let from = 0;
let startTs = 0;

function frame(ts: number) {
  if (!startTs) startTs = ts;
  const p = Math.min(1, (ts - startTs) / durationMs);
  const eased = 1 - Math.pow(1 - p, 3);
  display.value = Math.round(from + (target.value - from) * eased);
  if (p < 1) raf = requestAnimationFrame(frame);
}

watch(
  target,
  (v) => {
    cancelAnimationFrame(raf);
    if (reduce) {
      display.value = v;
      return;
    }
    from = display.value;
    startTs = 0;
    raf = requestAnimationFrame(frame);
  },
  { immediate: true },
);

onUnmounted(() => cancelAnimationFrame(raf));
</script>

<template>
  <span>{{ display.toLocaleString() }}</span>
</template>
