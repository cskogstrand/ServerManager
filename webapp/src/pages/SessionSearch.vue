<script setup lang="ts">
// Global session search — find a driver's stint (connection) again later by tag,
// driver name or track. Tag chips are clickable to pivot the search; each result
// deep-links to that session on the driver's detail page.
import { ref, watch, onMounted } from "vue";
import { useRoute } from "vue-router";
import { searchSessions, fmtDate, fmtScore, shortGuid } from "@/lib/driversApi";
import { lapTime } from "@/lib/raceTelemetry";
import type { SessionSearchResult } from "@/types/driverStats";
import Card from "@/components/ui/Card.vue";
import Icon from "@/components/ui/Icon.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";

const route = useRoute();
const q = ref("");
const tag = ref("");
const results = ref<SessionSearchResult[]>([]);
const loading = ref(false);
const loaded = ref(false);

let timer: ReturnType<typeof setTimeout> | undefined;

async function run() {
  loading.value = true;
  try {
    results.value = await searchSessions({ q: q.value.trim(), tag: tag.value.trim() });
  } catch {
    results.value = [];
  } finally {
    loading.value = false;
    loaded.value = true;
  }
}

// Debounce keystrokes; tag changes (chip clicks) fire immediately.
function schedule() {
  if (timer) clearTimeout(timer);
  timer = setTimeout(run, 250);
}

watch(q, schedule);
watch(tag, run);

function pickTag(t: string) {
  tag.value = tag.value.toLowerCase() === t.toLowerCase() ? "" : t;
}
function clearAll() {
  q.value = "";
  tag.value = "";
}

function durationLabel(r: SessionSearchResult): string {
  const end = r.left_at ?? Date.now();
  const mins = Math.max(0, Math.round((end - r.joined_at) / 60000));
  if (mins < 60) return `${mins}m`;
  return `${Math.floor(mins / 60)}h ${mins % 60}m`;
}

onMounted(() => {
  if (typeof route.query.tag === "string") tag.value = route.query.tag;
  if (typeof route.query.q === "string") q.value = route.query.q;
  void run();
});
</script>

<template>
  <div class="space-y-4">
    <header class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-2xl font-black tracking-tight text-text">Session Search</h1>
        <p class="mt-0.5 text-sm text-muted">Find a stint by tag, driver or track — across every driver.</p>
      </div>
      <RouterLink
        :to="{ name: 'drivers' }"
        class="inline-flex items-center gap-1.5 text-sm font-semibold text-muted transition-colors hover:text-accent"
      >
        <Icon name="trophy" :size="16" /> Driver Stats
      </RouterLink>
    </header>

    <!-- Filter bar -->
    <Card>
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
        <label class="relative flex-1">
          <Icon name="search" :size="16" class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-dim" />
          <input
            v-model="q"
            type="text"
            placeholder="Driver name or track…"
            class="h-10 w-full rounded-md border border-line bg-surface pl-9 pr-3 text-sm text-text placeholder:text-dim focus:border-accent/50 focus:outline-none"
          />
        </label>
        <label class="relative sm:w-64">
          <span class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 font-mono text-sm text-dim">#</span>
          <input
            v-model="tag"
            type="text"
            placeholder="Tag (e.g. tandem night)"
            class="h-10 w-full rounded-md border border-line bg-surface pl-8 pr-3 text-sm text-text placeholder:text-dim focus:border-accent/50 focus:outline-none"
          />
        </label>
        <button
          v-if="q || tag"
          type="button"
          class="inline-flex h-10 shrink-0 items-center gap-1.5 rounded-md border border-line bg-surface-2 px-3 text-sm font-semibold text-muted transition-colors hover:border-accent/50 hover:text-accent"
          @click="clearAll"
        >
          <Icon name="x" :size="14" /> Clear
        </button>
      </div>
    </Card>

    <!-- Loading -->
    <div v-if="loading && !results.length" class="space-y-2">
      <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-lg border border-line bg-surface-2/50" />
    </div>

    <!-- Results -->
    <div v-else-if="results.length" class="space-y-2">
      <p class="text-xs text-dim">{{ results.length }} session{{ results.length === 1 ? "" : "s" }}</p>
      <article
        v-for="(r, i) in results"
        :key="r.id"
        class="reveal flex flex-col gap-3 rounded-lg border border-line bg-surface-2/30 p-3.5 sm:flex-row sm:items-center"
        :style="{ animationDelay: Math.min(i, 12) * 35 + 'ms' }"
      >
        <DriverAvatar :name="r.driver" :guid="r.guid" :src="r.avatar_url ?? null" :size="44" radius="md" class="shrink-0" />

        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <RouterLink
              :to="{ name: 'driver-detail', params: { guid: r.guid }, query: { session: r.id } }"
              class="truncate text-sm font-bold text-text transition-colors hover:text-accent"
            >
              {{ r.driver }}
            </RouterLink>
            <span
              v-if="r.online"
              class="inline-flex items-center gap-1 rounded-full border border-ok/40 bg-ok-glow px-1.5 py-0.5 text-[10px] font-bold tracking-wide text-ok uppercase"
            >
              <span class="size-1.5 rounded-full bg-ok" /> Live
            </span>
            <span class="font-mono text-[10px] text-dim">{{ shortGuid(r.guid) }}</span>
          </div>
          <div class="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted">
            <span class="inline-flex items-center gap-1"><Icon name="mapPin" :size="12" class="text-dim" /> {{ r.track.name }}</span>
            <span class="inline-flex items-center gap-1"><Icon name="car" :size="12" class="text-dim" /> {{ r.car.name }}</span>
            <span>{{ fmtDate(r.joined_at) }} · {{ r.left_at ? durationLabel(r) : "ongoing" }}</span>
          </div>
          <div v-if="r.tags.length" class="mt-1.5 flex flex-wrap gap-1.5">
            <button
              v-for="t in r.tags"
              :key="t"
              type="button"
              class="inline-flex cursor-pointer items-center rounded-full border px-2 py-0.5 text-[11px] font-semibold transition-colors"
              :class="tag.toLowerCase() === t.toLowerCase() ? 'border-accent bg-accent text-bg' : 'border-accent/35 bg-accent-dim text-accent hover:border-accent/60'"
              @click="pickTag(t)"
            >
              {{ t }}
            </button>
          </div>
        </div>

        <div class="flex shrink-0 items-center gap-4 sm:gap-5">
          <div v-if="r.best_drift" class="text-right">
            <div class="font-mono text-sm font-bold tabular-nums text-accent">{{ fmtScore(r.best_drift) }}</div>
            <div class="text-[9px] font-bold tracking-wide text-dim uppercase">Best drift</div>
          </div>
          <div v-if="r.best_lap_ms" class="text-right">
            <div class="font-mono text-sm font-bold tabular-nums text-text">{{ lapTime(r.best_lap_ms) }}</div>
            <div class="text-[9px] font-bold tracking-wide text-dim uppercase">Best lap</div>
          </div>
          <div class="text-right">
            <div class="font-mono text-sm font-bold tabular-nums text-text">{{ r.laps }}</div>
            <div class="text-[9px] font-bold tracking-wide text-dim uppercase">Laps</div>
          </div>
          <RouterLink
            :to="{ name: 'driver-detail', params: { guid: r.guid }, query: { session: r.id } }"
            class="grid size-8 shrink-0 place-items-center rounded-md border border-line bg-surface-2 text-muted transition-colors hover:border-accent/50 hover:text-accent"
            title="Open session"
          >
            <Icon name="arrowUp" :size="15" class="rotate-90" />
          </RouterLink>
        </div>
      </article>
    </div>

    <!-- Empty -->
    <Card v-else-if="loaded">
      <div class="py-12 text-center">
        <div class="mx-auto grid size-12 place-items-center rounded-lg border border-line bg-surface-2 text-dim">
          <Icon name="search" :size="22" />
        </div>
        <p class="mt-3 text-sm font-semibold text-text">No sessions found</p>
        <p class="mx-auto mt-0.5 max-w-md text-sm text-muted">
          {{ q || tag ? "Nothing matches that search. Try a different tag or name." : "Tag a driver's session on their detail page to find it here later." }}
        </p>
      </div>
    </Card>
  </div>
</template>

<style scoped>
@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
.reveal {
  animation: rise 0.45s cubic-bezier(0.22, 0.61, 0.36, 1) both;
}
</style>
