// API contract for the Driver Stats / Driver Detail surfaces.
//
// Nothing here is persisted by the backend yet — drivers, drift scores and lap
// times are live-only and reset every session. These types describe the shape
// the aggregation endpoints WILL return once the persistence layer (driver,
// driver_session, driver_lap, driver_drift_run tables + event hooks) lands; see
// docs/driver-stats.md. The frontend ships now against mock data (driversApi.ts)
// and lights up automatically when /api/drivers starts answering.
//
// Field names are snake_case to match the Go JSON the endpoints will emit.

export type SessionKind = "race" | "qualify" | "practice" | "drift";

export interface CarRef {
  key: string;
  name: string;
  skin?: string;
}

export interface TrackRef {
  key: string;
  config?: string;
  name: string;
  country?: string;
}

// One session/run a driver completed. Race-like sessions carry a finishing
// position + lap time; drift sessions carry a score. `kind` says which to read.
export interface DriverResult {
  session_id: string;
  kind: SessionKind;
  date: number; // epoch ms
  track: TrackRef;
  car: CarRef;
  // race / qualify / practice
  position?: number | null;
  entrants?: number | null;
  best_lap_ms?: number | null;
  laps?: number | null;
  // drift
  drift_score?: number | null;
  drift_best?: number | null;
}

// A still or clip auto-captured from a driver's stream on a big drift spike.
export interface MediaItem {
  id: string;
  kind: "screenshot" | "clip";
  url: string;
  thumb_url?: string;
  caption: string;
  captured_at: number; // epoch ms
  duration_s?: number; // clips only
  // Why the capture fired — the drift spike that tripped the trigger.
  trigger?: {
    drift_score: number;
    delta: number;
    track?: string;
  };
}

export interface StreamRef {
  embed_url: string;
  status: "live" | "offline" | "unknown" | "not_configured";
}

// Row in the leaderboard / list.
export interface DriverSummary {
  guid: string;
  name: string;
  online: boolean;
  avatar_url?: string | null;
  first_seen: number; // epoch ms
  last_seen: number; // epoch ms
  sessions: number;
  total_laps: number;
  best_drift: number;
  best_lap_ms: number; // 0 when the driver has never set a timed lap
  podiums: number;
  favourite_car?: CarRef | null;
  favourite_track?: TrackRef | null;
  last_result: DriverResult | null;
  drift_trend: number[]; // recent drift-run scores, oldest → newest (sparkline)
  // Highlight clip auto-captured on this driver's best drift run, if any. Lets
  // the leaderboard link straight to the video without fetching full detail.
  best_drift_clip?: MediaItem | null;
  // Live overlay, filled from the SSE store — not the stats endpoint.
  live_drift?: number;
}

export interface DriverDetail extends DriverSummary {
  results: DriverResult[]; // newest first
  media: MediaItem[];
  stream?: StreamRef | null;
}

// One ranked row in the all-servers leaderboard: a single drift run
// (kind "drift") or a session's best timed lap (kind "lap"). NOT collapsed per
// driver — every run/lap is its own entry, so a driver can appear many times.
export interface ScoreEntry {
  id: string;
  guid: string;
  driver: string;
  kind: "drift" | "lap";
  date: number; // epoch ms
  track: TrackRef;
  car: CarRef;
  online: boolean;
  drift_score?: number | null; // kind "drift"
  best_lap_ms?: number | null; // kind "lap"
  position?: number | null; // race finish, when known
  entrants?: number | null;
  clip?: MediaItem | null; // highlight clip captured on this drift run, if any
}
