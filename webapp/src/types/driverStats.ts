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
  // Guest-driver attribution for a standalone manual recording (null = the
  // account's own name). Only meaningful for items in the manual-recordings reel.
  guest_driver_id?: number | null;
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
  // Set for roster guest drivers listed alongside GUID drivers: `guid` is then
  // empty and `guest_id` points at the guest's profile (/guest-drivers/:id).
  is_guest?: boolean;
  guest_id?: number;
}

export interface DriverDetail extends DriverSummary {
  results: DriverResult[]; // newest first
  media: MediaItem[];
  stream?: StreamRef | null;
  // Activity grouped by connection (connect→disconnect) — the user-facing
  // "session" view. Newest connection first; the live one (if any) is first.
  // (Named session_history to avoid clashing with the `sessions` count above.)
  session_history: DriverSession[];
}

// One timed lap inside a session. is_best flags the session's fastest lap.
export interface SessionLap {
  lap: number;
  laptime_ms: number;
  cuts: number;
  is_best?: boolean;
}

// One completed drift run inside a session, with the highlight clip captured on
// it when one exists.
export interface DriftRun {
  id: string;
  score: number;
  ended_at: number; // epoch ms
  clip?: MediaItem | null;
}

// A "session": one continuous connection from connect to disconnect. Groups the
// AC-session segments (practice/qualify/race), per-lap times, drift runs and
// captured media of a single stint, plus searchable tags. `left_at` is null
// while the driver is still connected.
export interface DriverSession {
  id: string;
  joined_at: number; // epoch ms
  left_at: number | null; // null = still connected
  online: boolean;
  track: TrackRef;
  car: CarRef;
  tags: string[];
  best_lap_ms?: number | null;
  laps_total: number;
  best_drift?: number | null;
  segments: DriverResult[]; // per AC session, chronological
  laps: SessionLap[];
  drift_runs: DriftRun[];
  media: MediaItem[];
  // Set when the whole session (connection) is attributed to a guest driver
  // (a real person sharing this account's GUID); null/absent = the account's
  // own name. The UI resolves the name from its loaded guest roster.
  guest_driver_id?: number | null;
}

// One hit in the global session search (GET /api/driver-sessions): a connection
// matched by tag / driver name / track, enough to render a result card and
// deep-link to /drivers/:guid?session=:id.
export interface SessionSearchResult {
  id: string;
  guid: string;
  driver: string;
  avatar_url?: string | null;
  joined_at: number;
  left_at: number | null;
  online: boolean;
  track: TrackRef;
  car: CarRef;
  tags: string[];
  laps: number;
  best_lap_ms?: number | null;
  best_drift?: number | null;
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
  // Set when this row is attributed to a guest driver (a real person sharing
  // the account's GUID): `driver` above is then that person's name. Lets the
  // leaderboard editor preselect the current assignment and link the row to the
  // guest's profile.
  guest_driver_id?: number | null;
}

// Profile served at GET /api/guest-drivers/:id. Reuses the GUID driver summary
// fields (KPIs, favourites, trend) so the guest detail page can mirror the GUID
// one, plus guest-only fields. Guests have no GUID (`guid` is "") and no stream.
export interface GuestDriverDetail extends DriverSummary {
  is_guest: true;
  notes?: string;
  created_at: number; // epoch ms — when the guest was added to the roster
  results: DriverResult[]; // newest first
  media: MediaItem[]; // highlight clips from their attributed drift runs
}
