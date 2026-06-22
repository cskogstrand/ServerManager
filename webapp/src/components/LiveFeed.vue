<script setup lang="ts">
// Dashboard "live feed": the most recent streamed events (sessions, laps, drift
// runs, media, server start/stop) newest-first. Purely a read view of the feed
// store — the SSE stream fills it; nothing here fetches.
import { computed } from "vue";
import { useFeedStore, type FeedTone } from "@/stores/feed";
import { timeAgo } from "@/lib/driversApi";
import Icon from "@/components/ui/Icon.vue";

const feed = useFeedStore();

// Show a compact window; the store keeps more for the watchers that drive
// per-driver refresh.
const items = computed(() => feed.items.slice(0, 12));

const toneClass: Record<FeedTone, string> = {
  info: "text-muted",
  ok: "text-ok",
  warn: "text-warn",
  accent: "text-accent",
  danger: "text-danger",
  dim: "text-dim",
};
</script>

<template>
  <section class="rounded-md border border-line bg-surface">
    <header class="flex items-center gap-2.5 border-b border-line px-4 py-3">
      <span class="relative flex size-2">
        <span class="absolute inline-flex size-full animate-ping rounded-full bg-ok opacity-70" />
        <span class="relative inline-flex size-2 rounded-full bg-ok" />
      </span>
      <h2 class="text-sm font-bold tracking-tight">Live feed</h2>
      <span class="font-mono text-xs text-dim">recent events</span>
    </header>

    <div v-if="!items.length" class="px-4 py-6 text-center text-sm text-dim">
      Waiting for live events — laps, drift runs, sessions and saved media show up here.
    </div>

    <ul v-else class="divide-y divide-line">
      <li v-for="it in items" :key="it.id">
        <component
          :is="it.link ? 'RouterLink' : 'div'"
          :to="it.link"
          class="flex items-center gap-3 px-4 py-2.5 text-sm transition-colors"
          :class="it.link ? 'hover:bg-surface-2/60' : ''"
        >
          <Icon :name="it.icon" :size="16" class="shrink-0" :class="toneClass[it.tone]" />
          <span class="min-w-0 flex-1 truncate text-text">{{ it.text }}</span>
          <span class="shrink-0 font-mono text-xs text-dim">{{ timeAgo(it.ts) }}</span>
          <Icon v-if="it.link" name="arrowUp" :size="13" class="shrink-0 rotate-90 text-dim" />
        </component>
      </li>
    </ul>
  </section>
</template>
