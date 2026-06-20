<script setup lang="ts">
// Full-screen broadcast overlay for one instance — a trackside TV graphics
// surface meant for a second screen. Two equal columns: the live track map
// fills the left half (car pucks glide over it), and the right half holds one
// card per driver, in running order, each pairing that driver's telemetry and
// car/livery preview with their video stream (stream to the right of the
// metrics on wide screens, below them on narrow). Cards are capped at half the
// stage height so at least two drivers are always on screen. All live data
// comes from the SSE-fed server store; this page only paints it.
import {computed, onBeforeUnmount, onMounted, ref, watch} from "vue";
import {useRoute} from "vue-router";
import {api} from "@/lib/api";
import {type CarPositionState, type InstanceState, type StatusResponse, useServerStore,} from "@/stores/server";
import {useContentStore} from "@/stores/content";
import {
  computeRunningOrder,
  formatClock,
  gearLabel,
  lapTime,
  rpmCeiling,
  sessionTypeLabel,
  speedKmh,
  type TimingRow,
} from "@/lib/raceTelemetry";
import {type StreamChannel, useDriverStreams} from "@/lib/useDriverStreams";
import {useBroadcastDemo} from "@/lib/broadcastDemo";
import {listDrivers, fmtScore} from "@/lib/driversApi";
import type {DriverSummary} from "@/types/driverStats";
import {useDriverCapture, fmtClipDuration} from "@/lib/useDriverCapture";
import {useAuthStore} from "@/stores/auth";
import {useToastStore} from "@/stores/toast";
import Icon from "@/components/ui/Icon.vue";
import StreamTheater from "@/components/StreamTheater.vue";

interface CurrentEvent {
  name: string;
  track: string;
  track_key: string;
  track_config: string;
  weather: string;
  weather_key: string;
}

interface StatusPayload {
  current_event: CurrentEvent;
}

interface TrackMapMeta {
  width: number;
  height: number;
  x_offset: number;
  z_offset: number;
  scale_factor: number;
  margin: number;
}

const route = useRoute();
const server = useServerStore();
const content = useContentStore();
const auth = useAuthStore();
const toast = useToastStore();
const cap = useDriverCapture();

// Auto mode (route /broadcast, no :id): follow the live action instead of one
// fixed instance — broadcast whichever running server has the most players, and
// hand off automatically when another server overtakes it. When nobody is
// online anywhere we drop to an all-servers leaderboard (below).
const autoMode = computed(() => route.name === "broadcast-auto");

// Busiest running server: most players, ties broken by lowest id (instanceList
// is id-sorted, and reduce keeps the incumbent on a tie). Undefined = empty.
const busiest = computed<InstanceState | undefined>(() => {
  const live = server.instanceList.filter((i) => i.running && i.players > 0);
  if (!live.length) return undefined;
  return live.reduce((best, i) => (i.players > best.players ? i : best));
});

const instanceId = computed(() =>
    autoMode.value ? (busiest.value?.id ?? -1) : Number(route.params.id),
);
const inst = computed<InstanceState | undefined>(() => server.instances[instanceId.value]);
const exitTo = computed(() => (autoMode.value ? "/" : `/server/${instanceId.value}`));

// All-servers leaderboard: shown only in auto mode when no server has players.
// Drift bests and best laps aggregated across every driver the backend knows.
const aggregate = computed(() => autoMode.value && !busiest.value);
const allDrivers = ref<DriverSummary[]>([]);
const leaderLoaded = ref(false);
async function loadLeaderboard() {
  try {
    allDrivers.value = await listDrivers();
  } catch {
    allDrivers.value = [];
  } finally {
    leaderLoaded.value = true;
  }
}
const topDrift = computed(() =>
    [...allDrivers.value]
        .filter((d) => d.best_drift > 0)
        .sort((a, b) => b.best_drift - a.best_drift)
        .slice(0, 12),
);
const topLap = computed(() =>
    [...allDrivers.value]
        .filter((d) => d.best_lap_ms > 0)
        .sort((a, b) => a.best_lap_ms - b.best_lap_ms)
        .slice(0, 12),
);

const detail = ref<StatusPayload | null>(null);
const mapMeta = ref<TrackMapMeta | null>(null);
const mapImageOk = ref(true);

// --- Client-only demo mode: fabricate a fake running session so the broadcast
// layout can be exercised without a live server. Nothing here hits the backend;
// every data source below simply switches to the demo feed while `debug` is on.
const debug = ref(false);
const demo = useBroadcastDemo(content);

// --- Live state straight from the store (SSE-updated), or the demo feed ---
const drivers = computed(() => (debug.value ? demo.drivers.value : (inst.value?.drivers ?? [])));
const positions = computed(() => (debug.value ? demo.positions.value : (inst.value?.positions ?? [])));
const session = computed(() => (debug.value ? demo.session.value : (inst.value?.session ?? null)));
const telemetry = computed(() => inst.value?.telemetry ?? null);
const running = computed(() => (debug.value ? true : (inst.value?.running ?? false)));
// Drift servers swap the lap-time rows for drift scores. Demo always shows the
// drift layout so it can be previewed without a live drift instance.
const isDrift = computed(() => (debug.value ? true : inst.value?.drift_score_enabled === 1));

const telemetryOnline = computed(() => (debug.value ? true : !!telemetry.value?.udp_online));

// Track on stage: the live session wins; the status payload's current event is
// the fallback before the first session frame lands.
const activeTrack = computed(() => {
  const s = session.value;
  if (s?.track) return {key: s.track, config: s.track_config ?? ""};
  const ev = detail.value?.current_event;
  if (ev?.track_key) return {key: ev.track_key, config: ev.track_config ?? ""};
  return null;
});

function trackUrl(kind: "map" | "mapmeta", key: string, config: string): string {
  const cfg = config ? `/${encodeURIComponent(config)}` : "";
  return `/api/track/${kind}/${encodeURIComponent(key)}${cfg}`;
}

const sessionTypeNum = computed(() => session.value?.type ?? 1);
const timingRows = computed<TimingRow[]>(() =>
    computeRunningOrder(drivers.value, positions.value, sessionTypeNum.value),
);

// --- Focused car for the telemetry block: the leader, unless the operator
// clicks another card. Falls back to the leader if that car drops out.
const pinnedCarId = ref<number | null>(null);
const focusRow = computed<TimingRow | null>(() => {
  const rows = timingRows.value;
  if (!rows.length) return null;
  if (pinnedCarId.value != null) {
    const pinned = rows.find((r) => r.car_id === pinnedCarId.value);
    if (pinned) return pinned;
  }
  return rows[0];
});

function focusCar(id: number) {
  pinnedCarId.value = pinnedCarId.value === id ? null : id;
}

const rpmMax = computed(() => rpmCeiling(positions.value));

function rpmPct(pos?: CarPositionState | null): number {
  if (!pos?.engine_rpm) return 0;
  return Math.max(0, Math.min(100, (pos.engine_rpm / rpmMax.value) * 100));
}

// --- Session clock ---
const clock = computed(() => {
  const s = session.value;
  if (!running.value || !s) return null;
  const timed = !s.laps && s.time > 0;
  const totalMs = timed ? s.time * 60000 : 0;
  return {
    type: sessionTypeLabel(s.type),
    index: (s.current_session_index ?? 0) + 1,
    count: s.session_count || 1,
    elapsed: formatClock(s.elapsed_ms),
    timed,
    remaining: timed ? formatClock(Math.max(0, totalMs - s.elapsed_ms)) : null,
    lapsLabel: s.laps ? `${s.laps} laps` : timed ? `${s.time} min` : "open",
    ambient: s.ambient_temp,
    road: s.road_temp,
  };
});

function carName(model: string): string {
  return content.carByKey(model)?.name || model;
}

function weatherImageUrl(key: string): string {
  return `/api/weather/preview/${encodeURIComponent(key)}`;
}

// Car + livery preview for a driver's card. The backend falls back through
// preview.jpg/png/livery.png; we just hide the <img> if nothing resolves.
// Demo drivers carry real car/skin keys, so this serves real previews there too.
function carImageUrl(model: string, skin: string): string {
  return `/api/car/image/${encodeURIComponent(model)}/${encodeURIComponent(skin || "")}`;
}

// Map plumbing resolves to the demo feed in debug mode, otherwise the real
// track image + meta (fetched below).
const effectiveMapMeta = computed<TrackMapMeta | null>(() =>
    debug.value ? demo.mapMeta.value : mapMeta.value,
);
const mapImageUrl = computed(() =>
    debug.value
        ? demo.mapImageUrl.value
        : activeTrack.value
            ? trackUrl("map", activeTrack.value.key, activeTrack.value.config)
            : "",
);

// --- Map geometry (mirrors ServerDetail's projection) ---
function mapWrapStyle(meta: TrackMapMeta) {
  const ratio = (meta.width || 16) / (meta.height || 9);
  // Fit the column: cap width so the derived height never exceeds the height
  // budget (--map-h). That budget is the full stage on desktop, but a small
  // slice on mobile where the map sits stacked above the cards — keeps the
  // map from overflowing the stage in the y direction.
  return {aspectRatio: String(ratio), width: `min(100%, calc(var(--map-h) * ${ratio}))`, margin: "auto"};
}

function mapPoint(pos: CarPositionState, meta: TrackMapMeta) {
  // AC map.ini projection — same maths the in-game minimap uses.
  // World (x,z) -> map.png pixels: divide by SCALE_FACTOR, add MARGIN.
  // The Z axis is NOT flipped: map.png pixel-Y already grows with world Z.
  const scale = meta.scale_factor || 1;
  const px = (pos.x + meta.x_offset) / scale + meta.margin;
  const py = (pos.z + meta.z_offset) / scale + meta.margin;
  return {
    left: `${Math.max(0, Math.min(100, (px / meta.width) * 100))}%`,
    top: `${Math.max(0, Math.min(100, (py / meta.height) * 100))}%`,
  };
}

function positionFor(carId: number): CarPositionState | undefined {
  return positions.value.find((p) => p.car_id === carId);
}

function timingFor(carId: number): TimingRow | undefined {
  return timingRows.value.find((r) => r.car_id === carId);
}

// --- Watchable driver streams ---
const driverStreams = useDriverStreams();
const theaterOpen = ref(false);
const theaterKey = ref<string | null>(null);
const streamChannels = computed<StreamChannel[]>(() =>
    debug.value ? demo.channels.value : driverStreams.allChannelsFor(drivers.value),
);
const onlineStreamCount = computed(() => streamChannels.value.filter((c) => c.online).length);
// guid → resolved channel, so each driver card can look up its stream in O(1).
const channelByGuid = computed(() => {
  const m = new Map<string, StreamChannel>();
  for (const c of streamChannels.value) {
    if (c.key.startsWith("driver:")) m.set(c.key.slice("driver:".length), c);
  }
  return m;
});

function guidForCar(carId: number): string | undefined {
  return drivers.value.find((d) => d.car_id === carId)?.guid;
}

function channelForCar(carId: number): StreamChannel | null {
  const guid = guidForCar(carId);
  return guid ? (channelByGuid.value.get(guid) ?? null) : null;
}

function openTheater(key: string | null) {
  theaterKey.value = key;
  theaterOpen.value = true;
}

function openStream(carId: number) {
  openTheater(`driver:${guidForCar(carId)}`);
}

// One card per driver, in running order, joined to its stream channel.
interface DriverCard {
  row: TimingRow;
  channel: StreamChannel | null;
}

const driverCards = computed<DriverCard[]>(() =>
    timingRows.value.map((row) => ({row, channel: channelForCar(row.car_id)})),
);

// --- per-driver capture (operate role): grab a still / record from a card ----
const snappingGuids = ref<Set<string>>(new Set());

async function takePic(guid: string | undefined) {
  if (!guid || debug.value || snappingGuids.value.has(guid)) return;
  snappingGuids.value.add(guid);
  try {
    await cap.takePicture(guid);
    toast.success("Picture captured.");
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e));
  } finally {
    snappingGuids.value.delete(guid);
  }
}

async function recordCar(guid: string | undefined) {
  if (!guid || debug.value) return;
  try {
    if (cap.isRecording(guid)) {
      await cap.stopRecording(guid);
      toast.success("Recording stopped — clip saved.");
    } else {
      await cap.recordNow(guid);
      toast.info("Recording — clip ends with the next drift run.");
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e));
  }
}

async function fetchStatus() {
  if (debug.value || instanceId.value <= 0) return;
  try {
    const payload = await api.get<StatusPayload & StatusResponse>(
        `/api/server/status?instance=${instanceId.value}`,
    );
    detail.value = payload;
    server.syncStatus(instanceId.value, payload);
  } catch {
    /* the store still feeds live data; ignore a one-off status miss */
  }
}

async function fetchMapMeta() {
  if (debug.value) return;
  const t = activeTrack.value;
  if (!t) {
    mapMeta.value = null;
    return;
  }
  try {
    mapMeta.value = await api.get<TrackMapMeta>(trackUrl("mapmeta", t.key, t.config));
    mapImageOk.value = true;
  } catch {
    mapMeta.value = null;
  }
}

watch(activeTrack, (next, prev) => {
  if (next?.key !== prev?.key || next?.config !== prev?.config) void fetchMapMeta();
});

// Auto mode handoff: when the followed instance changes (a server overtakes the
// current one, or the last players leave) drop pinned focus and re-pull the new
// server's status + map. The store already feeds live drivers/positions.
watch(instanceId, (id, prev) => {
  if (id === prev) return;
  pinnedCarId.value = null;
  if (id > 0) {
    void fetchStatus();
    void fetchMapMeta();
  }
});

// Refresh the leaderboard whenever we enter (or re-enter) the no-players view.
watch(
    aggregate,
    (on) => {
      if (on) void loadLeaderboard();
    },
    {immediate: true},
);

// Fullscreen helps on a dedicated broadcast screen; best-effort only.
function toggleFullscreen() {
  if (document.fullscreenElement) void document.exitFullscreen();
  else void document.documentElement.requestFullscreen().catch(() => {
  });
}

// Demo mode toggle + a "reshuffle" while it is on. Swapping `debug` flips every
// data source above; here we just spin the fake feed up and down.
function toggleDebug() {
  debug.value = !debug.value;
}

watch(debug, async (on) => {
  pinnedCarId.value = null;
  if (on) {
    mapImageOk.value = true;
    if (!content.loaded) await content.load();
    demo.regenerate();
    demo.start();
  } else {
    demo.stop();
    void fetchMapMeta();
  }
});

onMounted(async () => {
  if (!server.loaded) await server.load();
  void content.load();
  void driverStreams.loadStreams();
  driverStreams.startHealthPoll(() => instanceId.value);
  cap.startPoll();
  await fetchStatus();
  await fetchMapMeta();
});

let poll: ReturnType<typeof setInterval> | null = null;
onMounted(() => {
  // Light safety net for the current-event/track fallback if SSE drops a frame.
  poll = setInterval(() => void fetchStatus(), 20000);
});
onBeforeUnmount(() => {
  if (poll) clearInterval(poll);
  driverStreams.stopHealthPoll();
  cap.stopPoll();
  demo.stop();
});
</script>

<template>
  <div class="bcast fixed inset-0 z-50 overflow-hidden bg-bg text-text select-none">
    <!-- Atmosphere layers -->
    <div class="bcast-bg pointer-events-none absolute inset-0"/>
    <div class="bcast-scan pointer-events-none absolute inset-0"/>
    <div class="bcast-vignette pointer-events-none absolute inset-0"/>

    <!-- ░░ Top strap ░░ -->
    <header class="absolute inset-x-0 top-0 z-30 flex h-16 items-center gap-2 px-3 sm:gap-4 sm:px-5">
      <div class="flex min-w-0 items-center gap-2.5">
        <span
            class="inline-flex items-center gap-1.5 rounded-sm px-2 py-1 text-xs font-black tracking-[0.2em]"
            :class="debug ? 'bg-accent text-bg' : running ? 'bg-danger text-bg' : 'bg-surface-3 text-dim'"
        >
          <span v-if="running" class="live-dot size-1.5 rounded-full bg-bg"/>
          {{ debug ? "DEMO" : running ? "LIVE" : "OFFLINE" }}
        </span>
        <span
            v-if="autoMode"
            class="hidden items-center gap-1 rounded-sm bg-accent-dim px-1.5 py-1 text-[10px] font-black tracking-[0.2em] text-accent sm:inline-flex"
            title="Auto-following the busiest server"
        >
          <Icon name="repeat" :size="12"/>
          AUTO
        </span>
        <div class="min-w-0 leading-tight">
          <div class="truncate text-sm font-extrabold tracking-tight">{{
              debug ? "Demo Server" : aggregate ? "All Servers" : (inst?.name ?? "Server")
            }}
          </div>
          <div class="font-mono text-[11px] text-dim">
            <template v-if="debug">demo feed · {{ drivers.length }} cars</template>
            <template v-else-if="aggregate">leaderboard · no players online</template>
            <template v-else>:{{ inst?.tcp_port ?? "" }} · {{ drivers.length }} cars</template>
          </div>
        </div>
      </div>

      <!-- Session clock -->
      <div v-if="clock" class="mx-auto hidden items-center gap-5 sm:flex">
        <div class="text-right leading-none">
          <div class="text-[11px] font-bold tracking-[0.25em] text-accent uppercase">{{ clock.type }}</div>
          <div class="font-mono text-[10px] text-dim">Session {{ clock.index }} / {{ clock.count }}</div>
        </div>
        <div class="numerals text-5xl leading-none font-medium tracking-tight tabular-nums">
          {{ clock.elapsed }}
        </div>
        <div class="leading-none">
          <div v-if="clock.remaining" class="numerals text-xl text-warn tabular-nums">−{{ clock.remaining }}</div>
          <div class="font-mono text-[10px] tracking-wide text-dim uppercase">{{ clock.lapsLabel }}</div>
        </div>
      </div>
      <div v-else class="mx-auto hidden numerals text-3xl text-dim tracking-tight sm:block">— STANDBY —</div>

      <!-- Track + conditions -->
      <div class="ml-auto flex items-center gap-2 sm:gap-4">
        <div class="hidden text-right leading-tight md:block">
          <div class="max-w-[18ch] truncate text-sm font-bold">
            {{ detail?.current_event?.track || activeTrack?.key || "—" }}
          </div>
          <div class="font-mono text-[11px] text-dim">
            <span v-if="clock">{{ clock.ambient }}° air · {{ clock.road }}° track</span>
            <span v-else>{{ activeTrack?.config || "default" }}</span>
          </div>
        </div>
        <img
            v-if="detail?.current_event?.weather_key"
            :src="weatherImageUrl(detail.current_event.weather_key)"
            alt=""
            class="hidden size-9 rounded-full border border-line object-cover sm:block"
            @error="($event.target as HTMLImageElement).style.display = 'none'"
        />
        <div class="flex items-center gap-1.5">
          <span
              v-if="streamChannels.length"
              class="hidden h-9 items-center gap-1.5 rounded-md border border-line bg-surface/70 px-2.5 text-xs font-semibold text-muted sm:inline-flex"
              title="Driver streams online / total"
          >
            <Icon name="broadcast" :size="16"/>
            <span class="font-mono">{{ onlineStreamCount }}/{{ streamChannels.length }}</span>
          </span>
          <button
              v-if="debug"
              type="button"
              class="grid size-8 place-items-center sm:size-9 rounded-md border border-accent/60 bg-accent-dim text-accent transition-colors hover:bg-accent/20"
              title="Reshuffle demo grid"
              @click="demo.regenerate()"
          >
            <Icon name="repeat" :size="16"/>
          </button>
          <button
              type="button"
              class="grid size-8 place-items-center sm:size-9 rounded-md border transition-colors"
              :class="
              debug
                ? 'border-accent/60 bg-accent-dim text-accent'
                : 'border-line bg-surface/70 text-muted hover:border-line-hi hover:text-text'
            "
              title="Toggle demo mode (client-side fake data)"
              @click="toggleDebug"
          >
            <Icon name="shuffle" :size="16"/>
          </button>
          <button
              type="button"
              class="grid size-8 place-items-center sm:size-9 rounded-md border border-line bg-surface/70 text-muted transition-colors hover:border-line-hi hover:text-text"
              title="Toggle fullscreen"
              @click="toggleFullscreen"
          >
            <Icon name="maximize" :size="16"/>
          </button>
          <RouterLink
              :to="exitTo"
              class="grid size-8 place-items-center sm:size-9 rounded-md border border-line bg-surface/70 text-muted transition-colors hover:border-danger/60 hover:text-danger"
              title="Exit broadcast"
          >
            <Icon name="x" :size="16"/>
          </RouterLink>
        </div>
      </div>
    </header>

    <!-- ░░ Two-column stage: map (1/2) + driver cards (1/2) ░░ -->
    <div
        v-if="!aggregate"
        class="absolute inset-x-0 bottom-0 top-16 z-10 flex flex-col gap-3 px-3 pb-3 lg:flex-row lg:gap-4 lg:px-4 lg:pb-4"
    >
      <section
          class="relative flex w-full min-h-0 shrink-0 basis-2/5 items-center justify-center overflow-hidden rounded-xl border border-line bg-surface/30 backdrop-blur-sm lg:w-1/2 lg:basis-auto">
        <div
            v-if="activeTrack && effectiveMapMeta && mapImageOk"
            class="bcast-map relative"
            :style="mapWrapStyle(effectiveMapMeta)"
        >
          <!-- Padded inner stage so the track layout keeps some air around it -->
          <div class="absolute inset-[6%]">
            <img
                :src="mapImageUrl"
                alt="Track Map"
                class="absolute inset-0 size-full object-contain opacity-60 blend-luminosity"
                @error="mapImageOk = false"
            />

            <template v-for="d in drivers" :key="d.car_id">
            <button
                v-if="positionFor(d.car_id)"
                type="button"
                class="puck absolute -translate-x-1/2 -translate-y-1/2 transition-transform duration-200 ease-out hover:scale-110"
                :class="{ 'puck-focus z-20': focusRow?.car_id === d.car_id }"
                :style="mapPoint(positionFor(d.car_id)!, effectiveMapMeta)"
                @click="focusCar(d.car_id)"
            >
          <span
              v-if="timingFor(d.car_id)?.isLeader"
              class="puck-ring absolute inset-0 -m-1 animate-pulse rounded-full border border-accent/40 bg-accent/5"
          />

              <span
                  class="relative grid size-7 place-items-center rounded-full border-2 text-xs font-bold shadow-lg"
                  :class="
              timingFor(d.car_id)?.isLeader
                ? 'border-bg bg-accent text-bg shadow-accent/40'
                : focusRow?.car_id === d.car_id
                  ? 'border-accent bg-surface-4 text-accent'
                  : 'border-bg bg-surface-3 text-text'
            "
              >
            {{ (d.name || 'C' + d.car_id).slice(0, 1).toUpperCase() }}
          </span>

              <span
                  class="pointer-events-none absolute left-1/2 top-9 -translate-x-1/2 rounded bg-bg/90 px-1.5 py-0.5 font-mono text-[10px] font-medium tracking-tight text-text shadow-sm backdrop-blur-sm">
            {{ d.name || 'Car ' + d.car_id }}
          </span>
            </button>
          </template>
          </div>
        </div>

        <div v-else class="flex flex-col items-center p-6 text-center">
          <div class="mb-4 rounded-full bg-surface-3 p-4 text-dim">
            <Icon name="broadcast" :size="32"/>
          </div>
          <h3 class="numerals text-xl font-medium tracking-tight text-muted">
            {{ !running ? "SERVER OFFLINE" : !activeTrack ? "NO TRACK LOADED" : "NO LIVE MAP AVAILABLE" }}
          </h3>
          <RouterLink
              v-if="!running"
              :to="exitTo"
              class="mt-4 inline-flex items-center gap-2 rounded-lg border border-line bg-surface px-4 py-2 text-sm font-medium text-text shadow-sm transition-all hover:bg-surface-2"
          >
            <Icon name="arrowUp" :size="14" class="-rotate-90"/>
            Return to Control Panel
          </RouterLink>
        </div>

        <Transition name="fade">
          <div
              v-if="running && !telemetryOnline"
              class="absolute bottom-4 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-lg border border-warn/30 bg-warn-glow/90 px-4 py-2.5 text-xs font-medium text-warn shadow-lg backdrop-blur-md"
          >
            <span class="size-1.5 animate-ping rounded-full bg-warn"/>
            Waiting for Assetto Corsa telemetry plugin...
          </div>
        </Transition>
      </section>

      <section class="w-full min-w-0 flex-1 overflow-hidden">
        <TransitionGroup
            v-if="driverCards.length"
            tag="div"
            name="tower"
            class="flex h-full flex-col gap-3 overflow-y-auto pr-1"
        >
          <article
              v-for="card in driverCards"
              :key="card.row.car_id"
              class="tower-card flex h-auto shrink-0 cursor-pointer flex-col overflow-hidden rounded-xl border bg-surface/60 shadow-sm backdrop-blur-md transition-all duration-200 lg:h-[calc(50%-0.375rem)] lg:flex-row"
              :class="
          focusRow?.car_id === card.row.car_id
            ? 'border-accent bg-surface-2/80 ring-1 ring-accent/30'
            : card.row.isLeader
              ? 'border-accent/30 bg-accent-glow/5'
              : 'border-line hover:border-line-hi hover:bg-surface/90'
        "
              @click="focusCar(card.row.car_id)"
          >
            <!-- Metrics side -->
            <div class="flex w-full min-w-0 shrink-0 flex-col gap-3 p-4 lg:w-auto xl:w-72">
              <!-- Identity: position · driver + car -->
              <header class="flex items-center gap-3">
                <span
                    class="numerals grid size-9 shrink-0 place-items-center rounded-lg text-lg font-bold tabular-nums"
                    :class="card.row.isLeader ? 'bg-accent text-bg' : 'bg-surface-3 text-muted'"
                >
                  {{ card.row.position }}
                </span>
                <img
                    :src="carImageUrl(card.row.carModel, card.row.skin)"
                    alt=""
                    class="h-10 w-16 shrink-0 rounded border border-line bg-surface-4 object-cover lg:hidden"
                    @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"
                />
                <div class="min-w-0 flex-1">
                  <h4 class="truncate text-lg font-bold leading-tight text-text">{{ card.row.name }}</h4>
                  <p class="truncate font-mono text-xs tracking-tight text-dim lg:hidden">{{ carName(card.row.carModel) }}</p>
                </div>
                <!-- Driver-detail link (any role); capture controls live over the stream -->
                <RouterLink
                  v-if="guidForCar(card.row.car_id)"
                  :to="{ name: 'driver-detail', params: { guid: guidForCar(card.row.car_id)! } }"
                  target="_blank"
                  title="Open driver detail"
                  class="grid size-7 shrink-0 place-items-center rounded-md border border-line bg-surface-2/70 text-text/90 transition-colors hover:border-accent/50 hover:text-accent"
                  @click.stop
                >
                  <Icon name="user" :size="14" />
                </RouterLink>
              </header>

              <!-- Speed / gear + RPM bar -->
              <div class="flex flex-1 flex-col justify-center gap-3">
                <div class="flex items-end justify-between leading-none">
                  <div class="flex items-baseline">
                    <span class="numerals text-5xl font-light tabular-nums xl:text-6xl">{{ speedKmh(card.row.pos) }}</span>
                    <span class="ml-1 font-mono text-xs text-dim">km/h</span>
                  </div>
                  <div class="flex flex-col items-center">
                    <span class="numerals text-4xl font-bold text-accent tabular-nums xl:text-5xl">{{ gearLabel(card.row.pos) }}</span>
                    <span class="mt-0.5 font-mono text-[9px] tracking-widest text-dim uppercase">Gear</span>
                  </div>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-surface-3">
                  <div
                      class="h-full rounded-full transition-[width] duration-200 ease-linear"
                      :class="rpmPct(card.row.pos) > 88 ? 'bg-danger' : rpmPct(card.row.pos) > 70 ? 'bg-warn' : 'bg-accent'"
                      :style="{ width: `${rpmPct(card.row.pos)}%` }"
                  />
                </div>
                <!-- Car name + livery, below the RPM bar where there is room.
                     On mobile this moves inline into the name header. -->
                <div class="mt-3 hidden flex-col gap-1.5 lg:flex">
                  <p class="truncate text-center font-mono text-xs tracking-tight text-dim">{{ carName(card.row.carModel) }}</p>
                  <img
                      :src="carImageUrl(card.row.carModel, card.row.skin)"
                      alt=""
                      class="h-28 w-full shrink-0 rounded border border-line bg-surface-4 object-cover shadow-sm"
                      @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"
                  />
                </div>
              </div>

              <!-- Drift servers swap the Gap/Lap racing rows for the live drift
                   score; everyone else keeps the standings + lap times. -->
              <footer
                  class="border-t border-line/60 pt-3"
                  :class="isDrift ? 'grid grid-cols-3 gap-2 lg:flex lg:flex-col lg:gap-1.5' : 'flex flex-col gap-1.5'"
              >
                <template v-if="isDrift">
                  <div class="flex flex-col items-center lg:flex-row lg:items-center lg:justify-between">
                    <span class="font-mono text-xs tracking-wider text-dim uppercase">Live</span>
                    <span
                        class="numerals text-lg font-semibold tabular-nums"
                        :class="card.row.driftLive ? 'text-accent' : 'text-dim'"
                    >
                      {{ card.row.driftLive ? card.row.driftLive.toLocaleString() : "—" }}
                    </span>
                  </div>
                  <div class="flex flex-col items-center lg:flex-row lg:items-center lg:justify-between">
                    <span class="font-mono text-xs tracking-wider text-dim uppercase">Best</span>
                    <span
                        class="numerals text-lg font-semibold tabular-nums"
                        :class="card.row.driftBest ? 'text-accent' : 'text-dim'"
                    >
                      {{ card.row.driftBest ? card.row.driftBest.toLocaleString() : "—" }}
                    </span>
                  </div>
                  <div class="flex flex-col items-center lg:flex-row lg:items-center lg:justify-between">
                    <span class="font-mono text-xs tracking-wider text-dim uppercase">Last</span>
                    <span
                        class="numerals text-lg font-medium tabular-nums"
                        :class="card.row.driftLast ? 'text-text' : 'text-dim'"
                    >
                      {{ card.row.driftLast ? card.row.driftLast.toLocaleString() : "—" }}
                    </span>
                  </div>
                </template>
                <template v-else>
                  <div class="flex items-center justify-between">
                    <span class="font-mono text-xs tracking-wider text-dim uppercase">Gap</span>
                    <span
                        class="numerals text-lg font-semibold tracking-tight tabular-nums"
                        :class="card.row.gapTone === 'leader' ? 'text-accent' : card.row.gapTone === 'warn' ? 'text-warn' : 'text-text'"
                    >
                      {{ card.row.gapLabel }}
                    </span>
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="font-mono text-xs tracking-wider text-dim uppercase">Lap</span>
                    <span class="numerals text-lg tabular-nums text-text">{{ card.row.laps }}</span>
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="font-mono text-xs tracking-wider text-dim uppercase">Last Lap</span>
                    <span
                        class="numerals text-lg font-medium tabular-nums"
                        :class="card.row.last_lap_ms ? 'text-text' : 'text-dim'"
                    >
                      {{ lapTime(card.row.last_lap_ms) }}
                    </span>
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="font-mono text-xs tracking-wider text-dim uppercase">Best Lap</span>
                    <span
                        class="numerals text-lg font-semibold tabular-nums"
                        :class="card.row.best_lap_ms ? 'text-ok' : 'text-dim'"
                    >
                      {{ lapTime(card.row.best_lap_ms) }}
                    </span>
                  </div>
                </template>
              </footer>
            </div>

            <div class="relative aspect-video w-full min-h-0 flex-1 border-t border-line bg-bg lg:aspect-auto lg:w-auto xl:border-l xl:border-t-0">
              <!-- Capture controls (operate role): top-left over the stream -->
              <div
                v-if="auth.canOperate && guidForCar(card.row.car_id)"
                class="absolute left-3 top-3 z-10 flex items-center gap-1"
              >
                <button
                  type="button"
                  title="Take picture from this stream"
                  class="grid size-8 place-items-center rounded-lg border border-line bg-bg/60 text-muted shadow-sm backdrop-blur transition-all hover:border-accent/60 hover:bg-bg/90 hover:text-accent disabled:opacity-40"
                  :disabled="snappingGuids.has(guidForCar(card.row.car_id)!)"
                  @click.stop="takePic(guidForCar(card.row.car_id))"
                >
                  <Icon name="camera" :size="14" />
                </button>
                <button
                  type="button"
                  :title="cap.isRecording(guidForCar(card.row.car_id)!) ? 'Stop recording and save the clip' : 'Record clip (ends with next drift run)'"
                  class="flex h-8 items-center gap-1 rounded-lg border px-2 text-[10px] font-bold shadow-sm backdrop-blur transition-all"
                  :class="cap.isRecording(guidForCar(card.row.car_id)!)
                    ? 'border-danger/50 bg-danger-glow text-danger'
                    : 'border-line bg-bg/60 text-muted hover:border-danger/50 hover:bg-bg/90 hover:text-danger'"
                  @click.stop="recordCar(guidForCar(card.row.car_id))"
                >
                  <Icon :name="cap.isRecording(guidForCar(card.row.car_id)!) ? 'stop' : 'record'" :size="14" />
                  <span v-if="cap.isRecording(guidForCar(card.row.car_id)!)">{{ fmtClipDuration(cap.manualElapsed(guidForCar(card.row.car_id)!)) }}</span>
                </button>
                <span
                  v-if="cap.isBuffering(guidForCar(card.row.car_id)!)"
                  class="size-1.5 rounded-full bg-ok live-dot"
                  title="Rolling buffer recording — drift-run clips cut from this"
                />
              </div>
              <iframe
                  v-if="card.channel?.online"
                  :src="card.channel.url"
                  :title="card.row.name"
                  class="absolute inset-0 size-full border-0"
                  allow="autoplay; fullscreen; picture-in-picture"
                  sandbox="allow-scripts allow-same-origin allow-forms allow-presentation"
              />

              <div v-else class="grid size-full place-items-center bg-surface-2/20">
                <div class="flex flex-col items-center gap-2.5 text-dim">
                  <div class="grid size-11 place-items-center rounded-full border border-line bg-surface/40">
                    <Icon name="user" :size="18"/>
                  </div>
                  <span class="font-mono text-[11px] tracking-wider text-muted uppercase">
                {{ card.channel ? "Stream Offline" : "No Stream Linked" }}
              </span>
                </div>
              </div>

              <button
                  v-if="card.channel"
                  type="button"
                  class="absolute right-3 top-3 grid size-8 place-items-center rounded-lg border border-line bg-bg/60 text-muted shadow-sm backdrop-blur transition-all hover:border-accent/60 hover:bg-bg/90 hover:text-accent"
                  :aria-label="`Watch ${card.row.name} fullscreen`"
                  :title="`Watch ${card.row.name} fullscreen`"
                  @click.stop="openStream(card.row.car_id)"
              >
                <Icon name="maximize" :size="13"/>
              </button>
            </div>
          </article>
        </TransitionGroup>

        <div v-else class="grid h-full place-items-center rounded-xl border border-dashed border-line bg-surface/10">
          <div class="flex flex-col items-center p-6 text-center">
            <Icon name="broadcast" :size="36" class="animate-pulse text-dim"/>
            <h3 class="numerals mt-4 text-xl tracking-tight text-muted">
              {{ running ? "Waiting for competitors to join session…" : "Session is currently halted." }}
            </h3>
          </div>
        </div>
      </section>
    </div>

    <!-- ░░ All-servers leaderboard: auto mode, nobody online ░░ -->
    <div
        v-else
        class="absolute inset-x-0 bottom-0 top-16 z-10 flex flex-col gap-3 overflow-y-auto px-3 pb-3 lg:flex-row lg:gap-4 lg:px-4 lg:pb-4"
    >
      <!-- Top drift -->
      <section class="flex w-full min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-line bg-surface/30 backdrop-blur-sm">
        <header class="flex items-center gap-2 border-b border-line/60 px-4 py-3">
          <Icon name="broadcast" :size="16" class="text-accent"/>
          <h2 class="numerals text-lg tracking-tight">Top Drift</h2>
          <span class="ml-auto font-mono text-[11px] tracking-wider text-dim uppercase">all servers</span>
        </header>
        <ol class="flex-1 overflow-y-auto p-2">
          <li
              v-for="(d, i) in topDrift"
              :key="d.guid"
              class="flex items-center gap-3 rounded-lg px-3 py-2.5"
              :class="i === 0 ? 'bg-accent-glow/5' : i % 2 ? 'bg-surface/30' : ''"
          >
            <span
                class="numerals grid size-8 shrink-0 place-items-center rounded-lg text-base font-bold tabular-nums"
                :class="i === 0 ? 'bg-accent text-bg' : 'bg-surface-3 text-muted'"
            >{{ i + 1 }}</span>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-bold">{{ d.name }}</div>
              <div class="truncate font-mono text-[11px] text-dim">{{ d.favourite_track?.name || d.favourite_car?.name || "—" }}</div>
            </div>
            <span class="numerals text-xl font-semibold tabular-nums text-accent">{{ fmtScore(d.best_drift) }}</span>
            <span class="font-mono text-[10px] tracking-wider text-dim uppercase">pts</span>
          </li>
          <li v-if="!topDrift.length" class="px-3 py-6 text-center font-mono text-xs text-dim">
            {{ leaderLoaded ? "No drift scores recorded yet." : "Loading…" }}
          </li>
        </ol>
      </section>

      <!-- Best laps -->
      <section class="flex w-full min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-line bg-surface/30 backdrop-blur-sm">
        <header class="flex items-center gap-2 border-b border-line/60 px-4 py-3">
          <Icon name="activity" :size="16" class="text-ok"/>
          <h2 class="numerals text-lg tracking-tight">Best Laps</h2>
          <span class="ml-auto font-mono text-[11px] tracking-wider text-dim uppercase">all servers</span>
        </header>
        <ol class="flex-1 overflow-y-auto p-2">
          <li
              v-for="(d, i) in topLap"
              :key="d.guid"
              class="flex items-center gap-3 rounded-lg px-3 py-2.5"
              :class="i === 0 ? 'bg-accent-glow/5' : i % 2 ? 'bg-surface/30' : ''"
          >
            <span
                class="numerals grid size-8 shrink-0 place-items-center rounded-lg text-base font-bold tabular-nums"
                :class="i === 0 ? 'bg-ok text-bg' : 'bg-surface-3 text-muted'"
            >{{ i + 1 }}</span>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-bold">{{ d.name }}</div>
              <div class="truncate font-mono text-[11px] text-dim">{{ d.favourite_track?.name || d.favourite_car?.name || "—" }}</div>
            </div>
            <span class="numerals text-xl font-semibold tabular-nums text-ok">{{ lapTime(d.best_lap_ms) }}</span>
          </li>
          <li v-if="!topLap.length" class="px-3 py-6 text-center font-mono text-xs text-dim">
            {{ leaderLoaded ? "No lap times recorded yet." : "Loading…" }}
          </li>
        </ol>
      </section>
    </div>

    <StreamTheater
        :open="theaterOpen"
        :channels="streamChannels"
        :initial-key="theaterKey"
        @close="theaterOpen = false"
    />
  </div>
</template>

<style scoped>
/* Distinctive broadcast display face for big numerals/timers; body text stays
   on the app's Plus Jakarta Sans for cohesion. */
@import url("https://fonts.googleapis.com/css2?family=Saira+Condensed:wght@400;500;600;700&display=swap");

.numerals {
  font-family: "Saira Condensed", "Plus Jakarta Sans", sans-serif;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.01em;
}

/* Blueprint grid + depth glow behind the stage. */
.bcast-bg {
  background-color: var(--color-bg);
  background-image: radial-gradient(ellipse 80% 60% at 50% 18%, rgba(98, 179, 232, 0.1), transparent 70%),
  linear-gradient(var(--color-line) 1px, transparent 1px),
  linear-gradient(90deg, var(--color-line) 1px, transparent 1px);
  background-size: 100% 100%,
  44px 44px,
  44px 44px;
  background-position: 0 0,
  -1px -1px,
  -1px -1px;
  opacity: 0.9;
}

/* Cinematic vignette so the corners fall off behind the overlays. */
.bcast-vignette {
  background: radial-gradient(ellipse 75% 75% at 50% 45%, transparent 55%, rgba(0, 0, 0, 0.55) 100%);
}

/* Faint broadcast scanlines, drifting slowly. */
.bcast-scan {
  background: repeating-linear-gradient(
      to bottom,
      rgba(255, 255, 255, 0.018) 0px,
      rgba(255, 255, 255, 0.018) 1px,
      transparent 2px,
      transparent 4px
  );
  animation: scan 14s linear infinite;
  mix-blend-mode: overlay;
}

@keyframes scan {
  from {
    background-position-y: 0;
  }
  to {
    background-position-y: 200px;
  }
}

/* Map height budget: a small slice on mobile (map stacked above cards),
   the full stage minus the top strap on desktop. mapWrapStyle() derives the
   map width from this so the layout never overflows vertically. */
.bcast {
  --map-h: 36vh;
}
@media (min-width: 1024px) {
  .bcast {
    --map-h: calc(100vh - 6rem);
  }
}

.bcast-map {
  border: 1px solid var(--color-line-hi);
  border-radius: var(--radius-md);
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.55),
  inset 0 0 60px rgba(98, 179, 232, 0.05);
}

/* Car pucks glide between SSE position frames (≈200ms publish cadence). */
.puck {
  transition: left 0.2s linear,
  top 0.2s linear;
  z-index: 1;
}

.puck:hover,
.puck-focus {
  z-index: 10;
}

/* Leader gets a slow expanding ring. */
.puck-ring {
  border: 2px solid var(--color-accent);
  animation: ring 1.8s ease-out infinite;
}

@keyframes ring {
  0% {
    transform: scale(0.7);
    opacity: 0.8;
  }
  100% {
    transform: scale(1.9);
    opacity: 0;
  }
}

.live-dot {
  animation: blink 1.1s steps(1, end) infinite;
}

@keyframes blink {
  0%,
  60% {
    opacity: 1;
  }
  61%,
  100% {
    opacity: 0.15;
  }
}

/* Timing tower reordering: smooth FLIP move as positions change. */
.tower-move {
  transition: transform 0.5s cubic-bezier(0.22, 0.9, 0.3, 1);
}

.tower-enter-active,
.tower-leave-active {
  transition: all 0.35s ease;
}

.tower-enter-from,
.tower-leave-to {
  opacity: 0;
  transform: translateY(12px);
}

.tower-leave-active {
  position: absolute;
}
</style>
