<script setup lang="ts">
// Shared full-screen stream viewer. Plays one embedded WebRTC/player URL at a
// time and, when handed more than one channel, offers a chip strip to switch
// between them (driver cams, spectator cam). Teleports to body at a z-index
// above the broadcast overlay and side sheets so it works from every surface.
import { computed, ref, watch } from "vue";
import Icon from "@/components/ui/Icon.vue";
import WhepPlayer from "@/components/WhepPlayer.vue";
import type { StreamChannel, StreamHealthStatus } from "@/lib/useDriverStreams";

const props = defineProps<{
  open: boolean;
  channels: StreamChannel[];
  initialKey?: string | null;
}>();
const emit = defineEmits<{ close: [] }>();

const activeKey = ref<string | null>(null);

// On open (and when the requested channel changes) snap to the asked-for
// channel, falling back to the first available one.
watch(
  () => [props.open, props.initialKey] as const,
  ([open, key]) => {
    if (open) activeKey.value = key ?? props.channels[0]?.key ?? null;
  },
  { immediate: true },
);

const active = computed(
  () => props.channels.find((c) => c.key === activeKey.value) ?? props.channels[0] ?? null,
);

function healthLabel(h: StreamHealthStatus): string {
  return h === "live" ? "Live" : h === "offline" ? "Offline" : h === "unknown" ? "Status unknown" : "Not configured";
}
function healthDot(h: StreamHealthStatus): string {
  return h === "live" ? "bg-ok" : h === "offline" ? "bg-danger" : "bg-dim";
}
</script>

<template>
  <Teleport to="body">
    <Transition name="theater">
      <div
        v-if="open"
        class="fixed inset-0 z-60 flex flex-col bg-black/85 backdrop-blur-sm"
        @click.self="emit('close')"
      >
        <!-- Header: title, health, close -->
        <header class="flex min-h-14 shrink-0 items-center gap-3 px-4 py-3">
          <Icon name="broadcast" :size="18" class="shrink-0 text-accent" />
          <div v-if="active" class="min-w-0">
            <div class="truncate text-sm font-bold text-text">{{ active.title }}</div>
            <div v-if="active.subtitle" class="truncate font-mono text-[11px] text-dim">{{ active.subtitle }}</div>
          </div>
          <span
            v-if="active"
            class="ml-2 inline-flex shrink-0 items-center gap-1.5 rounded-full border border-line bg-surface/70 px-2 py-0.5 text-[11px] text-muted"
          >
            <span class="size-1.5 rounded-full" :class="healthDot(active.health)" />
            {{ healthLabel(active.health) }}
          </span>
          <button
            type="button"
            class="ml-auto grid size-9 shrink-0 place-items-center rounded-md border border-line bg-surface/70 text-muted transition-colors hover:border-danger/60 hover:text-danger"
            aria-label="Close stream"
            @click="emit('close')"
          >
            <Icon name="x" :size="16" />
          </button>
        </header>

        <!-- Player -->
        <div class="flex min-h-0 flex-1 items-center justify-center px-4">
          <!-- Native WHEP player (MediaMTX WebRTC): low-latency real <video> -->
          <div
            v-if="active && active.online && active.kind === 'whep'"
            class="aspect-video max-h-full w-full max-w-6xl overflow-hidden rounded-lg border border-line shadow-2xl"
          >
            <WhepPlayer :key="active.key" :url="active.url" />
          </div>
          <!-- Iframe embed fallback (external player pages) -->
          <iframe
            v-else-if="active && active.online"
            :key="active.key"
            :src="active.url"
            :title="active.title"
            class="aspect-video max-h-full w-full max-w-6xl rounded-lg border border-line bg-bg shadow-2xl"
            allow="autoplay; fullscreen; picture-in-picture"
            sandbox="allow-scripts allow-same-origin allow-forms allow-presentation"
          />
          <!-- Offline placeholder: stream configured but driver not connected -->
          <div
            v-else-if="active"
            class="grid aspect-video max-h-full w-full max-w-6xl place-items-center rounded-lg border border-line bg-bg shadow-2xl"
          >
            <div class="flex flex-col items-center gap-3 text-center">
              <div class="grid size-20 place-items-center rounded-full border border-line bg-surface/70 text-dim">
                <Icon name="user" :size="40" />
              </div>
              <div class="text-sm font-bold text-muted">{{ active.title }}</div>
              <div class="font-mono text-[11px] uppercase tracking-wide text-dim">Driver offline</div>
            </div>
          </div>
          <p v-else class="font-mono text-sm text-dim">No stream available.</p>
        </div>

        <!-- Channel switcher (only with more than one stream) -->
        <footer v-if="channels.length > 1" class="shrink-0 overflow-x-auto px-4 py-3">
          <div class="mx-auto flex w-max max-w-full gap-2">
            <button
              v-for="ch in channels"
              :key="ch.key"
              type="button"
              class="flex shrink-0 items-center gap-2 rounded-md border px-3 py-1.5 text-left text-xs transition-colors"
              :class="
                ch.key === activeKey
                  ? 'border-accent/60 bg-accent-dim text-text'
                  : 'border-line bg-surface/70 text-muted hover:border-line-hi hover:text-text'
              "
              @click="activeKey = ch.key"
            >
              <span class="size-1.5 shrink-0 rounded-full" :class="healthDot(ch.health)" />
              <span class="max-w-40 truncate font-semibold">{{ ch.title }}</span>
            </button>
          </div>
        </footer>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
/* Generated by <Transition name="theater"> at runtime. */
/*noinspection CssUnusedSymbol*/
.theater-enter-active,
.theater-leave-active {
  transition: opacity 0.18s ease;
}
/*noinspection CssUnusedSymbol*/
.theater-enter-from,
.theater-leave-to {
  opacity: 0;
}
</style>
