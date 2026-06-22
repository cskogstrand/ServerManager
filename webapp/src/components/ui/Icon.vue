<script setup lang="ts">
import { computed } from "vue";

const paths = {
  activity: ["M3 12h4l3-8 4 16 3-8h4"],
  alert: ["M12 4 2 20h20L12 4Z", "M12 10v5", "M12 18h.01"],
  arrowDown: ["M12 5v14", "m19 12-7 7-7-7"],
  broadcast: [
    "M12 13a1 1 0 1 0 0-2 1 1 0 0 0 0 2Z",
    "M8.1 8.1a5.5 5.5 0 0 0 0 7.8",
    "M15.9 15.9a5.5 5.5 0 0 0 0-7.8",
    "M5.3 5.3a9.5 9.5 0 0 0 0 13.4",
    "M18.7 18.7a9.5 9.5 0 0 0 0-13.4",
  ],
  maximize: ["M4 9V4h5", "M20 9V4h-5", "M4 15v5h5", "M20 15v5h-5"],
  minimize: ["M9 4v5H4", "M15 4v5h5", "M9 20v-5H4", "M15 20v-5h5"],
  arrowUp: ["M12 19V5", "m5 12 7-7 7 7"],
  calendar: ["M5 6h14v14H5z", "M5 10h14", "M9 4v4", "M15 4v4"],
  car: ["M5 16h14l-1.5-5.5a3 3 0 0 0-2.9-2.2H9.4a3 3 0 0 0-2.9 2.2L5 16Z", "M7 16v2", "M17 16v2", "M7.5 13h9"],
  check: ["m5 12 4 4L19 6"],
  clock: ["M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18Z", "M12 7v5l3 2"],
  content: ["M4 7h16", "M5 7l1.4 12h11.2L19 7", "M9 7V5h6v2"],
  copy: ["M9 9h10v10H9z", "M5 15V5h10"],
  dashboard: ["M4 5h7v6H4z", "M13 5h7v4h-7z", "M13 11h7v8h-7z", "M4 13h7v6H4z"],
  difficulty: ["M5 16a7 7 0 0 1 14 0", "M12 16l4-5", "M8 20h8"],
  edit: ["M4 20h4L18.5 9.5a2.1 2.1 0 0 0-3-3L5 17v3Z", "M13.5 6.5l3 3"],
  events: ["M6 21V4", "M6 5h11l-2 4 2 4H6"],
  folder: ["M4 7h5l2 2h9v9a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V7Z"],
  info: ["M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z", "M12 11v5", "M12 8h.01"],
  instances: ["M5 5h14v5H5z", "M5 14h14v5H5z", "M8 7h.01", "M8 16h.01"],
  lock: ["M6 11h12v9H6z", "M9 11V8a3 3 0 0 1 6 0v3"],
  logOut: ["M10 17l5-5-5-5", "M15 12H3", "M21 5v14"],
  menu: ["M4 7h16", "M4 12h16", "M4 17h16"],
  play: ["M7 5l12 7-12 7V5Z"],
  plus: ["M12 5v14", "M5 12h14"],
  power: ["M12 3v8", "M7 6.8a8 8 0 1 0 10 0"],
  queue: ["M8 6h13", "M8 12h13", "M8 18h13", "M3 6h.01", "M3 12h.01", "M3 18h.01"],
  repeat: ["m17 2 4 4-4 4", "M3 11V9a4 4 0 0 1 4-4h14", "m7 22-4-4 4-4", "M21 13v2a4 4 0 0 1-4 4H3"],
  search: ["M11 11a5 5 0 1 0 0-10 5 5 0 0 0 0 10Z", "m20 20-5.5-5.5"],
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
  trophy: [
    "M6 9H4.5a2.5 2.5 0 0 1 0-5H6",
    "M18 9h1.5a2.5 2.5 0 0 0 0-5H18",
    "M4 22h16",
    "M10 14.7V17c0 .6-.5 1-1 1.2C7.9 18.8 7 20.2 7 22",
    "M14 14.7V17c0 .6.5 1 1 1.2C16.1 18.8 17 20.2 17 22",
    "M6 2h12v7a6 6 0 0 1-12 0V2Z",
  ],
  flag: ["M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1Z", "M4 22V4"],
  star: ["M12 3l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9 6.8 19.2l1-5.8L3.5 9.2l5.9-.9L12 3Z"],
  camera: [
    "M14.5 5h-5L7.5 7.5H4a1.5 1.5 0 0 0-1.5 1.5v8A1.5 1.5 0 0 0 4 18.5h16a1.5 1.5 0 0 0 1.5-1.5V9A1.5 1.5 0 0 0 20 7.5h-3.5L14.5 5Z",
    "M12 16a3.2 3.2 0 1 0 0-6.4 3.2 3.2 0 0 0 0 6.4Z",
  ],
  film: ["M3.5 4.5h17v15h-17z", "M8 4.5v15", "M16 4.5v15", "M3.5 9.5h4.5", "M16 9.5h4.5", "M3.5 14.5h4.5", "M16 14.5h4.5"],
  mapPin: ["M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0Z", "M12 12.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z"],
  gauge: ["M3.6 18.4a10 10 0 1 1 16.8 0", "m12 14 4-4"],
  upload: ["M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4", "M17 8l-5-5-5 5", "M12 3v12"],
  download: ["M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4", "M7 10l5 5 5-5", "M12 15V3"],
  arrowLeft: ["M19 12H5", "m12 19-7-7 7-7"],
  externalLink: ["M15 3h6v6", "M10 14 21 3", "M21 14v5a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5"],
  record: ["M12 20a8 8 0 1 0 0-16 8 8 0 0 0 0 16Z", "M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z"],
  sun: [
    "M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Z",
    "M12 2v2",
    "M12 20v2",
    "M4 12H2",
    "M22 12h-2",
    "M5.6 5.6 4.2 4.2",
    "M19.8 19.8l-1.4-1.4",
    "M18.4 5.6l1.4-1.4",
    "M4.2 19.8l1.4-1.4",
  ],
  moon: ["M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z"],
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
