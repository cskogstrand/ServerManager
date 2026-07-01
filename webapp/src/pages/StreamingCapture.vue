<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import type { UserConfig } from "@/types/generated";
import PageHeader from "@/components/ui/PageHeader.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Toggle from "@/components/ui/Toggle.vue";
import Modal from "@/components/ui/Modal.vue";
import Icon from "@/components/ui/Icon.vue";

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

const toast = useToastStore();
const confirm = useConfirmStore();
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
const driverOpen = ref(false);
const driverForm = ref<DriverStreamForm | null>(null);
const markCaptureClean = () => (captureBaseline = captureSnapshot());
const markDriverClean = () => (driverBaseline = driverForm.value ? JSON.stringify(driverForm.value) : "");
useUnsavedGuard(
  () =>
    (form.value !== null && captureSnapshot() !== captureBaseline) ||
    (driverOpen.value && driverForm.value !== null && JSON.stringify(driverForm.value) !== driverBaseline),
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

      <Card title="Fixed spectator stream">
        <p class="text-sm text-muted">
          Fixed spectator camera URLs and reserved spectator slots are configured per server instance.
        </p>
        <RouterLink to="/settings/instances">
          <Button class="mt-3" variant="dark" size="sm">
            <Icon name="instances" :size="14" />
            Open instances
          </Button>
        </RouterLink>
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
</template>
