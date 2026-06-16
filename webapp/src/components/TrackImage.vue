<script setup lang="ts">
// Track thumbnail: the preview photo with the track's map layout overlaid on
// top. The layout prefers the in-game minimap (map.png) and falls back to the
// schematic outline (outline.png); it's rendered as a white silhouette with a
// drop shadow so it stays legible over any preview photo.
import { computed, ref, watch } from "vue";

const props = defineProps<{
  trackKey?: string | null;
  config?: string | null;
  // Set false for tiny thumbs where a layout overlay would just be noise.
  overlay?: boolean;
}>();

function url(kind: "preview" | "map" | "outline"): string {
  if (!props.trackKey) return "";
  const cfg = props.config ? `/${encodeURIComponent(props.config)}` : "";
  return `/api/track/${kind}/${encodeURIComponent(props.trackKey)}${cfg}`;
}

const previewUrl = computed(() => url("preview"));
const previewOk = ref(true);
const overlaySrc = ref("");

// Reset both layers whenever the track changes; the overlay restarts at map.png.
watch(
  () => [props.trackKey, props.config],
  () => {
    previewOk.value = true;
    overlaySrc.value = props.overlay === false ? "" : url("map");
  },
  { immediate: true },
);

function onOverlayError() {
  // map.png missing → try outline.png; if that fails too, drop the overlay.
  overlaySrc.value = overlaySrc.value.includes("/map/") ? url("outline") : "";
}
</script>

<template>
  <div class="relative overflow-hidden">
    <img
      v-if="previewUrl && previewOk"
      :src="previewUrl"
      alt=""
      loading="lazy"
      class="size-full object-cover"
      @error="previewOk = false"
    />
    <img
      v-if="overlaySrc"
      :src="overlaySrc"
      alt=""
      loading="lazy"
      class="pointer-events-none absolute inset-[8%] size-[84%] object-contain opacity-90 invert brightness-0 drop-shadow-[0_1px_2px_rgba(0,0,0,0.85)]"
      @error="onOverlayError"
    />
  </div>
</template>
