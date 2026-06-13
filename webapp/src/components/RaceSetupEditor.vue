<script setup lang="ts">
// Shared race-setup editor: track, car class, sessions, time/weather,
// difficulty, race length and overflow strategy in one form. Presets are
// accelerators — each has an inline "New" that creates one without leaving
// the editor. Used by the Events library and the Setup Workbench (and reusable
// anywhere else that edits a race setup). The host owns the surrounding
// Sheet/inline layout, footer and save; this component is just the fields.
import { onMounted, ref } from "vue";
import { api } from "@/lib/api";
import { useContentStore } from "@/stores/content";
import type { DropDownList } from "@/types/generated";
import type { RaceSetupDraft } from "@/lib/useRaceSetupDraft";
import type { PresetKind } from "@/lib/presetForms";
import TrackPicker from "@/components/TrackPicker.vue";
import InlinePresetSheet from "@/components/presets/InlinePresetSheet.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Combobox from "@/components/ui/Combobox.vue";
import Icon from "@/components/ui/Icon.vue";

const draft = defineModel<RaceSetupDraft>({ required: true });

const content = useContentStore();
const difficulties = ref<DropDownList[]>([]);
const sessions = ref<DropDownList[]>([]);
const classes = ref<DropDownList[]>([]);
const times = ref<DropDownList[]>([]);

async function loadPresetLists() {
  [difficulties.value, sessions.value, classes.value, times.value] = await Promise.all([
    api.get<{ items: DropDownList[] }>("/api/difficulties?filled=1").then((r) => r.items),
    api.get<{ items: DropDownList[] }>("/api/sessions?filled=1").then((r) => r.items),
    api.get<{ items: DropDownList[] }>("/api/classes?filled=1").then((r) => r.items),
    api.get<{ items: DropDownList[] }>("/api/times?filled=1").then((r) => r.items),
  ]);
}

onMounted(() => {
  void content.load();
  void loadPresetLists();
});

// --- Track picker ---
const trackPickerOpen = ref(false);
function onTrackPicked(track: { key: string; config: string; name: string; pitboxes: number }) {
  draft.value.track_key = track.key;
  draft.value.track_config = track.config;
  draft.value.track_name = track.name;
  draft.value.pitboxes = track.pitboxes;
  trackPickerOpen.value = false;
}

// --- Inline preset creation ---
const inlineOpen = ref(false);
const inlineKind = ref<PresetKind>("class");
const presetFieldByKind: Record<PresetKind, keyof RaceSetupDraft> = {
  class: "class_id",
  session: "session_id",
  time: "time_id",
  difficulty: "difficulty_id",
};

function openInline(kind: PresetKind) {
  inlineKind.value = kind;
  inlineOpen.value = true;
}

async function onPresetCreated(id: number) {
  await loadPresetLists();
  (draft.value[presetFieldByKind[inlineKind.value]] as number | null) = id;
}
</script>

<template>
  <div>
    <FormRow label="Event name" hint="Optional — shown in lists and appended to the lobby name. Defaults to the group name.">
      <Input v-model="draft.name" placeholder="e.g. Round 3 — Night Sprint" maxlength="80" />
    </FormRow>

    <FormRow label="Track">
      <button
        type="button"
        class="w-full cursor-pointer overflow-hidden rounded-md border border-line bg-surface-2 text-left transition-colors hover:border-line-hi focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        @click="trackPickerOpen = true"
      >
        <img
          v-if="draft.track_key"
          :src="`/api/track/preview/${draft.track_key}${draft.track_config ? '/' + draft.track_config : ''}`"
          alt=""
          class="aspect-video w-full object-cover"
        />
        <div class="p-2 text-sm">
          <template v-if="draft.track_key">
            {{ draft.track_name }}
            <span class="text-xs text-dim">
              {{ draft.track_config || "default" }} · {{ draft.pitboxes }} pits — tap to change
            </span>
          </template>
          <span v-else class="text-muted">Choose a track…</span>
        </div>
      </button>
    </FormRow>

    <FormRow label="Car class">
      <div class="flex gap-1">
        <Combobox
          v-model="draft.class_id"
          class="flex-1"
          placeholder="Search car classes…"
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
        <Combobox
          v-model="draft.session_id"
          class="flex-1"
          placeholder="Search session presets…"
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
        <Combobox
          v-model="draft.time_id"
          class="flex-1"
          placeholder="Search time presets…"
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
        <Combobox
          v-model="draft.difficulty_id"
          class="flex-1"
          placeholder="Search difficulty presets…"
          :options="difficulties.map((d) => ({ value: d.id ?? 0, label: d.name ?? '' }))"
        />
        <Button variant="dark" size="sm" title="Create a new difficulty preset" @click="openInline('difficulty')">
          <Icon name="plus" :size="14" />
          New
        </Button>
      </div>
    </FormRow>

    <div class="grid gap-x-4 sm:grid-cols-2">
      <FormRow label="Race laps" hint="0 = time-based race">
        <Input v-model="draft.race_laps" type="number" :min="0" />
      </FormRow>
      <FormRow label="Overflow strategy" hint="When entries exceed pitboxes">
        <Select
          v-model="draft.strategy"
          :options="[
            { value: 1, label: 'First in list' },
            { value: 2, label: 'Random' },
          ]"
        />
      </FormRow>
    </div>

    <TrackPicker
      :open="trackPickerOpen"
      :selected-key="draft.track_key"
      :selected-config="draft.track_config"
      @close="trackPickerOpen = false"
      @select="onTrackPicked"
    />
    <InlinePresetSheet :open="inlineOpen" :kind="inlineKind" @close="inlineOpen = false" @created="onPresetCreated" />
  </div>
</template>
