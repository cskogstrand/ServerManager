import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { createPinia, setActivePinia } from "pinia";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn() },
  ApiError: class ApiError extends Error {},
}));

import { api } from "@/lib/api";
import { useServerStore, type InstanceState } from "@/stores/server";

const apiGet = api.get as Mock;

function seedInstance(overrides: Partial<InstanceState> = {}): InstanceState {
  return {
    id: 1,
    name: "Test",
    udp_port: 9600,
    tcp_port: 9600,
    http_port: 8081,
    plugin_port: 5000,
    plugin_listen_port: 5001,
    running: false,
    players: 0,
    session: null,
    drivers: [],
    positions: [],
    telemetry: null,
    run_mode: "manual_queue",
    repeat_event_id: null,
    repeat_event: null,
    scheduled_start: null,
    start_on_boot: 0,
    drift_score_enabled: 0,
    drift_scoring_mode_id: 1,
    allow_wrong_way: 0,
    stream_enabled: 0,
    stream_embed_url: null,
    stream_status_url: null,
    spectator_enabled: 0,
    spectator_driver_name: null,
    spectator_guid: null,
    spectator_car_key: null,
    spectator_skin_key: null,
    ...overrides,
  };
}

function listItem(id = 1, running = true) {
  return {
    id,
    name: "Test",
    udp_port: 9600,
    tcp_port: 9600,
    http_port: 8081,
    plugin_port: 5000,
    plugin_listen_port: 5001,
    is_running: running,
    players: running ? 1 : 0,
    run_mode: "manual_queue",
    repeat_event_id: null,
    repeat_event: null,
    scheduled_start: null,
      start_on_boot: 0,
      drift_score_enabled: 0,
      drift_scoring_mode_id: 1,
      allow_wrong_way: 0,
    stream_enabled: 0,
    stream_embed_url: null,
    stream_status_url: null,
    spectator_enabled: 0,
    spectator_driver_name: null,
    spectator_guid: null,
    spectator_car_key: null,
    spectator_skin_key: null,
  };
}

function statusResponse(running = true) {
  return {
    is_running: running,
    players: running ? 2 : 0,
    session: running
      ? {
          name: "Race 1",
          type: "Race",
          type_id: 3,
          index: 2,
          current_session_index: 2,
          session_count: 3,
          track: "spa",
          track_config: "",
          server_name: "SM",
          time: 20,
          laps: 0,
          wait_time: 0,
          ambient_temp: 22,
          road_temp: 28,
          weather_graphics: "clear",
          elapsed_ms: 60000,
        }
      : null,
    drivers: running ? [{ car_id: 1, name: "A", connected: true }] : [],
    positions: running ? [{ car_id: 1, x: 1, y: 0, z: 2 }] : [],
    telemetry: { udp_online: running, last_packet_ms: 1, plugin_listen_port: 5001, plugin_send_port: 5000 },
  };
}

function routeApi() {
  apiGet.mockImplementation((url: string) => {
    if (url.startsWith("/api/instances")) return Promise.resolve({ instances: [listItem(1, true)] });
    if (url.startsWith("/api/server/status")) return Promise.resolve(statusResponse(true));
    return Promise.resolve({});
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
  apiGet.mockReset();
});

describe("server store telemetry events", () => {
  it("snapshot applies players, drivers, positions and telemetry", () => {
    const store = useServerStore();
    store.instances[1] = seedInstance();

    store.applyEvent({
      type: "snapshot",
      instance_id: 1,
      ts: 0,
      data: {
        running: true,
        players: 2,
        session: null,
        drivers: [{ car_id: 1, name: "A", connected: true }],
        positions: [{ car_id: 1, x: 0, y: 0, z: 0 }],
        telemetry: { udp_online: true, last_packet_ms: 1000, plugin_listen_port: 5001, plugin_send_port: 5000 },
      },
    });

    const inst = store.instances[1];
    expect(inst.running).toBe(true);
    expect(inst.players).toBe(2);
    expect(inst.drivers).toHaveLength(1);
    expect(inst.positions).toHaveLength(1);
    expect(inst.telemetry?.udp_online).toBe(true);
  });

  it("telemetry event updates only telemetry health", () => {
    const store = useServerStore();
    store.instances[1] = seedInstance({ players: 5 });

    store.applyEvent({
      type: "telemetry",
      instance_id: 1,
      ts: 0,
      data: { telemetry: { udp_online: true, last_packet_ms: 42, plugin_listen_port: 5001, plugin_send_port: 5000 } },
    });

    expect(store.instances[1].telemetry?.udp_online).toBe(true);
    expect(store.instances[1].players).toBe(5);
  });

  it("server stop clears roster and marks telemetry offline", () => {
    const store = useServerStore();
    store.instances[1] = seedInstance({
      running: true,
      players: 3,
      drivers: [{ car_id: 1, name: "A", car: "", skin: "", guid: "", laps: 0, last_lap_ms: 0, best_lap_ms: 0, connected: true }],
      positions: [{ car_id: 1, x: 0, y: 0, z: 0, velocity_x: 0, velocity_y: 0, velocity_z: 0, gear: 0, engine_rpm: 0, normalized_spline_pos: 0, updated_at: 0 }],
      telemetry: { udp_online: true, last_packet_ms: 1, last_driver_ms: 1, last_position_ms: 1, plugin_listen_port: 5001, plugin_send_port: 5000 },
    });

    store.applyEvent({ type: "server", instance_id: 1, ts: 0, data: { running: false } });

    const inst = store.instances[1];
    expect(inst.running).toBe(false);
    expect(inst.players).toBe(0);
    expect(inst.drivers).toHaveLength(0);
    expect(inst.positions).toHaveLength(0);
    expect(inst.telemetry?.udp_online).toBe(false);
  });
});

describe("server store live-state recovery", () => {
  it("bootstrap loads instances then hydrates full status", async () => {
    routeApi();
    const store = useServerStore();

    await store.bootstrapLiveState();

    const inst = store.instances[1];
    expect(inst).toBeTruthy();
    expect(inst.running).toBe(true);
    expect(inst.players).toBe(2);
    expect(inst.drivers).toHaveLength(1);
    expect(inst.positions).toHaveLength(1);
    expect(inst.telemetry?.udp_online).toBe(true);
    // session hydrated from REST using numeric type_id
    expect(inst.session?.type).toBe(3);
    expect(inst.session?.session_count).toBe(3);
    expect(apiGet).toHaveBeenCalledWith("/api/instances");
    expect(apiGet).toHaveBeenCalledWith("/api/server/status?instance=1");
  });

  it("refreshInstanceStatus updates players, drivers, positions and telemetry", async () => {
    apiGet.mockResolvedValue(statusResponse(true));
    const store = useServerStore();
    store.instances[1] = seedInstance();

    await store.refreshInstanceStatus(1);

    const inst = store.instances[1];
    expect(inst.players).toBe(2);
    expect(inst.drivers).toHaveLength(1);
    expect(inst.positions).toHaveLength(1);
    expect(inst.telemetry?.udp_online).toBe(true);
  });

  it("syncStatus on a stopped instance clears stale drivers/positions", () => {
    const store = useServerStore();
    store.instances[1] = seedInstance({
      running: true,
      players: 4,
      drivers: [{ car_id: 1, name: "A", car: "", skin: "", guid: "", laps: 0, last_lap_ms: 0, best_lap_ms: 0, connected: true }],
      positions: [{ car_id: 1, x: 0, y: 0, z: 0, velocity_x: 0, velocity_y: 0, velocity_z: 0, gear: 0, engine_rpm: 0, normalized_spline_pos: 0, updated_at: 0 }],
      telemetry: { udp_online: true, last_packet_ms: 1, last_driver_ms: 1, last_position_ms: 1, plugin_listen_port: 5001, plugin_send_port: 5000 },
    });

    store.syncStatus(1, statusResponse(false) as never);

    const inst = store.instances[1];
    expect(inst.running).toBe(false);
    expect(inst.players).toBe(0);
    expect(inst.drivers).toHaveLength(0);
    expect(inst.positions).toHaveLength(0);
    expect(inst.session).toBeNull();
    expect(inst.telemetry?.udp_online).toBe(false);
  });

  it("unknown-instance snapshot triggers one throttled recovery instead of being lost", () => {
    // Far-future clock so the module-level throttle always passes on the first call.
    vi.spyOn(Date, "now").mockReturnValue(99_999_999_999_999);
    const store = useServerStore();
    const spy = vi.spyOn(store, "bootstrapLiveState").mockResolvedValue();

    const evt = { type: "snapshot" as const, instance_id: 42, ts: 0, data: { running: true, players: 1 } };
    store.applyEvent(evt);
    store.applyEvent(evt); // within throttle window → ignored

    expect(spy).toHaveBeenCalledTimes(1);
    vi.restoreAllMocks();
  });
});
