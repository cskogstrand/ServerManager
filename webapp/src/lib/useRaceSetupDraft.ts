// Shared "race setup" draft model — the editable shape behind an event, used
// by the Events library, the Setup Workbench, the Run Plan (queue) and Server
// Detail so they all speak one vocabulary instead of bespoke per-page forms.

export interface RaceSetupDraft {
  id: number | null;
  name: string;
  track_key: string;
  track_config: string;
  track_name: string;
  pitboxes: number | null;
  difficulty_id: number | null;
  difficulty_name: string;
  session_id: number | null;
  session_name: string;
  class_id: number | null;
  class_name: string;
  entries: number | null;
  time_id: number | null;
  time_name: string;
  race_laps: number | null;
  strategy: number | null;
}

export function emptyRaceSetup(): RaceSetupDraft {
  return {
    id: null,
    name: "",
    track_key: "",
    track_config: "",
    track_name: "",
    pitboxes: null,
    difficulty_id: null,
    difficulty_name: "",
    session_id: null,
    session_name: "",
    class_id: null,
    class_name: "",
    entries: null,
    time_id: null,
    time_name: "",
    race_laps: 0,
    strategy: 1,
  };
}

// The category GET serializes events with legacy mixed-case keys and
// stringified ids — normalize once here.
export function normalizeRaceSetup(raw: Record<string, unknown>): RaceSetupDraft {
  const num = (v: unknown) => (v === null || v === undefined || v === "" ? null : Number(v));
  const str = (v: unknown) => (v == null ? "" : String(v));
  return {
    id: (raw.Id as number) ?? null,
    name: str(raw.name),
    track_key: str(raw.CacheTrackKey),
    track_config: str(raw.CacheTrackConfig),
    track_name: str(raw.TrackName ?? raw.CacheTrackKey),
    pitboxes: num(raw.Pitboxes),
    difficulty_id: num(raw.difficulty),
    difficulty_name: str(raw.DifficultyName),
    session_id: num(raw.session),
    session_name: str(raw.SessionName),
    class_id: num(raw.class),
    class_name: str(raw.ClassName),
    entries: num(raw.Entries),
    time_id: num(raw.time),
    time_name: str(raw.TimeName),
    race_laps: num(raw.race_laps),
    strategy: num(raw.strategy),
  };
}

// raceSetupBody builds the POST/PUT payload for /api/events and /api/event/:id.
export function raceSetupBody(d: RaceSetupDraft, categoryId: number) {
  return {
    event_category_id: categoryId,
    name: d.name?.trim() ?? "",
    track_key: d.track_key,
    track_config: d.track_config,
    difficulty_id: d.difficulty_id,
    session_id: d.session_id,
    class_id: d.class_id,
    time_id: d.time_id,
    race_laps: d.race_laps ?? 0,
    strategy: d.strategy ?? 1,
  };
}

// raceSetupValid reports whether the draft has everything a runnable event
// needs: a track plus all four presets.
export function raceSetupValid(d: RaceSetupDraft | null): boolean {
  return !!(d && d.track_key && d.class_id && d.session_id && d.time_id && d.difficulty_id);
}
