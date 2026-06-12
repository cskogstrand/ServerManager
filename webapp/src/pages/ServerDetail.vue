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
            :style="{ aspectRatio: `${mapMeta.width || 16} / ${mapMeta.height || 9}` }"
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

      <!-- Circuit facts -->
      <Card>
        <template #header>
          <h2 class="text-sm font-bold">Circuit</h2>
          <span v-if="activeTrack" class="ml-auto font-mono text-xs text-dim">{{ activeTrack.key }}</span>
        </template>
        <template v-if="activeTrack">
          <img
            :src="trackUrl('preview', activeTrack.key, activeTrack.config)"
            alt=""
            class="mb-3 aspect-video w-full rounded-sm border border-line object-cover"
          />
          <div class="text-sm font-semibold">{{ trackInfo?.name || activeTrack.name }}</div>
          <div v-if="trackInfo?.country || trackInfo?.city" class="mb-2 text-xs text-dim">
            {{ [trackInfo?.city, trackInfo?.country].filter(Boolean).join(", ") }}
          </div>
          <dl class="grid grid-cols-2 gap-x-4 gap-y-1.5 text-sm">
            <dt class="text-muted">Length</dt>
            <dd class="font-mono text-xs leading-5">{{ trackLengthLabel(trackInfo) }}</dd>
            <dt class="text-muted">Width</dt>
            <dd class="font-mono text-xs leading-5">{{ trackInfo?.width || "—" }}</dd>
            <dt class="text-muted">Pitboxes</dt>
            <dd class="font-mono text-xs leading-5">{{ trackInfo?.pitboxes ?? "—" }}</dd>
            <dt class="text-muted">Layout</dt>
            <dd class="font-mono text-xs leading-5">{{ activeTrack.config || "default" }}</dd>
          </dl>
          <div v-if="trackInfo?.tags?.length" class="mt-3 flex flex-wrap gap-1.5">
            <span v-for="tag in trackInfo.tags.slice(0, 8)" :key="tag" class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
              {{ tag }}
            </span>
          </div>
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

    <!-- Current event -->
    <Card v-if="detail?.current_event?.id" class="page-enter mt-4" style="animation-delay: 180ms">
      <template #header>
        <h2 class="text-sm font-bold">Current event</h2>
        <span class="ml-auto text-xs text-dim">{{ detail.current_event.category }}</span>
      </template>
      <div class="flex flex-wrap gap-1.5 text-xs">
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ detail.current_event.class }}</span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ detail.current_event.session }}</span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ detail.current_event.time }}</span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ detail.current_event.difficulty }}</span>
        <span v-if="detail.current_event.weather" class="rounded-full border border-line bg-surface-2 px-2 py-0.5">
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
        <div v-for="row in gridRows" :key="`${row.carKey}-${row.skinKey}`" class="overflow-hidden rounded-md border border-line bg-surface-2/40">
          <img
            :src="carImageUrl(row)"
            alt=""
            loading="lazy"
            class="aspect-video w-full border-b border-line object-cover"
            @error="($event.target as HTMLImageElement).style.display = 'none'"
          />
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
        </div>
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
