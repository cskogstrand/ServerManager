<script setup lang="ts">
// Server instances: each one is an independent acServer process with its
// own ports and queue.
import { onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useServerStore, type InstanceState } from "@/stores/server";
import { useContentStore } from "@/stores/content";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Modal from "@/components/ui/Modal.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import Toggle from "@/components/ui/Toggle.vue";
import Combobox from "@/components/ui/Combobox.vue";
import Select from "@/components/ui/Select.vue";

const server = useServerStore();
const content = useContentStore();
onMounted(() => {
  void server.load();
  void content.load();
  void loadDriverStreams();
});

const notice = ref("");
const error = ref("");
const busy = ref(false);

interface InstanceForm {
  id: number | null;
  name: string;
  udp_port: number | null;
  tcp_port: number | null;
  http_port: number | null;
  plugin_port: number | null;
  plugin_listen_port: number | null;
  stream_enabled: boolean;
  stream_embed_url: string;
  stream_status_url: string;
  spectator_enabled: boolean;
  spectator_driver_name: string;
  spectator_guid: string;
  spectator_car_key: string;
  spectator_skin_key: string;
}

interface DriverStream {
  id?: number;
  driver_guid?: string;
  display_name?: string;
  enabled?: number;
  stream_embed_url?: string;
  stream_status_url?: string;
}

interface DriverStreamForm {
  id: number | null;
  driver_guid: string;
  display_name: string;
  enabled: boolean;
  stream_embed_url: string;
  stream_status_url: string;
}

const editorOpen = ref(false);
const form = ref<InstanceForm | null>(null);

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
    stream_enabled: false,
    stream_embed_url: "",
    stream_status_url: "",
    spectator_enabled: false,
    spectator_driver_name: "Broadcast",
    spectator_guid: "",
    spectator_car_key: "",
    spectator_skin_key: "",
  };
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
    stream_enabled: inst.stream_enabled === 1,
    stream_embed_url: inst.stream_embed_url ?? "",
    stream_status_url: inst.stream_status_url ?? "",
    spectator_enabled: inst.spectator_enabled === 1,
    spectator_driver_name: inst.spectator_driver_name ?? "",
    spectator_guid: inst.spectator_guid ?? "",
    spectator_car_key: inst.spectator_car_key ?? "",
    spectator_skin_key: inst.spectator_skin_key ?? "",
  };
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
    if (!window.confirm(`Delete "${inst.name}"? Its queue entries are removed too.`)) return;
    await api.delete(`/api/instances/${inst.id}`);
    notice.value = "Instance deleted.";
    await server.load();
  });

function skins(carKey: string) {
  return content.carByKey(carKey)?.skins ?? [];
}

function onSpectatorCar() {
  const f = form.value;
  if (!f) return;
  f.spectator_skin_key = skins(f.spectator_car_key)[0]?.key ?? "";
}

const driverStreams = ref<DriverStream[]>([]);
const driverEditorOpen = ref(false);
const driverForm = ref<DriverStreamForm | null>(null);

async function loadDriverStreams() {
  try {
    const res = await api.get<{ streams: DriverStream[] }>("/api/driver-streams");
    driverStreams.value = res.streams ?? [];
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
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
  };
  driverEditorOpen.value = true;
}

function openDriverEdit(stream: DriverStream) {
  driverForm.value = {
    id: stream.id ?? null,
    driver_guid: stream.driver_guid ?? "",
    display_name: stream.display_name ?? "",
    enabled: stream.enabled !== 0,
    stream_embed_url: stream.stream_embed_url ?? "",
    stream_status_url: stream.stream_status_url ?? "",
  };
  driverEditorOpen.value = true;
}

const saveDriverStream = () =>
  guard(async () => {
    const f = driverForm.value;
    if (!f) return;
    const body = {
      driver_guid: f.driver_guid,
      display_name: f.display_name,
      enabled: f.enabled ? 1 : 0,
      stream_embed_url: f.stream_embed_url,
      stream_status_url: f.stream_status_url,
    };
    if (f.id) {
      await api.put(`/api/driver-streams/${f.id}`, body);
      notice.value = "Driver stream updated.";
    } else {
      await api.post("/api/driver-streams", body);
      notice.value = "Driver stream created.";
    }
    driverEditorOpen.value = false;
    await loadDriverStreams();
  });

const removeDriverStream = (stream: DriverStream) =>
  guard(async () => {
    if (!stream.id || !window.confirm(`Delete stream for "${stream.display_name || stream.driver_guid}"?`)) return;
    await api.delete(`/api/driver-streams/${stream.id}`);
    notice.value = "Driver stream deleted.";
    await loadDriverStreams();
  });
</script>

<template>
  <PageHeader
    title="Server Instances"
    subtitle="Create and maintain independent acServer processes, ports, plugin pairs, and queues."
    icon="instances"
  >
    <template #actions>
      <Button @click="openCreate">
        <Icon name="plus" :size="15" />
        Add instance
      </Button>
    </template>
  </PageHeader>

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
        <dt class="font-sans text-muted">Stream</dt>
        <dd>{{ inst.stream_enabled === 1 ? "Configured" : "Off" }}</dd>
        <dt class="font-sans text-muted">Spectator slot</dt>
        <dd>{{ inst.spectator_enabled === 1 ? "Reserved" : "Off" }}</dd>
      </dl>
      <p v-if="inst.running" class="mt-2 text-xs text-dim">Stop the server to edit or delete.</p>
    </Card>
  </div>

  <div class="mt-6">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
      <div class="min-w-0">
        <h2 class="text-sm font-bold">Driver Streams</h2>
        <p class="text-xs text-dim">Map driver GUIDs to external WebRTC player URLs for dashboard watch actions.</p>
      </div>
      <Button size="sm" @click="openDriverCreate">
        <Icon name="plus" :size="14" />
        Add driver stream
      </Button>
    </div>

    <div v-if="driverStreams.length" class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
      <Card v-for="stream in driverStreams" :key="stream.id">
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
          <dt class="text-muted">Health URL</dt>
          <dd class="min-w-0 truncate font-mono">{{ stream.stream_status_url || "—" }}</dd>
        </dl>
      </Card>
    </div>
    <p v-else class="rounded-md border border-line bg-surface px-3 py-3 text-sm text-dim">
      No driver streams configured.
    </p>
  </div>

  <Modal :open="editorOpen" :title="form?.id ? 'Edit instance' : 'New instance'" @close="editorOpen = false">
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
        <Toggle v-model="form.stream_enabled" label="Show fixed spectator stream on dashboard" />
        <div v-if="form.stream_enabled" class="mt-3">
          <FormRow label="WebRTC player URL" for-id="istream-url" hint="Browser-playable page or WHEP/player URL exposed by OBS, MediaMTX or similar.">
            <Input id="istream-url" v-model="form.stream_embed_url" placeholder="https://stream.example.com/camera" />
          </FormRow>
          <FormRow label="Health URL" for-id="istream-status" hint="Optional URL SM probes to show live/offline status.">
            <Input id="istream-status" v-model="form.stream_status_url" placeholder="https://stream.example.com/health" />
          </FormRow>
        </div>
      </div>

      <div class="mt-2 border-t border-line pt-4">
        <Toggle v-model="form.spectator_enabled" label="Reserve a locked spectator slot for the stream client" />
        <div v-if="form.spectator_enabled" class="mt-3">
          <div class="grid gap-x-4 sm:grid-cols-2">
            <FormRow label="Driver name" for-id="ispec-name">
              <Input id="ispec-name" v-model="form.spectator_driver_name" />
            </FormRow>
            <FormRow label="Driver GUID" for-id="ispec-guid">
              <Input id="ispec-guid" v-model="form.spectator_guid" class="font-mono" />
            </FormRow>
          </div>
          <div class="grid gap-x-4 sm:grid-cols-2">
            <FormRow label="Car" hint="The full AC client on the stream PC must have this car installed.">
              <Combobox
                v-model="form.spectator_car_key"
                placeholder="Search cars…"
                :options="content.cars.map((c) => ({ value: c.key ?? '', label: c.name ?? c.key ?? '' }))"
                @update:model-value="onSpectatorCar"
              />
            </FormRow>
            <FormRow label="Skin">
              <Select
                v-model="form.spectator_skin_key"
                :options="skins(form.spectator_car_key).map((s) => ({ value: s.key, label: s.name || s.key }))"
              />
            </FormRow>
          </div>
        </div>
      </div>
      <p class="text-xs text-dim">
        Docker setups map 9601-9609 (game) and 8082-8090 (http) by default — stay inside those ranges or extend the
        compose file. Plugin ports never leave the machine.
      </p>
    </template>
    <template #footer>
      <Button variant="ghost" @click="editorOpen = false">Cancel</Button>
      <Button :disabled="busy" @click="save">{{ form?.id ? "Save" : "Create" }}</Button>
    </template>
  </Modal>

  <Modal :open="driverEditorOpen" :title="driverForm?.id ? 'Edit driver stream' : 'New driver stream'" @close="driverEditorOpen = false">
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
    </template>
    <template #footer>
      <Button variant="ghost" @click="driverEditorOpen = false">Cancel</Button>
      <Button :disabled="busy" @click="saveDriverStream">{{ driverForm?.id ? "Save" : "Create" }}</Button>
    </template>
  </Modal>
</template>
