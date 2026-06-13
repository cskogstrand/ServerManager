import { defineStore } from "pinia";
import { api } from "@/lib/api";
import { subscribeServerEvents, type ServerEvent } from "@/lib/sse";
import { useContentStore } from "@/stores/content";

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
  stream_enabled: number | null;
  stream_embed_url: string | null;
  stream_status_url: string | null;
  spectator_enabled: number | null;
  spectator_driver_name: string | null;
  spectator_guid: string | null;
  spectator_car_key: string | null;
  spectator_skin_key: string | null;
}

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

    connect() {
      if (this.unsubscribe) return;
      this.unsubscribe = subscribeServerEvents(
        (event) => this.applyEvent(event),
        (connected) => {
          this.connected = connected;
          // Re-sync after a reconnect: events may have been missed
          if (connected && this.loaded) void this.load();
        },
      );
    },

    disconnect() {
      this.unsubscribe?.();
      this.unsubscribe = null;
      this.connected = false;
    },

    applyEvent(event: ServerEvent) {
      const inst = this.instances[event.instance_id];
      switch (event.type) {
        case "snapshot":
          if (!inst) return;
          inst.running = event.data.running;
          inst.players = event.data.players;
          inst.session = event.data.session;
          inst.drivers = event.data.drivers ?? [];
          inst.positions = event.data.positions ?? [];
          inst.telemetry = event.data.telemetry ?? null;
          break;
        case "server":
          if (!inst) return;
          inst.running = event.data.running;
          if (!event.data.running) {
            inst.players = 0;
            inst.session = null;
            inst.drivers = [];
            inst.positions = [];
            if (inst.telemetry) inst.telemetry.udp_online = false;
          }
          break;
        case "telemetry":
          if (!inst) return;
          inst.telemetry = event.data.telemetry ?? inst.telemetry;
          break;
        case "players":
          if (!inst) return;
          inst.players = event.data.players;
          break;
        case "drivers":
          if (!inst) return;
          inst.drivers = event.data.drivers ?? [];
          break;
        case "positions":
          if (!inst) return;
          inst.positions = event.data.positions ?? [];
          break;
        case "session":
          if (!inst) return;
          inst.session = event.data;
          break;
        case "content_job":
          useContentStore().applyJobEvent(event);
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
