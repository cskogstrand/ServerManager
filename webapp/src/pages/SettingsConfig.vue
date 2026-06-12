<script setup lang="ts">
// Server configuration (the user_config row). Ports are intentionally not
// here anymore — each server instance owns its ports (Settings → Instances,
// Phase 5).
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import type { UserConfig } from "@/types/generated";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Toggle from "@/components/ui/Toggle.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";

const form = ref<UserConfig | null>(null);
const busy = ref(false);
const notice = ref("");
const error = ref("");

// Dedicated-server engine + AssettoServer install state
type EngineStatus = {
  engine: string;
  assettoserver_version: string;
  assettoserver_installed: boolean;
};
const engineStatus = ref<EngineStatus | null>(null);
const installing = ref(false);
const installError = ref("");

const serverEngine = computed({
  get: () => form.value?.server_engine || "kunos",
  set: (v: string) => {
    if (form.value) form.value.server_engine = v;
  },
});

async function installAssettoServer() {
  installing.value = true;
  installError.value = "";
  try {
    await api.post("/api/server/assettoserver/install", {});
    engineStatus.value = await api.get<EngineStatus>("/api/server/engine");
  } catch (e) {
    installError.value = e instanceof ApiError ? e.message : String(e);
  } finally {
    installing.value = false;
  }
}

// Bridges a 0/1 int field to the Toggle's boolean model
function intToggle(key: keyof UserConfig) {
  return computed({
    get: () => (form.value?.[key] ?? 0) === 1,
    set: (v: boolean) => {
      if (form.value) (form.value as any)[key] = v ? 1 : 0;
    },
  });
}

const registerToLobby = intToggle("register_to_lobby");
const lockedEntryList = intToggle("locked_entry_list");
const appendEventname = intToggle("append_eventname");
const appendModlinks = intToggle("append_modlinks");
const autoStart = intToggle("auto_start_server");

onMounted(async () => {
  form.value = await api.get<UserConfig>("/api/config");
  try {
    engineStatus.value = await api.get<EngineStatus>("/api/server/engine");
  } catch {
    /* non-fatal: status panel just won't render */
  }
});

async function save() {
  if (!form.value) return;
  busy.value = true;
  notice.value = "";
  error.value = "";
  try {
    await api.put("/api/config", form.value);
    notice.value = "Configuration saved.";
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <PageHeader
    title="Server Configuration"
    subtitle="Global server identity, lobby behavior, access settings, and engine limits."
    icon="settings"
  />

  <p v-if="notice" class="mb-4 rounded-md border border-ok/40 bg-ok-glow px-3 py-2 text-sm text-ok">
    {{ notice }}
  </p>
  <p v-if="error" class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger">
    {{ error }}
  </p>

  <form v-if="form" class="max-w-2xl space-y-4" @submit.prevent="save">
    <Card title="Identity">
      <FormRow label="Server name" for-id="name" hint="Shown in the Assetto Corsa lobby">
        <Input id="name" v-model="form.name" />
      </FormRow>
      <FormRow label="Welcome message" for-id="welcome">
        <Input id="welcome" v-model="form.welcome_message" />
      </FormRow>
      <div class="flex flex-wrap gap-x-8 gap-y-3 pt-1">
        <Toggle v-model="registerToLobby" label="Register to lobby" />
        <Toggle v-model="appendEventname" label="Append event name" />
        <Toggle v-model="appendModlinks" label="Append mod links" />
      </div>
      <FormRow
        v-if="appendModlinks"
        label="Mod download URL"
        for-id="modurl"
        hint="Public base URL players use to download mods (e.g. http://your-host:3030). Leave empty to use the detected public IP on port 3030. Set this when the web port is remapped behind Docker/NAT."
      >
        <Input id="modurl" v-model="form.mod_download_url" placeholder="http://your-host:3030" />
      </FormRow>
    </Card>

    <Card title="Access">
      <FormRow label="Server password" for-id="pw" hint="Leave empty for a public server">
        <Input id="pw" v-model="form.password" />
      </FormRow>
      <FormRow label="Admin password" for-id="adminpw">
        <Input id="adminpw" v-model="form.admin_password" />
      </FormRow>
      <Toggle v-model="lockedEntryList" label="Locked entry list" />
    </Card>

    <Card title="Engine">
      <div class="grid gap-x-4 sm:grid-cols-2">
        <FormRow label="Max clients" for-id="maxclients">
          <Input id="maxclients" v-model="form.max_clients" type="number" :min="1" :max="64" />
        </FormRow>
        <FormRow label="Result screen time (s)" for-id="resulttime">
          <Input id="resulttime" v-model="form.result_screen_time" type="number" :min="0" />
        </FormRow>
        <FormRow label="Client send interval (Hz)" for-id="sendinterval">
          <Input id="sendinterval" v-model="form.client_send_interval" type="number" :min="10" :max="35" />
        </FormRow>
        <FormRow label="Threads" for-id="threads">
          <Input id="threads" v-model="form.num_threads" type="number" :min="1" :max="32" />
        </FormRow>
      </div>
      <Toggle v-model="autoStart" label="Auto-start queue on launch" />
    </Card>

    <Card title="Dedicated server engine">
      <FormRow
        label="Engine"
        for-id="engine"
        hint="Kunos is the stock acServer. AssettoServer is a drop-in replacement that serves Content Manager's 'Install missing content' button — required for one-click mod downloads."
      >
        <Select
          id="engine"
          v-model="serverEngine"
          :options="[
            { value: 'kunos', label: 'Kunos acServer (stock)' },
            { value: 'assettoserver', label: 'AssettoServer (Content Manager downloads)' },
          ]"
        />
      </FormRow>

      <div v-if="serverEngine === 'assettoserver' && engineStatus" class="space-y-2 pt-1">
        <p v-if="engineStatus.assettoserver_installed" class="text-sm text-ok">
          <Icon name="check" :size="14" /> AssettoServer {{ engineStatus.assettoserver_version }} installed.
        </p>
        <p v-else class="text-sm text-muted">
          AssettoServer {{ engineStatus.assettoserver_version }} not downloaded yet — it will be fetched
          automatically on first start, or download it now:
        </p>
        <Button type="button" variant="ghost" :disabled="installing" @click="installAssettoServer">
          <Icon name="folder" :size="15" />
          {{
            installing
              ? "Downloading…"
              : engineStatus.assettoserver_installed
                ? "Re-download AssettoServer"
                : "Download AssettoServer"
          }}
        </Button>
        <p v-if="installError" class="text-sm text-danger">{{ installError }}</p>
      </div>
    </Card>

    <Button type="submit" :disabled="busy">
      <Icon name="check" :size="15" />
      {{ busy ? "Saving…" : "Save configuration" }}
    </Button>
  </form>
</template>
