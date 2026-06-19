// Shared live capture state + actions for driver streams, used by the Driver
// Detail and Broadcast pages. A single module-level poller (refcounted) fetches
// /api/streams/capture-status so any number of consumers share one request.
import { reactive, ref } from "vue";
import { api } from "@/lib/api";

export interface CaptureStatus {
  recorder_running: boolean;
  segment_count: number;
  buffering: boolean;
  manual_active: boolean;
  manual_started_ms: number;
}

const statuses = reactive<Record<string, CaptureStatus>>({});
const nowMs = ref(0);
let timer: number | undefined;
let refcount = 0;

async function refresh() {
  try {
    const res = await api.get<{ now_ms: number; drivers: Record<string, CaptureStatus> }>(
      "/api/streams/capture-status",
    );
    nowMs.value = res.now_ms || Date.now();
    const live = new Set(Object.keys(res.drivers ?? {}));
    for (const k of Object.keys(statuses)) if (!live.has(k)) delete statuses[k];
    Object.assign(statuses, res.drivers ?? {});
  } catch {
    /* transient — keep the last snapshot */
  }
}

export function useDriverCapture() {
  function startPoll(intervalMs = 4000) {
    refcount++;
    if (!timer) {
      void refresh();
      timer = window.setInterval(refresh, intervalMs);
    }
  }
  function stopPoll() {
    refcount = Math.max(0, refcount - 1);
    if (refcount === 0 && timer) {
      window.clearInterval(timer);
      timer = undefined;
    }
  }
  function statusFor(guid: string): CaptureStatus | undefined {
    return guid ? statuses[guid] : undefined;
  }
  function isRecording(guid: string): boolean {
    return !!statusFor(guid)?.manual_active;
  }
  function isBuffering(guid: string): boolean {
    return !!statusFor(guid)?.buffering;
  }
  // Seconds since the manual recording was armed (steps with the poll interval).
  function manualElapsed(guid: string): number {
    const s = statusFor(guid);
    if (!s?.manual_active || !s.manual_started_ms) return 0;
    return Math.max(0, Math.floor(((nowMs.value || Date.now()) - s.manual_started_ms) / 1000));
  }
  async function takePicture(guid: string): Promise<void> {
    await api.post(`/api/drivers/${encodeURIComponent(guid)}/snapshot`);
    void refresh();
  }
  async function recordNow(guid: string): Promise<void> {
    await api.post(`/api/drivers/${encodeURIComponent(guid)}/record`);
    void refresh();
  }
  return {
    statuses,
    nowMs,
    startPoll,
    stopPoll,
    refresh,
    statusFor,
    isRecording,
    isBuffering,
    manualElapsed,
    takePicture,
    recordNow,
  };
}

// m:ss formatter for clip durations (runs can exceed 60s).
export function fmtClipDuration(s: number): string {
  if (!s || s < 0) return "0:00";
  return `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`;
}
