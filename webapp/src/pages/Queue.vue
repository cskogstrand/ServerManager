<script setup lang="ts">
// Per-instance queue: reorder, remove, skip, clear; add events or whole
// categories. Refetches when an instance starts/stops (SSE-driven).
import { computed, onMounted, ref, watch } from "vue";
import { api, ApiError } from "@/lib/api";
import { useServerStore } from "@/stores/server";
import type { DropDownList, UserEventList } from "@/types/generated";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Select from "@/components/ui/Select.vue";

interface QueueRow {
  id: number;
  event_id: number;
  instance_id: number;
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

const instanceId = ref<number | null>(null);
const rows = ref<QueueRow[]>([]);
const busy = ref(false);
const notice = ref("");
const error = ref("");

const categories = ref<DropDownList[]>([]);
const allEvents = ref<UserEventList[]>([]);
const addCategory = ref<number | null>(null);
const addEvent = ref<number | null>(null);

const instance = computed(() => server.instanceList.find((i) => i.id === instanceId.value) ?? null);
const eventsInCategory = computed(() =>
  allEvents.value.filter((e) => e.event_category_id === addCategory.value),
);
const pendingRows = computed(() => rows.value.filter((r) => !r.finished));
const activeRow = computed(
  () => rows.value.find((r) => !r.finished && (r.started_at ?? 0) > 0 && instance.value?.running) ?? null,
);

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
  if (instanceId.value === null) return;
  rows.value = (await api.get<{ items: QueueRow[] }>(`/api/queue?instance=${instanceId.value}`)).items;
}

const act = (fn: () => Promise<unknown>) =>
  guard(async () => {
    notice.value = "";
    await fn();
    await reload();
  });

const moveUp = (id: number) => act(() => api.post(`/api/queue/moveup/${id}`));
const moveDown = (id: number) => act(() => api.post(`/api/queue/movedown/${id}`));
const removeRow = (id: number) => act(() => api.delete(`/api/queue/${id}`));
const clearCompleted = () => act(() => api.post("/api/queue/clearcompleted"));

const skip = () =>
  act(async () => {
    if (!window.confirm("Skip the current event and advance the queue? Players get kicked.")) return;
    await api.post(`/api/queue/skipevent?instance=${instanceId.value}`);
  });

const start = () => act(() => server.start(instanceId.value!));
const stop = () => act(() => server.stop(instanceId.value!));

const addEventToQueue = () =>
  act(async () => {
    if (!addEvent.value) return;
    await api.post(`/api/queue/event/${addEvent.value}?instance=${instanceId.value}`);
    notice.value = "Event queued.";
  });

const addCategoryToQueue = () =>
  act(async () => {
    if (!addCategory.value) return;
    await api.post(`/api/queue/category/${addCategory.value}?instance=${instanceId.value}`);
    notice.value = "All events from the category queued.";
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
      api.get<{ items: DropDownList[] }>("/api/categories?filled=1"),
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
  <div class="mb-5 flex flex-wrap items-center gap-3">
    <h1 class="text-xl font-bold">Queue</h1>

    <div v-if="server.instanceList.length > 1" class="flex gap-1">
      <button
        v-for="inst in server.instanceList"
        :key="inst.id"
        type="button"
        class="flex items-center gap-1.5 rounded-md px-3 py-1 text-sm"
        :class="inst.id === instanceId ? 'bg-accent-dim text-accent' : 'text-muted hover:text-text'"
        @click="instanceId = inst.id"
      >
        <span class="size-1.5 rounded-full" :class="inst.running ? 'bg-ok' : 'bg-dim'" />
        {{ inst.name }}
      </button>
    </div>

    <div class="ml-auto flex gap-2">
      <Button v-if="activeRow" variant="ghost" size="sm" :disabled="busy" @click="skip">Skip event</Button>
      <Button
        v-if="instance && !instance.running && pendingRows.length"
        variant="success"
        :disabled="busy"
        @click="start"
      >
        ▶ Start server
      </Button>
      <Button v-if="instance?.running" variant="danger" :disabled="busy" @click="stop">■ Stop server</Button>
    </div>
  </div>

  <p v-if="notice" class="mb-4 rounded-md border border-ok/40 bg-ok-glow px-3 py-2 text-sm text-ok">{{ notice }}</p>
  <p v-if="error" class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger">
    {{ error }}
  </p>

  <div class="grid items-start gap-5 xl:grid-cols-[1fr_340px]">
    <Card>
      <template #header>
        <h2 class="text-sm font-semibold">Server queue</h2>
        <span v-if="instance" class="text-xs text-dim">{{ instance.name }}</span>
      </template>
      <template #actions>
        <Button v-if="rows.some((r) => r.finished)" variant="ghost" size="sm" @click="clearCompleted">
          Clear completed
        </Button>
      </template>

      <table v-if="rows.length" class="w-full text-sm">
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
            class="border-b border-line/60"
            :class="{
              'opacity-45': rowState(r) === 'done',
              'bg-accent-dim/40': rowState(r) === 'active',
            }"
          >
            <td class="py-2 pr-2">
              <span v-if="rowState(r) === 'active'" class="text-ok">▶</span>
              <span v-else-if="rowState(r) === 'done'" class="text-dim">✓</span>
              <span v-else class="text-muted">{{ pendingRows.indexOf(r) + 1 }}</span>
            </td>
            <td class="py-2 pr-2">
              <div class="font-medium">{{ r.track }}</div>
              <div class="text-xs text-dim">{{ r.category }}</div>
            </td>
            <td class="py-2 pr-2 max-md:hidden">{{ r.class }}</td>
            <td class="py-2 pr-2 text-xs text-muted max-lg:hidden">
              {{ r.session }} · {{ r.time }} · {{ r.difficulty }}
            </td>
            <td class="py-2 text-right whitespace-nowrap">
              <template v-if="rowState(r) === 'pending'">
                <Button variant="dark" size="sm" :disabled="i === 0 || busy" aria-label="Move up" @click="moveUp(r.id)">↑</Button>
                <Button
                  variant="dark"
                  size="sm"
                  class="ml-1"
                  :disabled="i === rows.length - 1 || busy"
                  aria-label="Move down"
                  @click="moveDown(r.id)"
                >
                  ↓
                </Button>
                <Button variant="ghost" size="sm" class="ml-1" aria-label="Remove" @click="removeRow(r.id)">✕</Button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>

      <p v-else class="py-6 text-center text-sm text-muted">Queue is empty — add events on the right.</p>
    </Card>

    <Card title="Add to queue">
      <FormRow label="Category" for-id="qcat">
        <Select
          id="qcat"
          v-model="addCategory"
          :options="categories.map((c) => ({ value: c.id ?? 0, label: c.name ?? '' }))"
        />
      </FormRow>
      <Button variant="dark" class="mb-4 w-full" :disabled="!addCategory || busy" @click="addCategoryToQueue">
        Add all from category
      </Button>

      <FormRow label="Single event" for-id="qevent">
        <Select
          id="qevent"
          v-model="addEvent"
          :options="eventsInCategory.map((e) => ({ value: e.id ?? 0, label: e.track_name ?? '' }))"
        />
      </FormRow>
      <Button class="w-full" :disabled="!addEvent || busy" @click="addEventToQueue">Add event</Button>
    </Card>
  </div>
</template>
