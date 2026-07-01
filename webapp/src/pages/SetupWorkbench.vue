<script setup lang="ts">
// Setup Workbench: a guided first-run command center. From login, a fresh
// install can reach a running server here without visiting Content, Presets,
// Events, Queue or Instances — every blocking item has an inline fix. Steps are
// non-blocking (you can jump around) and resumable (status comes from
// /api/setup/summary, so leaving and returning lands on the first gap).
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api, ApiError, csrfToken } from "@/lib/api";
import { useQueryParam, enumParam } from "@/lib/useQueryParam";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import { useToastStore } from "@/stores/toast";
import { useServerStore } from "@/stores/server";
import {
  useSetupSummary,
  SETUP_STEPS,
  stepDone,
  firstIncompleteStep,
  type SetupStep,
} from "@/lib/useSetupSummary";
import { emptyRaceSetup, raceSetupBody, raceSetupValid, type RaceSetupDraft } from "@/lib/useRaceSetupDraft";
import type { UserConfig } from "@/types/generated";
import PageHeader from "@/components/ui/PageHeader.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Toggle from "@/components/ui/Toggle.vue";
import Skeleton from "@/components/ui/Skeleton.vue";
import RaceSetupEditor from "@/components/RaceSetupEditor.vue";

const route = useRoute();
const router = useRouter();
const toast = useToastStore();
const server = useServerStore();
const { summary, loading, reload } = useSetupSummary();

const STEP_META: Record<SetupStep, { label: string; icon: string }> = {
  install: { label: "Install", icon: "folder" },
  content: { label: "Content", icon: "content" },
  server: { label: "Server", icon: "settings" },
  instance: { label: "Instance", icon: "instances" },
  race: { label: "Race", icon: "events" },
  run: { label: "Run", icon: "power" },
};

// Active step mirrored to ?step; when absent we resume at the first gap.
const step = useQueryParam<SetupStep>("step", "install", enumParam(SETUP_STEPS, "install"));
const busy = ref(false);

// Full config (server identity + install path). Edited in place, saved whole.
const config = ref<UserConfig | null>(null);
const pathValid = ref<boolean | null>(null);

// Race draft + the saved event id it produces.
const draft = ref<RaceSetupDraft>(emptyRaceSetup());
const raceGroupId = ref<number | null>(null);
const savedEventId = ref<number | null>(null);
const savedEventName = ref("");

// Instance + run selections.
const runInstanceId = ref<number | null>(null);
const runAction = ref<"start" | "queue" | "repeat">("start");

const DRAFT_AUTOSAVE_KEY = "sm.setup.raceDraft.v1";

function hasDraftValue(d: RaceSetupDraft): boolean {
  return !!(d.name || d.track_key || d.class_id || d.session_id || d.time_id || d.difficulty_id);
}

function restoreDraftAutosave() {
  try {
    const raw = localStorage.getItem(DRAFT_AUTOSAVE_KEY);
    if (!raw) return;
    const saved = JSON.parse(raw) as {
      draft?: Partial<RaceSetupDraft>;
      raceGroupId?: number | null;
      savedEventId?: number | null;
      savedEventName?: string;
    };
    if (saved.draft) draft.value = { ...emptyRaceSetup(), ...saved.draft };
    raceGroupId.value = saved.raceGroupId ?? null;
    savedEventId.value = saved.savedEventId ?? null;
    savedEventName.value = saved.savedEventName ?? "";
  } catch {
    localStorage.removeItem(DRAFT_AUTOSAVE_KEY);
  }
}

function persistDraftAutosave() {
  if (!hasDraftValue(draft.value)) {
    localStorage.removeItem(DRAFT_AUTOSAVE_KEY);
    return;
  }
  localStorage.setItem(
    DRAFT_AUTOSAVE_KEY,
    JSON.stringify({
      draft: draft.value,
      raceGroupId: raceGroupId.value,
      savedEventId: savedEventId.value,
      savedEventName: savedEventName.value,
    }),
  );
}

function clearDraftAutosave() {
  localStorage.removeItem(DRAFT_AUTOSAVE_KEY);
}

restoreDraftAutosave();
watch([draft, raceGroupId], persistDraftAutosave, { deep: true });

let configBaseline = "";
let draftBaseline = "";
const configSnapshot = () => (config.value ? JSON.stringify(config.value) : "");
const draftSnapshot = () => JSON.stringify(draft.value);
const markConfigClean = () => (configBaseline = configSnapshot());
const markDraftClean = () => (draftBaseline = draftSnapshot());
markDraftClean();
useUnsavedGuard(() => configSnapshot() !== configBaseline || draftSnapshot() !== draftBaseline);

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

async function loadConfig() {
  config.value = await api.get<UserConfig>("/api/config");
  markConfigClean();
}

onMounted(async () => {
  await Promise.all([reload(), loadConfig(), server.load()]);
  // URL wins on resume; otherwise land on the first incomplete step.
  if (!route.query.step) step.value = firstIncompleteStep(summary.value);
  runInstanceId.value = summary.value?.instances[0]?.id ?? null;
  raceGroupId.value ??= summary.value?.groups[0]?.id ?? null;
});

// Keep run-instance selection valid as instances appear.
watch(summary, (s) => {
  if (s && runInstanceId.value === null) runInstanceId.value = s.instances[0]?.id ?? null;
  if (s && raceGroupId.value === null) raceGroupId.value = s.groups[0]?.id ?? null;
});

async function refresh() {
  await Promise.all([reload(), server.load()]);
}

const steps = computed(() =>
  SETUP_STEPS.map((s) => ({
    id: s,
    label: STEP_META[s].label,
    icon: STEP_META[s].icon,
    done: stepDone(summary.value, s),
  })),
);

const canStart = computed(() => summary.value?.can_start ?? false);

function go(s: SetupStep) {
  step.value = s;
}
function nextStep() {
  const i = SETUP_STEPS.indexOf(step.value);
  if (i < SETUP_STEPS.length - 1) step.value = SETUP_STEPS[i + 1];
}
function prevStep() {
  const i = SETUP_STEPS.indexOf(step.value);
  if (i > 0) step.value = SETUP_STEPS[i - 1];
}

// --- Step: Install & Content ---
const validatePath = () =>
  guard(async () => {
    pathValid.value = null;
    const data = new FormData();
    data.append("path", config.value?.install_path ?? "");
    const res = await fetch("/api/validate/installpath", {
      method: "POST",
      headers: { "X-CSRF-Token": csrfToken() },
      body: data,
    });
    pathValid.value = (await res.json()).result === true;
  });

const saveInstall = () =>
  guard(async () => {
    if (!config.value) return;
    await api.put("/api/config/content", config.value);
    markConfigClean();
    toast.success("Install path saved.");
    await refresh();
  });

const rebuildCache = () =>
  guard(async () => {
    await api.post("/api/content/recache");
    toast.success("Cache rebuild started — counts update when it finishes.");
    await refresh();
  });

// --- Step: Server identity ---
function intToggle(key: keyof UserConfig) {
  return computed({
    get: () => (config.value?.[key] ?? 0) === 1,
    set: (v: boolean) => {
      if (config.value) (config.value as Record<string, unknown>)[key] = v ? 1 : 0;
    },
  });
}
const registerToLobby = intToggle("register_to_lobby");

const saveServer = () =>
  guard(async () => {
    if (!config.value) return;
    await api.put("/api/config", config.value);
    markConfigClean();
    toast.success("Server configuration saved.");
    await refresh();
  });

// --- Step: Instance ---
const createInstance = () =>
  guard(async () => {
    const s = summary.value;
    if (!s) return;
    const ports = s.suggested_ports;
    const { id } = await api.post<{ id: number }>("/api/instances", {
      name: `Server ${s.instances.length + 1}`,
      udp_port: ports.udp_port,
      tcp_port: ports.tcp_port,
      http_port: ports.http_port,
      plugin_port: ports.plugin_port,
      plugin_listen_port: ports.plugin_listen_port,
    });
    runInstanceId.value = id;
    toast.success("Instance created.");
    await refresh();
  });

// --- Step: Race setup ---
async function ensureGroup(): Promise<number> {
  if (raceGroupId.value) return raceGroupId.value;
  const { id } = await api.post<{ id: number }>("/api/categories", { name: "Race setups" });
  raceGroupId.value = id;
  await reload();
  return id;
}

const saveRaceSetup = () =>
  guard(async () => {
    if (!raceSetupValid(draft.value)) {
      toast.error("Track and all four presets are required.");
      return;
    }
    const groupId = await ensureGroup();
    const body = raceSetupBody(draft.value, groupId);
    if (savedEventId.value) {
      await api.put(`/api/event/${savedEventId.value}`, body);
    } else {
      const res = await api.post<{ id: number }>("/api/events", body);
      savedEventId.value = res.id ?? null;
    }
    savedEventName.value = draft.value.name || draft.value.track_name;
    clearDraftAutosave();
    markDraftClean();
    toast.success("Race setup saved.");
    await refresh();
    if (savedEventId.value) step.value = "run";
  });

// --- Step: Run ---
const runReady = computed(() => savedEventId.value !== null && runInstanceId.value !== null && canStart.value);

const runNow = () =>
  guard(async () => {
    const eid = savedEventId.value;
    const iid = runInstanceId.value;
    if (eid === null || iid === null) return;

    if (runAction.value === "repeat") {
      await api.put(`/api/instances/${iid}/runmode`, { run_mode: "repeat_event", repeat_event_id: eid });
      await api.post(`/api/server/start?instance=${iid}`);
      toast.success("Repeat mode set and server starting.");
    } else {
      await api.post(`/api/queue/event/${eid}?instance=${iid}`);
      if (runAction.value === "start") {
        await api.post(`/api/server/start?instance=${iid}`);
        toast.success("Event queued and server starting.");
      } else {
        toast.success("Event queued.");
      }
    }
    await refresh();
    void router.push(`/server/${iid}`);
  });

// --- "What will run" summary ---
const runInstance = computed(() => summary.value?.instances.find((i) => i.id === runInstanceId.value) ?? null);
const serverName = computed(() => config.value?.name?.trim() || runInstance.value?.name || "Unnamed server");
</script>

<template>
  <PageHeader
    title="Server Setup"
    subtitle="Everything from a fresh install to a running server, in one guided workbench."
    icon="settings"
  >
    <template #actions>
      <Button variant="dark" size="sm" :disabled="busy || loading" @click="refresh">
        <Icon name="activity" :size="15" />
        Re-check
      </Button>
    </template>
  </PageHeader>

  <div v-if="loading" class="space-y-4">
    <Skeleton class="h-12" />
    <Skeleton class="h-96" />
  </div>

  <template v-else-if="summary">
    <!-- Readiness bar -->
    <div class="mb-4 flex flex-wrap gap-2">
      <button
        v-for="s in steps"
        :key="s.id"
        type="button"
        class="inline-flex min-h-8 items-center gap-1.5 rounded-md border px-2.5 text-xs font-semibold transition-colors"
        :class="
          step === s.id
            ? 'border-accent/50 bg-accent-dim text-accent'
            : s.done
              ? 'border-ok/40 bg-ok-glow text-ok hover:border-ok/60'
              : 'border-line bg-surface text-muted hover:border-line-hi'
        "
        @click="go(s.id)"
      >
        <Icon :name="s.done ? 'check' : s.icon" :size="14" />
        {{ s.label }}
      </button>
    </div>

    <div class="grid gap-4 lg:grid-cols-[200px_1fr_280px]">
      <!-- Left rail -->
      <nav class="flex flex-row flex-wrap gap-1.5 lg:flex-col">
        <button
          v-for="s in steps"
          :key="s.id"
          type="button"
          class="flex items-center gap-2 rounded-md border px-3 py-2 text-left text-sm transition-colors"
          :class="step === s.id ? 'border-accent/50 bg-accent-dim text-text' : 'border-line bg-surface text-muted hover:border-line-hi hover:text-text'"
          @click="go(s.id)"
        >
          <span
            class="grid size-5 shrink-0 place-items-center rounded-full text-[10px]"
            :class="s.done ? 'bg-ok text-bg' : step === s.id ? 'bg-accent text-bg' : 'bg-surface-3 text-dim'"
          >
            <Icon v-if="s.done" name="check" :size="12" />
            <template v-else>{{ SETUP_STEPS.indexOf(s.id) + 1 }}</template>
          </span>
          {{ s.label }}
        </button>
      </nav>

      <!-- Main panel -->
      <Card>
        <!-- Install & Content -->
        <template v-if="step === 'install'">
          <h2 class="mb-1 text-sm font-bold">Install &amp; content path</h2>
          <p class="mb-4 text-sm text-muted">Point Server Manager at your Assetto Corsa install so it can find the server binary and content.</p>
          <FormRow v-if="config" label="Assetto Corsa install path" hint="The folder containing server/acServer and content/.">
            <div class="flex gap-2">
              <Input v-model="config.install_path" class="flex-1 font-mono" placeholder="/path/to/assettocorsa" @update:model-value="pathValid = null" />
              <Button variant="dark" :disabled="busy" @click="validatePath">Validate</Button>
            </div>
          </FormRow>
          <p v-if="pathValid === true" class="mb-3 text-xs text-ok">acServer found under that path.</p>
          <p v-else-if="pathValid === false" class="mb-3 text-xs text-danger">No acServer binary found — expected &lt;path&gt;/server/acServer.</p>
          <div class="flex flex-wrap gap-2">
            <Button :disabled="busy" @click="saveInstall">
              <Icon name="check" :size="15" />
              Save path
            </Button>
            <RouterLink to="/content">
              <Button variant="ghost">Open full content library</Button>
            </RouterLink>
          </div>
        </template>

        <!-- Content -->
        <template v-else-if="step === 'content'">
          <h2 class="mb-1 text-sm font-bold">Content cache</h2>
          <p class="mb-4 text-sm text-muted">Rebuild the cache to import tracks, cars and weather from the install path.</p>
          <dl class="mb-4 grid grid-cols-3 gap-3">
            <div class="rounded-md border border-line bg-surface-2/40 px-3 py-2 text-center">
              <dt class="text-xs text-dim">Tracks</dt>
              <dd class="font-mono text-lg" :class="summary.content.tracks ? 'text-text' : 'text-danger'">{{ summary.content.tracks }}</dd>
            </div>
            <div class="rounded-md border border-line bg-surface-2/40 px-3 py-2 text-center">
              <dt class="text-xs text-dim">Cars</dt>
              <dd class="font-mono text-lg" :class="summary.content.cars ? 'text-text' : 'text-danger'">{{ summary.content.cars }}</dd>
            </div>
            <div class="rounded-md border border-line bg-surface-2/40 px-3 py-2 text-center">
              <dt class="text-xs text-dim">Weather</dt>
              <dd class="font-mono text-lg text-text">{{ summary.content.weathers }}</dd>
            </div>
          </dl>
          <div class="flex flex-wrap gap-2">
            <Button :disabled="busy" @click="rebuildCache">
              <Icon name="activity" :size="15" />
              Rebuild cache
            </Button>
            <RouterLink to="/content">
              <Button variant="ghost">Upload / browse content</Button>
            </RouterLink>
          </div>
        </template>

        <!-- Server identity -->
        <template v-else-if="step === 'server'">
          <h2 class="mb-1 text-sm font-bold">Server identity</h2>
          <p class="mb-4 text-sm text-muted">The lobby name and access controls players see.</p>
          <template v-if="config">
            <FormRow label="Lobby name">
              <Input v-model="config.name" placeholder="My Race Server" />
            </FormRow>
            <div class="grid gap-x-4 sm:grid-cols-2">
              <FormRow label="Join password" hint="Leave blank for an open server">
                <Input v-model="config.password" />
              </FormRow>
              <FormRow label="Admin password">
                <Input v-model="config.admin_password" />
              </FormRow>
            </div>
            <div class="grid gap-x-4 sm:grid-cols-2">
              <FormRow label="Engine">
                <Select
                  v-model="config.server_engine"
                  :options="[
                    { value: 'kunos', label: 'Kunos acServer' },
                    { value: 'assettoserver', label: 'AssettoServer' },
                  ]"
                />
              </FormRow>
              <FormRow label="Max clients">
                <Input v-model="config.max_clients" type="number" :min="1" :max="64" />
              </FormRow>
            </div>
            <Toggle v-model="registerToLobby" label="List on the public Assetto Corsa lobby" />
            <Button class="mt-3" :disabled="busy" @click="saveServer">
              <Icon name="check" :size="15" />
              Save configuration
            </Button>
            <RouterLink to="/settings" class="ml-2 text-xs text-accent hover:underline">Advanced settings →</RouterLink>
          </template>
        </template>

        <!-- Instance -->
        <template v-else-if="step === 'instance'">
          <h2 class="mb-1 text-sm font-bold">Server instance</h2>
          <p class="mb-4 text-sm text-muted">An instance is one running server process with its own ports and queue.</p>
          <div v-if="summary.instances.length" class="space-y-2">
            <div
              v-for="inst in summary.instances"
              :key="inst.id"
              class="flex items-center gap-3 rounded-md border px-3 py-2"
              :class="runInstanceId === inst.id ? 'border-accent/50 bg-accent-dim' : 'border-line'"
            >
              <span class="size-2 rounded-full" :class="inst.is_running ? 'bg-ok' : 'bg-dim'" />
              <span class="flex-1 text-sm font-medium">{{ inst.name }}</span>
              <Button v-if="runInstanceId !== inst.id" variant="dark" size="sm" @click="runInstanceId = inst.id">Use this</Button>
              <span v-else class="text-xs text-accent">Selected</span>
            </div>
            <p v-if="summary.port_conflict" class="flex items-center gap-2 text-xs text-danger">
              <Icon name="alert" :size="14" />
              {{ summary.port_conflict }}
            </p>
            <div class="flex flex-wrap gap-x-3 gap-y-1">
              <RouterLink to="/settings/instances" class="inline-block text-xs text-accent hover:underline">Manage ports →</RouterLink>
              <RouterLink to="/settings/streaming" class="inline-block text-xs text-accent hover:underline">Manage streams →</RouterLink>
            </div>
          </div>
          <div v-else>
            <p class="mb-3 text-sm text-muted">
              Suggested ports: game {{ summary.suggested_ports.udp_port }}, HTTP {{ summary.suggested_ports.http_port }},
              plugin {{ summary.suggested_ports.plugin_port }}→{{ summary.suggested_ports.plugin_listen_port }}.
              Docker setups map 9601-9609 / 8082-8090 by default.
            </p>
            <Button :disabled="busy" @click="createInstance">
              <Icon name="plus" :size="15" />
              Create instance with suggested ports
            </Button>
          </div>
        </template>

        <!-- Race setup -->
        <template v-else-if="step === 'race'">
          <div class="mb-3 flex items-center justify-between gap-2">
            <h2 class="text-sm font-bold">Race setup</h2>
            <FormRow v-if="summary.groups.length" label="" class="!mb-0">
              <Select
                v-model="raceGroupId"
                :options="summary.groups.map((g) => ({ value: g.id ?? 0, label: g.name ?? '' }))"
              />
            </FormRow>
          </div>
          <RaceSetupEditor v-model="draft" />
          <Button class="mt-3" :disabled="busy || !raceSetupValid(draft)" @click="saveRaceSetup">
            <Icon name="check" :size="15" />
            Save race setup
          </Button>
          <span v-if="!raceSetupValid(draft)" class="ml-2 text-xs text-muted">Track and all four presets are required.</span>
        </template>

        <!-- Run -->
        <template v-else-if="step === 'run'">
          <h2 class="mb-1 text-sm font-bold">Run</h2>
          <p class="mb-4 text-sm text-muted">Choose how to launch the race you just set up.</p>

          <FormRow v-if="summary.instances.length > 1" label="Instance">
            <Select
              v-model="runInstanceId"
              :options="summary.instances.map((i) => ({ value: i.id, label: i.name + (i.is_running ? ' (running)' : '') }))"
            />
          </FormRow>

          <FormRow label="When you start">
            <Select
              v-model="runAction"
              :options="[
                { value: 'start', label: 'Start now' },
                { value: 'queue', label: 'Queue only' },
                { value: 'repeat', label: 'Repeat continuously' },
              ]"
            />
          </FormRow>

          <p v-if="!savedEventId" class="mb-3 flex items-center gap-2 rounded-md border border-warn/40 bg-warn-glow px-3 py-2 text-xs text-warn">
            <Icon name="alert" :size="14" />
            Save a race setup first.
            <button type="button" class="font-semibold underline" @click="go('race')">Go to Race setup</button>
          </p>

          <Button :variant="runReady ? 'success' : 'primary'" :disabled="busy || !runReady" @click="runNow">
            <Icon name="power" :size="15" />
            {{ runAction === "queue" ? "Queue event" : "Start server" }}
          </Button>
        </template>

        <!-- Footer -->
        <div class="mt-5 flex items-center justify-between border-t border-line pt-3">
          <Button variant="ghost" :disabled="step === 'install'" @click="prevStep">
            <Icon name="arrowUp" :size="15" class="-rotate-90" />
            Back
          </Button>
          <Button v-if="step !== 'run'" variant="dark" @click="nextStep">
            Next
            <Icon name="arrowUp" :size="15" class="rotate-90" />
          </Button>
        </div>
      </Card>

      <!-- Right panel: what will run -->
      <Card title="What will run" class="h-fit">
        <dl class="grid grid-cols-1 gap-y-2 text-sm">
          <div>
            <dt class="text-xs text-dim">Server</dt>
            <dd class="truncate font-medium">{{ serverName }}</dd>
          </div>
          <div>
            <dt class="text-xs text-dim">Instance</dt>
            <dd class="truncate">{{ runInstance?.name ?? "—" }}</dd>
          </div>
          <div>
            <dt class="text-xs text-dim">Track</dt>
            <dd class="truncate">{{ draft.track_name || "—" }}</dd>
          </div>
          <div>
            <dt class="text-xs text-dim">Cars</dt>
            <dd class="truncate">{{ draft.class_name || "—" }}</dd>
          </div>
          <div>
            <dt class="text-xs text-dim">Sessions</dt>
            <dd class="truncate">{{ draft.session_name || "—" }}</dd>
          </div>
          <div>
            <dt class="text-xs text-dim">Time / weather</dt>
            <dd class="truncate">{{ draft.time_name || "—" }}</dd>
          </div>
          <div>
            <dt class="text-xs text-dim">Race length</dt>
            <dd class="truncate">{{ draft.race_laps ? `${draft.race_laps} laps` : "Timed race" }}</dd>
          </div>
        </dl>

        <div class="mt-4 border-t border-line pt-3">
          <div class="mb-1.5 flex items-center gap-2 text-xs font-semibold" :class="canStart ? 'text-ok' : 'text-warn'">
            <Icon :name="canStart ? 'check' : 'alert'" :size="14" />
            {{ canStart ? "Ready to run" : `${summary.blocking.length} thing${summary.blocking.length === 1 ? "" : "s"} to fix` }}
          </div>
          <ul v-if="summary.blocking.length" class="space-y-1">
            <li v-for="b in summary.blocking" :key="b.step + b.message">
              <button type="button" class="text-left text-xs text-muted hover:text-text" @click="go(b.step as SetupStep)">
                · {{ b.message }}
              </button>
            </li>
          </ul>
        </div>
      </Card>
    </div>
  </template>
</template>
