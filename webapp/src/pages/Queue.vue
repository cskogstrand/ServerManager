<script setup lang="ts">
// Per-instance queue: reorder, remove, skip, clear; add events or whole
// categories. Refetches when an instance starts/stops (SSE-driven).
import { computed, onMounted, ref, watch } from "vue";
import { api, ApiError } from "@/lib/api";
import { useServerStore } from "@/stores/server";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import type { DropDownList, UserEventList } from "@/types/generated";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Select from "@/components/ui/Select.vue";
import Icon from "@/components/ui/Icon.vue";
import Modal from "@/components/ui/Modal.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import EmptyState from "@/components/ui/EmptyState.vue";

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
  started_at: number | null;
  finished: number;
}

const server = useServerStore();
const toast = useToastStore();
const confirm = useConfirmStore();

const instanceId = ref<number | null>(null);
const rows = ref<QueueRow[]>([]);
const busy = ref(false);

const categories = ref<DropDownList[]>([]);
const allEvents = ref<UserEventList[]>([]);
const addCategory = ref<number | null>(null);
const addEvent = ref<number | null>(null);

const instance = computed(() => server.instanceList.find((i) => i.id === instanceId.value) ?? null);
const repeatMode = computed(() => instance.value?.run_mode === "repeat_event");
const eventsInCategory = computed(() =>
  allEvents.value.filter((e) => e.event_category_id === addCategory.value),
);
const pendingRows = computed(() => rows.value.filter((r) => !r.finished));
const activeRow = computed(
  () => rows.value.find((r) => !r.finished && (r.started_at ?? 0) > 0 && instance.value?.running) ?? null,
);

async function guard(fn: () => Promise<void>) {
  busy.value = true;
  try {
    await fn();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

async function reload() {
  if (instanceId.value === null) return;
  rows.value = (await api.get<{ items: QueueRow[] }>(`/api/queue?instance=${instanceId.value}`)).items;
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
  const ids = pendingRows.value.map((r) => r.id);
  const fi = ids.indexOf(from);
  const ti = ids.indexOf(targetId);
  if (fi < 0 || ti < 0) return;
  ids.splice(ti, 0, ids.splice(fi, 1)[0]);
  // Optimistic: reflect the new order locally, then persist + reload.
  const byId = new Map(rows.value.map((r) => [r.id, r]));
  const reordered = ids.map((id) => byId.get(id)!).filter(Boolean);
  const finished = rows.value.filter((r) => r.finished);
  rows.value = [...finished, ...reordered];
  act(() => api.put("/api/queue/order", { instance: instanceId.value, ids }));
}
const removeRow = (id: number) => act(() => api.delete(`/api/queue/${id}`));
const clearCompleted = () => act(() => api.post("/api/queue/clearcompleted"));

const skip = () =>
  act(async () => {
    const ok = await confirm.ask({
      title: "Skip current event",
      message: "Skip the current event and advance the queue?",
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
    if (!addEvent.value) return;
    await api.post(`/api/queue/event/${addEvent.value}?instance=${instanceId.value}`);
    toast.success("Event queued.");
  });

const addCategoryToQueue = () =>
  act(async () => {
    if (!addCategory.value) return;
    await api.post(`/api/queue/category/${addCategory.value}?instance=${instanceId.value}`);
    toast.success("All events from the group queued.");
  });

function rowState(r: QueueRow): "done" | "active" | "pending" {
  if (r.finished) return "done";
  if ((r.started_at ?? 0) > 0 && instance.value?.running) return "active";
  return "pending";
}

onMounted(() =>
  guard(async () => {
    await server.load();
    instanceId.value = server.instanceList[0]?.id ?? null;
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

watch(instanceId, () => guard(reload));

// Track changes / starts / stops flip running flags — refetch the queue
watch(
  () => server.instanceList.map((i) => `${i.id}:${i.running}`).join(","),
  () => void reload(),
);
</script>

<template>
  <PageHeader
    title="Queue"
    subtitle="Manage the per-instance run order, start servers, and queue individual events or whole event groups."
    icon="queue"
  >
    <template #actions>
      <Button v-if="activeRow" variant="ghost" size="sm" :disabled="busy" @click="skip">
        <Icon name="skip" :size="15" />
        Skip
      </Button>
      <Button
        v-if="instance && !instance.running && (pendingRows.length || repeatMode)"
        variant="dark"
        :disabled="busy"
        @click="openSchedule"
      >
        <Icon name="calendar" :size="15" />
        {{ instance?.scheduled_start ? "Reschedule" : "Schedule" }}
      </Button>
      <Button
        v-if="instance && !instance.running && (pendingRows.length || repeatMode)"
        variant="success"
        :disabled="busy"
        @click="start"
      >
        <Icon name="power" :size="15" />
        Start
      </Button>
      <Button v-if="instance?.running" variant="danger" :disabled="busy" @click="stop">
        <Icon name="stop" :size="15" />
        Stop
      </Button>
    </template>
  </PageHeader>

  <div v-if="server.instanceList.length > 1" class="mb-4 flex flex-wrap gap-1">
    <button
      v-for="inst in server.instanceList"
      :key="inst.id"
      type="button"
      class="flex min-h-9 cursor-pointer items-center gap-2 rounded-md border px-3 text-sm font-semibold transition-colors"
      :class="
        inst.id === instanceId
          ? 'border-accent/45 bg-accent-dim text-accent'
          : 'border-line bg-surface text-muted hover:border-line-hi hover:text-text'
      "
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
    <button type="button" class="ml-auto cursor-pointer text-xs font-semibold text-accent hover:underline" @click="clearSchedule">
      Cancel
    </button>
  </p>

  <!-- Repeat mode: the manual queue is frozen while one event auto-repeats -->
  <Card v-if="repeatMode">
    <template #header>
      <Icon name="repeat" :size="16" class="text-accent" />
      <h2 class="text-sm font-bold">Repeat mode</h2>
      <span v-if="instance" class="text-xs text-dim">{{ instance.name }}</span>
    </template>
    <template #actions>
      <Button variant="dark" size="sm" :disabled="busy" @click="switchToManual">
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
        <Button v-if="rows.some((r) => r.finished)" variant="ghost" size="sm" @click="clearCompleted">
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
            v-for="(r, i) in rows"
            :key="r.id"
            :draggable="rowState(r) === 'pending'"
            class="border-b border-line/60"
            :class="{
              'opacity-45': rowState(r) === 'done',
              'bg-accent-dim/40': rowState(r) === 'active',
              'cursor-grab': rowState(r) === 'pending',
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
                {{ pendingRows.indexOf(r) + 1 }}
              </span>
              <Icon v-else-if="rowState(r) === 'active'" name="activity" :size="15" class="text-ok" />
              <Icon v-else name="check" :size="15" class="text-dim" />
            </td>
            <td class="py-2 pr-2">
              <div class="font-medium">{{ r.name || r.track }}</div>
              <div class="text-xs text-dim">{{ r.name ? `${r.track} · ${r.category}` : r.category }}</div>
            </td>
            <td class="py-2 pr-2 max-md:hidden">{{ r.class }}</td>
            <td class="py-2 pr-2 text-xs text-muted max-lg:hidden">
              {{ r.session }} · {{ r.time }} · {{ r.difficulty }}
            </td>
            <td class="py-2 text-right whitespace-nowrap">
              <template v-if="rowState(r) === 'pending'">
                <Button variant="dark" size="sm" :disabled="i === 0 || busy" aria-label="Move up" @click="moveUp(r.id)">
                  <Icon name="arrowUp" :size="14" />
                </Button>
                <Button
                  variant="dark"
                  size="sm"
                  class="ml-1"
                  :disabled="i === rows.length - 1 || busy"
                  aria-label="Move down"
                  @click="moveDown(r.id)"
                >
                  <Icon name="arrowDown" :size="14" />
                </Button>
                <Button variant="ghost" size="sm" class="ml-1" aria-label="Remove" @click="removeRow(r.id)">
                  <Icon name="x" :size="14" />
                </Button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
      </div>

      <EmptyState
        v-else
        icon="queue"
        title="Queue is empty"
        message="Add a single event or a whole event group on the right, then start the server."
      />
    </Card>

    <Card title="Add to queue">
      <FormRow label="Event group" for-id="qcat">
        <Select
          id="qcat"
          v-model="addCategory"
          :options="categories.map((c) => ({ value: c.id ?? 0, label: c.name ?? '' }))"
        />
      </FormRow>
      <Button variant="dark" class="mb-4 w-full" :disabled="!addCategory || busy" @click="addCategoryToQueue">
        <Icon name="plus" :size="15" />
        Add all from group
      </Button>

      <FormRow label="Single event" for-id="qevent">
        <Select
          id="qevent"
          v-model="addEvent"
          :options="eventsInCategory.map((e) => ({ value: e.id ?? 0, label: e.name || e.track_name || '' }))"
        />
      </FormRow>
      <Button class="w-full" :disabled="!addEvent || busy" @click="addEventToQueue">
        <Icon name="plus" :size="15" />
        Add event
      </Button>
    </Card>
  </div>

  <!-- Schedule start -->
  <Modal :open="scheduleOpen" title="Schedule start" @close="scheduleOpen = false">
    <p class="mb-3 text-sm text-muted">
      The server starts automatically at this time (within ~20s), running the queued event or the repeat event.
    </p>
    <FormRow label="Start at" for-id="sched">
      <input
        id="sched"
        v-model="scheduleValue"
        type="datetime-local"
        class="min-h-9 w-full rounded-md border border-line bg-surface-2 px-3 text-sm text-text outline-none focus:border-accent focus:bg-surface-3 focus:ring-2 focus:ring-accent/20"
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
