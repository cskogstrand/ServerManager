<script setup lang="ts">
import { computed } from "vue";

const paths = {
  activity: ["M3 12h4l3-8 4 16 3-8h4"],
  arrowDown: ["M12 5v14", "m19 12-7 7-7-7"],
  arrowUp: ["M12 19V5", "m5 12 7-7 7 7"],
  car: ["M5 16h14l-1.5-5.5a3 3 0 0 0-2.9-2.2H9.4a3 3 0 0 0-2.9 2.2L5 16Z", "M7 16v2", "M17 16v2", "M7.5 13h9"],
  check: ["m5 12 4 4L19 6"],
  clock: ["M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18Z", "M12 7v5l3 2"],
  content: ["M4 7h16", "M5 7l1.4 12h11.2L19 7", "M9 7V5h6v2"],
  dashboard: ["M4 5h7v6H4z", "M13 5h7v4h-7z", "M13 11h7v8h-7z", "M4 13h7v6H4z"],
  difficulty: ["M5 16a7 7 0 0 1 14 0", "M12 16l4-5", "M8 20h8"],
  events: ["M6 21V4", "M6 5h11l-2 4 2 4H6"],
  info: ["M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z", "M12 11v5", "M12 8h.01"],
  instances: ["M5 5h14v5H5z", "M5 14h14v5H5z", "M8 7h.01", "M8 16h.01"],
  logOut: ["M10 17l5-5-5-5", "M15 12H3", "M21 5v14"],
  menu: ["M4 7h16", "M4 12h16", "M4 17h16"],
  plus: ["M12 5v14", "M5 12h14"],
  power: ["M12 3v8", "M7 6.8a8 8 0 1 0 10 0"],
  queue: ["M8 6h13", "M8 12h13", "M8 18h13", "M3 6h.01", "M3 12h.01", "M3 18h.01"],
  settings: ["M4 8h10", "M18 8h2", "M16 6v4", "M4 16h2", "M10 16h10", "M8 14v4"],
  shuffle: ["M16 3h5v5", "M4 7h4l8 10h5", "M21 16v5h-5", "M4 17h4l2.5-3"],
  skip: ["M5 6l7 6-7 6V6Z", "M14 6l7 6-7 6V6Z"],
  stop: ["M7 7h10v10H7z"],
  terminal: ["m5 7 5 5-5 5", "M12 17h7"],
  trash: ["M4 7h16", "M9 7V5h6v2", "M7 7l1 14h8l1-14"],
  user: ["M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Z", "M4 21a8 8 0 0 1 16 0"],
  users: ["M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8Z", "M17 11a3 3 0 1 0 0-6", "M3 21a7 7 0 0 1 12 0", "M15 19a5 5 0 0 1 6 2"],
  weather: ["M12 4v2", "M5.6 7.6 7 9", "M18.4 7.6l-1.4 1.4", "M4 14a4 4 0 0 1 6.9-2.8A5 5 0 0 1 20 14.5 3.5 3.5 0 0 1 16.5 18H7a3 3 0 0 1-3-4Z"],
  x: ["M6 6l12 12", "M18 6 6 18"],
} as const;

const props = withDefaults(
  defineProps<{
    name: string;
    size?: number;
  }>(),
  { size: 18 },
);

const iconPaths = computed(() => paths[props.name as keyof typeof paths] ?? paths.info);
</script>

<template>
  <svg
    :width="props.size"
    :height="props.size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path v-for="d in iconPaths" :key="d" :d="d" />
  </svg>
</template>
