<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { listScores, searchSessions, fmtDate, fmtScore, shortGuid } from "@/lib/driversApi";
import { lapTime } from "@/lib/raceTelemetry";
import type { ScoreEntry, SessionSearchResult } from "@/types/driverStats";
import PageHeader from "@/components/ui/PageHeader.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import Input from "@/components/ui/Input.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";

const q = ref("");
const sessions = ref<SessionSearchResult[]>([]);
const scores = ref<ScoreEntry[]>([]);
const loading = ref(true);

let timer: ReturnType<typeof setTimeout> | undefined;

async function load() {
  loading.value = true;
  try {
    [sessions.value, scores.value] = await Promise.all([searchSessions({ q: q.value.trim() }), listScores()]);
  } finally {
    loading.value = false;
  }
}

watch(q, () => {
  if (timer) clearTimeout(timer);
  timer = setTimeout(load, 250);
});

onMounted(load);

const finishedSessions = computed(() => sessions.value.filter((s) => s.left_at !== null));
const totalLaps = computed(() => sessions.value.reduce((n, s) => n + s.laps, 0));
const topDrift = computed(() =>
  [...scores.value].filter((s) => s.kind === "drift").sort((a, b) => (b.drift_score ?? 0) - (a.drift_score ?? 0))[0] ?? null,
);
const bestLap = computed(() =>
  [...scores.value].filter((s) => s.kind === "lap" && s.best_lap_ms).sort((a, b) => (a.best_lap_ms ?? 0) - (b.best_lap_ms ?? 0))[0] ?? null,
);
const recentSessions = computed(() => [...sessions.value].sort((a, b) => b.joined_at - a.joined_at).slice(0, 12));

function durationLabel(r: SessionSearchResult): string {
  const end = r.left_at ?? Date.now();
  const mins = Math.max(0, Math.round((end - r.joined_at) / 60000));
  return mins < 60 ? `${mins}m` : `${Math.floor(mins / 60)}h ${mins % 60}m`;
}
</script>

<template>
  <PageHeader
    title="Results & History"
    subtitle="Recent sessions, lap records and drift results across all drivers."
    icon="trophy"
  >
    <template #actions>
      <RouterLink to="/sessions">
        <Button variant="ghost" size="sm">
          <Icon name="search" :size="14" />
          Session Search
        </Button>
      </RouterLink>
      <RouterLink to="/leaderboard">
        <Button variant="dark" size="sm">
          <Icon name="trophy" :size="14" />
          Leaderboard
        </Button>
      </RouterLink>
    </template>
  </PageHeader>

  <div class="mb-4 max-w-xl">
    <Input v-model="q" placeholder="Filter by driver or track..." />
  </div>

  <div class="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
    <Card>
      <div class="text-xs text-dim">Sessions</div>
      <div class="mt-1 font-mono text-2xl font-black">{{ sessions.length }}</div>
      <div class="text-xs text-muted">{{ finishedSessions.length }} finished</div>
    </Card>
    <Card>
      <div class="text-xs text-dim">Laps</div>
      <div class="mt-1 font-mono text-2xl font-black">{{ totalLaps }}</div>
      <div class="text-xs text-muted">recorded in history</div>
    </Card>
    <Card>
      <div class="text-xs text-dim">Top drift</div>
      <div class="mt-1 truncate font-mono text-2xl font-black text-accent">
        {{ topDrift ? fmtScore(topDrift.drift_score ?? 0) : "—" }}
      </div>
      <div class="truncate text-xs text-muted">{{ topDrift?.driver ?? "No drift runs" }}</div>
    </Card>
    <Card>
      <div class="text-xs text-dim">Best lap</div>
      <div class="mt-1 font-mono text-2xl font-black">{{ bestLap ? lapTime(bestLap.best_lap_ms ?? 0) : "—" }}</div>
      <div class="truncate text-xs text-muted">{{ bestLap?.driver ?? "No timed laps" }}</div>
    </Card>
  </div>

  <Card>
    <template #header>
      <Icon name="clock" :size="15" class="text-accent" />
      <h2 class="text-sm font-bold">Recent sessions</h2>
      <span class="ml-auto text-xs text-dim">{{ loading ? "Loading..." : `${recentSessions.length} shown` }}</span>
    </template>

    <div v-if="loading && !recentSessions.length" class="space-y-2">
      <div v-for="i in 5" :key="i" class="h-14 animate-pulse rounded-md bg-surface-2/60" />
    </div>

    <div v-else-if="recentSessions.length" class="divide-y divide-line/60">
      <RouterLink
        v-for="r in recentSessions"
        :key="r.id"
        :to="{ name: 'driver-detail', params: { guid: r.guid }, query: { session: r.id } }"
        class="flex flex-col gap-2 py-3 transition-colors hover:bg-surface-2/35 sm:flex-row sm:items-center"
      >
        <DriverAvatar :name="r.driver" :guid="r.guid" :src="r.avatar_url ?? null" :size="38" radius="md" class="shrink-0" />
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <span class="truncate text-sm font-bold">{{ r.driver }}</span>
            <span v-if="r.online" class="rounded-full border border-ok/40 bg-ok-glow px-1.5 py-0.5 text-[10px] font-bold text-ok">Live</span>
            <span class="font-mono text-[10px] text-dim">{{ shortGuid(r.guid) }}</span>
          </div>
          <div class="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-muted">
            <span>{{ r.track.name }}</span>
            <span>{{ r.car.name }}</span>
            <span>{{ fmtDate(r.joined_at) }} · {{ r.left_at ? durationLabel(r) : "ongoing" }}</span>
          </div>
        </div>
        <div class="flex shrink-0 gap-4 text-right">
          <div v-if="r.best_drift">
            <div class="font-mono text-sm font-bold text-accent">{{ fmtScore(r.best_drift) }}</div>
            <div class="text-[9px] font-bold tracking-wide text-dim uppercase">Drift</div>
          </div>
          <div v-if="r.best_lap_ms">
            <div class="font-mono text-sm font-bold">{{ lapTime(r.best_lap_ms) }}</div>
            <div class="text-[9px] font-bold tracking-wide text-dim uppercase">Lap</div>
          </div>
          <div>
            <div class="font-mono text-sm font-bold">{{ r.laps }}</div>
            <div class="text-[9px] font-bold tracking-wide text-dim uppercase">Laps</div>
          </div>
        </div>
      </RouterLink>
    </div>

    <div v-else class="py-10 text-center text-sm text-muted">
      No sessions found.
    </div>
  </Card>
</template>
