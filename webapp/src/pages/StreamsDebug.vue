<script setup lang="ts">
// Stream diagnostics — everything SM knows about each driver stream: the capture
// subsystem, the per-stream rolling-buffer recorder + on-disk segment buffer,
// recent capture logs, and a live ffmpeg probe to test a capture URL straight
// from the server. Admin-only (capture URLs can carry tokens).
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useToastStore } from "@/stores/toast";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";

interface CaptureInfo {
  ffmpeg_available: boolean;
  ffmpeg_path: string;
  enabled: boolean;
  screenshots: boolean;
  clips: boolean;
  trigger_score: number;
  clip_seconds: number;
  cooldown_seconds: number;
  max_per_session: number;
  ring_seconds: number;
  segment_seconds: number;
  max_recorders: number;
  active_recorders: number;
}
interface StreamRow {
  id: number | null;
  guid: string;
  display_name: string;
  enabled: boolean;
  embed_url: string;
  status_url: string;
  capture_url: string;
  capture_scheme: string;
  capture_supported: boolean;
  armed: boolean;
  online: boolean;
  recorder_running: boolean;
  recorder_dead: boolean;
  recorder_started_ms: number;
  segment_count: number;
  oldest_segment_ms: number;
  newest_segment_ms: number;
  buffer_bytes: number;
  manual_active: boolean;
  media_count: number;
}
interface DebugSnapshot {
  now_ms: number;
  capture: CaptureInfo;
  streams: StreamRow[];
  logs: string[];
}
interface ProbeResult {
  ok: boolean;
  elapsed_ms: number;
  timed_out: boolean;
  error: string;
  streams: string[];
  log_tail: string;
}

const toast = useToastStore();
const snap = ref<DebugSnapshot | null>(null);
const loading = ref(true);
const live = ref(true);
let timer: number | undefined;

const probes = ref<Record<string, { loading: boolean; result?: ProbeResult }>>({});

async function load(quiet = false) {
  if (!quiet) loading.value = true;
  try {
    snap.value = await api.get<DebugSnapshot>("/api/streams/debug");
  } catch (e) {
    if (!quiet) toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    loading.value = false;
  }
}

function rowKey(s: StreamRow): string {
  return String(s.id ?? s.guid);
}

async function probe(s: StreamRow) {
  const key = rowKey(s);
  if (!s.capture_url) {
    toast.info("No capture URL set for this stream.");
    return;
  }
  probes.value[key] = { loading: true };
  try {
    const result = await api.post<ProbeResult>("/api/streams/debug/probe", { url: s.capture_url });
    probes.value[key] = { loading: false, result };
  } catch (e) {
    probes.value[key] = { loading: false };
    const msg = e instanceof ApiError ? e.message : e instanceof Error ? e.message : String(e);
    toast.error(msg || "Probe request failed — it may have timed out at a proxy. Try again, or check the segment count below.");
  }
}

function toggleLive() {
  live.value = !live.value;
  schedule();
}
function schedule() {
  if (timer) window.clearInterval(timer);
  if (live.value) timer = window.setInterval(() => load(true), 4000);
}

onMounted(() => {
  void load();
  schedule();
});
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer);
});

// ---- diagnosis: the most useful signal on the page --------------------------
type Diag = { tone: "ok" | "warn" | "danger" | "dim"; text: string };
function diagnose(s: StreamRow): Diag {
  if (!s.enabled) return { tone: "dim", text: "Stream disabled" };
  if (!s.capture_url) return { tone: "dim", text: "No capture URL — capture off for this driver" };
  if (!s.capture_supported)
    return { tone: "danger", text: `Unsupported source (${s.capture_scheme || "?"}). ffmpeg can't pull WebRTC/WHEP — use HLS (mpegTS) or RTSP.` };
  if (s.recorder_running && s.segment_count === 0)
    return { tone: "danger", text: "Recorder connected but wrote 0 segments — source not pullable (WebRTC/LL-HLS). Probe it." };
  if (s.recorder_dead) return { tone: "danger", text: "Recorder process died — ffmpeg couldn't read the source. Probe it." };
  if (s.recorder_running && s.segment_count > 0)
    return { tone: "ok", text: `Recording 24/7 — ${s.segment_count} segments buffered${s.online ? " · driver online, clips persisting" : " · driver offline, clips paused"}` };
  if (!s.recorder_running) return { tone: "warn", text: "No recorder running — capture disabled globally, or starting up" };
  return { tone: "dim", text: "Idle" };
}

const toneClass: Record<Diag["tone"], string> = {
  ok: "border-ok/40 bg-ok-glow text-ok",
  warn: "border-warn/40 bg-warn-glow text-warn",
  danger: "border-danger/45 bg-danger-glow text-danger",
  dim: "border-line bg-surface-2 text-muted",
};

// ---- formatting -------------------------------------------------------------
function fmtBytes(n: number): string {
  if (!n) return "0 B";
  const u = ["B", "KB", "MB", "GB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 && i > 0 ? 1 : 0)} ${u[i]}`;
}
function fmtAgo(ms: number): string {
  if (!ms) return "—";
  const d = Math.max(0, (snap.value?.now_ms ?? Date.now()) - ms);
  const s = Math.floor(d / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s ago`;
  return `${Math.floor(m / 60)}h ${m % 60}m ago`;
}
function spanSeconds(s: StreamRow): string {
  if (!s.segment_count) return "—";
  return `${Math.round((s.newest_segment_ms - s.oldest_segment_ms) / 1000) + snap.value!.capture.segment_seconds}s`;
}

const cap = computed(() => snap.value?.capture ?? null);
const logText = computed(() => (snap.value?.logs ?? []).join("\n"));
</script>

<template>
  <PageHeader
    title="Stream Diagnostics"
    subtitle="Recorder state, buffers, logs, and source probes for driver streams."
    icon="broadcast"
  >
    <template #actions>
      <Button size="sm" :variant="live ? 'ghost' : 'ghost'" :title="live ? 'Auto-refreshing every 4s' : 'Auto-refresh paused'" @click="toggleLive">
        <span class="size-1.5 rounded-full" :class="live ? 'bg-ok live-dot' : 'bg-dim'" />
        {{ live ? "Live" : "Paused" }}
      </Button>
      <Button size="sm" variant="ghost" @click="load()">
        <Icon name="repeat" :size="14" /> Refresh
      </Button>
    </template>
  </PageHeader>

  <!-- Capture subsystem -->
  <Card v-if="cap">
    <template #header>
      <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight"><Icon name="film" :size="15" class="text-accent" /> Capture subsystem</h2>
    </template>
    <div class="grid grid-cols-2 gap-x-6 gap-y-2 text-sm sm:grid-cols-3 lg:grid-cols-4">
      <div class="flex items-center gap-2">
        <span class="text-dim">ffmpeg</span>
        <span v-if="cap.ffmpeg_available" class="inline-flex items-center gap-1 rounded border border-ok/40 bg-ok-glow px-1.5 py-0.5 text-[11px] font-bold text-ok"><Icon name="check" :size="11" /> available</span>
        <span v-else class="inline-flex items-center gap-1 rounded border border-danger/45 bg-danger-glow px-1.5 py-0.5 text-[11px] font-bold text-danger"><Icon name="x" :size="11" /> missing</span>
      </div>
      <div><span class="text-dim">Capture enabled</span> <span class="font-semibold" :class="cap.enabled ? 'text-text' : 'text-danger'">{{ cap.enabled ? "yes" : "no" }}</span></div>
      <div><span class="text-dim">Screenshots / Clips</span> <span class="font-semibold text-text">{{ cap.screenshots ? "on" : "off" }} / {{ cap.clips ? "on" : "off" }}</span></div>
      <div><span class="text-dim">Active recorders</span> <span class="font-mono font-semibold text-text">{{ cap.active_recorders }} / {{ cap.max_recorders }}</span></div>
      <div><span class="text-dim">Trigger score</span> <span class="font-mono text-text">{{ cap.trigger_score.toLocaleString() }}</span></div>
      <div><span class="text-dim">Clip length</span> <span class="font-mono text-text">{{ cap.clip_seconds }}s</span></div>
      <div><span class="text-dim">Cooldown</span> <span class="font-mono text-text">{{ cap.cooldown_seconds }}s</span></div>
      <div><span class="text-dim">Max / session</span> <span class="font-mono text-text">{{ cap.max_per_session }}</span></div>
      <div><span class="text-dim">Ring buffer</span> <span class="font-mono text-text">{{ cap.ring_seconds }}s @ {{ cap.segment_seconds }}s segs</span></div>
      <div class="col-span-2 sm:col-span-3 lg:col-span-4"><span class="text-dim">Mode</span> <span class="font-semibold text-text">always-on — records every armed stream 24/7; clips persist only while the driver is online &amp; drifting</span></div>
      <div v-if="cap.ffmpeg_path" class="col-span-2 truncate sm:col-span-3 lg:col-span-4"><span class="text-dim">ffmpeg path</span> <span class="font-mono text-xs text-muted">{{ cap.ffmpeg_path }}</span></div>
    </div>
  </Card>

  <!-- Streams -->
  <div v-if="loading && !snap" class="mt-4 h-40 animate-pulse rounded-lg border border-line bg-surface-2/50" />

  <div v-else class="mt-4 space-y-3">
    <Card v-for="s in snap?.streams ?? []" :key="rowKey(s)">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <span class="truncate text-sm font-bold text-text">{{ s.display_name || s.guid || "Unnamed stream" }}</span>
            <span v-if="s.online" class="inline-flex items-center gap-1 rounded-full border border-ok/40 bg-ok-glow px-1.5 py-0.5 text-[10px] font-bold tracking-wide text-ok uppercase"><span class="size-1.5 rounded-full bg-ok live-dot" /> online</span>
          </div>
          <div class="mt-0.5 font-mono text-[11px] text-dim">{{ s.guid || "—" }}</div>
        </div>
        <Button size="sm" variant="ghost" :disabled="probes[rowKey(s)]?.loading || !s.capture_url" @click="probe(s)">
          <Icon name="activity" :size="14" /> {{ probes[rowKey(s)]?.loading ? "Probing…" : "Probe source" }}
        </Button>
      </div>

      <!-- Diagnosis line -->
      <div class="mt-2 inline-flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs font-semibold" :class="toneClass[diagnose(s).tone]">
        <Icon :name="diagnose(s).tone === 'ok' ? 'check' : diagnose(s).tone === 'danger' ? 'alert' : 'info'" :size="13" />
        {{ diagnose(s).text }}
      </div>

      <!-- State chips -->
      <div class="mt-3 flex flex-wrap gap-1.5 text-[11px] font-semibold">
        <span class="rounded border px-1.5 py-0.5" :class="s.enabled ? 'border-line-hi bg-surface-3 text-text' : 'border-line bg-surface-2 text-dim'">enabled: {{ s.enabled ? "yes" : "no" }}</span>
        <span class="rounded border px-1.5 py-0.5" :class="s.armed ? 'border-line-hi bg-surface-3 text-text' : 'border-line bg-surface-2 text-dim'">armed: {{ s.armed ? "yes" : "no" }}</span>
        <span class="rounded border px-1.5 py-0.5" :class="s.recorder_running ? 'border-ok/40 bg-ok-glow text-ok' : s.recorder_dead ? 'border-danger/45 bg-danger-glow text-danger' : 'border-line bg-surface-2 text-dim'">
          recorder: {{ s.recorder_running ? "running" : s.recorder_dead ? "dead" : "off" }}
        </span>
        <span v-if="s.manual_active" class="rounded border border-warn/40 bg-warn-glow px-1.5 py-0.5 text-warn">manual recording</span>
      </div>

      <!-- Metrics -->
      <div class="mt-3 grid grid-cols-2 gap-x-6 gap-y-1.5 text-sm sm:grid-cols-4">
        <div><span class="text-dim">Buffer</span> <span class="font-mono text-text">{{ s.segment_count }} segs</span></div>
        <div><span class="text-dim">Span</span> <span class="font-mono text-text">{{ spanSeconds(s) }}</span></div>
        <div><span class="text-dim">Size</span> <span class="font-mono text-text">{{ fmtBytes(s.buffer_bytes) }}</span></div>
        <div><span class="text-dim">Media</span> <span class="font-mono text-text">{{ s.media_count }}</span></div>
        <div v-if="s.recorder_running"><span class="text-dim">Recording since</span> <span class="text-text">{{ fmtAgo(s.recorder_started_ms) }}</span></div>
        <div v-if="s.newest_segment_ms"><span class="text-dim">Newest seg</span> <span class="text-text">{{ fmtAgo(s.newest_segment_ms) }}</span></div>
      </div>

      <!-- Capture URL -->
      <div class="mt-3 flex flex-wrap items-center gap-2 text-xs">
        <span class="text-dim">capture</span>
        <span v-if="s.capture_scheme" class="rounded border px-1.5 py-0.5 font-bold uppercase" :class="s.capture_supported ? 'border-ok/40 bg-ok-glow text-ok' : 'border-danger/45 bg-danger-glow text-danger'">{{ s.capture_scheme }}</span>
        <code class="min-w-0 flex-1 truncate rounded bg-surface-2 px-2 py-1 font-mono text-muted">{{ s.capture_url || "— not set —" }}</code>
      </div>

      <!-- Probe result -->
      <div v-if="probes[rowKey(s)]?.result" class="mt-3 rounded-md border border-line bg-bg/40 p-3">
        <div class="flex items-center gap-2 text-sm font-bold">
          <template v-if="probes[rowKey(s)]!.result!.ok">
            <Icon name="check" :size="15" class="text-ok" /><span class="text-ok">Pullable</span>
          </template>
          <template v-else-if="probes[rowKey(s)]!.result!.timed_out">
            <Icon name="alert" :size="15" class="text-danger" /><span class="text-danger">Timed out — ffmpeg stalled (WebRTC / Low-Latency HLS)</span>
          </template>
          <template v-else>
            <Icon name="x" :size="15" class="text-danger" /><span class="text-danger">Failed</span>
          </template>
          <span class="ml-auto font-mono text-xs text-dim">{{ probes[rowKey(s)]!.result!.elapsed_ms }} ms</span>
        </div>
        <div v-if="probes[rowKey(s)]!.result!.streams?.length" class="mt-2 space-y-0.5">
          <div v-for="(st, i) in probes[rowKey(s)]!.result!.streams" :key="i" class="font-mono text-[11px] text-muted">{{ st }}</div>
        </div>
        <pre v-if="probes[rowKey(s)]!.result!.log_tail" class="mt-2 max-h-48 overflow-auto rounded bg-surface-2 p-2 font-mono text-[10px] leading-relaxed text-dim">{{ probes[rowKey(s)]!.result!.log_tail }}</pre>
      </div>
    </Card>

    <Card v-if="(snap?.streams.length ?? 0) === 0">
      <div class="py-8 text-center text-sm text-muted">No driver streams configured. Add one under Streaming & Capture.</div>
    </Card>
  </div>

  <!-- Logs -->
  <Card class="mt-4">
    <template #header>
      <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight"><Icon name="terminal" :size="15" class="text-accent" /> Recent capture logs</h2>
      <span class="text-xs text-dim">{{ snap?.logs.length ?? 0 }} lines</span>
    </template>
    <pre v-if="logText" class="max-h-96 overflow-auto rounded-md bg-surface-2 p-3 font-mono text-[11px] leading-relaxed text-muted">{{ logText }}</pre>
    <div v-else class="py-6 text-center text-sm text-dim">No stream-related log lines yet.</div>
  </Card>
</template>
