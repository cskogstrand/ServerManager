<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api, ApiError, csrfToken } from "@/lib/api";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import { useToastStore } from "@/stores/toast";
import type { UserConfig } from "@/types/generated";
import PageHeader from "@/components/ui/PageHeader.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Toggle from "@/components/ui/Toggle.vue";
import Icon from "@/components/ui/Icon.vue";

const toast = useToastStore();
const form = ref<UserConfig | null>(null);
const busy = ref(false);
const pathValid = ref<boolean | null>(null);

const snapshot = () =>
  form.value
    ? JSON.stringify({
        install_path: form.value.install_path,
        csp_required: form.value.csp_required,
        csp_version: form.value.csp_version,
        csp_phycars: form.value.csp_phycars,
        csp_phytracks: form.value.csp_phytracks,
        csp_hidepit: form.value.csp_hidepit,
      })
    : "";
let baseline = "";
const markClean = () => (baseline = snapshot());
useUnsavedGuard(() => snapshot() !== baseline);

function intToggle(key: keyof UserConfig) {
  return computed({
    get: () => (form.value?.[key] ?? 0) === 1,
    set: (v: boolean) => {
      if (form.value) (form.value as Record<string, unknown>)[key] = v ? 1 : 0;
    },
  });
}

const cspRequired = intToggle("csp_required");
const cspPhycars = intToggle("csp_phycars");
const cspPhytracks = intToggle("csp_phytracks");
const cspHidepit = intToggle("csp_hidepit");

onMounted(async () => {
  form.value = await api.get<UserConfig>("/api/config");
  markClean();
});

async function validatePath() {
  pathValid.value = null;
  const data = new FormData();
  data.append("path", form.value?.install_path ?? "");
  const res = await fetch("/api/validate/installpath", {
    method: "POST",
    headers: { "X-CSRF-Token": csrfToken() },
    body: data,
  });
  pathValid.value = (await res.json()).result === true;
  return pathValid.value;
}

async function save() {
  if (!form.value) return;
  busy.value = true;
  try {
    if (!(await validatePath())) {
      toast.error("No acServer binary found under that path - expected <path>/server/acServer.");
      return;
    }
    await api.put("/api/config/content", form.value);
    markClean();
    toast.success("Installation settings saved. Rebuild the content cache to import content.");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <PageHeader
    title="Installation"
    subtitle="Assetto Corsa install path and CSP requirements used when building server configs."
    icon="folder"
  >
    <template #actions>
      <RouterLink to="/content">
        <Button variant="ghost" size="sm">
          <Icon name="content" :size="14" />
          Content Library
        </Button>
      </RouterLink>
    </template>
  </PageHeader>

  <form v-if="form" class="max-w-2xl" @submit.prevent="save">
    <Card title="Install path">
      <FormRow
        label="Assetto Corsa install path"
        for-id="installpath"
        hint="Folder containing server/acServer - e.g. .../steamapps/common/assettocorsa (or /corsa in docker)"
      >
        <div class="flex gap-1">
          <Input id="installpath" v-model="form.install_path" class="flex-1" @update:model-value="pathValid = null" />
          <Button type="button" variant="dark" size="sm" @click="validatePath">
            <Icon v-if="pathValid === true" name="check" :size="14" />
            <Icon v-else-if="pathValid === false" name="x" :size="14" />
            <span>{{ pathValid === null ? "Check" : pathValid ? "Valid" : "Invalid" }}</span>
          </Button>
        </div>
      </FormRow>

      <div class="mt-4 border-t border-line pt-4">
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
      </div>
    </Card>

    <Button class="mt-4" type="submit" :disabled="busy">
      <Icon name="check" :size="15" />
      {{ busy ? "Saving..." : "Save installation" }}
    </Button>
  </form>
</template>
