<script setup lang="ts">
// Server configuration (the user_config row). Ports are intentionally not
// here anymore — each server instance owns its ports (Settings → Instances,
// Phase 5).
import { computed, onMounted, ref } from "vue";
import { api, ApiError, csrfToken } from "@/lib/api";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import { useToastStore } from "@/stores/toast";
import type { UserConfig } from "@/types/generated";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Toggle from "@/components/ui/Toggle.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";

const toast = useToastStore();
const form = ref<UserConfig | null>(null);
const busy = ref(false);
const notice = ref("");
const error = ref("");

// The install/CSP fields live on the same config row but save through their own
// endpoint (with path validation), so they get a separate baseline. Tracking
// them apart from the main config fields keeps each Save button's dirty state
// — and the unsaved guard — honest.
const INSTALL_KEYS: (keyof UserConfig)[] = [
  "install_path",
  "csp_required",
  "csp_version",
  "csp_phycars",
  "csp_phytracks",
  "csp_hidepit",
];
const snapshot = (keep: boolean) => {
  if (!form.value) return "";
  const o: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(form.value)) {
    if (INSTALL_KEYS.includes(k as keyof UserConfig) === keep) o[k] = v;
  }
  return JSON.stringify(o);
};

// Two baselines: the config half and the install half, compared independently.
let cfgBaseline = "";
let installBaseline = "";
const markCfgClean = () => (cfgBaseline = snapshot(false));
const markInstallClean = () => (installBaseline = snapshot(true));
const cfgDirty = () => form.value !== null && snapshot(false) !== cfgBaseline;
const installDirty = () => form.value !== null && snapshot(true) !== installBaseline;
useUnsavedGuard(() => cfgDirty() || installDirty());

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
const relaxChecksums = intToggle("as_relax_checksums");
const autoStart = intToggle("auto_start_server");
const captureEnabled = intToggle("capture_enabled");
const captureScreenshots = intToggle("capture_screenshots");
const captureClips = intToggle("capture_clips");
const cspRequired = intToggle("csp_required");
const cspPhycars = intToggle("csp_phycars");
const cspPhytracks = intToggle("csp_phytracks");
const cspHidepit = intToggle("csp_hidepit");
const pathValid = ref<boolean | null>(null);

onMounted(async () => {
  form.value = await api.get<UserConfig>("/api/config");
  markCfgClean();
  markInstallClean();
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
    markCfgClean();
    notice.value = "Configuration saved.";
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function validatePath() {
  pathValid.value = null;
  const data = new FormData();
  data.append("path", form.value?.install_path ?? "");
  const res = await fetch("/api/validate/installpath", {
    method: "POST",
    headers: { "X-CSRF-Token": csrfToken() },
    body: data,
  });
  const json = await res.json();
  pathValid.value = json.result === true;
  return pathValid.value;
}

async function saveInstall() {
  if (!form.value) return;
  if (!(await validatePath())) {
    toast.error("No acServer binary found under that path — expected <path>/server/acServer.");
    return;
  }
  try {
    await api.put("/api/config/content", form.value);
    markInstallClean();
    toast.success("Installation settings saved. Rebuild the content cache to import content.");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
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

      <div v-if="serverEngine === 'assettoserver'" class="pt-1">
        <Toggle
          v-model="relaxChecksums"
          label="Allow mod content without checksums / track params"
        />
        <p class="pt-1 text-xs text-muted">
          Lets mod cars without a packed <code>data.acd</code> and mod tracks without
          location/timezone params start (sets <code>MissingCarChecksums</code> and
          <code>MissingTrackParams</code>). Weakens AssettoServer's content validation — enable only
          on trusted/LAN servers.
        </p>
      </div>

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

    <Card title="Stream highlight capture">
      <p class="mb-3 text-xs text-muted">
        Auto-capture screenshots and clips from a driver's stream when they land a big drift run. Requires a
        per-driver capture URL set under Instances → Driver streams, and ffmpeg installed on the server.
      </p>
      <Toggle v-model="captureEnabled" label="Enable automatic capture" />
      <div class="mt-2 grid gap-2 sm:grid-cols-2">
        <Toggle v-model="captureScreenshots" label="Capture screenshots" />
        <Toggle v-model="captureClips" label="Capture clips" />
      </div>
      <div class="mt-3 grid gap-3 sm:grid-cols-2">
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
    </Card>

    <Button type="submit" :disabled="busy">
      <Icon name="check" :size="15" />
      {{ busy ? "Saving…" : "Save configuration" }}
    </Button>
  </form>

  <form v-if="form" class="mt-4 max-w-2xl" @submit.prevent="saveInstall">
    <Card title="Installation">
      <FormRow
        label="Assetto Corsa install path"
        for-id="installpath"
        hint="Folder containing server/acServer — e.g. .../steamapps/common/assettocorsa (or /corsa in docker)"
      >
        <div class="flex gap-1">
          <Input id="installpath" v-model="form.install_path" class="flex-1" />
          <Button type="button" variant="dark" size="sm" @click="validatePath">
            <Icon v-if="pathValid === true" name="check" :size="14" />
            <Icon v-else-if="pathValid === false" name="x" :size="14" />
            <span>{{ pathValid === null ? "Check" : pathValid ? "Valid" : "Invalid" }}</span>
          </Button>
        </div>
      </FormRow>

      <Toggle v-model="cspRequired" label="Require Custom Shaders Patch (CSP)" />
      <div v-if="cspRequired" class="mt-3">
        <FormRow label="Minimum CSP version" for-id="cspver" hint="Build number, e.g. 3155">
          <Input id="cspver" v-model="form.csp_version" type="number" :min="0" />
        </FormRow>
        <div class="grid gap-2 sm:grid-cols-2">
          <Toggle v-model="cspPhycars" label="Extended car physics" />
          <Toggle v-model="cspPhytracks" label="Extended track physics" />
          <Toggle v-model="cspHidepit" label="Hide pitboxes" />
        </div>
      </div>

      <Button type="submit" class="mt-3">
        <Icon name="check" :size="15" />
        Save installation
      </Button>
    </Card>
  </form>
</template>
