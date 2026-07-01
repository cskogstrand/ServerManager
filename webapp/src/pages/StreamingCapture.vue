<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import { useServerStore, type InstanceState } from "@/stores/server";
import { useContentStore } from "@/stores/content";
import type { UserConfig } from "@/types/generated";
import PageHeader from "@/components/ui/PageHeader.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Toggle from "@/components/ui/Toggle.vue";
import Modal from "@/components/ui/Modal.vue";
import Icon from "@/components/ui/Icon.vue";
import Combobox from "@/components/ui/Combobox.vue";
import Select from "@/components/ui/Select.vue";

interface DriverStream {
  id?: number;
  driver_guid?: string;
  display_name?: string;
  enabled?: number;
  stream_embed_url?: string;
  stream_status_url?: string;
  stream_capture_url?: string;
}

interface DriverStreamForm {
  id: number | null;
  driver_guid: string;
  display_name: string;
  enabled: boolean;
  stream_embed_url: string;
  stream_status_url: string;
  stream_capture_url: string;
}

interface InstanceStreamForm {
  id: number;
  stream_enabled: boolean;
  stream_embed_url: string;
  stream_status_url: string;
  spectator_enabled: boolean;
  spectator_driver_name: string;
  spectator_guid: string;
  spectator_car_key: string;
  spectator_skin_key: string;
}

const toast = useToastStore();
const confirm = useConfirmStore();
const server = useServerStore();
const content = useContentStore();
const form = ref<UserConfig | null>(null);
const streams = ref<DriverStream[]>([]);
const busy = ref(false);
const loading = ref(true);

const CAPTURE_KEYS: (keyof UserConfig)[] = [
  "capture_enabled",
  "capture_screenshots",
  "capture_clips",
  "capture_trigger_score",
  "capture_clip_seconds",
  "capture_cooldown_seconds",
  "capture_max_per_session",
];
const captureSnapshot = () => {
  if (!form.value) return "";
  const out: Record<string, unknown> = {};
  for (const k of CAPTURE_KEYS) out[k] = form.value[k];
  return JSON.stringify(out);
};

let captureBaseline = "";
let driverBaseline = "";
let instanceBaseline = "";
const driverOpen = ref(false);
const driverForm = ref<DriverStreamForm | null>(null);
const instanceOpen = ref(false);
const instanceForm = ref<InstanceStreamForm | null>(null);
const markCaptureClean = () => (captureBaseline = captureSnapshot());
const markDriverClean = () => (driverBaseline = driverForm.value ? JSON.stringify(driverForm.value) : "");
const markInstanceClean = () => (instanceBaseline = instanceForm.value ? JSON.stringify(instanceForm.value) : "");
useUnsavedGuard(
  () =>
    (form.value !== null && captureSnapshot() !== captureBaseline) ||
    (driverOpen.value && driverForm.value !== null && JSON.stringify(driverForm.value) !== driverBaseline) ||
    (instanceOpen.value && instanceForm.value !== null && JSON.stringify(instanceForm.value) !== instanceBaseline),
);

function intToggle(key: keyof UserConfig) {
  return computed({
    get: () => (form.value?.[key] ?? 0) === 1,
    set: (v: boolean) => {
      if (form.value) (form.value as Record<string, unknown>)[key] = v ? 1 : 0;
    },
  });
}

const captureEnabled = intToggle("capture_enabled");
const captureScreenshots = intToggle("capture_screenshots");
const captureClips = intToggle("capture_clips");

async function load() {
  loading.value = true;
  try {
    const [cfg, res] = await Promise.all([
      api.get<UserConfig>("/api/config"),
      api.get<{ streams: DriverStream[] }>("/api/driver-streams"),
      server.load(),
      content.load(),
    ]);
    form.value = cfg;
    streams.value = res.streams ?? [];
    markCaptureClean();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    loading.value = false;
  }
}

function skins(carKey: string) {
  return content.carByKey(carKey)?.skins ?? [];
}

function openInstanceStream(inst: InstanceState) {
  instanceForm.value = {
    id: inst.id,
    stream_enabled: inst.stream_enabled === 1,
    stream_embed_url: inst.stream_embed_url ?? "",
    stream_status_url: inst.stream_status_url ?? "",
    spectator_enabled: inst.spectator_enabled === 1,
    spectator_driver_name: inst.spectator_driver_name ?? "Broadcast",
    spectator_guid: inst.spectator_guid ?? "",
    spectator_car_key: inst.spectator_car_key ?? "",
    spectator_skin_key: inst.spectator_skin_key ?? "",
  };
  markInstanceClean();
  instanceOpen.value = true;
}

function onSpectatorCar() {
  const f = instanceForm.value;
  if (!f) return;
  f.spectator_skin_key = skins(f.spectator_car_key)[0]?.key ?? "";
}

async function saveInstanceStream() {
  const f = instanceForm.value;
  const inst = f ? server.instances[f.id] : null;
  if (!f || !inst) return;
  busy.value = true;
  try {
    await api.put(`/api/instances/${f.id}`, {
      name: inst.name,
      udp_port: inst.udp_port,
      tcp_port: inst.tcp_port,
      http_port: inst.http_port,
      plugin_port: inst.plugin_port,
      plugin_listen_port: inst.plugin_listen_port,
      start_on_boot: inst.start_on_boot,
      drift_score_enabled: inst.drift_score_enabled,
      allow_wrong_way: inst.allow_wrong_way,
      stream_enabled: f.stream_enabled ? 1 : 0,
      stream_embed_url: f.stream_embed_url,
      stream_status_url: f.stream_status_url,
      spectator_enabled: f.spectator_enabled ? 1 : 0,
      spectator_driver_name: f.spectator_driver_name,
      spectator_guid: f.spectator_guid,
      spectator_car_key: f.spectator_car_key,
      spectator_skin_key: f.spectator_skin_key,
    });
    instanceOpen.value = false;
    await server.load();
    toast.success("Instance stream saved.");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

async function saveCapture() {
  if (!form.value) return;
  busy.value = true;
  try {
    await api.put("/api/config", form.value);
    markCaptureClean();
    toast.success("Capture settings saved.");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

function openDriverCreate() {
  driverForm.value = {
    id: null,
    driver_guid: "",
    display_name: "",
    enabled: true,
    stream_embed_url: "",
    stream_status_url: "",
    stream_capture_url: "",
  };
  markDriverClean();
  driverOpen.value = true;
}

function openDriverEdit(stream: DriverStream) {
  driverForm.value = {
    id: stream.id ?? null,
    driver_guid: stream.driver_guid ?? "",
    display_name: stream.display_name ?? "",
    enabled: stream.enabled !== 0,
    stream_embed_url: stream.stream_embed_url ?? "",
    stream_status_url: stream.stream_status_url ?? "",
    stream_capture_url: stream.stream_capture_url ?? "",
  };
  markDriverClean();
  driverOpen.value = true;
}

async function saveDriverStream() {
  const f = driverForm.value;
  if (!f) return;
  busy.value = true;
  try {
    const body = {
      driver_guid: f.driver_guid,
      display_name: f.display_name,
      enabled: f.enabled ? 1 : 0,
      stream_embed_url: f.stream_embed_url,
      stream_status_url: f.stream_status_url,
      stream_capture_url: f.stream_capture_url,
    };
    if (f.id) await api.put(`/api/driver-streams/${f.id}`, body);
    else await api.post("/api/driver-streams", body);
    driverOpen.value = false;
    await load();
    toast.success("Driver stream saved.");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

async function removeDriverStream(stream: DriverStream) {
  if (!stream.id) return;
  const ok = await confirm.ask({
    title: "Delete driver stream",
    message: `Delete stream for "${stream.display_name || stream.driver_guid}"?`,
    detail: "The driver will no longer appear with a linked stream or capture source.",
    confirmLabel: "Delete stream",
    tone: "danger",
  });
  if (!ok) return;
  busy.value = true;
  try {
    await api.delete(`/api/driver-streams/${stream.id}`);
    await load();
    toast.success("Driver stream deleted.");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

onMounted(load);
</script>

<template>
  <PageHeader
    title="Streaming & Capture"
    subtitle="Driver stream links, capture sources, and automatic highlight capture."
    icon="broadcast"
  >
    <template #actions>
      <RouterLink to="/settings/streams">
        <Button variant="ghost" size="sm">
          <Icon name="activity" :size="14" />
          Diagnostics
        </Button>
      </RouterLink>
    </template>
  </PageHeader>

  <div v-if="loading" class="space-y-4">
    <div class="h-36 animate-pulse rounded-md border border-line bg-surface-2/50" />
    <div class="h-56 animate-pulse rounded-md border border-line bg-surface-2/50" />
  </div>

  <template v-else>
    <div class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
      <Card title="Stream highlight capture">
        <p class="mb-3 text-xs text-muted">
          Auto-capture screenshots and clips from a driver's stream when they land a big drift run. Requires a per-driver capture URL and ffmpeg on the server.
        </p>
        <Toggle v-model="captureEnabled" label="Enable automatic capture" />
        <div class="mt-2 grid gap-2 sm:grid-cols-2">
          <Toggle v-model="captureScreenshots" label="Capture screenshots" />
          <Toggle v-model="captureClips" label="Capture clips" />
        </div>
        <div v-if="form" class="mt-3 grid gap-3 sm:grid-cols-2">
          <FormRow label="Trigger score" for-id="captrigger" hint="Minimum drift run score that fires a capture.">
            <Input id="captrigger" v-model="form.capture_trigger_score" type="number" :min="0" />
          </FormRow>
          <FormRow label="Clip length (s)" for-id="capclip" hint="Auto-clip duration, max 120.">
            <Input id="capclip" v-model="form.capture_clip_seconds" type="number" :min="1" :max="120" />
          </FormRow>
          <FormRow label="Cooldown (s)" for-id="capcool" hint="Minimum gap between captures for one driver.">
            <Input id="capcool" v-model="form.capture_cooldown_seconds" type="number" :min="0" />
          </FormRow>
          <FormRow label="Max per session" for-id="capmax" hint="Cap on captures per driver each session.">
            <Input id="capmax" v-model="form.capture_max_per_session" type="number" :min="1" />
          </FormRow>
        </div>
        <Button class="mt-3" :disabled="busy || !form" @click="saveCapture">
          <Icon name="check" :size="15" />
          Save capture settings
        </Button>
      </Card>

      <Card title="Fixed spectator streams">
        <div v-if="server.instanceList.length" class="space-y-2">
          <div
            v-for="inst in server.instanceList"
            :key="inst.id"
            class="flex items-center gap-2 rounded-md border border-line bg-surface-2/40 px-3 py-2"
          >
            <span class="size-2 rounded-full" :class="inst.stream_enabled === 1 ? 'bg-ok' : 'bg-dim'" />
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-semibold">{{ inst.name }}</div>
              <div class="text-xs text-dim">
                {{ inst.stream_enabled === 1 ? "stream configured" : "stream off" }} ·
                {{ inst.spectator_enabled === 1 ? "spectator reserved" : "no spectator slot" }}
              </div>
            </div>
            <Button variant="dark" size="sm" @click="openInstanceStream(inst)">Edit</Button>
          </div>
        </div>
        <p v-else class="text-sm text-muted">No server instances configured.</p>
      </Card>
    </div>

    <div class="mt-5">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-sm font-bold">Driver streams</h2>
          <p class="text-xs text-dim">Map driver GUIDs to browser player URLs and optional raw capture URLs.</p>
        </div>
        <Button size="sm" @click="openDriverCreate">
          <Icon name="plus" :size="14" />
          Add driver stream
        </Button>
      </div>

      <div v-if="streams.length" class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        <Card v-for="stream in streams" :key="stream.id">
          <template #header>
            <span class="size-2 rounded-full" :class="stream.enabled !== 0 ? 'bg-ok' : 'bg-dim'" />
            <h3 class="min-w-0 truncate text-sm font-bold">{{ stream.display_name || stream.driver_guid }}</h3>
          </template>
          <template #actions>
            <Button variant="dark" size="sm" @click="openDriverEdit(stream)">Edit</Button>
            <Button variant="ghost" size="sm" @click="removeDriverStream(stream)">Delete</Button>
          </template>
          <dl class="grid gap-y-1.5 text-xs">
            <dt class="text-muted">GUID</dt>
            <dd class="min-w-0 truncate font-mono">{{ stream.driver_guid }}</dd>
            <dt class="text-muted">Player URL</dt>
            <dd class="min-w-0 truncate font-mono">{{ stream.stream_embed_url }}</dd>
            <dt class="text-muted">Capture URL</dt>
            <dd class="min-w-0 truncate font-mono">{{ stream.stream_capture_url || "-" }}</dd>
          </dl>
        </Card>
      </div>
      <p v-else class="rounded-md border border-line bg-surface px-3 py-3 text-sm text-dim">
        No driver streams configured.
      </p>
    </div>
  </template>

  <Modal :open="driverOpen" :title="driverForm?.id ? 'Edit driver stream' : 'New driver stream'" @close="driverOpen = false">
    <template v-if="driverForm">
      <Toggle v-model="driverForm.enabled" label="Show this stream when the driver is connected" />
      <div class="mt-3 grid gap-x-4 sm:grid-cols-2">
        <FormRow label="Driver GUID" for-id="dsguid">
          <Input id="dsguid" v-model="driverForm.driver_guid" class="font-mono" />
        </FormRow>
        <FormRow label="Display name" for-id="dsname">
          <Input id="dsname" v-model="driverForm.display_name" />
        </FormRow>
      </div>
      <FormRow label="WebRTC player URL" for-id="dsurl">
        <Input id="dsurl" v-model="driverForm.stream_embed_url" placeholder="https://stream.example.com/driver" />
      </FormRow>
      <FormRow label="Health URL" for-id="dsstatus" hint="Optional URL SM probes to show live/offline status.">
        <Input id="dsstatus" v-model="driverForm.stream_status_url" placeholder="https://stream.example.com/driver/health" />
      </FormRow>
      <FormRow
        label="Capture URL"
        for-id="dscapture"
        hint="Optional raw stream (HLS/RTMP/RTSP/SRT) ffmpeg pulls from to auto-capture drift-spike screenshots and clips."
      >
        <Input id="dscapture" v-model="driverForm.stream_capture_url" placeholder="https://stream.example.com/driver/index.m3u8" />
      </FormRow>
    </template>
    <template #footer>
      <Button variant="ghost" @click="driverOpen = false">Cancel</Button>
      <Button :disabled="busy" @click="saveDriverStream">{{ driverForm?.id ? "Save" : "Create" }}</Button>
    </template>
  </Modal>

  <Modal :open="instanceOpen" title="Fixed spectator stream" @close="instanceOpen = false">
    <template v-if="instanceForm">
      <Toggle v-model="instanceForm.stream_enabled" label="Show fixed spectator stream on dashboard" />
      <div v-if="instanceForm.stream_enabled" class="mt-3">
        <FormRow label="WebRTC player URL" for-id="istream-url" hint="Browser-playable page or WHEP/player URL exposed by OBS, MediaMTX or similar.">
          <Input id="istream-url" v-model="instanceForm.stream_embed_url" placeholder="https://stream.example.com/camera" />
        </FormRow>
        <FormRow label="Health URL" for-id="istream-status" hint="Optional URL SM probes to show live/offline status.">
          <Input id="istream-status" v-model="instanceForm.stream_status_url" placeholder="https://stream.example.com/health" />
        </FormRow>
      </div>

      <div class="mt-3 border-t border-line pt-3">
        <Toggle v-model="instanceForm.spectator_enabled" label="Reserve a locked spectator slot for the stream client" />
        <div v-if="instanceForm.spectator_enabled" class="mt-3">
          <div class="grid gap-x-4 sm:grid-cols-2">
            <FormRow label="Driver name" for-id="ispec-name">
              <Input id="ispec-name" v-model="instanceForm.spectator_driver_name" />
            </FormRow>
            <FormRow label="Driver GUID" for-id="ispec-guid">
              <Input id="ispec-guid" v-model="instanceForm.spectator_guid" class="font-mono" />
            </FormRow>
          </div>
          <div class="grid gap-x-4 sm:grid-cols-2">
            <FormRow label="Car" hint="The full AC client on the stream PC must have this car installed.">
              <Combobox
                v-model="instanceForm.spectator_car_key"
                placeholder="Search cars..."
                :options="content.cars.map((c) => ({ value: c.key ?? '', label: c.name ?? c.key ?? '' }))"
                @update:model-value="onSpectatorCar"
              />
            </FormRow>
            <FormRow label="Skin">
              <Select
                v-model="instanceForm.spectator_skin_key"
                :options="skins(instanceForm.spectator_car_key).map((s) => ({ value: s.key, label: s.name || s.key }))"
              />
            </FormRow>
          </div>
        </div>
      </div>
    </template>
    <template #footer>
      <Button variant="ghost" @click="instanceOpen = false">Cancel</Button>
      <Button :disabled="busy" @click="saveInstanceStream">Save stream</Button>
    </template>
  </Modal>
</template>
