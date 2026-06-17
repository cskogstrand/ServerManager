<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useContentStore } from "@/stores/content";
import { api, ApiError, csrfToken } from "@/lib/api";
import { useQueryParam, enumParam } from "@/lib/useQueryParam";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import type { CacheCar, CacheTrack, CacheWeather } from "@/types/generated";
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
import TrackImage from "@/components/TrackImage.vue";

const content = useContentStore();
const toast = useToastStore();
const confirm = useConfirmStore();
const router = useRouter();
const libraryLoading = ref(true);

onMounted(() => {
  content.load().finally(() => (libraryLoading.value = false));
  void content.loadImageStats();
});

// --- Delete content (removes from disk + cache) ---
async function deleteTrack(t: CacheTrack) {
  if (
    !(await confirm.ask({
      title: "Delete track?",
      message: `Permanently delete "${t.name || t.key}" and all its layouts from disk?`,
      detail: "This removes the track folder from your Assetto Corsa install and cannot be undone.",
      confirmLabel: "Delete track",
      cancelLabel: "Keep",
      tone: "danger",
    }))
  )
    return;
  try {
    await content.deleteTrack(t.key!);
    toast.success(`Deleted ${t.name || t.key}.`);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

async function deleteCar(carItem: CacheCar) {
  if (
    !(await confirm.ask({
      title: "Delete car?",
      message: `Permanently delete "${carItem.name || carItem.key}" from disk?`,
      detail: "This removes the car folder from your Assetto Corsa install and cannot be undone.",
      confirmLabel: "Delete car",
      cancelLabel: "Keep",
      tone: "danger",
    }))
  )
    return;
  try {
    await content.deleteCar(carItem.key!);
    toast.success(`Deleted ${carItem.name || carItem.key}.`);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

async function deleteWeather(w: CacheWeather) {
  if (
    !(await confirm.ask({
      title: "Delete weather?",
      message: `Permanently delete "${w.name || w.key}" from disk?`,
      detail: "This removes the weather folder from your Assetto Corsa install and cannot be undone.",
      confirmLabel: "Delete weather",
      cancelLabel: "Keep",
      tone: "danger",
    }))
  )
    return;
  try {
    await content.deleteWeather(w.key!);
    toast.success(`Deleted ${w.name || w.key}.`);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

// Library tab and search mirrored to the URL (?tab, ?q).
const tab = useQueryParam<"tracks" | "cars" | "weathers">(
  "tab",
  "tracks",
  enumParam(["tracks", "cars", "weathers"] as const, "tracks"),
);
const search = useQueryParam("q", "");

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
      // Synchronous (file) imports auto-compress before responding; async (URL)
      // imports refresh again on job completion. Bust image URLs either way.
      void content.loadImageStats();
      content.bumpImageVersion();
    } else {
      toast.error(xhr.response?.message ?? xhr.response?.error?.message ?? "Upload failed.");
    }
  });
  xhr.send(data);
}

const recaching = ref(false);
const compressing = ref(false);

async function recache() {
  recaching.value = true;
  try {
    const r = await api.post<{ cached_images: number }>("/api/content/recache");
    await content.load(true);
    content.cachedImages = r.cached_images ?? content.cachedImages;
    toast.success("Content cache rebuilt.");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    recaching.value = false;
  }
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** i).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

// Downscale + re-encode all car/track preview images into the DB cache, which
// the image endpoints then serve in preference to the full-size originals.
async function compressImages() {
  compressing.value = true;
  try {
    const r = await api.post<{ images: number; cached: number; src_bytes: number; out_bytes: number }>(
      "/api/content/compress",
    );
    content.cachedImages = r.cached;
    content.bumpImageVersion();
    const saved = Math.max(0, r.src_bytes - r.out_bytes);
    toast.success(`Compressed ${r.images} images — saved ${formatBytes(saved)}.`);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    compressing.value = false;
  }
}

// Empty-state CTAs: install path now lives on the Configuration page; upload
// stays here in the right column.
function focusInstall() {
  void router.push("/settings");
}
function focusUpload() {
  document.getElementById("upload-content")?.scrollIntoView({ behavior: "smooth", block: "center" });
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
    subtitle="Browse installed tracks, cars, and weather; upload, delete, or rebuild the content cache."
    icon="content"
  >
    <!-- Global content actions — apply across tracks, cars and weather, so they
         live here rather than inside the library's tab selector. -->
    <template #actions>
      <span class="text-xs text-dim">{{ content.cachedImages }} images cached</span>
      <Button variant="dark" :disabled="recaching || compressing" @click="recache">
        <Icon name="repeat" :size="15" />
        {{ recaching ? "Rebuilding…" : "Rebuild cache" }}
      </Button>
      <Button variant="dark" :disabled="recaching || compressing" @click="compressImages">
        <Icon name="minimize" :size="15" />
        {{ compressing ? "Compressing…" : "Compress images" }}
      </Button>
    </template>
  </PageHeader>

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
        <div v-for="t in filteredTracks" :key="`${t.key}:${t.config}`" class="group relative overflow-hidden rounded-md border border-line">
          <TrackImage
            :track-key="t.key"
            :config="t.config"
            class="aspect-video w-full"
          />
          <button
            type="button"
            class="absolute top-1.5 right-1.5 grid size-7 cursor-pointer place-items-center rounded-md bg-surface/80 text-muted opacity-0 backdrop-blur transition group-hover:opacity-100 hover:bg-danger hover:text-white focus:opacity-100"
            :aria-label="`Delete ${t.name || t.key}`"
            @click="deleteTrack(t)"
          >
            <Icon name="trash" :size="15" />
          </button>
          <div class="p-2">
            <div class="truncate text-sm font-medium">{{ t.name }}</div>
            <div class="text-xs text-dim">{{ t.config || "default" }} · {{ t.pitboxes }} pits</div>
          </div>
        </div>
      </div>

      <div v-else-if="tab === 'cars'" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="c in filteredCars" :key="c.key" class="group relative overflow-hidden rounded-md border border-line">
          <img
            v-if="c.skins?.length"
            :src="`/api/car/image/${c.key}/${c.skins[0].key}?v=${content.imageVersion}`"
            alt=""
            loading="lazy"
            class="aspect-video w-full object-cover"
          />
          <button
            type="button"
            class="absolute top-1.5 right-1.5 grid size-7 cursor-pointer place-items-center rounded-md bg-surface/80 text-muted opacity-0 backdrop-blur transition group-hover:opacity-100 hover:bg-danger hover:text-white focus:opacity-100"
            :aria-label="`Delete ${c.name || c.key}`"
            @click="deleteCar(c)"
          >
            <Icon name="trash" :size="15" />
          </button>
          <div class="p-2">
            <div class="truncate text-sm font-medium">{{ c.name }}</div>
            <div class="text-xs text-dim">{{ c.brand }} · {{ c.skins?.length ?? 0 }} skins</div>
          </div>
        </div>
      </div>

      <div v-else class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="w in filteredWeathers" :key="w.key" class="flex items-center gap-2 rounded-md border border-line p-2 text-sm">
          <span class="min-w-0 flex-1 truncate">{{ w.name }}</span>
          <button
            type="button"
            class="grid size-7 shrink-0 cursor-pointer place-items-center rounded-md text-muted transition hover:bg-danger hover:text-white"
            :aria-label="`Delete ${w.name || w.key}`"
            @click="deleteWeather(w)"
          >
            <Icon name="trash" :size="15" />
          </button>
        </div>
      </div>

      <EmptyState
        v-if="!content.tracks.length && !content.cars.length && !content.weathers.length"
        icon="content"
        title="No content cached yet"
        message="Point Server Manager at your Assetto Corsa install and rebuild the cache to import tracks, cars and weather — or upload an archive."
      >
        <div class="flex flex-wrap justify-center gap-2">
          <Button @click="focusInstall">
            <Icon name="settings" :size="15" />
            Set install path
          </Button>
          <Button variant="dark" @click="recache">
            <Icon name="repeat" :size="15" />
            Rebuild cache
          </Button>
          <Button variant="ghost" @click="focusUpload">
            <Icon name="plus" :size="15" />
            Upload content
          </Button>
        </div>
      </EmptyState>
      </template>
    </Card>

    <!-- Upload & jobs -->
    <div class="space-y-4">
      <Card id="upload-content" title="Upload content">
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
        </div>
      </Card>

      <Card v-if="content.jobList.length" title="Import jobs">
        <div v-for="job in content.jobList" :key="job.id" class="mb-3 last:mb-0">
          <div class="flex items-baseline justify-between gap-2 text-sm">
            <span class="min-w-0 truncate">{{ job.source_name || job.kind }}</span>
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
