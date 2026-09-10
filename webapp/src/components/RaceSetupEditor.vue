<script setup lang="ts">
// Shared race-setup editor: track, car class, sessions, time/weather,
// difficulty, race length and overflow strategy in one form. Presets are
// accelerators — each has an inline "New" that creates one without leaving
// the editor. Used by the Events library and the Setup Workbench (and reusable
// anywhere else that edits a race setup). The host owns the surrounding
// Sheet/inline layout, footer and save; this component is just the fields.
import { computed, onMounted, ref, watch } from "vue";
import { api } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { useServerStore } from "@/stores/server";
import { useConfirmStore } from "@/stores/confirm";
import { useContentStore } from "@/stores/content";
import type { DropDownList } from "@/types/generated";
import { normalizeRaceSetup, raceSetupValid, type RaceSetupDraft } from "@/lib/useRaceSetupDraft";
import { previewRaceSetup, type PreviewResult } from "@/lib/useRaceSetupPreview";
import type { PresetKind } from "@/lib/presetForms";
import TrackPicker from "@/components/TrackPicker.vue";
import TrackImage from "@/components/TrackImage.vue";
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
const auth = useAuthStore();
const server = useServerStore();
const confirm = useConfirmStore();
const loadError = ref("");
const templates = ref<RaceSetupDraft[]>([]);
const templateId = ref<number | null>(null);
const loading = ref(false);
const targetName = computed(() => props.instanceId ? server.instances[props.instanceId]?.name : "");
async function loadTemplates() {
  const groups = (await api.get<{ items: DropDownList[] }>("/api/categories")).items;
  const group = groups.find(item => item.name?.toLowerCase() === "templates");
  templates.value = group?.id ? (await api.get<{ events?: Record<string, unknown>[] }>(`/api/category/${group.id}`)).events?.map(normalizeRaceSetup) ?? [] : [];
}
async function loadEditor() {
  loading.value = true;
  loadError.value = "";
  try { await Promise.all([content.load(true), loadPresetLists(), loadTemplates()]); }
  catch (error) { loadError.value = error instanceof Error ? error.message : "Could not load race options."; }
  finally { loading.value = false; }
}
async function applyTemplate() {
  const template = templates.value.find(item => item.id === templateId.value);
  if (!template) return;
  if (draft.value.track_key || draft.value.class_id || draft.value.session_id || draft.value.time_id || draft.value.difficulty_id) {
    if (!await confirm.ask({ title: "Use this template?", message: "Replace the track and preset choices in this draft?", detail: "The race name and saved setup identity stay the same. Shared presets are reused, not changed.", confirmLabel: "Use template", cancelLabel: "Keep current choices" })) return;
  }
  draft.value = { ...template, id: draft.value.id, name: draft.value.name || template.name };
}
const presetGroups = computed(() => [
  { kind: "class" as const, label: "Car class", items: classes.value, field: "class_id" as const, name: "class_name" as const },
  { kind: "session" as const, label: "Sessions", items: sessions.value, field: "session_id" as const, name: "session_name" as const },
  { kind: "time" as const, label: "Time & weather", items: times.value, field: "time_id" as const, name: "time_name" as const },
  { kind: "difficulty" as const, label: "Difficulty", items: difficulties.value, field: "difficulty_id" as const, name: "difficulty_name" as const },
]);
const missingFields = computed(() => [!draft.value.track_key && "Track", ...presetGroups.value.filter(group => !draft.value[group.field]).map(group => group.label)].filter(Boolean));
function presetLabel(group: typeof presetGroups.value[number]) { return group.items.find(item => item.id === draft.value[group.field])?.name || draft.value[group.name] || "Not chosen"; }
function selectPreset(group: typeof presetGroups.value[number], value: string | number | null | undefined) {
  draft.value[group.field] = value == null ? null : Number(value);
  draft.value[group.name] = group.items.find(item => item.id === Number(value))?.name || "";
}
const difficulties = ref<DropDownList[]>([]);
const sessions = ref<DropDownList[]>([]);
const classes = ref<DropDownList[]>([]);
const times = ref<DropDownList[]>([]);
const driftModes = ref<DropDownList[]>([]);

async function loadPresetLists() {
  [difficulties.value, sessions.value, classes.value, times.value, driftModes.value] = await Promise.all([
    api.get<{ items: DropDownList[] }>("/api/difficulties?filled=1").then((r) => r.items),
    api.get<{ items: DropDownList[] }>("/api/sessions?filled=1").then((r) => r.items),
    api.get<{ items: DropDownList[] }>("/api/classes?filled=1").then((r) => r.items),
    api.get<{ items: DropDownList[] }>("/api/times?filled=1").then((r) => r.items),
    api.get<{ items: DropDownList[] }>("/api/drift-scoring-modes?filled=1").then((r) => r.items),
  ]);
}

onMounted(loadEditor);

// --- Track picker ---
const trackPickerOpen = ref(false);
const selectedTrack = computed(() => content.trackByKey(draft.value.track_key, draft.value.track_config));
async function openTrackPicker() {
  try { await content.load(true); trackPickerOpen.value = true; }
  catch (error) { loadError.value = error instanceof Error ? error.message : "Could not load tracks."; }
}

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
  const group = presetGroups.value.find(group => group.kind === inlineKind.value);
  if (group) selectPreset(group, id);
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
  const fingerprint = JSON.stringify([draft.value, props.instanceId]);
  try {
    const result = await previewRaceSetup(draft.value, props.instanceId);
    if (fingerprint === JSON.stringify([draft.value, props.instanceId])) review.value = result;
  } catch (e) {
    reviewError.value = e instanceof Error ? e.message : String(e);
    review.value = null;
  } finally {
    reviewing.value = false;
  }
}
watch(() => JSON.stringify([draft.value, props.instanceId]), () => { review.value = null; reviewError.value = ""; });
</script>


<template>
  <div class="race-editor">
    <div v-if="loadError" role="alert" class="mb-5 rounded-md border border-danger/40 bg-danger-glow p-4 text-sm">
      <p class="font-semibold text-danger">Race options could not be loaded</p><p class="mt-1 text-muted">{{ loadError }}</p>
      <Button variant="dark" class="mt-3" :disabled="loading" @click="loadEditor">{{ loading ? 'Loading…' : 'Retry' }}</Button>
    </div>
    <div class="editor-layout">
      <div class="min-w-0">
        <FormRow v-if="templates.length" label="Start from a saved template" hint="Reuses a track and preset choices. It does not change the original template or start a server.">
          <div class="flex flex-wrap gap-2">
            <Combobox v-model="templateId" class="min-w-40 flex-1" :options="templates.map(item => ({ value: item.id!, label: item.name || item.track_name }))" placeholder="Choose a template…" />
            <Button variant="dark" :disabled="!templateId" @click="applyTemplate">Use template</Button>
          </div>
        </FormRow>
        <FormRow v-slot="{ id, describedBy }" label="Track" required>
          <button :id="id" type="button" :aria-label="draft.track_key ? `Change track: ${draft.track_name}` : 'Choose a track (required)'" :aria-describedby="describedBy" class="flex min-h-20 w-full cursor-pointer items-center gap-4 overflow-hidden rounded-md border border-control bg-surface-2 p-3 text-left hover:bg-surface-3" @click="openTrackPicker">
            <TrackImage v-if="draft.track_key" :track-key="draft.track_key" :config="draft.track_config" class="h-20 w-28 shrink-0 rounded-sm" />
            <span class="min-w-0"><span class="block text-sm font-semibold">{{ draft.track_name || 'Choose a track…' }}</span><span v-if="draft.track_key" class="mt-1 block text-xs text-muted">{{ draft.track_config || 'Default layout' }} · {{ draft.pitboxes ?? '—' }} pitboxes<span v-if="selectedTrack?.version"> · {{ selectedTrack.version }}</span><br>Change track →</span></span>
          </button>
        </FormRow>
        <FormRow label="Race name" hint="Optional. Uses the group name when left blank."><Input v-model="draft.name" placeholder="e.g. Friday night sprint" maxlength="80" /></FormRow>
        <div class="mb-3 flex items-center justify-between gap-3"><h3 class="text-base font-semibold">Race essentials</h3><span class="text-xs text-muted">Reusable presets</span></div>
        <FormRow v-for="group in presetGroups" :key="group.kind" :label="group.label" required>
          <div class="flex gap-2">
            <Combobox :model-value="draft[group.field]" class="flex-1" :placeholder="`Choose ${group.label.toLowerCase()}…`" :options="group.items.map(item => ({ value: item.id ?? 0, label: item.name ?? '' }))" @update:model-value="selectPreset(group, $event)" />
            <Button v-if="auth.isAdmin || (auth.canOperate && group.kind === 'class')" variant="dark" size="sm" :aria-label="`Create ${group.label.toLowerCase()} preset`" @click="openInline(group.kind)"><Icon name="plus" :size="16" /><span class="hidden sm:inline">New</span></Button>
          </div>
        </FormRow>
        <details class="rounded-md border border-line bg-surface-2/40 p-4">
          <summary class="cursor-pointer text-sm font-semibold">Advanced settings</summary>
          <div class="mt-5">
            <FormRow label="Drift scoring"><Combobox v-model="draft.drift_scoring_mode_id" placeholder="Use server default" :options="[{ value: 0, label: 'Use server default' }, ...driftModes.map(item => ({ value: item.id ?? 0, label: item.name ?? '' }))]" /></FormRow>
            <FormRow label="Race laps" hint="0 uses the timed race in the session preset."><Input v-model="draft.race_laps" type="number" :min="0" /></FormRow>
            <FormRow label="Overflow strategy" hint="How to choose entries when the grid exceeds available pitboxes."><Select v-model="draft.strategy" :options="[{ value: 1, label: 'First in list' }, { value: 2, label: 'Random' }]" /></FormRow>
          </div>
        </details>
      </div>
      <aside class="editor-review h-fit rounded-md border border-line bg-surface-2/40 p-5">
        <h3 class="mb-4 text-base font-semibold">What will run</h3>
        <dl class="space-y-3 text-sm">
          <div><dt class="text-xs text-muted">Track</dt><dd class="mt-1 font-medium">{{ draft.track_name || 'Not chosen' }}</dd></div>
          <div v-for="group in presetGroups" :key="group.kind"><dt class="text-xs text-muted">{{ group.label }}</dt><dd class="mt-1">{{ presetLabel(group) }}</dd></div>
          <div><dt class="text-xs text-muted">Race length</dt><dd class="mt-1">{{ draft.race_laps ? `${draft.race_laps} laps` : 'From session preset' }}</dd></div>
          <div><dt class="text-xs text-muted">Preview server</dt><dd class="mt-1">{{ targetName || 'Default server configuration' }}</dd></div>
        </dl>
        <p v-if="missingFields.length" class="mt-5 rounded-md border border-warn/30 bg-warn-glow p-3 text-sm text-warn">Still needed: {{ missingFields.join(', ') }}.</p>
        <p v-else class="mt-5 text-sm text-muted">Choices complete. Check the grid and server limits before saving. Drift scoring is configured separately from this preview.</p>
        <Button class="mt-4 w-full" variant="dark" :disabled="reviewing || !raceSetupValid(draft)" @click="runReview"><Icon name="activity" :size="16" />{{ reviewing ? 'Checking…' : 'Check setup' }}</Button>
        <p v-if="reviewError" role="alert" class="mt-3 text-sm text-danger">{{ reviewError }} Try Check setup again.</p>
        <div v-if="review" class="mt-4 border-t border-line pt-4" role="status">
          <dl class="grid grid-cols-2 gap-3 text-sm"><div><dt class="text-xs text-muted">Grid</dt><dd class="mt-1 font-mono">{{ review.grid_count }} / {{ review.requested_grid }}</dd></div><div><dt class="text-xs text-muted">Pitboxes</dt><dd class="mt-1 font-mono">{{ review.pitboxes }}</dd></div><div><dt class="text-xs text-muted">Max clients</dt><dd class="mt-1 font-mono">{{ review.max_clients }}</dd></div></dl>
          <ul v-if="review.render_errors.length" class="mt-3 space-y-2 text-sm text-danger"><li v-for="error in review.render_errors" :key="error">{{ error }}</li></ul>
          <ul v-if="review.warnings.length" class="mt-3 space-y-2 text-sm text-warn"><li v-for="warning in review.warnings" :key="warning">{{ warning }}</li></ul>
          <p v-if="!review.warnings.length && !review.render_errors.length" class="mt-3 text-sm text-ok">✓ Setup checked · {{ review.grid_count }} cars on the grid.</p>
          <button type="button" class="mt-3 min-h-11 text-sm text-accent underline" :aria-expanded="showIni" @click="showIni = !showIni">{{ showIni ? 'Hide' : 'Show' }} rendered configuration</button>
          <div v-if="showIni" class="space-y-3"><pre class="max-h-48 overflow-auto rounded-md bg-bg p-3 text-xs whitespace-pre-wrap break-all text-muted">{{ review.server_cfg }}</pre><pre class="max-h-48 overflow-auto rounded-md bg-bg p-3 text-xs whitespace-pre-wrap break-all text-muted">{{ review.entry_list }}</pre></div>
        </div>
      </aside>
    </div>
    <TrackPicker :open="trackPickerOpen" :selected-key="draft.track_key" :selected-config="draft.track_config" @close="trackPickerOpen = false" @select="onTrackPicked" />
    <InlinePresetSheet :open="inlineOpen" :kind="inlineKind" @close="inlineOpen = false" @created="onPresetCreated" />
  </div>
</template>

<style scoped>
.race-editor { container-type: inline-size; }
.editor-layout { display: grid; gap: 1.5rem; }
@container (min-width: 650px) {
  .editor-layout { grid-template-columns: minmax(0, 1fr) 280px; align-items: start; }
  .editor-review { position: sticky; top: 0; }
}
</style>
