import { describe, expect, it } from "vitest";
import { computeRunningOrder, deltaTime, gearLabel, lapTime } from "@/lib/raceTelemetry";
import type { CarPositionState, DriverState } from "@/stores/server";

function driver(p: Partial<DriverState>): DriverState {
  return {
    car_id: 0,
    name: "",
    car: "",
    skin: "",
    guid: "",
    laps: 0,
    last_lap_ms: 0,
    best_lap_ms: 0,
    connected: true,
    ...p,
  };
}
function pos(car_id: number, spline: number, rpm = 0): CarPositionState {
  return {
    car_id,
    x: 0,
    y: 0,
    z: 0,
    velocity_x: 0,
    velocity_y: 0,
    velocity_z: 0,
    gear: 2,
    engine_rpm: rpm,
    normalized_spline_pos: spline,
    updated_at: 0,
  };
}

describe("computeRunningOrder", () => {
  it("orders a race by laps then track position, labelling lap gaps", () => {
    const drivers = [
      driver({ car_id: 1, name: "A", laps: 3 }),
      driver({ car_id: 2, name: "B", laps: 4 }),
      driver({ car_id: 3, name: "C", laps: 4 }),
    ];
    const positions = [pos(1, 0.9), pos(2, 0.2), pos(3, 0.8)];

    const rows = computeRunningOrder(drivers, positions, 3);
    expect(rows.map((r) => r.car_id)).toEqual([3, 2, 1]); // C & B on lap 4 (C ahead on spline), A a lap down
    expect(rows[0].gapLabel).toBe("LEADER");
    expect(rows[1].gapLabel).toBe("—"); // same lap as leader → interval unknown
    expect(rows[2].gapLabel).toBe("+1 LAP");
  });

  it("orders qualifying by best lap with a time gap to pole", () => {
    const drivers = [
      driver({ car_id: 1, name: "A", best_lap_ms: 91500 }),
      driver({ car_id: 2, name: "B", best_lap_ms: 90250 }),
      driver({ car_id: 3, name: "C", best_lap_ms: 0 }),
    ];
    const rows = computeRunningOrder(drivers, [], 2);
    expect(rows.map((r) => r.car_id)).toEqual([2, 1, 3]); // fastest first, no-timer last
    expect(rows[0].gapLabel).toBe("POLE");
    expect(rows[1].gapLabel).toBe("+1.250");
    expect(rows[2].gapLabel).toBe("NO TIME");
  });
});

describe("formatters", () => {
  it("formats lap times and deltas", () => {
    expect(lapTime(0)).toBe("—");
    expect(lapTime(91234)).toBe("1:31.234");
    expect(deltaTime(1250)).toBe("1.250");
    expect(deltaTime(61250)).toBe("1:01.250");
  });

  it("maps AC gears to labels", () => {
    expect(gearLabel(pos(1, 0, 0))).toBe("1"); // gear 2 → first
    expect(gearLabel({ ...pos(1, 0), gear: 0 })).toBe("R");
    expect(gearLabel({ ...pos(1, 0), gear: 1 })).toBe("N");
  });
});
