<script setup lang="ts">
import { computed } from "vue";

// Placeholder driver avatar: a deterministic tinted monogram derived from the
// driver guid (so a driver keeps the same color everywhere) until a real photo
// is uploaded. Semantic colors adapt the monogram to both Paddock themes.
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

const TINTS = ["accent", "ok", "warn", "danger", "lilac", "info"];

const initials = computed(() => {
  const parts = props.name.trim().split(/\s+/).filter(Boolean);
  if (!parts.length) return "?";
  return parts.slice(0, 2).map((p) => p[0]?.toUpperCase() ?? "").join("") || "?";
});

const tint = computed(() => {
  const seed = props.guid || props.name;
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) >>> 0;
  const color = `var(--color-${TINTS[h % TINTS.length]})`;
  return { bg: `color-mix(in srgb, ${color} 12%, var(--color-surface))`, fg: color, ring: `color-mix(in srgb, ${color} 35%, transparent)` };
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
