<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useContentStore, type ContentJob } from "@/stores/content";
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
import AdminBackButton from "@/components/AdminBackButton.vue";

const content = useContentStore();
const toast = useToastStore();
const confirm = useConfirmStore();
const router = useRouter();
const libraryLoading = ref(true);

onMounted(() => {
  content.load().finally(() => (libraryLoading.value = false));
  void content.loadImageStats();
  void content.loadJobs().catch(() => undefined);
});

// --- Delete content (removes from disk + cache) ---
// The three kinds differ only in noun and store method, so they share one flow.
async function deleteContent(
  noun: "track" | "car" | "weather",
  item: { name?: string; key?: string },
  remove: (key: string) => Promise<unknown>,
) {
  const label = item.name || item.key;
  if (
    !(await confirm.ask({
      title: `Delete ${noun}?`,
      message: `Permanently delete "${label}"${noun === "track" ? " and all its layouts" : ""} from disk?`,
      detail: `This removes the ${noun} folder from your Assetto Corsa install and cannot be undone.`,
      confirmLabel: `Delete ${noun}`,
      cancelLabel: "Keep",
      tone: "danger",
    }))
  )
    return;
  try {
    await remove(item.key!);
    toast.success(`Deleted ${label}.`);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

const deleteTrack = (t: CacheTrack) => deleteContent("track", t, (k) => content.deleteTrack(k));
const deleteCar = (c: CacheCar) => deleteContent("car", c, (k) => content.deleteCar(k));
const deleteWeather = (w: CacheWeather) => deleteContent("weather", w, (k) => content.deleteWeather(k));

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
const fileInput = ref<HTMLInputElement | null>(null);
const file = ref<File | null>(null);
const uploadProgress = ref(0);
const uploading = ref(false);
type UploadStage = "idle" | "submitting" | "uploading" | "processing" | "queued" | "completed" | "failed";
interface UploadResponse {
  async?: boolean;
  error?: { message?: string };
  job?: ContentJob;
  message?: string;
  imported_count?: number;
  imported_assets?: string[];
  files_written?: number;
  tracks_total?: number;
  cars_total?: number;
  weathers_total?: number;
  cached_images?: number;
}

const uploadStage = ref<UploadStage>("idle");
const uploadMessage = ref("");
const uploadDetail = ref("");

const uploadStatusVisible = computed(() => uploadStage.value !== "idle" || uploading.value);
const uploadStatusTitle = computed(() => {
  if (uploadStage.value === "failed") return "Import failed";
  if (uploadStage.value === "completed") return "Import complete";
  if (uploadStage.value === "queued") return "Background import started";
  if (uploadStage.value === "processing") return "Server is processing the archive";
  if (uploadStage.value === "submitting") return "Starting import";
  return "Uploading archive";
});
const uploadStatusIcon = computed(() => {
  if (uploadStage.value === "failed") return "alert";
  if (uploadStage.value === "completed") return "check";
  if (uploadStage.value === "queued") return "download";
  return "upload";
});
const uploadStatusClass = computed(() => {
  if (uploadStage.value === "failed") return "border-danger/45 bg-danger-glow text-danger";
  if (uploadStage.value === "completed") return "border-ok/40 bg-ok-glow text-ok";
  return "border-accent/40 bg-accent-dim text-text";
});
const uploadBarClass = computed(() => (uploadStage.value === "completed" ? "bg-ok" : "bg-accent"));
const uploadButtonLabel = computed(() => {
  if (!uploading.value) return "Upload";
  if (uploadStage.value === "processing") return "Importing…";
  if (archiveUrl.value.trim()) return "Starting…";
  return `Uploading ${uploadProgress.value}%`;
});

function onFileChange(e: Event) {
  file.value = (e.target as HTMLInputElement).files?.[0] ?? null;
}

function clearUploadInputs() {
  file.value = null;
  archiveUrl.value = "";
  if (fileInput.value) fileInput.value.value = "";
}

function setUploadStatus(stage: UploadStage, message: string, detail = "") {
  uploadStage.value = stage;
  uploadMessage.value = message;
  uploadDetail.value = detail;
}

function summarizeUploadResponse(r: UploadResponse | null): string {
  const parts: string[] = [];
  if (typeof r?.imported_count === "number") parts.push(`${r.imported_count} item${r.imported_count === 1 ? "" : "s"} imported`);
  if (typeof r?.files_written === "number") parts.push(`${r.files_written} file${r.files_written === 1 ? "" : "s"} written`);
  if (typeof r?.cached_images === "number") parts.push(`${r.cached_images} image${r.cached_images === 1 ? "" : "s"} cached`);
  if (typeof r?.tracks_total === "number" && typeof r?.cars_total === "number") {
    parts.push(`Library now has ${r.tracks_total} tracks and ${r.cars_total} cars`);
  }
  return parts.join(" · ") || "The content library is refreshing.";
}

function uploadFailureMessage(xhr: XMLHttpRequest, r: UploadResponse | null): string {
  if (xhr.status === 0) return "Connection lost while sending the archive.";
  return r?.message ?? r?.error?.message ?? "Upload failed.";
}

// XHR instead of fetch: upload progress events. Job progress after the
// upload itself arrives over SSE (content store).
function upload() {
  const url = archiveUrl.value.trim();
  const selectedFile = file.value;
  if (!selectedFile && !url) {
    toast.error("Choose an archive file or paste a download URL.");
    return;
  }
  uploading.value = true;
  uploadProgress.value = 0;
  const sourceName = url || selectedFile?.name || "archive";
  const usingUrl = Boolean(url);
  setUploadStatus(
    usingUrl ? "submitting" : "uploading",
    usingUrl ? `Asking Server Manager to download ${sourceName}.` : `Uploading ${sourceName} to Server Manager.`,
    usingUrl
      ? `The server will download, extract, and cache the ${kind.value} archive in the background.${selectedFile ? " The selected file is ignored while a URL is set." : ""}`
      : "Keep this page open until the browser upload reaches 100%. Server-side import starts after the archive is received.",
  );

  const data = new FormData();
  data.append("kind", kind.value);
  data.append("overwrite", overwrite.value ? "1" : "0");
  if (url) {
    data.append("archive_url", url);
  } else if (selectedFile) {
    data.append("archive", selectedFile);
  }

  const xhr = new XMLHttpRequest();
  xhr.open("POST", "/api/content/upload");
  xhr.setRequestHeader("X-CSRF-Token", csrfToken());
  xhr.responseType = "json";
  xhr.upload.addEventListener("progress", (e) => {
    if (!usingUrl && e.lengthComputable) {
      uploadProgress.value = Math.round((e.loaded / e.total) * 100);
      setUploadStatus("uploading", `Uploading ${sourceName} to Server Manager.`, "Server-side import starts after the archive is received.");
    }
  });
  xhr.upload.addEventListener("load", () => {
    if (!usingUrl) {
      uploadProgress.value = 100;
      setUploadStatus(
        "processing",
        "Archive received. Server Manager is importing content.",
        "Large archives can sit here while files are extracted, previews are optimized, and the content cache is rebuilt.",
      );
    }
  });
  xhr.addEventListener("loadend", () => {
    uploading.value = false;
    const response = xhr.response as UploadResponse | null;
    if (xhr.status >= 200 && xhr.status < 300) {
      if (response?.async && response.job) {
        content.upsertJob(response.job);
        uploadProgress.value = jobProgress(response.job);
        setUploadStatus(
          "queued",
          `${response.job.source_name || sourceName} is queued for import.`,
          "The Import jobs panel updates as Server Manager downloads, extracts, and rebuilds the content cache.",
        );
        toast.info("Import job started. Progress is visible in Import jobs.");
      } else {
        uploadProgress.value = 100;
        setUploadStatus("completed", response?.message ?? `Imported ${sourceName}.`, summarizeUploadResponse(response));
        if (typeof response?.cached_images === "number") content.cachedImages = response.cached_images;
        void content.load(true);
        void content.loadImageStats();
        content.bumpImageVersion();
        toast.success(response?.message ?? "Content imported.");
      }
      clearUploadInputs();
    } else {
      const message = uploadFailureMessage(xhr, response);
      setUploadStatus("failed", message, "The import did not complete. Fix the issue and try again.");
      toast.error(message);
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

// Empty-state CTAs: install path belongs in the guided setup flow; upload stays
// here in the right column.
function focusInstall() {
  void router.push({ name: "setup", query: { step: "install" } });
}
function focusUpload() {
  document.getElementById("upload-content")?.scrollIntoView({ behavior: "smooth", block: "center" });
}

function jobProgress(job: Pick<ContentJob, "progress">): number {
  return Math.min(100, Math.max(0, job.progress || 0));
}

function jobPhaseLabel(phase: string): string {
  const labels: Record<string, string> = {
    queued: "Queued",
    downloading: "Downloading",
    extracting: "Extracting",
    recaching: "Rebuilding cache",
    completed: "Completed",
    failed: "Failed",
  };
  return labels[phase] ?? phase;
}

function jobStatusLabel(job: ContentJob): string {
  if (job.status === "running") return `${jobPhaseLabel(job.phase)} ${jobProgress(job)}%`;
  if (job.status === "queued") return "Queued";
  return jobPhaseLabel(job.status);
}

function jobBadgeClass(status: string): string {
  if (status === "completed") return "border-ok/40 bg-ok-glow text-ok";
  if (status === "failed") return "border-danger/45 bg-danger-glow text-danger";
  return "border-accent/40 bg-accent-dim text-accent";
}

function jobSourceLabel(job: ContentJob): string {
  return job.source === "url" ? "Download URL" : "Browser upload";
}

function jobMeta(job: ContentJob): string[] {
  const parts: string[] = [];
  if (job.download_total_bytes > 0) {
    parts.push(`${formatBytes(job.downloaded_bytes)} / ${formatBytes(job.download_total_bytes)} downloaded`);
  } else if (job.downloaded_bytes > 0) {
    parts.push(`${formatBytes(job.downloaded_bytes)} downloaded`);
  }
  if (job.files_written > 0) parts.push(`${job.files_written} file${job.files_written === 1 ? "" : "s"} written`);
  if (job.imported_assets?.length) {
    const shown = job.imported_assets.slice(0, 2).join(", ");
    const extra = job.imported_assets.length > 2 ? ` +${job.imported_assets.length - 2} more` : "";
    parts.push(`${job.imported_assets.length} item${job.imported_assets.length === 1 ? "" : "s"}: ${shown}${extra}`);
  }
  if (job.tracks_total || job.cars_total || job.weathers_total) {
    parts.push(`Library: ${job.tracks_total} tracks, ${job.cars_total} cars, ${job.weathers_total} weather`);
  }
  return parts;
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
    <template #prefix>
      <AdminBackButton />
    </template>
    <template #actions>
      <span class="text-xs text-dim">{{ content.cachedImages }} images cached</span>
      <Button variant="dark" :disabled="recaching || compressing" @click="recache">
        <Icon name="repeat" :size="15" />
        {{ recaching ? "Rebuilding…" : "Rebuild cache" }}
      </Button>
      <Button variant="ghost" :disabled="recaching || compressing" @click="compressImages">
        <Icon name="minimize" :size="15" />
        {{ compressing ? "Optimizing…" : "Optimize images" }}
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
        <FormRow label="Archive file" hint="zip / 7z / rar, up to 10 GB">
          <input
            ref="fileInput"
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

        <div v-if="uploadStatusVisible" class="mb-3 rounded-md border px-3 py-2.5" :class="uploadStatusClass">
          <div class="flex items-start gap-2">
            <Icon :name="uploadStatusIcon" :size="15" class="mt-0.5 shrink-0" />
            <div class="min-w-0 flex-1">
              <div class="flex items-center justify-between gap-2">
                <p class="text-sm font-semibold">{{ uploadStatusTitle }}</p>
                <span v-if="uploadStage !== 'failed'" class="shrink-0 font-mono text-xs tabular-nums">{{ uploadProgress }}%</span>
              </div>
              <p class="mt-0.5 text-xs opacity-90">{{ uploadMessage }}</p>
              <p v-if="uploadDetail" class="mt-1 text-xs opacity-75">{{ uploadDetail }}</p>
            </div>
          </div>
          <div v-if="uploadStage !== 'failed'" class="mt-2 h-1.5 overflow-hidden rounded-full bg-surface-3">
            <div class="h-full transition-all duration-200" :class="uploadBarClass" :style="{ width: uploadProgress + '%' }" />
          </div>
        </div>

        <div class="flex gap-2">
          <Button :disabled="uploading" @click="upload">
            <Icon :name="archiveUrl.trim() ? 'download' : 'upload'" :size="15" />
            {{ uploadButtonLabel }}
          </Button>
        </div>
      </Card>

      <Card v-if="content.jobList.length" title="Import jobs">
        <div v-for="job in content.jobList" :key="job.id" class="mb-3 rounded-md border border-line bg-surface-2/35 p-3 last:mb-0">
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0">
              <div class="truncate text-sm font-semibold">{{ job.source_name || job.kind }}</div>
              <div class="mt-0.5 flex flex-wrap gap-x-2 gap-y-1 text-[11px] text-dim">
                <span class="capitalize">{{ job.kind }}</span>
                <span>{{ jobSourceLabel(job) }}</span>
                <span>{{ jobPhaseLabel(job.phase) }}</span>
              </div>
            </div>
            <span class="shrink-0 rounded-full border px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase" :class="jobBadgeClass(job.status)">
              {{ jobStatusLabel(job) }}
            </span>
          </div>
          <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-surface-3">
            <div
              class="h-full transition-all"
              :class="job.status === 'failed' ? 'bg-danger' : 'bg-accent'"
              :style="{ width: jobProgress(job) + '%' }"
            />
          </div>
          <div class="mt-1.5 text-xs text-muted">{{ job.message }}</div>
          <div v-if="jobMeta(job).length" class="mt-2 flex flex-wrap gap-1.5 text-[11px] text-dim">
            <span v-for="part in jobMeta(job)" :key="part" class="rounded border border-line bg-surface px-1.5 py-0.5">{{ part }}</span>
          </div>
        </div>
      </Card>
    </div>
  </div>
</template>
