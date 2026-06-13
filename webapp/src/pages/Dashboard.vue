<script setup lang="ts">
// Lightweight multi-instance overview. One compact card per instance with
// status, the current (or queued) event and the essential start/stop/skip
// controls. The full race-control surface — map, live timing, grid editor,
// telemetry, console, streams — lives on the per-instance detail page.
import { onMounted, ref, watch } from "vue";
import { api, ApiError } from "@/lib/api";
import { useServerStore } from "@/stores/server";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import Skeleton from "@/components/ui/Skeleton.vue";

interface CurrentEvent {
  id: number;
  name: string;
  category: string;
  track: string;
  track_key: string;
  track_config: string;
  session: string;
  class: string;
  time: string;
  weather: string;
}

interface StatusPayload {
  instance_id: number;
  public_ip: string;
  current_event: CurrentEvent;
  session: {
    type: string;
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
const toast = useToastStore();
const confirm = useConfirmStore();

const details = ref<Record<number, StatusPayload>>({});
const busy = ref<Record<number, boolean>>({});
const loading = ref(true);

async function fetchDetail(id: number) {
  try {
    details.value[id] = await api.get<StatusPayload>(`/api/server/status?instance=${id}`);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

async function refreshAll() {
  await Promise.all(server.instanceList.map((i) => fetchDetail(i.id)));
}

function eventTitle(id: number): string {
  const ev = details.value[id]?.current_event;
  if (!ev?.id) return "";
  return ev.name || ev.track;
}

function publicIp(): string {
  const first = server.instanceList[0];
  return first ? (details.value[first.id]?.public_ip ?? "") : "";
}

// Live session values come from the SSE-fed store, not the REST detail payload.
function elapsed(id: number): string {
  const ms = server.instances[id]?.session?.elapsed_ms ?? 0;
  if (ms <= 0) return "—";
  const min = Math.floor(ms / 60000);
  const sec = Math.floor((ms % 60000) / 1000);
  return `${min}:${String(sec).padStart(2, "0")}`;
}

// AC numeric session type → display label (mirrors the backend mapping).
function sessionTypeLabel(t: number): string {
  return ["Booking", "Practice", "Qualify", "Race"][t] ?? "—";
}

function previewUrl(id: number): string {
  const ev = details.value[id]?.current_event;
  if (!ev?.track_key) return "";
  return `/api/track/preview/${ev.track_key}${ev.track_config ? "/" + ev.track_config : ""}`;
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

// SSE running-state flips → refetch the affected detail payload
watch(
  () => server.instanceList.map((i) => `${i.id}:${i.running}`).join(","),
  () => void refreshAll(),
);

onMounted(async () => {
  await server.load();
  await refreshAll();
  loading.value = false;
});
</script>

<template>
  <PageHeader
    title="Dashboard"
    subtitle="Live status across every server instance. Open one for the map, live timing and full race control."
    icon="dashboard"
  >
    <template #actions>
      <span
        v-if="publicIp()"
        class="inline-flex min-h-8 items-center gap-2 rounded-md border border-line bg-surface px-2.5 font-mono text-xs text-dim"
      >
        <Icon name="activity" :size="15" />
        {{ publicIp() }}
      </span>
    </template>
  </PageHeader>

  <div v-if="loading" class="space-y-3">
    <Skeleton v-for="n in 3" :key="n" class="h-24" />
  </div>

  <EmptyState
    v-else-if="!server.instanceList.length"
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

  <div v-else class="space-y-3">
    <Card v-for="inst in server.instanceList" :key="inst.id" class="transition-colors hover:border-line-hi">
      <template #header>
        <span
          class="size-2 rounded-full"
          :class="inst.running ? 'bg-ok shadow-[0_0_14px_rgba(79,216,132,0.55)]' : 'bg-dim'"
        />
        <RouterLink :to="`/server/${inst.id}`" class="text-sm font-bold hover:text-accent">{{ inst.name }}</RouterLink>
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
        <RouterLink :to="`/server/${inst.id}`">
          <Button variant="dark" size="sm">
            Details
            <Icon name="arrowUp" :size="14" class="rotate-90" />
          </Button>
        </RouterLink>
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

      <!-- One-line current-event summary -->
      <RouterLink
        v-if="details[inst.id]?.current_event?.id"
        :to="`/server/${inst.id}`"
        class="flex items-center gap-3 rounded-md p-1 transition-colors hover:bg-surface-2/50"
      >
        <img
          v-if="previewUrl(inst.id)"
          :src="previewUrl(inst.id)"
          alt=""
          class="h-14 w-24 shrink-0 rounded-sm border border-line object-cover"
        />
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-semibold">{{ eventTitle(inst.id) }}</div>
          <div class="flex flex-wrap gap-x-2 gap-y-0.5 text-xs text-dim">
            <span>{{ details[inst.id].current_event.class }}</span>
            <span>· {{ details[inst.id].current_event.session }}</span>
            <span>· {{ details[inst.id].current_event.time }}</span>
            <span v-if="details[inst.id].current_event.weather">· {{ details[inst.id].current_event.weather }}</span>
          </div>
        </div>
        <!-- Live session stats (running only) -->
        <dl
          v-if="inst.running && inst.session"
          class="hidden shrink-0 grid-cols-2 gap-x-4 gap-y-0.5 text-right text-xs sm:grid"
        >
          <dt class="text-dim">Session</dt>
          <dd class="font-mono">
            {{ sessionTypeLabel(inst.session.type) }}
            <span class="text-dim">{{ (inst.session.current_session_index ?? 0) + 1 }}/{{ inst.session.session_count }}</span>
          </dd>
          <dt class="text-dim">Elapsed</dt>
          <dd class="font-mono">{{ elapsed(inst.id) }}</dd>
          <dt class="text-dim">Air / Road</dt>
          <dd class="font-mono">{{ inst.session.ambient_temp }}° / {{ inst.session.road_temp }}°</dd>
        </dl>
      </RouterLink>

      <p v-else class="text-sm text-dim">
        Server stopped — nothing loaded.
        <RouterLink to="/setup" class="text-accent hover:underline">Set up a race</RouterLink>
        or
        <RouterLink to="/queue" class="text-accent hover:underline">open the run plan →</RouterLink>
      </p>
    </Card>
  </div>
</template>
