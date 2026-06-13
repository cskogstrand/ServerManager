<script setup lang="ts">
// Full-page race control view for one instance: track map front and center
// (rendered whenever the track ships map data, with or without live cars),
// circuit facts, session telemetry, live timing, grid with car imagery and
// the upcoming queue. Opened from the Dashboard card header.
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api, ApiError } from "@/lib/api";
import { useServerStore, type CarPositionState, type DriverState, type InstanceState } from "@/stores/server";
import { useContentStore } from "@/stores/content";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import type { CacheCar, CacheTrack } from "@/types/generated";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import Sheet from "@/components/ui/Sheet.vue";
import Skeleton from "@/components/ui/Skeleton.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import LineChart from "@/components/ui/LineChart.vue";

interface CurrentEvent {
  id: number;
  category: string;
  track: string;
  track_key: string;
  track_config: string;
  difficulty: string;
  session: string;
  class: string;
  class_id: number;
  time: string;
  time_id: number;
  weather: string;
  weather_key: string;
  started_at: number;
  finished: number;
}

interface StatusPayload {
  instance_id: number;
  is_running: boolean;
  text: string;
  players: number;
  public_ip: string;
  current_event: CurrentEvent;
  drivers: DriverState[];
  current_cars: { cache_car_key: string; skin_key: string }[];
  session: {
    type: string;
    name: string;
    current_session_index: number;
    session_count: number;
    time: number;
    laps: number;
    ambient_temp: number;
    road_temp: number;
    elapsed_ms: number;
  };
  positions: CarPositionState[];
}

interface TrackMapMeta {
  width: number;
  height: number;
  x_offset: number;
  z_offset: number;
  scale_factor: number;
  margin: number;
}

interface QueueItem {
  id: number;
  event_id: number;
  instance_id: number;
  category: string;
  track: string;
  track_key: string | null;
  track_config: string | null;
  difficulty: string;
  session: string;
  class: string;
  time: string;
  started_at: number | null;
  finished: number;
}

const route = useRoute();
const router = useRouter();
const server = useServerStore();
const content = useContentStore();
const toast = useToastStore();
const confirm = useConfirmStore();

const instanceId = computed(() => Number(route.params.id));
const inst = computed<InstanceState | undefined>(() => server.instances[instanceId.value]);

const detail = ref<StatusPayload | null>(null);
const queue = ref<QueueItem[]>([]);
const mapMeta = ref<TrackMapMeta | null>(null);
const mapImageOk = ref(true);
const loading = ref(true);
const busy = ref(false);
const consoleOpen = ref(false);
const streamViewer = ref<{ title: string; url: string } | null>(null);

// Track shown on the page: the loaded event wins; with nothing loaded we fall
// back to the next unfinished queue entry so the map never disappears.
const activeTrack = computed<{ key: string; config: string; name: string; source: "current" | "queued" } | null>(() => {
  const ev = detail.value?.current_event;
  if (ev?.track_key) {
    return { key: ev.track_key, config: ev.track_config ?? "", name: ev.track, source: "current" };
  }
  const next = queue.value.find((q) => !q.finished && q.track_key);
  if (next?.track_key) {
    return { key: next.track_key, config: next.track_config ?? "", name: next.track, source: "queued" };
  }
  return null;
});

const trackInfo = computed<CacheTrack | undefined>(() => {
  const t = activeTrack.value;
  if (!t) return undefined;
  return content.tracks.find((c) => c.key === t.key && (c.config ?? "") === t.config);
});

function trackUrl(kind: "map" | "mapmeta" | "outline" | "preview", key: string, config: string): string {
  const cfg = config ? `/${encodeURIComponent(config)}` : "";
  return `/api/track/${kind}/${encodeURIComponent(key)}${cfg}`;
}

const upcoming = computed(() => queue.value.filter((q) => !q.finished).slice(0, 6));

const positions = computed(() => inst.value?.positions ?? []);
const drivers = computed(() => inst.value?.drivers ?? []);
const connected = computed(() => drivers.value.filter((d) => d.connected));

async function fetchDetail() {
  try {
    const payload = await api.get<StatusPayload>(`/api/server/status?instance=${instanceId.value}`);
    detail.value = payload;
    const live = server.instances[instanceId.value];
    if (live) {
      live.drivers = payload.drivers ?? [];
      live.positions = payload.positions ?? [];
    }
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

async function fetchQueue() {
  try {
    const res = await api.get<{ items: QueueItem[] }>(`/api/queue?instance=${instanceId.value}`);
    queue.value = res.items ?? [];
  } catch {
    queue.value = [];
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

// SSE running flips → refetch the detail payload (event may have rotated)
watch(
  () => inst.value?.running,
  () => {
    void fetchDetail();
    void fetchQueue();
  },
);

async function toggleServer() {
  const running = inst.value?.running ?? false;
  if (running) {
    const ok = await confirm.ask({
      title: "Stop server",
      message: `Stop ${inst.value?.name ?? "this instance"}?`,
      detail: "Connected players are disconnected.",
      confirmLabel: "Stop server",
      tone: "danger",
    });
    if (!ok) return;
  }
  busy.value = true;
  try {
    await (running ? server.stop(instanceId.value) : server.start(instanceId.value));
    await Promise.all([fetchDetail(), fetchQueue()]);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

function kick(carId: number, name: string) {
  void confirm
    .ask({
      title: "Kick driver",
      message: `Kick ${name || "car " + carId} from the server?`,
      confirmLabel: "Kick",
      tone: "danger",
    })
    .then(async (ok) => {
      if (!ok) return;
      busy.value = true;
      try {
        await api.post(`/api/server/kick?instance=${instanceId.value}`, { car_id: carId });
        toast.success("Driver kicked.");
      } catch (e) {
        toast.error(e instanceof ApiError ? e.message : String(e));
      } finally {
        busy.value = false;
      }
    });
}

// --- Map geometry ---
// Cap the map at 80vh: width follows from the height cap so tall, narrow
// circuits don't fill the whole page; wide ones still span the card.
function mapCanvasStyle(meta: TrackMapMeta) {
  const ratio = (meta.width || 16) / (meta.height || 9);
  return {
    aspectRatio: String(ratio),
    width: `min(100%, calc(80vh * ${ratio}))`,
    marginInline: "auto",
  };
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

function speedKmh(pos?: CarPositionState): number {
  if (!pos) return 0;
  const ms = Math.sqrt(pos.velocity_x ** 2 + pos.velocity_y ** 2 + pos.velocity_z ** 2);
  return Math.round(ms * 3.6);
}

function lapTime(ms: number): string {
  if (!ms) return "—";
  const m = Math.floor(ms / 60000);
  const s = Math.floor((ms % 60000) / 1000);
  const t = ms % 1000;
  return `${m}:${String(s).padStart(2, "0")}.${String(t).padStart(3, "0")}`;
}

function elapsed(): string {
  const ms = detail.value?.session?.elapsed_ms ?? 0;
  if (ms <= 0) return "—";
  const min = Math.floor(ms / 60000);
  const sec = Math.floor((ms % 60000) / 1000);
  return `${min}:${String(sec).padStart(2, "0")}`;
}

function trackLengthLabel(t?: CacheTrack): string {
  if (!t?.length) return "—";
  return t.length >= 1000 ? `${(t.length / 1000).toFixed(2)} km` : `${t.length} m`;
}

// --- Grid: collapse the rendered entry list into per car+skin rows enriched
// from the content cache (brand, specs, display names).
interface GridRow {
  carKey: string;
  skinKey: string;
  count: number;
  car?: CacheCar;
  skinName: string;
}

const gridRows = computed<GridRow[]>(() => {
  const rows = new Map<string, GridRow>();
  for (const entry of detail.value?.current_cars ?? []) {
    const id = `${entry.cache_car_key}::${entry.skin_key}`;
    const existing = rows.get(id);
    if (existing) {
      existing.count += 1;
      continue;
    }
    const car = content.carByKey(entry.cache_car_key);
    rows.set(id, {
      carKey: entry.cache_car_key,
      skinKey: entry.skin_key,
      count: 1,
      car,
      skinName: car?.skins?.find((s) => s.key === entry.skin_key)?.name || entry.skin_key,
    });
  }
  return [...rows.values()];
});

function carImageUrl(row: GridRow): string {
  return `/api/car/image/${encodeURIComponent(row.carKey)}/${encodeURIComponent(row.skinKey)}`;
}

// --- Car detail viewer: photo, specs and power/torque curves ---
interface CarCurves {
  key: string;
  desc: string;
  power: number[];
  torque: number[];
  labels: number[];
}

const carViewer = ref<GridRow | null>(null);
const carCurves = ref<CarCurves | null>(null);
const carCurvesLoading = ref(false);
const curveCache = new Map<string, CarCurves>();

async function openCar(row: GridRow) {
  carViewer.value = row;
  carCurves.value = curveCache.get(row.carKey) ?? null;
  if (carCurves.value) return;
  carCurvesLoading.value = true;
  try {
    const res = await api.get<CarCurves>(`/api/car/${encodeURIComponent(row.carKey)}`);
    curveCache.set(row.carKey, res);
    // Guard against a stale response if the user reopened a different car.
    if (carViewer.value?.carKey === row.carKey) carCurves.value = res;
  } catch {
    if (carViewer.value?.carKey === row.carKey) carCurves.value = null;
  } finally {
    carCurvesLoading.value = false;
  }
}

const carChartSeries = computed(() => {
  const c = carCurves.value;
  if (!c) return [];
  const out: { name: string; color: string; values: number[]; unit?: string }[] = [];
  if (c.power?.some((v) => v > 0)) out.push({ name: "Power", color: "#62b3e8", values: c.power, unit: " bhp" });
  if (c.torque?.some((v) => v > 0)) out.push({ name: "Torque", color: "#f0b95a", values: c.torque, unit: " Nm" });
  return out;
});

const carSpecRows = computed(() => {
  const s = carViewer.value?.car?.specs;
  if (!s) return [];
  return [
    { label: "Power", value: s.bhp },
    { label: "Torque", value: s.torque },
    { label: "Weight", value: s.weight },
    { label: "Top speed", value: s.topspeed },
    { label: "0–100", value: s.acceleration },
    { label: "P/W ratio", value: s.pwratio },
  ].filter((r) => r.value);
});

// Strip any HTML tags/entities mods stuff into the description; render plain.
const carDescText = computed(() => {
  const raw = carCurves.value?.desc;
  if (!raw) return "";
  return raw
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<[^>]+>/g, "")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .trim();
});

// --- Weather thumbnail ---
function weatherImageUrl(key: string): string {
  return `/api/weather/preview/${encodeURIComponent(key)}`;
}

function consoleLines(): string {
  const text = detail.value?.text ?? "";
  return text.split("\n").slice(-200).join("\n").trim() || "No output yet.";
}

const sessionTiles = computed(() => {
  const s = detail.value?.session;
  const running = inst.value?.running;
  return [
    { label: "Session", value: running && s ? `${s.type || "—"}` : "—", sub: running && s ? `${(s.current_session_index ?? 0) + 1} of ${s.session_count}` : "stopped" },
    { label: "Elapsed", value: running ? elapsed() : "—", sub: running && s ? (s.laps ? `${s.laps} laps` : `${s.time} min`) : "" },
    { label: "Air / Road", value: running && s ? `${s.ambient_temp}° / ${s.road_temp}°` : "—", sub: "temperature" },
    { label: "Drivers", value: running ? String(connected.value.length) : "0", sub: `${positions.value.length} live on map` },
  ];
});

// --- Session timeline ---
// pips: one per session in the weekend (current highlighted). For timed
// sessions we also derive a fill fraction from elapsed/total; lap sessions
// have no clean denominator so the bar stays indeterminate.
const sessionTimeline = computed(() => {
  const s = detail.value?.session;
  if (!inst.value?.running || !s || !s.session_count) return null;
  const idx = s.current_session_index ?? 0;
  const timed = !s.laps && s.time > 0;
  const totalMs = timed ? s.time * 60000 : 0;
  const fraction = timed ? Math.max(0, Math.min(1, s.elapsed_ms / totalMs)) : null;
  return {
    type: s.type,
    index: idx,
    count: s.session_count,
    timed,
    fraction,
    elapsedLabel: elapsed(),
    totalLabel: timed ? `${s.time} min` : s.laps ? `${s.laps} laps` : "—",
    remaining: timed ? Math.max(0, Math.ceil((totalMs - s.elapsed_ms) / 60000)) : null,
  };
});

// --- Live telemetry helpers ---
// AC gear convention: 0 = reverse, 1 = neutral, 2 = first gear, …
function gearLabel(pos?: CarPositionState): string {
  if (!pos) return "—";
  const g = pos.gear;
  if (g <= 0) return "R";
  if (g === 1) return "N";
  return String(g - 1);
}

// Bar scales against the fastest-revving car on track (floor 8000) so the
// fill stays meaningful without knowing each car's redline.
const rpmMax = computed(() => {
  const peak = Math.max(8000, ...positions.value.map((p) => p.engine_rpm || 0));
  return Math.ceil(peak / 1000) * 1000;
});

function rpmPct(pos?: CarPositionState): number {
  if (!pos?.engine_rpm) return 0;
  return Math.max(0, Math.min(100, (pos.engine_rpm / rpmMax.value) * 100));
}

onMounted(async () => {
  await server.load();
  if (!server.instances[instanceId.value]) {
    toast.error("Unknown server instance.");
    void router.replace("/");
    return;
  }
  void content.load();
  await Promise.all([fetchDetail(), fetchQueue()]);
  await fetchMapMeta();
  loading.value = false;
});

// Refresh console output while the panel is open
let consoleTimer: ReturnType<typeof setInterval> | null = null;
watch(consoleOpen, (open) => {
  if (open && !consoleTimer) {
    consoleTimer = setInterval(() => void fetchDetail(), 5000);
  } else if (!open && consoleTimer) {
    clearInterval(consoleTimer);
    consoleTimer = null;
  }
});
onBeforeUnmount(() => {
  if (consoleTimer) clearInterval(consoleTimer);
});
</script>

<template>
  <div v-if="loading" class="space-y-4">
    <Skeleton class="h-12" />
    <Skeleton class="h-96" />
    <Skeleton class="h-48" />
  </div>

  <template v-else-if="inst">
    <!-- Masthead -->
    <header class="page-enter mb-5 flex flex-wrap items-center gap-3">
      <RouterLink
        to="/"
        class="inline-flex min-h-9 items-center gap-1.5 rounded-md border border-line bg-surface px-2.5 text-xs font-semibold text-muted transition-colors hover:border-line-hi hover:text-text"
      >
        <Icon name="arrowUp" :size="14" class="-rotate-90" />
        Dashboard
      </RouterLink>
      <span
        class="size-2.5 rounded-full"
        :class="inst.running ? 'bg-ok shadow-[0_0_16px_rgba(79,216,132,0.6)]' : 'bg-dim'"
      />
      <h1 class="text-xl font-black tracking-tight">{{ inst.name }}</h1>
      <span class="font-mono text-xs text-dim">:{{ inst.tcp_port }}</span>
      <span v-if="inst.running" class="rounded-full bg-surface-2 px-2 py-0.5 text-xs text-muted">
        {{ inst.players }} player{{ inst.players === 1 ? "" : "s" }}
      </span>
      <span
        v-if="inst.run_mode === 'repeat_event'"
        class="inline-flex items-center gap-1 rounded-full border border-accent/40 bg-accent-dim px-2 py-0.5 text-xs text-accent"
      >
        <Icon name="repeat" :size="12" />
        Repeat
      </span>
      <div class="ml-auto flex flex-wrap items-center gap-2">
        <span v-if="detail?.public_ip" class="inline-flex min-h-8 items-center gap-2 rounded-md border border-line bg-surface px-2.5 font-mono text-xs text-dim">
          <Icon name="activity" :size="14" />
          {{ detail.public_ip }}
        </span>
        <Button :variant="inst.running ? 'danger' : 'success'" size="sm" :disabled="busy" @click="toggleServer">
          <Icon :name="inst.running ? 'stop' : 'power'" :size="15" />
          {{ busy ? "Working" : inst.running ? "Stop" : "Start" }}
        </Button>
      </div>
    </header>

    <!-- Track hero: photo backdrop + name, location, description -->
    <section
      v-if="activeTrack"
      class="track-hero page-enter relative mb-4 overflow-hidden rounded-lg border border-line"
      style="animation-delay: 30ms"
    >
      <img
        :src="trackUrl('preview', activeTrack.key, activeTrack.config)"
        alt=""
        class="absolute inset-0 size-full object-cover"
        @error="($event.target as HTMLImageElement).style.display = 'none'"
      />
      <div class="relative flex min-h-44 flex-col justify-end gap-1 p-4 sm:p-5">
        <div class="flex flex-wrap items-center gap-2">
          <span
            v-if="activeTrack.source === 'queued'"
            class="rounded-full border border-accent/40 bg-accent-dim px-2 py-0.5 text-xs font-semibold text-accent"
          >
            Up next
          </span>
          <span v-if="trackInfo?.country || trackInfo?.city" class="text-xs font-medium tracking-wide text-muted uppercase">
            {{ [trackInfo?.city, trackInfo?.country].filter(Boolean).join(" · ") }}
          </span>
        </div>
        <h2 class="max-w-3xl text-2xl font-black tracking-tight text-text drop-shadow-[0_2px_8px_rgba(0,0,0,0.7)]">
          {{ trackInfo?.name || activeTrack.name }}
        </h2>
        <p
          v-if="trackInfo?.desc"
          class="line-clamp-3 max-w-3xl text-sm text-muted drop-shadow-[0_1px_6px_rgba(0,0,0,0.8)]"
        >
          {{ trackInfo.desc }}
        </p>
        <dl class="mt-1.5 flex flex-wrap gap-x-5 gap-y-1 font-mono text-xs text-muted">
          <div class="flex items-center gap-1.5">
            <span class="text-dim">Length</span>
            <span class="text-text">{{ trackLengthLabel(trackInfo) }}</span>
          </div>
          <div v-if="trackInfo?.width" class="flex items-center gap-1.5">
            <span class="text-dim">Width</span>
            <span class="text-text">{{ trackInfo.width }}</span>
          </div>
          <div class="flex items-center gap-1.5">
            <span class="text-dim">Pitboxes</span>
            <span class="text-text">{{ trackInfo?.pitboxes ?? "—" }}</span>
          </div>
          <div class="flex items-center gap-1.5">
            <span class="text-dim">Layout</span>
            <span class="text-text">{{ activeTrack.config || "default" }}</span>
          </div>
        </dl>
      </div>
    </section>

    <!-- Hero: map + circuit panel -->
    <div class="page-enter grid gap-4 lg:grid-cols-3" style="animation-delay: 60ms">
      <Card class="lg:col-span-2" :muted="false">
        <template #header>
          <h2 class="text-sm font-bold">Track map</h2>
          <span class="ml-auto font-mono text-xs" :class="positions.length ? 'text-ok' : 'text-dim'">
            {{ positions.length ? `${positions.length} live position${positions.length === 1 ? "" : "s"}` : "no live telemetry" }}
          </span>
        </template>

        <template v-if="activeTrack">
          <div
            v-if="mapMeta && mapImageOk"
            class="map-canvas relative overflow-hidden rounded-md border border-line"
            :style="mapCanvasStyle(mapMeta)"
          >
            <img
              :src="trackUrl('map', activeTrack.key, activeTrack.config)"
              alt=""
              class="absolute inset-0 size-full object-fill opacity-85"
              @error="mapImageOk = false"
            />
            <template v-for="d in drivers" :key="d.car_id">
              <button
                v-if="positionFor(d.car_id)"
                type="button"
                class="absolute grid size-7 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full border border-bg bg-accent text-[10px] font-black text-bg shadow-[0_0_18px_rgba(98,179,232,0.65)] transition-transform hover:z-10 hover:scale-110"
                :style="mapPoint(positionFor(d.car_id)!, mapMeta)"
                :title="`${d.name || 'car ' + d.car_id} · ${speedKmh(positionFor(d.car_id))} km/h · gear ${positionFor(d.car_id)?.gear ?? 0}`"
              >
                {{ (d.name || String(d.car_id)).slice(0, 1).toUpperCase() }}
              </button>
            </template>
            <span
              v-if="!positions.length"
              class="absolute bottom-2 left-2 rounded-md border border-line bg-bg/80 px-2 py-1 text-xs text-dim backdrop-blur-sm"
            >
              Cars appear here once drivers are on track
            </span>
            <span
              v-if="activeTrack.source === 'queued'"
              class="absolute top-2 left-2 rounded-md border border-accent/40 bg-accent-dim px-2 py-1 text-xs text-accent backdrop-blur-sm"
            >
              Up next — not on track yet
            </span>
          </div>

          <!-- No map.ini / map.png: fall back to the outline drawing -->
          <div v-else class="map-canvas relative grid place-items-center overflow-hidden rounded-md border border-line py-8">
            <img
              :src="trackUrl('outline', activeTrack.key, activeTrack.config)"
              alt=""
              class="max-h-72 opacity-80"
              @error="($event.target as HTMLImageElement).style.display = 'none'"
            />
            <span class="absolute bottom-2 left-2 rounded-md border border-line bg-bg/80 px-2 py-1 text-xs text-dim backdrop-blur-sm">
              This layout ships no live-map metadata — outline only
            </span>
          </div>
        </template>
        <EmptyState
          v-else
          icon="events"
          title="No track to show"
          message="Queue an event for this instance and its track map appears here."
        >
          <RouterLink to="/queue">
            <Button size="sm">
              <Icon name="queue" :size="14" />
              Open queue
            </Button>
          </RouterLink>
        </EmptyState>
      </Card>

      <!-- Layout outline + tags -->
      <Card>
        <template #header>
          <h2 class="text-sm font-bold">Layout</h2>
          <span v-if="activeTrack" class="ml-auto font-mono text-xs text-dim">{{ activeTrack.key }}</span>
        </template>
        <template v-if="activeTrack">
          <div class="map-canvas grid place-items-center rounded-md border border-line p-4">
            <img
              :src="trackUrl('outline', activeTrack.key, activeTrack.config)"
              alt=""
              class="max-h-52 w-auto opacity-90"
              @error="($event.target as HTMLImageElement).style.display = 'none'"
            />
          </div>
          <div v-if="trackInfo?.tags?.length" class="mt-3 flex flex-wrap gap-1.5">
            <span v-for="tag in trackInfo.tags.slice(0, 10)" :key="tag" class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
              {{ tag }}
            </span>
          </div>
          <p v-else-if="!trackInfo" class="mt-3 text-xs text-dim">Content metadata not cached for this track.</p>
        </template>
        <p v-else class="text-sm text-dim">No track selected.</p>
      </Card>
    </div>

    <!-- Session telemetry strip -->
    <div class="page-enter mt-4 grid grid-cols-2 gap-3 md:grid-cols-4" style="animation-delay: 120ms">
      <div v-for="tile in sessionTiles" :key="tile.label" class="rounded-md border border-line bg-surface px-3 py-2.5">
        <div class="text-xs font-semibold tracking-wide text-muted uppercase">{{ tile.label }}</div>
        <div class="mt-0.5 truncate font-mono text-lg font-medium" :class="tile.value === '—' ? 'text-dim' : 'text-text'">
          {{ tile.value }}
        </div>
        <div class="text-xs text-dim">{{ tile.sub }}</div>
      </div>
    </div>

    <!-- Session timeline -->
    <Card v-if="sessionTimeline" class="page-enter mt-4" style="animation-delay: 150ms">
      <template #header>
        <h2 class="text-sm font-bold">Session timeline</h2>
        <span class="ml-auto font-mono text-xs text-dim">
          {{ sessionTimeline.elapsedLabel }} / {{ sessionTimeline.totalLabel }}
        </span>
      </template>

      <!-- weekend pips -->
      <div class="mb-3 flex items-center gap-1.5">
        <span
          v-for="n in sessionTimeline.count"
          :key="n"
          class="h-1.5 flex-1 rounded-full transition-colors"
          :class="
            n - 1 < sessionTimeline.index
              ? 'bg-ok/70'
              : n - 1 === sessionTimeline.index
                ? 'bg-accent'
                : 'bg-surface-3'
          "
          :title="`Session ${n} of ${sessionTimeline.count}`"
        />
      </div>

      <div class="flex items-center justify-between text-xs">
        <span class="font-semibold text-text">{{ sessionTimeline.type }}</span>
        <span class="text-dim">Session {{ sessionTimeline.index + 1 }} of {{ sessionTimeline.count }}</span>
      </div>

      <!-- elapsed fill for timed sessions -->
      <div v-if="sessionTimeline.timed" class="mt-2">
        <div class="h-2 overflow-hidden rounded-full bg-surface-3">
          <div
            class="h-full rounded-full bg-gradient-to-r from-accent/70 to-accent transition-[width] duration-700 ease-out"
            :style="{ width: `${(sessionTimeline.fraction ?? 0) * 100}%` }"
          />
        </div>
        <div class="mt-1 flex justify-between font-mono text-xs text-dim">
          <span>{{ sessionTimeline.elapsedLabel }} elapsed</span>
          <span v-if="sessionTimeline.remaining !== null">~{{ sessionTimeline.remaining }} min left</span>
        </div>
      </div>
      <p v-else class="mt-2 font-mono text-xs text-dim">
        Lap session — {{ sessionTimeline.totalLabel }}, {{ sessionTimeline.elapsedLabel }} elapsed
      </p>
    </Card>

    <!-- Current event -->
    <Card v-if="detail?.current_event?.id" class="page-enter mt-4" style="animation-delay: 180ms">
      <template #header>
        <h2 class="text-sm font-bold">Current event</h2>
        <span class="ml-auto text-xs text-dim">{{ detail.current_event.category }}</span>
      </template>
      <div class="flex flex-wrap items-center gap-1.5 text-xs">
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ detail.current_event.class }}</span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ detail.current_event.session }}</span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ detail.current_event.time }}</span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ detail.current_event.difficulty }}</span>
        <span
          v-if="detail.current_event.weather"
          class="inline-flex items-center gap-1.5 rounded-full border border-line bg-surface-2 py-0.5 pr-2.5 pl-0.5"
        >
          <img
            v-if="detail.current_event.weather_key"
            :src="weatherImageUrl(detail.current_event.weather_key)"
            alt=""
            class="size-5 rounded-full border border-line object-cover"
            @error="($event.target as HTMLImageElement).style.display = 'none'"
          />
          {{ detail.current_event.weather }}
        </span>
      </div>
    </Card>

    <!-- Live timing -->
    <Card v-if="inst.running && drivers.length" class="page-enter mt-4" style="animation-delay: 220ms">
      <template #header>
        <h2 class="text-sm font-bold">Live timing</h2>
        <span class="ml-auto font-mono text-xs text-dim">{{ connected.length }} connected</span>
      </template>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-line text-left text-xs tracking-wide text-muted uppercase">
              <th class="py-1.5 pr-2 font-semibold">Driver</th>
              <th class="py-1.5 pr-2 font-semibold max-sm:hidden">Car</th>
              <th class="py-1.5 pr-2 text-right font-semibold">Speed</th>
              <th class="py-1.5 pr-3 pl-2 font-semibold max-md:hidden">RPM</th>
              <th class="py-1.5 pr-2 text-center font-semibold">Gear</th>
              <th class="py-1.5 pr-2 text-right font-semibold">Laps</th>
              <th class="py-1.5 pr-2 text-right font-semibold">Last</th>
              <th class="py-1.5 pr-2 text-right font-semibold">Best</th>
              <th class="py-1.5 font-semibold"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="d in drivers" :key="d.car_id" class="border-b border-line/60">
              <td class="py-1.5 pr-2 font-medium">{{ d.name || "car " + d.car_id }}</td>
              <td class="py-1.5 pr-2 font-mono text-xs text-muted max-sm:hidden">{{ d.car }}</td>
              <td class="py-1.5 pr-2 text-right font-mono text-xs">
                {{ positionFor(d.car_id) ? `${speedKmh(positionFor(d.car_id))} km/h` : "—" }}
              </td>
              <td class="py-1.5 pr-3 pl-2 max-md:hidden">
                <div v-if="positionFor(d.car_id)?.engine_rpm" class="flex items-center gap-2">
                  <div class="h-1.5 w-20 overflow-hidden rounded-full bg-surface-3">
                    <div
                      class="h-full rounded-full transition-[width] duration-300"
                      :class="rpmPct(positionFor(d.car_id)) > 88 ? 'bg-danger' : rpmPct(positionFor(d.car_id)) > 70 ? 'bg-warn' : 'bg-accent'"
                      :style="{ width: `${rpmPct(positionFor(d.car_id))}%` }"
                    />
                  </div>
                  <span class="w-12 text-right font-mono text-xs tabular-nums text-muted">
                    {{ positionFor(d.car_id)!.engine_rpm.toLocaleString() }}
                  </span>
                </div>
                <span v-else class="font-mono text-xs text-dim">—</span>
              </td>
              <td class="py-1.5 pr-2 text-center font-mono text-xs font-semibold">{{ gearLabel(positionFor(d.car_id)) }}</td>
              <td class="py-1.5 pr-2 text-right">{{ d.laps }}</td>
              <td class="py-1.5 pr-2 text-right font-mono text-xs">{{ lapTime(d.last_lap_ms) }}</td>
              <td class="py-1.5 pr-2 text-right font-mono text-xs text-ok">{{ lapTime(d.best_lap_ms) }}</td>
              <td class="py-1.5 text-right">
                <Button variant="ghost" size="sm" :disabled="busy" aria-label="Kick driver" @click="kick(d.car_id, d.name)">
                  <Icon name="x" :size="14" />
                </Button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </Card>

    <!-- Grid with car imagery -->
    <Card v-if="gridRows.length" class="page-enter mt-4" style="animation-delay: 260ms">
      <template #header>
        <h2 class="text-sm font-bold">Grid</h2>
        <span class="ml-auto font-mono text-xs text-dim">
          {{ detail?.current_cars?.length ?? 0 }} slot{{ (detail?.current_cars?.length ?? 0) === 1 ? "" : "s" }}
        </span>
      </template>
      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <button
          v-for="row in gridRows"
          :key="`${row.carKey}-${row.skinKey}`"
          type="button"
          class="group overflow-hidden rounded-md border border-line bg-surface-2/40 text-left transition-colors hover:border-line-hi hover:bg-surface-2"
          @click="openCar(row)"
        >
          <div class="relative">
            <img
              :src="carImageUrl(row)"
              alt=""
              loading="lazy"
              class="aspect-video w-full border-b border-line object-cover"
              @error="($event.target as HTMLImageElement).style.display = 'none'"
            />
            <span class="absolute right-1.5 bottom-1.5 inline-flex items-center gap-1 rounded-md border border-line bg-bg/80 px-1.5 py-0.5 text-xs text-muted opacity-0 backdrop-blur-sm transition-opacity group-hover:opacity-100">
              <Icon name="activity" :size="12" />
              Specs
            </span>
          </div>
          <div class="p-2.5">
            <div class="flex items-baseline justify-between gap-2">
              <span class="min-w-0 truncate text-sm font-semibold">{{ row.car?.name || row.carKey }}</span>
              <span class="shrink-0 text-xs text-muted">×{{ row.count }}</span>
            </div>
            <div class="min-w-0 truncate text-xs text-dim">{{ row.car?.brand || "" }} · {{ row.skinName }}</div>
            <div v-if="row.car?.specs" class="mt-1.5 flex flex-wrap gap-x-3 gap-y-0.5 font-mono text-xs text-muted">
              <span v-if="row.car.specs.bhp">{{ row.car.specs.bhp }}</span>
              <span v-if="row.car.specs.weight">{{ row.car.specs.weight }}</span>
              <span v-if="row.car.specs.topspeed">{{ row.car.specs.topspeed }}</span>
            </div>
          </div>
        </button>
      </div>
    </Card>

    <!-- Spectator stream -->
    <Card v-if="inst.stream_enabled === 1 && inst.stream_embed_url" class="page-enter mt-4" style="animation-delay: 300ms">
      <template #header>
        <h2 class="text-sm font-bold">Spectator stream</h2>
        <Button variant="ghost" size="sm" class="ml-auto" @click="streamViewer = { title: `${inst.name} spectator`, url: inst.stream_embed_url! }">
          <Icon name="activity" :size="14" />
          Watch
        </Button>
      </template>
      <iframe
        :src="inst.stream_embed_url"
        :title="`${inst.name} spectator stream`"
        class="aspect-video w-full rounded-md border border-line bg-bg"
        allow="autoplay; fullscreen; picture-in-picture"
        sandbox="allow-scripts allow-same-origin allow-forms allow-presentation"
      />
    </Card>

    <!-- Upcoming queue -->
    <Card v-if="upcoming.length" class="page-enter mt-4" style="animation-delay: 340ms">
      <template #header>
        <h2 class="text-sm font-bold">Up next</h2>
        <RouterLink to="/queue" class="ml-auto text-xs text-accent hover:underline">Manage queue →</RouterLink>
      </template>
      <ul class="divide-y divide-line/60">
        <li v-for="(q, i) in upcoming" :key="q.id" class="flex items-center gap-3 py-2 first:pt-0 last:pb-0">
          <span class="w-5 shrink-0 text-right font-mono text-xs text-dim">{{ i + 1 }}</span>
          <img
            v-if="q.track_key"
            :src="trackUrl('outline', q.track_key, q.track_config ?? '')"
            alt=""
            loading="lazy"
            class="h-9 w-14 shrink-0 rounded-sm border border-line bg-surface-2/50 object-contain p-0.5"
            @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"
          />
          <div class="min-w-0">
            <div class="truncate text-sm font-medium">{{ q.track }}</div>
            <div class="truncate text-xs text-dim">{{ q.class }} · {{ q.session }} · {{ q.time }}</div>
          </div>
          <span v-if="q.started_at" class="ml-auto shrink-0 rounded-full border border-warn/40 bg-warn-glow px-2 py-0.5 text-xs text-warn">
            In progress
          </span>
        </li>
      </ul>
    </Card>

    <!-- Console -->
    <div class="page-enter mt-4" style="animation-delay: 380ms">
      <button
        type="button"
        class="inline-flex min-h-8 cursor-pointer items-center gap-2 rounded-md px-2 text-xs font-semibold text-muted transition-colors hover:bg-surface-2 hover:text-text"
        @click="consoleOpen = !consoleOpen"
      >
        <Icon name="terminal" :size="15" />
        {{ consoleOpen ? "Hide console" : "Show console" }}
      </button>
      <pre
        v-if="consoleOpen"
        class="mt-2 max-h-72 overflow-y-auto rounded-md border border-line bg-bg p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap text-muted"
        >{{ consoleLines() }}</pre>
    </div>

    <!-- Car detail: photo, skin, specs, power/torque curves -->
    <Sheet :open="!!carViewer" :title="carViewer?.car?.name || carViewer?.carKey || 'Car'" @close="carViewer = null">
      <template v-if="carViewer">
        <img
          :src="carImageUrl(carViewer)"
          alt=""
          class="aspect-video w-full rounded-md border border-line object-cover"
          @error="($event.target as HTMLImageElement).style.display = 'none'"
        />
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <span v-if="carViewer.car?.brand" class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
            {{ carViewer.car.brand }}
          </span>
          <span v-if="carViewer.car?.class" class="rounded-full border border-accent/40 bg-accent-dim px-2 py-0.5 text-xs text-accent">
            {{ carViewer.car.class }}
          </span>
          <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">{{ carViewer.skinName }}</span>
          <span class="ml-auto text-xs text-dim">×{{ carViewer.count }} on grid</span>
        </div>

        <dl v-if="carSpecRows.length" class="mt-4 grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-3">
          <div v-for="spec in carSpecRows" :key="spec.label" class="rounded-md border border-line bg-surface-2/40 px-2.5 py-1.5">
            <dt class="text-xs text-dim">{{ spec.label }}</dt>
            <dd class="font-mono text-sm text-text">{{ spec.value }}</dd>
          </div>
        </dl>

        <div class="mt-4">
          <h3 class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">Power &amp; torque</h3>
          <div v-if="carCurvesLoading" class="grid h-44 place-items-center rounded-md border border-line bg-surface-2/40 text-sm text-dim">
            Loading curves…
          </div>
          <div v-else-if="carChartSeries.length" class="rounded-md border border-line bg-surface-2/40 p-3">
            <LineChart :series="carChartSeries" :labels="carCurves?.labels ?? []" :height="200" x-label="RPM" />
          </div>
          <p v-else class="rounded-md border border-line bg-surface-2/40 px-3 py-3 text-sm text-dim">
            No dyno data shipped with this car.
          </p>
        </div>

        <p v-if="carDescText" class="mt-4 text-sm leading-relaxed whitespace-pre-line text-muted">{{ carDescText }}</p>
      </template>
      <template #footer>
        <Button variant="ghost" @click="carViewer = null">Close</Button>
      </template>
    </Sheet>

    <Sheet :open="!!streamViewer" :title="streamViewer?.title ?? 'Stream'" @close="streamViewer = null">
      <template v-if="streamViewer">
        <iframe
          :src="streamViewer.url"
          :title="streamViewer.title"
          class="aspect-video w-full rounded-md border border-line bg-bg"
          allow="autoplay; fullscreen; picture-in-picture"
          sandbox="allow-scripts allow-same-origin allow-forms allow-presentation"
        />
      </template>
      <template #footer>
        <Button variant="ghost" @click="streamViewer = null">Close</Button>
      </template>
    </Sheet>
  </template>
</template>

<style scoped>
/* Staggered entrance — one orchestrated reveal, then static. */
.page-enter {
  animation: rise 0.45s cubic-bezier(0.22, 0.9, 0.3, 1) both;
}
@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Map panel atmosphere: subtle blueprint grid over the page bg so the track
   artwork floats instead of sitting on flat black. */
.map-canvas {
  background-color: var(--color-bg);
  background-image:
    radial-gradient(ellipse at 30% 0%, rgba(98, 179, 232, 0.07), transparent 60%),
    linear-gradient(var(--color-line) 1px, transparent 1px),
    linear-gradient(90deg, var(--color-line) 1px, transparent 1px);
  background-size:
    100% 100%,
    32px 32px,
    32px 32px;
  background-position:
    0 0,
    -1px -1px,
    -1px -1px;
}
</style>
