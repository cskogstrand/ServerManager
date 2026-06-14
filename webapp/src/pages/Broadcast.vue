<script setup lang="ts">
// Full-screen broadcast overlay for one instance — a trackside TV graphics
// surface meant for a second screen. The live track map is the stage; car
// pucks glide over it, a reordering timing tower runs along the lower third,
// and a shift-light telemetry block reads out the focused car. All live data
// comes from the SSE-fed server store; this page only paints it.
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "@/lib/api";
import {
  useServerStore,
  type CarPositionState,
  type InstanceState,
  type StatusResponse,
} from "@/stores/server";
import { useContentStore } from "@/stores/content";
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
import { useDriverStreams, type StreamChannel } from "@/lib/useDriverStreams";
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

const instanceId = computed(() => Number(route.params.id));
const inst = computed<InstanceState | undefined>(() => server.instances[instanceId.value]);
const exitTo = computed(() => `/server/${instanceId.value}`);

const detail = ref<StatusPayload | null>(null);
const mapMeta = ref<TrackMapMeta | null>(null);
const mapImageOk = ref(true);

// --- Live state straight from the store (SSE-updated) ---
const drivers = computed(() => inst.value?.drivers ?? []);
const positions = computed(() => inst.value?.positions ?? []);
const session = computed(() => inst.value?.session ?? null);
const telemetry = computed(() => inst.value?.telemetry ?? null);
const running = computed(() => inst.value?.running ?? false);

const telemetryOnline = computed(() => !!telemetry.value?.udp_online);

// Track on stage: the live session wins; the status payload's current event is
// the fallback before the first session frame lands.
const activeTrack = computed(() => {
  const s = session.value;
  if (s?.track) return { key: s.track, config: s.track_config ?? "" };
  const ev = detail.value?.current_event;
  if (ev?.track_key) return { key: ev.track_key, config: ev.track_config ?? "" };
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

// Shift-light strip: 16 segments lit proportional to RPM, greens → ambers → reds.
const SHIFT_SEGMENTS = 16;
const shiftLights = computed(() => {
  const lit = Math.round((rpmPct(focusRow.value?.pos) / 100) * SHIFT_SEGMENTS);
  return Array.from({ length: SHIFT_SEGMENTS }, (_, i) => {
    const frac = i / SHIFT_SEGMENTS;
    const tone = frac < 0.6 ? "ok" : frac < 0.85 ? "warn" : "danger";
    return { on: i < lit, tone };
  });
});

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

// --- Map geometry (mirrors ServerDetail's projection) ---
function mapWrapStyle(meta: TrackMapMeta) {
  const ratio = (meta.width || 16) / (meta.height || 9);
  return { aspectRatio: String(ratio), width: `min(100%, calc(72vh * ${ratio}))`, margin: "auto" };
}
function mapPoint(pos: CarPositionState, meta: TrackMapMeta) {
  const scale = meta.scale_factor || 1;
  const x = ((pos.x + meta.x_offset) * scale) / meta.width;
  const y = (meta.height - (pos.z + meta.z_offset) * scale) / meta.height;
  return {
    left: `${Math.max(0, Math.min(100, x * 100))}%`,
    top: `${Math.max(0, Math.min(100, y * 100))}%`,
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
const streamChannels = computed<StreamChannel[]>(() => driverStreams.channelsFor(drivers.value));

function guidForCar(carId: number): string | undefined {
  return drivers.value.find((d) => d.car_id === carId)?.guid;
}
function hasStreamForCar(carId: number): boolean {
  return !!driverStreams.streamForGuid(guidForCar(carId));
}
function openStream(carId: number) {
  theaterKey.value = `driver:${guidForCar(carId)}`;
  theaterOpen.value = true;
}

async function fetchStatus() {
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

// Fullscreen helps on a dedicated broadcast screen; best-effort only.
function toggleFullscreen() {
  if (document.fullscreenElement) void document.exitFullscreen();
  else void document.documentElement.requestFullscreen().catch(() => {});
}

onMounted(async () => {
  if (!server.loaded) await server.load();
  void content.load();
  void driverStreams.loadStreams();
  driverStreams.startHealthPoll(() => instanceId.value);
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
});
</script>

<template>
  <div class="bcast fixed inset-0 z-50 overflow-hidden bg-bg text-text select-none">
    <!-- Atmosphere layers -->
    <div class="bcast-bg pointer-events-none absolute inset-0" />
    <div class="bcast-scan pointer-events-none absolute inset-0" />
    <div class="bcast-vignette pointer-events-none absolute inset-0" />

    <!-- ░░ Top strap ░░ -->
    <header class="absolute inset-x-0 top-0 z-30 flex h-16 items-center gap-4 px-5">
      <div class="flex items-center gap-2.5">
        <span
          class="inline-flex items-center gap-1.5 rounded-sm px-2 py-1 text-xs font-black tracking-[0.2em]"
          :class="running ? 'bg-danger text-bg' : 'bg-surface-3 text-dim'"
        >
          <span v-if="running" class="live-dot size-1.5 rounded-full bg-bg" />
          {{ running ? "LIVE" : "OFFLINE" }}
        </span>
        <div class="leading-tight">
          <div class="text-sm font-extrabold tracking-tight">{{ inst?.name ?? "Server" }}</div>
          <div class="font-mono text-[11px] text-dim">:{{ inst?.tcp_port }} · {{ drivers.length }} cars</div>
        </div>
      </div>

      <!-- Session clock -->
      <div v-if="clock" class="mx-auto flex items-center gap-5">
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
      <div v-else class="mx-auto numerals text-3xl text-dim tracking-tight">— STANDBY —</div>

      <!-- Track + conditions -->
      <div class="flex items-center gap-4">
        <div class="text-right leading-tight">
          <div class="max-w-[18ch] truncate text-sm font-bold">{{ detail?.current_event?.track || activeTrack?.key || "—" }}</div>
          <div class="font-mono text-[11px] text-dim">
            <span v-if="clock">{{ clock.ambient }}° air · {{ clock.road }}° track</span>
            <span v-else>{{ activeTrack?.config || "default" }}</span>
          </div>
        </div>
        <img
          v-if="detail?.current_event?.weather_key"
          :src="weatherImageUrl(detail.current_event.weather_key)"
          alt=""
          class="size-9 rounded-full border border-line object-cover"
          @error="($event.target as HTMLImageElement).style.display = 'none'"
        />
        <div class="flex items-center gap-1.5">
          <button
            type="button"
            class="grid size-9 place-items-center rounded-md border border-line bg-surface/70 text-muted transition-colors hover:border-line-hi hover:text-text"
            title="Toggle fullscreen"
            @click="toggleFullscreen"
          >
            <Icon name="maximize" :size="16" />
          </button>
          <RouterLink
            :to="exitTo"
            class="grid size-9 place-items-center rounded-md border border-line bg-surface/70 text-muted transition-colors hover:border-danger/60 hover:text-danger"
            title="Exit broadcast"
          >
            <Icon name="x" :size="16" />
          </RouterLink>
        </div>
      </div>
    </header>

    <!-- ░░ Map stage ░░ -->
    <main class="absolute inset-x-0 top-16 bottom-44 z-10 grid place-items-center px-6">
      <div
        v-if="activeTrack && mapMeta && mapImageOk"
        class="bcast-map relative"
        :style="mapWrapStyle(mapMeta)"
      >
        <img
          :src="trackUrl('map', activeTrack.key, activeTrack.config)"
          alt=""
          class="absolute inset-0 size-full object-fill opacity-70"
          @error="mapImageOk = false"
        />
        <template v-for="d in drivers" :key="d.car_id">
          <button
            v-if="positionFor(d.car_id)"
            type="button"
            class="puck absolute -translate-x-1/2 -translate-y-1/2"
            :class="{ 'puck-focus': focusRow?.car_id === d.car_id }"
            :style="mapPoint(positionFor(d.car_id)!, mapMeta)"
            @click="focusCar(d.car_id)"
          >
            <span
              v-if="timingFor(d.car_id)?.isLeader"
              class="puck-ring absolute inset-0 -m-1 rounded-full"
            />
            <span
              class="relative grid size-8 place-items-center rounded-full border-2 numerals text-sm leading-none font-semibold tabular-nums"
              :class="
                timingFor(d.car_id)?.isLeader
                  ? 'border-bg bg-accent text-bg shadow-[0_0_22px_rgba(98,179,232,0.85)]'
                  : 'border-bg bg-surface-4 text-text shadow-[0_0_14px_rgba(0,0,0,0.7)]'
              "
            >
              {{ timingFor(d.car_id)?.position ?? "?" }}
            </span>
            <span
              class="pointer-events-none absolute top-9 left-1/2 -translate-x-1/2 rounded-sm bg-bg/80 px-1.5 py-0.5 text-[10px] font-semibold whitespace-nowrap backdrop-blur-sm"
            >
              {{ d.name || "Car " + d.car_id }}
            </span>
          </button>
        </template>
      </div>

      <!-- Map unavailable / offline states -->
      <div v-else class="text-center">
        <Icon name="broadcast" :size="40" class="mx-auto text-dim" />
        <p class="numerals mt-3 text-2xl tracking-tight text-muted">
          {{ !running ? "SERVER OFFLINE" : !activeTrack ? "NO TRACK LOADED" : "NO LIVE MAP FOR THIS LAYOUT" }}
        </p>
        <RouterLink
          v-if="!running"
          :to="exitTo"
          class="mt-4 inline-flex items-center gap-1.5 rounded-md border border-line bg-surface/70 px-3 py-1.5 text-sm text-muted hover:text-text"
        >
          <Icon name="arrowUp" :size="14" class="-rotate-90" />
          Back to control
        </RouterLink>
      </div>

      <!-- Telemetry-offline notice -->
      <div
        v-if="running && !telemetryOnline"
        class="absolute top-3 left-1/2 -translate-x-1/2 rounded-md border border-warn/40 bg-warn-glow px-3 py-1.5 text-xs font-semibold text-warn backdrop-blur-sm"
      >
        Waiting for the AC telemetry plugin — map and timing stay empty until it connects.
      </div>
    </main>

    <!-- ░░ Focus telemetry (shift lights + speed) ░░ -->
    <section
      v-if="focusRow"
      class="absolute top-20 left-5 z-20 w-64 rounded-lg border border-line bg-surface/80 p-3.5 backdrop-blur-md"
    >
      <div class="flex items-center gap-2">
        <span
          class="grid size-7 place-items-center rounded-md numerals text-base font-semibold tabular-nums"
          :class="focusRow.isLeader ? 'bg-accent text-bg' : 'bg-surface-3 text-muted'"
        >
          {{ focusRow.position }}
        </span>
        <div class="min-w-0">
          <div class="truncate text-sm font-bold">{{ focusRow.name }}</div>
          <div class="truncate font-mono text-[11px] text-dim">{{ carName(focusRow.carModel) }}</div>
        </div>
      </div>

      <!-- Shift lights -->
      <div class="mt-3 flex gap-[3px]">
        <span
          v-for="(seg, i) in shiftLights"
          :key="i"
          class="h-2 flex-1 rounded-[1px] transition-colors duration-100"
          :class="
            seg.on
              ? seg.tone === 'danger'
                ? 'bg-danger shadow-[0_0_8px_rgba(239,113,104,0.8)]'
                : seg.tone === 'warn'
                  ? 'bg-warn shadow-[0_0_8px_rgba(240,185,90,0.7)]'
                  : 'bg-ok shadow-[0_0_8px_rgba(79,216,132,0.7)]'
              : 'bg-surface-3'
          "
        />
      </div>

      <!-- Speed + gear -->
      <div class="mt-3 flex items-end justify-between">
        <div class="leading-none">
          <span class="numerals text-6xl font-medium tabular-nums">{{ speedKmh(focusRow.pos) }}</span>
          <span class="ml-1 font-mono text-xs text-dim">km/h</span>
        </div>
        <div class="text-center leading-none">
          <div class="numerals text-5xl font-semibold text-accent tabular-nums">{{ gearLabel(focusRow.pos) }}</div>
          <div class="font-mono text-[10px] tracking-widest text-dim uppercase">gear</div>
        </div>
      </div>

      <div class="mt-3 grid grid-cols-2 gap-2 border-t border-line pt-2.5 font-mono text-xs">
        <div>
          <div class="text-[10px] tracking-wide text-dim">LAST</div>
          <div :class="focusRow.last_lap_ms ? 'text-text' : 'text-dim'">{{ lapTime(focusRow.last_lap_ms) }}</div>
        </div>
        <div>
          <div class="text-[10px] tracking-wide text-dim">BEST</div>
          <div :class="focusRow.best_lap_ms ? 'text-ok' : 'text-dim'">{{ lapTime(focusRow.best_lap_ms) }}</div>
        </div>
      </div>
    </section>

    <!-- ░░ Timing tower (lower third) ░░ -->
    <footer class="absolute inset-x-0 bottom-0 z-20 px-5 pb-5">
      <div class="mb-2 flex items-center gap-2 px-1">
        <span class="numerals text-sm tracking-[0.2em] text-dim uppercase">Running order</span>
        <span class="h-px flex-1 bg-line" />
        <span class="font-mono text-[11px] text-dim">tap a car to follow it</span>
      </div>

      <TransitionGroup
        v-if="timingRows.length"
        tag="ol"
        name="tower"
        class="flex flex-wrap gap-2.5"
      >
        <li
          v-for="row in timingRows"
          :key="row.car_id"
          class="tower-card min-w-60 flex-1 cursor-pointer rounded-md border bg-surface/85 p-2.5 backdrop-blur-md transition-colors"
          :class="
            focusRow?.car_id === row.car_id
              ? 'border-accent/70 ring-1 ring-accent/40'
              : row.isLeader
                ? 'border-accent/40'
                : 'border-line hover:border-line-hi'
          "
          @click="focusCar(row.car_id)"
        >
          <div class="flex items-center gap-2.5">
            <span
              class="grid size-9 shrink-0 place-items-center rounded-md numerals text-xl font-semibold tabular-nums"
              :class="row.isLeader ? 'bg-accent text-bg' : 'bg-surface-3 text-muted'"
            >
              {{ row.position }}
            </span>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-bold">{{ row.name }}</div>
              <div class="truncate font-mono text-[11px] text-dim">{{ carName(row.carModel) }}</div>
            </div>
            <div class="shrink-0 text-right">
              <div
                class="numerals text-lg tabular-nums"
                :class="row.gapTone === 'leader' ? 'text-accent' : row.gapTone === 'warn' ? 'text-warn' : 'text-text'"
              >
                {{ row.gapLabel }}
              </div>
              <div class="font-mono text-[10px] text-dim">LAP {{ row.laps }}</div>
            </div>
            <button
              v-if="hasStreamForCar(row.car_id)"
              type="button"
              class="grid size-8 shrink-0 place-items-center rounded-md border border-accent/40 bg-accent-dim text-accent transition-colors hover:bg-accent/20"
              :aria-label="`Watch ${row.name}'s stream`"
              :title="`Watch ${row.name}'s stream`"
              @click.stop="openStream(row.car_id)"
            >
              <Icon name="play" :size="14" />
            </button>
          </div>

          <div class="mt-2 flex items-center gap-3 font-mono text-[11px]">
            <span class="text-dim">L <span class="text-text">{{ lapTime(row.last_lap_ms) }}</span></span>
            <span class="text-dim">B <span class="text-ok">{{ lapTime(row.best_lap_ms) }}</span></span>
            <span class="ml-auto numerals text-sm tabular-nums">{{ speedKmh(row.pos) }}<span class="text-[10px] text-dim"> km/h</span></span>
          </div>

          <!-- RPM under-bar -->
          <div class="mt-1.5 h-1 overflow-hidden rounded-full bg-surface-3">
            <div
              class="h-full rounded-full transition-[width] duration-200"
              :class="rpmPct(row.pos) > 88 ? 'bg-danger' : rpmPct(row.pos) > 70 ? 'bg-warn' : 'bg-accent'"
              :style="{ width: `${rpmPct(row.pos)}%` }"
            />
          </div>
        </li>
      </TransitionGroup>

      <div
        v-else
        class="rounded-md border border-line bg-surface/70 px-4 py-3 text-center font-mono text-xs text-dim backdrop-blur-md"
      >
        {{ running ? "Waiting for cars to join the session…" : "Server is stopped." }}
      </div>
    </footer>

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
  background-image:
    radial-gradient(ellipse 80% 60% at 50% 18%, rgba(98, 179, 232, 0.1), transparent 70%),
    linear-gradient(var(--color-line) 1px, transparent 1px),
    linear-gradient(90deg, var(--color-line) 1px, transparent 1px);
  background-size:
    100% 100%,
    44px 44px,
    44px 44px;
  background-position:
    0 0,
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

.bcast-map {
  border: 1px solid var(--color-line-hi);
  border-radius: var(--radius-md);
  box-shadow:
    0 30px 80px rgba(0, 0, 0, 0.55),
    inset 0 0 60px rgba(98, 179, 232, 0.05);
}

/* Car pucks glide between SSE position frames. */
.puck {
  transition:
    left 0.3s linear,
    top 0.3s linear;
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
