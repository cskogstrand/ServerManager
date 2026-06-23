<script setup lang="ts">
// Driver Stats — a pit-wall timing tower for everyone who has turned a lap or
// laid down a drift run on the server. Drift score is the signature metric, so
// it headlines each row; the last session reads as a drift score or a lap time
// depending on what kind of session it was. Live drivers are pulled from the
// SSE store and pinned/marked online on top of the stats snapshot.
import { computed, onMounted, ref } from "vue";
import { listDrivers, describeResult, fmtScore, shortGuid, timeAgo } from "@/lib/driversApi";
import { lapTime } from "@/lib/raceTelemetry";
import { ApiError } from "@/lib/api";
import { useToastStore } from "@/stores/toast";
import { useServerStore } from "@/stores/server";
import type { DriverState } from "@/stores/server";
import type { DriverSummary } from "@/types/driverStats";
import PageHeader from "@/components/ui/PageHeader.vue";
import Card from "@/components/ui/Card.vue";
import Icon from "@/components/ui/Icon.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";
import Sparkline from "@/components/ui/Sparkline.vue";

const toast = useToastStore();
const server = useServerStore();

const raw = ref<DriverSummary[]>([]);
const loading = ref(true);
const search = ref("");
const onlineOnly = ref(false);

type SortKey = "drift" | "recent" | "laps" | "name";
const sort = ref<SortKey>("drift");
const sortOptions: { value: SortKey; label: string }[] = [
  { value: "drift", label: "Drift" },
  { value: "recent", label: "Recent" },
  { value: "laps", label: "Laps" },
  { value: "name", label: "Name" },
];

// guid → live driver from any running instance. Overlays online state + a live
// drift readout on top of the persisted snapshot.
const liveByGuid = computed(() => {
  const m = new Map<string, DriverState>();
  for (const inst of server.instanceList) {
    if (!inst.running) continue;
    for (const d of inst.drivers) {
      if (d.connected && d.guid) m.set(d.guid, d);
    }
  }
  return m;
});

const drivers = computed<DriverSummary[]>(() =>
  raw.value.map((d) => {
    const live = liveByGuid.value.get(d.guid);
    if (!live) return d;
    return { ...d, online: true, live_drift: live.drift_live || live.drift_best || 0, last_seen: Date.now() };
  }),
);

const liveCount = computed(() => drivers.value.filter((d) => d.online).length);
const maxDrift = computed(() => Math.max(1, ...drivers.value.map((d) => d.best_drift)));

const topDrift = computed(() => [...drivers.value].sort((a, b) => b.best_drift - a.best_drift)[0]);
const fastest = computed(() =>
  drivers.value.filter((d) => d.best_lap_ms > 0).sort((a, b) => a.best_lap_ms - b.best_lap_ms)[0],
);

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase();
  let list = drivers.value.filter((d) => {
    if (onlineOnly.value && !d.online) return false;
    if (!q) return true;
    return d.name.toLowerCase().includes(q) || d.guid.toLowerCase().includes(q);
  });
  const by: Record<SortKey, (a: DriverSummary, b: DriverSummary) => number> = {
    drift: (a, b) => b.best_drift - a.best_drift,
    recent: (a, b) => b.last_seen - a.last_seen,
    laps: (a, b) => b.total_laps - a.total_laps,
    name: (a, b) => a.name.localeCompare(b.name),
  };
  list = [...list].sort(by[sort.value]);
  // Online drivers always float to the top of the current ordering.
  return list.sort((a, b) => Number(b.online) - Number(a.online));
});

function rankClass(i: number): string {
  if (i === 0) return "border-warn/45 bg-warn-glow text-warn";
  if (i <= 2) return "border-line-hi bg-surface-3 text-text";
  return "border-line bg-surface-2 text-dim";
}

// Guests share an empty guid, so rows key off a guest-scoped id and link to the
// guest profile instead of a GUID driver detail.
function rowKey(d: DriverSummary): string {
  return d.is_guest ? `g${d.guest_id}` : d.guid;
}
function rowTo(d: DriverSummary) {
  return d.is_guest
    ? { name: "guest-driver-detail", params: { id: d.guest_id } }
    : { name: "driver-detail", params: { guid: d.guid } };
}

async function load() {
  loading.value = true;
  try {
    raw.value = await listDrivers();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <PageHeader
    title="Driver Stats"
    subtitle="Everyone who's turned a lap or laid down a drift run on the server."
    icon="trophy"
  >
    <template #actions>
      <span
        class="inline-flex items-center gap-2 rounded-md border border-line bg-surface-2 px-3 py-1.5 text-xs font-semibold text-muted"
      >
        <span class="size-1.5 rounded-full" :class="liveCount ? 'bg-ok live-dot' : 'bg-dim'" />
        {{ liveCount }} live now
      </span>
    </template>
  </PageHeader>

  <!-- KPI strip -->
  <div class="mb-4 grid grid-cols-2 gap-2 sm:gap-3 lg:grid-cols-4">
    <div class="rounded-md border border-line bg-surface px-3 py-2 sm:px-4 sm:py-3">
      <div class="flex items-center gap-1.5 text-[10px] font-bold tracking-wide text-dim uppercase sm:text-[11px]">
        <Icon name="users" :size="13" /> Drivers seen
      </div>
      <div class="mt-0.5 font-mono text-xl font-bold tabular-nums text-text sm:mt-1 sm:text-2xl">{{ drivers.length }}</div>
      <div class="truncate text-[11px] text-muted sm:text-xs">across all sessions</div>
    </div>
    <div class="rounded-md border border-line bg-surface px-3 py-2 sm:px-4 sm:py-3">
      <div class="flex items-center gap-1.5 text-[10px] font-bold tracking-wide text-dim uppercase sm:text-[11px]">
        <Icon name="activity" :size="13" /> On track now
      </div>
      <div class="mt-0.5 font-mono text-xl font-bold tabular-nums sm:mt-1 sm:text-2xl" :class="liveCount ? 'text-ok' : 'text-text'">
        {{ liveCount }}
      </div>
      <div class="truncate text-[11px] text-muted sm:text-xs">{{ liveCount ? "live from the server" : "garage is quiet" }}</div>
    </div>
    <div class="rounded-md border border-line bg-surface px-3 py-2 sm:px-4 sm:py-3">
      <div class="flex items-center gap-1.5 text-[10px] font-bold tracking-wide text-dim uppercase sm:text-[11px]">
        <Icon name="gauge" :size="13" /> Top drift
      </div>
      <div class="mt-0.5 font-mono text-xl font-bold tabular-nums text-accent sm:mt-1 sm:text-2xl">
        {{ topDrift ? fmtScore(topDrift.best_drift) : "—" }}
      </div>
      <div class="truncate text-[11px] text-muted sm:text-xs">{{ topDrift?.name ?? "no runs yet" }}</div>
    </div>
    <div class="rounded-md border border-line bg-surface px-3 py-2 sm:px-4 sm:py-3">
      <div class="flex items-center gap-1.5 text-[10px] font-bold tracking-wide text-dim uppercase sm:text-[11px]">
        <Icon name="clock" :size="13" /> Fastest lap
      </div>
      <div class="mt-0.5 font-mono text-xl font-bold tabular-nums text-text sm:mt-1 sm:text-2xl">
        {{ fastest ? lapTime(fastest.best_lap_ms) : "—" }}
      </div>
      <div class="truncate text-[11px] text-muted sm:text-xs">{{ fastest?.name ?? "no timed laps yet" }}</div>
    </div>
  </div>

  <!-- Controls -->
  <div class="mb-3 flex flex-wrap items-center gap-2">
    <div class="relative min-w-[200px] flex-1">
      <Icon name="search" :size="16" class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-dim" />
      <input
        v-model="search"
        type="search"
        placeholder="Search by driver or GUID…"
        class="h-9 w-full rounded-md border border-line bg-surface-2 pr-3 pl-9 text-sm text-text transition-colors placeholder:text-dim focus:border-accent/60 focus:outline-none"
      />
    </div>
    <div class="flex rounded-md border border-line bg-surface-2 p-0.5">
      <button
        v-for="o in sortOptions"
        :key="o.value"
        type="button"
        class="h-8 cursor-pointer rounded-sm px-2.5 text-xs font-semibold transition-colors"
        :class="sort === o.value ? 'bg-accent-dim text-accent' : 'text-muted hover:text-text'"
        @click="sort = o.value"
      >
        {{ o.label }}
      </button>
    </div>
    <button
      type="button"
      class="inline-flex h-9 cursor-pointer items-center gap-2 rounded-md border px-3 text-xs font-semibold transition-colors"
      :class="onlineOnly ? 'border-ok/45 bg-ok-glow text-ok' : 'border-line bg-surface-2 text-muted hover:text-text'"
      @click="onlineOnly = !onlineOnly"
    >
      <span class="size-1.5 rounded-full" :class="onlineOnly ? 'bg-ok' : 'bg-dim'" />
      Live only
    </button>
  </div>

  <Card>
    <template #header>
      <h2 class="text-sm font-bold tracking-tight">Leaderboard</h2>
      <span class="text-xs text-dim">{{ filtered.length }} driver{{ filtered.length === 1 ? "" : "s" }}</span>
    </template>

    <!-- Loading skeleton -->
    <div v-if="loading" class="space-y-2">
      <div v-for="n in 6" :key="n" class="h-[66px] animate-pulse rounded-md border border-line bg-surface-2/50" />
    </div>

    <!-- Empty -->
    <div v-else-if="!filtered.length" class="py-14 text-center">
      <div class="mx-auto grid size-12 place-items-center rounded-lg border border-line bg-surface-2 text-dim">
        <Icon name="users" :size="22" />
      </div>
      <p class="mt-3 text-sm font-semibold text-text">No drivers match</p>
      <p class="mt-0.5 text-sm text-muted">
        {{ search || onlineOnly ? "Try clearing the search or the live-only filter." : "Drivers appear here once they join a session." }}
      </p>
    </div>

    <!-- Rows -->
    <div v-else class="space-y-2">
      <!-- Mobile: taller stacked card -->
      <RouterLink
        v-for="(d, i) in filtered"
        :key="`m-${rowKey(d)}`"
        :to="rowTo(d)"
        class="reveal group flex flex-col gap-2.5 rounded-md border border-line bg-surface-2/40 px-3 py-3 transition-colors hover:border-line-hi hover:bg-surface-2 sm:hidden"
        :style="{ animationDelay: Math.min(i, 12) * 35 + 'ms' }"
      >
        <div class="flex items-center gap-3">
          <span
            class="grid size-7 shrink-0 place-items-center rounded-md border font-mono text-xs font-bold tabular-nums"
            :class="rankClass(i)"
          >
            {{ i + 1 }}
          </span>
          <DriverAvatar :name="d.name" :guid="d.is_guest ? undefined : d.guid" :src="d.avatar_url" :size="38" />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="truncate text-sm font-bold text-text group-hover:text-accent">{{ d.name }}</span>
              <span
                v-if="d.is_guest"
                class="inline-flex shrink-0 items-center gap-1 rounded-full border border-warn/40 bg-warn-glow px-1.5 py-px text-[10px] font-bold tracking-wide text-warn uppercase"
                title="Guest driver"
              >
                <Icon name="users" :size="10" /> Guest
              </span>
              <span
                v-if="d.online"
                class="inline-flex shrink-0 items-center gap-1 rounded-full border border-ok/40 bg-ok-glow px-1.5 py-px text-[10px] font-bold tracking-wide text-ok uppercase"
              >
                <span class="size-1 rounded-full bg-ok live-dot" />
                {{ d.live_drift ? fmtScore(d.live_drift) : "Live" }}
              </span>
            </div>
            <div class="mt-0.5 truncate text-xs text-dim">
              <Icon name="car" :size="12" class="-mt-px mr-1 inline text-dim" />{{ d.favourite_car?.name ?? "—" }}
            </div>
          </div>
          <Icon name="arrowUp" :size="15" class="shrink-0 rotate-90 text-dim transition-colors group-hover:text-accent" />
        </div>

        <div class="flex items-end justify-between gap-3 border-t border-line/60 pt-2.5">
          <div class="min-w-0 flex-1">
            <div class="font-mono text-base font-bold tabular-nums text-accent">{{ fmtScore(d.best_drift) }}</div>
            <div class="mt-1 h-1 w-full overflow-hidden rounded-full bg-surface-3">
              <div class="h-full rounded-full bg-accent/70" :style="{ width: (d.best_drift / maxDrift) * 100 + '%' }" />
            </div>
            <div class="mt-0.5 text-[10px] font-bold tracking-wide text-dim uppercase">Best drift</div>
          </div>
          <div class="shrink-0 text-right">
            <div
              class="truncate font-mono text-sm font-semibold tabular-nums"
              :class="describeResult(d.last_result).tone === 'warn' ? 'text-warn' : describeResult(d.last_result).tone === 'accent' ? 'text-accent' : 'text-text'"
            >
              {{ describeResult(d.last_result).primary }}
            </div>
            <div class="truncate text-[11px] text-dim">{{ describeResult(d.last_result).secondary }}</div>
            <div class="text-[10px] text-dim/70">{{ timeAgo(d.last_seen) }}</div>
          </div>
        </div>
      </RouterLink>

      <RouterLink
        v-for="(d, i) in filtered"
        :key="rowKey(d)"
        :to="rowTo(d)"
        class="reveal group hidden items-center gap-3 rounded-md border border-line bg-surface-2/40 px-3 py-2.5 transition-all duration-200 hover:translate-x-0.5 hover:border-line-hi hover:bg-surface-2 sm:flex sm:gap-4 sm:px-4"
        :style="{ animationDelay: Math.min(i, 12) * 35 + 'ms' }"
      >
        <span
          class="grid size-7 shrink-0 place-items-center rounded-md border font-mono text-xs font-bold tabular-nums"
          :class="rankClass(i)"
        >
          {{ i + 1 }}
        </span>

        <DriverAvatar :name="d.name" :guid="d.is_guest ? undefined : d.guid" :src="d.avatar_url" :size="42" />

        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <span class="truncate text-sm font-bold text-text group-hover:text-accent">{{ d.name }}</span>
            <span
              v-if="d.is_guest"
              class="inline-flex shrink-0 items-center gap-1 rounded-full border border-warn/40 bg-warn-glow px-1.5 py-px text-[10px] font-bold tracking-wide text-warn uppercase"
              title="Guest driver"
            >
              <Icon name="users" :size="10" /> Guest
            </span>
            <span
              v-if="d.online"
              class="inline-flex shrink-0 items-center gap-1 rounded-full border border-ok/40 bg-ok-glow px-1.5 py-px text-[10px] font-bold tracking-wide text-ok uppercase"
            >
              <span class="size-1 rounded-full bg-ok live-dot" />
              {{ d.live_drift ? fmtScore(d.live_drift) : "Live" }}
            </span>
          </div>
          <div class="mt-0.5 flex items-center gap-2 text-xs text-dim">
            <span class="truncate">
              <Icon name="car" :size="12" class="-mt-px mr-1 inline text-dim" />{{ d.favourite_car?.name ?? "—" }}
            </span>
            <span v-if="!d.is_guest" class="hidden font-mono text-[11px] text-dim/80 sm:inline">{{ shortGuid(d.guid) }}</span>
          </div>
        </div>

        <div class="hidden shrink-0 text-accent/80 md:block" :title="`${d.drift_trend.length} recent drift runs`">
          <Sparkline :values="d.drift_trend" :width="92" :height="28" />
        </div>

        <div class="w-[8rem] shrink-0 text-right">
          <div class="font-mono text-base font-bold tabular-nums text-accent">{{ fmtScore(d.best_drift) }}</div>
          <div class="mt-1 ml-auto h-1 w-full overflow-hidden rounded-full bg-surface-3">
            <div class="h-full rounded-full bg-accent/70" :style="{ width: (d.best_drift / maxDrift) * 100 + '%' }" />
          </div>
          <div class="mt-0.5 text-[10px] font-bold tracking-wide text-dim uppercase">Best drift</div>
        </div>

        <div class="hidden w-[10rem] shrink-0 border-l border-line/70 pl-4 text-right lg:block">
          <div
            class="truncate font-mono text-sm font-semibold tabular-nums"
            :class="describeResult(d.last_result).tone === 'warn' ? 'text-warn' : describeResult(d.last_result).tone === 'accent' ? 'text-accent' : 'text-text'"
          >
            {{ describeResult(d.last_result).primary }}
          </div>
          <div class="truncate text-[11px] text-dim">{{ describeResult(d.last_result).secondary }}</div>
          <div class="text-[10px] text-dim/70">{{ timeAgo(d.last_seen) }}</div>
        </div>

        <Icon
          name="arrowUp"
          :size="15"
          class="shrink-0 rotate-90 text-dim transition-colors group-hover:text-accent"
        />
      </RouterLink>
    </div>
  </Card>
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
  animation: rise 0.5s cubic-bezier(0.22, 0.61, 0.36, 1) both;
}
@keyframes ping-soft {
  0% {
    box-shadow: 0 0 0 0 rgba(79, 216, 132, 0.5);
  }
  70%,
  100% {
    box-shadow: 0 0 0 5px rgba(79, 216, 132, 0);
  }
}
.live-dot {
  animation: ping-soft 1.8s ease-out infinite;
}
</style>
