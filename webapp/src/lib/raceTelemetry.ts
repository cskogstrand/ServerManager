// Shared race-telemetry derivations for the live-timing tower (ServerDetail)
// and the broadcast overlay (Broadcast.vue), so both surfaces order the field
// and label gaps identically. All inputs come straight from the SSE-fed store.
import type { CarPositionState, DriverState } from "@/stores/server";

// AC numeric session type → display label (mirrors the backend mapping).
export function sessionTypeLabel(t: number | null | undefined): string {
  return ["Booking", "Practice", "Qualify", "Race"][t ?? -1] ?? "—";
}

// Velocity vector magnitude → km/h.
export function speedKmh(pos?: CarPositionState | null): number {
  if (!pos) return 0;
  const ms = Math.sqrt(pos.velocity_x ** 2 + pos.velocity_y ** 2 + pos.velocity_z ** 2);
  return Math.round(ms * 3.6);
}

// AC gear convention: 0 = reverse, 1 = neutral, 2 = first gear, …
export function gearLabel(pos?: CarPositionState | null): string {
  if (!pos) return "—";
  const g = pos.gear;
  if (g <= 0) return "R";
  if (g === 1) return "N";
  return String(g - 1);
}

// Lap time in ms → "M:SS.mmm".
export function lapTime(ms: number): string {
  if (!ms) return "—";
  const m = Math.floor(ms / 60000);
  const s = Math.floor((ms % 60000) / 1000);
  const t = ms % 1000;
  return `${m}:${String(s).padStart(2, "0")}.${String(t).padStart(3, "0")}`;
}

// Positive delta in ms → "S.mmm" (or "M:SS.mmm" past a minute). Caller prefixes
// the sign so the same helper serves "+1.234" gaps and bare durations.
export function deltaTime(ms: number): string {
  if (ms <= 0) return "0.000";
  const s = ms / 1000;
  if (s < 60) return s.toFixed(3);
  const m = Math.floor(s / 60);
  return `${m}:${(s % 60).toFixed(3).padStart(6, "0")}`;
}

// Elapsed/session clock in ms → "M:SS".
export function formatClock(ms: number): string {
  if (ms <= 0) return "0:00";
  const m = Math.floor(ms / 60000);
  const s = Math.floor((ms % 60000) / 1000);
  return `${m}:${String(s).padStart(2, "0")}`;
}

export type GapTone = "leader" | "warn" | "muted";

export interface TimingRow {
  car_id: number;
  name: string;
  carModel: string;
  skin: string;
  laps: number;
  last_lap_ms: number;
  best_lap_ms: number;
  pos?: CarPositionState;
  connected: boolean;
  position: number;
  isLeader: boolean;
  gapLabel: string;
  gapTone: GapTone;
  splinePct: number;
  driftLive: number;
  driftLast: number;
  driftBest: number;
}

// Order the field and label each car's gap to the leader.
//
// Race (type 3): order by laps, then track position (normalized spline). Real
// time-gaps need per-car timing we do not receive, so the honest gap is the lap
// deficit; same-lap cars read "—" and lean on the spline bar for relative
// position.
//
// Practice/Qualify/Booking: order by best lap. Here the gap to P1 *is*
// meaningful — it is the best-lap delta — so we show "+S.mmm".
export function computeRunningOrder(
  drivers: DriverState[],
  positions: CarPositionState[],
  sessionType: number | null | undefined,
): TimingRow[] {
  const posById = new Map(positions.map((p) => [p.car_id, p]));
  const rows = drivers.map((d) => ({
    car_id: d.car_id,
    name: d.name || `Car ${d.car_id}`,
    carModel: d.car,
    skin: d.skin,
    laps: d.laps,
    last_lap_ms: d.last_lap_ms,
    best_lap_ms: d.best_lap_ms,
    pos: posById.get(d.car_id),
    connected: d.connected,
    driftLive: d.drift_live ?? 0,
    driftLast: d.drift_last ?? 0,
    driftBest: d.drift_best ?? 0,
  }));

  const isRace = sessionType === 3;
  if (isRace) {
    rows.sort(
      (a, b) =>
        b.laps - a.laps ||
        (b.pos?.normalized_spline_pos ?? 0) - (a.pos?.normalized_spline_pos ?? 0) ||
        a.car_id - b.car_id,
    );
  } else {
    const key = (r: (typeof rows)[number]) =>
      r.best_lap_ms && r.best_lap_ms > 0 ? r.best_lap_ms : Number.POSITIVE_INFINITY;
    rows.sort((a, b) => key(a) - key(b) || b.laps - a.laps || a.car_id - b.car_id);
  }

  const leader = rows[0];
  return rows.map((r, i) => {
    let gapLabel = "—";
    let gapTone: GapTone = "muted";
    if (i === 0) {
      gapLabel = isRace ? "LEADER" : r.best_lap_ms ? "POLE" : "—";
      gapTone = "leader";
    } else if (isRace) {
      const dl = leader.laps - r.laps;
      gapLabel = dl > 0 ? `+${dl} LAP${dl > 1 ? "S" : ""}` : "—";
      gapTone = dl > 0 ? "warn" : "muted";
    } else if (r.best_lap_ms > 0 && leader.best_lap_ms > 0) {
      gapLabel = `+${deltaTime(r.best_lap_ms - leader.best_lap_ms)}`;
    } else {
      gapLabel = "NO TIME";
    }
    return {
      ...r,
      position: i + 1,
      isLeader: i === 0,
      gapLabel,
      gapTone,
      splinePct: Math.round((r.pos?.normalized_spline_pos ?? 0) * 100),
    };
  });
}

// RPM bar/shift-light scale: against the fastest-revving car on track (floored
// at 8000) so the fill stays meaningful without knowing each car's redline.
export function rpmCeiling(positions: CarPositionState[]): number {
  const peak = Math.max(8000, ...positions.map((p) => p.engine_rpm || 0));
  return Math.ceil(peak / 1000) * 1000;
}

function mix(a: number, b: number, t: number): number {
  return a + (b - a) * t;
}

function pct(n: number): string {
  return `${Number(n.toFixed(4))}%`;
}

function clamp(n: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, n));
}

export function positionFrameMs(from: CarPositionState, to: CarPositionState): number {
  const dt = to.updated_at - from.updated_at;
  return dt > 0 ? Math.max(120, Math.min(450, dt)) : 220;
}

export function interpolatePosition(from: CarPositionState, to: CarPositionState, t: number): CarPositionState {
  const p = Math.max(0, Math.min(1, t));
  return {
    car_id: to.car_id,
    x: mix(from.x, to.x, p),
    y: mix(from.y, to.y, p),
    z: mix(from.z, to.z, p),
    velocity_x: mix(from.velocity_x, to.velocity_x, p),
    velocity_y: mix(from.velocity_y, to.velocity_y, p),
    velocity_z: mix(from.velocity_z, to.velocity_z, p),
    gear: p < 0.5 ? from.gear : to.gear,
    engine_rpm: Math.round(mix(from.engine_rpm, to.engine_rpm, p)),
    normalized_spline_pos: to.normalized_spline_pos,
    updated_at: to.updated_at,
  };
}

export interface TrackMapProjection {
  width: number;
  height: number;
  x_offset: number;
  z_offset: number;
  scale_factor: number;
  margin: number;
}

export interface TrackMapPoint {
  left: string;
  top: string;
  inBounds: boolean;
  distanceMeters: number;
  angleDeg: number;
}

export function trackMapPoint(
  pos: CarPositionState,
  meta: TrackMapProjection,
  natural?: { w: number; h: number } | null,
  edgeInsetPct = 0,
): TrackMapPoint {
  const scale = meta.scale_factor || 1;
  const width = natural?.w || meta.width;
  const height = natural?.h || meta.height;
  const px = (pos.x + meta.x_offset) / scale + meta.margin;
  const py = (pos.z + meta.z_offset) / scale + meta.margin;
  const edgeX = clamp(px, 0, width);
  const edgeY = clamp(py, 0, height);
  const left = (px / width) * 100;
  const top = (py / height) * 100;
  const inBounds = left >= 0 && left <= 100 && top >= 0 && top <= 100;
  const inset = inBounds ? 0 : edgeInsetPct;
  const dx = px - edgeX;
  const dy = py - edgeY;
  return {
    left: pct(clamp(left, inset, 100 - inset)),
    top: pct(clamp(top, inset, 100 - inset)),
    inBounds,
    distanceMeters: Math.hypot(dx, dy) * scale,
    angleDeg: dx || dy ? (Math.atan2(dy, dx) * 180) / Math.PI : 0,
  };
}
