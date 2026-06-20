<script setup lang="ts">
// Race Setup library: every runnable race setup across all groups in one
// searchable, filterable grid. Groups are an organisation filter, not a gate —
// you can create a setup from anywhere. The shared RaceSetupEditor drives
// create/edit; presets are accelerators chosen or created inline.
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useQueryParam, enumParam } from "@/lib/useQueryParam";
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
import PageHeader from "@/components/ui/PageHeader.vue";
import RaceSetupEditor from "@/components/RaceSetupEditor.vue";
import TrackImage from "@/components/TrackImage.vue";
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

type LibrarySetup = RaceSetupDraft & { group_id: number; group_name: string };

const server = useServerStore();
const toast = useToastStore();
const confirm = useConfirmStore();

const groups = ref<DropDownList[]>([]);
const setups = ref<LibrarySetup[]>([]);
const loading = ref(true);
const busy = ref(false);

// --- Filters --- (mirrored to the URL so refresh/back restores them)
const search = useQueryParam("q", "");
const groupFilter = useQueryParam<number | "all">("group", "all", {
  parse: (r) => {
    const n = Number(r);
    return r !== "" && Number.isFinite(n) ? n : "all";
  },
  serialize: (v) => (v === "all" ? null : String(v)),
});
const runFilter = useQueryParam<"all" | "repeating">("run", "all", enumParam(["all", "repeating"] as const, "all"));

// Events pinned as an instance's repeat event (from the live store).
const repeatingIds = computed(
  () => new Set(server.instanceList.map((i) => i.repeat_event_id).filter((id): id is number => id != null)),
);
function isRepeating(s: LibrarySetup): boolean {
  return s.id != null && repeatingIds.value.has(s.id);
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase();
  return setups.value.filter((s) => {
    if (groupFilter.value !== "all" && s.group_id !== groupFilter.value) return false;
    if (runFilter.value === "repeating" && !isRepeating(s)) return false;
    return !q || `${s.name} ${s.track_name} ${s.class_name} ${s.group_name}`.toLowerCase().includes(q);
  });
});

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

// Load every group and flatten its events into one library array.
async function loadAll() {
  groups.value = (await api.get<{ items: DropDownList[] }>("/api/categories")).items;
  const cats = await Promise.all(
    groups.value.map((g) =>
      api.get<{ name?: string; events?: Record<string, unknown>[] }>(`/api/category/${g.id}`).then((c) => ({ g, c })),
    ),
  );
  setups.value = cats.flatMap(({ g, c }) =>
    (c.events ?? [])
      .filter((e) => e.Id != null)
      .map((e) => ({ ...normalizeRaceSetup(e), group_id: g.id ?? 0, group_name: g.name ?? "" })),
  );
}

// --- Builder ---
const builderOpen = ref(false);
const editing = ref<RaceSetupDraft | null>(null);
const editingGroupId = ref<number | null>(null);

let builderBaseline = "";
const markBuilderClean = () => (builderBaseline = editing.value ? JSON.stringify(editing.value) : "");
useUnsavedGuard(() => builderOpen.value && editing.value !== null && JSON.stringify(editing.value) !== builderBaseline);

async function ensureGroup(): Promise<number> {
  if (groupFilter.value !== "all") return groupFilter.value;
  if (groups.value.length) return groups.value[0].id!;
  const { id } = await api.post<{ id: number }>("/api/categories", { name: "Race setups" });
  await loadAll();
  return id;
}

const openCreate = () =>
  guard(async () => {
    editingGroupId.value = await ensureGroup();
    editing.value = emptyRaceSetup();
    markBuilderClean();
    builderOpen.value = true;
  });

function openEdit(s: LibrarySetup) {
  editingGroupId.value = s.group_id;
  editing.value = { ...s };
  markBuilderClean();
  builderOpen.value = true;
}

const saveSetup = () =>
  guard(async () => {
    const e = editing.value;
    if (!e || editingGroupId.value === null) return;
    if (!raceSetupValid(e)) {
      toast.error("Track and all four presets are required.");
      return;
    }
    const body = raceSetupBody(e, editingGroupId.value);
    if (e.id) await api.put(`/api/event/${e.id}`, body);
    else await api.post("/api/events", body);
    builderOpen.value = false;
    await loadAll();
    toast.success(e.id ? "Race setup saved." : "Race setup added.");
  });

const deleteSetup = (s: LibrarySetup) =>
  guard(async () => {
    if (!s.id) return;
    const ok = await confirm.ask({
      title: "Delete race setup",
      message: `Delete the ${s.name || s.track_name} setup?`,
      detail: "Its queue entries are removed too. This cannot be undone.",
      confirmLabel: "Delete race setup",
      tone: "danger",
    });
    if (!ok) return;
    await api.delete(`/api/event/${s.id}`);
    await loadAll();
    toast.success("Race setup deleted.");
  });

const duplicateSetup = (s: LibrarySetup) =>
  guard(async () => {
    if (!s.id) return;
    await api.post("/api/events", raceSetupBody({ ...s, name: s.name ? `${s.name} (copy)` : "" }, s.group_id));
    await loadAll();
    toast.success("Race setup duplicated.");
  });

// --- Instance action picker (queue / start / repeat) ---
const actionOpen = ref(false);
const actionTarget = ref<LibrarySetup | null>(null);
const actionInstanceId = ref<number | null>(null);
const actionKind = ref<"queue" | "start" | "repeat">("queue");

function openAction(s: LibrarySetup, kind: "queue" | "start" | "repeat") {
  actionTarget.value = s;
  actionKind.value = kind;
  actionInstanceId.value = server.instanceList[0]?.id ?? null;
  actionOpen.value = true;
}

const confirmAction = () =>
  guard(async () => {
    const s = actionTarget.value;
    const iid = actionInstanceId.value;
    if (!s?.id || iid === null) return;
    const inst = server.instanceList.find((i) => i.id === iid);
    if (actionKind.value === "repeat") {
      await server.setRunMode(iid, "repeat_event", s.id);
      toast.success(`${inst?.name ?? "Instance"} will repeat ${s.name || s.track_name}.`);
    } else {
      await api.post(`/api/queue/event/${s.id}?instance=${iid}`);
      if (actionKind.value === "start") {
        await api.post(`/api/server/start?instance=${iid}`);
        toast.success(`Queued and starting on ${inst?.name ?? "instance"}.`);
      } else {
        toast.success(`Queued on ${inst?.name ?? "instance"}.`);
      }
    }
    actionOpen.value = false;
    await loadAll();
  });

// --- Group management ---
const groupModalOpen = ref(false);
const newGroupName = ref("");

const createGroup = () =>
  guard(async () => {
    const name = newGroupName.value.trim();
    if (!name) return;
    const { id } = await api.post<{ id: number }>("/api/categories", { name });
    newGroupName.value = "";
    groupModalOpen.value = false;
    await loadAll();
    groupFilter.value = id;
    toast.success("Group created.");
  });

const renameGroup = () =>
  guard(async () => {
    if (groupFilter.value === "all") return;
    const g = groups.value.find((x) => x.id === groupFilter.value);
    const name = window.prompt("Rename group", g?.name ?? "");
    if (!name?.trim()) return;
    await api.patch(`/api/category/${groupFilter.value}`, { name: name.trim() });
    await loadAll();
    toast.success("Group renamed.");
  });

const deleteGroup = () =>
  guard(async () => {
    if (groupFilter.value === "all") return;
    const g = groups.value.find((x) => x.id === groupFilter.value);
    const ok = await confirm.ask({
      title: "Delete group",
      message: `Delete "${g?.name ?? "this group"}" and all its race setups?`,
      detail: "Queue entries for those setups are removed too. This cannot be undone.",
      confirmLabel: "Delete group",
      tone: "danger",
    });
    if (!ok) return;
    await api.delete(`/api/category/${groupFilter.value}`);
    groupFilter.value = "all";
    await loadAll();
    toast.success("Group deleted.");
  });

onMounted(() =>
  guard(async () => {
    await Promise.all([loadAll(), server.load()]);
    loading.value = false;
  }),
);
</script>

<template>
  <PageHeader
    title="Race Setups"
    subtitle="Every runnable race setup — a track plus presets — in one library. Queue, start, or repeat any of them."
    icon="events"
  >
    <template #actions>
      <Button :disabled="busy" @click="openCreate">
        <Icon name="plus" :size="15" />
        New race setup
      </Button>
    </template>
  </PageHeader>

  <!-- Toolbar -->
  <div class="mb-4 flex flex-wrap items-center gap-2">
    <div class="relative min-w-48 flex-1">
      <Icon name="search" :size="15" class="absolute top-1/2 left-2.5 -translate-y-1/2 text-dim" />
      <Input v-model="search" placeholder="Search setups…" class="!pl-8" />
    </div>
    <Select
      v-model="groupFilter"
      class="w-44"
      :options="[{ value: 'all', label: 'All groups' }, ...groups.map((g) => ({ value: g.id ?? 0, label: g.name ?? '' }))]"
    />
    <div class="inline-flex overflow-hidden rounded-md border border-line">
      <button
        type="button"
        class="min-h-9 px-3 text-xs font-semibold transition-colors"
        :class="runFilter === 'all' ? 'bg-accent-dim text-accent' : 'bg-surface text-muted hover:text-text'"
        @click="runFilter = 'all'"
      >
        All
      </button>
      <button
        type="button"
        class="min-h-9 border-l border-line px-3 text-xs font-semibold transition-colors"
        :class="runFilter === 'repeating' ? 'bg-accent-dim text-accent' : 'bg-surface text-muted hover:text-text'"
        @click="runFilter = 'repeating'"
      >
        Repeating
      </button>
    </div>
    <Button variant="dark" size="sm" @click="groupModalOpen = true">
      <Icon name="folder" :size="14" />
      New group
    </Button>
    <template v-if="groupFilter !== 'all'">
      <Button variant="ghost" size="sm" aria-label="Rename group" @click="renameGroup">
        <Icon name="edit" :size="14" />
      </Button>
      <Button variant="ghost" size="sm" aria-label="Delete group" @click="deleteGroup">
        <Icon name="trash" :size="14" />
      </Button>
    </template>
  </div>

  <div v-if="loading" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
    <Skeleton v-for="n in 6" :key="n" class="h-64" />
  </div>

  <div v-else-if="filtered.length" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
    <Card v-for="s in filtered" :key="s.id ?? 0">
      <template #header>
        <Icon name="events" :size="16" class="text-accent" />
        <h2 class="min-w-0 truncate text-sm font-bold">{{ s.name || s.track_name }}</h2>
        <span
          v-if="isRepeating(s)"
          class="inline-flex items-center gap-1 rounded-full border border-accent/40 bg-accent-dim px-2 py-0.5 text-xs text-accent"
        >
          <Icon name="repeat" :size="11" />
          Repeating
        </span>
      </template>

      <TrackImage :track-key="s.track_key" :config="s.track_config" class="mb-2 aspect-video w-full rounded-sm border border-line" />
      <div class="mb-2 flex items-center justify-between gap-2 text-xs">
        <span class="min-w-0 truncate text-dim">{{ s.name ? s.track_name + " · " : "" }}{{ s.group_name }}</span>
        <span
          class="shrink-0 font-mono"
          :class="s.entries != null && s.pitboxes != null && s.entries > s.pitboxes ? 'text-warn' : 'text-muted'"
        >
          {{ s.entries ?? "?" }}/{{ s.pitboxes ?? "?" }} grid
        </span>
      </div>
      <div class="flex flex-wrap gap-1.5 text-xs">
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ s.class_name }}</span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ s.session_name }}</span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ s.time_name }}</span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">{{ s.difficulty_name }}</span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5">
          {{ s.race_laps ? `${s.race_laps} laps` : "timed" }}
        </span>
      </div>

      <div class="mt-3 flex flex-wrap gap-1.5">
        <Button variant="success" size="sm" @click="openAction(s, 'start')">
          <Icon name="power" :size="14" />
          Start
        </Button>
        <Button variant="dark" size="sm" @click="openAction(s, 'queue')">
          <Icon name="queue" :size="14" />
          Queue
        </Button>
        <Button variant="dark" size="sm" aria-label="Repeat on instance" title="Run repeatedly" @click="openAction(s, 'repeat')">
          <Icon name="repeat" :size="14" />
        </Button>
        <Button variant="ghost" size="sm" @click="openEdit(s)">Edit</Button>
        <Button variant="ghost" size="sm" aria-label="Duplicate" @click="duplicateSetup(s)">
          <Icon name="copy" :size="14" />
        </Button>
        <Button variant="ghost" size="sm" aria-label="Delete" @click="deleteSetup(s)">
          <Icon name="trash" :size="14" />
        </Button>
      </div>
    </Card>
  </div>

  <EmptyState
    v-else
    icon="events"
    :title="search || groupFilter !== 'all' || runFilter !== 'all' ? 'No setups match' : 'No race setups yet'"
    :message="
      search || groupFilter !== 'all' || runFilter !== 'all'
        ? 'Try clearing the search or filters.'
        : 'A race setup bundles a track and presets into one runnable race. Add your first one.'
    "
  >
    <Button :disabled="busy" @click="openCreate">
      <Icon name="plus" :size="15" />
      New race setup
    </Button>
  </EmptyState>

  <!-- Builder -->
  <Sheet :open="builderOpen" :title="editing?.id ? 'Edit race setup' : 'New race setup'" @close="builderOpen = false">
    <FormRow v-if="editing" label="Group" hint="Which group this setup belongs to.">
      <Select
        v-model="editingGroupId"
        :options="groups.map((g) => ({ value: g.id ?? 0, label: g.name ?? '' }))"
      />
    </FormRow>
    <RaceSetupEditor v-if="editing" v-model="editing" :instance-id="server.instanceList[0]?.id ?? null" />
    <template #footer>
      <span v-if="!raceSetupValid(editing)" class="mr-auto self-center text-xs text-muted">Track and all four presets are required.</span>
      <Button variant="ghost" @click="builderOpen = false">Cancel</Button>
      <Button :disabled="busy || !raceSetupValid(editing)" @click="saveSetup">{{ editing?.id ? "Save race setup" : "Add race setup" }}</Button>
    </template>
  </Sheet>

  <!-- Instance action picker -->
  <Modal
    :open="actionOpen"
    :title="actionKind === 'repeat' ? 'Run repeatedly' : actionKind === 'start' ? 'Start on instance' : 'Queue on instance'"
    @close="actionOpen = false"
  >
    <p class="mb-3 text-sm text-muted">
      <span class="font-medium text-text">{{ actionTarget?.name || actionTarget?.track_name }}</span>
      <template v-if="actionKind === 'repeat'"> will re-run every time the race finishes (manual queue paused).</template>
      <template v-else-if="actionKind === 'start'"> will be queued and the server started.</template>
      <template v-else> will be added to the instance's queue.</template>
    </p>
    <FormRow label="Instance" for-id="actinst">
      <Select
        id="actinst"
        v-model="actionInstanceId"
        :options="server.instanceList.map((i) => ({ value: i.id, label: i.name + (i.running ? ' (running)' : '') }))"
      />
    </FormRow>
    <template #footer>
      <Button variant="ghost" @click="actionOpen = false">Cancel</Button>
      <Button :disabled="busy || actionInstanceId === null" @click="confirmAction">
        {{ actionKind === "repeat" ? "Set repeat" : actionKind === "start" ? "Start" : "Queue" }}
      </Button>
    </template>
  </Modal>

  <!-- New group -->
  <Modal :open="groupModalOpen" title="New group" @close="groupModalOpen = false">
    <FormRow label="Group name" for-id="grpname" hint="Organise setups — e.g. a championship or a casual rotation.">
      <Input id="grpname" v-model="newGroupName" @keyup.enter="createGroup" />
    </FormRow>
    <template #footer>
      <Button variant="ghost" @click="groupModalOpen = false">Cancel</Button>
      <Button :disabled="busy || !newGroupName.trim()" @click="createGroup">Create group</Button>
    </template>
  </Modal>
</template>
