<script setup lang="ts">
import { computed } from "vue";

// Inline telemetry sparkline. Inherits color from `currentColor`, so the parent
// sets the trend hue with a text color. Dimensions are fixed numbers (not
// Tailwind arbitrary classes) so the geometry never depends on JIT generation.
const props = withDefaults(
  defineProps<{
    values: number[];
    width?: number;
    height?: number;
    area?: boolean;
  }>(),
  { width: 104, height: 30, area: true },
);

const PAD = 3;

const geo = computed(() => {
  const vals = props.values.length ? props.values : [0, 0];
  const n = vals.length;
  const min = Math.min(...vals);
  const max = Math.max(...vals);
  const span = max - min || 1;
  const w = props.width;
  const h = props.height;
  const x = (i: number) => (n === 1 ? w / 2 : PAD + (i / (n - 1)) * (w - PAD * 2));
  const y = (v: number) => h - PAD - ((v - min) / span) * (h - PAD * 2);
  const line = vals.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  const lastX = (n === 1 ? w / 2 : w - PAD).toFixed(1);
  return {
    line,
    fill: `${PAD},${h - PAD} ${line} ${lastX},${h - PAD}`,
    cx: x(n - 1).toFixed(1),
    cy: y(vals[n - 1]).toFixed(1),
  };
});
</script>

<template>
  <svg :width="width" :height="height" :viewBox="`0 0 ${width} ${height}`" fill="none" aria-hidden="true">
    <polygon v-if="area" :points="geo.fill" fill="currentColor" fill-opacity="0.12" />
    <polyline
      :points="geo.line"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linejoin="round"
      stroke-linecap="round"
    />
    <circle :cx="geo.cx" :cy="geo.cy" r="2.1" fill="currentColor" />
  </svg>
</template>
