<script setup lang="ts">
// Dashboard activity feed. Live (page 0, all types) renders the SSE-fed ring
// from the feed store, seeded from persisted history on mount so a reload isn't
// empty. Filtering by type or paging back switches to a static query against
// /api/feed; the live ring keeps updating underneath and a hint offers a jump
// back to live.
import { computed, onMounted, ref, watch } from "vue";
import { useFeedStore, feedItemFromEvent, type FeedItem, type FeedTone } from "@/stores/feed";
import { api } from "@/lib/api";
import type { ServerEvent } from "@/lib/sse";
import { timeAgo } from "@/lib/driversApi";
import Icon from "@/components/ui/Icon.vue";

const feed = useFeedStore();

const PAGE = 12;
const FILTERS = [
  { v: "", label: "All activity" },
  { v: "lap", label: "Laps" },
  { v: "drift_run", label: "Drift runs" },
  { v: "session_end", label: "Finishes" },
  { v: "session_start", label: "Joins" },
  { v: "media", label: "Media" },
  { v: "recording", label: "Recordings" },
  { v: "server", label: "Server" },
  { v: "app", label: "App" },
];

const filterType = ref("");
const page = ref(0);
const total = ref(0);
const rows = ref<FeedItem[]>([]);
const loading = ref(false);

// Live = newest, unfiltered, first page: render the reactive SSE ring.
const live = computed(() => page.value === 0 && filterType.value === "");
const items = computed(() => (live.value ? feed.items.slice(0, PAGE) : rows.value));
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / PAGE)));

// "New activity" hint while browsing: track the ring's id last seen in live mode.
const seenId = ref(0);
watch(
  () => feed.lastId,
  (id) => { if (live.value) seenId.value = id },
);
const hasNew = computed(() => !live.value && feed.lastId > seenId.value);

async function load() {
  loading.value = true;
  try {
    const params = new URLSearchParams({ limit: String(PAGE), offset: String(page.value * PAGE) });
    if (filterType.value) params.set("type", filterType.value);
    const res = await api.get<{ events: ServerEvent[]; total: number }>(`/api/feed?${params}`);
    total.value = res.total;
    if (live.value) {
      feed.seed(res.events); // fills the ring only if empty
      seenId.value = feed.lastId;
    } else {
      // ponytail: offset paging — page boundaries shift if new events land while
      // browsing. Fine for an activity feed; revisit with a cursor if it bites.
      rows.value = res.events.map(feedItemFromEvent).filter((i): i is FeedItem => i !== null);
    }
  } catch {
    if (!live.value) rows.value = [];
  } finally {
    loading.value = false;
  }
}

function resumeLive() {
  filterType.value = "";
  page.value = 0;
}

watch(filterType, () => { page.value = 0; });
watch([filterType, page], load);
onMounted(load);

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
    <header class="flex flex-wrap items-center gap-2.5 border-b border-line px-4 py-3">
      <span class="relative flex size-2">
        <span v-if="live" class="absolute inline-flex size-full animate-ping rounded-full bg-ok opacity-70" />
        <span class="relative inline-flex size-2 rounded-full" :class="live ? 'bg-ok' : 'bg-dim'" />
      </span>
      <h2 class="text-sm font-bold tracking-tight">{{ live ? "Live feed" : "Activity" }}</h2>
      <span class="font-mono text-xs text-dim">{{ total }} event{{ total === 1 ? "" : "s" }}</span>

      <div class="ml-auto flex items-center gap-2">
        <button
          v-if="hasNew"
          type="button"
          class="inline-flex items-center gap-1.5 rounded-full border border-ok/40 bg-ok-glow px-2 py-0.5 text-[11px] font-bold text-ok transition-colors hover:border-ok/70"
          @click="resumeLive"
        >
          <span class="size-1.5 rounded-full bg-ok" /> New activity
        </button>
        <select
          v-model="filterType"
          class="h-8 rounded-md border border-line bg-surface px-2 text-xs text-text focus:border-accent/50 focus:outline-none"
          aria-label="Filter events by type"
        >
          <option v-for="f in FILTERS" :key="f.v" :value="f.v">{{ f.label }}</option>
        </select>
      </div>
    </header>

    <div v-if="!items.length" class="px-4 py-6 text-center text-sm text-dim">
      <template v-if="loading">Loading…</template>
      <template v-else-if="filterType || page > 0">No events match this filter.</template>
      <template v-else>Waiting for live events — laps, drift runs, sessions and saved media show up here.</template>
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

    <footer
      v-if="pageCount > 1 || page > 0"
      class="flex items-center justify-between gap-2 border-t border-line px-4 py-2"
    >
      <span class="font-mono text-xs text-dim">Page {{ page + 1 }} / {{ pageCount }}</span>
      <div class="flex items-center gap-1.5">
        <button
          type="button"
          :disabled="page === 0"
          class="grid size-7 place-items-center rounded-md border border-line bg-surface-2 text-muted transition-colors hover:border-accent/50 hover:text-accent disabled:opacity-40 disabled:hover:border-line disabled:hover:text-muted"
          aria-label="Newer events"
          @click="page--"
        >
          <Icon name="arrowUp" :size="14" class="-rotate-90" />
        </button>
        <button
          type="button"
          :disabled="page >= pageCount - 1"
          class="grid size-7 place-items-center rounded-md border border-line bg-surface-2 text-muted transition-colors hover:border-accent/50 hover:text-accent disabled:opacity-40 disabled:hover:border-line disabled:hover:text-muted"
          aria-label="Older events"
          @click="page++"
        >
          <Icon name="arrowUp" :size="14" class="rotate-90" />
        </button>
      </div>
    </footer>
  </section>
</template>
