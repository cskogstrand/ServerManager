import { describe, expect, it } from "vitest";
import { demoPathPoint, demoTrackPosition, type DemoMapMeta } from "@/lib/broadcastDemo";
import { trackMapPoint } from "@/lib/raceTelemetry";
import type { CarPositionState } from "@/stores/server";

const driftPlaygroundMeta: DemoMapMeta = {
  width: 236.851,
  height: 173.273,
  x_offset: 136.434,
  z_offset: 101.236,
  scale_factor: 1,
  margin: 20,
};

function pos(x: number, z: number): CarPositionState {
  return {
    car_id: 1,
    x,
    y: 0,
    z,
    velocity_x: 0,
    velocity_y: 0,
    velocity_z: 0,
    gear: 2,
    engine_rpm: 0,
    normalized_spline_pos: 0,
    updated_at: 0,
  };
}

describe("broadcast demo route", () => {
  it("projects route samples back onto the fixed track map", () => {
    for (const phase of [0, 0.125, 0.25, 0.5, 0.75, 0.99]) {
      const sample = demoPathPoint(phase);
      const world = demoTrackPosition(phase, driftPlaygroundMeta);
      const point = trackMapPoint(pos(world.x, world.z), driftPlaygroundMeta);

      expect(point.inBounds).toBe(true);
      expect(parseFloat(point.left)).toBeCloseTo(sample.fx * 100, 3);
      expect(parseFloat(point.top)).toBeCloseTo(sample.fy * 100, 3);
    }
  });

  it("wraps phases around the lap", () => {
    expect(demoPathPoint(1)).toEqual(demoPathPoint(0));
    expect(demoPathPoint(-0.25)).toEqual(demoPathPoint(0.75));
  });
});
