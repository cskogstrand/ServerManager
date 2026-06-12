<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useContentStore } from "@/stores/content";
import { api, ApiError, csrfToken } from "@/lib/api";
import { intToggle } from "@/lib/forms";
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
import EmptyState from "@/components/ui/EmptyState.vue";
import Skeleton from "@/components/ui/Skeleton.vue";

const content = useContentStore();
const toast = useToastStore();
const libraryLoading = ref(true);

// --- Installation & CSP (the content half of user_config) ---
const config = ref<UserConfig | null>(null);
const cspRequired = intToggle(config, "csp_required");
const cspPhycars = intToggle(config, "csp_phycars");
const cspPhytracks = intToggle(config, "csp_phytracks");
const cspHidepit = intToggle(config, "csp_hidepit");
const pathValid = ref<boolean | null>(null);

onMounted(async () => {
  content.load().finally(() => (libraryLoading.value = false));
  config.value = await api.get<UserConfig>("/api/config");
});

async function validatePath() {
  pathValid.value = null;
  const data = new FormData();
  data.append("path", config.value?.install_path ?? "");
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
  if (!config.value) return;
  if (!(await validatePath())) {
    toast.error("No acServer binary found under that path — expected <path>/server/acServer.");
    return;
  }
  try {
    await api.put("/api/config/content", config.value);
    toast.success("Installation settings saved. Rebuild the cache to import content.");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

const tab = ref<"tracks" | "cars" | "weathers">("tracks");
const search = ref("");

const filteredTracks = computed(() =>
  content.tracks.filter((t) => (t.name ?? t.key ?? "").toLowerCase().includes(search.value.toLowerCase())),
);
const filteredCars = computed(() =>
  content.cars.filter((c) => (c.name ?? c.key ?? "").toLowerCase().includes(search.value.toLowerCase())),
);
const filteredWeathers = computed(() =>
  content.weathers.filter((w) => (w.name ?? w.key ?? "").toLowerCase().includes(search.value.toLowerCase())),
);

// --- Upload ---
const kind = ref<"track" | "car">("track");
const overwrite = ref(false);
const archiveUrl = ref("");
const file = ref<File | null>(null);
const uploadProgress = ref(0);
const uploading = ref(false);

function onFileChange(e: Event) {
  file.value = (e.target as HTMLInputElement).files?.[0] ?? null;
}

// XHR instead of fetch: upload progress events. Job progress after the
// upload itself arrives over SSE (content store).
function upload() {
  if (!file.value && !archiveUrl.value.trim()) {
    toast.error("Choose an archive file or paste a download URL.");
    return;
  }
  uploading.value = true;
  uploadProgress.value = 0;

  const data = new FormData();
  data.append("kind", kind.value);
  data.append("overwrite", overwrite.value ? "1" : "0");
  if (archiveUrl.value.trim()) {
    data.append("archive_url", archiveUrl.value.trim());
  } else if (file.value) {
    data.append("archive", file.value);
  }

  const xhr = new XMLHttpRequest();
  xhr.open("POST", "/api/content/upload");
  xhr.setRequestHeader("X-CSRF-Token", csrfToken());
  xhr.responseType = "json";
  xhr.upload.addEventListener("progress", (e) => {
    if (e.lengthComputable) uploadProgress.value = Math.round((e.loaded / e.total) * 100);
  });
  xhr.addEventListener("loadend", () => {
    uploading.value = false;
    if (xhr.status >= 200 && xhr.status < 300) {
      toast.success("Upload accepted — import progress shows below.");
      file.value = null;
      archiveUrl.value = "";
    } else {
      toast.error(xhr.response?.message ?? xhr.response?.error?.message ?? "Upload failed.");
    }
  });
  xhr.send(data);
}

async function recache() {
  try {
    await api.post("/api/content/recache");
    await content.load(true);
    toast.success("Content cache rebuilt.");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

function jobTone(status: string) {
  if (status === "completed") return "text-ok";
  if (status === "failed") return "text-danger";
  return "text-accent";
}
</script>

<template>
  <PageHeader
    title="Content"
    subtitle="Browse installed tracks, cars, and weather; validate the AC path; upload or rebuild content cache."
    icon="content"
  />

  <div class="grid items-start gap-5 xl:grid-cols-[1fr_360px]">
    <!-- Library -->
    <Card>
      <template #header>
        <div class="flex gap-1">
          <button
            v-for="t in (['tracks', 'cars', 'weathers'] as const)"
            :key="t"
            type="button"
            class="min-h-8 cursor-pointer rounded-md px-3 text-sm font-semibold capitalize transition-colors"
            :class="tab === t ? 'bg-accent-dim text-accent' : 'text-muted hover:text-text'"
            @click="tab = t"
          >
            {{ t }}
            <span class="ml-1 text-xs text-dim">
              {{ t === "tracks" ? content.tracks.length : t === "cars" ? content.cars.length : content.weathers.length }}
            </span>
          </button>
        </div>
      </template>
      <template #actions>
        <Input v-model="search" placeholder="Search…" class="!w-44" />
      </template>

      <div v-if="libraryLoading" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <Skeleton v-for="n in 6" :key="n" class="aspect-video" />
      </div>

      <template v-else>
      <div v-if="tab === 'tracks'" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="t in filteredTracks" :key="`${t.key}:${t.config}`" class="overflow-hidden rounded-md border border-line">
          <img
            :src="`/api/track/preview/${t.key}${t.config ? '/' + t.config : ''}`"
            alt=""
            loading="lazy"
            class="aspect-video w-full object-cover"
          />
          <div class="p-2">
            <div class="truncate text-sm font-medium">{{ t.name }}</div>
            <div class="text-xs text-dim">{{ t.config || "default" }} · {{ t.pitboxes }} pits</div>
          </div>
        </div>
      </div>

      <div v-else-if="tab === 'cars'" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="c in filteredCars" :key="c.key" class="overflow-hidden rounded-md border border-line">
          <img
            v-if="c.skins?.length"
            :src="`/api/car/image/${c.key}/${c.skins[0].key}`"
            alt=""
            loading="lazy"
            class="aspect-video w-full object-cover"
          />
          <div class="p-2">
            <div class="truncate text-sm font-medium">{{ c.name }}</div>
            <div class="text-xs text-dim">{{ c.brand }} · {{ c.skins?.length ?? 0 }} skins</div>
          </div>
        </div>
      </div>

      <div v-else class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="w in filteredWeathers" :key="w.key" class="rounded-md border border-line p-2 text-sm">
          {{ w.name }}
        </div>
      </div>

      <EmptyState
        v-if="!content.tracks.length && !content.cars.length && !content.weathers.length"
        icon="content"
        title="No content cached yet"
        message="Set the Assetto Corsa install path, then rebuild the cache to import tracks, cars and weather. You can also upload an archive."
      >
        <Button variant="dark" @click="recache">Rebuild cache</Button>
      </EmptyState>
      </template>
    </Card>

    <!-- Upload & jobs -->
    <div class="space-y-4">
      <Card v-if="config" title="Installation">
        <FormRow
          label="Assetto Corsa install path"
          for-id="installpath"
          hint="Folder containing server/acServer — e.g. .../steamapps/common/assettocorsa (or /corsa in docker)"
        >
          <div class="flex gap-1">
            <Input id="installpath" v-model="config.install_path" class="flex-1" />
            <Button variant="dark" size="sm" @click="validatePath">
              <Icon v-if="pathValid === true" name="check" :size="14" />
              <Icon v-else-if="pathValid === false" name="x" :size="14" />
              <span>{{ pathValid === null ? "Check" : pathValid ? "Valid" : "Invalid" }}</span>
            </Button>
          </div>
        </FormRow>

        <Toggle v-model="cspRequired" label="Require Custom Shaders Patch (CSP)" />
        <div v-if="cspRequired" class="mt-3">
          <FormRow label="Minimum CSP version" for-id="cspver" hint="Build number, e.g. 3155">
            <Input id="cspver" v-model="config.csp_version" type="number" :min="0" />
          </FormRow>
          <div class="grid gap-2 sm:grid-cols-2">
            <Toggle v-model="cspPhycars" label="Extended car physics" />
            <Toggle v-model="cspPhytracks" label="Extended track physics" />
            <Toggle v-model="cspHidepit" label="Hide pitboxes" />
          </div>
        </div>

        <Button class="mt-3" @click="saveInstall">Save installation</Button>
      </Card>

      <Card title="Upload content">
        <FormRow label="Type" for-id="kind">
          <Select
            id="kind"
            v-model="kind"
            :options="[
              { value: 'track', label: 'Track' },
              { value: 'car', label: 'Car' },
            ]"
          />
        </FormRow>
        <FormRow label="Archive file" hint="zip / 7z / rar, up to 2 GB">
          <input
            type="file"
            accept=".zip,.7z,.rar"
            class="w-full text-sm text-muted file:mr-3 file:rounded-md file:border file:border-line file:bg-surface-2 file:px-3 file:py-1.5 file:text-sm file:text-text"
            @change="onFileChange"
          />
        </FormRow>
        <FormRow label="…or download URL" for-id="url">
          <Input id="url" v-model="archiveUrl" placeholder="https://…" />
        </FormRow>
        <div class="mb-3">
          <Toggle v-model="overwrite" label="Overwrite existing files" />
        </div>

        <div v-if="uploading" class="mb-3 h-1.5 overflow-hidden rounded-full bg-surface-3">
          <div class="h-full bg-accent transition-all" :style="{ width: uploadProgress + '%' }" />
        </div>

        <div class="flex gap-2">
          <Button :disabled="uploading" @click="upload">
            {{ uploading ? `Uploading ${uploadProgress}%` : "Upload" }}
          </Button>
          <Button variant="dark" @click="recache">Rebuild cache</Button>
        </div>
      </Card>

      <Card v-if="content.jobList.length" title="Import jobs">
        <div v-for="job in content.jobList" :key="job.id" class="mb-3 last:mb-0">
          <div class="flex items-baseline justify-between gap-2 text-sm">
            <span class="truncate">{{ job.source_name || job.kind }}</span>
            <span class="shrink-0 text-xs" :class="jobTone(job.status)">{{ job.status }}</span>
          </div>
          <div class="mt-1 h-1 overflow-hidden rounded-full bg-surface-3">
            <div
              class="h-full transition-all"
              :class="job.status === 'failed' ? 'bg-danger' : 'bg-accent'"
              :style="{ width: job.progress + '%' }"
            />
          </div>
          <div class="mt-0.5 truncate text-xs text-dim">{{ job.message }}</div>
        </div>
      </Card>
    </div>
  </div>
</template>
