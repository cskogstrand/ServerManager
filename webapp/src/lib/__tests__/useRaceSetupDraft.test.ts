import { describe, expect, it } from "vitest";
import {
  emptyRaceSetup,
  normalizeRaceSetup,
  raceSetupBody,
  raceSetupValid,
} from "@/lib/useRaceSetupDraft";

describe("useRaceSetupDraft", () => {
  it("emptyRaceSetup is a blank, invalid draft", () => {
    const d = emptyRaceSetup();
    expect(d.id).toBeNull();
    expect(d.race_laps).toBe(0);
    expect(d.strategy).toBe(1);
    expect(d.drift_scoring_mode_id).toBeNull();
    expect(raceSetupValid(d)).toBe(false);
  });

  it("normalizeRaceSetup maps the legacy mixed-case/stringified shape", () => {
    const d = normalizeRaceSetup({
      Id: 7,
      name: "Round 1",
      CacheTrackKey: "spa",
      CacheTrackConfig: "",
      TrackName: "Spa-Francorchamps",
      Pitboxes: "24",
      difficulty: "3",
      DifficultyName: "Pro",
      session: "2",
      SessionName: "Race weekend",
      class: "5",
      ClassName: "GT3",
      Entries: "12",
      time: "4",
      TimeName: "Afternoon",
      drift_scoring_mode_id: "6",
      race_laps: "10",
      strategy: "1",
    });
    expect(d.id).toBe(7);
    expect(d.name).toBe("Round 1");
    expect(d.track_key).toBe("spa");
    expect(d.track_name).toBe("Spa-Francorchamps");
    expect(d.pitboxes).toBe(24);
    expect(d.difficulty_id).toBe(3);
    expect(d.class_id).toBe(5);
    expect(d.entries).toBe(12);
    expect(d.drift_scoring_mode_id).toBe(6);
    expect(d.race_laps).toBe(10);
    expect(raceSetupValid(d)).toBe(true);
  });

  it("raceSetupValid requires a track and all four presets", () => {
    const base = { ...emptyRaceSetup(), track_key: "spa", class_id: 1, session_id: 1, time_id: 1, difficulty_id: 1 };
    expect(raceSetupValid(base)).toBe(true);
    expect(raceSetupValid({ ...base, track_key: "" })).toBe(false);
    expect(raceSetupValid({ ...base, class_id: null })).toBe(false);
    expect(raceSetupValid({ ...base, time_id: null })).toBe(false);
    expect(raceSetupValid(null)).toBe(false);
  });

  it("raceSetupBody builds the API payload with trimmed name and defaults", () => {
    const d = { ...emptyRaceSetup(), name: "  Sprint  ", track_key: "spa", track_config: "gp", class_id: 5, session_id: 2, time_id: 4, difficulty_id: 3, race_laps: null, strategy: null };
    const body = raceSetupBody(d, 9);
    expect(body.event_category_id).toBe(9);
    expect(body.name).toBe("Sprint");
    expect(body.track_key).toBe("spa");
    expect(body.track_config).toBe("gp");
    expect(body.class_id).toBe(5);
    expect(body.drift_scoring_mode_id).toBeNull();
    expect(body.race_laps).toBe(0); // null → 0
    expect(body.strategy).toBe(1); // null → 1
    expect(raceSetupBody({ ...d, drift_scoring_mode_id: 7 }, 9).drift_scoring_mode_id).toBe(7);
  });
});
