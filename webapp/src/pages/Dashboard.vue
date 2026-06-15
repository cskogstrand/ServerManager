<script setup lang="ts">
// Lightweight multi-instance overview. One compact card per instance with
// status, the current (or queued) event and the essential start/stop/skip
// controls. The full race-control surface — map, live timing, grid editor,
// telemetry, console, streams — lives on the per-instance detail page.
import { computed, onMounted, ref, watch } from "vue";
import { api, ApiError } from "@/lib/api";
import { useServerStore, type InstanceState } from "@/stores/server";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import { useSetupSummary } from "@/lib/useSetupSummary";
import { useDriverStreams, type StreamChannel } from "@/lib/useDriverStreams";
import {
  normalizeRaceSetup,
  raceSetupBody,
  raceSetupValid,
  type RaceSetupDraft,
} from "@/lib/useRaceSetupDraft";
import Card from "@/components/ui/Card.vue";
import StreamTheater from "@/components/StreamTheater.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import Sheet from "@/components/ui/Sheet.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import Skeleton from "@/components/ui/Skeleton.vue";
import RaceSetupEditor from "@/components/RaceSetupEditor.vue";

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
const { summary, reload: reloadSummary } = useSetupSummary();

const details = ref<Record<number, StatusPayload>>({});
const busy = ref<Record<number, boolean>>({});
const loading = ref(true);

// --- Watchable driver streams (overview-level, no live health poll) ---
const driverStreams = useDriverStreams();
const theaterOpen = ref(false);
const theaterChannels = ref<StreamChannel[]>([]);

// Connected drivers come from the SSE-fed store (App keeps it live app-wide).
function streamChannelsFor(id: number): StreamChannel[] {
  return driverStreams.allChannelsFor(server.instances[id]?.drivers ?? []);
}
function openStreams(id: number) {
  theaterChannels.value = streamChannelsFor(id);
  if (theaterChannels.value.length) theaterOpen.value = true;
}

async function fetchDetail(id: number) {
  try {
    details.value[id] = await api.get<StatusPayload>(`/api/server/status?instance=${id}`);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

async function refreshAll() {
  await Promise.all([...server.instanceList.map((i) => fetchDetail(i.id)), reloadSummary()]);
}

// --- Readiness-driven next actions for idle instances ---
// can_start reflects global setup health (install path, content, presets, config,
// at least one race setup, no port clash). queue_pending is per instance.
const canStart = computed(() => summary.value?.can_start ?? false);
const firstBlocker = computed(() => summary.value?.blocking?.[0]?.message ?? "");

function pendingCount(id: number): number {
  return summary.value?.instances.find((i) => i.id === id)?.queue_pending ?? 0;
}
// An instance can actually start when setup is healthy and it has something to
// run — a manual queue with entries, or a pinned repeat event.
function startable(inst: InstanceState): boolean {
  return canStart.value && (inst.run_mode === "repeat_event" || pendingCount(inst.id) > 0);
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

// --- Edit run setup: open the shared editor on the current event (source edit) ---
const editOpen = ref(false);
const editDraft = ref<RaceSetupDraft | null>(null);
const editGroupId = ref<number | null>(null);
const editEventId = ref<number | null>(null);
const editInstanceId = ref<number | null>(null);
const editSaving = ref(false);

let editBaseline = "";
const markEditClean = () => (editBaseline = editDraft.value ? JSON.stringify(editDraft.value) : "");
useUnsavedGuard(() => editOpen.value && editDraft.value !== null && JSON.stringify(editDraft.value) !== editBaseline);

const openEditRun = (instId: number, eventId: number) =>
  withBusy(instId, async () => {
    const raw = await api.get<Record<string, unknown>>(`/api/event/${eventId}`);
    editDraft.value = normalizeRaceSetup(raw);
    editGroupId.value = raw.EventCategoryId != null ? Number(raw.EventCategoryId) : null;
    editEventId.value = eventId;
    editInstanceId.value = instId;
    markEditClean();
    editOpen.value = true;
  });

async function withBusy(id: number, fn: () => Promise<void>) {
  busy.value[id] = true;
  try {
    await fn();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value[id] = false;
  }
}

async function saveEditRun() {
  const draft = editDraft.value;
  if (!draft || editEventId.value === null || editGroupId.value === null) return;
  if (!raceSetupValid(draft)) {
    toast.error("Track and all four presets are required.");
    return;
  }
  editSaving.value = true;
  try {
    await api.put(`/api/event/${editEventId.value}`, raceSetupBody(draft, editGroupId.value));
    toast.success("Race setup updated — applies when the event next restarts.");
    editOpen.value = false;
    markEditClean();
    if (editInstanceId.value !== null) await fetchDetail(editInstanceId.value);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    editSaving.value = false;
  }
}

// SSE running-state flips → refetch the affected detail payload
watch(
  () => server.instanceList.map((i) => `${i.id}:${i.running}`).join(","),
  () => void refreshAll(),
);

onMounted(async () => {
  await server.load();
  void driverStreams.loadStreams();
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
        <Button
          v-if="streamChannelsFor(inst.id).length"
          variant="ghost"
          size="sm"
          @click="openStreams(inst.id)"
        >
          <Icon name="play" :size="14" />
          Watch
          <span class="font-mono text-dim">{{ streamChannelsFor(inst.id).length }}</span>
        </Button>
        <Button
          v-if="inst.running && (details[inst.id]?.current_event?.id ?? 0) > 0"
          variant="ghost"
          size="sm"
          :disabled="busy[inst.id]"
          @click="openEditRun(inst.id, details[inst.id]!.current_event.id)"
        >
          <Icon name="edit" :size="14" />
          Edit setup
        </Button>
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
        <Button v-if="inst.running" variant="danger" size="sm" :disabled="busy[inst.id]" @click="toggle(inst.id, true)">
          <Icon name="stop" :size="15" />
          {{ busy[inst.id] ? "Working" : "Stop" }}
        </Button>
        <Button
          v-else-if="startable(inst)"
          variant="success"
          size="sm"
          :disabled="busy[inst.id]"
          @click="toggle(inst.id, false)"
        >
          <Icon name="power" :size="15" />
          {{ busy[inst.id] ? "Working" : inst.run_mode === "repeat_event" ? "Start repeat" : "Start" }}
        </Button>
        <RouterLink v-else-if="!canStart" to="/setup">
          <Button variant="dark" size="sm">
            <Icon name="settings" :size="15" />
            Finish setup
          </Button>
        </RouterLink>
        <RouterLink v-else to="/setup">
          <Button variant="dark" size="sm">
            <Icon name="plus" :size="15" />
            Set up a race
          </Button>
        </RouterLink>
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

      <!-- Idle: readiness-driven next action -->
      <div v-else>
        <!-- Setup incomplete: surface the next blocker, route to setup -->
        <div
          v-if="!canStart"
          class="flex items-start gap-2.5 rounded-md border border-warn/40 bg-warn-glow px-3 py-2.5"
        >
          <Icon name="alert" :size="16" class="mt-0.5 shrink-0 text-warn" />
          <div class="min-w-0 text-sm">
            <span class="font-semibold text-warn">Can't start yet.</span>
            <span class="text-muted"> {{ firstBlocker }}</span>
            <RouterLink to="/setup" class="ml-1 font-semibold text-accent hover:underline">Open setup →</RouterLink>
          </div>
        </div>

        <!-- Repeat mode: pinned event ready to roll -->
        <div v-else-if="inst.run_mode === 'repeat_event'" class="flex items-center gap-2 text-sm text-muted">
          <Icon name="repeat" :size="16" class="shrink-0 text-accent" />
          <span>
            Repeats
            <span class="font-medium text-text">{{ inst.repeat_event?.track || "the pinned event" }}</span>
            on every finish. Press Start repeat to begin.
          </span>
        </div>

        <!-- Manual queue with entries ready -->
        <div v-else-if="pendingCount(inst.id) > 0" class="flex items-center gap-2 text-sm text-muted">
          <Icon name="queue" :size="16" class="shrink-0 text-accent" />
          <span>
            <span class="font-medium text-text">{{ pendingCount(inst.id) }}</span>
            race{{ pendingCount(inst.id) === 1 ? "" : "s" }} queued — ready to start.
          </span>
          <RouterLink to="/queue" class="ml-auto text-xs font-semibold text-accent hover:underline">Run plan →</RouterLink>
        </div>

        <!-- Ready but nothing to run: queue something -->
        <div v-else class="flex flex-wrap items-center gap-2 text-sm text-dim">
          <span>Nothing queued for this instance.</span>
          <RouterLink to="/setup">
            <Button variant="dark" size="sm">
              <Icon name="plus" :size="14" />
              Set up a race
            </Button>
          </RouterLink>
          <RouterLink to="/events">
            <Button variant="ghost" size="sm">
              <Icon name="queue" :size="14" />
              Queue a saved setup
            </Button>
          </RouterLink>
        </div>
      </div>
    </Card>
  </div>

  <!-- Edit run setup: shared editor on the current event -->
  <Sheet :open="editOpen" title="Edit run setup" @close="editOpen = false">
    <RaceSetupEditor v-if="editDraft" v-model="editDraft" :instance-id="editInstanceId" />
    <template #footer>
      <span v-if="!raceSetupValid(editDraft)" class="mr-auto self-center text-xs text-muted">
        Track and all four presets are required.
      </span>
      <Button variant="ghost" @click="editOpen = false">Cancel</Button>
      <Button :disabled="editSaving || !raceSetupValid(editDraft)" @click="saveEditRun">
        {{ editSaving ? "Saving…" : "Save setup" }}
      </Button>
    </template>
  </Sheet>

  <StreamTheater :open="theaterOpen" :channels="theaterChannels" @close="theaterOpen = false" />
</template>
