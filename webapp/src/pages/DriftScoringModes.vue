<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { restartServersUsingTemplate } from "@/lib/restartServersForTemplate";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import { useConfirmStore } from "@/stores/confirm";
import type { DriftScoringMode, DropDownList } from "@/types/generated";
import PageHeader from "@/components/ui/PageHeader.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Icon from "@/components/ui/Icon.vue";
import Input from "@/components/ui/Input.vue";
import Slider from "@/components/ui/Slider.vue";
import Toggle from "@/components/ui/Toggle.vue";

type ModeFlag =
  | "collision_reset_score"
  | "car_collision_reset_score"
  | "reset_score_enabled"
  | "reset_multiplier_enabled";

const confirm = useConfirmStore();
const items = ref<DropDownList[]>([]);
const usage = ref<Record<string, number>>({});
const selectedId = ref<number | null>(null);
const form = ref<DriftScoringMode | null>(null);
const busy = ref(false);
const notice = ref("");
const error = ref("");
const newName = ref("");

let baseline = "";
const snapshot = () => (form.value ? JSON.stringify(form.value) : "");
const markClean = () => (baseline = snapshot());
useUnsavedGuard(() => form.value !== null && snapshot() !== baseline);

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

async function reloadList() {
  const res = await api.get<{ items: DropDownList[]; usage: Record<string, number> }>("/api/drift-scoring-modes");
  items.value = res.items ?? [];
  usage.value = res.usage ?? {};
}

function flag(key: ModeFlag) {
  return computed({
    get: () => form.value?.[key] === 1,
    set: (v: boolean) => {
      if (form.value) form.value[key] = v ? 1 : 0;
    },
  });
}

const collisionReset = flag("collision_reset_score");
const carCollisionReset = flag("car_collision_reset_score");
const resetScore = flag("reset_score_enabled");
const resetMultiplier = flag("reset_multiplier_enabled");

const selectedUsage = computed(() => (selectedId.value == null ? 0 : (usage.value[String(selectedId.value)] ?? 0)));
const driftFpsReference = 60;
const multiplierGainReferenceKmh = 100;

function gainToSeconds(gain?: number | null) {
  return gain && gain > 0 ? 1 / (multiplierGainReferenceKmh * gain * driftFpsReference) : 12;
}

const multiplierGainSeconds = computed({
  get: () => Number(Math.min(12, Math.max(0.5, gainToSeconds(form.value?.multiplier_gain))).toFixed(1)),
  set: (seconds: number | null | undefined) => {
    if (!form.value || !seconds || seconds <= 0) return;
    form.value.multiplier_gain = Number((1 / (multiplierGainReferenceKmh * seconds * driftFpsReference)).toFixed(8));
  },
});

const multiplierGainHint = computed(() => {
  const seconds = gainToSeconds(form.value?.multiplier_gain);
  return `At ${multiplierGainReferenceKmh} km/h, +1x multiplier every ${seconds.toFixed(1)} seconds.`;
});

const select = (id: number) =>
  guard(async () => {
    const res = await api.get<{ data: DriftScoringMode }>(`/api/drift-scoring-mode/${id}`);
    form.value = res.data;
    selectedId.value = id;
    markClean();
  });

const create = () =>
  guard(async () => {
    const name = newName.value.trim();
    if (!name) return;
    const res = await api.post<{ id: number }>("/api/drift-scoring-modes", { name });
    newName.value = "";
    await reloadList();
    await select(res.id);
  });

const save = () =>
  guard(async () => {
    if (!form.value || selectedId.value == null) return;
    await api.put(`/api/drift-scoring-mode/${selectedId.value}`, form.value);
    await reloadList();
    markClean();
    const restarted = await restartServersUsingTemplate("drift-scoring-modes", selectedId.value);
    notice.value = restarted ? "Saved and restarted." : "Saved.";
  });

const remove = (id: number) =>
  guard(async () => {
    const ok = await confirm.ask({
      title: "Delete scoring mode",
      message: "Delete this drift scoring mode?",
      detail: "Servers or race setups using it will block the delete.",
      confirmLabel: "Delete mode",
      tone: "danger",
    });
    if (!ok) return;
    await api.delete(`/api/drift-scoring-mode/${id}`);
    if (selectedId.value === id) {
      selectedId.value = null;
      form.value = null;
    }
    await reloadList();
  });

onMounted(() =>
  guard(async () => {
    await reloadList();
    if (items.value[0]?.id) await select(items.value[0].id);
  }),
);
</script>

<template>
  <PageHeader
    title="Drift Scoring"
    subtitle="Create and maintain reusable drift scoring modes."
    icon="gauge"
  >
    <template #prefix>
      <RouterLink to="/presets">
        <Button variant="ghost" size="sm">
          <Icon name="arrowLeft" :size="14" />
          Back to templates
        </Button>
      </RouterLink>
    </template>
  </PageHeader>

  <p v-if="notice" class="mb-4 rounded-md border border-ok/40 bg-ok-glow px-3 py-2 text-sm text-ok">{{ notice }}</p>
  <p v-if="error" class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger">
    {{ error }}
  </p>

  <div class="flex flex-col gap-5 lg:flex-row">
    <aside class="w-full shrink-0 rounded-md border border-line bg-surface p-3 lg:w-72">
      <form class="mb-2 flex gap-2" @submit.prevent="create">
        <Input v-model="newName" placeholder="New scoring mode..." />
        <Button type="submit" variant="dark" :disabled="busy" aria-label="Add">
          <Icon name="plus" :size="15" />
        </Button>
      </form>

      <ul class="space-y-1">
        <li v-for="item in items" :key="item.id ?? 0" class="group flex items-center">
          <button
            type="button"
            class="flex min-h-9 min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-md px-3 text-left text-sm font-medium transition-colors"
            :class="item.id === selectedId ? 'bg-accent-dim text-accent' : 'text-muted hover:bg-surface-2 hover:text-text'"
            @click="select(item.id!)"
          >
            <span class="min-w-0 flex-1 truncate">{{ item.name }}</span>
            <span
              v-if="usage[String(item.id)]"
              class="shrink-0 rounded-full bg-surface-3 px-1.5 text-xs text-dim"
              :title="`Used by ${usage[String(item.id)]} server/race setup reference${usage[String(item.id)] === 1 ? '' : 's'}`"
            >
              {{ usage[String(item.id)] }}
            </span>
          </button>
          <button
            type="button"
            class="ml-1 hidden size-8 cursor-pointer place-items-center rounded-md text-dim transition-colors group-hover:grid hover:bg-danger-glow hover:text-danger disabled:hidden"
            :disabled="item.id === 1 || !!usage[String(item.id)]"
            :aria-label="`Delete ${item.name}`"
            @click="remove(item.id!)"
          >
            <Icon name="trash" :size="14" />
          </button>
        </li>
      </ul>
    </aside>

    <form v-if="form" class="min-w-0 flex-1 space-y-4" @submit.prevent="save">
      <div class="rounded-md border border-line bg-surface p-4">
        <FormRow label="Name">
          <Input v-model="form.name" required />
        </FormRow>

        <div class="grid gap-3 sm:grid-cols-2">
          <Toggle v-model="collisionReset" label="Collision resets score" />
          <Toggle v-model="carCollisionReset" label="Car collision resets score" />
        </div>
      </div>

      <div class="rounded-md border border-line bg-surface p-4">
        <h2 class="mb-3 text-sm font-bold">Reset Timers</h2>
        <div class="grid gap-x-4 sm:grid-cols-2">
          <FormRow label="Reset score">
            <Toggle v-model="resetScore" label="Enabled" />
          </FormRow>
          <FormRow label="Score reset seconds" hint="Time below the speed or angle threshold before the run score ends.">
            <Slider v-model="form.reset_score_seconds" :min="0" :max="10" :step="0.1" :decimals="1" suffix="s" />
          </FormRow>
          <FormRow label="Reset multiplier">
            <Toggle v-model="resetMultiplier" label="Enabled" />
          </FormRow>
          <FormRow label="Multiplier reset seconds" hint="Time below the speed or angle threshold before multiplier drops back to 1x.">
            <Slider v-model="form.reset_multiplier_seconds" :min="0" :max="10" :step="0.1" :decimals="1" suffix="s" />
          </FormRow>
        </div>
      </div>

      <div class="rounded-md border border-line bg-surface p-4">
        <h2 class="mb-3 text-sm font-bold">Multiplier</h2>
        <div class="grid gap-x-4 sm:grid-cols-2">
          <FormRow label="Gain pace" :hint="multiplierGainHint">
            <Slider v-model="multiplierGainSeconds" :min="0.5" :max="12" :step="0.1" :decimals="1" suffix="s" />
          </FormRow>
          <FormRow label="Cap" hint="Maximum multiplier. Set to 1x to effectively disable multiplier growth.">
            <Slider v-model="form.multiplier_cap" :min="1" :max="50" :step="1" suffix="x" />
          </FormRow>
        </div>
      </div>

      <div class="rounded-md border border-line bg-surface p-4">
        <h2 class="mb-3 text-sm font-bold">Scoring Criteria</h2>
        <div class="grid gap-x-4 sm:grid-cols-2">
          <FormRow label="Minimum speed">
            <Slider v-model="form.min_speed_kmh" :min="0" :max="200" :step="1" suffix="km/h" />
          </FormRow>
          <FormRow label="Minimum angle">
            <Slider v-model="form.min_angle_deg" :min="0" :max="60" :step="0.1" :decimals="1" suffix="deg" />
          </FormRow>
          <FormRow label="Angle weight" hint="How much slip angle contributes to score.">
            <Slider v-model="form.angle_weight" :min="0" :max="0.05" :step="0.001" :decimals="3" />
          </FormRow>
          <FormRow label="Speed weight" hint="How much speed contributes to score. 0 disables speed scoring.">
            <Slider v-model="form.speed_weight" :min="0" :max="0.02" :step="0.001" :decimals="3" />
          </FormRow>
          <FormRow label="Proximity weight" hint="How much nearby-car or wall proximity contributes. 0 disables proximity scoring.">
            <Slider v-model="form.proximity_weight" :min="0" :max="5" :step="0.05" :decimals="2" />
          </FormRow>
          <FormRow label="Proximity range">
            <Slider v-model="form.proximity_range_m" :min="0.1" :max="20" :step="0.1" :decimals="1" suffix="m" />
          </FormRow>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <Button type="submit" :disabled="busy">
          <Icon name="check" :size="15" />
          Save mode
        </Button>
        <span v-if="selectedUsage" class="text-xs text-dim">Used by {{ selectedUsage }} server/race setup reference{{ selectedUsage === 1 ? "" : "s" }}.</span>
      </div>
    </form>

    <p v-else class="text-muted">Select a scoring mode or create a new one.</p>
  </div>
</template>
