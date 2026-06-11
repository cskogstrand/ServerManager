<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useContentStore } from "@/stores/content";
import { api, ApiError, csrfToken } from "@/lib/api";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Toggle from "@/components/ui/Toggle.vue";

const content = useContentStore();
onMounted(() => void content.load());

const tab = ref<"tracks" | "cars" | "weathers">("tracks");
const search = ref("");
const error = ref("");
const notice = ref("");

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
    error.value = "Choose an archive file or paste a download URL.";
    return;
  }
  error.value = "";
  notice.value = "";
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
      notice.value = "Upload accepted — import progress shows below.";
      file.value = null;
      archiveUrl.value = "";
    } else {
      error.value = xhr.response?.message ?? xhr.response?.error?.message ?? "Upload failed.";
    }
  });
  xhr.send(data);
}

async function recache() {
  error.value = "";
  notice.value = "";
  try {
    await api.post("/api/content/recache");
    await content.load(true);
    notice.value = "Content cache rebuilt.";
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  }
}

function jobTone(status: string) {
  if (status === "completed") return "text-ok";
  if (status === "failed") return "text-danger";
  return "text-accent";
}
</script>

<template>
  <h1 class="mb-5 text-xl font-bold">Content</h1>

  <p v-if="notice" class="mb-4 rounded-md border border-ok/40 bg-ok-glow px-3 py-2 text-sm text-ok">{{ notice }}</p>
  <p v-if="error" class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger">
    {{ error }}
  </p>

  <div class="grid items-start gap-5 xl:grid-cols-[1fr_360px]">
    <!-- Library -->
    <Card>
      <template #header>
        <div class="flex gap-1">
          <button
            v-for="t in (['tracks', 'cars', 'weathers'] as const)"
            :key="t"
            type="button"
            class="rounded-md px-3 py-1 text-sm capitalize"
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
    </Card>

    <!-- Upload & jobs -->
    <div class="space-y-4">
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
