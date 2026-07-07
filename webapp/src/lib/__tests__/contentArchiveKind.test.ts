import { describe, expect, it } from "vitest";
import { detectArchiveKindFromPaths } from "../contentArchiveKind";

describe("content archive kind detection", () => {
  it("detects car archives from ui_car.json", () => {
    expect(detectArchiveKindFromPaths(["content/cars/ks_car/ui/ui_car.json"])).toBe("car");
  });

  it("detects track archives from ui_track.json", () => {
    expect(detectArchiveKindFromPaths(["content/tracks/ks_track/layout/ui/ui_track.json"])).toBe("track");
  });

  it("detects mixed archives", () => {
    expect(detectArchiveKindFromPaths(["ks_car/ui/ui_car.json", "ks_track/ui/ui_track.json"])).toBe("mixed");
  });

  it("returns unknown when no supported content marker exists", () => {
    expect(detectArchiveKindFromPaths(["readme.txt"])).toBe("unknown");
  });
});
