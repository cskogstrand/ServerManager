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
import { raceSetupValid, type RaceSetupDraft } from "@/lib/useRaceSetupDraft";
import { previewRaceSetup, type PreviewResult } from "@/lib/useRaceSetupPreview";
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
const props = defineProps<{ instanceId?: number | null }>();

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

// --- Review: render the draft and surface grid/pitbox/max-client warnings ---
const review = ref<PreviewResult | null>(null);
const reviewing = ref(false);
const reviewError = ref("");
const showIni = ref(false);

async function runReview() {
  if (!raceSetupValid(draft.value)) return;
  reviewing.value = true;
  reviewError.value = "";
  try {
    review.value = await previewRaceSetup(draft.value, props.instanceId);
  } catch (e) {
    reviewError.value = e instanceof Error ? e.message : String(e);
    review.value = null;
  } finally {
    reviewing.value = false;
  }
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

    <!-- Review: grid vs pitboxes, max clients, warnings, rendered INI -->
    <div class="mt-4 border-t border-line pt-3">
      <div class="flex items-center justify-between gap-2">
        <h3 class="text-xs font-semibold tracking-wide text-muted uppercase">Review</h3>
        <Button variant="dark" size="sm" :disabled="reviewing || !raceSetupValid(draft)" @click="runReview">
          <Icon name="activity" :size="14" />
          {{ reviewing ? "Checking…" : "Check setup" }}
        </Button>
      </div>

      <p v-if="reviewError" class="mt-2 text-xs text-danger">{{ reviewError }}</p>

      <template v-if="review">
        <dl class="mt-2 grid grid-cols-3 gap-2">
          <div class="rounded-md border border-line bg-surface-2/40 px-2.5 py-1.5 text-center">
            <dt class="text-xs text-dim">Grid</dt>
            <dd class="font-mono text-sm" :class="review.pitboxes && review.requested_grid > review.pitboxes ? 'text-warn' : 'text-text'">
              {{ review.grid_count }}<span class="text-dim">/{{ review.pitboxes || "?" }}</span>
            </dd>
          </div>
          <div class="rounded-md border border-line bg-surface-2/40 px-2.5 py-1.5 text-center">
            <dt class="text-xs text-dim">Requested</dt>
            <dd class="font-mono text-sm text-text">{{ review.requested_grid }}</dd>
          </div>
          <div class="rounded-md border border-line bg-surface-2/40 px-2.5 py-1.5 text-center">
            <dt class="text-xs text-dim">Max clients</dt>
            <dd class="font-mono text-sm text-text">{{ review.max_clients }}</dd>
          </div>
        </dl>

        <ul v-if="review.render_errors.length" class="mt-2 space-y-1">
          <li v-for="(e, i) in review.render_errors" :key="`e${i}`" class="flex items-start gap-1.5 text-xs text-danger">
            <Icon name="alert" :size="13" class="mt-0.5 shrink-0" />
            {{ e }}
          </li>
        </ul>
        <ul v-if="review.warnings.length" class="mt-2 space-y-1">
          <li v-for="(w, i) in review.warnings" :key="`w${i}`" class="flex items-start gap-1.5 text-xs text-warn">
            <Icon name="alert" :size="13" class="mt-0.5 shrink-0" />
            {{ w }}
          </li>
        </ul>
        <p
          v-if="!review.warnings.length && !review.render_errors.length"
          class="mt-2 flex items-center gap-1.5 text-xs text-ok"
        >
          <Icon name="check" :size="13" />
          Renders cleanly — {{ review.grid_count }} car{{ review.grid_count === 1 ? "" : "s" }} on the grid.
        </p>

        <button
          type="button"
          class="mt-2 inline-flex items-center gap-1.5 text-xs font-semibold text-muted hover:text-text"
          @click="showIni = !showIni"
        >
          <Icon name="content" :size="13" />
          {{ showIni ? "Hide" : "Show" }} rendered config
        </button>
        <div v-if="showIni" class="mt-2 space-y-2">
          <pre class="max-h-48 overflow-auto rounded-md border border-line bg-bg p-2.5 font-mono text-xs whitespace-pre-wrap text-muted">{{ review.server_cfg }}</pre>
          <pre class="max-h-48 overflow-auto rounded-md border border-line bg-bg p-2.5 font-mono text-xs whitespace-pre-wrap text-muted">{{ review.entry_list }}</pre>
        </div>
      </template>
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
