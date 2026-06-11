<script setup lang="ts">
// Live multi-instance dashboard. Status/players/session arrive over SSE;
// the detail payload (current event, cars, console) is fetched per instance
// and refreshed when SSE reports a change. Console refreshes on a short
// timer only while its panel is open.
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { api, ApiError } from "@/lib/api";
import { useServerStore } from "@/stores/server";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";

interface CurrentEvent {
  id: number;
  category: string;
  track: string;
  track_key: string;
  track_config: string;
  difficulty: string;
  session: string;
  class: string;
  time: string;
  weather: string;
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
}

const server = useServerStore();

const details = ref<Record<number, StatusPayload>>({});
const consoleOpen = ref<Record<number, boolean>>({});
const busy = ref<Record<number, boolean>>({});
const error = ref("");

async function fetchDetail(id: number) {
  try {
    details.value[id] = await api.get<StatusPayload>(`/api/server/status?instance=${id}`);
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  }
}

async function refreshAll() {
  await Promise.all(server.instanceList.map((i) => fetchDetail(i.id)));
}

async function toggle(id: number, running: boolean) {
  busy.value[id] = true;
  error.value = "";
  try {
    await (running ? server.stop(id) : server.start(id));
    await fetchDetail(id);
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  } finally {
    busy.value[id] = false;
  }
}

async function skip(id: number) {
  if (!window.confirm("Skip the current event and advance the queue? Players get kicked.")) return;
  busy.value[id] = true;
  try {
    await api.post(`/api/queue/skipevent?instance=${id}`);
    await fetchDetail(id);
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  } finally {
    busy.value[id] = false;
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
  await refreshAll();
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
</script>

<template>
  <div class="mb-5 flex items-center gap-3">
    <h1 class="text-xl font-bold">Dashboard</h1>
    <span v-if="details[server.instanceList[0]?.id ?? 0]?.public_ip" class="ml-auto font-mono text-xs text-dim">
      public IP {{ details[server.instanceList[0]!.id].public_ip }}
    </span>
  </div>

  <p v-if="error" class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger">
    {{ error }}
  </p>

  <div class="space-y-4">
    <Card v-for="inst in server.instanceList" :key="inst.id">
      <template #header>
        <span class="size-2 rounded-full" :class="inst.running ? 'bg-ok' : 'bg-dim'" />
        <h2 class="text-sm font-semibold">{{ inst.name }}</h2>
        <span class="font-mono text-xs text-dim">:{{ inst.tcp_port }}</span>
        <span v-if="inst.running" class="rounded-full bg-surface-2 px-2 py-0.5 text-xs text-muted">
          {{ inst.players }} player{{ inst.players === 1 ? "" : "s" }}
        </span>
      </template>
      <template #actions>
        <Button
          v-if="inst.running && (details[inst.id]?.current_event?.id ?? 0) > 0"
          variant="ghost"
          size="sm"
          :disabled="busy[inst.id]"
          @click="skip(inst.id)"
        >
          Skip event
        </Button>
        <Button
          :variant="inst.running ? 'danger' : 'success'"
          size="sm"
          :disabled="busy[inst.id]"
          @click="toggle(inst.id, inst.running)"
        >
          {{ busy[inst.id] ? "…" : inst.running ? "■ Stop" : "▶ Start" }}
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
              <span class="rounded-full bg-surface-2 px-2 py-0.5">{{ details[inst.id].current_event.class }}</span>
              <span class="rounded-full bg-surface-2 px-2 py-0.5">{{ details[inst.id].current_event.session }}</span>
              <span class="rounded-full bg-surface-2 px-2 py-0.5">{{ details[inst.id].current_event.time }}</span>
              <span v-if="details[inst.id].current_event.weather" class="rounded-full bg-surface-2 px-2 py-0.5">
                {{ details[inst.id].current_event.weather }}
              </span>
            </div>
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
            <dd class="truncate font-mono text-xs">{{ inst.session.weather_graphics || "—" }}</dd>
          </dl>
          <p v-else class="text-sm text-dim">Server stopped.</p>
        </div>

        <!-- Grid -->
        <div>
          <h3 class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">Grid</h3>
          <ul v-if="details[inst.id] && carSummary(details[inst.id]).length" class="space-y-1 text-sm">
            <li v-for="c in carSummary(details[inst.id])" :key="c.model" class="flex justify-between gap-2">
              <span class="truncate font-mono text-xs">{{ c.model }}</span>
              <span class="text-muted">×{{ c.count }}</span>
            </li>
          </ul>
          <p v-else class="text-sm text-dim">No entry list rendered yet.</p>
        </div>
      </div>

      <!-- Console -->
      <div class="mt-4 border-t border-line pt-3">
        <button
          type="button"
          class="text-xs text-muted hover:text-text"
          @click="consoleOpen[inst.id] = !consoleOpen[inst.id]"
        >
          {{ consoleOpen[inst.id] ? "▾ Hide console" : "▸ Show console" }}
        </button>
        <pre
          v-if="consoleOpen[inst.id]"
          class="mt-2 max-h-72 overflow-y-auto rounded-md border border-line bg-bg p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap text-muted"
          >{{ consoleLines(details[inst.id]) }}</pre>
      </div>
    </Card>
  </div>
</template>
