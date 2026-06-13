<script setup lang="ts">
// Race setup library: groups on the left, race setups as cards, the shared
// RaceSetupEditor in a side sheet. Presets are accelerators picked (or created
// inline) from the editor, not a required page visit.
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import {
  emptyRaceSetup,
  normalizeRaceSetup,
  raceSetupBody,
  raceSetupValid,
  type RaceSetupDraft,
} from "@/lib/useRaceSetupDraft";
import { useServerStore } from "@/stores/server";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import type { DropDownList } from "@/types/generated";
import PresetShell from "@/components/presets/PresetShell.vue";
import RaceSetupEditor from "@/components/RaceSetupEditor.vue";
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

const server = useServerStore();
const toast = useToastStore();
const confirm = useConfirmStore();

const categories = ref<DropDownList[]>([]);
const selectedId = ref<number | null>(null);
const categoryName = ref("");
const events = ref<RaceSetupDraft[]>([]);
const busy = ref(false);
const loading = ref(true);

const canSave = computed(() => raceSetupValid(editing.value));

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

const select = (id: number) =>
  guard(async () => {
    const cat = await api.get<{ name?: string; events?: Record<string, unknown>[] }>(`/api/category/${id}`);
    selectedId.value = id;
    categoryName.value = cat.name ?? "";
    events.value = (cat.events ?? []).filter((e) => e.Id != null).map(normalizeRaceSetup);
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

const duplicateGroup = (id: number) =>
  guard(async () => {
    const { id: newId } = await api.post<{ id: number }>(`/api/category/${id}/duplicate`);
    await loadCategories();
    await select(newId);
    toast.success("Event group duplicated.");
  });

// --- Builder sheet ---
const builderOpen = ref(false);
const editing = ref<RaceSetupDraft | null>(null);

// Warn before navigating away while the builder holds unsaved edits.
let builderBaseline = "";
const markBuilderClean = () => (builderBaseline = editing.value ? JSON.stringify(editing.value) : "");
useUnsavedGuard(() => builderOpen.value && editing.value !== null && JSON.stringify(editing.value) !== builderBaseline);

function openBuilder(event?: RaceSetupDraft) {
  editing.value = event ? { ...event } : emptyRaceSetup();
  markBuilderClean();
  builderOpen.value = true;
}

// Start a race setup without first picking a group: reuse the selected group,
// else the first existing one, else create a default group on the fly.
const newRaceSetupQuick = () =>
  guard(async () => {
    if (selectedId.value === null) {
      if (categories.value.length) {
        await select(categories.value[0].id!);
      } else {
        const { id } = await api.post<{ id: number }>("/api/categories", { name: "Race setups" });
        await loadCategories();
        selectedId.value = id;
        categoryName.value = "Race setups";
        events.value = [];
      }
    }
    openBuilder();
  });

const saveEvent = () =>
  guard(async () => {
    const e = editing.value;
    if (!e || !selectedId.value) return;
    if (!raceSetupValid(e)) {
      toast.error("Track and all four presets are required.");
      return;
    }
    const body = raceSetupBody(e, selectedId.value);
    if (e.id) {
      await api.put(`/api/event/${e.id}`, body);
    } else {
      await api.post("/api/events", body);
    }
    builderOpen.value = false;
    await select(selectedId.value);
    toast.success(e.id ? "Race setup saved." : "Race setup added.");
  });

const deleteEvent = (e: RaceSetupDraft) =>
  guard(async () => {
    if (!e.id || !selectedId.value) return;
    const ok = await confirm.ask({
      title: "Delete race setup",
      message: `Delete the ${e.name || e.track_name} setup?`,
      detail: "Its queue entries are removed too. This cannot be undone.",
      confirmLabel: "Delete race setup",
      tone: "danger",
    });
    if (!ok) return;
    await api.delete(`/api/event/${e.id}`);
    await select(selectedId.value);
    toast.success("Race setup deleted.");
  });

// Duplicate: clone the row into the same group, then re-select so the new card
// appears. Saves rebuilding near-identical setups by hand.
const duplicateEvent = (e: RaceSetupDraft) =>
  guard(async () => {
    if (!e.id || !selectedId.value) return;
    const body = raceSetupBody({ ...e, name: e.name ? `${e.name} (copy)` : "" }, selectedId.value);
    await api.post("/api/events", body);
    await select(selectedId.value);
    toast.success("Race setup duplicated.");
  });

const queueEvent = (e: RaceSetupDraft) =>
  guard(async () => {
    if (!e.id) return;
    const inst = server.instanceList[0];
    await api.post(`/api/queue/event/${e.id}${inst ? `?instance=${inst.id}` : ""}`);
    toast.success(`Queued ${e.name || e.track_name} on ${inst?.name ?? "the default instance"}.`);
  });

// --- Repeat-on-instance picker ---
const repeatPickerOpen = ref(false);
const repeatTarget = ref<RaceSetupDraft | null>(null);
const repeatInstanceId = ref<number | null>(null);

function openRepeat(e: RaceSetupDraft) {
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

onMounted(() =>
  guard(async () => {
    await Promise.all([loadCategories(), server.load()]);
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
    duplicatable
    @select="select"
    @create="create"
    @remove="remove"
    @duplicate="duplicateGroup"
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

      <div v-if="events.length" class="mb-4 grid gap-4 md:grid-cols-2">
        <Card v-for="e in events" :key="e.id ?? 0">
          <template #header>
            <Icon name="events" :size="16" class="text-accent" />
            <h2 class="min-w-0 truncate text-sm font-bold">{{ e.name || e.track_name }}</h2>
            <span v-if="e.name" class="min-w-0 truncate text-xs text-muted">{{ e.track_name }}</span>
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
        title="No race setups in this group yet"
        message="A race setup picks a track and the presets to run it. Add your first one."
      >
        <Button @click="openBuilder()">
          <Icon name="plus" :size="15" />
          New race setup
        </Button>
      </EmptyState>
      <Button v-else @click="openBuilder()">
        <Icon name="plus" :size="15" />
        New race setup
      </Button>
    </template>

    <EmptyState
      v-else
      icon="events"
      title="Build a race setup"
      message="Race setups bundle a track and presets into one runnable race. Start one now — it lands in a group you can rename later — or pick a group on the left."
    >
      <Button :disabled="busy" @click="newRaceSetupQuick">
        <Icon name="plus" :size="15" />
        New race setup
      </Button>
    </EmptyState>
  </PresetShell>

  <!-- Builder -->
  <Sheet :open="builderOpen" :title="editing?.id ? 'Edit race setup' : 'New race setup'" @close="builderOpen = false">
    <RaceSetupEditor v-if="editing" v-model="editing" />

    <template #footer>
      <span v-if="!canSave" class="mr-auto self-center text-xs text-muted">Track and all four presets are required.</span>
      <Button v-if="editing?.id" variant="ghost" :disabled="busy" @click="openPreview">
        <Icon name="content" :size="15" />
        Preview
      </Button>
      <Button variant="ghost" @click="builderOpen = false">Cancel</Button>
      <Button :disabled="busy || !canSave" @click="saveEvent">{{ editing?.id ? "Save race setup" : "Add race setup" }}</Button>
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

</template>
