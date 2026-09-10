<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useContentStore, type ContentJob } from "@/stores/content";
import { api, ApiError, csrfToken } from "@/lib/api";
import { resolveUploadArchiveKind, type ContentArchiveKind } from "@/lib/contentArchiveKind";
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
import Sheet from "@/components/ui/Sheet.vue";
import LineChart from "@/components/ui/LineChart.vue";
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

function defaultSortForTab(value = tab.value): string {
  return value === "weathers" ? "name" : "newest";
}

const sort = ref<string>(defaultSortForTab());
const carBrand = ref("");
const carClass = ref("");
const minPower = ref<number | null>(null);
const trackCountry = ref("");
const minPitboxes = ref<number | null>(null);

const sortOptions = computed(() => {
  const base = [
    { value: "name", label: "Name" },
    { value: "newest", label: "Newest" },
    { value: "oldest", label: "Oldest" },
  ];
  if (tab.value === "cars") {
    return [
      ...base,
      { value: "brand", label: "Brand" },
      { value: "powerDesc", label: "Power high" },
      { value: "powerAsc", label: "Power low" },
    ];
  }
  if (tab.value === "tracks") {
    return [
      ...base,
      { value: "lengthDesc", label: "Length long" },
      { value: "lengthAsc", label: "Length short" },
      { value: "pitboxesDesc", label: "Pit boxes high" },
      { value: "pitboxesAsc", label: "Pit boxes low" },
    ];
  }
  return [{ value: "name", label: "Name" }];
});

watch(tab, (next, previous) => {
  if (!sortOptions.value.some((o) => o.value === sort.value) || sort.value === defaultSortForTab(previous)) {
    sort.value = defaultSortForTab(next);
  }
});

const carBrandOptions = computed(() => optionList("All brands", content.cars.map((c) => c.brand)));
const carClassOptions = computed(() => optionList("All classes", content.cars.map((c) => c.class)));
const trackCountryOptions = computed(() => optionList("All countries", content.tracks.map((t) => t.country)));

const filteredTracks = computed(() => {
  const q = search.value.toLowerCase();
  const rows = content.tracks.filter((t) => {
    if (trackCountry.value && t.country !== trackCountry.value) return false;
    if (minPitboxes.value !== null && (t.pitboxes ?? 0) < minPitboxes.value) return false;
    return searchText([t.name, t.key, t.config, t.version, t.country, t.city, t.tags?.join(" ")]).includes(q);
  });
  return sortRows(rows, trackSortValue);
});
const filteredCars = computed(() => {
  const q = search.value.toLowerCase();
  const rows = content.cars.filter((c) => {
    if (carBrand.value && c.brand !== carBrand.value) return false;
    if (carClass.value && c.class !== carClass.value) return false;
    if (minPower.value !== null && carPower(c) < minPower.value) return false;
    return searchText([c.name, c.key, c.brand, c.class, c.tags?.join(" ")]).includes(q);
  });
  return sortRows(rows, carSortValue);
});
const filteredWeathers = computed(() => {
  const q = search.value.toLowerCase();
  return content.weathers
    .filter((w) => searchText([w.name, w.key]).includes(q))
    .sort((a, b) => byText(a.name ?? a.key, b.name ?? b.key));
});
const visibleCount = computed(() => {
  if (tab.value === "tracks") return filteredTracks.value.length;
  if (tab.value === "cars") return filteredCars.value.length;
  return filteredWeathers.value.length;
});
const filtersActive = computed(() => {
  if (sort.value !== defaultSortForTab()) return true;
  if (tab.value === "cars") return Boolean(carBrand.value || carClass.value || minPower.value !== null);
  if (tab.value === "tracks") return Boolean(trackCountry.value || minPitboxes.value !== null);
  return false;
});

function optionList(label: string, values: (string | undefined | null)[]) {
  const unique = Array.from(new Set(values.map((v) => (v ?? "").trim()).filter(Boolean))).sort((a, b) => byText(a, b));
  return [{ value: "", label }, ...unique.map((value) => ({ value, label: value }))];
}

function searchText(values: (string | undefined | null)[]): string {
  return values.filter(Boolean).join(" ").toLowerCase();
}

function byText(a?: string | null, b?: string | null): number {
  return (a || "").localeCompare(b || "", undefined, { sensitivity: "base", numeric: true });
}

function numericSpec(value?: string): number {
  const match = (value || "").replace(",", ".").match(/\d+(\.\d+)?/);
  return match ? Number(match[0]) : 0;
}

function carPower(c: CacheCar): number {
  return numericSpec(c.specs?.bhp);
}

function carPowerLabel(c: CacheCar): string {
  const raw = c.specs?.bhp?.trim();
  return raw && raw !== "-" ? raw : "";
}

function modifiedAt(row: { modified_at?: number }): number {
  return row.modified_at || 0;
}

function sortRows<T>(rows: T[], value: (row: T) => string | number): T[] {
  return [...rows].sort((a, b) => {
    const av = value(a);
    const bv = value(b);
    if (typeof av === "number" && typeof bv === "number") return av === bv ? 0 : av - bv;
    return byText(String(av), String(bv));
  });
}

function carSortValue(c: CacheCar): string | number {
  if (sort.value === "newest") return -modifiedAt(c);
  if (sort.value === "oldest") return modifiedAt(c);
  if (sort.value === "brand") return `${c.brand || ""} ${c.name || c.key || ""}`;
  if (sort.value === "powerDesc") return -carPower(c);
  if (sort.value === "powerAsc") return carPower(c);
  return c.name || c.key || "";
}

function trackSortValue(t: CacheTrack): string | number {
  if (sort.value === "newest") return -modifiedAt(t);
  if (sort.value === "oldest") return modifiedAt(t);
  if (sort.value === "lengthDesc") return -(t.length || 0);
  if (sort.value === "lengthAsc") return t.length || 0;
  if (sort.value === "pitboxesDesc") return -(t.pitboxes || 0);
  if (sort.value === "pitboxesAsc") return t.pitboxes || 0;
  return t.name || t.key || "";
}

function formatDate(ts?: number): string {
  if (!ts) return "";
  return new Date(ts * 1000).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

function clearLibraryFilters() {
  sort.value = defaultSortForTab();
  carBrand.value = "";
  carClass.value = "";
  minPower.value = null;
  trackCountry.value = "";
  minPitboxes.value = null;
}

// --- Detail drawers ---
interface CarCurves {
  key: string;
  desc: string;
  power: number[];
  torque: number[];
  labels: number[];
}

const selectedTrack = ref<CacheTrack | null>(null);
const selectedCar = ref<CacheCar | null>(null);
const carCurves = ref<CarCurves | null>(null);
const carCurvesLoading = ref(false);
const curveCache = new Map<string, CarCurves>();

function trackLengthLabel(t?: CacheTrack | null): string {
  if (!t?.length) return "—";
  return t.length >= 1000 ? `${(t.length / 1000).toFixed(2)} km` : `${t.length} m`;
}

const trackLocation = computed(() => {
  const t = selectedTrack.value;
  return [t?.city, t?.country].filter(Boolean).join(" · ");
});

const trackSpecRows = computed(() => {
  const t = selectedTrack.value;
  if (!t) return [];
  return [
    { label: "Layout", value: t.config || "default" },
    { label: "Version", value: t.version || "—" },
    { label: "Length", value: trackLengthLabel(t) },
    { label: "Pit boxes", value: t.pitboxes ?? "—" },
    { label: "Width", value: t.width || "—" },
    { label: "Run", value: t.run || "—" },
    { label: "Modified", value: formatDate(t.modified_at) || "—" },
    { label: "Key", value: t.key || "—" },
  ];
});

function carPreviewUrl(c: CacheCar | null): string {
  const key = c?.key;
  const skin = c?.skins?.[0]?.key;
  return key && skin ? `/api/car/image/${encodeURIComponent(key)}/${encodeURIComponent(skin)}?v=${content.imageVersion}` : "";
}

async function openCar(c: CacheCar) {
  selectedCar.value = c;
  const key = c.key;
  carCurves.value = key ? curveCache.get(key) ?? null : null;
  if (!key || carCurves.value) {
    carCurvesLoading.value = false;
    return;
  }
  carCurvesLoading.value = true;
  try {
    const res = await api.get<CarCurves>(`/api/car/${encodeURIComponent(key)}`);
    curveCache.set(key, res);
    if (selectedCar.value?.key === key) carCurves.value = res;
  } catch {
    if (selectedCar.value?.key === key) carCurves.value = null;
  } finally {
    if (!selectedCar.value || selectedCar.value.key === key) carCurvesLoading.value = false;
  }
}

const carSpecRows = computed(() => {
  const c = selectedCar.value;
  const s = c?.specs;
  if (!s) return [];
  return [
    { label: "Power", value: s.bhp },
    { label: "Torque", value: s.torque },
    { label: "Weight", value: s.weight },
    { label: "Top speed", value: s.topspeed },
    { label: "0–100", value: s.acceleration },
    { label: "P/W ratio", value: s.pwratio },
    { label: "Modified", value: formatDate(c?.modified_at) },
  ].filter((r) => r.value);
});

const carChartSeries = computed(() => {
  const c = carCurves.value;
  if (!c) return [];
  const out: { name: string; color: string; values: number[]; unit?: string }[] = [];
  if (c.power?.some((v) => v > 0)) out.push({ name: "Power", color: "var(--color-accent)", values: c.power, unit: " bhp" });
  if (c.torque?.some((v) => v > 0)) out.push({ name: "Torque", color: "var(--color-warn)", values: c.torque, unit: " Nm" });
  return out;
});

const carDescText = computed(() => {
  const raw = carCurves.value?.desc || selectedCar.value?.description;
  if (!raw) return "";
  return raw
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<[^>]+>/g, "")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .trim();
});

// --- Upload ---
const overwrite = ref(false);
const archiveUrl = ref("");
const fileInput = ref<HTMLInputElement | null>(null);
const files = ref<File[]>([]);
const uploadProgress = ref(0);
const uploading = ref(false);
const batchIndex = ref(0);
const batchTotal = ref(0);
type UploadStage = "idle" | "submitting" | "uploading" | "processing" | "queued" | "completed" | "failed";
type UploadItemStage = "scanning" | "needsChoice" | "ready" | "uploading" | "processing" | "queued" | "completed" | "failed";
interface UploadResponse {
  async?: boolean;
  error?: { message?: string };
  job?: ContentJob;
  kind?: "car" | "track";
  message?: string;
  imported_count?: number;
  imported_assets?: string[];
  files_written?: number;
  tracks_total?: number;
  cars_total?: number;
  weathers_total?: number;
  cached_images?: number;
}
interface UploadItem {
  id: string;
  file: File;
  name: string;
  size: number;
  kind: ContentArchiveKind;
  stage: UploadItemStage;
  progress: number;
  message: string;
  detail?: string;
}

const uploadStage = ref<UploadStage>("idle");
const uploadMessage = ref("");
const uploadDetail = ref("");
const uploadItems = ref<UploadItem[]>([]);
let uploadScanVersion = 0;
let activeFileScan: Promise<void> | null = null;
const uploadKindChoiceOptions = [
  { value: "auto", label: "Choose type" },
  { value: "car", label: "Car" },
  { value: "track", label: "Track" },
];

const uploadStatusVisible = computed(() => uploadStage.value !== "idle" || uploading.value);
const scanningFiles = computed(() => uploadItems.value.some((item) => item.stage === "scanning"));
const filesNeedTypeChoice = computed(() => uploadItems.value.some((item) => item.stage === "needsChoice"));
const pendingFileUploadCount = computed(() => uploadItems.value.filter((item) => item.stage !== "completed").length);
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
  if (!uploading.value && !archiveUrl.value.trim() && scanningFiles.value) return "Scanning…";
  if (!uploading.value && !archiveUrl.value.trim() && filesNeedTypeChoice.value) return "Choose type";
  if (!uploading.value) return archiveUrl.value.trim() ? "Start download" : pendingFileUploadCount.value > 1 ? `Upload ${pendingFileUploadCount.value}` : "Upload";
  if (batchTotal.value > 1) return `Importing ${Math.min(batchIndex.value + 1, batchTotal.value)}/${batchTotal.value}`;
  if (uploadStage.value === "processing") return "Importing…";
  if (archiveUrl.value.trim()) return "Starting…";
  return `Uploading ${uploadProgress.value}%`;
});
const selectedFileSummary = computed(() => {
  if (!files.value.length) return "";
  if (files.value.length === 1) return `${files.value[0].name} (${formatBytes(files.value[0].size)})`;
  const totalBytes = files.value.reduce((sum, f) => sum + f.size, 0);
  return `${files.value.length} archives selected (${formatBytes(totalBytes)} total)`;
});

function onFileChange(e: Event) {
  files.value = Array.from((e.target as HTMLInputElement).files ?? []);
  uploadProgress.value = 0;
  setUploadStatus("idle", "");
  activeFileScan = scanUploadItems(files.value);
}

function clearUploadInputs(options: { keepItems?: boolean } = {}) {
  files.value = [];
  archiveUrl.value = "";
  if (!options.keepItems) {
    uploadScanVersion++;
    uploadItems.value = [];
    activeFileScan = null;
  }
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
  return friendlyArchiveError(r?.message ?? r?.error?.message ?? "Upload failed.");
}

function friendlyArchiveError(message: string): string {
  const cleaned = message.replace(/\s+/g, " ").trim();
  const lower = cleaned.toLowerCase();
  if (cleaned.includes("Cannot open the file as archive")) {
    return "The archive could not be opened. It may be corrupt, incomplete, password-protected, an unsupported RAR variant, or one part of a multi-part RAR.";
  }
  if (lower.includes("wrong password") || lower.includes("encrypted")) {
    return "The archive is password-protected or encrypted and cannot be imported.";
  }
  if (lower.includes("unexpected end") || lower.includes("headers error")) {
    return "The archive appears to be incomplete or corrupt.";
  }
  if (cleaned.includes("ERROR:") || cleaned.includes("7-Zip")) {
    return "The archive could not be processed. Check that it is a complete, supported archive.";
  }
  return cleaned || "Upload failed.";
}

function batchProgress(index: number, total: number, fileProgress: number): number {
  if (total <= 1) return fileProgress;
  return Math.min(100, Math.round(((index + fileProgress / 100) / total) * 100));
}

function batchPrefix(index: number, total: number): string {
  return total > 1 ? `Archive ${index + 1} of ${total}: ` : "";
}

async function scanUploadItems(selectedFiles: File[]) {
  const version = ++uploadScanVersion;
  uploadItems.value = selectedFiles.map((file, index) => {
    const zip = isZipArchive(file);
    return {
      id: `${file.name}:${file.size}:${file.lastModified}:${index}`,
      file,
      name: file.name,
      size: file.size,
      kind: "auto",
      stage: zip ? "scanning" : "ready",
      progress: zip ? 8 : 0,
      message: zip ? "Scanning archive for car or track markers." : "Server will detect content type during import.",
    };
  });

  await Promise.all(
    uploadItems.value.map(async (item) => {
      if (!isZipArchive(item.file)) return;
      try {
        const kind = await resolveUploadArchiveKind(item.file);
        if (version !== uploadScanVersion) return;
        item.kind = kind;
        item.stage = "ready";
        item.progress = 0;
        item.message = kind === "auto" ? "Server will detect content type during import." : `Detected ${kind}. Ready to upload.`;
      } catch (e) {
        if (version !== uploadScanVersion) return;
        markUploadItemNeedsChoice(item, e instanceof Error ? e.message : String(e));
      }
    }),
  );
}

function isZipArchive(file: File): boolean {
  return file.name.toLowerCase().endsWith(".zip");
}

function isManualContentChoiceError(message: string): boolean {
  return message.includes("No valid car or track content was found") || message.includes("Archive contains both car and track content");
}

function markUploadItemNeedsChoice(item: UploadItem, detail: string) {
  item.kind = "auto";
  item.stage = "needsChoice";
  item.progress = 0;
  item.message = "Server Manager could not detect the content type. Choose car or track to continue.";
  item.detail = detail;
}

function setUploadItemKind(item: UploadItem, value: string | number | null | undefined) {
  if (value !== "car" && value !== "track") {
    markUploadItemNeedsChoice(item, item.detail || "Choose how this archive should be installed.");
    return;
  }
  item.kind = value;
  item.stage = "ready";
  item.progress = 0;
  item.message = `Using ${value}. Ready to upload.`;
  item.detail = "Manual choice will be sent with the upload.";
  setUploadStatus("idle", "");
}

function canRemoveUploadItem(item: UploadItem): boolean {
  return !uploading.value && (item.stage === "failed" || item.stage === "needsChoice");
}

function removeUploadItem(item: UploadItem) {
  if (!canRemoveUploadItem(item)) return;
  files.value = files.value.filter((file) => file !== item.file);
  uploadItems.value = uploadItems.value.filter((candidate) => candidate.id !== item.id);
  if (fileInput.value) fileInput.value.value = "";

  const hasBlockingItem = uploadItems.value.some((candidate) => candidate.stage === "failed" || candidate.stage === "needsChoice");
  if (!uploadItems.value.length) {
    activeFileScan = null;
    setUploadStatus("idle", "");
  } else if (!hasBlockingItem && uploadStage.value === "failed") {
    setUploadStatus("idle", "");
  }
}

function applyUploadResponse(response: UploadResponse | null, sourceName: string, notify: boolean, item?: UploadItem) {
  if (response?.async && response.job) {
    content.upsertJob(response.job);
    uploadProgress.value = jobProgress(response.job);
    if (item) {
      item.stage = "queued";
      item.progress = jobProgress(response.job);
      item.message = `${response.job.source_name || sourceName} is queued for import.`;
      item.detail = "Progress continues in Import jobs.";
    }
    if (notify) {
      setUploadStatus(
        "queued",
        `${response.job.source_name || sourceName} is queued for import.`,
        "The Import jobs panel updates as Server Manager downloads, extracts, and rebuilds the content cache.",
      );
      toast.info("Import job started. Progress is visible in Import jobs.");
    }
    return;
  }
  if (typeof response?.cached_images === "number") content.cachedImages = response.cached_images;
  content.bumpImageVersion();
  if (item) {
    item.kind = response?.kind ?? item.kind;
    item.stage = "completed";
    item.progress = 100;
    item.message = response?.message ?? `Imported ${sourceName}.`;
    item.detail = summarizeUploadResponse(response);
  }
  if (notify) {
    uploadProgress.value = 100;
    setUploadStatus("completed", response?.message ?? `Imported ${sourceName}.`, summarizeUploadResponse(response));
    toast.success(response?.message ?? "Content imported.");
  }
}

async function sendUploadRequest(opts: { file?: File; url?: string; index: number; total: number; kind?: ContentArchiveKind; item?: UploadItem }): Promise<UploadResponse | null> {
  const sourceName = opts.url || opts.file?.name || "archive";
  const usingUrl = Boolean(opts.url);
  uploadProgress.value = batchProgress(opts.index, opts.total, 0);
  if (opts.item) {
    opts.item.stage = "uploading";
    opts.item.progress = 0;
    opts.item.message = "Uploading archive to Server Manager.";
    opts.item.detail = opts.kind === "auto" ? "Server will validate and detect the content type after upload." : `Browser detected ${opts.kind}.`;
  }
  setUploadStatus(
    usingUrl ? "submitting" : "uploading",
    usingUrl
      ? `Asking Server Manager to download ${sourceName}.`
      : `${batchPrefix(opts.index, opts.total)}Uploading ${sourceName} to Server Manager.`,
    usingUrl
      ? `The server will download, detect, extract, and cache the content archive in the background.${files.value.length ? " Selected files are ignored while a URL is set." : ""}`
      : opts.total > 1
        ? `Selected ${opts.total} archives. Each one is detected and starts after the previous import finishes.`
        : "Keep this page open until the browser upload reaches 100%. Server-side detection and import start after the archive is received.",
  );

  const data = new FormData();
  data.append("kind", opts.kind ?? "auto");
  data.append("overwrite", overwrite.value ? "1" : "0");
  if (opts.url) data.append("archive_url", opts.url);
  else if (opts.file) data.append("archive", opts.file);

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/content/upload");
    xhr.setRequestHeader("X-CSRF-Token", csrfToken());
    xhr.responseType = "json";
    xhr.upload.addEventListener("progress", (e) => {
      if (!usingUrl && e.lengthComputable) {
        const progress = Math.round((e.loaded / e.total) * 100);
        uploadProgress.value = batchProgress(opts.index, opts.total, progress);
        if (opts.item) {
          opts.item.progress = progress;
          opts.item.message = "Uploading archive to Server Manager.";
          opts.item.detail = `${formatBytes(e.loaded)} / ${formatBytes(e.total)} uploaded.`;
        }
        setUploadStatus(
          "uploading",
          `${batchPrefix(opts.index, opts.total)}Uploading ${sourceName} to Server Manager.`,
          opts.total > 1
            ? `${formatBytes(e.loaded)} / ${formatBytes(e.total)} for this archive.`
            : "Server-side import starts after the archive is received.",
        );
      }
    });
    xhr.upload.addEventListener("load", () => {
      if (!usingUrl) {
        uploadProgress.value = batchProgress(opts.index, opts.total, 100);
        if (opts.item) {
          opts.item.stage = "processing";
          opts.item.progress = 100;
          opts.item.message = "Upload received. Server is validating and importing.";
          opts.item.detail = "This can take a while for large archives.";
        }
        setUploadStatus(
          "processing",
          `${batchPrefix(opts.index, opts.total)}Archive received. Server Manager is importing content.`,
          opts.total > 1
            ? "Waiting for this import to finish before the next archive starts."
            : "Large archives can sit here while files are extracted, previews are optimized, and the content cache is rebuilt.",
        );
      }
    });
    xhr.addEventListener("loadend", () => {
      const response = xhr.response as UploadResponse | null;
      if (xhr.status >= 200 && xhr.status < 300) resolve(response);
      else {
        const message = uploadFailureMessage(xhr, response);
        if (opts.item) {
          if (isManualContentChoiceError(message)) {
            markUploadItemNeedsChoice(opts.item, message);
          } else {
            opts.item.stage = "failed";
            opts.item.progress = 0;
            opts.item.message = message;
            opts.item.detail = "The import did not complete.";
          }
        }
        reject(new Error(message));
      }
    });
    xhr.send(data);
  });
}

// XHR instead of fetch: upload progress events. Job progress after the
// upload itself arrives over SSE (content store).
async function upload() {
  if (uploading.value) return;
  const url = archiveUrl.value.trim();
  const selectedFiles = files.value.slice();
  if (!selectedFiles.length && !url) {
    toast.error("Choose one or more archive files or paste a download URL.");
    return;
  }
  if (!url && activeFileScan) await activeFileScan;
  const choicePending = uploadItems.value.find((item) => item.stage === "needsChoice");
  if (!url && choicePending) {
    setUploadStatus("failed", `${choicePending.name}: choose car or track before uploading.`, "The browser could not detect the content type.");
    toast.error("Choose car or track for the archive before uploading.");
    return;
  }

  uploading.value = true;
  batchIndex.value = 0;
  batchTotal.value = url ? 1 : selectedFiles.length;
  let completed = 0;
  let failed = 0;
  let importedItems = 0;
  let filesWritten = 0;
  const importedByKind = { car: 0, track: 0 };

  try {
    if (url) {
      const response = await sendUploadRequest({ url, index: 0, total: 1 });
      applyUploadResponse(response, url, true);
      clearUploadInputs();
    } else {
      const selectedItems = uploadItems.value.filter((item) => selectedFiles.includes(item.file) && item.stage !== "completed");
      batchTotal.value = selectedItems.length;
      if (!selectedItems.length) {
        uploadProgress.value = 100;
        setUploadStatus("completed", "No pending archives to upload.", "All selected archives are already imported.");
        clearUploadInputs({ keepItems: true });
        return;
      }
      for (const [index, item] of selectedItems.entries()) {
        const archive = item.file;
        batchIndex.value = index;
        try {
          const response = await sendUploadRequest({ file: archive, index, total: selectedItems.length, kind: item.kind, item });
          applyUploadResponse(response, archive.name, selectedItems.length === 1, item);
          completed++;
          importedItems += response?.imported_count ?? 0;
          filesWritten += response?.files_written ?? 0;
          if (response?.kind) importedByKind[response.kind] += response.imported_count ?? 0;
        } catch (e) {
          failed++;
          const message = friendlyArchiveError(e instanceof Error ? e.message : String(e));
          if (isManualContentChoiceError(message)) {
            markUploadItemNeedsChoice(item, message);
          } else if (item.stage !== "needsChoice") {
            item.stage = "failed";
            item.progress = 0;
            item.message = message;
            item.detail = "The import did not complete.";
          }
        }
      }
      if (completed) {
        void content.load(true);
        void content.loadImageStats();
      }
      if (failed) {
        uploadProgress.value = 100;
        setUploadStatus(
          "failed",
          completed
            ? `Imported ${completed} archive${completed === 1 ? "" : "s"}. ${failed} failed.`
            : `${failed} archive${failed === 1 ? "" : "s"} failed.`,
          completed
            ? "The content library is refreshing. Failed rows are kept in the list so you can remove them or fix and retry."
            : "Failed rows are kept in the list so you can remove them or fix and retry.",
        );
        toast.error(`${failed} archive${failed === 1 ? "" : "s"} failed.`);
      } else if (selectedItems.length > 1) {
        uploadProgress.value = 100;
        const detectedItems = importedByKind.car + importedByKind.track;
        const detail = [
          importedByKind.car ? `${importedByKind.car} car item${importedByKind.car === 1 ? "" : "s"}` : "",
          importedByKind.track ? `${importedByKind.track} track item${importedByKind.track === 1 ? "" : "s"}` : "",
          importedItems && detectedItems !== importedItems ? `${importedItems} item${importedItems === 1 ? "" : "s"} imported` : "",
          filesWritten ? `${filesWritten} file${filesWritten === 1 ? "" : "s"} written` : "",
          "The content library is refreshing.",
        ]
          .filter(Boolean)
          .join(" · ");
        setUploadStatus("completed", `Imported ${completed} archive${completed === 1 ? "" : "s"}.`, detail);
        toast.success(`Imported ${completed} archive${completed === 1 ? "" : "s"}.`);
        clearUploadInputs({ keepItems: true });
      } else {
        clearUploadInputs({ keepItems: true });
      }
    }
  } catch (e) {
    const message = friendlyArchiveError(e instanceof Error ? e.message : String(e));
    setUploadStatus(
      "failed",
      message,
      isManualContentChoiceError(message)
        ? "Choose car or track for this archive, then upload again."
        : completed
          ? `${completed} earlier archive${completed === 1 ? "" : "s"} imported. The content library is refreshing.`
          : "The import did not complete. Fix the issue and try again.",
    );
    if (completed) {
      void content.load(true);
      void content.loadImageStats();
    }
    toast.error(message);
  } finally {
    uploading.value = false;
  }
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

function uploadItemStatusLabel(item: UploadItem): string {
  if (item.stage === "scanning") return "Scanning";
  if (item.stage === "needsChoice") return "Choose type";
  if (item.stage === "ready") return "Ready";
  if (item.stage === "uploading") return `Uploading ${item.progress}%`;
  if (item.stage === "processing") return "Importing";
  if (item.stage === "queued") return "Queued";
  if (item.stage === "completed") return "Done";
  return "Failed";
}

function uploadItemMeta(item: UploadItem): string {
  const kind = item.kind === "car" ? "Car" : item.kind === "track" ? "Track" : "Auto-detect";
  return `${formatBytes(item.size)} · ${kind}`;
}

function uploadItemIcon(item: UploadItem): string {
  if (item.stage === "failed") return "alert";
  if (item.stage === "needsChoice") return "alert";
  if (item.stage === "completed") return "check";
  if (item.stage === "queued") return "download";
  return "upload";
}

function uploadItemBadgeClass(item: UploadItem): string {
  if (item.stage === "failed") return "border-danger/45 bg-danger-glow text-danger";
  if (item.stage === "needsChoice") return "border-warn/40 bg-warn-glow text-warn";
  if (item.stage === "completed") return "border-ok/40 bg-ok-glow text-ok";
  if (item.stage === "ready") return "border-line bg-surface-2 text-muted";
  return "border-accent/40 bg-accent-dim text-accent";
}

function uploadItemShowsProgress(item: UploadItem): boolean {
  return item.stage !== "ready" && item.stage !== "needsChoice" && item.stage !== "failed";
}

function uploadItemBarClass(item: UploadItem): string {
  return item.stage === "completed" ? "bg-ok" : "bg-accent";
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
      <div class="mb-4 grid gap-2 md:grid-cols-2 xl:grid-cols-5">
        <label class="min-w-0">
          <span class="mb-1 block text-[11px] font-semibold tracking-wide text-dim uppercase">Sort</span>
          <Select v-model="sort" :options="sortOptions" />
        </label>
        <template v-if="tab === 'cars'">
          <label class="min-w-0">
            <span class="mb-1 block text-[11px] font-semibold tracking-wide text-dim uppercase">Brand</span>
            <Select v-model="carBrand" :options="carBrandOptions" />
          </label>
          <label class="min-w-0">
            <span class="mb-1 block text-[11px] font-semibold tracking-wide text-dim uppercase">Class</span>
            <Select v-model="carClass" :options="carClassOptions" />
          </label>
          <label class="min-w-0">
            <span class="mb-1 block text-[11px] font-semibold tracking-wide text-dim uppercase">Min power</span>
            <Input v-model="minPower" type="number" :min="0" step="25" placeholder="Any" />
          </label>
        </template>
        <template v-else-if="tab === 'tracks'">
          <label class="min-w-0">
            <span class="mb-1 block text-[11px] font-semibold tracking-wide text-dim uppercase">Country</span>
            <Select v-model="trackCountry" :options="trackCountryOptions" />
          </label>
          <label class="min-w-0">
            <span class="mb-1 block text-[11px] font-semibold tracking-wide text-dim uppercase">Min pits</span>
            <Input v-model="minPitboxes" type="number" :min="0" step="1" placeholder="Any" />
          </label>
        </template>
        <div class="flex items-end justify-between gap-2 text-xs text-dim xl:justify-end">
          <span class="pb-2">{{ visibleCount }} shown</span>
          <Button v-if="filtersActive" variant="ghost" size="sm" @click="clearLibraryFilters">
            <Icon name="x" :size="13" />
            Clear
          </Button>
        </div>
      </div>

      <div v-if="tab === 'tracks'" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="t in filteredTracks" :key="`${t.key}:${t.config}`" class="group relative overflow-hidden rounded-md border border-line bg-surface-2/35 transition-colors hover:border-line-hi">
          <button
            type="button"
            class="block w-full cursor-pointer text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            :aria-label="`View details for ${t.name || t.key}`"
            @click="selectedTrack = t"
          >
            <TrackImage
              :track-key="t.key"
              :config="t.config"
              class="aspect-video w-full"
            />
            <div class="p-2">
              <div class="truncate text-sm font-medium">{{ t.name }}</div>
              <div class="text-xs text-dim">
                {{ t.config || "default" }}<span v-if="t.version"> · version {{ t.version }}</span> · {{ t.pitboxes }} pits<span v-if="formatDate(t.modified_at)"> · {{ formatDate(t.modified_at) }}</span>
              </div>
            </div>
          </button>
          <button
            type="button"
            class="absolute top-1.5 right-1.5 grid size-7 cursor-pointer place-items-center rounded-md bg-surface/80 text-muted opacity-0 backdrop-blur transition group-hover:opacity-100 hover:bg-danger-glow hover:text-danger focus:opacity-100"
            :aria-label="`Delete ${t.name || t.key}`"
            @click.stop="deleteTrack(t)"
          >
            <Icon name="trash" :size="15" />
          </button>
        </div>
      </div>

      <div v-else-if="tab === 'cars'" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="c in filteredCars" :key="c.key" class="group relative overflow-hidden rounded-md border border-line bg-surface-2/35 transition-colors hover:border-line-hi">
          <button
            type="button"
            class="block w-full cursor-pointer text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            :aria-label="`View details for ${c.name || c.key}`"
            @click="openCar(c)"
          >
            <div class="aspect-video w-full bg-surface-3">
              <img
                v-if="carPreviewUrl(c)"
                :src="carPreviewUrl(c)"
                alt=""
                loading="lazy"
                class="size-full object-cover"
              />
              <div v-else class="grid size-full place-items-center text-xs text-dim">No preview</div>
            </div>
            <div class="p-2">
              <div class="truncate text-sm font-medium">{{ c.name }}</div>
              <div class="text-xs text-dim">
                {{ c.brand || "Unknown" }}<span v-if="carPowerLabel(c)"> · {{ carPowerLabel(c) }}</span> · {{ c.skins?.length ?? 0 }} skins<span v-if="formatDate(c.modified_at)"> · {{ formatDate(c.modified_at) }}</span>
              </div>
            </div>
          </button>
          <button
            type="button"
            class="absolute top-1.5 right-1.5 grid size-7 cursor-pointer place-items-center rounded-md bg-surface/80 text-muted opacity-0 backdrop-blur transition group-hover:opacity-100 hover:bg-danger-glow hover:text-danger focus:opacity-100"
            :aria-label="`Delete ${c.name || c.key}`"
            @click.stop="deleteCar(c)"
          >
            <Icon name="trash" :size="15" />
          </button>
        </div>
      </div>

      <div v-else class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="w in filteredWeathers" :key="w.key" class="flex items-center gap-2 rounded-md border border-line p-2 text-sm">
          <span class="min-w-0 flex-1 truncate">{{ w.name }}</span>
          <button
            type="button"
            class="grid size-7 shrink-0 cursor-pointer place-items-center rounded-md text-muted transition hover:bg-danger-glow hover:text-danger"
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
        <FormRow label="Archive files" hint="Select one or more zip / 7z / rar archives, up to 10 GB each">
          <input
            ref="fileInput"
            type="file"
            multiple
            accept=".zip,.7z,.rar"
            class="w-full cursor-pointer text-sm text-muted file:mr-3 file:cursor-pointer file:rounded-md file:border file:border-line file:bg-surface-2 file:px-3 file:py-1.5 file:text-sm file:text-text file:transition-colors file:duration-200 hover:file:border-line-hi hover:file:bg-surface-3"
            @change="onFileChange"
          />
          <p v-if="selectedFileSummary" class="mt-1 text-xs text-dim">{{ selectedFileSummary }}</p>
          <div v-if="uploadItems.length" class="mt-2 overflow-hidden rounded-md border border-line bg-surface-2/25">
            <div v-for="item in uploadItems" :key="item.id" class="border-t border-line px-2.5 py-2 first:border-t-0">
              <div class="flex items-start gap-2">
                <Icon :name="uploadItemIcon(item)" :size="15" class="mt-0.5 shrink-0 text-muted" />
                <div class="min-w-0 flex-1">
                  <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0">
                      <p class="truncate text-sm font-medium text-text">{{ item.name }}</p>
                      <p class="mt-0.5 text-xs text-dim">{{ uploadItemMeta(item) }}</p>
                    </div>
                    <div class="flex shrink-0 items-center gap-1.5">
                      <span class="rounded-full border px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase" :class="uploadItemBadgeClass(item)">
                        {{ uploadItemStatusLabel(item) }}
                      </span>
                      <Button
                        v-if="canRemoveUploadItem(item)"
                        variant="ghost"
                        size="sm"
                        :aria-label="`Remove ${item.name}`"
                        @click="removeUploadItem(item)"
                      >
                        <Icon name="x" :size="13" />
                        Remove
                      </Button>
                    </div>
                  </div>
                  <p class="mt-1 text-xs text-muted">{{ item.message }}</p>
                  <p v-if="item.detail" class="mt-0.5 text-xs text-dim">{{ item.detail }}</p>
                  <label v-if="item.stage === 'needsChoice'" class="mt-2 block max-w-48">
                    <span class="mb-1 block text-[11px] font-semibold tracking-wide text-dim uppercase">Content type</span>
                    <Select
                      :model-value="item.kind"
                      :options="uploadKindChoiceOptions"
                      @update:model-value="(value) => setUploadItemKind(item, value)"
                    />
                  </label>
                  <div v-if="uploadItemShowsProgress(item)" class="mt-2 h-1 overflow-hidden rounded-full bg-surface-3">
                    <div class="h-full transition-all duration-200" :class="uploadItemBarClass(item)" :style="{ width: item.progress + '%' }" />
                  </div>
                </div>
              </div>
            </div>
          </div>
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
          <Button :disabled="uploading || (!archiveUrl.trim() && (scanningFiles || filesNeedTypeChoice))" @click="upload">
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

  <Sheet :open="!!selectedTrack" :title="selectedTrack?.name || selectedTrack?.key || 'Track'" @close="selectedTrack = null">
    <template v-if="selectedTrack">
      <TrackImage
        :track-key="selectedTrack.key"
        :config="selectedTrack.config"
        class="aspect-video w-full rounded-md border border-line"
      />

      <div class="mt-3 flex flex-wrap items-center gap-2">
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
          {{ selectedTrack.config || "default" }}
        </span>
        <span v-if="trackLocation" class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
          {{ trackLocation }}
        </span>
        <span v-if="selectedTrack.content_path" class="ml-auto truncate text-xs text-dim">{{ selectedTrack.content_path }}</span>
      </div>

      <dl class="mt-4 grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-3">
        <div v-for="spec in trackSpecRows" :key="spec.label" class="rounded-md border border-line bg-surface-2/40 px-2.5 py-1.5">
          <dt class="text-xs text-dim">{{ spec.label }}</dt>
          <dd class="break-words font-mono text-sm text-text">{{ spec.value }}</dd>
        </div>
      </dl>

      <p v-if="selectedTrack.desc" class="mt-4 text-sm leading-relaxed whitespace-pre-line text-muted">{{ selectedTrack.desc }}</p>

      <div v-if="selectedTrack.tags?.length" class="mt-4">
        <h3 class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">Tags</h3>
        <div class="flex flex-wrap gap-1.5">
          <span v-for="tag in (selectedTrack.tags ?? []).slice(0, 16)" :key="tag" class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
            {{ tag }}
          </span>
        </div>
      </div>
    </template>
    <template #footer>
      <Button variant="ghost" @click="selectedTrack = null">Close</Button>
    </template>
  </Sheet>

  <Sheet :open="!!selectedCar" :title="selectedCar?.name || selectedCar?.key || 'Car'" @close="selectedCar = null">
    <template v-if="selectedCar">
      <div class="aspect-video w-full overflow-hidden rounded-md border border-line bg-surface-3">
        <img
          v-if="carPreviewUrl(selectedCar)"
          :src="carPreviewUrl(selectedCar)"
          alt=""
          class="size-full object-cover"
          @error="($event.target as HTMLImageElement).style.display = 'none'"
        />
        <div v-else class="grid size-full place-items-center text-sm text-dim">No preview</div>
      </div>

      <div class="mt-3 flex flex-wrap items-center gap-2">
        <span v-if="selectedCar.brand" class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
          {{ selectedCar.brand }}
        </span>
        <span v-if="selectedCar.class" class="rounded-full border border-accent/40 bg-accent-dim px-2 py-0.5 text-xs text-accent">
          {{ selectedCar.class }}
        </span>
        <span class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
          {{ selectedCar.skins?.length ?? 0 }} skins
        </span>
        <span v-if="selectedCar.key" class="ml-auto font-mono text-xs text-dim">{{ selectedCar.key }}</span>
      </div>

      <dl v-if="carSpecRows.length" class="mt-4 grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-3">
        <div v-for="spec in carSpecRows" :key="spec.label" class="rounded-md border border-line bg-surface-2/40 px-2.5 py-1.5">
          <dt class="text-xs text-dim">{{ spec.label }}</dt>
          <dd class="font-mono text-sm text-text">{{ spec.value }}</dd>
        </div>
      </dl>

      <div class="mt-4">
        <h3 class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">Power &amp; torque</h3>
        <div v-if="carCurvesLoading" class="grid h-44 place-items-center rounded-md border border-line bg-surface-2/40 text-sm text-dim">
          Loading curves…
        </div>
        <div v-else-if="carChartSeries.length" class="rounded-md border border-line bg-surface-2/40 p-3">
          <LineChart :series="carChartSeries" :labels="carCurves?.labels ?? []" :height="200" x-label="RPM" />
        </div>
        <p v-else class="rounded-md border border-line bg-surface-2/40 px-3 py-3 text-sm text-dim">
          No dyno data shipped with this car.
        </p>
      </div>

      <div v-if="selectedCar.skins?.length" class="mt-4">
        <h3 class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">Skins</h3>
        <div class="flex flex-wrap gap-1.5">
          <span v-for="skin in (selectedCar.skins ?? []).slice(0, 24)" :key="skin.key" class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
            {{ skin.name || skin.key }}
          </span>
          <span v-if="(selectedCar.skins?.length ?? 0) > 24" class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-dim">
            +{{ (selectedCar.skins?.length ?? 0) - 24 }} more
          </span>
        </div>
      </div>

      <div v-if="selectedCar.tags?.length" class="mt-4">
        <h3 class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">Tags</h3>
        <div class="flex flex-wrap gap-1.5">
          <span v-for="tag in (selectedCar.tags ?? []).slice(0, 16)" :key="tag" class="rounded-full border border-line bg-surface-2 px-2 py-0.5 text-xs text-muted">
            {{ tag }}
          </span>
        </div>
      </div>

      <p v-if="carDescText" class="mt-4 text-sm leading-relaxed whitespace-pre-line text-muted">{{ carDescText }}</p>
    </template>
    <template #footer>
      <Button variant="ghost" @click="selectedCar = null">Close</Button>
    </template>
  </Sheet>
</template>
