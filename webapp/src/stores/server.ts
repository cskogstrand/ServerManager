import { defineStore } from "pinia";
import { api } from "@/lib/api";
import { subscribeServerEvents, type ServerEvent } from "@/lib/sse";
import { useContentStore } from "@/stores/content";
import { useFeedStore } from "@/stores/feed";
import { useToastStore } from "@/stores/toast";

// Persisted domain changes that belong on the live feed, not in per-instance
// state. Routed before the instance lookup so guid-only events (instance 0)
// don't trip the unknown-instance recovery.
const FEED_TYPES = new Set([
  "session_start",
  "session_end",
  "lap",
  "drift_run",
  "media",
  "recording",
]);

export interface SessionState {
  name: string;
  type: number;
  index: number;
  current_session_index: number;
  session_count: number;
  track: string;
  track_config: string;
  server_name: string;
  time: number;
  laps: number;
  wait_time: number;
  ambient_temp: number;
  road_temp: number;
  weather_graphics: string;
  elapsed_ms: number;
}

export interface DriverState {
  car_id: number;
  name: string;
  car: string;
  skin: string;
  guid: string;
  laps: number;
  last_lap_ms: number;
  best_lap_ms: number;
  connected: boolean;
  drift_live?: number;
  drift_last?: number;
  drift_best?: number;
  // Roster guest-driver assigned to this car (0/undefined = none). When set, the
  // car's completed runs are recorded under that person's name.
  guest_driver_id?: number;
}

export interface CarPositionState {
  car_id: number;
  x: number;
  y: number;
  z: number;
  velocity_x: number;
  velocity_y: number;
  velocity_z: number;
  gear: number;
  engine_rpm: number;
  normalized_spline_pos: number;
  updated_at: number;
}

export interface TelemetryHealth {
  udp_online: boolean;
  last_packet_ms: number;
  last_driver_ms: number;
  last_position_ms: number;
  plugin_listen_port: number;
  plugin_send_port: number;
}

export type RunMode = "manual_queue" | "repeat_event";

export interface RepeatEventInfo {
  id: number;
  track?: string | null;
  category?: string | null;
  class?: string | null;
}

export interface InstanceState {
  id: number;
  name: string;
  udp_port: number | null;
  tcp_port: number | null;
  http_port: number | null;
  plugin_port: number | null;
  plugin_listen_port: number | null;
  running: boolean;
  players: number;
  session: SessionState | null;
  drivers: DriverState[];
  positions: CarPositionState[];
  telemetry: TelemetryHealth | null;
  run_mode: RunMode;
  repeat_event_id: number | null;
  repeat_event: RepeatEventInfo | null;
  scheduled_start: number | null;
  start_on_boot: number | null;
  drift_score_enabled: number | null;
  allow_wrong_way: number | null;
  stream_enabled: number | null;
  stream_embed_url: string | null;
  stream_status_url: string | null;
  spectator_enabled: number | null;
  spectator_driver_name: string | null;
  spectator_guid: string | null;
  spectator_car_key: string | null;
  spectator_skin_key: string | null;
}

interface InstanceListItem {
  id: number;
  name: string;
  udp_port: number | null;
  tcp_port: number | null;
  http_port: number | null;
  plugin_port: number | null;
  plugin_listen_port: number | null;
  is_running: boolean;
  players: number;
  run_mode: RunMode;
  repeat_event_id: number | null;
  repeat_event: RepeatEventInfo | null;
  scheduled_start: number | null;
  start_on_boot: number | null;
  drift_score_enabled: number | null;
  allow_wrong_way: number | null;
  stream_enabled: number | null;
  stream_embed_url: string | null;
  stream_status_url: string | null;
  spectator_enabled: number | null;
  spectator_driver_name: string | null;
  spectator_guid: string | null;
  spectator_car_key: string | null;
  spectator_skin_key: string | null;
}

// Shape of /api/server/status, the authoritative fallback snapshot used to
// recover state the SSE stream may have missed (startup race, reconnect,
// backgrounded tab). session.type_id is the numeric session type matching
// SessionState.type; session.type (string) is for display elsewhere.
export interface StatusResponse {
  is_running: boolean;
  players: number;
  session:
    | (Omit<SessionState, "type"> & { type: string; type_id: number })
    | null;
  drivers: DriverState[];
  positions: CarPositionState[];
  telemetry: TelemetryHealth | null;
}

// Recovery throttle (module-scoped: the store is a singleton). Stops a burst of
// unknown-instance events from triggering a stampede of bootstrap fetches.
let recoveryInflight = false;
let lastRecoveryAt = 0;

// One live store for everything the SSE stream feeds: per-instance status,
// players, current session. Replaces the old UI's 1s polling loops.
export const useServerStore = defineStore("server", {
  state: () => ({
    instances: {} as Record<number, InstanceState>,
    connected: false,
    loaded: false,
    unsubscribe: null as null | (() => void),
  }),

  getters: {
    instanceList(state): InstanceState[] {
      return Object.values(state.instances).sort((a, b) => a.id - b.id);
    },
  },

  actions: {
    async load() {
      const res = await api.get<{ instances: InstanceListItem[] }>("/api/instances");
      for (const item of res.instances) {
        this.instances[item.id] = {
          ...this.instances[item.id],
          id: item.id,
          name: item.name,
          udp_port: item.udp_port,
          tcp_port: item.tcp_port,
          http_port: item.http_port,
          plugin_port: item.plugin_port,
          plugin_listen_port: item.plugin_listen_port,
          running: item.is_running,
          players: item.players,
          session: this.instances[item.id]?.session ?? null,
          drivers: this.instances[item.id]?.drivers ?? [],
          positions: this.instances[item.id]?.positions ?? [],
          telemetry: this.instances[item.id]?.telemetry ?? null,
          run_mode: item.run_mode ?? "manual_queue",
          repeat_event_id: item.repeat_event_id ?? null,
          repeat_event: item.repeat_event ?? null,
          scheduled_start: item.scheduled_start ?? null,
          start_on_boot: item.start_on_boot ?? 0,
          drift_score_enabled: item.drift_score_enabled ?? 0,
          allow_wrong_way: item.allow_wrong_way ?? 0,
          stream_enabled: item.stream_enabled ?? 0,
          stream_embed_url: item.stream_embed_url ?? null,
          stream_status_url: item.stream_status_url ?? null,
          spectator_enabled: item.spectator_enabled ?? 0,
          spectator_driver_name: item.spectator_driver_name ?? null,
          spectator_guid: item.spectator_guid ?? null,
          spectator_car_key: item.spectator_car_key ?? null,
          spectator_skin_key: item.spectator_skin_key ?? null,
        };
      }
      for (const id of Object.keys(this.instances).map(Number)) {
        if (!res.instances.some((i) => i.id === id)) {
          delete this.instances[id];
        }
      }
      this.loaded = true;
    },

    // syncStatus applies an authoritative /api/server/status snapshot onto an
    // existing instance. Running servers get full live state; stopped servers
    // are cleared so no stale roster/positions linger.
    syncStatus(id: number, s: StatusResponse) {
      const inst = this.instances[id];
      if (!inst) return;
      inst.running = s.is_running;
      if (!s.is_running) {
        inst.players = 0;
        inst.session = null;
        inst.drivers = [];
        inst.positions = [];
        if (inst.telemetry) inst.telemetry.udp_online = false;
        return;
      }
      inst.players = s.players;
      inst.drivers = s.drivers ?? [];
      inst.positions = s.positions ?? [];
      inst.telemetry = s.telemetry ?? inst.telemetry;
      inst.session = s.session
        ? {
            name: s.session.name,
            type: s.session.type_id,
            index: s.session.index,
            current_session_index: s.session.current_session_index,
            session_count: s.session.session_count,
            track: s.session.track,
            track_config: s.session.track_config,
            server_name: s.session.server_name,
            time: s.session.time,
            laps: s.session.laps,
            wait_time: s.session.wait_time,
            ambient_temp: s.session.ambient_temp,
            road_temp: s.session.road_temp,
            weather_graphics: s.session.weather_graphics,
            elapsed_ms: s.session.elapsed_ms,
          }
        : null;
    },

    async refreshInstanceStatus(id: number) {
      const s = await api.get<StatusResponse>(`/api/server/status?instance=${id}`);
      this.syncStatus(id, s);
    },

    // bootstrapLiveState is the authoritative full hydrate: load the instance
    // list, then pull each instance's live status. Used on login, SSE
    // reconnect, and tab-visibility recovery so the UI never needs a refresh.
    async bootstrapLiveState() {
      await this.load();
      await Promise.all(
        this.instanceList.map((i) => this.refreshInstanceStatus(i.id).catch(() => {})),
      );
    },

    // recoverRunning is the cheap periodic heal: only re-pull status for
    // instances currently marked running, recovering dropped one-time
    // players/drivers events without a full instance-list round trip.
    async recoverRunning() {
      await Promise.all(
        this.instanceList
          .filter((i) => i.running)
          .map((i) => this.refreshInstanceStatus(i.id).catch(() => {})),
      );
    },

    // scheduleRecovery runs a throttled bootstrap when an event arrives for an
    // instance we do not yet know about — instead of silently dropping it.
    scheduleRecovery() {
      const now = Date.now();
      if (recoveryInflight || now - lastRecoveryAt < 3000) return;
      recoveryInflight = true;
      lastRecoveryAt = now;
      void this.bootstrapLiveState().finally(() => {
        recoveryInflight = false;
      });
    },

    connect() {
      if (this.unsubscribe) return;
      this.unsubscribe = subscribeServerEvents(
        (event) => this.applyEvent(event),
        (connected) => {
          this.connected = connected;
          // Re-sync after a reconnect: events may have been missed while down.
          if (connected && this.loaded) void this.bootstrapLiveState();
        },
      );
    },

    disconnect() {
      this.unsubscribe?.();
      this.unsubscribe = null;
      this.connected = false;
    },

    applyEvent(event: ServerEvent) {
      if (event.type === "content_job") {
        useContentStore().applyJobEvent(event);
        return;
      }

      if (FEED_TYPES.has(event.type)) {
        useFeedStore().ingest(event);
        return;
      }

      const inst = this.instances[event.instance_id];
      if (!inst) {
        // Event for an instance we have not loaded yet (startup race, or one
        // created since last load). Heal via a throttled bootstrap instead of
        // dropping the update and going stale until a manual refresh.
        this.scheduleRecovery();
        return;
      }

      switch (event.type) {
        case "snapshot":
          inst.running = event.data.running;
          inst.players = event.data.players;
          inst.session = event.data.session;
          inst.drivers = event.data.drivers ?? [];
          inst.positions = event.data.positions ?? [];
          inst.telemetry = event.data.telemetry ?? null;
          break;
        case "server":
          inst.running = event.data.running;
          if (event.data.running) {
            useFeedStore().add({ type: "server", icon: "power", tone: "ok", text: `${inst.name} started`, link: `/server/${inst.id}` });
            useToastStore().push("success", `${inst.name} — race started`, undefined, { label: "Go to race", to: `/server/${inst.id}` });
          } else {
            useFeedStore().add({ type: "server", icon: "power", tone: "dim", text: `${inst.name} stopped` });
            inst.players = 0;
            inst.session = null;
            inst.drivers = [];
            inst.positions = [];
            if (inst.telemetry) inst.telemetry.udp_online = false;
          }
          break;
        case "telemetry":
          inst.telemetry = event.data.telemetry ?? inst.telemetry;
          break;
        case "players":
          inst.players = event.data.players;
          break;
        case "drivers":
          inst.drivers = event.data.drivers ?? [];
          break;
        case "positions":
          inst.positions = event.data.positions ?? [];
          break;
        case "session":
          inst.session = event.data;
          break;
      }
    },

    async start(id: number) {
      await api.post(`/api/server/start?instance=${id}`);
      await this.load();
    },

    async stop(id: number) {
      await api.post(`/api/server/stop?instance=${id}`);
      await this.load();
    },

    async setRunMode(id: number, mode: RunMode, eventId?: number) {
      await api.put(`/api/instances/${id}/runmode`, {
        run_mode: mode,
        repeat_event_id: mode === "repeat_event" ? (eventId ?? null) : null,
      });
      await this.load();
    },

    async setSchedule(id: number, scheduledStart: number | null) {
      await api.put(`/api/instances/${id}/schedule`, { scheduled_start: scheduledStart });
      await this.load();
    },
  },
});
