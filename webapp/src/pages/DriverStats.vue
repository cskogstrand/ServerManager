<script setup lang="ts">
// One directory for account drivers and guests; live state comes from shared SSE.
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { listDrivers, describeResult, fmtScore, timeAgo } from "@/lib/driversApi";
import { lapTime } from "@/lib/raceTelemetry";
import { ApiError } from "@/lib/api";
import { useQueryParam, enumParam } from "@/lib/useQueryParam";
import Button from "@/components/ui/Button.vue";
import { useServerStore } from "@/stores/server";
import type { DriverState } from "@/stores/server";
import type { DriverSummary } from "@/types/driverStats";
import PageHeader from "@/components/ui/PageHeader.vue";
import { useAuthStore } from "@/stores/auth";
import Icon from "@/components/ui/Icon.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";
import Sparkline from "@/components/ui/Sparkline.vue";

const auth = useAuthStore();
const error = ref("");
const server = useServerStore();
const router = useRouter();
function clearFilters() { void router.replace({ query: { ...router.currentRoute.value.query, q: undefined, live: undefined } }); }

const raw = ref<DriverSummary[]>([]);
const loading = ref(true);
const search = useQueryParam("q", "");
const liveFilter = useQueryParam("live", "", enumParam(["", "1"] as const, ""));
const onlineOnly = computed({ get: () => liveFilter.value === "1", set: (value: boolean) => { liveFilter.value = value ? "1" : ""; } });

type SortKey = "drift" | "recent" | "laps" | "name";
const sort = useQueryParam<SortKey>("sort", "drift", enumParam(["drift", "recent", "laps", "name"] as const, "drift"));
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
    error.value = "";
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <PageHeader eyebrow="Your drivers" title="The people make it." subtitle="Every lap, drift, and good moment. One place for everyone who drives with the club.">
    <template #actions>
      <RouterLink to="/leaderboard" class="pitlane-button">Club records</RouterLink>
      <RouterLink v-if="auth.canOperate" to="/guest-drivers" class="pitlane-button primary">Manage guests</RouterLink>
    </template>
  </PageHeader>

  <div v-if="error" role="alert" class="pitlane-error"><p>Drivers could not be loaded. {{ error }}</p><Button variant="ghost" class="mt-3" :disabled="loading" @click="load">Retry loading drivers</Button></div>

  <dl class="pitlane-metrics" aria-label="Club driving statistics">
    <div><dt>People in the club</dt><dd>{{ loading ? '—' : drivers.length }}</dd><small>Account drivers and guests</small></div>
    <div><dt>On track now</dt><dd>{{ loading ? '—' : liveCount }}</dd><small>{{ liveCount ? 'Across your servers' : 'Ready for the next drive' }}</small></div>
    <div><dt>Best drift</dt><dd>{{ topDrift?.best_drift ? fmtScore(topDrift.best_drift) : '—' }}</dd><small>{{ topDrift?.best_drift ? topDrift.name : 'No scored runs yet' }}</small></div>
    <div><dt>Fastest lap</dt><dd>{{ fastest ? lapTime(fastest.best_lap_ms) : '—' }}</dd><small>{{ fastest?.name ?? 'No timed laps yet' }}</small></div>
  </dl>

  <section aria-label="Driver directory">
    <div class="pitlane-directory-tools">
      <label class="pitlane-field driver-search"><span>Find a driver</span><input v-model="search" type="search" aria-label="Search drivers by name or GUID" placeholder="Name or account GUID…" /></label>
      <div class="pitlane-tabs" aria-label="Sort drivers"><button v-for="o in sortOptions" :key="o.value" type="button" :aria-pressed="sort === o.value" @click="sort = o.value">{{ o.label }}</button></div>
      <button type="button" class="pitlane-button" :aria-pressed="onlineOnly" :class="{ 'is-active': onlineOnly }" @click="onlineOnly = !onlineOnly"><span class="pitlane-status-dot" :class="{ online: onlineOnly }" />On track only</button>
    </div>
    <div class="pitlane-section-heading"><h2>Everyone at the club</h2><span role="status">{{ loading ? 'Loading drivers…' : `${filtered.length} ${filtered.length === 1 ? 'person' : 'people'}` }}</span></div>
    <div v-if="loading" class="space-y-4"><div v-for="n in 4" :key="n" class="h-24 animate-pulse rounded-xl bg-surface-2" /></div>
    <p v-else-if="error && !drivers.length" class="pitlane-empty">Use Retry loading drivers above to try again.</p>
    <div v-else-if="!filtered.length" class="pitlane-empty"><h2>{{ search || onlineOnly ? 'No drivers match.' : 'The first drive starts here.' }}</h2><p>{{ search || onlineOnly ? 'Try another name or clear the on-track filter.' : 'Drivers appear here when they join a session. Add guests for people sharing a simulator.' }}</p><Button v-if="search || onlineOnly" variant="ghost" @click="clearFilters">Clear filters</Button></div>
    <div v-else class="pitlane-driver-list">
      <RouterLink v-for="d in filtered" :key="rowKey(d)" :to="rowTo(d)" class="pitlane-driver-row">
        <div class="pitlane-person">
          <DriverAvatar :name="d.name" :guid="d.is_guest ? undefined : d.guid" :src="d.avatar_url" :size="64" />
          <div><h3>{{ d.name }}</h3><p><span v-if="d.online" class="text-ok">On track{{ d.live_drift ? ` · ${fmtScore(d.live_drift)} points` : '' }}</span><span v-else>{{ d.sessions && d.last_seen ? `Last drove ${timeAgo(d.last_seen)}` : 'First drive still to come' }}</span><span v-if="d.is_guest"> · Guest driver</span></p><small v-if="d.favourite_car">{{ d.favourite_car.name }}</small></div>
        </div>
        <div class="pitlane-driver-stat"><b>{{ d.best_drift ? fmtScore(d.best_drift) : '—' }}</b><span>Personal best · drift points</span><Sparkline v-if="d.drift_trend.length" :values="d.drift_trend" :width="100" :height="22" class="text-accent" :title="`${d.drift_trend.length} recent drift runs`" /></div>
        <div class="pitlane-driver-stat"><b>{{ d.sessions }}</b><span>Sessions with the club</span><small>{{ d.total_laps }} laps</small></div>
        <div class="pitlane-driver-recent"><b>{{ describeResult(d.last_result).primary }}</b><span>{{ describeResult(d.last_result).secondary }}</span><small>Most recent drive</small></div>
        <span class="pitlane-profile-link">Profile <Icon name="arrowUp" :size="16" class="rotate-90" /></span>
      </RouterLink>
    </div>
  </section>
</template>
