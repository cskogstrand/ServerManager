<script setup lang="ts">
// Grid of live driver-stream tiles. Online drivers play their embed inline;
// offline drivers show a placeholder. Each tile has a button to pop the stream
// full screen — the parent owns the StreamTheater and handles the `watch` event
// with the channel key. Used on the dashboard, server detail and broadcast.
import Icon from "@/components/ui/Icon.vue";
import WhepPlayer from "@/components/WhepPlayer.vue";
import type { StreamChannel, StreamHealthStatus } from "@/lib/useDriverStreams";

withDefaults(
  defineProps<{
    channels: StreamChannel[];
    // Tailwind grid-template-columns classes; override for narrow containers.
    gridClass?: string;
  }>(),
  { gridClass: "grid-cols-1 sm:grid-cols-2 xl:grid-cols-3" },
);

defineEmits<{ watch: [key: string] }>();

function healthDot(h: StreamHealthStatus): string {
  return h === "live" ? "bg-ok" : h === "offline" ? "bg-danger" : "bg-dim";
}
</script>

<template>
  <div class="grid gap-3" :class="gridClass">
    <div
      v-for="ch in channels"
      :key="ch.key"
      class="overflow-hidden rounded-md border border-line bg-surface shadow-[0_18px_45px_rgba(0,0,0,0.18)]"
    >
      <div class="relative aspect-video bg-bg">
        <WhepPlayer v-if="ch.online && ch.kind === 'whep'" :key="ch.key" :url="ch.url" minimal />
        <iframe
          v-else-if="ch.online"
          :src="ch.url"
          :title="ch.title"
          class="size-full border-0"
          allow="autoplay; fullscreen; picture-in-picture"
          sandbox="allow-scripts allow-same-origin allow-forms allow-presentation"
        />
        <div v-else class="grid size-full place-items-center">
          <div class="flex flex-col items-center gap-2 text-dim">
            <div class="grid size-12 place-items-center rounded-full border border-line bg-surface/70">
              <Icon name="user" :size="24" />
            </div>
            <span class="font-mono text-[11px] uppercase tracking-wide">Driver offline</span>
          </div>
        </div>
        <button
          type="button"
          class="absolute top-2 right-2 grid size-8 place-items-center rounded-md border border-line bg-bg/70 text-muted backdrop-blur transition-colors hover:border-accent/60 hover:text-accent"
          :aria-label="`Watch ${ch.title} full screen`"
          :title="`Watch ${ch.title} full screen`"
          @click="$emit('watch', ch.key)"
        >
          <Icon name="maximize" :size="14" />
        </button>
      </div>
      <div class="flex items-center gap-2 px-3 py-2">
        <span class="size-1.5 shrink-0 rounded-full" :class="healthDot(ch.health)" />
        <span class="min-w-0 flex-1 truncate text-sm font-semibold">{{ ch.title }}</span>
        <span v-if="ch.subtitle" class="shrink-0 truncate font-mono text-[11px] text-dim">{{ ch.subtitle }}</span>
      </div>
    </div>
  </div>
</template>
