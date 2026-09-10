<script setup lang="ts">
// Per-instance queue: reorder, remove, skip, clear; add events or whole
// categories. Refetches when an instance starts/stops (SSE-driven).
import { computed, onMounted, ref, watch } from "vue";
import { api, ApiError } from "@/lib/api";
import { useQueryParam, numberParam } from "@/lib/useQueryParam";
import { queueState } from "@/lib/queueState";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import { emptyRaceSetup, raceSetupBody, raceSetupValid, type RaceSetupDraft } from "@/lib/useRaceSetupDraft";
import { useServerStore } from "@/stores/server";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import { useAuthStore } from "@/stores/auth";
import type { DropDownList, UserEventList } from "@/types/generated";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Combobox from "@/components/ui/Combobox.vue";
import Icon from "@/components/ui/Icon.vue";
import Modal from "@/components/ui/Modal.vue";
import Sheet from "@/components/ui/Sheet.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import RaceSetupEditor from "@/components/RaceSetupEditor.vue";

interface QueueRow {
  id: number;
  event_id: number;
  instance_id: number;
  name: string | null;
  category: string;
  track: string;
  difficulty: string;
  session: string;
  class: string;
  time: string;
  duration_min: number;
  started_at: number | null;
  finished: number;
}

const server = useServerStore();
const toast = useToastStore();
const confirm = useConfirmStore();
const auth = useAuthStore();

// Selected instance is mirrored to ?instance so refresh/back restores the tab.
const instanceId = useQueryParam<number | null>("instance", null, numberParam());
const rows = ref<QueueRow[]>([]);
const selectionNotice = ref("");
const busy = ref(false);
const error = ref("");
const loading = ref(false);
let loadVersion = 0;

const categories = ref<DropDownList[]>([]);
const allEvents = ref<UserEventList[]>([]);
const addCategory = ref<number | null>(null);
const addEvent = ref<number | null>(null);

const instance = computed(() => server.instanceList.find((i) => i.id === instanceId.value) ?? null);
const repeatMode = computed(() => instance.value?.run_mode === "repeat_event");
const eventsInCategory = computed(() =>
  allEvents.value.filter((e) => !addCategory.value || e.event_category_id === addCategory.value),
);
watch(eventsInCategory, (events) => {
  if (!events.some(event => event.id === addEvent.value)) addEvent.value = null;
});
const pendingRows = computed(() => rows.value.filter((r) => !r.finished));
const activeRow = computed(
  () => rows.value.find((r) => !r.finished && (r.started_at ?? 0) > 0 && instance.value?.running) ?? null,
);
// Rows that haven't started yet (the running one is excluded).
const upcomingRows = computed(() => rows.value.filter((r) => rowState(r) === "pending"));

function fmtDuration(min: number): string {
  if (min <= 0) return "0m";
  const h = Math.floor(min / 60);
  const m = min % 60;
  if (h && m) return `${h}h ${m}m`;
  return h ? `${h}h` : `${m}m`;
}

// Best-effort ETA: cumulative timed-session minutes of the upcoming rows ahead
// of this one. The first upcoming row runs after whatever is on track now (its
// remaining time is unknown), so it is simply "next". A lap race upstream has
// no fixed duration, so everything past it is reported relative to it.
function etaLabel(r: QueueRow): string {
  const idx = upcomingRows.value.indexOf(r);
  if (idx < 0) return "";
  if (idx === 0) return activeRow.value ? "After current" : "Next up";
  const upstream = upcomingRows.value.slice(0, idx);
  if (upstream.some((u) => !u.duration_min)) return "After a lap race";
  return `~${fmtDuration(upstream.reduce((s, u) => s + u.duration_min, 0))} in`;
}

async function guard(fn: () => Promise<void>) {
  busy.value = true;
  error.value = "";
  try {
    await fn();
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function reload() {
  const version = ++loadVersion;
  const id = instanceId.value;
  if (id === null || !server.instances[id]) { loading.value = false; return; }
  loading.value = true;
  try {
    const result = await api.get<{ items: QueueRow[] }>(`/api/queue?instance=${id}`);
    if (version === loadVersion && id === instanceId.value) { rows.value = result.items; error.value = ""; }
  } catch (e) {
    if (version === loadVersion && id === instanceId.value) error.value = e instanceof Error ? e.message : "Could not load the run plan.";
  } finally { if (version === loadVersion) loading.value = false; }
}

const act = (fn: () => Promise<unknown>) =>
  guard(async () => {
    await fn();
    await reload();
  });

const moveUp = (id: number) => act(() => api.post(`/api/queue/moveup/${id}`));
const moveDown = (id: number) => act(() => api.post(`/api/queue/movedown/${id}`));

// --- Drag-and-drop reorder (keyboard up/down remain as fallback) ---
const dragId = ref<number | null>(null);
const dragOverId = ref<number | null>(null);

function onDrop(targetId: number) {
  const from = dragId.value;
  dragId.value = null;
  dragOverId.value = null;
  if (from === null || from === targetId) return;
  const ids = upcomingRows.value.map((r) => r.id);
  const fi = ids.indexOf(from);
  const ti = ids.indexOf(targetId);
  if (fi < 0 || ti < 0) return;
  ids.splice(ti, 0, ids.splice(fi, 1)[0]);
  // Optimistic: reflect the new order locally, then persist + reload.
  const byId = new Map(rows.value.map((r) => [r.id, r]));
  const reordered = ids.map((id) => byId.get(id)!).filter(Boolean);
  const finished = rows.value.filter((r) => r.finished);
  const active = activeRow.value;
  rows.value = [...finished, ...(active ? [active] : []), ...reordered];
  act(() => api.put("/api/queue/order", { instance: instanceId.value, ids: [...(active ? [active.id] : []), ...ids] }));
}
const removeRow = (id: number) => act(() => api.delete(`/api/queue/${id}`));
const clearCompleted = () => act(async () => {
  // The legacy clearcompleted endpoint clears every server. This page owns only
  // the selected server's rows, so remove its completed entries individually.
  for (const row of rows.value.filter(row => row.finished && row.instance_id === instanceId.value)) {
    await api.delete(`/api/queue/${row.id}`);
    rows.value = rows.value.filter(item => item.id !== row.id);
  }
});

const skip = () =>
  act(async () => {
    const ok = await confirm.ask({
      title: "Skip current event",
      message: `Skip the current event on ${instance.value?.name ?? "this server"} and advance its queue?`,
      detail: "Everyone on the server is kicked when the event rotates.",
      confirmLabel: "Skip event",
      tone: "danger",
    });
    if (!ok) return;
    await api.post(`/api/queue/skipevent?instance=${instanceId.value}`);
    toast.success("Skipping to next event.");
  });

// --- Scheduled start ---
const scheduleOpen = ref(false);
const scheduleValue = ref("");

function toLocalInput(unix: number): string {
  const d = new Date(unix * 1000);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

const scheduledLabel = computed(() =>
  instance.value?.scheduled_start ? new Date(instance.value.scheduled_start * 1000).toLocaleString() : "",
);

function openSchedule() {
  const existing = instance.value?.scheduled_start;
  scheduleValue.value = existing ? toLocalInput(existing) : toLocalInput(Math.floor(Date.now() / 1000) + 3600);
  scheduleOpen.value = true;
}

const confirmSchedule = () =>
  act(async () => {
    if (instanceId.value === null || !scheduleValue.value) return;
    const ts = Math.floor(new Date(scheduleValue.value).getTime() / 1000);
    await server.setSchedule(instanceId.value, ts);
    scheduleOpen.value = false;
    toast.success(`Start scheduled for ${new Date(ts * 1000).toLocaleString()}.`);
  });

const clearSchedule = () =>
  act(async () => {
    if (instanceId.value === null) return;
    await server.setSchedule(instanceId.value, null);
    toast.success("Scheduled start cleared.");
  });

const start = () => act(() => server.start(instanceId.value!));
const stop = () =>
  act(async () => {
    const ok = await confirm.ask({
      title: "Stop server",
      message: `Stop ${instance.value?.name ?? "this instance"}?`,
      detail: "Connected players are disconnected.",
      confirmLabel: "Stop server",
      tone: "danger",
    });
    if (!ok) return;
    await server.stop(instanceId.value!);
  });

const switchToManual = () =>
  act(async () => {
    if (instanceId.value === null) return;
    const ok = await confirm.ask({
      title: "Switch to manual queue",
      message: `Stop repeating and return ${instance.value?.name ?? "this instance"} to manual queue?`,
      detail: "Any previously queued events are kept and become editable again.",
      confirmLabel: "Switch to manual",
    });
    if (!ok) return;
    await server.setRunMode(instanceId.value, "manual_queue");
    toast.success("Switched to manual queue.");
  });

const addEventToQueue = () =>
  act(async () => {
    if (!addEvent.value || !eventsInCategory.value.some(event => event.id === addEvent.value)) return;
    await api.post(`/api/queue/event/${addEvent.value}?instance=${instanceId.value}`);
    toast.success("Event queued.");
  });

const addCategoryToQueue = () =>
  act(async () => {
    if (!addCategory.value) return;
    await api.post(`/api/queue/category/${addCategory.value}?instance=${instanceId.value}`);
    toast.success("All events from the group queued.");
  });

function rowState(r: QueueRow) { return queueState(r, !!instance.value?.running); }

// --- Inline "New race setup" → save into a group, then queue it on this instance ---
const newSetupOpen = ref(false);
const newSetup = ref<RaceSetupDraft>(emptyRaceSetup());

let newSetupBaseline = "";
const markNewSetupClean = () => (newSetupBaseline = JSON.stringify(newSetup.value));
const guardClose = useUnsavedGuard(() => newSetupOpen.value && JSON.stringify(newSetup.value) !== newSetupBaseline);
const closeNewSetup = () => guardClose(() => { newSetupOpen.value = false; });

function openNewSetup() {
  newSetup.value = emptyRaceSetup();
  markNewSetupClean();
  newSetupOpen.value = true;
}

// A new setup needs a home group: the chosen "add" group, else the first group,
// else a freshly created default one.
async function ensureGroup(): Promise<number> {
  if (addCategory.value) return addCategory.value;
  if (categories.value.length) return categories.value[0].id!;
  const { id } = await api.post<{ id: number }>("/api/categories", { name: "Race setups" });
  categories.value = (await api.get<{ items: DropDownList[] }>("/api/categories")).items;
  return id;
}

const saveAndQueueSetup = () =>
  act(async () => {
    if (instanceId.value === null) return;
    if (!raceSetupValid(newSetup.value)) {
      toast.error("Track and all four presets are required.");
      return;
    }
    const groupId = await ensureGroup();
    const body = raceSetupBody(newSetup.value, groupId);
    if (newSetup.value.id) await api.put(`/api/event/${newSetup.value.id}`, body);
    else {
      const saved = await api.post<{ id: number }>("/api/events", body);
      newSetup.value.id = saved.id;
    }
    await api.post(`/api/queue/event/${newSetup.value.id}?instance=${instanceId.value}`);
    newSetupOpen.value = false;
    allEvents.value = (await api.get<{ items: UserEventList[] }>("/api/events")).items;
    toast.success("Race setup created and queued.");
  });

onMounted(() =>
  guard(async () => {
    await server.load();
    // Honor ?instance from the URL when it points at a real instance, else
    // ask for a choice when more than one server is available.
    if (instanceId.value !== null && !server.instances[instanceId.value]) {
      selectionNotice.value = "That server is no longer available. Choose a server to view its run plan.";
      instanceId.value = null;
    } else if (instanceId.value === null) {
      instanceId.value = server.selectedInstanceId ?? (server.instanceList.length === 1 ? server.instanceList[0].id : null);
    }
    if (instanceId.value !== null) server.selectInstance(instanceId.value);
    const [cats, events] = await Promise.all([
      // All event groups — a group is just a folder of events. The legacy
      // ?filled=1 hid groups that were never renamed (filled stays 0 on create),
      // so only some of the user's groups showed. The event dropdown below
      // already narrows to events in the chosen group.
      api.get<{ items: DropDownList[] }>("/api/categories"),
      api.get<{ items: UserEventList[] }>("/api/events"),
    ]);
    categories.value = cats.items;
    allEvents.value = events.items;
    await reload();
  }),
);

watch(instanceId, () => {
  rows.value = [];
  server.selectInstance(instanceId.value);
  if (instance.value) selectionNotice.value = "";
  else if (instanceId.value !== null) selectionNotice.value = "That server is no longer available. Choose a server to continue.";
  void guard(reload);
});

// Track changes / starts / stops flip running flags — refetch the queue
watch(
  () => server.instanceList.map((i) => `${i.id}:${i.running}`).join(","),
  () => void reload(),
);
</script>

<template>
  <div v-if="error" role="alert" class="mb-4 rounded-md border border-danger/40 bg-danger-glow p-4 text-sm"><p class="text-danger">{{ error }}</p><Button variant="dark" class="mt-3" :disabled="loading" @click="reload">Retry loading run plan</Button></div>
  <p v-if="loading" role="status" class="mb-3 text-sm text-muted">Updating run plan…</p>
  <PageHeader
    :title="instance ? `${instance.name} · Run Plan` : 'Run Plan'"
    subtitle="Manage the per-instance run order, start servers, and queue individual race setups or whole groups."
    icon="queue"
  >
    <template #actions>
      <Button v-if="auth.canOperate && activeRow" variant="ghost" size="sm" :disabled="busy" @click="skip">
        <Icon name="skip" :size="15" />
        Skip
      </Button>
      <Button
        v-if="auth.canOperate && instance && !instance.running && (pendingRows.length || repeatMode)"
        variant="dark"
        :disabled="busy"
        @click="openSchedule"
      >
        <Icon name="calendar" :size="15" />
        {{ instance?.scheduled_start ? "Reschedule" : "Schedule" }}
      </Button>
      <Button
        v-if="auth.canOperate && instance && !instance.running && (pendingRows.length || repeatMode)"
        variant="success"
        :disabled="busy"
        @click="start"
      >
        <Icon name="power" :size="15" />
        Start
      </Button>
      <Button v-if="auth.canOperate && instance?.running" variant="danger" :disabled="busy" @click="stop">
        <Icon name="stop" :size="15" />
        Stop
      </Button>
    </template>
  </PageHeader>

  <p v-if="selectionNotice" role="status" class="mb-3 text-sm text-warn">{{ selectionNotice }}</p>
  <div v-if="server.instanceList.length > 1" class="mb-4 flex flex-wrap gap-1">
    <button
      v-for="inst in server.instanceList"
      :key="inst.id"
      type="button"
      class="flex min-h-11 cursor-pointer items-center gap-2 rounded-md border px-4 text-sm font-medium transition-colors"
      :class="
        inst.id === instanceId
          ? 'border-accent/45 bg-accent-dim text-accent'
          : 'border-line bg-surface text-muted hover:border-line-hi hover:text-text'
      "
      :aria-pressed="instanceId === inst.id"
      @click="instanceId = inst.id"
    >
      <span class="size-1.5 rounded-full" :class="inst.running ? 'bg-ok' : 'bg-dim'" />
      {{ inst.name }}
    </button>
  </div>

  <p
    v-if="scheduledLabel && !instance?.running"
    class="mb-4 flex items-center gap-2 rounded-md border border-accent/40 bg-accent-dim px-3 py-2 text-sm text-text"
  >
    <Icon name="calendar" :size="16" class="shrink-0 text-accent" />
    <span>Scheduled to start {{ scheduledLabel }}.</span>
    <button
      v-if="auth.canOperate"
      type="button"
      class="ml-auto cursor-pointer text-xs font-semibold text-accent hover:underline"
      @click="clearSchedule"
    >
      Cancel
    </button>
  </p>

  <!-- Repeat mode: the manual queue is frozen while one event auto-repeats -->
  <EmptyState v-if="!instance" icon="instances" title="Choose a server" message="Select a server to see its run order and available actions." />
  <Card v-else-if="repeatMode">
    <template #header>
      <Icon name="repeat" :size="16" class="text-accent" />
      <h2 class="text-sm font-bold">Repeat mode</h2>
      <span v-if="instance" class="text-xs text-dim">{{ instance.name }}</span>
    </template>
    <template #actions>
      <Button v-if="auth.canOperate" variant="dark" size="sm" :disabled="busy" @click="switchToManual">
        <Icon name="queue" :size="14" />
        Switch to manual queue
      </Button>
    </template>

    <div class="flex items-start gap-3">
      <Icon name="repeat" :size="20" class="mt-0.5 shrink-0 text-accent" />
      <div>
        <p class="text-sm">
          This instance re-runs the same event continuously. It restarts automatically when the
          race finishes; manual queueing is disabled.
        </p>
        <p v-if="instance?.repeat_event" class="mt-2 text-sm font-medium">
          {{ instance.repeat_event.track || "Event #" + instance.repeat_event.id }}
          <span v-if="instance.repeat_event.category" class="text-dim">· {{ instance.repeat_event.category }}</span>
        </p>
        <p class="mt-2 text-xs text-muted">
          <Icon name="lock" :size="12" class="-mt-0.5 mr-1 inline" />
          Any previously queued events are preserved and return when you switch back to manual.
        </p>
      </div>
    </div>
  </Card>

  <div v-else class="grid items-start gap-5 xl:grid-cols-[1fr_340px]">
    <Card>
      <template #header>
        <Icon name="queue" :size="16" class="text-accent" />
        <h2 class="text-sm font-bold">Server queue</h2>
        <span v-if="instance" class="text-xs text-dim">{{ instance.name }}</span>
      </template>
      <template #actions>
        <Button v-if="auth.canOperate && rows.some((r) => r.finished)" variant="ghost" size="sm" @click="clearCompleted">
          <Icon name="trash" :size="14" />
          Clear completed
        </Button>
      </template>

      <div v-if="rows.length" class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-line text-left text-xs tracking-wide text-muted uppercase">
            <th class="py-2 pr-2 font-semibold">#</th>
            <th class="py-2 pr-2 font-semibold">Track</th>
            <th class="py-2 pr-2 font-semibold max-md:hidden">Class</th>
            <th class="py-2 pr-2 font-semibold max-lg:hidden">Sessions · Time · Difficulty</th>
            <th class="py-2 font-semibold"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="r in rows"
            :key="r.id"
            :draggable="auth.canOperate && rowState(r) === 'pending'"
            class="border-b border-line/60"
            :class="{
              'bg-surface-2/30': rowState(r) === 'done',
              'bg-accent-dim/40': rowState(r) === 'active',
              'cursor-grab': auth.canOperate && rowState(r) === 'pending',
              'border-t-2 border-t-accent': dragOverId === r.id && dragId !== r.id,
              'opacity-60': dragId === r.id,
            }"
            @dragstart="dragId = r.id"
            @dragover.prevent="dragOverId = r.id"
            @drop.prevent="onDrop(r.id)"
            @dragend="dragId = null; dragOverId = null"
          >
            <td class="py-2 pr-2">
              <span v-if="rowState(r) === 'pending'" class="inline-flex items-center gap-1.5 text-muted">
                <Icon name="menu" :size="14" class="text-dim" />
                {{ upcomingRows.indexOf(r) + 1 }}
              </span>
              <Icon v-else-if="rowState(r) === 'active'" name="activity" :size="15" class="text-ok" />
              <span v-else class="inline-flex items-center gap-1 text-xs text-muted"><Icon name="check" :size="15" />Done</span>
            </td>
            <td class="py-2 pr-2">
              <RouterLink :to="`/server/${r.instance_id}`" class="font-medium transition-colors hover:text-accent">
                {{ r.name || r.track }}
              </RouterLink>
              <div class="text-xs text-muted">{{ r.name ? `${r.track} · ${r.category}` : r.category }}</div>
              <details class="mt-2 lg:hidden"><summary class="cursor-pointer py-2 text-xs text-accent">Race details</summary><dl class="space-y-1 py-2 text-xs text-muted"><div><dt class="inline">Class: </dt><dd class="inline">{{ r.class }}</dd></div><div><dt class="inline">Sessions: </dt><dd class="inline">{{ r.session }}</dd></div><div><dt class="inline">Time / weather: </dt><dd class="inline">{{ r.time }}</dd></div><div><dt class="inline">Difficulty: </dt><dd class="inline">{{ r.difficulty }}</dd></div></dl></details>
              <div v-if="rowState(r) === 'pending'" class="mt-0.5 flex items-center gap-1 text-[11px] text-accent/80">
                <Icon name="clock" :size="11" />
                {{ etaLabel(r) }}
              </div>
            </td>
            <td class="py-2 pr-2 max-md:hidden">{{ r.class }}</td>
            <td class="py-2 pr-2 text-xs text-muted max-lg:hidden">
              {{ r.session }} · {{ r.time }} · {{ r.difficulty }}
            </td>
            <td class="py-2 text-right whitespace-nowrap">
              <div v-if="auth.canOperate && rowState(r) === 'pending'" class="ml-auto flex w-24 flex-wrap justify-end gap-1 sm:w-auto sm:flex-nowrap">
                <Button variant="dark" size="sm" :disabled="upcomingRows[0]?.id === r.id || busy" :aria-label="`Move ${r.name || r.track} up`" @click="moveUp(r.id)">
                  <Icon name="arrowUp" :size="14" />
                </Button>
                <Button
                  variant="dark"
                  size="sm"

                  :disabled="upcomingRows.at(-1)?.id === r.id || busy"
                  :aria-label="`Move ${r.name || r.track} down`"
                  @click="moveDown(r.id)"
                >
                  <Icon name="arrowDown" :size="14" />
                </Button>
                <Button variant="ghost" size="sm"  :aria-label="`Remove ${r.name || r.track} from queue`" @click="removeRow(r.id)">
                  <Icon name="x" :size="14" />
                </Button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      </div>

      <p v-else-if="loading" role="status" class="py-8 text-sm text-muted">Loading races…</p>
      <p v-else-if="error" class="py-8 text-sm text-muted">Run plan unavailable. Use Retry loading run plan above.</p>
      <EmptyState
        v-else
        icon="queue"
        title="Queue is empty"
        message="Use Add to queue to choose a saved race or create a new setup, then start this server."
      />
    </Card>

    <Card v-if="auth.canOperate" title="Plan the next race" class="paddock-plan">
      <Button class="mb-4 w-full" @click="openNewSetup">
        <Icon name="plus" :size="15" />
        New race setup
      </Button>
      <div class="mb-4 flex items-center gap-2 text-xs text-dim">
        <span class="h-px flex-1 bg-line" />
        or add an existing one
        <span class="h-px flex-1 bg-line" />
      </div>

      <FormRow label="Filter by group" for-id="qcat" hint="Optional. Search all setups or narrow the list to a group.">
        <Combobox
          id="qcat"
          v-model="addCategory"
          placeholder="All groups"
          :options="[{ value: 0, label: 'All groups' }, ...categories.map((c) => ({ value: c.id ?? 0, label: c.name ?? '' }))]"
        />
      </FormRow>
      <FormRow label="Race setup" for-id="qevent">
        <Combobox
          id="qevent"
          v-model="addEvent"
          placeholder="Search race setups..."
          :options="eventsInCategory.map((e) => ({ value: e.id ?? 0, label: e.name || e.track_name || '' }))"
        />
      </FormRow>
      <Button variant="dark" class="w-full" :disabled="!addEvent || busy" @click="addEventToQueue">
        <Icon name="plus" :size="15" />
        Add to run plan
      </Button>
      <Button v-if="addCategory" variant="ghost" class="mt-2 w-full" :disabled="!eventsInCategory.length || busy" @click="addCategoryToQueue">
        Add all {{ eventsInCategory.length }} setups from group
      </Button>
    </Card>
  </div>

  <!-- New race setup → create + queue in one step -->
  <Sheet wide :open="newSetupOpen" title="New race setup" @close="closeNewSetup">
    <p class="mb-3 text-sm text-muted">
      Build a race setup and queue it on
      <span class="font-medium text-text">{{ instance?.name ?? "this instance" }}</span>
      in one step. It is also saved to your library{{ addCategory ? " in the selected group" : "" }}.
    </p>
    <RaceSetupEditor v-model="newSetup" :instance-id="instanceId" />
    <p v-if="error" role="alert" class="mt-4 text-sm text-danger">{{ error }} {{ newSetup.id ? "The setup is saved in your library. Retry to add it to this run plan." : "Your draft is still here." }}</p>
    <template #footer>
      <span v-if="!raceSetupValid(newSetup)" class="basis-full self-center text-xs text-muted">
        Track and all four presets are required.
      </span>
      <Button variant="ghost" @click="closeNewSetup">Cancel</Button>
      <Button :disabled="busy || !raceSetupValid(newSetup)" @click="saveAndQueueSetup">
        <Icon name="queue" :size="15" />
        {{ newSetup.id ? "Save & queue" : "Create & queue" }}
      </Button>
    </template>
  </Sheet>

  <!-- Schedule start -->
  <Modal :open="scheduleOpen" title="Schedule start" @close="scheduleOpen = false">
    <p v-if="error" role="alert" class="mb-3 text-sm text-danger">{{ error }}</p>
    <p class="mb-3 text-sm text-muted">
      The server starts automatically at this time (within ~20s), running the queued event or the repeat event.
    </p>
    <FormRow label="Start at" for-id="sched">
      <input
        id="sched"
        v-model="scheduleValue"
        type="datetime-local"
        class="min-h-11 w-full rounded-md border border-control bg-input px-3 text-sm text-text outline-none focus:border-accent focus:bg-input focus:ring-2 focus:ring-accent/20"
      />
    </FormRow>
    <template #footer>
      <Button variant="ghost" @click="scheduleOpen = false">Cancel</Button>
      <Button :disabled="busy || !scheduleValue" @click="confirmSchedule">
        <Icon name="calendar" :size="15" />
        Schedule
      </Button>
    </template>
  </Modal>
</template>
