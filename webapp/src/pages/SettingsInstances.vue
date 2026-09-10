<script setup lang="ts">
// Server instances: each one is an independent acServer process with its
// own ports and queue.
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import { useServerStore, type InstanceState } from "@/stores/server";
import { useSetupSummary } from "@/lib/useSetupSummary";
import { useConfirmStore } from "@/stores/confirm";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Modal from "@/components/ui/Modal.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import Select from "@/components/ui/Select.vue";
import Toggle from "@/components/ui/Toggle.vue";
import AdminBackButton from "@/components/AdminBackButton.vue";
import type { DropDownList } from "@/types/generated";

const server = useServerStore();
const { summary, reload: reloadSummary } = useSetupSummary();
const confirm = useConfirmStore();

// Setup health: instances aren't a first-run gate, but if global setup blocks
// running we surface it here with a link back to the guided /setup flow.
const setupHealthy = computed(() => summary.value?.can_start ?? true);
const setupBlocker = computed(() => summary.value?.blocking?.[0]?.message ?? "");

onMounted(() => {
  void server.load();
  void reloadSummary();
  void loadDriftModes();
});

const notice = ref("");
const error = ref("");
const busy = ref(false);
const driftModes = ref<DropDownList[]>([]);

interface InstanceForm {
  id: number | null;
  name: string;
  udp_port: number | null;
  tcp_port: number | null;
  http_port: number | null;
  plugin_port: number | null;
  plugin_listen_port: number | null;
  start_on_boot: boolean;
  drift_score_enabled: boolean;
  drift_scoring_mode_id: number | null;
  allow_wrong_way: boolean;
  stream_enabled: boolean;
  stream_embed_url: string;
  stream_status_url: string;
  spectator_enabled: boolean;
  spectator_driver_name: string;
  spectator_guid: string;
  spectator_car_key: string;
  spectator_skin_key: string;
}

const editorOpen = ref(false);
const form = ref<InstanceForm | null>(null);

// Unsaved-changes guard for the modal editor.
let instanceBaseline = "";
const markInstanceClean = () => (instanceBaseline = form.value ? JSON.stringify(form.value) : "");
const guardClose = useUnsavedGuard(() => editorOpen.value && form.value !== null && JSON.stringify(form.value) !== instanceBaseline);
const closeEditor = () => guardClose(() => { editorOpen.value = false; });

function nextFree(values: (number | null)[], fallback: number): number {
  const used = values.filter((v): v is number => v !== null);
  return used.length ? Math.max(...used) + 1 : fallback;
}

function openCreate() {
  const list = server.instanceList;
  // Suggest the next port block so new instances don't collide.
  // Game docker mappings cover 9601-9609 / 8082-8090 out of the box.
  form.value = {
    id: null,
    name: `Server ${list.length + 1}`,
    udp_port: nextFree(list.map((i) => i.udp_port), 9600),
    tcp_port: nextFree(list.map((i) => i.tcp_port), 9600),
    http_port: nextFree(list.map((i) => i.http_port), 8081),
    plugin_port: nextFree([...list.map((i) => i.plugin_port), ...list.map((i) => i.plugin_listen_port)], 5000),
    plugin_listen_port:
      nextFree([...list.map((i) => i.plugin_port), ...list.map((i) => i.plugin_listen_port)], 5000) + 1,
    start_on_boot: false,
    drift_score_enabled: false,
    drift_scoring_mode_id: 1,
    allow_wrong_way: false,
    stream_enabled: false,
    stream_embed_url: "",
    stream_status_url: "",
    spectator_enabled: false,
    spectator_driver_name: "Broadcast",
    spectator_guid: "",
    spectator_car_key: "",
    spectator_skin_key: "",
  };
  markInstanceClean();
  editorOpen.value = true;
}

function openEdit(inst: InstanceState) {
  form.value = {
    id: inst.id,
    name: inst.name,
    udp_port: inst.udp_port,
    tcp_port: inst.tcp_port,
    http_port: inst.http_port,
    plugin_port: inst.plugin_port,
    plugin_listen_port: inst.plugin_listen_port,
    start_on_boot: inst.start_on_boot === 1,
    drift_score_enabled: inst.drift_score_enabled === 1,
    drift_scoring_mode_id: inst.drift_scoring_mode_id ?? 1,
    allow_wrong_way: inst.allow_wrong_way === 1,
    stream_enabled: inst.stream_enabled === 1,
    stream_embed_url: inst.stream_embed_url ?? "",
    stream_status_url: inst.stream_status_url ?? "",
    spectator_enabled: inst.spectator_enabled === 1,
    spectator_driver_name: inst.spectator_driver_name ?? "",
    spectator_guid: inst.spectator_guid ?? "",
    spectator_car_key: inst.spectator_car_key ?? "",
    spectator_skin_key: inst.spectator_skin_key ?? "",
  };
  markInstanceClean();
  editorOpen.value = true;
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

const save = () =>
  guard(async () => {
    const f = form.value;
    if (!f) return;
    const body = {
      name: f.name,
      udp_port: f.udp_port,
      tcp_port: f.tcp_port,
      http_port: f.http_port,
      plugin_port: f.plugin_port,
      plugin_listen_port: f.plugin_listen_port,
      start_on_boot: f.start_on_boot ? 1 : 0,
      drift_score_enabled: f.drift_score_enabled ? 1 : 0,
      drift_scoring_mode_id: f.drift_scoring_mode_id ?? 1,
      allow_wrong_way: f.allow_wrong_way ? 1 : 0,
      stream_enabled: f.stream_enabled ? 1 : 0,
      stream_embed_url: f.stream_embed_url,
      stream_status_url: f.stream_status_url,
      spectator_enabled: f.spectator_enabled ? 1 : 0,
      spectator_driver_name: f.spectator_driver_name,
      spectator_guid: f.spectator_guid,
      spectator_car_key: f.spectator_car_key,
      spectator_skin_key: f.spectator_skin_key,
    };
    if (f.id) {
      await api.put(`/api/instances/${f.id}`, body);
      notice.value = "Instance updated.";
    } else {
      await api.post("/api/instances", body);
      notice.value = "Instance created.";
    }
    editorOpen.value = false;
    await server.load();
  });

const remove = (inst: InstanceState) =>
  guard(async () => {
    const ok = await confirm.ask({
      title: "Delete instance",
      message: `Delete "${inst.name}"?`,
      detail: "Its queue entries are removed too. This cannot be undone.",
      confirmLabel: "Delete instance",
      tone: "danger",
    });
    if (!ok) return;
    await api.delete(`/api/instances/${inst.id}`);
    notice.value = "Instance deleted.";
    await server.load();
  });

async function loadDriftModes() {
  const res = await api.get<{ items: DropDownList[] }>("/api/drift-scoring-modes?filled=1");
  driftModes.value = res.items ?? [];
}

</script>

<template>
  <PageHeader
    title="Server Instances"
    subtitle="Create and maintain independent acServer processes, ports, plugin pairs, and queues."
    icon="instances"
  >
    <template #prefix>
      <AdminBackButton />
    </template>
    <template #actions>
      <RouterLink v-if="!setupHealthy" to="/setup">
        <Button variant="dark" size="sm">
          <Icon name="alert" :size="14" class="text-warn" />
          Setup needed
        </Button>
      </RouterLink>
      <RouterLink to="/settings/streaming">
        <Button variant="ghost" size="sm">
          <Icon name="broadcast" :size="14" />
          Streaming
        </Button>
      </RouterLink>
      <Button @click="openCreate">
        <Icon name="plus" :size="15" />
        Add instance
      </Button>
    </template>
  </PageHeader>

  <RouterLink
    v-if="!setupHealthy"
    to="/setup"
    class="mb-4 flex items-start gap-2.5 rounded-md border border-warn/40 bg-warn-glow px-3 py-2.5 transition-colors hover:border-warn/60"
  >
    <Icon name="alert" :size="16" class="mt-0.5 shrink-0 text-warn" />
    <span class="text-sm">
      <span class="font-semibold text-warn">Configuration blocks running.</span>
      <span class="text-muted"> {{ setupBlocker }}</span>
      <span class="ml-1 font-semibold text-accent">Open setup →</span>
    </span>
  </RouterLink>

  <p v-if="notice" class="mb-4 rounded-md border border-ok/40 bg-ok-glow px-3 py-2 text-sm text-ok">{{ notice }}</p>
  <p v-if="error" class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger">
    {{ error }}
  </p>

  <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
    <Card v-for="inst in server.instanceList" :key="inst.id">
      <template #header>
        <span class="size-2 rounded-full" :class="inst.running ? 'bg-ok' : 'bg-dim'" />
        <h2 class="text-sm font-bold">{{ inst.name }}</h2>
      </template>
      <template #actions>
        <Button variant="dark" size="sm" :disabled="inst.running" @click="openEdit(inst)">Edit</Button>
        <Button
          v-if="server.instanceList.length > 1"
          variant="ghost"
          size="sm"
          :disabled="inst.running"
          @click="remove(inst)"
        >
          Delete
        </Button>
      </template>

      <dl class="grid grid-cols-2 gap-x-4 gap-y-1.5 font-mono text-xs">
        <dt class="font-sans text-muted">Game UDP / TCP</dt>
        <dd>{{ inst.udp_port }} / {{ inst.tcp_port }}</dd>
        <dt class="font-sans text-muted">HTTP</dt>
        <dd>{{ inst.http_port }}</dd>
        <dt class="font-sans text-muted">Plugin ports</dt>
        <dd>{{ inst.plugin_port }} → {{ inst.plugin_listen_port }}</dd>
        <dt class="font-sans text-muted">Start on boot</dt>
        <dd>{{ inst.start_on_boot === 1 ? "On" : "Off" }}</dd>
      </dl>
      <p v-if="inst.running" class="mt-2 text-xs text-dim">Stop the server to edit or delete.</p>
    </Card>
  </div>

  <Modal :open="editorOpen" :title="form?.id ? 'Edit instance' : 'New instance'" @close="closeEditor">
    <template v-if="form">
      <FormRow label="Name" for-id="iname" hint="Shown in the Assetto Corsa lobby">
        <Input id="iname" v-model="form.name" />
      </FormRow>
      <div class="grid gap-x-4 sm:grid-cols-2">
        <FormRow label="UDP port" for-id="iudp">
          <Input id="iudp" v-model="form.udp_port" type="number" :min="1024" :max="65535" />
        </FormRow>
        <FormRow label="TCP port" for-id="itcp">
          <Input id="itcp" v-model="form.tcp_port" type="number" :min="1024" :max="65535" />
        </FormRow>
        <FormRow label="HTTP port" for-id="ihttp">
          <Input id="ihttp" v-model="form.http_port" type="number" :min="1024" :max="65535" />
        </FormRow>
      </div>
      <div class="grid gap-x-4 sm:grid-cols-2">
        <FormRow label="Plugin port (acServer)" for-id="iplugin">
          <Input id="iplugin" v-model="form.plugin_port" type="number" :min="1024" :max="65535" />
        </FormRow>
        <FormRow label="Plugin listen port (SM)" for-id="ipluginl">
          <Input id="ipluginl" v-model="form.plugin_listen_port" type="number" :min="1024" :max="65535" />
        </FormRow>
      </div>
      <div class="mt-2 border-t border-line pt-4">
        <Toggle v-model="form.start_on_boot" label="Start this server when Server Manager launches" />
        <p class="mt-1 text-xs text-dim">Picks up the next queued event (or the repeat event) automatically on boot.</p>
      </div>
      <div class="mt-2 border-t border-line pt-4">
        <Toggle v-model="form.drift_score_enabled" label="Enable drift scoring HUD" />
        <p class="mt-1 text-xs text-dim">Serves a CSP Lua drift-score overlay to players. Requires the AssettoServer engine and CSP on the client; players without CSP simply won't see it.</p>
        <FormRow v-if="form.drift_score_enabled" label="Scoring mode" class="mt-3">
          <Select
            v-model="form.drift_scoring_mode_id"
            :options="driftModes.map((m) => ({ value: m.id ?? 1, label: m.name ?? '' }))"
          />
        </FormRow>
      </div>
      <div class="mt-2 border-t border-line pt-4">
        <Toggle v-model="form.allow_wrong_way" label="Allow driving the wrong way" />
        <p class="mt-1 text-xs text-dim">Writes <code>ALLOW_WRONG_WAY</code> to the CSP extra rules so drivers can go the opposite direction without being teleported back to the pits. Requires the AssettoServer engine and CSP on the client.</p>
      </div>
      <p class="text-xs text-dim">
        Docker setups map 9601-9609 (game) and 8082-8090 (http) by default — stay inside those ranges or extend the
        compose file. Plugin ports never leave the machine.
      </p>
    </template>
    <template #footer>
      <Button variant="ghost" @click="closeEditor">Cancel</Button>
      <Button :disabled="busy" @click="save">{{ form?.id ? "Save" : "Create" }}</Button>
    </template>
  </Modal>

</template>
