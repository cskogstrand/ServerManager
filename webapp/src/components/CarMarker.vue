<script setup lang="ts">
// Top-down car sprite for the live map, oriented by heading. Presentational:
// the parent positions it (mapPoint) and owns click/title; this draws the
// rotating car, an upright position number, the leader ring and an optional
// name caption. Heading comes from velocity (see createHeadingTracker), so it
// tracks direction of travel — there is no chassis-yaw in AC's UDP stream.
const props = defineProps<{
  label: string | number;
  heading: number;
  isLeader?: boolean;
  focused?: boolean;
  size?: number;
  name?: string;
}>();

const width = () => props.size ?? 26;
</script>

<template>
  <span
    class="relative grid place-items-center"
    :style="{ width: `${width()}px`, height: `${width() * 1.5}px` }"
  >
    <!-- leader pulse ring (kept circular regardless of the sprite's tall box) -->
    <span
      v-if="isLeader"
      class="car-ring absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full"
      :style="{ width: `${width()}px`, height: `${width()}px` }"
    />

    <!-- rotating top-down car sprite -->
    <svg
      class="car-sprite absolute inset-0 size-full drop-shadow-[0_1px_3px_rgba(0,0,0,0.7)]"
      :class="isLeader ? 'text-accent' : focused ? 'text-text' : 'text-surface-4'"
      :style="{ transform: `rotate(${heading}deg)` }"
      viewBox="0 0 28 44"
      fill="none"
    >
      <!-- body (nose at top = heading 0) -->
      <rect x="3" y="2" width="22" height="40" rx="8" fill="currentColor" stroke="var(--color-bg)" stroke-width="2" />
      <!-- windscreen -->
      <path d="M8 11 H20 L17.5 19 H10.5 Z" fill="rgba(0,0,0,0.45)" />
      <!-- rear window -->
      <path d="M10 31 H18 L19 37 H9 Z" fill="rgba(0,0,0,0.3)" />
    </svg>

    <!-- upright position number (does not rotate with the car) -->
    <span
      class="numerals relative text-[10px] leading-none font-bold tabular-nums drop-shadow-[0_1px_1px_rgba(0,0,0,0.8)]"
      :class="isLeader ? 'text-bg' : 'text-text'"
      >{{ label }}</span
    >

    <!-- optional name caption -->
    <span
      v-if="name"
      class="pointer-events-none absolute top-full left-1/2 mt-1 -translate-x-1/2 rounded-sm bg-bg/80 px-1.5 py-0.5 text-[10px] font-semibold whitespace-nowrap backdrop-blur-sm"
      >{{ name }}</span
    >
  </span>
</template>

<style scoped>
/* Glide between heading frames the same 0.3s the position transition uses. */
.car-sprite {
  transition: transform 0.3s linear;
}
.car-ring {
  border: 2px solid var(--color-accent);
  animation: car-ring 1.8s ease-out infinite;
}
@keyframes car-ring {
  0% {
    transform: translate(-50%, -50%) scale(0.7);
    opacity: 0.8;
  }
  100% {
    transform: translate(-50%, -50%) scale(1.9);
    opacity: 0;
  }
}
</style>
