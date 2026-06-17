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
