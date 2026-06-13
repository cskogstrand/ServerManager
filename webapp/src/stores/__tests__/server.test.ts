import { beforeEach, describe, expect, it } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { useServerStore, type InstanceState } from "@/stores/server";

function seedInstance(): InstanceState {
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

describe("server store telemetry", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

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
    expect(inst.telemetry?.plugin_listen_port).toBe(5001);
  });

  it("telemetry event updates only telemetry health", () => {
    const store = useServerStore();
    store.instances[1] = { ...seedInstance(), players: 5 };

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
    store.instances[1] = {
      ...seedInstance(),
      running: true,
      players: 3,
      drivers: [{ car_id: 1, name: "A", car: "", skin: "", guid: "", laps: 0, last_lap_ms: 0, best_lap_ms: 0, connected: true }],
      positions: [{ car_id: 1, x: 0, y: 0, z: 0, velocity_x: 0, velocity_y: 0, velocity_z: 0, gear: 0, engine_rpm: 0, normalized_spline_pos: 0, updated_at: 0 }],
      telemetry: { udp_online: true, last_packet_ms: 1, last_driver_ms: 1, last_position_ms: 1, plugin_listen_port: 5001, plugin_send_port: 5000 },
    };

    store.applyEvent({ type: "server", instance_id: 1, ts: 0, data: { running: false } });

    const inst = store.instances[1];
    expect(inst.running).toBe(false);
    expect(inst.players).toBe(0);
    expect(inst.drivers).toHaveLength(0);
    expect(inst.positions).toHaveLength(0);
    expect(inst.telemetry?.udp_online).toBe(false);
  });
});
