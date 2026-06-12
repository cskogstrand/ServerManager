<script setup lang="ts">
// Event Builder: categories on the left, events as cards, builder in a side
// sheet. Presets are picked from dropdowns (filled ones only); each select
// links to its editor for creating new presets.
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useServerStore } from "@/stores/server";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import type { DropDownList } from "@/types/generated";
import PresetShell from "@/components/presets/PresetShell.vue";
import InlinePresetSheet from "@/components/presets/InlinePresetSheet.vue";
import type { PresetKind } from "@/lib/presetForms";
import TrackPicker from "@/components/TrackPicker.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Sheet from "@/components/ui/Sheet.vue";
import Modal from "@/components/ui/Modal.vue";
import Icon from "@/components/ui/Icon.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import Skeleton from "@/components/ui/Skeleton.vue";

interface EventRow {
  id: number | null;
  track_key: string;
  track_config: string;
  track_name: string;
  pitboxes: number | null;
  difficulty_id: number | null;
  difficulty_name: string;
  session_id: number | null;
  session_name: string;
  class_id: number | null;
  class_name: string;
  entries: number | null;
  time_id: number | null;
  time_name: string;
  race_laps: number | null;
  strategy: number | null;
}

const server = useServerStore();
const toast = useToastStore();
const confirm = useConfirmStore();

const categories = ref<DropDownList[]>([]);
const selectedId = ref<number | null>(null);
const categoryName = ref("");
const events = ref<EventRow[]>([]);
const busy = ref(false);
const loading = ref(true);

// Preset dropdown options (only "filled" presets render valid configs)
const difficulties = ref<DropDownList[]>([]);
const sessions = ref<DropDownList[]>([]);
const classes = ref<DropDownList[]>([]);
const times = ref<DropDownList[]>([]);

// Inline validation: every runnable event needs all four presets + a track.
const missingPresets = computed(() => {
  const m: string[] = [];
  if (!classes.value.length) m.push("car class");
  if (!sessions.value.length) m.push("session");
  if (!times.value.length) m.push("time & weather");
  if (!difficulties.value.length) m.push("difficulty");
  return m;
});
const canSave = computed(() => {
  const e = editing.value;
  return !!(e && e.track_key && e.class_id && e.session_id && e.time_id && e.difficulty_id);
});

// The category GET serializes events with legacy mixed-case keys and
// stringified ids — normalize once here.
function normalizeEvent(raw: any): EventRow {
  const num = (v: any) => (v === null || v === undefined || v === "" ? null : Number(v));
  return {
    id: raw.Id ?? null,
    track_key: raw.CacheTrackKey ?? "",
    track_config: raw.CacheTrackConfig ?? "",
    track_name: raw.TrackName ?? raw.CacheTrackKey ?? "",
    pitboxes: num(raw.Pitboxes),
    difficulty_id: num(raw.difficulty),
    difficulty_name: raw.DifficultyName ?? "",
    session_id: num(raw.session),
    session_name: raw.SessionName ?? "",
    class_id: num(raw.class),
    class_name: raw.ClassName ?? "",
    entries: num(raw.Entries),
    time_id: num(raw.time),
    time_name: raw.TimeName ?? "",
    race_laps: num(raw.race_laps),
    strategy: num(raw.strategy),
  };
}

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

const loadCategories = async () => {
  categories.value = (await api.get<{ items: DropDownList[] }>("/api/categories")).items;
};

const loadPresetLists = async () => {
  [difficulties.value, sessions.value, classes.value, times.value] = await Promise.all([
    api.get<{ items: DropDownList[] }>("/api/difficulties?filled=1").then((r) => r.items),
    api.get<{ items: DropDownList[] }>("/api/sessions?filled=1").then((r) => r.items),
    api.get<{ items: DropDownList[] }>("/api/classes?filled=1").then((r) => r.items),
    api.get<{ items: DropDownList[] }>("/api/times?filled=1").then((r) => r.items),
  ]);
};

const select = (id: number) =>
  guard(async () => {
    const cat = await api.get<any>(`/api/category/${id}`);
    selectedId.value = id;
    categoryName.value = cat.name ?? "";
    events.value = (cat.events ?? []).filter((e: any) => e.Id != null).map(normalizeEvent);
  });

const create = (name: string) =>
  guard(async () => {
    const { id } = await api.post<{ id: number }>("/api/categories", { name });
    await loadCategories();
    await select(id);
  });

const remove = (id: number) =>
  guard(async () => {
    const grp = categories.value.find((c) => c.id === id);
    const ok = await confirm.ask({
      title: "Delete event group",
      message: `Delete "${grp?.name ?? "this group"}" and all its events?`,
      detail: "Any queue entries for those events are removed too. This cannot be undone.",
      confirmLabel: "Delete group",
      tone: "danger",
    });
    if (!ok) return;
    await api.delete(`/api/category/${id}`);
    if (selectedId.value === id) {
      selectedId.value = null;
      events.value = [];
    }
    await loadCategories();
    toast.success("Event group deleted.");
  });

const rename = () =>
  guard(async () => {
    if (!selectedId.value) return;
    await api.patch(`/api/category/${selectedId.value}`, { name: categoryName.value });
    await loadCategories();
    toast.success("Event group renamed.");
  });

// --- Builder sheet ---
const builderOpen = ref(false);
const trackPickerOpen = ref(false);
const editing = ref<EventRow | null>(null);

function emptyEvent(): EventRow {
  return {
    id: null,
    track_key: "",
    track_config: "",
    track_name: "",
    pitboxes: null,
    difficulty_id: null,
    difficulty_name: "",
    session_id: null,
    session_name: "",
    class_id: null,
    class_name: "",
    entries: null,
    time_id: null,
    time_name: "",
    race_laps: 0,
    strategy: 1,
  };
}

function openBuilder(event?: EventRow) {
  editing.value = event ? { ...event } : emptyEvent();
  builderOpen.value = true;
}

function onTrackPicked(track: { key: string; config: string; name: string; pitboxes: number }) {
  if (!editing.value) return;
  editing.value.track_key = track.key;
  editing.value.track_config = track.config;
  editing.value.track_name = track.name;
  editing.value.pitboxes = track.pitboxes;
  trackPickerOpen.value = false;
}

const saveEvent = () =>
  guard(async () => {
    const e = editing.value;
    if (!e || !selectedId.value) return;
    if (!e.track_key) {
      toast.error("Pick a track first.");
      return;
    }
    if (!e.difficulty_id || !e.session_id || !e.class_id || !e.time_id) {
      toast.error("Difficulty, sessions, class and time are all required.");
      return;
    }

    const body = {
      event_category_id: selectedId.value,
      track_key: e.track_key,
      track_config: e.track_config,
      difficulty_id: e.difficulty_id,
      session_id: e.session_id,
      class_id: e.class_id,
      time_id: e.time_id,
      race_laps: e.race_laps ?? 0,
      strategy: e.strategy ?? 1,
    };

    if (e.id) {
      await api.put(`/api/event/${e.id}`, body);
    } else {
      await api.post("/api/events", body);
    }
    builderOpen.value = false;
    await select(selectedId.value);
    toast.success(e.id ? "Event saved." : "Event added.");
  });

const deleteEvent = (e: EventRow) =>
  guard(async () => {
    if (!e.id || !selectedId.value) return;
    const ok = await confirm.ask({
      title: "Delete event",
      message: `Delete the ${e.track_name} event?`,
      detail: "Its queue entries are removed too. This cannot be undone.",
      confirmLabel: "Delete event",
      tone: "danger",
    });
    if (!ok) return;
    await api.delete(`/api/event/${e.id}`);
    await select(selectedId.value);
    toast.success("Event deleted.");
  });

// Duplicate event: clone the row into the same group, then re-select so the
// new card appears. Saves rebuilding near-identical events by hand.
const duplicateEvent = (e: EventRow) =>
  guard(async () => {
    if (!e.id || !selectedId.value) return;
    await api.post("/api/events", {
      event_category_id: selectedId.value,
      track_key: e.track_key,
      track_config: e.track_config,
      difficulty_id: e.difficulty_id,
      session_id: e.session_id,
      class_id: e.class_id,
      time_id: e.time_id,
      race_laps: e.race_laps ?? 0,
      strategy: e.strategy ?? 1,
    });
    await select(selectedId.value);
    toast.success("Event duplicated.");
  });

const queueEvent = (e: EventRow) =>
  guard(async () => {
    if (!e.id) return;
    const inst = server.instanceList[0];
    await api.post(`/api/queue/event/${e.id}${inst ? `?instance=${inst.id}` : ""}`);
    toast.success(`Queued ${e.track_name} on ${inst?.name ?? "the default instance"}.`);
  });

// --- Repeat-on-instance picker ---
const repeatPickerOpen = ref(false);
const repeatTarget = ref<EventRow | null>(null);
const repeatInstanceId = ref<number | null>(null);

function openRepeat(e: EventRow) {
  repeatTarget.value = e;
  repeatInstanceId.value = server.instanceList[0]?.id ?? null;
  repeatPickerOpen.value = true;
}

const confirmRepeat = () =>
  guard(async () => {
    if (!repeatTarget.value?.id || repeatInstanceId.value === null) return;
    await server.setRunMode(repeatInstanceId.value, "repeat_event", repeatTarget.value.id);
    const inst = server.instanceList.find((i) => i.id === repeatInstanceId.value);
    repeatPickerOpen.value = false;
    toast.success(`${inst?.name ?? "Instance"} will repeat ${repeatTarget.value.track_name}. Start it from the dashboard.`);
  });

// --- Config preview (rendered server_cfg.ini / entry_list.ini) ---
const previewOpen = ref(false);
const previewCfg = ref("");
const previewEntry = ref("");
const previewBusy = ref(false);

async function openPreview() {
  const e = editing.value;
  if (!e?.id) return;
  previewOpen.value = true;
  previewBusy.value = true;
  previewCfg.value = "";
  previewEntry.value = "";
  try {
    const inst = server.instanceList[0];
    const q = `id=${e.id}${inst ? `&instance=${inst.id}` : ""}`;
    const [cfg, entry] = await Promise.all([
      fetch(`/api/server/server_cfg.ini?${q}`).then((r) => r.text()),
      fetch(`/api/server/entry_list.ini?${q}`).then((r) => r.text()),
    ]);
    previewCfg.value = cfg;
    previewEntry.value = entry;
  } catch (e) {
    toast.error(String(e));
  } finally {
    previewBusy.value = false;
  }
}

// --- Inline preset creation from the builder ---
const inlineOpen = ref(false);
const inlineKind = ref<PresetKind>("class");

function openInline(kind: PresetKind) {
  inlineKind.value = kind;
  inlineOpen.value = true;
}

const presetFieldByKind: Record<PresetKind, keyof EventRow> = {
  class: "class_id",
  session: "session_id",
  time: "time_id",
  difficulty: "difficulty_id",
};

async function onPresetCreated(id: number) {
  await loadPresetLists();
  if (editing.value) {
    (editing.value[presetFieldByKind[inlineKind.value]] as number | null) = id;
  }
}

onMounted(() =>
  guard(async () => {
    await Promise.all([loadCategories(), loadPresetLists(), server.load()]);
    loading.value = false;
  }),
);
</script>

<template>
  <PresetShell
    title="Event Groups"
    subtitle="Group your runnable race events. An event sets track, cars, sessions, time/weather and difficulty — everything needed to run."
    icon="events"
    :items="categories"
    :selected-id="selectedId"
    :busy="busy"
    @select="select"
    @create="create"
    @remove="remove"
  >
    <div v-if="loading" class="grid gap-4 md:grid-cols-2">
      <Skeleton v-for="n in 4" :key="n" class="h-56" />
    </div>

    <template v-else-if="selectedId">
      <Card title="Event group" class="mb-4">
        <form class="flex max-w-md items-end gap-2" @submit.prevent="rename">
          <div class="flex-1">
            <FormRow label="Name" for-id="catname" class="!mb-0">
              <Input id="catname" v-model="categoryName" />
            </FormRow>
          </div>
          <Button type="submit" variant="dark" :disabled="busy">Rename</Button>
        </form>
      </Card>

      <p
        v-if="missingPresets.length"
        class="mb-4 flex items-start gap-2 rounded-md border border-accent/40 bg-accent-dim px-3 py-2 text-sm text-text"
      >
        <Icon name="info" :size="16" class="mt-0.5 shrink-0 text-accent" />
        <span>
          Create at least one {{ missingPresets.join(", ") }} preset under
          <RouterLink to="/presets/classes" class="font-semibold text-accent hover:underline">Build</RouterLink>
          before an event can run.
        </span>
      </p>

      <div v-if="events.length" class="mb-4 grid gap-4 md:grid-cols-2">
        <Card v-for="e in events" :key="e.id ?? 0">
          <template #header>
            <Icon name="events" :size="16" class="text-accent" />
            <h2 class="truncate text-sm font-bold">{{ e.track_name }}</h2>
            <span v-if="e.track_config" class="text-xs text-dim">{{ e.track_config }}</span>
          </template>
          <template #actions>
            <Button variant="success" size="sm" @click="queueEvent(e)">
              <Icon name="queue" :size="14" />
              Queue
            </Button>
            <Button variant="dark" size="sm" aria-label="Run repeatedly" title="Run repeatedly on an instance" @click="openRepeat(e)">
              <Icon name="repeat" :size="14" />
            </Button>
            <Button variant="dark" size="sm" @click="openBuilder(e)">Edit</Button>
            <Button variant="ghost" size="sm" aria-label="Duplicate event" @click="duplicateEvent(e)">
              <Icon name="copy" :size="14" />
            </Button>
            <Button variant="ghost" size="sm" aria-label="Delete event" @click="deleteEvent(e)">
              <Icon name="trash" :size="14" />
            </Button>
          </template>

          <img
            :src="`/api/track/preview/${e.track_key}${e.track_config ? '/' + e.track_config : ''}`"
            alt=""
            loading="lazy"
            class="mb-3 aspect-video w-full rounded-sm border border-line object-cover"
          />
          <div class="flex flex-wrap gap-1.5 text-xs">
            <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ e.class_name }} ({{ e.entries }})</span>
            <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ e.session_name }}</span>
            <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ e.time_name }}</span>
            <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ e.difficulty_name }}</span>
            <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">
              {{ e.race_laps ? `${e.race_laps} laps` : "timed race" }}
            </span>
          </div>
        </Card>
      </div>

      <EmptyState
        v-if="!events.length"
        icon="events"
        title="No events in this group yet"
        message="An event picks a track and the presets to run it. Add your first one."
      >
        <Button @click="openBuilder()">
          <Icon name="plus" :size="15" />
          New event
        </Button>
      </EmptyState>
      <Button v-else @click="openBuilder()">
        <Icon name="plus" :size="15" />
        New event
      </Button>
    </template>

    <EmptyState
      v-else
      icon="folder"
      title="Pick an event group"
      message="Event groups organise the events you can queue — like a championship or a casual rotation. Select one on the left, or create a new group."
    />
  </PresetShell>

  <!-- Builder -->
  <Sheet :open="builderOpen" :title="editing?.id ? 'Edit event' : 'New event'" @close="builderOpen = false">
    <template v-if="editing">
      <FormRow label="Track">
        <button
          type="button"
          class="w-full cursor-pointer overflow-hidden rounded-md border border-line bg-surface-2 text-left transition-colors hover:border-line-hi focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          @click="trackPickerOpen = true"
        >
          <img
            v-if="editing.track_key"
            :src="`/api/track/preview/${editing.track_key}${editing.track_config ? '/' + editing.track_config : ''}`"
            alt=""
            class="aspect-video w-full object-cover"
          />
          <div class="p-2 text-sm">
            <template v-if="editing.track_key">
              {{ editing.track_name }}
              <span class="text-xs text-dim">
                {{ editing.track_config || "default" }} · {{ editing.pitboxes }} pits — tap to change
              </span>
            </template>
            <span v-else class="text-muted">Choose a track…</span>
          </div>
        </button>
      </FormRow>

      <FormRow label="Car class">
        <div class="flex gap-1">
          <Select
            v-model="editing.class_id"
            class="flex-1"
            :options="classes.map((c) => ({ value: c.id ?? 0, label: c.name ?? '' }))"
          />
          <Button variant="dark" size="sm" title="Create a new car class" @click="openInline('class')">
            <Icon name="plus" :size="14" />
            New
          </Button>
        </div>
      </FormRow>
      <FormRow label="Sessions">
        <div class="flex gap-1">
          <Select
            v-model="editing.session_id"
            class="flex-1"
            :options="sessions.map((s) => ({ value: s.id ?? 0, label: s.name ?? '' }))"
          />
          <Button variant="dark" size="sm" title="Create a new session preset" @click="openInline('session')">
            <Icon name="plus" :size="14" />
            New
          </Button>
        </div>
      </FormRow>
      <FormRow label="Time & weather">
        <div class="flex gap-1">
          <Select
            v-model="editing.time_id"
            class="flex-1"
            :options="times.map((t) => ({ value: t.id ?? 0, label: t.name ?? '' }))"
          />
          <Button variant="dark" size="sm" title="Create a new time & weather preset" @click="openInline('time')">
            <Icon name="plus" :size="14" />
            New
          </Button>
        </div>
      </FormRow>
      <FormRow label="Difficulty">
        <div class="flex gap-1">
          <Select
            v-model="editing.difficulty_id"
            class="flex-1"
            :options="difficulties.map((d) => ({ value: d.id ?? 0, label: d.name ?? '' }))"
          />
          <Button variant="dark" size="sm" title="Create a new difficulty preset" @click="openInline('difficulty')">
            <Icon name="plus" :size="14" />
            New
          </Button>
        </div>
      </FormRow>

      <div class="grid grid-cols-2 gap-x-4">
        <FormRow label="Race laps" hint="0 = time-based race">
          <Input v-model="editing.race_laps" type="number" :min="0" />
        </FormRow>
        <FormRow label="Overflow strategy" hint="When entries exceed pitboxes">
          <Select
            v-model="editing.strategy"
            :options="[
              { value: 1, label: 'First in list' },
              { value: 2, label: 'Random' },
            ]"
          />
        </FormRow>
      </div>
    </template>

    <template #footer>
      <span v-if="!canSave" class="mr-auto self-center text-xs text-muted">Track and all four presets are required.</span>
      <Button v-if="editing?.id" variant="ghost" :disabled="busy" @click="openPreview">
        <Icon name="content" :size="15" />
        Preview
      </Button>
      <Button variant="ghost" @click="builderOpen = false">Cancel</Button>
      <Button :disabled="busy || !canSave" @click="saveEvent">{{ editing?.id ? "Save event" : "Add event" }}</Button>
    </template>
  </Sheet>

  <!-- Rendered config preview -->
  <Modal :open="previewOpen" title="Rendered config" @close="previewOpen = false">
    <p v-if="previewBusy" class="text-sm text-muted">Rendering…</p>
    <template v-else>
      <p class="mb-3 text-xs text-muted">
        Preview only — entry order may differ at runtime when a random overflow strategy trims the grid.
      </p>
      <h3 class="mb-1 text-xs font-bold tracking-wide text-muted uppercase">server_cfg.ini</h3>
      <pre class="mb-4 max-h-64 overflow-auto rounded-md border border-line bg-bg p-3 font-mono text-xs whitespace-pre-wrap text-muted">{{ previewCfg }}</pre>
      <h3 class="mb-1 text-xs font-bold tracking-wide text-muted uppercase">entry_list.ini</h3>
      <pre class="max-h-64 overflow-auto rounded-md border border-line bg-bg p-3 font-mono text-xs whitespace-pre-wrap text-muted">{{ previewEntry }}</pre>
    </template>
  </Modal>

  <TrackPicker
    :open="trackPickerOpen"
    :selected-key="editing?.track_key"
    :selected-config="editing?.track_config"
    @close="trackPickerOpen = false"
    @select="onTrackPicked"
  />

  <!-- Run repeatedly: pin this event to an instance's repeat mode -->
  <Modal :open="repeatPickerOpen" title="Run repeatedly" @close="repeatPickerOpen = false">
    <p class="mb-3 text-sm text-muted">
      The chosen instance re-runs <span class="font-medium text-text">{{ repeatTarget?.track_name }}</span>
      every time the race finishes. Its manual queue is paused until you switch back.
    </p>
    <FormRow label="Instance" for-id="repeatinst">
      <Select
        id="repeatinst"
        v-model="repeatInstanceId"
        :options="server.instanceList.map((i) => ({ value: i.id, label: i.name + (i.running ? ' (running)' : '') }))"
      />
    </FormRow>
    <p class="text-xs text-muted">
      If the instance is already running, the new event applies on the next restart.
    </p>
    <template #footer>
      <Button variant="ghost" @click="repeatPickerOpen = false">Cancel</Button>
      <Button :disabled="busy || repeatInstanceId === null" @click="confirmRepeat">
        <Icon name="repeat" :size="15" />
        Set repeat
      </Button>
    </template>
  </Modal>

  <!-- Inline preset creation from the builder -->
  <InlinePresetSheet
    :open="inlineOpen"
    :kind="inlineKind"
    @close="inlineOpen = false"
    @created="onPresetCreated"
  />
</template>
