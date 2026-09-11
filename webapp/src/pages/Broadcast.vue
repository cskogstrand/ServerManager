<script setup lang="ts">
// Broadcast view for the workspace or a dedicated instance — a trackside TV graphics
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
  interpolatePosition,
  lapTime,
  positionFrameMs,
  rpmCeiling,
  sessionTypeLabel,
  speedKmh,
  trackMapPoint,
  type TimingRow,
} from "@/lib/raceTelemetry";
import {type StreamChannel, useDriverStreams} from "@/lib/useDriverStreams";
import {useBroadcastDemo} from "@/lib/broadcastDemo";
import {useDriverCapture, fmtClipDuration} from "@/lib/useDriverCapture";
import {useAuthStore} from "@/stores/auth";
import {useToastStore} from "@/stores/toast";
import Icon from "@/components/ui/Icon.vue";
import StreamTheater from "@/components/StreamTheater.vue";
import SourcePlayer from "@/components/SourcePlayer.vue";
import BroadcastLeaderboard from "@/components/BroadcastLeaderboard.vue";

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

const props = defineProps<{ targetInstanceId?: number }>();
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
    autoMode.value ? (busiest.value?.id ?? -1) : (props.targetInstanceId ?? Number(route.params.id)),
);
const inst = computed<InstanceState | undefined>(() => server.instances[instanceId.value]);
const exitTo = computed(() => (autoMode.value ? "/" : `/server/${instanceId.value}`));

// All-servers leaderboard: shown only in auto mode when no server has players.
// The aggregated drift/lap board + clip popup live in <BroadcastLeaderboard>,
// which loads and refreshes its own data; here we only decide when to show it.
const aggregate = computed(() => autoMode.value && !busiest.value);

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
const livePositions = computed(() => inst.value?.positions ?? []);
const smoothedLivePositions = ref<CarPositionState[]>([]);
const positions = computed(() => (debug.value ? demo.positions.value : smoothedLivePositions.value));
const session = computed(() => (debug.value ? demo.session.value : (inst.value?.session ?? null)));
const telemetry = computed(() => inst.value?.telemetry ?? null);
const running = computed(() => (debug.value ? true : (inst.value?.running ?? false)));
// Drift servers swap the lap-time rows for drift scores. Demo always shows the
// drift layout so it can be previewed without a live drift instance.
const isDrift = computed(() => (debug.value ? true : inst.value?.drift_score_enabled === 1));

const telemetryOnline = computed(() => (debug.value ? true : !!telemetry.value?.udp_online));

interface PositionTween {
  from: CarPositionState;
  to: CarPositionState;
  started: number;
  duration: number;
}

const positionTweens = new Map<number, PositionTween>();
let positionFrame: number | null = null;

function samePositionFrame(a: CarPositionState, b: CarPositionState): boolean {
  return a.updated_at === b.updated_at && a.engine_rpm === b.engine_rpm && a.x === b.x && a.z === b.z;
}

function cancelPositionFrame() {
  if (positionFrame != null) cancelAnimationFrame(positionFrame);
  positionFrame = null;
}

function stopPositionSmoothing() {
  cancelPositionFrame();
  positionTweens.clear();
}

function renderPositionTweens(now = performance.now()) {
  let active = false;
  smoothedLivePositions.value = livePositions.value.map((target) => {
    const tween = positionTweens.get(target.car_id);
    if (!tween) return target;
    const progress = Math.min(1, (now - tween.started) / tween.duration);
    if (progress >= 1) {
      positionTweens.delete(target.car_id);
      return target;
    }
    active = true;
    return interpolatePosition(tween.from, tween.to, progress);
  });
  positionFrame = active ? requestAnimationFrame(renderPositionTweens) : null;
}

function smoothLivePositionSnapshot(next: CarPositionState[], prev: CarPositionState[]) {
  const now = performance.now();
  const current = new Map(smoothedLivePositions.value.map((p) => [p.car_id, p]));
  const previous = new Map(prev.map((p) => [p.car_id, p]));
  const liveIds = new Set(next.map((p) => p.car_id));
  for (const id of positionTweens.keys()) {
    if (!liveIds.has(id)) positionTweens.delete(id);
  }
  for (const target of next) {
    const oldTarget = previous.get(target.car_id);
    if (!oldTarget || samePositionFrame(oldTarget, target)) continue;
    positionTweens.set(target.car_id, {
      from: current.get(target.car_id) ?? oldTarget,
      to: target,
      started: now,
      duration: positionFrameMs(oldTarget, target),
    });
  }
  cancelPositionFrame();
  if (positionTweens.size) renderPositionTweens(now);
  else smoothedLivePositions.value = next;
}

watch(
    livePositions,
    (next, prev = []) => {
      if (!debug.value) smoothLivePositionSnapshot(next, prev);
    },
    {immediate: true},
);

// Track on stage: prefer the event's content-cache key. AC's live session track
// string can differ from the configured key/layout, while telemetry projection
// must use the same map asset ServerDetail uses.
const activeTrack = computed(() => {
  const ev = detail.value?.current_event;
  if (ev?.track_key) return {key: ev.track_key, config: ev.track_config ?? ""};
  const s = session.value;
  if (s?.track) return {key: s.track, config: s.track_config ?? ""};
  return null;
});
const activeTrackVersion = computed(() => activeTrack.value ? (content.trackByKey(activeTrack.value.key, activeTrack.value.config)?.version ?? "") : "");

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
const cameraOverride = ref<string | null>(null);
const playbackState = ref("Playback not confirmed");
const fullscreen = ref(false);
const isDevelopment = import.meta.env.DEV;
const focusRow = computed<TimingRow | null>(() => {
  if (cameraOverride.value) return null;
  const rows = timingRows.value;
  if (!rows.length) return null;
  if (pinnedCarId.value != null) {
    const pinned = rows.find((r) => r.car_id === pinnedCarId.value);
    if (pinned) return pinned;
  }
  return rows[0];
});

function focusCar(id: number) {
  cameraOverride.value = null;
  pinnedCarId.value = id;
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

const MAP_EDGE_INSET_PCT = 6;

// --- Map geometry (mirrors ServerDetail's projection) ---
function mapWrapStyle(meta: TrackMapMeta) {
  const ratio = (meta.width || 16) / (meta.height || 9);
  // Fit the actual map panel, including the space used by app navigation.
  return {aspectRatio: String(ratio), width: `min(100%, calc(100cqh * ${ratio}))`, margin: "auto"};
}

function timingFor(carId: number): TimingRow | undefined {
  return timingRows.value.find((r) => r.car_id === carId);
}

const mapCars = computed(() => {
  const meta = effectiveMapMeta.value;
  if (!meta) return [];
  return drivers.value.flatMap((driver) => {
    const pos = positions.value.find((p) => p.car_id === driver.car_id);
    return pos
        ? [{driver, point: trackMapPoint(pos, meta, MAP_EDGE_INSET_PCT)}]
        : [];
  });
});

function offMapArrowStyle(angleDeg: number) {
  return {transform: `rotate(${angleDeg + 90}deg)`};
}

function distanceLabel(meters: number): string {
  if (meters >= 950) return `${Number((meters / 1000).toFixed(1))} km`;
  return `${Math.max(1, Math.round(meters))} m`;
}

// --- Watchable driver streams ---
const driverStreams = useDriverStreams();
const theaterOpen = ref(false);
const theaterKey = ref<string | null>(null);
const streamChannels = computed<StreamChannel[]>(() =>
    debug.value ? demo.channels.value : driverStreams.allChannelsFor(drivers.value).filter(c => c.instanceId === instanceId.value || drivers.value.some(d => d.guid === c.driverGuid)),
);
const onlineStreamCount = computed(() => streamChannels.value.filter((c) => c.online).length);
// guid → resolved channel, so each driver card can look up its stream in O(1).
const channelByGuid = computed(() => {
  const m = new Map<string, StreamChannel>();
  for (const c of streamChannels.value) {
    if (c.driverGuid) m.set(c.driverGuid, c);
    else if (debug.value && c.key.startsWith("driver:")) m.set(c.key.slice("driver:".length), c);
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

const featured = computed(() => cameraOverride.value ? streamChannels.value.find(c => c.key === cameraOverride.value) ?? null : focusRow.value ? channelForCar(focusRow.value.car_id) : streamChannels.value.find(c => !c.driverGuid) ?? null);
function chooseCamera(channel: StreamChannel) {
  const driver = drivers.value.find(d => d.guid === channel.driverGuid);
  if (driver) focusCar(driver.car_id);
  else { cameraOverride.value = channel.key; pinnedCarId.value = null; }
}
watch(() => featured.value?.key, () => { playbackState.value = 'Playback not confirmed'; });
const captureTarget = computed(()=>featured.value?.key.startsWith('source:')?featured.value.key:featured.value?.driverGuid);
const telemetryDelayed = computed(() => !telemetryOnline.value || (!debug.value && (cap.nowMs.value || Date.now()) - (telemetry.value?.last_position_ms ?? 0) > 5000));
function fullscreenChanged() { fullscreen.value = !!document.fullscreenElement; }
onMounted(() => document.addEventListener('fullscreenchange', fullscreenChanged));
onBeforeUnmount(() => {document.removeEventListener('fullscreenchange', fullscreenChanged);if(document.fullscreenElement)void document.exitFullscreen();});

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
      toast.success("Recording stopped. The clip is being assembled; check Saved moments for the result.");
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
  if (next?.key !== prev?.key || next?.config !== prev?.config) {
    void fetchMapMeta();
  }
});

// Auto mode handoff: when the followed instance changes (a server overtakes the
// current one, or the last players leave) drop pinned focus and re-pull the new
// server's status + map. The store already feeds live drivers/positions.
watch(instanceId, (id, prev) => {
  if (id === prev) return;
  pinnedCarId.value = null;
  cameraOverride.value = null;
  if (id > 0) {
    void fetchStatus();
    void fetchMapMeta();
  }
});

// Fullscreen helps on a dedicated broadcast screen; best-effort only.
function toggleFullscreen() {
  if (document.fullscreenElement) void document.exitFullscreen();
  else void document.documentElement.requestFullscreen().catch(() => { toast.error("Fullscreen is unavailable in this browser."); });
}

// Demo mode toggle + a "reshuffle" while it is on. Swapping `debug` flips every
// data source above; here we just spin the fake feed up and down.
function toggleDebug() {
  debug.value = !debug.value;
}

watch(debug, async (on) => {
  pinnedCarId.value = null;
  if (on) {
    stopPositionSmoothing();
    mapImageOk.value = true;
    if (!content.loaded) await content.load();
    demo.regenerate();
    demo.start();
  } else {
    demo.stop();
    smoothedLivePositions.value = livePositions.value;
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
  stopPositionSmoothing();
  demo.stop();
});
</script>

<template>
  <div class="bcast pitlane-watch isolate bg-bg text-text" :class="[{'watch-fullscreen':fullscreen}, autoMode ? 'relative min-h-0 flex-1' : 'fixed inset-0 z-50']">

    <!-- ░░ Top strap ░░ -->
    <header class="absolute inset-x-0 top-0 z-30 flex h-16 items-center gap-2 px-3 @min-[640px]/broadcast:gap-4 @min-[640px]/broadcast:px-5">
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
            class="hidden items-center gap-1 rounded-sm bg-accent-dim px-1.5 py-1 text-[10px] font-black tracking-[0.2em] text-accent @min-[640px]/broadcast:inline-flex"
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
      <div v-if="clock" class="mx-auto hidden items-center gap-5 @min-[640px]/broadcast:flex">
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
      <div v-else class="mx-auto hidden numerals text-3xl text-dim tracking-tight @min-[640px]/broadcast:block">— STANDBY —</div>

      <!-- Track + conditions -->
      <div class="ml-auto flex items-center gap-2 @min-[640px]/broadcast:gap-4">
        <div class="hidden text-right leading-tight @min-[768px]/broadcast:block">
          <div class="max-w-[18ch] truncate text-sm font-bold">
            {{ detail?.current_event?.track || activeTrack?.key || "—" }}
          </div>
          <div class="font-mono text-[11px] text-dim">
            <span v-if="clock">{{ clock.ambient }}° air · {{ clock.road }}° track<span v-if="activeTrackVersion"> · v{{ activeTrackVersion }}</span></span>
            <span v-else>{{ activeTrack?.config || "default" }}<span v-if="activeTrackVersion"> · v{{ activeTrackVersion }}</span></span>
          </div>
        </div>
        <img
            v-if="detail?.current_event?.weather_key"
            :src="weatherImageUrl(detail.current_event.weather_key)"
            alt=""
            class="hidden size-9 rounded-full border border-line object-cover @min-[640px]/broadcast:block"
            @error="($event.target as HTMLImageElement).style.display = 'none'"
        />
        <div class="flex items-center gap-1.5">
          <span
              v-if="streamChannels.length"
              class="hidden h-9 items-center gap-1.5 rounded-md border border-line bg-surface/70 px-2.5 text-xs font-semibold text-muted @min-[640px]/broadcast:inline-flex"
              title="Driver streams online / total"
          >
            <Icon name="broadcast" :size="16"/>
            <span class="font-mono">{{ onlineStreamCount }}/{{ streamChannels.length }}</span>
          </span>
          <button
              v-if="debug"
              type="button"
              class="grid size-8 place-items-center @min-[640px]/broadcast:size-9 rounded-md border border-accent/60 bg-accent-dim text-accent transition-colors hover:bg-accent/20"
              title="Reshuffle demo grid"
              @click="demo.regenerate()"
          >
            <Icon name="repeat" :size="16"/>
          </button>
          <button
              type="button"
              class="grid size-8 place-items-center @min-[640px]/broadcast:size-9 rounded-md border transition-colors"
              :class="
              debug
                ? 'border-accent/60 bg-accent-dim text-accent'
                : 'border-line bg-surface/70 text-muted hover:border-line-hi hover:text-text'
            "
              v-if="isDevelopment"
              title="Toggle explicit sample telemetry (development only)"
              @click="toggleDebug"
          >
            <Icon name="shuffle" :size="16"/>
          </button>
          <button
              type="button"
              class="grid size-8 place-items-center @min-[640px]/broadcast:size-9 rounded-md border border-line bg-surface/70 text-muted transition-colors hover:border-line-hi hover:text-text"
               :title="fullscreen ? 'Exit fullscreen' : 'Fullscreen'" :aria-label="fullscreen ? 'Exit fullscreen' : 'Fullscreen'"
              @click="toggleFullscreen"
          >
            <Icon name="maximize" :size="16"/>
          </button>
          <RouterLink
              :to="exitTo"
              class="grid size-8 place-items-center @min-[640px]/broadcast:size-9 rounded-md border border-line bg-surface/70 text-muted transition-colors hover:border-danger/60 hover:text-danger"
              title="Exit broadcast"
          >
            <Icon name="x" :size="16"/>
          </RouterLink>
        </div>
      </div>
    </header>

    <div v-if="!aggregate" class="watch-workspace">
      <div class="watch-context"><span>{{ debug ? 'Sample telemetry · development fixture' : 'Live camera, track and timing' }}</span><RouterLink v-if="auth.isAdmin" :to="featured ? `/garage/rigs?source=${encodeURIComponent(featured.key)}` : '/garage/rigs?cameras=1'" class="pitlane-button">Set up streams</RouterLink></div>
      <div class="watch-stage">
        <section class="watch-video-panel">
          <SourcePlayer v-if="featured" :url="featured.url" :name="featured.title" selected @playing="playbackState='Playing'" @error="playbackState=$event" />
          <div v-else class="watch-no-camera"><Icon name="camera" :size="32"/><h2>{{ focusRow ? `No camera linked to ${focusRow.name}` : 'No camera selected' }}</h2><p>Timing and positions remain available independently.</p><RouterLink v-if="auth.isAdmin" to="/garage/rigs?cameras=1" class="pitlane-button">Set up a stream</RouterLink></div>
        </section>
      <section
          class="watch-map-panel relative [container-type:size] flex items-center justify-center overflow-hidden rounded-xl border border-line bg-surface/30">
        <div
            v-if="activeTrack && effectiveMapMeta && mapImageOk"
            class="bcast-map relative"
            :style="mapWrapStyle(effectiveMapMeta)"
        >
          <!-- Padded inner stage so the track layout keeps some air around it -->
          <div class="absolute inset-[6%] overflow-hidden">
            <img
                :src="mapImageUrl"
                alt="Track Map"
                class="absolute inset-0 size-full object-fill opacity-60 blend-luminosity"
                @error="mapImageOk = false"
            />

            <button
                v-for="car in mapCars"
                :key="car.driver.car_id"
                type="button"
                class="puck absolute -translate-x-1/2 -translate-y-1/2 transition-[top,left,transform] duration-200 ease-linear hover:scale-110"
                :class="{ 'puck-focus z-20': focusRow?.car_id === car.driver.car_id, 'puck-offmap': !car.point.inBounds }"
                :style="{ left: car.point.left, top: car.point.top }"
                @click="focusCar(car.driver.car_id)"
            >
              <span
                  v-if="!car.point.inBounds"
                  class="offmap-pin relative inline-flex h-7 items-center gap-1 rounded-full border border-warn/70 bg-bg/90 px-2 font-mono text-[10px] font-bold text-warn shadow-lg backdrop-blur-sm"
              >
                <Icon name="arrowUp" :size="13" class="shrink-0" :style="offMapArrowStyle(car.point.angleDeg)" />
                <span class="grid size-4 place-items-center rounded-full bg-warn text-[9px] font-black text-bg">
                  {{ (car.driver.name || 'C' + car.driver.car_id).slice(0, 1).toUpperCase() }}
                </span>
                <span>{{ distanceLabel(car.point.distanceMeters) }}</span>
              </span>

              <template v-else>
          <span
              v-if="timingFor(car.driver.car_id)?.isLeader"
              class="puck-ring absolute inset-0 -m-1 animate-pulse rounded-full border border-accent/40 bg-accent/5"
          />

              <span
                  class="relative grid size-7 place-items-center rounded-full border-2 text-xs font-bold shadow-lg"
                  :class="
              timingFor(car.driver.car_id)?.isLeader
                ? 'border-bg bg-accent text-bg shadow-accent/40'
                : focusRow?.car_id === car.driver.car_id
                  ? 'border-accent bg-surface-4 text-accent'
                  : 'border-bg bg-surface-3 text-text'
            "
              >
            {{ (car.driver.name || 'C' + car.driver.car_id).slice(0, 1).toUpperCase() }}
          </span>

              <span
                  class="pointer-events-none absolute left-1/2 top-9 -translate-x-1/2 rounded bg-bg/90 px-1.5 py-0.5 font-mono text-[10px] font-medium tracking-tight text-text shadow-sm backdrop-blur-sm">
            {{ car.driver.name || 'Car ' + car.driver.car_id }}
          </span>
              </template>
            </button>
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
      </div>
      <div class="watch-lower">
        <section>
          <div class="watch-featured-label"><div><h2>{{ featured?.title || focusRow?.name || 'Camera' }}</h2><p>{{ featured ? (featured.kind==='iframe'?'Embedded player · playback status not observable':playbackState) : 'No player configured' }}</p></div><button v-if="featured" class="pitlane-button" @click="openTheater(featured.key)">Enlarge camera</button></div>
          <div v-if="focusRow" class="watch-instruments"><div><span>Speed</span><b>{{ telemetryDelayed ? '—' : speedKmh(focusRow.pos) }} <small>km/h</small></b></div><div><span>Gear</span><b>{{ telemetryDelayed ? '—' : gearLabel(focusRow.pos) }}</b></div><div><span>RPM</span><b>{{ telemetryDelayed ? '—' : focusRow.pos?.engine_rpm || '—' }}</b></div><div class="watch-rpm"><i :style="{width:rpmPct(focusRow.pos)+'%'}" /></div></div>
          <p v-if="focusRow && telemetryDelayed" role="status" class="text-warn text-sm my-3">Position updates delayed. Last update: {{ telemetry?.last_position_ms ? new Date(telemetry.last_position_ms).toLocaleTimeString() : 'not received' }}. Last timing values are retained.</p>
          <div v-if="auth.canOperate && captureTarget" class="pitlane-actions my-4"><button class="pitlane-button" :disabled="debug || !cap.isBuffering(captureTarget) || snappingGuids.has(captureTarget)" @click="takePic(captureTarget)">Snapshot</button><button class="pitlane-button" :disabled="debug || !cap.isBuffering(captureTarget) && !cap.isRecording(captureTarget)" @click="recordCar(captureTarget)">{{ cap.isRecording(captureTarget) ? `Stop recording · ${fmtClipDuration(cap.manualElapsed(captureTarget))}` : 'Record clip' }}</button><span class="text-xs text-muted">{{ cap.isBuffering(captureTarget)?'Recorder buffering':'Recorder not ready' }}</span><RouterLink :to="captureTarget.startsWith('source:')?`/garage/cameras/${encodeURIComponent(captureTarget)}`:`/drivers/${encodeURIComponent(captureTarget)}`" class="pitlane-button">Saved moments</RouterLink></div>
          <div class="watch-camera-strip"><button v-for="channel in streamChannels" :key="channel.key" :aria-pressed="featured?.key===channel.key" @click="chooseCamera(channel)"><span>{{ channel.title }}</span><small>{{ channel.health === 'live' ? 'Status responding' : channel.health }}</small></button></div>
        </section>
        <section class="watch-timing"><h2>On the circuit</h2><div class="overflow-x-auto"><table><thead><tr><th>Pos</th><th>Driver</th><th>{{ isDrift?'Live drift':'Last lap' }}</th><th>{{ isDrift?'Best drift':'Best lap' }}</th><th>Gap</th></tr></thead><tbody><tr v-for="card in driverCards" :key="card.row.car_id" :class="{'watch-selected':focusRow?.car_id===card.row.car_id}"><td>{{ card.row.position }}</td><td><button @click="focusCar(card.row.car_id)" :aria-pressed="focusRow?.car_id===card.row.car_id"><img :src="carImageUrl(card.row.carModel,card.row.skin)" alt="" @error="($event.target as HTMLImageElement).hidden=true"/><span>{{ card.row.name }}<small>{{ carName(card.row.carModel) }}</small></span></button></td><td>{{ isDrift?card.row.driftLive.toLocaleString():lapTime(card.row.last_lap_ms) }}</td><td>{{ isDrift?card.row.driftBest.toLocaleString():lapTime(card.row.best_lap_ms) }}</td><td>{{ card.row.gapLabel }}</td></tr></tbody></table></div><p v-if="!driverCards.length" class="text-muted my-5">No drivers connected. Configured spectator cameras remain available.</p></section>
      </div>
    </div>

    <!-- ░░ All-servers leaderboard: auto mode, nobody online ░░ -->
    <div
        v-else
        class="absolute inset-x-0 bottom-0 top-16 z-10 px-3 pb-3 @min-[1024px]/broadcast:px-4 @min-[1024px]/broadcast:pb-4"
    >
      <BroadcastLeaderboard/>
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
.pitlane-watch { overflow:auto; --watch-panel-height:clamp(300px,43vh,650px); }
.watch-workspace{padding:80px 24px 36px;min-width:0}.watch-context{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:16px;font-size:13px;color:var(--color-muted)}
.watch-stage,.watch-lower{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:24px}.watch-video-panel,.watch-map-panel{height:var(--watch-panel-height);min-width:0;border:1px solid var(--color-line);border-radius:12px;overflow:hidden;background:#111b17}.watch-map-panel{padding:24px}.watch-lower{margin-top:22px}.watch-no-camera{height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:18px;padding:24px;text-align:center;color:#edf1e8}.watch-no-camera p{font-size:13px;color:#adb6aa}.watch-featured-label{display:flex;justify-content:space-between;align-items:center;gap:12px}.watch-featured-label h2,.watch-timing h2{font-size:20px;font-weight:550}.watch-featured-label p{font-size:12px;color:var(--color-muted);margin-top:7px}.watch-instruments{display:flex;position:relative;gap:40px;padding:24px 0}.watch-instruments span{display:block;color:var(--color-muted);font-size:12px;margin-bottom:6px}.watch-instruments b{font:28px var(--font-mono)}.watch-instruments small{font-size:12px}.watch-rpm{position:absolute;bottom:0;left:0;width:100%;height:4px;background:var(--color-line)}.watch-rpm i{display:block;height:100%;background:var(--color-accent)}.watch-camera-strip{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin-top:20px}.watch-camera-strip button{min-width:0;min-height:78px;padding:12px;border:1px solid var(--color-line);border-radius:9px;text-align:left}.watch-camera-strip button[aria-pressed=true]{border-color:var(--color-accent);background:var(--color-accent-dim)}.watch-camera-strip span{display:block;font-size:14px;overflow-wrap:anywhere}.watch-camera-strip small{display:block;margin-top:8px;color:var(--color-muted)}.watch-timing table{width:100%;font-size:13px;border-collapse:collapse;margin-top:12px}.watch-timing td,.watch-timing th{text-align:left;padding:12px 8px;border-bottom:1px solid var(--color-line)}.watch-timing th{font-size:11px;color:var(--color-muted)}.watch-timing button{display:flex;align-items:center;gap:10px;min-height:44px;text-align:left}.watch-timing img{width:50px;height:34px;object-fit:cover;border-radius:4px}.watch-timing small{display:block;font-size:11px;color:var(--color-muted)}.watch-selected{background:var(--color-accent-dim)}
@media(min-width:1440px){.pitlane-watch.watch-fullscreen{--watch-panel-height:clamp(360px,48vh,900px)}.watch-fullscreen .watch-stage,.watch-fullscreen .watch-lower{gap:32px}.watch-fullscreen .watch-map-panel{padding:32px}.watch-fullscreen .puck>span:first-child{min-width:34px;min-height:34px}.watch-fullscreen .watch-camera-strip{grid-template-columns:repeat(4,minmax(0,1fr))}}
@media(max-width:767px){.watch-workspace{padding:80px 12px 24px}.watch-stage,.watch-lower{grid-template-columns:1fr;gap:16px}.watch-video-panel{height:auto;aspect-ratio:16/9}.watch-map-panel{height:340px}.watch-video-panel:has(.watch-no-camera){min-height:220px}.watch-camera-strip{grid-template-columns:repeat(2,minmax(0,1fr))}.watch-instruments{gap:28px}}
/* Stable-width numerals keep live timing readable as values update. */

.numerals {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.01em;
}

/* The app rail changes the available width independently of the viewport. */
.bcast {
  container: broadcast / size;
}

.bcast-map {
  overflow: hidden;
  border: 1px solid var(--color-line-hi);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
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

.puck-offmap {
  z-index: 15;
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

/* Timing tower reordering: smooth FLIP move as positions change.
   Selectors below are generated by <TransitionGroup name="tower"> at runtime. */
/*noinspection CssUnusedSymbol*/
.tower-move {
  transition: transform 0.5s cubic-bezier(0.22, 0.9, 0.3, 1);
}

/*noinspection CssUnusedSymbol*/
.tower-enter-active,
.tower-leave-active {
  transition: all 0.35s ease;
}

/*noinspection CssUnusedSymbol*/
.tower-enter-from,
.tower-leave-to {
  opacity: 0;
  transform: translateY(12px);
}

/*noinspection CssUnusedSymbol*/
.tower-leave-active {
  position: absolute;
}
</style>
