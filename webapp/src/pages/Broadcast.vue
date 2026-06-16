<script setup lang="ts">
// Full-screen broadcast overlay for one instance — a trackside TV graphics
// surface meant for a second screen. Two equal columns: the live track map
// fills the left half (car pucks glide over it), and the right half holds one
// card per driver, in running order, each pairing that driver's telemetry and
// car/livery preview with their video stream (stream to the right of the
// metrics on wide screens, below them on narrow). Cards are capped at half the
// stage height so at least two drivers are always on screen. All live data
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
function carImageUrl(model: string, skin: string): string {
  return `/api/car/image/${encodeURIComponent(model)}/${encodeURIComponent(skin || "")}`;
}

// --- Map geometry (mirrors ServerDetail's projection) ---
function mapWrapStyle(meta: TrackMapMeta) {
  const ratio = (meta.width || 16) / (meta.height || 9);
  // Fit the left column: cap width so the derived height never exceeds the
  // available stage height (viewport minus the top strap + padding).
  return { aspectRatio: String(ratio), width: `min(100%, calc((100vh - 6rem) * ${ratio}))`, margin: "auto" };
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
const streamChannels = computed<StreamChannel[]>(() => driverStreams.allChannelsFor(drivers.value));
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
  timingRows.value.map((row) => ({ row, channel: channelForCar(row.car_id) })),
);

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
          <span
            v-if="streamChannels.length"
            class="inline-flex h-9 items-center gap-1.5 rounded-md border border-line bg-surface/70 px-2.5 text-xs font-semibold text-muted"
            title="Driver streams online / total"
          >
            <Icon name="broadcast" :size="16" />
            <span class="font-mono">{{ onlineStreamCount }}/{{ streamChannels.length }}</span>
          </span>
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

    <!-- ░░ Two-column stage: map (1/2) + driver cards (1/2) ░░ -->
    <div class="absolute inset-x-0 top-16 bottom-0 z-10 flex gap-4 px-4 pb-4">
      <!-- Left column: live track map -->
      <section class="relative flex w-1/2 shrink-0 items-center justify-center overflow-hidden">
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
                class="relative grid size-7 place-items-center rounded-full border-2 text-xs leading-none font-semibold"
                :class="
                  timingFor(d.car_id)?.isLeader
                    ? 'border-bg bg-accent text-bg shadow-[0_0_22px_rgba(98,179,232,0.85)]'
                    : 'border-bg bg-surface-4 text-text shadow-[0_0_14px_rgba(0,0,0,0.7)]'
                "
              >
                {{ (d.name || "Car " + d.car_id).slice(0, 1).toUpperCase() }}
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
      </section>

      <!-- Right column: one card per driver (telemetry + stream + livery) -->
      <section class="min-w-0 flex-1 overflow-hidden">
        <TransitionGroup
          v-if="driverCards.length"
          tag="div"
          name="tower"
          class="flex h-full flex-col gap-3 overflow-y-auto pr-1"
        >
          <!-- Capped at half the stage height (minus half the gap) so two cards
               always fit; on wide cards the stream sits right of the metrics. -->
          <article
            v-for="card in driverCards"
            :key="card.row.car_id"
            class="tower-card flex h-[calc(50%-0.375rem)] shrink-0 cursor-pointer flex-col overflow-hidden rounded-lg border bg-surface/80 backdrop-blur-md transition-colors xl:flex-row"
            :class="
              focusRow?.car_id === card.row.car_id
                ? 'border-accent/70 ring-1 ring-accent/40'
                : card.row.isLeader
                  ? 'border-accent/40'
                  : 'border-line hover:border-line-hi'
            "
            @click="focusCar(card.row.car_id)"
          >
            <!-- Metrics side -->
            <div class="flex min-w-0 shrink-0 flex-col gap-2 p-2.5 xl:w-64">
              <!-- head: position · livery · driver/car -->
              <div class="flex items-center gap-2.5">
                <span
                  class="grid size-9 shrink-0 place-items-center rounded-md numerals text-xl font-semibold tabular-nums"
                  :class="card.row.isLeader ? 'bg-accent text-bg' : 'bg-surface-3 text-muted'"
                >
                  {{ card.row.position }}
                </span>
                <img
                  :src="carImageUrl(card.row.carModel, card.row.skin)"
                  alt=""
                  class="h-9 w-14 shrink-0 rounded border border-line bg-surface-3 object-cover"
                  @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"
                />
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-bold">{{ card.row.name }}</div>
                  <div class="truncate font-mono text-[11px] text-dim">{{ carName(card.row.carModel) }}</div>
                </div>
              </div>

              <!-- gap + lap -->
              <div class="flex items-baseline justify-between border-t border-line pt-2">
                <span
                  class="numerals text-lg tabular-nums"
                  :class="card.row.gapTone === 'leader' ? 'text-accent' : card.row.gapTone === 'warn' ? 'text-warn' : 'text-text'"
                >
                  {{ card.row.gapLabel }}
                </span>
                <span class="font-mono text-[10px] text-dim">LAP {{ card.row.laps }}</span>
              </div>

              <!-- speed/gear + laptimes, pinned to the bottom on tall cards -->
              <div class="mt-auto flex items-end gap-3 pt-1">
                <div class="leading-none">
                  <span class="numerals text-3xl font-medium tabular-nums">{{ speedKmh(card.row.pos) }}</span>
                  <span class="ml-1 font-mono text-[10px] text-dim">km/h</span>
                </div>
                <div class="text-center leading-none">
                  <div class="numerals text-2xl font-semibold text-accent tabular-nums">{{ gearLabel(card.row.pos) }}</div>
                  <div class="font-mono text-[9px] tracking-widest text-dim uppercase">gear</div>
                </div>
                <div class="ml-auto grid grid-cols-2 gap-x-3 text-right font-mono text-[11px]">
                  <span class="text-dim">LAST</span>
                  <span :class="card.row.last_lap_ms ? 'text-text' : 'text-dim'">{{ lapTime(card.row.last_lap_ms) }}</span>
                  <span class="text-dim">BEST</span>
                  <span :class="card.row.best_lap_ms ? 'text-ok' : 'text-dim'">{{ lapTime(card.row.best_lap_ms) }}</span>
                </div>
              </div>

              <!-- RPM under-bar -->
              <div class="h-1 overflow-hidden rounded-full bg-surface-3">
                <div
                  class="h-full rounded-full transition-[width] duration-200"
                  :class="rpmPct(card.row.pos) > 88 ? 'bg-danger' : rpmPct(card.row.pos) > 70 ? 'bg-warn' : 'bg-accent'"
                  :style="{ width: `${rpmPct(card.row.pos)}%` }"
                />
              </div>
            </div>

            <!-- Video broadcast: right of the metrics on wide screens, below on narrow -->
            <div class="relative min-h-0 flex-1 border-t border-line bg-bg xl:border-t-0 xl:border-l">
              <iframe
                v-if="card.channel?.online"
                :src="card.channel.url"
                :title="card.row.name"
                class="absolute inset-0 size-full border-0"
                allow="autoplay; fullscreen; picture-in-picture"
                sandbox="allow-scripts allow-same-origin allow-forms allow-presentation"
              />
              <div v-else class="grid size-full place-items-center">
                <div class="flex flex-col items-center gap-2 text-dim">
                  <div class="grid size-12 place-items-center rounded-full border border-line bg-surface/70">
                    <Icon name="user" :size="22" />
                  </div>
                  <span class="font-mono text-[11px] tracking-wide uppercase">
                    {{ card.channel ? "Stream offline" : "No stream" }}
                  </span>
                </div>
              </div>
              <button
                v-if="card.channel"
                type="button"
                class="absolute top-2 right-2 grid size-8 place-items-center rounded-md border border-line bg-bg/70 text-muted backdrop-blur transition-colors hover:border-accent/60 hover:text-accent"
                :aria-label="`Watch ${card.row.name} full screen`"
                :title="`Watch ${card.row.name} full screen`"
                @click.stop="openStream(card.row.car_id)"
              >
                <Icon name="maximize" :size="14" />
              </button>
            </div>
          </article>
        </TransitionGroup>

        <div v-else class="grid h-full place-items-center">
          <div class="text-center">
            <Icon name="broadcast" :size="40" class="mx-auto text-dim" />
            <p class="numerals mt-3 text-2xl tracking-tight text-muted">
              {{ running ? "Waiting for cars to join the session…" : "Server is stopped." }}
            </p>
          </div>
        </div>
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

/* Car pucks glide between SSE position frames (≈200ms publish cadence). */
.puck {
  transition:
    left 0.2s linear,
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
