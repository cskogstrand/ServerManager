// Shared driver-stream plumbing for the watch surfaces (Dashboard, server
// detail, broadcast). The backend exposes two endpoints:
//   GET /api/driver-streams                         → all configured streams
//   GET /api/instances/:id/driver-streams/status    → live health per guid
// The list carries the embed URLs needed to actually play a stream; the status
// poll only carries health for currently-connected drivers. This composable
// joins them into watchable StreamChannels keyed by driver guid.
import { ref } from "vue";
import { api } from "@/lib/api";
import type { DriverState } from "@/stores/server";
import type { DriverStream } from "@/types/generated";

export type StreamHealthStatus = "live" | "offline" | "unknown" | "not_configured";

export interface StreamHealth {
  status: StreamHealthStatus;
  status_code?: number;
  message?: string;
}

// A resolved, watchable channel handed to the StreamTheater.
export interface StreamChannel {
  key: string; // stable id, e.g. `driver:<guid>` or `spectator:<instanceId>`
  title: string;
  subtitle?: string;
  url: string;
  health: StreamHealthStatus;
}

export function useDriverStreams() {
  // guid → configured, enabled stream that actually has an embed URL
  const byGuid = ref<Record<string, DriverStream>>({});
  // guid → live health from the per-instance status poll
  const health = ref<Record<string, StreamHealth>>({});
  const loaded = ref(false);
  let poll: ReturnType<typeof setInterval> | null = null;

  async function loadStreams() {
    try {
      const res = await api.get<{ streams: DriverStream[] }>("/api/driver-streams");
      const map: Record<string, DriverStream> = {};
      for (const s of res.streams ?? []) {
        if (s.driver_guid && (s.enabled ?? 0) !== 0 && s.stream_embed_url) {
          map[s.driver_guid] = s;
        }
      }
      byGuid.value = map;
    } catch {
      /* config endpoint is best-effort; watch actions simply stay hidden */
    } finally {
      loaded.value = true;
    }
  }

  async function refreshHealth(instanceId: number) {
    try {
      const res = await api.get<{ statuses: Record<string, StreamHealth> }>(
        `/api/instances/${instanceId}/driver-streams/status`,
      );
      health.value = res.statuses ?? {};
    } catch {
      /* keep the last known health on a transient miss */
    }
  }

  // Light health poll for one instance. Safe to call repeatedly.
  function startHealthPoll(instanceId: () => number, intervalMs = 15000) {
    stopHealthPoll();
    void refreshHealth(instanceId());
    poll = setInterval(() => void refreshHealth(instanceId()), intervalMs);
  }
  function stopHealthPoll() {
    if (poll) {
      clearInterval(poll);
      poll = null;
    }
  }

  function streamForGuid(guid: string | null | undefined): DriverStream | null {
    if (!guid) return null;
    return byGuid.value[guid] ?? null;
  }

  function healthForGuid(guid: string | null | undefined): StreamHealthStatus {
    if (!guid) return "not_configured";
    return health.value[guid]?.status ?? (byGuid.value[guid] ? "unknown" : "not_configured");
  }

  // Watchable channels for the connected drivers that have a stream configured.
  function channelsFor(drivers: DriverState[]): StreamChannel[] {
    return drivers
      .filter((d) => d.connected && streamForGuid(d.guid))
      .map((d) => {
        const s = streamForGuid(d.guid)!;
        const name = s.display_name || d.name || `Car ${d.car_id}`;
        return {
          key: `driver:${d.guid}`,
          title: name,
          subtitle: d.name && s.display_name && d.name !== s.display_name ? d.name : undefined,
          url: s.stream_embed_url!,
          health: healthForGuid(d.guid),
        };
      });
  }

  return {
    byGuid,
    health,
    loaded,
    loadStreams,
    refreshHealth,
    startHealthPoll,
    stopHealthPoll,
    streamForGuid,
    healthForGuid,
    channelsFor,
  };
}
