<script setup lang="ts">
// Live multi-instance dashboard. Status/players/session arrive over SSE;
// the detail payload (current event, cars, console) is fetched per instance
// and refreshed when SSE reports a change. Console refreshes on a short
// timer only while its panel is open.
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { api, ApiError } from "@/lib/api";
import { useServerStore } from "@/stores/server";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import { useContentStore } from "@/stores/content";
import type { UserClass, UserClassEntry } from "@/types/generated";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Combobox from "@/components/ui/Combobox.vue";
import Toggle from "@/components/ui/Toggle.vue";
import Sheet from "@/components/ui/Sheet.vue";
import FormRow from "@/components/ui/FormRow.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import Skeleton from "@/components/ui/Skeleton.vue";

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
  cfg_path: string;
  current_event: CurrentEvent;
  drivers: import("@/stores/server").DriverState[];
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
  positions: import("@/stores/server").CarPositionState[];
}

interface StreamHealth {
  status: "not_configured" | "unknown" | "live" | "offline";
  status_code?: number;
  message?: string;
}

interface DriverStream {
  id?: number;
  driver_guid?: string;
  display_name?: string;
  enabled?: number;
  stream_embed_url?: string;
  stream_status_url?: string;
}

interface TrackMapMeta {
  width: number;
  height: number;
  x_offset: number;
  z_offset: number;
  scale_factor: number;
  margin: number;
}

const server = useServerStore();
const toast = useToastStore();
const confirm = useConfirmStore();
const content = useContentStore();

const details = ref<Record<number, StatusPayload>>({});
const consoleOpen = ref<Record<number, boolean>>({});
const busy = ref<Record<number, boolean>>({});
const loading = ref(true);
const instanceStreamHealth = ref<Record<number, StreamHealth>>({});
const driverStreamHealth = ref<Record<number, Record<string, StreamHealth>>>({});
const driverStreams = ref<DriverStream[]>([]);
const streamViewer = ref<{ title: string; url: string; health?: StreamHealth } | null>(null);
const trackMapMeta = ref<Record<number, TrackMapMeta | null>>({});

async function fetchDetail(id: number) {
  try {
    const payload = await api.get<StatusPayload>(`/api/server/status?instance=${id}`);
    details.value[id] = payload;
    // Seed the live roster; SSE "drivers" events keep it fresh after this.
    const inst = server.instances[id];
    if (inst) {
      inst.drivers = payload.drivers ?? [];
      inst.positions = payload.positions ?? [];
    }
    await fetchTrackMapMeta(id, payload);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

async function fetchTrackMapMeta(id: number, payload?: StatusPayload) {
  const detail = payload ?? details.value[id];
  const event = detail?.current_event;
  if (!event?.track_key) {
    trackMapMeta.value[id] = null;
    return;
  }
  const config = event.track_config ? `/${encodeURIComponent(event.track_config)}` : "";
  try {
    trackMapMeta.value[id] = await api.get<TrackMapMeta>(`/api/track/mapmeta/${encodeURIComponent(event.track_key)}${config}`);
  } catch {
    trackMapMeta.value[id] = null;
  }
}

function lapTime(ms: number): string {
  if (!ms) return "—";
  const m = Math.floor(ms / 60000);
  const s = Math.floor((ms % 60000) / 1000);
  const t = ms % 1000;
  return `${m}:${String(s).padStart(2, "0")}.${String(t).padStart(3, "0")}`;
}

function kick(id: number, carId: number, name: string) {
  void confirm
    .ask({
      title: "Kick driver",
      message: `Kick ${name || "car " + carId} from the server?`,
      confirmLabel: "Kick",
      tone: "danger",
    })
    .then((ok) => {
      if (!ok) return;
      void raceAction(id, () => api.post(`/api/server/kick?instance=${id}`, { car_id: carId }), "Driver kicked.");
    });
}

async function refreshAll() {
  await Promise.all(server.instanceList.map((i) => fetchDetail(i.id)));
  await Promise.all(server.instanceList.map((i) => fetchStreamStatuses(i.id)));
}

async function loadDriverStreams() {
  try {
    const res = await api.get<{ streams: DriverStream[] }>("/api/driver-streams");
    driverStreams.value = res.streams ?? [];
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

async function fetchStreamStatuses(id: number) {
  const inst = server.instances[id];
  if (!inst) return;
  if (inst.stream_enabled === 1 && inst.stream_embed_url) {
    try {
      instanceStreamHealth.value[id] = await api.get<StreamHealth>(`/api/instances/${id}/stream/status`);
    } catch {
      instanceStreamHealth.value[id] = { status: "offline" };
    }
  } else {
    instanceStreamHealth.value[id] = { status: "not_configured" };
  }

  if (inst.drivers.length) {
    try {
      const res = await api.get<{ statuses: Record<string, StreamHealth> }>(`/api/instances/${id}/driver-streams/status`);
      driverStreamHealth.value[id] = res.statuses ?? {};
    } catch {
      driverStreamHealth.value[id] = {};
    }
  }
}

function streamStatusLabel(health?: StreamHealth, configured = true): string {
  if (!configured) return "Not configured";
  if (!health) return "Unknown";
  switch (health?.status) {
    case "live":
      return "Live";
    case "offline":
      return "Offline";
    case "unknown":
      return "Unknown";
    case "not_configured":
    default:
      return "Not configured";
  }
}

function streamStatusClass(health?: StreamHealth, configured = true): string {
  if (!configured || health?.status === "not_configured") return "border-line bg-surface-2 text-muted";
  if (health?.status === "live") return "border-ok/45 bg-ok-glow text-ok";
  if (health?.status === "offline") return "border-danger/45 bg-danger-glow text-danger";
  return "border-line bg-surface-2 text-muted";
}

function driverStreamFor(guid: string): DriverStream | undefined {
  return driverStreams.value.find((s) => s.enabled !== 0 && s.driver_guid === guid && !!s.stream_embed_url);
}

function openDriverStream(instanceId: number, driver: import("@/stores/server").DriverState) {
  const stream = driverStreamFor(driver.guid);
  if (!stream?.stream_embed_url) return;
  streamViewer.value = {
    title: stream.display_name || driver.name || `Car ${driver.car_id}`,
    url: stream.stream_embed_url,
    health: driverStreamHealth.value[instanceId]?.[driver.guid],
  };
}

function openStream(title: string, url?: string | null, health?: StreamHealth) {
  if (!url) return;
  streamViewer.value = { title, url, health };
}

async function toggle(id: number, running: boolean) {
  if (running) {
    const inst = server.instanceList.find((i) => i.id === id);
    const ok = await confirm.ask({
      title: "Stop server",
      message: `Stop ${inst?.name ?? "this instance"}?`,
      detail: "Connected players are disconnected.",
      confirmLabel: "Stop server",
      tone: "danger",
    });
    if (!ok) return;
  }
  busy.value[id] = true;
  try {
    await (running ? server.stop(id) : server.start(id));
    await fetchDetail(id);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value[id] = false;
  }
}

async function stopRepeat(id: number) {
  const inst = server.instanceList.find((i) => i.id === id);
  const ok = await confirm.ask({
    title: "Stop repeat mode",
    message: `Return ${inst?.name ?? "this instance"} to manual queue?`,
    detail: "It stops re-running the same event. Previously queued events are preserved.",
    confirmLabel: "Switch to manual",
  });
  if (!ok) return;
  busy.value[id] = true;
  try {
    await server.setRunMode(id, "manual_queue");
    toast.success("Switched to manual queue.");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value[id] = false;
  }
}

async function skip(id: number) {
  const ok = await confirm.ask({
    title: "Skip current event",
    message: "Skip the current event and advance the queue?",
    detail: "Everyone on the server is kicked when the event rotates.",
    confirmLabel: "Skip event",
    tone: "danger",
  });
  if (!ok) return;
  busy.value[id] = true;
  try {
    await api.post(`/api/queue/skipevent?instance=${id}`);
    await fetchDetail(id);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value[id] = false;
  }
}

// --- Race control (live UDP commands; only meaningful while running) ---
const raceOpen = ref<Record<number, boolean>>({});
const broadcastMsg = ref<Record<number, string>>({});
const adminCmd = ref<Record<number, string>>({});

async function raceAction(id: number, fn: () => Promise<unknown>, ok: string) {
  busy.value[id] = true;
  try {
    await fn();
    toast.success(ok);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value[id] = false;
  }
}

function broadcast(id: number) {
  const message = (broadcastMsg.value[id] ?? "").trim();
  if (!message) return;
  void raceAction(id, () => api.post(`/api/server/broadcast?instance=${id}`, { message }), "Message broadcast.").then(
    () => (broadcastMsg.value[id] = ""),
  );
}

function runAdmin(id: number) {
  const command = (adminCmd.value[id] ?? "").trim();
  if (!command) return;
  void raceAction(id, () => api.post(`/api/server/admin-command?instance=${id}`, { command }), "Command sent.").then(
    () => (adminCmd.value[id] = ""),
  );
}

async function nextSession(id: number) {
  const ok = await confirm.ask({
    title: "Advance session",
    message: "Skip to the next session now?",
    detail: "The current session ends immediately for everyone.",
    confirmLabel: "Next session",
  });
  if (ok) void raceAction(id, () => api.post(`/api/server/next-session?instance=${id}`), "Advancing to next session.");
}

async function restartSession(id: number) {
  const ok = await confirm.ask({
    title: "Restart session",
    message: "Restart the current session?",
    detail: "Everyone returns to the start of the current session.",
    confirmLabel: "Restart session",
  });
  if (ok) void raceAction(id, () => api.post(`/api/server/restart-session?instance=${id}`), "Restarting session.");
}

// --- Grid / weather editor for the running event ---
const gridOpen = ref(false);
const gridInstanceId = ref<number | null>(null);
const gridEventId = ref<number | null>(null);
const gridClassId = ref<number | null>(null);
const gridForm = ref<UserClass | null>(null);
const gridWeatherKey = ref("");
const gridRestart = ref(false);
const gridSaving = ref(false);

async function openGrid(id: number) {
  const d = details.value[id];
  if (!d?.current_event?.id || !d.current_event.class_id) {
    toast.error("No current event to edit.");
    return;
  }
  gridInstanceId.value = id;
  gridEventId.value = d.current_event.id;
  gridClassId.value = d.current_event.class_id;
  gridWeatherKey.value = d.current_event.weather_key ?? "";
  gridRestart.value = false;
  try {
    void content.load();
    const { data } = await api.get<{ data: UserClass }>(`/api/class/${d.current_event.class_id}`);
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
  if (gridInstanceId.value === null || gridEventId.value === null || !gridForm.value) return;
  gridSaving.value = true;
  try {
    const entries = (gridForm.value.entries ?? [])
      .filter((e) => e.cache_car_key)
      .map((e) => ({ cache_car_key: e.cache_car_key, skin_key: e.skin_key ?? "", count: e.count ?? 1 }));
    const res = await api.post<{ restarted: boolean; restart_required: boolean }>(
      `/api/server/current-event?instance=${gridInstanceId.value}`,
      {
        event_id: gridEventId.value,
        class_id: gridClassId.value,
        weather_key: gridWeatherKey.value || undefined,
        class_entries: entries,
        restart_now: gridRestart.value,
      },
    );
    toast.success(res.restarted ? "Applied and restarted." : "Saved — restart the event to apply on track.");
    gridOpen.value = false;
    await fetchDetail(gridInstanceId.value);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    gridSaving.value = false;
  }
}

// Console auto-refresh while any panel is open
let consoleTimer: ReturnType<typeof setInterval> | null = null;
watch(
  consoleOpen,
  (open) => {
    const anyOpen = Object.values(open).some(Boolean);
    if (anyOpen && !consoleTimer) {
      consoleTimer = setInterval(() => {
        for (const [id, isOpen] of Object.entries(consoleOpen.value)) {
          if (isOpen) void fetchDetail(Number(id));
        }
      }, 5000);
    } else if (!anyOpen && consoleTimer) {
      clearInterval(consoleTimer);
      consoleTimer = null;
    }
  },
  { deep: true },
);

// SSE running-state flips → refetch detail
watch(
  () => server.instanceList.map((i) => `${i.id}:${i.running}`).join(","),
  () => void refreshAll(),
);

onMounted(async () => {
  await server.load();
  await loadDriverStreams();
  await refreshAll();
  loading.value = false;
});

onBeforeUnmount(() => {
  if (consoleTimer) clearInterval(consoleTimer);
});

function carSummary(detail: StatusPayload): { model: string; count: number }[] {
  const counts = new Map<string, number>();
  for (const car of detail.current_cars ?? []) {
    counts.set(car.cache_car_key, (counts.get(car.cache_car_key) ?? 0) + 1);
  }
  return [...counts.entries()].map(([model, count]) => ({ model, count }));
}

function elapsed(detail: StatusPayload): string {
  const ms = detail.session?.elapsed_ms ?? 0;
  if (ms <= 0) return "—";
  const min = Math.floor(ms / 60000);
  const sec = Math.floor((ms % 60000) / 1000);
  return `${min}:${String(sec).padStart(2, "0")}`;
}

function consoleLines(detail?: StatusPayload): string {
  const text = detail?.text ?? "";
  return text.split("\n").slice(-200).join("\n").trim() || "No output yet.";
}

function mapImageUrl(detail?: StatusPayload): string {
  const event = detail?.current_event;
  if (!event?.track_key) return "";
  const config = event.track_config ? `/${encodeURIComponent(event.track_config)}` : "";
  return `/api/track/map/${encodeURIComponent(event.track_key)}${config}`;
}

function positionFor(inst: import("@/stores/server").InstanceState, carId: number) {
  return inst.positions.find((p) => p.car_id === carId);
}

function mapPoint(pos: import("@/stores/server").CarPositionState, meta: TrackMapMeta) {
  const scale = meta.scale_factor || 1;
  const x = ((pos.x + meta.x_offset) * scale) / meta.width;
  const y = (meta.height - (pos.z + meta.z_offset) * scale) / meta.height;
  return {
    left: `${Math.max(0, Math.min(100, x * 100))}%`,
    top: `${Math.max(0, Math.min(100, y * 100))}%`,
  };
}

function speedKmh(pos?: import("@/stores/server").CarPositionState): number {
  if (!pos) return 0;
  const ms = Math.sqrt(pos.velocity_x ** 2 + pos.velocity_y ** 2 + pos.velocity_z ** 2);
  return Math.round(ms * 3.6);
}
</script>

<template>
  <PageHeader
    title="Dashboard"
    subtitle="Live instance status, current event, session telemetry, grid summary and server console."
    icon="dashboard"
  >
    <template #actions>
      <span
        v-if="details[server.instanceList[0]?.id ?? 0]?.public_ip"
        class="inline-flex min-h-8 items-center gap-2 rounded-md border border-line bg-surface px-2.5 font-mono text-xs text-dim"
      >
        <Icon name="activity" :size="15" />
        {{ details[server.instanceList[0]!.id].public_ip }}
      </span>
    </template>
  </PageHeader>

  <div v-if="loading" class="space-y-4">
    <Skeleton v-for="n in 2" :key="n" class="h-60" />
  </div>

  <EmptyState
    v-if="!loading && !server.instanceList.length"
    icon="instances"
    title="No server instances yet"
    message="Create a server instance to assign ports and run events on it."
  >
    <RouterLink to="/settings/instances">
      <Button>
        <Icon name="plus" :size="15" />
        Add an instance
      </Button>
    </RouterLink>
  </EmptyState>

  <div class="space-y-4">
    <Card v-for="inst in server.instanceList" :key="inst.id">
      <template #header>
        <span
          class="size-2 rounded-full"
          :class="inst.running ? 'bg-ok shadow-[0_0_14px_rgba(79,216,132,0.55)]' : 'bg-dim'"
        />
        <h2 class="text-sm font-bold">{{ inst.name }}</h2>
        <span class="font-mono text-xs text-dim">:{{ inst.tcp_port }}</span>
        <span v-if="inst.running" class="rounded-full bg-surface-2 px-2 py-0.5 text-xs text-muted">
          {{ inst.players }} player{{ inst.players === 1 ? "" : "s" }}
        </span>
        <span
          v-if="inst.run_mode === 'repeat_event'"
          class="inline-flex items-center gap-1 rounded-full border border-accent/40 bg-accent-dim px-2 py-0.5 text-xs text-accent"
          :title="inst.repeat_event?.track ? `Repeating ${inst.repeat_event.track}` : 'Repeat mode'"
        >
          <Icon name="repeat" :size="12" />
          Repeat
        </span>
      </template>
      <template #actions>
        <Button
          v-if="inst.run_mode === 'repeat_event'"
          variant="ghost"
          size="sm"
          :disabled="busy[inst.id]"
          @click="stopRepeat(inst.id)"
        >
          <Icon name="queue" :size="15" />
          Stop repeat
        </Button>
        <Button
          v-if="inst.running && (details[inst.id]?.current_event?.id ?? 0) > 0"
          variant="ghost"
          size="sm"
          :disabled="busy[inst.id]"
          @click="skip(inst.id)"
        >
          <Icon name="skip" :size="15" />
          Skip
        </Button>
        <Button
          :variant="inst.running ? 'danger' : 'success'"
          size="sm"
          :disabled="busy[inst.id]"
          @click="toggle(inst.id, inst.running)"
        >
          <Icon :name="inst.running ? 'stop' : 'power'" :size="15" />
          {{ busy[inst.id] ? "Working" : inst.running ? "Stop" : "Start" }}
        </Button>
      </template>

      <div class="grid gap-4 lg:grid-cols-3">
        <!-- Current event -->
        <div>
          <h3 class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">Current event</h3>
          <template v-if="details[inst.id]?.current_event?.id">
            <img
              v-if="details[inst.id].current_event.track_key"
              :src="`/api/track/preview/${details[inst.id].current_event.track_key}${
                details[inst.id].current_event.track_config ? '/' + details[inst.id].current_event.track_config : ''
              }`"
              alt=""
              class="mb-2 aspect-video w-full rounded-sm border border-line object-cover"
            />
            <div class="text-sm font-medium">{{ details[inst.id].current_event.track }}</div>
            <div class="mb-2 text-xs text-dim">{{ details[inst.id].current_event.category }}</div>
            <div class="flex flex-wrap gap-1.5 text-xs">
              <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ details[inst.id].current_event.class }}</span>
              <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ details[inst.id].current_event.session }}</span>
              <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ details[inst.id].current_event.time }}</span>
              <span v-if="details[inst.id].current_event.weather" class="rounded-full border border-line bg-surface-2 px-2 py-0.5">
                {{ details[inst.id].current_event.weather }}
              </span>
            </div>
            <Button variant="dark" size="sm" class="mt-3" @click="openGrid(inst.id)">
              <Icon name="edit" :size="14" />
              Edit grid & weather
            </Button>
          </template>
          <p v-else class="text-sm text-dim">
            Nothing loaded — queue an event and start the server.
            <RouterLink to="/queue" class="text-accent hover:underline">Open queue →</RouterLink>
          </p>
        </div>

        <!-- Live session -->
        <div>
          <h3 class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">Session</h3>
          <dl v-if="inst.running && inst.session" class="grid grid-cols-2 gap-x-4 gap-y-1.5 text-sm">
            <dt class="text-muted">Type</dt>
            <dd>
              {{ details[inst.id]?.session?.type ?? "—" }}
              <span class="text-xs text-dim">
                ({{ (inst.session.current_session_index ?? 0) + 1 }}/{{ inst.session.session_count }})
              </span>
            </dd>
            <dt class="text-muted">Length</dt>
            <dd>{{ inst.session.laps ? `${inst.session.laps} laps` : `${inst.session.time} min` }}</dd>
            <dt class="text-muted">Elapsed</dt>
            <dd class="font-mono">{{ details[inst.id] ? elapsed(details[inst.id]) : "—" }}</dd>
            <dt class="text-muted">Air / Road</dt>
            <dd>{{ inst.session.ambient_temp }}° / {{ inst.session.road_temp }}°</dd>
            <dt class="text-muted">Weather</dt>
            <dd class="min-w-0 truncate font-mono text-xs">{{ inst.session.weather_graphics || "—" }}</dd>
          </dl>
          <p v-else class="text-sm text-dim">Server stopped.</p>
        </div>

        <!-- Grid -->
        <div>
          <h3 class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">Grid</h3>
          <ul v-if="details[inst.id] && carSummary(details[inst.id]).length" class="space-y-1 text-sm">
            <li v-for="c in carSummary(details[inst.id])" :key="c.model" class="flex justify-between gap-2">
              <span class="min-w-0 truncate font-mono text-xs">{{ c.model }}</span>
              <span class="text-muted">×{{ c.count }}</span>
            </li>
          </ul>
          <p v-else class="text-sm text-dim">No entry list rendered yet.</p>
        </div>
      </div>

      <!-- Track map -->
      <div v-if="details[inst.id]?.current_event?.track_key" class="mt-4 border-t border-line pt-3">
        <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
          <h3 class="text-xs font-semibold tracking-wide text-muted uppercase">Track map</h3>
          <span class="font-mono text-xs text-dim">
            {{ inst.positions.length }} live position{{ inst.positions.length === 1 ? "" : "s" }}
          </span>
        </div>
        <div
          v-if="trackMapMeta[inst.id]"
          class="relative overflow-hidden rounded-md border border-line bg-bg"
          :style="{ aspectRatio: `${trackMapMeta[inst.id]?.width ?? 16} / ${trackMapMeta[inst.id]?.height ?? 9}` }"
        >
          <img
            :src="mapImageUrl(details[inst.id])"
            alt=""
            class="absolute inset-0 size-full object-fill opacity-80"
          />
          <template v-for="d in inst.drivers" :key="d.car_id">
            <button
              v-if="positionFor(inst, d.car_id)"
              type="button"
              class="absolute grid size-7 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full border border-bg bg-accent text-[10px] font-black text-bg shadow-[0_0_18px_rgba(91,141,239,0.65)] transition-transform hover:z-10 hover:scale-110"
              :style="mapPoint(positionFor(inst, d.car_id)!, trackMapMeta[inst.id]!)"
              :title="`${d.name || 'car ' + d.car_id} · ${speedKmh(positionFor(inst, d.car_id))} km/h · gear ${positionFor(inst, d.car_id)?.gear ?? 0}`"
            >
              {{ (d.name || String(d.car_id)).slice(0, 1).toUpperCase() }}
            </button>
          </template>
        </div>
        <p v-else class="rounded-md border border-line bg-surface px-3 py-3 text-sm text-dim">
          Track map metadata is not available for this layout.
        </p>
      </div>

      <!-- Spectator stream -->
      <div v-if="inst.stream_enabled === 1 && inst.stream_embed_url" class="mt-4 border-t border-line pt-3">
        <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
          <h3 class="text-xs font-semibold tracking-wide text-muted uppercase">Spectator stream</h3>
          <div class="flex items-center gap-2">
            <span
              class="inline-flex min-h-7 items-center rounded-md border px-2 text-xs font-semibold"
              :class="streamStatusClass(instanceStreamHealth[inst.id], true)"
            >
              {{ streamStatusLabel(instanceStreamHealth[inst.id], true) }}
            </span>
            <Button variant="ghost" size="sm" @click="openStream(`${inst.name} spectator`, inst.stream_embed_url, instanceStreamHealth[inst.id])">
              <Icon name="activity" :size="14" />
              Watch
            </Button>
          </div>
        </div>
        <iframe
          :src="inst.stream_embed_url"
          :title="`${inst.name} spectator stream`"
          class="aspect-video w-full rounded-md border border-line bg-bg"
          allow="autoplay; fullscreen; picture-in-picture"
          sandbox="allow-scripts allow-same-origin allow-forms allow-presentation"
        />
      </div>

      <!-- Live timing -->
      <div v-if="inst.running && inst.drivers.length" class="mt-4 border-t border-line pt-3">
        <h3 class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">
          Live timing — {{ inst.drivers.length }} connected
        </h3>
        <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-line text-left text-xs tracking-wide text-muted uppercase">
              <th class="py-1.5 pr-2 font-semibold">Driver</th>
              <th class="py-1.5 pr-2 font-semibold max-sm:hidden">Car</th>
              <th class="py-1.5 pr-2 text-right font-semibold">Laps</th>
              <th class="py-1.5 pr-2 text-right font-semibold">Last</th>
              <th class="py-1.5 pr-2 text-right font-semibold">Best</th>
              <th class="py-1.5 font-semibold"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="d in inst.drivers" :key="d.car_id" class="border-b border-line/60">
              <td class="py-1.5 pr-2 font-medium">{{ d.name || "car " + d.car_id }}</td>
              <td class="py-1.5 pr-2 font-mono text-xs text-muted max-sm:hidden">{{ d.car }}</td>
              <td class="py-1.5 pr-2 text-right">{{ d.laps }}</td>
              <td class="py-1.5 pr-2 text-right font-mono text-xs">{{ lapTime(d.last_lap_ms) }}</td>
              <td class="py-1.5 pr-2 text-right font-mono text-xs text-ok">{{ lapTime(d.best_lap_ms) }}</td>
              <td class="py-1.5 text-right">
                <div class="flex justify-end gap-1">
                  <Button
                    v-if="driverStreamFor(d.guid)"
                    variant="dark"
                    size="sm"
                    aria-label="Watch driver stream"
                    @click="openDriverStream(inst.id, d)"
                  >
                    <Icon name="activity" :size="14" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    :disabled="busy[inst.id]"
                    aria-label="Kick driver"
                    @click="kick(inst.id, d.car_id, d.name)"
                  >
                    <Icon name="x" :size="14" />
                  </Button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
        </div>
      </div>

      <!-- Race control -->
      <div v-if="inst.running" class="mt-4 border-t border-line pt-3">
        <button
          type="button"
          class="inline-flex min-h-8 cursor-pointer items-center gap-2 rounded-md px-2 text-xs font-semibold text-muted transition-colors hover:bg-surface-2 hover:text-text"
          @click="raceOpen[inst.id] = !raceOpen[inst.id]"
        >
          <Icon name="activity" :size="15" />
          {{ raceOpen[inst.id] ? "Hide race control" : "Race control" }}
        </button>

        <div v-if="raceOpen[inst.id]" class="mt-3 space-y-3">
          <div class="flex flex-wrap gap-2">
            <Button variant="dark" size="sm" :disabled="busy[inst.id]" @click="nextSession(inst.id)">
              <Icon name="skip" :size="14" />
              Next session
            </Button>
            <Button variant="dark" size="sm" :disabled="busy[inst.id]" @click="restartSession(inst.id)">
              <Icon name="repeat" :size="14" />
              Restart session
            </Button>
          </div>

          <form class="flex gap-2" @submit.prevent="broadcast(inst.id)">
            <Input v-model="broadcastMsg[inst.id]" placeholder="Broadcast a message to all drivers…" class="flex-1" />
            <Button type="submit" size="sm" :disabled="busy[inst.id] || !(broadcastMsg[inst.id] ?? '').trim()">Send</Button>
          </form>

          <form class="flex gap-2" @submit.prevent="runAdmin(inst.id)">
            <Input v-model="adminCmd[inst.id]" placeholder="Admin command, e.g. /kick name or /ballast 0 50" class="flex-1 font-mono" />
            <Button type="submit" variant="dark" size="sm" :disabled="busy[inst.id] || !(adminCmd[inst.id] ?? '').trim()">Run</Button>
          </form>
          <p class="text-xs text-dim">
            Admin commands run through the server's ACSP plugin — the same ones the in-game admin uses.
          </p>
        </div>
      </div>

      <!-- Console -->
      <div class="mt-4 border-t border-line pt-3">
        <button
          type="button"
          class="inline-flex min-h-8 cursor-pointer items-center gap-2 rounded-md px-2 text-xs font-semibold text-muted transition-colors hover:bg-surface-2 hover:text-text"
          @click="consoleOpen[inst.id] = !consoleOpen[inst.id]"
        >
          <Icon name="terminal" :size="15" />
          {{ consoleOpen[inst.id] ? "Hide console" : "Show console" }}
        </button>
        <pre
          v-if="consoleOpen[inst.id]"
          class="mt-2 max-h-72 overflow-y-auto rounded-md border border-line bg-bg p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap text-muted"
          >{{ consoleLines(details[inst.id]) }}</pre>
      </div>
    </Card>
  </div>

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

  <Sheet :open="!!streamViewer" :title="streamViewer?.title ?? 'Stream'" @close="streamViewer = null">
    <template v-if="streamViewer">
      <div class="mb-2 flex justify-end">
        <span
          class="inline-flex min-h-7 items-center rounded-md border px-2 text-xs font-semibold"
          :class="streamStatusClass(streamViewer.health, true)"
        >
          {{ streamStatusLabel(streamViewer.health, true) }}
        </span>
      </div>
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
      <a v-if="streamViewer" :href="streamViewer.url" target="_blank" rel="noreferrer">
        <Button variant="dark">Open stream</Button>
      </a>
    </template>
  </Sheet>
</template>
