<script setup lang="ts">
// Server instances: each one is an independent acServer process with its
// own ports and queue.
import { onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useServerStore, type InstanceState } from "@/stores/server";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Modal from "@/components/ui/Modal.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";

const server = useServerStore();
onMounted(() => void server.load());

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
      </dl>
      <p v-if="inst.running" class="mt-2 text-xs text-dim">Stop the server to edit or delete.</p>
    </Card>
  </div>

  <Modal :open="editorOpen" :title="form?.id ? 'Edit instance' : 'New instance'" @close="editorOpen = false">
    <template v-if="form">
      <FormRow label="Name" for-id="iname" hint="Shown in the Assetto Corsa lobby">
        <Input id="iname" v-model="form.name" />
      </FormRow>
      <div class="grid grid-cols-2 gap-x-4">
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
      <div class="grid grid-cols-2 gap-x-4">
        <FormRow label="Plugin port (acServer)" for-id="iplugin">
          <Input id="iplugin" v-model="form.plugin_port" type="number" :min="1024" :max="65535" />
        </FormRow>
        <FormRow label="Plugin listen port (SM)" for-id="ipluginl">
          <Input id="ipluginl" v-model="form.plugin_listen_port" type="number" :min="1024" :max="65535" />
        </FormRow>
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
</template>
