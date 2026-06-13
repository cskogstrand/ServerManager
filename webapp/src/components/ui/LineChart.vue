<script setup lang="ts">
// Dependency-free SVG line chart for overlaid curves (e.g. power + torque vs
// RPM). Each series is normalized to its own peak so curves with different
// units share the same frame; the peak value is annotated per series.
import { computed } from "vue";

interface Series {
  name: string;
  color: string;
  values: number[];
  unit?: string;
}

const props = withDefaults(
  defineProps<{
    series: Series[];
    labels: (number | string)[];
    height?: number;
    xLabel?: string;
  }>(),
  { height: 200 },
);

// Fixed internal coordinate system; the SVG scales to its container width.
const W = 600;
const PAD = { top: 16, right: 14, bottom: 26, left: 14 };

const innerW = W - PAD.left - PAD.right;
const innerH = computed(() => props.height - PAD.top - PAD.bottom);

const pointCount = computed(() => Math.max(...props.series.map((s) => s.values.length), 0));

function xAt(i: number): number {
  const n = pointCount.value;
  if (n <= 1) return PAD.left;
  return PAD.left + (i / (n - 1)) * innerW;
}

interface Plotted {
  name: string;
  color: string;
  unit: string;
  line: string;
  area: string;
  peak: { x: number; y: number; value: number };
}

const plotted = computed<Plotted[]>(() =>
  props.series
    .filter((s) => s.values.length)
    .map((s) => {
      const max = Math.max(...s.values, 1);
      const baseY = PAD.top + innerH.value;
      const pts = s.values.map((v, i) => ({ x: xAt(i), y: baseY - (v / max) * innerH.value, v }));
      const line = pts.map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(" ");
      const area = `${line} L${pts[pts.length - 1].x.toFixed(1)},${baseY} L${pts[0].x.toFixed(1)},${baseY} Z`;
      let peakIdx = 0;
      for (let i = 1; i < s.values.length; i++) if (s.values[i] > s.values[peakIdx]) peakIdx = i;
      return {
        name: s.name,
        color: s.color,
        unit: s.unit ?? "",
        line,
        area,
        peak: { x: pts[peakIdx].x, y: pts[peakIdx].y, value: Math.round(s.values[peakIdx]) },
      };
    }),
);

// A handful of evenly spaced x-axis ticks pulled from the supplied labels.
const ticks = computed(() => {
  const n = pointCount.value;
  if (n <= 1) return [];
  const want = Math.min(5, n);
  const out: { x: number; label: string }[] = [];
  for (let t = 0; t < want; t++) {
    const i = Math.round((t / (want - 1)) * (n - 1));
    out.push({ x: xAt(i), label: String(props.labels[i] ?? "") });
  }
  return out;
});

// Horizontal gridlines at 0/25/50/75/100% of the frame.
const gridLines = computed(() => {
  const base = PAD.top + innerH.value;
  return [0, 0.25, 0.5, 0.75, 1].map((f) => base - f * innerH.value);
});

const uid = `lc-${Math.round(props.height)}-${props.series.length}`;
</script>

<template>
  <figure class="m-0">
    <svg
      :viewBox="`0 0 ${W} ${height}`"
      :style="{ width: '100%', height: 'auto' }"
      preserveAspectRatio="none"
      role="img"
      :aria-label="series.map((s) => s.name).join(' and ') + ' curve'"
    >
      <defs>
        <linearGradient v-for="(p, i) in plotted" :id="`${uid}-g${i}`" :key="i" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" :stop-color="p.color" stop-opacity="0.22" />
          <stop offset="100%" :stop-color="p.color" stop-opacity="0" />
        </linearGradient>
      </defs>

      <!-- gridlines -->
      <line
        v-for="(y, i) in gridLines"
        :key="`g${i}`"
        :x1="PAD.left"
        :x2="W - PAD.right"
        :y1="y"
        :y2="y"
        stroke="var(--color-line)"
        stroke-width="1"
        :stroke-opacity="i === gridLines.length - 1 ? 0.9 : 0.4"
      />

      <!-- x ticks -->
      <text
        v-for="(t, i) in ticks"
        :key="`t${i}`"
        :x="t.x"
        :y="height - 8"
        text-anchor="middle"
        fill="var(--color-dim)"
        font-size="11"
        font-family="var(--font-mono)"
      >
        {{ t.label }}
      </text>

      <!-- series -->
      <g v-for="(p, i) in plotted" :key="`s${i}`">
        <path :d="p.area" :fill="`url(#${uid}-g${i})`" />
        <path :d="p.line" :stroke="p.color" stroke-width="2" fill="none" stroke-linejoin="round" stroke-linecap="round" />
        <circle :cx="p.peak.x" :cy="p.peak.y" r="3.5" :fill="p.color" stroke="var(--color-bg)" stroke-width="1.5" />
      </g>
    </svg>

    <figcaption class="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
      <span v-for="(p, i) in plotted" :key="`l${i}`" class="inline-flex items-center gap-1.5">
        <span class="size-2.5 rounded-full" :style="{ backgroundColor: p.color }" />
        <span class="text-muted">{{ p.name }}</span>
        <span class="font-mono text-text">{{ p.peak.value }}{{ p.unit }}</span>
      </span>
      <span v-if="xLabel" class="ml-auto text-dim">{{ xLabel }}</span>
    </figcaption>
  </figure>
</template>
