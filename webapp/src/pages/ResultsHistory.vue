<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from "vue";
import { useQueryParam } from "@/lib/useQueryParam";
import FormRow from "@/components/ui/FormRow.vue";
import { api } from "@/lib/api";
import { listScores, searchSessions, fmtDate, fmtScore, shortGuid } from "@/lib/driversApi";
import { lapTime } from "@/lib/raceTelemetry";
import type { ScoreEntry, SessionSearchResult } from "@/types/driverStats";
import PageHeader from "@/components/ui/PageHeader.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import Input from "@/components/ui/Input.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";

const q = useQueryParam<string>("q", "");
const sessionError = ref("");
const scoreError = ref("");
const fileError = ref("");
let sessionVersion = 0;
const sessions = ref<SessionSearchResult[]>([]);
const scores = ref<ScoreEntry[]>([]);
const files = ref<ResultFile[]>([]);
const loading = ref(true);
const sessionsLoading = ref(false);

interface ResultFile {
  file: string;
  instance_id: number;
  modified_at: number;
  type: string;
  track: string;
  winner: string;
  entries: number;
  best_lap_ms: number;
}

let timer: ReturnType<typeof setTimeout> | undefined;

async function loadSessions() {
  const version = ++sessionVersion;
  sessionsLoading.value = true;
  try {
    const result = await searchSessions({ q: q.value.trim() });
    if (version === sessionVersion) { sessions.value = result; sessionError.value = ""; }
  } catch (error) {
    if (version === sessionVersion) sessionError.value = error instanceof Error ? error.message : "Session history could not be loaded.";
  } finally { if (version === sessionVersion) sessionsLoading.value = false; }
}
async function load() {
  loading.value = true;
  await Promise.all([
    loadSessions(),
    listScores().then(rows => { scores.value = rows; scoreError.value = ""; }).catch(() => { scoreError.value = "Records are unavailable. Try again."; }),
    api.get<{ items: ResultFile[] }>("/api/results/files").then(rows => { files.value = rows.items ?? []; fileError.value = ""; }).catch(() => { fileError.value = "Result files are unavailable. Try again."; }),
  ]);
  loading.value = false;
}
watch(q, () => {
  ++sessionVersion;
  sessions.value = [];
  sessionsLoading.value = true;
  if (timer) clearTimeout(timer);
  timer = setTimeout(loadSessions, 250);
});
onBeforeUnmount(() => { if (timer) clearTimeout(timer); ++sessionVersion; });
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
const matchingFiles = computed(() =>
  files.value
    .filter((f) => {
      const query = q.value.trim().toLowerCase();
      return !query || `${f.file} ${f.track} ${f.winner} ${f.type}`.toLowerCase().includes(query);
    }),
);

const recentFiles = computed(() => matchingFiles.value.slice(0, 12));

function durationLabel(r: SessionSearchResult): string {
  const end = r.left_at ?? Date.now();
  const mins = Math.max(0, Math.round((end - r.joined_at) / 60000));
  return mins < 60 ? `${mins}m` : `${Math.floor(mins / 60)}h ${mins % 60}m`;
}
</script>

<template>
  <PageHeader
    title="Results & History"
    subtitle="Recent driver activity and race results across all servers."
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

  <FormRow class="max-w-xl" label="Search recent activity" hint="Filters sessions and result files. Record cards show the overall records from the available scores.">
    <div class="flex gap-2"><Input v-model="q" placeholder="Driver, track or result file…" /><Button v-if="q" variant="dark" @click="q = ''">Clear</Button></div>
  </FormRow>
  <div v-if="sessionError || scoreError || fileError" role="alert" class="mb-4 rounded-md border border-danger/40 bg-danger-glow p-4 text-sm">
    <p class="font-semibold text-danger">Some history is unavailable</p>
    <p v-for="message in [sessionError, scoreError, fileError].filter(Boolean)" :key="message" class="mt-1">{{ message }}</p>
    <p class="mt-2 text-muted">Any previously loaded data stays visible until a refresh succeeds.</p><Button variant="dark" class="mt-3" :disabled="loading" @click="load">{{ loading ? 'Loading…' : 'Retry' }}</Button>
  </div>

  <div class="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
    <Card>
      <div class="text-xs text-dim">Matching sessions</div>
      <div class="mt-1 font-mono text-2xl font-black">{{ sessions.length }}</div>
      <div class="text-xs text-muted">{{ finishedSessions.length }} finished</div>
    </Card>
    <Card>
      <div class="text-xs text-dim">Matching result files</div>
      <div class="mt-1 font-mono text-2xl font-black">{{ matchingFiles.length }}</div>
      <div class="text-xs text-muted">race results available</div>
    </Card>
    <Card>
      <div class="text-xs text-dim">Laps in matching sessions</div>
      <div class="mt-1 font-mono text-2xl font-black">{{ totalLaps }}</div>
      <div class="text-xs text-muted">from the sessions loaded</div>
    </Card>
    <Card>
      <div class="text-xs text-dim">Top drift · overall</div>
      <div class="mt-1 truncate font-mono text-2xl font-black text-accent">
        {{ topDrift ? fmtScore(topDrift.drift_score ?? 0) : "—" }}
      </div>
      <div class="truncate text-xs text-muted">{{ topDrift?.driver ?? "No drift runs" }}</div>
    </Card>
    <Card>
      <div class="text-xs text-dim">Best lap · overall</div>
      <div class="mt-1 font-mono text-2xl font-black">{{ bestLap ? lapTime(bestLap.best_lap_ms ?? 0) : "—" }}</div>
      <div class="truncate text-xs text-muted">{{ bestLap?.driver ?? "No timed laps" }}</div>
    </Card>
  </div>

  <div class="grid gap-4 xl:grid-cols-2">
  <Card>
    <template #header>
      <Icon name="clock" :size="15" class="text-accent" />
      <h2 class="text-sm font-bold">Recent sessions</h2>
      <span class="ml-auto text-xs text-dim" role="status">{{ sessionsLoading ? "Searching…" : `${recentSessions.length} shown` }}</span>
    </template>

    <div v-if="sessionsLoading && !recentSessions.length" class="space-y-2">
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
      {{ sessionError ? "Sessions could not be loaded. Use Retry above." : q ? "No matching sessions. Try another search or clear it." : "No sessions yet. Driver activity will appear here after someone connects." }}
    </div>
  </Card>

  <Card>
    <template #header>
      <Icon name="content" :size="15" class="text-accent" />
      <h2 class="text-sm font-bold">Result files</h2>
      <span class="ml-auto text-xs text-dim">{{ recentFiles.length }} shown</span>
    </template>

    <div v-if="loading && !files.length" class="py-10 text-center text-sm text-muted" role="status">Loading result files…</div>
    <div v-else-if="recentFiles.length" class="divide-y divide-line/60">
      <div v-for="f in recentFiles" :key="`${f.instance_id}:${f.file}`" class="py-3">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="truncate text-sm font-bold">{{ f.track || f.file }}</div>
            <div class="mt-0.5 truncate font-mono text-xs text-dim">{{ f.file }}</div>
          </div>
          <span class="shrink-0 rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
            {{ f.type }}
          </span>
        </div>
        <div class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-xs sm:grid-cols-4">
          <div>
            <div class="text-dim">Winner</div>
            <div class="truncate font-medium">{{ f.winner || "-" }}</div>
          </div>
          <div>
            <div class="text-dim">Best lap in race</div>
            <div class="font-mono">{{ f.best_lap_ms ? lapTime(f.best_lap_ms) : "-" }}</div>
          </div>
          <div>
            <div class="text-dim">Entries</div>
            <div class="font-mono">{{ f.entries || "-" }}</div>
          </div>
          <div>
            <div class="text-dim">Date</div>
            <div>{{ fmtDate(f.modified_at) }}</div>
          </div>
        </div>
      </div>
    </div>
    <div v-else class="py-10 text-center text-sm text-muted">
      {{ fileError ? "Result files could not be loaded. Use Retry above." : q ? "No matching result files. Try another search or clear it." : "No race results yet. Completed race results will appear here." }}
    </div>
  </Card>
  </div>
</template>
