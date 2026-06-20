<script setup lang="ts">
// Native WHEP video player. Replaces the iframe embed for MediaMTX WebRTC
// channels: sub-second latency, real <video> (so fullscreen / PiP / mute are
// native), no sandboxed third-party page. Hand it a WHEP endpoint URL.
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import Icon from "@/components/ui/Icon.vue";
import { useWhep } from "@/lib/useWhep";

const props = defineProps<{ url: string }>();
const emit = defineEmits<{ playing: []; error: [message: string] }>();

const video = ref<HTMLVideoElement | null>(null);
const muted = ref(true); // autoplay policy: must start muted
const { state, error, connect, disconnect } = useWhep();

onMounted(() => {
  if (video.value) {
    video.value.muted = true;
    if (props.url) void connect(video.value, props.url);
  }
});

// Reconnect when the active channel switches to a different WHEP URL.
watch(
  () => props.url,
  (url) => {
    if (video.value && url) void connect(video.value, url);
  },
);

watch(state, (s) => {
  if (s === "playing") emit("playing");
  else if (s === "error") emit("error", error.value ?? "stream error");
});

onBeforeUnmount(() => void disconnect());

function toggleMute() {
  muted.value = !muted.value;
  if (video.value) video.value.muted = muted.value;
}
</script>

<template>
  <div class="relative size-full overflow-hidden rounded-lg bg-black">
    <video ref="video" class="size-full object-contain" autoplay playsinline />

    <!-- Connecting / error overlays -->
    <div
      v-if="state === 'connecting'"
      class="absolute inset-0 grid place-items-center bg-black/40"
    >
      <span class="animate-pulse font-mono text-xs uppercase tracking-wide text-dim">Connecting…</span>
    </div>
    <div
      v-else-if="state === 'error'"
      class="absolute inset-0 grid place-items-center bg-black/60 px-6 text-center"
    >
      <div class="flex flex-col items-center gap-2">
        <Icon name="alert" :size="22" class="text-danger" />
        <span class="font-mono text-xs text-danger">{{ error }}</span>
        <button
          type="button"
          class="mt-1 rounded-md border border-line bg-surface/70 px-3 py-1 text-xs text-muted transition-colors hover:border-line-hi hover:text-text"
          @click="video && connect(video, props.url)"
        >
          Retry
        </button>
      </div>
    </div>

    <!-- Mute toggle (shown once playing) -->
    <button
      v-if="state === 'playing'"
      type="button"
      class="absolute bottom-3 right-3 rounded-md border border-line bg-black/50 px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted backdrop-blur transition-colors hover:border-line-hi hover:text-text"
      :aria-label="muted ? 'Unmute' : 'Mute'"
      @click="toggleMute"
    >
      {{ muted ? "Unmute" : "Mute" }}
    </button>
  </div>
</template>
