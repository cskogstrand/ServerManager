<script setup lang="ts">
import { computed } from "vue";

// Placeholder driver avatar: a deterministic tinted monogram derived from the
// driver guid (so a driver keeps the same color everywhere) until a real photo
// is uploaded. Colors come from the muted OPS-SLATE status palette — no neon.
// Dynamic colors are applied inline, never as Tailwind arbitrary classes.
const props = withDefaults(
  defineProps<{
    name: string;
    guid?: string;
    src?: string | null;
    size?: number;
    radius?: "md" | "lg";
  }>(),
  { size: 44, radius: "md" },
);

const TINTS = [
  { bg: "rgba(98,179,232,0.16)", fg: "#a6d4f4", ring: "rgba(98,179,232,0.45)" },
  { bg: "rgba(79,216,132,0.15)", fg: "#8ce8b1", ring: "rgba(79,216,132,0.42)" },
  { bg: "rgba(240,185,90,0.16)", fg: "#f4d29a", ring: "rgba(240,185,90,0.45)" },
  { bg: "rgba(239,113,104,0.15)", fg: "#f3a39d", ring: "rgba(239,113,104,0.42)" },
  { bg: "rgba(150,140,232,0.16)", fg: "#c4bcf2", ring: "rgba(150,140,232,0.45)" },
  { bg: "rgba(96,202,202,0.15)", fg: "#9adede", ring: "rgba(96,202,202,0.42)" },
];

const initials = computed(() => {
  const parts = props.name.trim().split(/\s+/).filter(Boolean);
  if (!parts.length) return "?";
  return parts.slice(0, 2).map((p) => p[0]?.toUpperCase() ?? "").join("") || "?";
});

const tint = computed(() => {
  const seed = props.guid || props.name;
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) >>> 0;
  return TINTS[h % TINTS.length];
});
</script>

<template>
  <div
    class="relative grid shrink-0 place-items-center overflow-hidden font-bold"
    :class="radius === 'lg' ? 'rounded-lg' : 'rounded-md'"
    :style="{
      width: size + 'px',
      height: size + 'px',
      background: tint.bg,
      color: tint.fg,
      boxShadow: `inset 0 0 0 1.5px ${tint.ring}`,
      fontSize: Math.round(size * 0.36) + 'px',
    }"
  >
    <img v-if="src" :src="src" alt="" class="size-full object-cover" />
    <span v-else class="tracking-tight select-none">{{ initials }}</span>
  </div>
</template>
