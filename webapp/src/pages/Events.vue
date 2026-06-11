<script setup lang="ts">
// Event Builder: categories on the left, events as cards, builder in a side
// sheet. Presets are picked from dropdowns (filled ones only); each select
// links to its editor for creating new presets.
import { onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useServerStore } from "@/stores/server";
import type { DropDownList } from "@/types/generated";
import PresetShell from "@/components/presets/PresetShell.vue";
import PresetNotices from "@/components/presets/PresetNotices.vue";
import TrackPicker from "@/components/TrackPicker.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Sheet from "@/components/ui/Sheet.vue";
import Icon from "@/components/ui/Icon.vue";

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

const categories = ref<DropDownList[]>([]);
const selectedId = ref<number | null>(null);
const categoryName = ref("");
const events = ref<EventRow[]>([]);
const busy = ref(false);
const notice = ref("");
const error = ref("");

// Preset dropdown options (only "filled" presets render valid configs)
const difficulties = ref<DropDownList[]>([]);
const sessions = ref<DropDownList[]>([]);
const classes = ref<DropDownList[]>([]);
const times = ref<DropDownList[]>([]);

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
  notice.value = "";
  error.value = "";
  try {
    await fn();
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
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
    if (!window.confirm("Delete this category and all its events? Queue entries are removed too.")) return;
    await api.delete(`/api/category/${id}`);
    if (selectedId.value === id) {
      selectedId.value = null;
      events.value = [];
    }
    await loadCategories();
  });

const rename = () =>
  guard(async () => {
    if (!selectedId.value) return;
    await api.patch(`/api/category/${selectedId.value}`, { name: categoryName.value });
    await loadCategories();
    notice.value = "Category renamed.";
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
      error.value = "Pick a track first.";
      return;
    }
    if (!e.difficulty_id || !e.session_id || !e.class_id || !e.time_id) {
      error.value = "Difficulty, sessions, class and time are all required.";
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
  });

const deleteEvent = (e: EventRow) =>
  guard(async () => {
    if (!e.id || !selectedId.value) return;
    if (!window.confirm("Delete this event? Its queue entries are removed too.")) return;
    await api.delete(`/api/event/${e.id}`);
    await select(selectedId.value);
  });

const queueEvent = (e: EventRow) =>
  guard(async () => {
    if (!e.id) return;
    const inst = server.instanceList[0];
    await api.post(`/api/queue/event/${e.id}${inst ? `?instance=${inst.id}` : ""}`);
    notice.value = `Queued ${e.track_name} on ${inst?.name ?? "the default instance"}.`;
  });

onMounted(() =>
  guard(async () => {
    await Promise.all([loadCategories(), loadPresetLists(), server.load()]);
  }),
);
</script>

<template>
  <PresetShell
    title="Events"
    subtitle="Build race events from track, class, session, time/weather, and difficulty presets."
    icon="events"
    :items="categories"
    :selected-id="selectedId"
    :busy="busy"
    @select="select"
    @create="create"
    @remove="remove"
  >
    <PresetNotices :notice="notice" :error="error" />

    <template v-if="selectedId">
      <Card title="Category" class="mb-4">
        <form class="flex max-w-md items-end gap-2" @submit.prevent="rename">
          <div class="flex-1">
            <FormRow label="Name" for-id="catname" class="!mb-0">
              <Input id="catname" v-model="categoryName" />
            </FormRow>
          </div>
          <Button type="submit" variant="dark" :disabled="busy">Rename</Button>
        </form>
      </Card>

      <div class="mb-4 grid gap-4 md:grid-cols-2">
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
            <Button variant="dark" size="sm" @click="openBuilder(e)">Edit</Button>
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

      <Button @click="openBuilder()">
        <Icon name="plus" :size="15" />
        Add event
      </Button>
    </template>

    <p v-else class="text-muted">
      Select an event category or create one — a category groups the events you can queue.
    </p>
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

      <FormRow label="Car class" hint="Create new ones under Car Classes">
        <Select
          v-model="editing.class_id"
          :options="classes.map((c) => ({ value: c.id ?? 0, label: c.name ?? '' }))"
        />
      </FormRow>
      <FormRow label="Sessions">
        <Select
          v-model="editing.session_id"
          :options="sessions.map((s) => ({ value: s.id ?? 0, label: s.name ?? '' }))"
        />
      </FormRow>
      <FormRow label="Time & weather">
        <Select
          v-model="editing.time_id"
          :options="times.map((t) => ({ value: t.id ?? 0, label: t.name ?? '' }))"
        />
      </FormRow>
      <FormRow label="Difficulty">
        <Select
          v-model="editing.difficulty_id"
          :options="difficulties.map((d) => ({ value: d.id ?? 0, label: d.name ?? '' }))"
        />
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
      <Button variant="ghost" @click="builderOpen = false">Cancel</Button>
      <Button :disabled="busy" @click="saveEvent">{{ editing?.id ? "Save event" : "Add event" }}</Button>
    </template>
  </Sheet>

  <TrackPicker
    :open="trackPickerOpen"
    :selected-key="editing?.track_key"
    :selected-config="editing?.track_config"
    @close="trackPickerOpen = false"
    @select="onTrackPicked"
  />
</template>
