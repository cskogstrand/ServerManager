<script setup lang="ts">
// Full-page race control view for one instance: track map front and center
// (rendered whenever the track ships map data, with or without live cars),
// circuit facts, session telemetry, live timing, grid with car imagery and
// the upcoming queue. Opened from the Dashboard card header.
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api, ApiError } from "@/lib/api";
import { useServerStore, type CarPositionState, type DriverState, type InstanceState, type StatusResponse, type TelemetryHealth } from "@/stores/server";
import { useContentStore } from "@/stores/content";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import {
  normalizeRaceSetup,
  raceSetupBody,
  raceSetupValid,
  type RaceSetupDraft,
} from "@/lib/useRaceSetupDraft";
import { computeRunningOrder, type TimingRow } from "@/lib/raceTelemetry";
import { isWhepUrl, useDriverStreams, type StreamChannel } from "@/lib/useDriverStreams";
import type { CacheCar, CacheTrack, UserClass, UserClassEntry } from "@/types/generated";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import WhepPlayer from "@/components/WhepPlayer.vue";
import Sheet from "@/components/ui/Sheet.vue";
import Modal from "@/components/ui/Modal.vue";
import RaceSetupEditor from "@/components/RaceSetupEditor.vue";
import StreamTheater from "@/components/StreamTheater.vue";
import TrackImage from "@/components/TrackImage.vue";
import StreamWall from "@/components/StreamWall.vue";
import Skeleton from "@/components/ui/Skeleton.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import LineChart from "@/components/ui/LineChart.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Combobox from "@/components/ui/Combobox.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Toggle from "@/components/ui/Toggle.vue";

interface CurrentEvent {
  id: number;
  name: string;
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
    type_id: number;
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
  telemetry: TelemetryHealth | null;
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
  name: string | null;
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

// --- Watchable streams (spectator cam + per-driver cams) ---
const driverStreams = useDriverStreams();
const theaterOpen = ref(false);
const theaterKey = ref<string | null>(null);

function openStream(key: string) {
  theaterKey.value = key;
  theaterOpen.value = true;
}

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

// Numeric session type drives the running-order sort (race vs. best-lap).
const sessionTypeNum = computed(() => inst.value?.session?.type ?? detail.value?.session?.type_id ?? 1);

// Ordered field for the live-timing tower (shared logic with the broadcast view).
const timingRows = computed<TimingRow[]>(() =>
  computeRunningOrder(drivers.value, positions.value, sessionTypeNum.value),
);

function carName(model: string): string {
  return content.carByKey(model)?.name || model;
}

const broadcastTo = computed(() => `/server/${instanceId.value}/broadcast`);

function timingFor(carId: number): TimingRow | undefined {
  return timingRows.value.find((r) => r.car_id === carId);
}

// Driver-stream lookup for the timing tower: car → guid → configured stream.
function guidForCar(carId: number): string | undefined {
  return drivers.value.find((d) => d.car_id === carId)?.guid;
}
function hasStreamForCar(carId: number): boolean {
  return !!driverStreams.streamForGuid(guidForCar(carId));
}

// Channels offered by the theater: the fixed spectator cam (if enabled) plus
// one per configured driver stream — offline drivers included, shown with a
// placeholder in the theater.
const streamChannels = computed<StreamChannel[]>(() => {
  const channels: StreamChannel[] = [];
  if (inst.value?.stream_enabled === 1 && inst.value.stream_embed_url) {
    channels.push({
      key: `spectator:${instanceId.value}`,
      title: `${inst.value.name} spectator`,
      subtitle: "Fixed spectator cam",
      url: inst.value.stream_embed_url,
      kind: isWhepUrl(inst.value.stream_embed_url) ? "whep" : "iframe",
      health: "unknown",
      online: true,
    });
  }
  channels.push(...driverStreamChannels.value);
  return channels;
});

// Driver streams only (the spectator cam has its own card) — for the inline
// stream wall. Includes offline drivers, shown as placeholders.
const driverStreamChannels = computed<StreamChannel[]>(() => driverStreams.allChannelsFor(drivers.value));
const onlineStreamCount = computed(() => driverStreamChannels.value.filter((c) => c.online).length);

// Telemetry health: the server can be "running" yet send nothing over the AC
// UDP plugin (misconfigured AssettoServer, crashed process, wrong ports). Flag
// that explicitly so an empty map/timing reads as a fault, not "no cars yet".
const telemetry = computed(() => inst.value?.telemetry ?? null);
const telemetryWarning = computed(() => {
  if (!inst.value?.running) return null;
  const t = telemetry.value;
  if (!t || !t.udp_online) {
    const port = t?.plugin_listen_port ?? null;
    return {
      title: "Live telemetry offline",
      detail:
        `Server Manager has not received any data from the AC UDP plugin` +
        (port ? ` on port ${port}` : "") +
        `. Players, live timing and the moving map stay empty until it connects. ` +
        `For AssettoServer, make sure EnableLegacyPluginInterface is enabled in cfg/extra_cfg.yml.`,
    };
  }
  return null;
});

async function fetchDetail() {
  try {
    const payload = await api.get<StatusPayload>(`/api/server/status?instance=${instanceId.value}`);
    detail.value = payload;
    // Sync the full live state (running, players, session, drivers, positions,
    // telemetry) into the store so every page reads one source of truth.
    server.syncStatus(instanceId.value, payload as unknown as StatusResponse);
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
// The map is a supporting view, not the centrepiece: cap it so it never eats
// the viewport. Width follows from the height cap (so tall, narrow circuits
// stay small) and is clamped to the card width for wide ones.
function mapCanvasStyle(meta: TrackMapMeta) {
  const ratio = (meta.width || 16) / (meta.height || 9);
  const cap = "min(42vh, 420px)";
  return {
    aspectRatio: String(ratio),
    width: `min(100%, calc(${cap} * ${ratio}))`,
    maxHeight: cap,
    marginInline: "auto",
  };
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

// AC numeric session type → display label (mirrors the backend mapping).
function sessionTypeLabel(t: number): string {
  return ["Booking", "Practice", "Qualify", "Race"][t] ?? "—";
}

// Unified live session view: the SSE-fed store is the source of truth (numeric
// type → label); the REST detail payload is only a fallback for the brief
// window before the store hydrates.
const liveSession = computed(() => {
  const s = inst.value?.session;
  if (s) {
    return {
      typeLabel: sessionTypeLabel(s.type),
      current_session_index: s.current_session_index,
      session_count: s.session_count,
      time: s.time,
      laps: s.laps,
      ambient_temp: s.ambient_temp,
      road_temp: s.road_temp,
      elapsed_ms: s.elapsed_ms,
    };
  }
  const d = detail.value?.session;
  if (d) {
    return {
      typeLabel: d.type || "—",
      current_session_index: d.current_session_index,
      session_count: d.session_count,
      time: d.time,
      laps: d.laps,
      ambient_temp: d.ambient_temp,
      road_temp: d.road_temp,
      elapsed_ms: d.elapsed_ms,
    };
  }
  return null;
});

function elapsed(): string {
  const ms = liveSession.value?.elapsed_ms ?? 0;
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

// --- Session timeline ---
// pips: one per session in the weekend (current highlighted). For timed
// sessions we also derive a fill fraction from elapsed/total; lap sessions
// have no clean denominator so the bar stays indeterminate.
const sessionTimeline = computed(() => {
  const s = liveSession.value;
  if (!inst.value?.running || !s || !s.session_count) return null;
  const idx = s.current_session_index ?? 0;
  const timed = !s.laps && s.time > 0;
  const totalMs = timed ? s.time * 60000 : 0;
  const fraction = timed ? Math.max(0, Math.min(1, s.elapsed_ms / totalMs)) : null;
  return {
    type: s.typeLabel,
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

// --- Race control (live UDP commands; only meaningful while running) ---
const broadcastMsg = ref("");
const adminCmd = ref("");

async function raceAction(fn: () => Promise<unknown>, ok: string) {
  busy.value = true;
  try {
    await fn();
    toast.success(ok);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

function broadcast() {
  const message = broadcastMsg.value.trim();
  if (!message) return;
  void raceAction(() => api.post(`/api/server/broadcast?instance=${instanceId.value}`, { message }), "Message broadcast.").then(
    () => (broadcastMsg.value = ""),
  );
}

function runAdmin() {
  const command = adminCmd.value.trim();
  if (!command) return;
  void raceAction(() => api.post(`/api/server/admin-command?instance=${instanceId.value}`, { command }), "Command sent.").then(
    () => (adminCmd.value = ""),
  );
}

async function nextSession() {
  const ok = await confirm.ask({
    title: "Advance session",
    message: "Skip to the next session now?",
    detail: "The current session ends immediately for everyone.",
    confirmLabel: "Next session",
  });
  if (ok) void raceAction(() => api.post(`/api/server/next-session?instance=${instanceId.value}`), "Advancing to next session.");
}

async function restartSession() {
  const ok = await confirm.ask({
    title: "Restart session",
    message: "Restart the current session?",
    detail: "Everyone returns to the start of the current session.",
    confirmLabel: "Restart session",
  });
  if (ok) void raceAction(() => api.post(`/api/server/restart-session?instance=${instanceId.value}`), "Restarting session.");
}

async function skipEvent() {
  const ok = await confirm.ask({
    title: "Skip current event",
    message: "Skip the current event and advance the queue?",
    detail: "Everyone on the server is kicked when the event rotates.",
    confirmLabel: "Skip event",
    tone: "danger",
  });
  if (!ok) return;
  busy.value = true;
  try {
    await api.post(`/api/queue/skipevent?instance=${instanceId.value}`);
    await fetchDetail();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

// --- Grid / weather editor for the running event ---
const gridOpen = ref(false);
const gridClassId = ref<number | null>(null);
const gridForm = ref<UserClass | null>(null);
const gridWeatherKey = ref("");
const gridRestart = ref(false);
const gridSaving = ref(false);

async function openGrid() {
  const ev = detail.value?.current_event;
  if (!ev?.id || !ev.class_id) {
    toast.error("No current event to edit.");
    return;
  }
  gridClassId.value = ev.class_id;
  gridWeatherKey.value = ev.weather_key ?? "";
  gridRestart.value = false;
  try {
    void content.load();
    const { data } = await api.get<{ data: UserClass }>(`/api/class/${ev.class_id}`);
    data.entries ??= [];
    for (const e of data.entries) e.count ??= 1;
    gridForm.value = data;
    gridOpen.value = true;
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

function gridSkins(carKey: string | undefined) {
  return content.carByKey(carKey)?.skins ?? [];
}
function addGridEntry() {
  gridForm.value?.entries?.push({ cache_car_key: undefined, skin_key: "", count: 1 } as UserClassEntry);
}
function removeGridEntry(i: number) {
  gridForm.value?.entries?.splice(i, 1);
}
function onGridCar(entry: UserClassEntry) {
  entry.skin_key = gridSkins(entry.cache_car_key)[0]?.key ?? "";
}

async function saveGrid() {
  const ev = detail.value?.current_event;
  if (!ev?.id || !gridForm.value) return;
  gridSaving.value = true;
  try {
    const entries = (gridForm.value.entries ?? [])
      .filter((e) => e.cache_car_key)
      .map((e) => ({ cache_car_key: e.cache_car_key, skin_key: e.skin_key ?? "", count: e.count ?? 1 }));
    const res = await api.post<{ restarted: boolean; restart_required: boolean }>(
      `/api/server/current-event?instance=${instanceId.value}`,
      {
        event_id: ev.id,
        class_id: gridClassId.value,
        weather_key: gridWeatherKey.value || undefined,
        class_entries: entries,
        restart_now: gridRestart.value,
      },
    );
    toast.success(res.restarted ? "Applied and restarted." : "Saved — restart the event to apply on track.");
    gridOpen.value = false;
    await fetchDetail();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    gridSaving.value = false;
  }
}

// --- Unified "Edit race setup": scope prompt → shared editor ---
// One entry point edits the current event's full setup. The scope prompt picks
// how the change lands: restart the event now to apply on track, save it for the
// next restart, or just edit the reusable source. The quick grid/weather sheet
// stays as a fast path for car/skin/weather-only tweaks.
type EditScope = "restart" | "after-restart" | "source";

const scopeOpen = ref(false);
const editOpen = ref(false);
const editScope = ref<EditScope>("after-restart");
const editDraft = ref<RaceSetupDraft | null>(null);
const editGroupId = ref<number | null>(null);
const editSaving = ref(false);

let editBaseline = "";
const markEditClean = () => (editBaseline = editDraft.value ? JSON.stringify(editDraft.value) : "");
useUnsavedGuard(() => editOpen.value && editDraft.value !== null && JSON.stringify(editDraft.value) !== editBaseline);

function openEditSetup() {
  if (!detail.value?.current_event?.id) {
    toast.error("No current event to edit.");
    return;
  }
  scopeOpen.value = true;
}

// Load the full saved event into a draft, then open the shared editor in the
// chosen scope. event_category_id isn't in the status payload, so we read it
// (and the rest of the setup) from the event record.
const chooseScope = (scope: EditScope) =>
  guardEdit(async () => {
    const ev = detail.value?.current_event;
    if (!ev?.id) return;
    void content.load();
    const raw = await api.get<Record<string, unknown>>(`/api/event/${ev.id}`);
    editDraft.value = normalizeRaceSetup(raw);
    editGroupId.value = raw.EventCategoryId != null ? Number(raw.EventCategoryId) : null;
    editScope.value = scope;
    markEditClean();
    scopeOpen.value = false;
    editOpen.value = true;
  });

async function guardEdit(fn: () => Promise<void>) {
  busy.value = true;
  try {
    await fn();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

function openQuickGrid() {
  scopeOpen.value = false;
  void openGrid();
}

async function saveEditSetup() {
  const ev = detail.value?.current_event;
  const draft = editDraft.value;
  if (!ev?.id || !draft || editGroupId.value === null) return;
  if (!raceSetupValid(draft)) {
    toast.error("Track and all four presets are required.");
    return;
  }
  editSaving.value = true;
  try {
    await api.put(`/api/event/${ev.id}`, raceSetupBody(draft, editGroupId.value));
    if (editScope.value === "restart") {
      // Re-render from the freshly saved event and restart it on track.
      const res = await api.post<{ restarted: boolean }>(`/api/server/current-event?instance=${instanceId.value}`, {
        event_id: ev.id,
        restart_now: true,
      });
      toast.success(res.restarted ? "Saved and restarted on track." : "Saved — restart the event to apply on track.");
    } else if (editScope.value === "after-restart") {
      toast.success("Saved — applies when the event next restarts.");
    } else {
      toast.success("Race setup updated.");
    }
    editOpen.value = false;
    markEditClean();
    await fetchDetail();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    editSaving.value = false;
  }
}

const editScopeTitle = computed(() =>
  editScope.value === "restart"
    ? "Edit race setup — restart now"
    : editScope.value === "after-restart"
      ? "Edit race setup — apply after restart"
      : "Edit reusable race setup",
);

onMounted(async () => {
  await server.load();
  if (!server.instances[instanceId.value]) {
    toast.error("Unknown server instance.");
    void router.replace("/");
    return;
  }
  void content.load();
  void driverStreams.loadStreams();
  driverStreams.startHealthPoll(() => instanceId.value);
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
  driverStreams.stopHealthPoll();
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
        <RouterLink
          :to="broadcastTo"
          class="inline-flex min-h-8 items-center gap-1.5 rounded-md border border-accent/45 bg-accent/15 px-2.5 text-xs font-semibold text-accent transition-colors hover:border-accent/70 hover:bg-accent/25"
          :class="inst.running ? 'shadow-[0_0_20px_rgba(98,179,232,0.28)]' : ''"
          title="Open the full-screen broadcast overlay"
        >
          <span v-if="inst.running" class="size-1.5 animate-pulse rounded-full bg-accent" />
          <Icon name="broadcast" :size="14" />
          Broadcast
        </RouterLink>
        <Button
          v-if="inst.running && (detail?.current_event?.id ?? 0) > 0"
          variant="ghost"
          size="sm"
          :disabled="busy"
          @click="skipEvent"
        >
          <Icon name="skip" :size="15" />
          Skip
        </Button>
        <Button :variant="inst.running ? 'danger' : 'success'" size="sm" :disabled="busy" @click="toggleServer">
          <Icon :name="inst.running ? 'stop' : 'power'" :size="15" />
          {{ busy ? "Working" : inst.running ? "Stop" : "Start" }}
        </Button>
      </div>
    </header>

    <!-- Telemetry fault banner -->
    <div
      v-if="telemetryWarning"
      class="page-enter mb-4 flex items-start gap-2.5 rounded-md border border-warn/40 bg-warn-glow px-3 py-2.5"
    >
      <Icon name="alert" :size="18" class="mt-0.5 shrink-0 text-warn" />
      <div class="min-w-0">
        <div class="text-sm font-semibold text-warn">{{ telemetryWarning.title }}</div>
        <p class="text-xs text-muted">{{ telemetryWarning.detail }}</p>
      </div>
    </div>

    <!-- Overview ribbon — the whole server at a glance: status, session, time,
         drivers, conditions and the loaded event in one scannable band. -->
    <section
      class="page-enter mb-4 overflow-hidden rounded-lg border border-line shadow-[0_18px_45px_rgba(0,0,0,0.18)]"
      style="animation-delay: 30ms"
    >
      <div class="grid grid-cols-2 gap-px bg-line sm:grid-cols-3 xl:grid-cols-6">
        <!-- Status -->
        <div class="bg-surface px-4 py-3">
          <div class="text-[11px] font-semibold tracking-wide text-dim uppercase">Status</div>
          <div class="mt-1 flex items-center gap-2">
            <span
              class="size-2.5 rounded-full"
              :class="inst.running ? 'bg-ok shadow-[0_0_12px_rgba(79,216,132,0.55)]' : 'bg-dim'"
            />
            <span class="text-base font-bold" :class="inst.running ? 'text-text' : 'text-dim'">
              {{ inst.running ? "Running" : "Stopped" }}
            </span>
          </div>
          <div class="mt-0.5 font-mono text-xs text-dim">{{ inst.running ? `${elapsed()} elapsed` : "offline" }}</div>
        </div>

        <!-- Session -->
        <div class="bg-surface px-4 py-3">
          <div class="text-[11px] font-semibold tracking-wide text-dim uppercase">Session</div>
          <div class="mt-1 truncate text-base font-bold" :class="inst.running && liveSession ? 'text-text' : 'text-dim'">
            {{ inst.running && liveSession ? liveSession.typeLabel : "—" }}
          </div>
          <div v-if="inst.running && liveSession?.session_count" class="mt-2 flex items-center gap-1">
            <span
              v-for="n in liveSession.session_count"
              :key="n"
              class="h-1 flex-1 rounded-full transition-colors"
              :class="
                n - 1 < (liveSession.current_session_index ?? 0)
                  ? 'bg-ok/70'
                  : n - 1 === (liveSession.current_session_index ?? 0)
                    ? 'bg-accent'
                    : 'bg-surface-3'
              "
            />
          </div>
          <div v-else class="mt-0.5 font-mono text-xs text-dim">no session</div>
        </div>

        <!-- Time -->
        <div class="bg-surface px-4 py-3">
          <div class="text-[11px] font-semibold tracking-wide text-dim uppercase">Time</div>
          <div class="mt-1 font-mono text-base font-bold" :class="inst.running ? 'text-text' : 'text-dim'">
            {{ inst.running ? elapsed() : "—" }}
          </div>
          <template v-if="sessionTimeline?.timed">
            <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-surface-3">
              <div
                class="h-full rounded-full bg-gradient-to-r from-accent/70 to-accent transition-[width] duration-700 ease-out"
                :style="{ width: `${(sessionTimeline.fraction ?? 0) * 100}%` }"
              />
            </div>
            <div class="mt-1 font-mono text-xs text-dim">
              {{ sessionTimeline.remaining !== null ? `~${sessionTimeline.remaining} min left` : sessionTimeline.totalLabel }}
            </div>
          </template>
          <div v-else class="mt-0.5 font-mono text-xs text-dim">{{ sessionTimeline?.totalLabel ?? "—" }}</div>
        </div>

        <!-- Drivers -->
        <div class="bg-surface px-4 py-3">
          <div class="text-[11px] font-semibold tracking-wide text-dim uppercase">Drivers</div>
          <div class="mt-1 font-mono text-base font-bold" :class="inst.running ? 'text-text' : 'text-dim'">
            {{ inst.running ? connected.length : 0 }}
          </div>
          <div class="mt-0.5 font-mono text-xs text-dim">{{ positions.length }} on map · {{ detail?.current_cars?.length ?? 0 }} slots</div>
        </div>

        <!-- Conditions -->
        <div class="bg-surface px-4 py-3">
          <div class="text-[11px] font-semibold tracking-wide text-dim uppercase">Conditions</div>
          <div class="mt-1 font-mono text-base font-bold" :class="inst.running && liveSession ? 'text-text' : 'text-dim'">
            {{ inst.running && liveSession ? `${liveSession.ambient_temp}° / ${liveSession.road_temp}°` : "—" }}
          </div>
          <div class="mt-0.5 font-mono text-xs text-dim">air / road</div>
        </div>

        <!-- Event -->
        <div class="bg-surface px-4 py-3">
          <div class="text-[11px] font-semibold tracking-wide text-dim uppercase">Event</div>
          <div
            class="mt-1 truncate text-base font-bold"
            :class="detail?.current_event?.id ? 'text-text' : 'text-dim'"
            :title="detail?.current_event?.name || detail?.current_event?.track || ''"
          >
            {{ detail?.current_event?.id ? detail.current_event.name || detail.current_event.track : "None" }}
          </div>
          <div class="mt-0.5 truncate font-mono text-xs text-dim">{{ detail?.current_event?.category || "nothing loaded" }}</div>
        </div>
      </div>
    </section>

    <!-- Live region: spatial track map (a supporting view) beside the timing tower -->
    <div class="page-enter grid gap-4 lg:grid-cols-3" style="animation-delay: 60ms">
      <section class="overflow-hidden rounded-md border border-line bg-surface shadow-[0_18px_45px_rgba(0,0,0,0.18)] lg:col-span-2">
        <!-- Compact track header: photo backdrop + name, location, live status -->
        <div v-if="activeTrack" class="relative">
          <img
            :src="trackUrl('preview', activeTrack.key, activeTrack.config)"
            alt=""
            class="absolute inset-0 size-full object-cover"
            @error="($event.target as HTMLImageElement).style.display = 'none'"
          />
          <div class="absolute inset-0 bg-gradient-to-t from-surface via-surface/90 to-surface/35" />
          <div class="relative flex items-end gap-3 px-4 pt-12 pb-3">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span
                  v-if="activeTrack.source === 'queued'"
                  class="rounded-full border border-accent/40 bg-accent-dim px-2 py-0.5 text-[11px] font-semibold text-accent"
                >
                  Up next
                </span>
                <span v-if="trackInfo?.country || trackInfo?.city" class="text-[11px] font-medium tracking-wide text-muted uppercase">
                  {{ [trackInfo?.city, trackInfo?.country].filter(Boolean).join(" · ") }}
                </span>
              </div>
              <h2 class="truncate text-xl font-black tracking-tight text-text drop-shadow-[0_2px_8px_rgba(0,0,0,0.7)]">
                {{ trackInfo?.name || activeTrack.name }}
              </h2>
            </div>
            <span
              class="ml-auto inline-flex shrink-0 items-center gap-1.5 self-start rounded-full border px-2 py-1 font-mono text-xs"
              :class="telemetryWarning ? 'border-warn/40 bg-warn-glow text-warn' : positions.length ? 'border-ok/40 bg-ok-glow text-ok' : 'border-line bg-surface-2/80 text-dim'"
            >
              <span v-if="positions.length && !telemetryWarning" class="size-1.5 animate-pulse rounded-full bg-ok" />
              {{
                telemetryWarning
                  ? "plugin offline"
                  : positions.length
                    ? `${positions.length} car${positions.length === 1 ? "" : "s"} on track`
                    : inst.running
                      ? "waiting for cars"
                      : "stopped"
              }}
            </span>
          </div>
          <!-- Circuit facts strip -->
          <dl class="relative flex flex-wrap gap-x-5 gap-y-1 border-t border-line bg-surface-2/40 px-4 py-2 font-mono text-xs">
            <div class="flex items-center gap-1.5">
              <span class="text-dim">Length</span>
              <span class="text-muted">{{ trackLengthLabel(trackInfo) }}</span>
            </div>
            <div v-if="trackInfo?.width" class="flex items-center gap-1.5">
              <span class="text-dim">Width</span>
              <span class="text-muted">{{ trackInfo.width }}</span>
            </div>
            <div class="flex items-center gap-1.5">
              <span class="text-dim">Pitboxes</span>
              <span class="text-muted">{{ trackInfo?.pitboxes ?? "—" }}</span>
            </div>
            <div class="flex items-center gap-1.5">
              <span class="text-dim">Layout</span>
              <span class="text-muted">{{ activeTrack.config || "default" }}</span>
            </div>
          </dl>
        </div>

        <!-- Map canvas (height-capped so it never dominates the page) -->
        <div class="p-4">
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
                  class="map-puck absolute grid size-6 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full border text-[9px] font-black transition-[left,top] duration-200 ease-linear hover:z-20 hover:scale-110"
                  :class="
                    timingFor(d.car_id)?.isLeader
                      ? 'border-bg bg-accent text-bg shadow-[0_0_18px_rgba(98,179,232,0.7)]'
                      : 'border-bg bg-surface-4 text-text shadow-[0_0_12px_rgba(0,0,0,0.6)]'
                  "
                  :style="mapPoint(positionFor(d.car_id)!, mapMeta)"
                  :title="`P${timingFor(d.car_id)?.position ?? '?'} · ${d.name || 'car ' + d.car_id} · ${speedKmh(positionFor(d.car_id))} km/h · gear ${gearLabel(positionFor(d.car_id))}`"
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
              <RouterLink
                v-if="inst.running"
                :to="broadcastTo"
                class="absolute right-2 bottom-2 inline-flex items-center gap-1.5 rounded-md border border-accent/45 bg-bg/80 px-2.5 py-1 text-xs font-semibold text-accent backdrop-blur-sm transition-colors hover:border-accent/70 hover:bg-accent/15"
              >
                <Icon name="maximize" :size="13" />
                Broadcast view
              </RouterLink>
            </div>

            <!-- No map.ini / map.png: fall back to the outline drawing -->
            <div v-else class="map-canvas relative grid place-items-center overflow-hidden rounded-md border border-line py-8">
              <img
                :src="trackUrl('outline', activeTrack.key, activeTrack.config)"
                alt=""
                class="max-h-64 opacity-80"
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
        </div>
      </section>

      <!-- Timing tower while running; circuit detail when idle -->
      <Card class="overflow-hidden lg:col-span-1">
        <template #header>
          <Icon :name="inst.running ? 'activity' : 'info'" :size="15" :class="inst.running ? 'text-accent' : 'text-dim'" />
          <h2 class="text-sm font-bold">{{ inst.running ? "Live timing" : "Circuit" }}</h2>
          <span class="ml-auto truncate font-mono text-xs text-dim">
            {{ inst.running ? `${connected.length} on grid` : activeTrack?.key }}
          </span>
        </template>

        <!-- Running: ordered timing tower (tuned for a small field) -->
        <template v-if="inst.running">
          <ol v-if="timingRows.length" class="max-h-[58vh] space-y-2 overflow-y-auto pr-0.5">
            <li
              v-for="row in timingRows"
              :key="row.car_id"
              class="group rounded-md border p-2.5 transition-colors"
              :class="row.isLeader ? 'border-accent/45 bg-accent-dim' : 'border-line bg-surface-2/40'"
            >
              <div class="flex items-center gap-2.5">
                <span
                  class="grid size-7 shrink-0 place-items-center rounded-md font-mono text-sm font-black tabular-nums"
                  :class="row.isLeader ? 'bg-accent text-bg' : 'bg-surface-3 text-muted'"
                >
                  {{ row.position }}
                </span>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-semibold">{{ row.name }}</div>
                  <div class="truncate font-mono text-[11px] text-dim">{{ carName(row.carModel) }}</div>
                </div>
                <div class="shrink-0 text-right">
                  <div
                    class="font-mono text-xs font-semibold"
                    :class="row.gapTone === 'leader' ? 'text-accent' : row.gapTone === 'warn' ? 'text-warn' : 'text-muted'"
                  >
                    {{ row.gapLabel }}
                  </div>
                  <div class="font-mono text-[11px] text-dim">L{{ row.laps }}</div>
                </div>
                <RouterLink
                  v-if="guidForCar(row.car_id)"
                  :to="{ name: 'driver-detail', params: { guid: guidForCar(row.car_id)! } }"
                  class="shrink-0 rounded-md p-1 text-dim transition-colors hover:bg-surface-3 hover:text-accent"
                  :title="`${row.name} — driver detail`"
                  :aria-label="`${row.name} — driver detail`"
                >
                  <Icon name="user" :size="14" />
                </RouterLink>
                <button
                  v-if="hasStreamForCar(row.car_id)"
                  type="button"
                  class="shrink-0 rounded-md p-1 text-accent/80 transition-colors hover:bg-accent-dim hover:text-accent"
                  :aria-label="`Watch ${row.name}'s stream`"
                  :title="`Watch ${row.name}'s stream`"
                  @click="openStream(`driver:${guidForCar(row.car_id)}`)"
                >
                  <Icon name="play" :size="14" />
                </button>
                <button
                  type="button"
                  class="shrink-0 rounded-md p-1 text-dim opacity-0 transition-opacity hover:text-danger group-hover:opacity-100"
                  :disabled="busy"
                  aria-label="Kick driver"
                  @click="kick(row.car_id, row.name)"
                >
                  <Icon name="x" :size="14" />
                </button>
              </div>

              <div class="mt-2 flex items-center gap-3 font-mono text-xs">
                <span class="flex items-baseline gap-1.5">
                  <span class="text-[10px] tracking-wide text-dim">LAST</span>
                  <span :class="row.last_lap_ms ? 'text-text' : 'text-dim'">{{ lapTime(row.last_lap_ms) }}</span>
                </span>
                <span class="flex items-baseline gap-1.5">
                  <span class="text-[10px] tracking-wide text-dim">BEST</span>
                  <span :class="row.best_lap_ms ? 'text-ok' : 'text-dim'">{{ lapTime(row.best_lap_ms) }}</span>
                </span>
              </div>

              <!-- Live: speed, gear, RPM -->
              <div class="mt-2 flex items-center gap-2.5">
                <span class="font-mono text-sm tabular-nums">
                  <span class="font-bold">{{ speedKmh(row.pos) }}</span>
                  <span class="text-[10px] text-dim"> km/h</span>
                </span>
                <span
                  class="grid size-5 place-items-center rounded-sm bg-surface-3 font-mono text-[11px] font-bold text-muted"
                >
                  {{ gearLabel(row.pos) }}
                </span>
                <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-surface-3">
                  <div
                    class="h-full rounded-full transition-[width] duration-300"
                    :class="rpmPct(row.pos) > 88 ? 'bg-danger' : rpmPct(row.pos) > 70 ? 'bg-warn' : 'bg-accent'"
                    :style="{ width: `${rpmPct(row.pos)}%` }"
                  />
                </div>
              </div>

              <!-- Track-position progress (relative running order at a glance) -->
              <div class="mt-2 h-1 overflow-hidden rounded-full bg-surface-3">
                <div
                  class="h-full rounded-full"
                  :class="row.isLeader ? 'bg-accent/70' : 'bg-line-hi'"
                  :style="{ width: `${row.splinePct}%` }"
                />
              </div>
            </li>
          </ol>

          <div v-else class="rounded-md border border-line bg-surface-2/40 px-3 py-6 text-center">
            <Icon name="car" :size="22" class="mx-auto text-dim" />
            <p class="mt-2 text-sm text-muted">No drivers connected yet.</p>
            <p class="text-xs text-dim">Live timing fills in as cars join the session.</p>
          </div>
        </template>

        <!-- Idle: circuit description + tags (the outline map sits beside this) -->
        <template v-else-if="activeTrack">
          <p v-if="trackInfo?.desc" class="text-sm leading-relaxed text-muted line-clamp-6">{{ trackInfo.desc }}</p>
          <p v-else class="text-sm text-dim">No description shipped with this layout.</p>
          <div v-if="trackInfo?.tags?.length" class="mt-3 flex flex-wrap gap-1.5">
            <span v-for="tag in trackInfo.tags.slice(0, 12)" :key="tag" class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
              {{ tag }}
            </span>
          </div>
          <p v-else-if="!trackInfo" class="mt-3 text-xs text-dim">Content metadata not cached for this track.</p>
        </template>
        <p v-else class="text-sm text-dim">No track selected.</p>
      </Card>
    </div>

    <!-- Operate now: the loaded event and what's queued next, side by side -->
    <div class="page-enter mt-4 grid gap-4 lg:grid-cols-2" style="animation-delay: 90ms">
      <!-- Current event -->
      <Card v-if="detail?.current_event?.id" class="min-w-0">
        <template #header>
          <Icon name="events" :size="15" class="text-accent" />
          <h2 class="text-sm font-bold">Current event</h2>
          <span class="truncate text-xs text-dim">{{ detail.current_event.category }}</span>
        </template>
        <template #actions>
          <Button variant="dark" size="sm" :disabled="busy" @click="openEditSetup">
            <Icon name="edit" :size="14" />
            Edit race setup
          </Button>
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
      <Card v-else class="min-w-0">
        <template #header>
          <Icon name="events" :size="15" class="text-dim" />
          <h2 class="text-sm font-bold">Current event</h2>
        </template>
        <p class="text-sm text-dim">Nothing loaded. Start the server or queue an event to load one.</p>
      </Card>

      <!-- Up next -->
      <Card class="min-w-0">
        <template #header>
          <Icon name="queue" :size="15" class="text-dim" />
          <h2 class="text-sm font-bold">Up next</h2>
          <RouterLink to="/queue" class="ml-auto text-xs text-accent hover:underline">Manage queue →</RouterLink>
        </template>
        <ul v-if="upcoming.length" class="divide-y divide-line/60">
          <li v-for="(q, i) in upcoming" :key="q.id" class="flex items-center gap-3 py-2 first:pt-0 last:pb-0">
            <span class="w-5 shrink-0 text-right font-mono text-xs text-dim">{{ i + 1 }}</span>
            <TrackImage
              v-if="q.track_key"
              :track-key="q.track_key"
              :config="q.track_config ?? ''"
              class="h-9 w-14 shrink-0 rounded-sm border border-line bg-surface-2/50"
            />
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium">{{ q.name || q.track }}</div>
              <div class="truncate text-xs text-dim">
                {{ q.name ? `${q.track} · ` : "" }}{{ q.class }} · {{ q.session }} · {{ q.time }}
              </div>
            </div>
            <span v-if="q.started_at" class="ml-auto shrink-0 rounded-full border border-warn/40 bg-warn-glow px-2 py-0.5 text-xs text-warn">
              In progress
            </span>
          </li>
        </ul>
        <p v-else class="text-sm text-dim">Queue is empty — add events from the queue page.</p>
      </Card>
    </div>

    <!-- Race control: live UDP commands, surfaced near the top while running -->
    <Card v-if="inst.running" class="page-enter mt-4" style="animation-delay: 120ms">
      <template #header>
        <Icon name="broadcast" :size="15" class="text-accent" />
        <h2 class="text-sm font-bold">Race control</h2>
      </template>
      <div class="space-y-3">
        <div class="flex flex-wrap gap-2">
          <Button variant="dark" size="sm" :disabled="busy" @click="nextSession">
            <Icon name="skip" :size="14" />
            Next session
          </Button>
          <Button variant="dark" size="sm" :disabled="busy" @click="restartSession">
            <Icon name="repeat" :size="14" />
            Restart session
          </Button>
        </div>

        <form class="flex gap-2" @submit.prevent="broadcast">
          <Input v-model="broadcastMsg" placeholder="Broadcast a message to all drivers…" class="flex-1" />
          <Button type="submit" size="sm" :disabled="busy || !broadcastMsg.trim()">Send</Button>
        </form>

        <form class="flex gap-2" @submit.prevent="runAdmin">
          <Input v-model="adminCmd" placeholder="Admin command, e.g. /kick name or /ballast 0 50" class="flex-1 font-mono" />
          <Button type="submit" variant="dark" size="sm" :disabled="busy || !adminCmd.trim()">Run</Button>
        </form>
        <p class="text-xs text-dim">
          Admin commands run through the server's ACSP plugin — the same ones the in-game admin uses.
        </p>
      </div>
    </Card>

    <!-- Grid with car imagery -->
    <Card v-if="gridRows.length" class="page-enter mt-4" style="animation-delay: 160ms">
      <template #header>
        <Icon name="car" :size="15" class="text-dim" />
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
        <Icon name="broadcast" :size="15" class="text-dim" />
        <h2 class="text-sm font-bold">Spectator stream</h2>
        <Button variant="ghost" size="sm" class="ml-auto" @click="openStream(`spectator:${instanceId}`)">
          <Icon name="play" :size="14" />
          Watch
        </Button>
      </template>
      <div v-if="isWhepUrl(inst.stream_embed_url)" class="aspect-video w-full overflow-hidden rounded-md border border-line">
        <WhepPlayer :url="inst.stream_embed_url" />
      </div>
      <iframe
        v-else
        :src="inst.stream_embed_url"
        :title="`${inst.name} spectator stream`"
        class="aspect-video w-full rounded-md border border-line bg-bg"
        allow="autoplay; fullscreen; picture-in-picture"
        sandbox="allow-scripts allow-same-origin allow-forms allow-presentation"
      />
    </Card>

    <!-- Driver streams: live tiles for every configured stream, watch full screen -->
    <section v-if="driverStreamChannels.length" class="page-enter mt-4" style="animation-delay: 350ms">
      <div class="mb-3 flex items-center gap-2">
        <Icon name="broadcast" :size="15" class="text-dim" />
        <h2 class="text-sm font-bold">Driver streams</h2>
        <span class="font-mono text-xs text-dim">{{ onlineStreamCount }}/{{ driverStreamChannels.length }} live</span>
      </div>
      <StreamWall :channels="driverStreamChannels" @watch="openStream" />
    </section>

    <!-- Console -->
    <div class="page-enter mt-4" style="animation-delay: 400ms">
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

    <!-- Edit-scope prompt: how should the change land? -->
    <Modal :open="scopeOpen" title="Edit race setup" @close="scopeOpen = false">
      <p class="mb-3 text-sm text-muted">
        Editing
        <span class="font-medium text-text">{{ detail?.current_event?.name || detail?.current_event?.track }}</span>.
        Choose how the change should apply.
      </p>
      <div class="space-y-2">
        <button
          v-if="inst?.running"
          type="button"
          class="flex w-full items-start gap-3 rounded-md border border-line bg-surface-2/40 p-3 text-left transition-colors hover:border-accent/50 hover:bg-surface-2"
          :disabled="busy"
          @click="chooseScope('restart')"
        >
          <Icon name="repeat" :size="18" class="mt-0.5 shrink-0 text-accent" />
          <span>
            <span class="block text-sm font-semibold">Restart now</span>
            <span class="block text-xs text-muted">Save and restart the event so the new setup loads on track immediately. Players are reconnected.</span>
          </span>
        </button>
        <button
          v-if="inst?.running"
          type="button"
          class="flex w-full items-start gap-3 rounded-md border border-line bg-surface-2/40 p-3 text-left transition-colors hover:border-accent/50 hover:bg-surface-2"
          :disabled="busy"
          @click="chooseScope('after-restart')"
        >
          <Icon name="clock" :size="18" class="mt-0.5 shrink-0 text-muted" />
          <span>
            <span class="block text-sm font-semibold">Apply after restart</span>
            <span class="block text-xs text-muted">Save changes now; they take effect the next time this event restarts. The current session keeps running.</span>
          </span>
        </button>
        <button
          type="button"
          class="flex w-full items-start gap-3 rounded-md border border-line bg-surface-2/40 p-3 text-left transition-colors hover:border-accent/50 hover:bg-surface-2"
          :disabled="busy"
          @click="chooseScope('source')"
        >
          <Icon name="edit" :size="18" class="mt-0.5 shrink-0 text-muted" />
          <span>
            <span class="block text-sm font-semibold">Edit reusable source</span>
            <span class="block text-xs text-muted">Edit the saved race setup in your library, with no effect on the running server.</span>
          </span>
        </button>
      </div>
      <div class="mt-3 border-t border-line pt-3">
        <button
          type="button"
          class="inline-flex items-center gap-1.5 text-xs font-semibold text-muted hover:text-text"
          :disabled="!detail?.current_event?.class_id"
          @click="openQuickGrid"
        >
          <Icon name="activity" :size="13" />
          Quick grid &amp; weather only
        </button>
      </div>
      <template #footer>
        <Button variant="ghost" @click="scopeOpen = false">Cancel</Button>
      </template>
    </Modal>

    <!-- Shared race-setup editor for the current event -->
    <Sheet :open="editOpen" :title="editScopeTitle" @close="editOpen = false">
      <RaceSetupEditor v-if="editDraft" v-model="editDraft" :instance-id="instanceId" />
      <template #footer>
        <span v-if="!raceSetupValid(editDraft)" class="mr-auto self-center text-xs text-muted">
          Track and all four presets are required.
        </span>
        <Button variant="ghost" @click="editOpen = false">Cancel</Button>
        <Button :disabled="editSaving || !raceSetupValid(editDraft)" @click="saveEditSetup">
          {{
            editSaving
              ? "Saving…"
              : editScope === "restart"
                ? "Save & restart"
                : "Save setup"
          }}
        </Button>
      </template>
    </Sheet>

    <!-- Grid & weather editor for the running event -->
    <Sheet :open="gridOpen" title="Edit grid & weather" @close="gridOpen = false">
      <template v-if="gridForm">
        <FormRow label="Weather" hint="Applies to the event's time/weather preset">
          <Combobox
            v-model="gridWeatherKey"
            placeholder="Search weather…"
            :options="content.weathers.map((w) => ({ value: w.key ?? '', label: w.name ?? w.key ?? '' }))"
          />
        </FormRow>

        <h3 class="mt-4 mb-2 text-xs font-semibold tracking-wide text-muted uppercase">Grid entries</h3>
        <div
          v-for="(entry, i) in gridForm.entries"
          :key="i"
          class="mb-2 flex flex-wrap items-end gap-2 rounded-md border border-line bg-surface-2/50 p-2"
        >
          <div class="min-w-40 flex-1">
            <Combobox
              v-model="entry.cache_car_key"
              placeholder="Search cars…"
              :options="content.cars.map((c) => ({ value: c.key ?? '', label: c.name ?? c.key ?? '' }))"
              @update:model-value="onGridCar(entry)"
            />
          </div>
          <Select
            v-model="entry.skin_key"
            class="w-32"
            :options="gridSkins(entry.cache_car_key).map((s) => ({ value: s.key, label: s.name || s.key }))"
          />
          <Input v-model="entry.count" type="number" :min="1" :max="64" class="w-16" />
          <Button variant="ghost" size="sm" aria-label="Remove entry" @click="removeGridEntry(i)">
            <Icon name="x" :size="14" />
          </Button>
        </div>
        <Button variant="dark" size="sm" @click="addGridEntry">
          <Icon name="plus" :size="14" />
          Add car
        </Button>

        <div class="mt-4 border-t border-line pt-3">
          <Toggle v-model="gridRestart" label="Restart the event now to apply on track" />
        </div>
      </template>
      <template #footer>
        <Button variant="ghost" @click="gridOpen = false">Cancel</Button>
        <Button :disabled="gridSaving" @click="saveGrid">{{ gridSaving ? "Saving…" : "Save changes" }}</Button>
      </template>
    </Sheet>

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

    <StreamTheater
      :open="theaterOpen"
      :channels="streamChannels"
      :initial-key="theaterKey"
      @close="theaterOpen = false"
    />
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
