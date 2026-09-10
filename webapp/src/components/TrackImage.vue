<script setup lang="ts">
// Track thumbnail: the preview photo with the track's map layout overlaid on
// top. The layout prefers the in-game minimap (map.png) and falls back to the
// schematic outline (outline.png); it's rendered as a white silhouette with a
// drop shadow so it stays legible over any preview photo.
import { computed, ref, watch } from "vue";
import { useContentStore } from "@/stores/content";
import Icon from "@/components/ui/Icon.vue";

const content = useContentStore();

// `overlay` defaults to true: a bare Boolean prop casts an absent attribute to
// `false`, which would silently hide the layout everywhere it isn't passed.
const props = withDefaults(
  defineProps<{
    trackKey?: string | null;
    config?: string | null;
    // Pass :overlay="false" for tiny thumbs where the layout would be noise.
    overlay?: boolean;
    variant?: "photo" | "map";
  }>(),
  { overlay: true, variant: "photo" },
);

function url(kind: "preview" | "map" | "outline"): string {
  if (!props.trackKey) return "";
  const cfg = props.config ? `/${encodeURIComponent(props.config)}` : "";
  return `/api/track/${kind}/${encodeURIComponent(props.trackKey)}${cfg}?v=${content.imageVersion}`;
}

const previewUrl = computed(() => url("preview"));
const previewOk = ref(true);

// Overlay layout asset: start at map.png, fall back to outline.png on error,
// then give up. A plain computed (mirroring previewUrl) so it renders on first
// paint with no reliance on a watch having fired.
const overlayKind = ref<"map" | "outline" | "none">("map");
const overlaySrc = computed(() =>
  props.overlay === false || overlayKind.value === "none" ? "" : url(overlayKind.value),
);

// New track → restart the overlay at map.png and re-show the preview.
watch(
  () => [props.trackKey, props.config],
  () => {
    previewOk.value = true;
    overlayKind.value = "map";
  },
);

function onOverlayError() {
  // map.png missing → try outline.png; if that fails too, drop the overlay.
  overlayKind.value = overlayKind.value === "map" ? "outline" : "none";
}
</script>

<template>
  <div class="relative overflow-hidden bg-surface-2" :class="{ 'track-art': variant === 'map' }">
    <img
      v-if="variant === 'photo' && previewUrl && previewOk"
      :src="previewUrl"
      alt=""
      loading="lazy"
      class="size-full object-cover"
      @error="previewOk = false"
    />
    <!-- Inline styles (not Tailwind arbitrary classes) so the overlay never
         depends on JIT class generation — those arbitrary utilities weren't
         landing in the global stylesheet. White silhouette + shadow reads on
         any preview photo. -->
    <img
      v-if="overlaySrc"
      :src="overlaySrc"
      alt=""
      loading="lazy"
      class="track-map pointer-events-none absolute object-contain"
      style="inset: 12%; width: 76%; height: 76%;"
      :style="variant === 'photo' ? { opacity: 0.9, filter: 'brightness(0) invert(1) drop-shadow(0 1px 2px rgba(0, 0, 0, 0.85))' } : undefined"
      @error="onOverlayError"
    />
    <div v-else-if="variant === 'map' || !previewUrl || !previewOk" class="absolute inset-0 grid place-items-center text-accent/60"><Icon name="events" :size="28" /></div>
  </div>
</template>
