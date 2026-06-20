// Driver-stats data access. The aggregation endpoints (/api/drivers,
// /api/drivers/:guid) do not exist yet — see docs/driver-stats.md for the
// planned persistence layer. Until they land, these fetchers fall back to
// realistic mock data so the UI can be built and reviewed. When the backend
// starts answering, the pages light up against real data with no further
// frontend change.
import { api, ApiError } from "@/lib/api";
import { lapTime } from "@/lib/raceTelemetry";
import { useContentStore } from "@/stores/content";
import type { CarRef, DriverDetail, DriverResult, DriverSummary, MediaItem, ScoreEntry, TrackRef } from "@/types/driverStats";

// ---- formatting helpers -----------------------------------------------------

export function fmtScore(n: number): string {
  return Math.round(n).toLocaleString("en-US");
}

// Steam64 guids are long; show a stable short tail for the list/detail chips.
export function shortGuid(guid: string): string {
  if (guid.length <= 8) return guid;
  return `…${guid.slice(-6)}`;
}

export function timeAgo(ms: number): string {
  const diff = Date.now() - ms;
  if (diff < 0) return "just now";
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  return `${Math.floor(months / 12)}y ago`;
}

export function fmtDate(ms: number): string {
  return new Date(ms).toLocaleDateString(undefined, { day: "2-digit", month: "short", year: "numeric" });
}

export const sessionKindLabel: Record<string, string> = {
  race: "Race",
  qualify: "Qualify",
  practice: "Practice",
  drift: "Drift",
};

export interface ResultView {
  primary: string;
  secondary: string;
  unit: string;
  tone: "accent" | "warn" | "text";
}

// Collapse a session into a single headline metric. Drift runs read as a score;
// timed sessions read as the best lap with finishing position. A win glows amber.
export function describeResult(r: DriverResult | null): ResultView {
  if (!r) return { primary: "—", secondary: "No sessions yet", unit: "", tone: "text" };
  if (r.kind === "drift") {
    return { primary: fmtScore(r.drift_score ?? 0), secondary: r.track.name, unit: "PTS", tone: "accent" };
  }
  const pos = r.position ?? 0;
  const posLabel = pos ? `P${pos}${r.entrants ? `/${r.entrants}` : ""}` : "—";
  return {
    primary: lapTime(r.best_lap_ms ?? 0),
    secondary: `${posLabel} · ${sessionKindLabel[r.kind]} · ${r.track.name}`,
    unit: "BEST LAP",
    tone: pos === 1 ? "warn" : "text",
  };
}

// ---- public API -------------------------------------------------------------

export async function listDrivers(): Promise<DriverSummary[]> {
  try {
    const res = await api.get<{ drivers: DriverSummary[] }>("/api/drivers");
    const drivers = res.drivers ?? [];
    // Empty roster (fresh server, nobody's raced yet) → seed one dummy driver so
    // the detail page stays reachable for testing. Its detail resolves through
    // the 404 mock fallback in getDriver().
    if (drivers.length === 0) {
      await ensureDummyContent();
      return [toSummary(MOCK[0])];
    }
    return drivers;
  } catch (e) {
    // 404 / 501 → endpoint not built yet. Anything else (auth, network) we let
    // bubble so the page can surface a real error.
    if (e instanceof ApiError && (e.status === 404 || e.status === 501)) {
      await ensureDummyContent();
      return MOCK.map(toSummary);
    }
    throw e;
  }
}

// Flat leaderboard feed: every drift run and timed-lap session across all
// drivers (not collapsed per driver). Falls back to flattening the mock driver
// results when the endpoint isn't built (404/501).
export async function listScores(): Promise<ScoreEntry[]> {
  try {
    const res = await api.get<{ scores: ScoreEntry[] }>("/api/scores");
    return res.scores ?? [];
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 501)) {
      await ensureDummyContent();
      return mockScores();
    }
    throw e;
  }
}

// Flatten the mock driver details into individual score rows — one per drift
// run, one per session that set a timed lap.
function mockScores(): ScoreEntry[] {
  const out: ScoreEntry[] = [];
  for (const d of MOCK) {
    // No capture timestamps in mock, so relate the driver's best clip to their
    // highest drift run — enough to exercise the video button in mock mode.
    const topClip = bestDriftClip(d);
    const maxDrift = Math.max(0, ...d.results.filter((r) => r.kind === "drift").map((r) => r.drift_score ?? 0));
    for (const r of d.results) {
      if (r.kind === "drift") {
        out.push({
          id: `d-${d.guid}-${r.session_id}`,
          guid: d.guid,
          driver: d.name,
          kind: "drift",
          date: r.date,
          track: r.track,
          car: r.car,
          online: d.online,
          drift_score: r.drift_score ?? 0,
          clip: topClip && (r.drift_score ?? 0) === maxDrift ? topClip : null,
        });
      } else if ((r.best_lap_ms ?? 0) > 0) {
        out.push({
          id: `l-${d.guid}-${r.session_id}`,
          guid: d.guid,
          driver: d.name,
          kind: "lap",
          date: r.date,
          track: r.track,
          car: r.car,
          online: d.online,
          best_lap_ms: r.best_lap_ms ?? 0,
          position: r.position ?? null,
          entrants: r.entrants ?? null,
        });
      }
    }
  }
  return out;
}

export async function getDriver(guid: string): Promise<DriverDetail | null> {
  try {
    return await api.get<DriverDetail>(`/api/drivers/${encodeURIComponent(guid)}`);
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 501)) {
      await ensureDummyContent();
      return MOCK.find((d) => d.guid === guid) ?? null;
    }
    throw e;
  }
}

// The canned mock content keys usually aren't installed, so the favourite
// car/track previews 404. Repoint the dummy driver at a real installed car +
// track (random, first skin) so the image lookups actually resolve. Runs once;
// mutates MOCK[0] so the list summary and detail page stay in sync.
let dummyContentPatched = false;
async function ensureDummyContent(): Promise<void> {
  const content = useContentStore();
  await content.load();
  if (dummyContentPatched) return;
  const d = MOCK[0];
  const cars = content.cars.filter((c) => c.key);
  if (cars.length) {
    const c = cars[Math.floor(Math.random() * cars.length)];
    d.favourite_car = { key: c.key!, name: c.name ?? c.key!, skin: c.skins?.[0]?.key };
  }
  const tracks = content.tracks.filter((t) => t.key);
  if (tracks.length) {
    const t = tracks[Math.floor(Math.random() * tracks.length)];
    d.favourite_track = { key: t.key!, config: t.config || undefined, name: t.name ?? t.key!, country: t.country };
  }
  dummyContentPatched = true;
}

// ---- mock data --------------------------------------------------------------

const NOW = Date.now();
const MIN = 60_000;
const HOUR = 60 * MIN;
const DAY = 24 * HOUR;

// The highlight clip from a driver's best drift run — the clip whose trigger
// score is highest. Surfaced on the leaderboard so a score can link to video.
function bestDriftClip(d: DriverDetail): MediaItem | null {
  const clips = d.media.filter((m) => m.kind === "clip");
  if (!clips.length) return null;
  return clips.reduce((best, m) => ((m.trigger?.drift_score ?? 0) > (best.trigger?.drift_score ?? 0) ? m : best));
}

function toSummary(d: DriverDetail): DriverSummary {
  const { results: _r, media: _m, stream: _s, ...summary } = d;
  return { ...summary, best_drift_clip: bestDriftClip(d) };
}

// Content keys are plausible AC mod folders; thumbnails degrade gracefully when
// a given install doesn't have them. The numbers are what matter for the design.
const cars = {
  ae86: { key: "ks_toyota_ae86_drift", name: "Toyota AE86 Trueno", skin: "drift_panda" } as CarRef,
  s15: { key: "ks_nissan_silvia_s15", name: "Nissan Silvia S15", skin: "rocket_bunny" } as CarRef,
  rx7: { key: "ks_mazda_rx7_spirit_r", name: "Mazda RX-7 Spirit R", skin: "efini" } as CarRef,
  r34: { key: "ks_nissan_skyline_r34", name: "Nissan Skyline GT-R R34", skin: "bayside_blue" } as CarRef,
  e30: { key: "ks_bmw_m3_e30_drift", name: "BMW M3 E30 Drift", skin: "warsteiner" } as CarRef,
  gt3p: { key: "ks_porsche_911_gt3_r_2016", name: "Porsche 911 GT3 R", skin: "01" } as CarRef,
  gt3f: { key: "ks_ferrari_488_gt3", name: "Ferrari 488 GT3", skin: "02" } as CarRef,
  gt3a: { key: "ks_audi_r8_lms_2016", name: "Audi R8 LMS", skin: "03" } as CarRef,
};

const tracks = {
  gunma: { key: "gunma_cycle_sports_center", name: "Gunma Cycle Sports Center", country: "Japan" } as TrackRef,
  ebisu: { key: "ebisu_minami", name: "Ebisu Minami", country: "Japan" } as TrackRef,
  iro: { key: "ek_irohazaka", name: "Irohazaka Downhill", country: "Japan" } as TrackRef,
  nords: { key: "ks_nordschleife", config: "endurance", name: "Nürburgring Nordschleife", country: "Germany" } as TrackRef,
  brands: { key: "ks_brands_hatch", config: "gp", name: "Brands Hatch GP", country: "United Kingdom" } as TrackRef,
  spa: { key: "spa", name: "Circuit de Spa-Francorchamps", country: "Belgium" } as TrackRef,
};

function driftRun(id: string, ago: number, track: TrackRef, car: CarRef, score: number, best: number): DriverResult {
  return { session_id: id, kind: "drift", date: NOW - ago, track, car, drift_score: score, drift_best: best };
}
function raceRun(
  id: string,
  ago: number,
  track: TrackRef,
  car: CarRef,
  position: number,
  entrants: number,
  bestLapMs: number,
  laps: number,
): DriverResult {
  return { session_id: id, kind: "race", date: NOW - ago, track, car, position, entrants, best_lap_ms: bestLapMs, laps };
}

function spikeMedia(prefix: string, track: string): MediaItem[] {
  return [
    {
      id: `${prefix}-c1`,
      kind: "clip",
      url: "#",
      caption: "Full-lock entry through the esses",
      captured_at: NOW - 2 * HOUR,
      duration_s: 14,
      trigger: { drift_score: 4820, delta: 2100, track },
    },
    {
      id: `${prefix}-s1`,
      kind: "screenshot",
      url: "#",
      caption: "Apex clip, smoke everywhere",
      captured_at: NOW - 2 * HOUR - 9 * MIN,
      trigger: { drift_score: 3650, delta: 1480, track },
    },
    {
      id: `${prefix}-c2`,
      kind: "clip",
      url: "#",
      caption: "Tandem chase, lead run",
      captured_at: NOW - DAY,
      duration_s: 22,
      trigger: { drift_score: 6210, delta: 3050, track },
    },
    {
      id: `${prefix}-s2`,
      kind: "screenshot",
      url: "#",
      caption: "Manji on the back straight",
      captured_at: NOW - DAY - 40 * MIN,
      trigger: { drift_score: 2980, delta: 1190, track },
    },
    {
      id: `${prefix}-s3`,
      kind: "screenshot",
      url: "#",
      caption: "Wall-tap save, kept it lit",
      captured_at: NOW - 3 * DAY,
      trigger: { drift_score: 5170, delta: 2620, track },
    },
  ];
}

const MOCK: DriverDetail[] = [
  {
    guid: "76561198011223344",
    name: "Kazuya Mori",
    online: true,
    first_seen: NOW - 280 * DAY,
    last_seen: NOW - 4 * MIN,
    sessions: 142,
    total_laps: 3180,
    best_drift: 14820,
    best_lap_ms: 0,
    podiums: 0,
    favourite_car: cars.ae86,
    favourite_track: tracks.gunma,
    drift_trend: [6200, 7400, 6900, 9100, 8800, 10250, 9700, 11900, 13100, 14820],
    last_result: driftRun("s-9001", 4 * MIN, tracks.gunma, cars.ae86, 14820, 14820),
    results: [
      driftRun("s-9001", 4 * MIN, tracks.gunma, cars.ae86, 14820, 14820),
      driftRun("s-9000", 5 * HOUR, tracks.ebisu, cars.ae86, 11900, 14820),
      driftRun("s-8990", DAY, tracks.iro, cars.s15, 13100, 13100),
      driftRun("s-8970", 2 * DAY, tracks.gunma, cars.ae86, 10250, 13100),
      driftRun("s-8950", 4 * DAY, tracks.ebisu, cars.ae86, 9100, 13100),
    ],
    media: spikeMedia("kaz", "Gunma Cycle Sports Center"),
    stream: { embed_url: "", status: "offline" },
  },
  {
    guid: "76561198022334455",
    name: "Lena Vogt",
    online: false,
    first_seen: NOW - 210 * DAY,
    last_seen: NOW - 6 * HOUR,
    sessions: 98,
    total_laps: 2740,
    best_drift: 1820,
    best_lap_ms: 121340,
    podiums: 31,
    favourite_car: cars.gt3p,
    favourite_track: tracks.spa,
    drift_trend: [900, 1100, 1450, 1200, 1600, 1500, 1700, 1750, 1800, 1820],
    last_result: raceRun("s-8881", 6 * HOUR, tracks.spa, cars.gt3p, 1, 18, 121340, 24),
    results: [
      raceRun("s-8881", 6 * HOUR, tracks.spa, cars.gt3p, 1, 18, 121340, 24),
      raceRun("s-8860", 2 * DAY, tracks.nords, cars.gt3p, 2, 16, 392010, 8),
      raceRun("s-8840", 3 * DAY, tracks.brands, cars.gt3f, 1, 14, 84720, 30),
      raceRun("s-8800", 6 * DAY, tracks.spa, cars.gt3p, 3, 20, 122110, 22),
      raceRun("s-8770", 9 * DAY, tracks.brands, cars.gt3p, 1, 15, 84990, 28),
    ],
    media: [],
    stream: { embed_url: "", status: "not_configured" },
  },
  {
    guid: "76561198033445566",
    name: "Sam \"Slideways\" Park",
    online: true,
    first_seen: NOW - 160 * DAY,
    last_seen: NOW - MIN,
    sessions: 117,
    total_laps: 2510,
    best_drift: 12640,
    best_lap_ms: 0,
    podiums: 0,
    favourite_car: cars.s15,
    favourite_track: tracks.ebisu,
    drift_trend: [5400, 6100, 7200, 6800, 8900, 9400, 10100, 11200, 11800, 12640],
    last_result: driftRun("s-8702", MIN, tracks.ebisu, cars.s15, 12640, 12640),
    results: [
      driftRun("s-8702", MIN, tracks.ebisu, cars.s15, 12640, 12640),
      driftRun("s-8690", 8 * HOUR, tracks.gunma, cars.s15, 11200, 12640),
      driftRun("s-8650", 2 * DAY, tracks.iro, cars.rx7, 10100, 12640),
      driftRun("s-8600", 5 * DAY, tracks.ebisu, cars.s15, 9400, 11800),
    ],
    media: spikeMedia("sam", "Ebisu Minami"),
    stream: { embed_url: "", status: "offline" },
  },
  {
    guid: "76561198044556677",
    name: "Marco Rossi",
    online: false,
    first_seen: NOW - 320 * DAY,
    last_seen: NOW - DAY,
    sessions: 205,
    total_laps: 5120,
    best_drift: 980,
    best_lap_ms: 83640,
    podiums: 24,
    favourite_car: cars.gt3f,
    favourite_track: tracks.brands,
    drift_trend: [400, 600, 520, 700, 650, 800, 760, 900, 880, 980],
    last_result: raceRun("s-8551", DAY, tracks.brands, cars.gt3f, 2, 17, 83640, 30),
    results: [
      raceRun("s-8551", DAY, tracks.brands, cars.gt3f, 2, 17, 83640, 30),
      raceRun("s-8520", 3 * DAY, tracks.spa, cars.gt3f, 4, 19, 121980, 20),
      raceRun("s-8490", 5 * DAY, tracks.brands, cars.gt3a, 1, 16, 83910, 28),
      raceRun("s-8450", 8 * DAY, tracks.nords, cars.gt3f, 3, 14, 393440, 6),
    ],
    media: [],
    stream: { embed_url: "", status: "not_configured" },
  },
  {
    guid: "76561198055667788",
    name: "Yuki Tanaka",
    online: false,
    first_seen: NOW - 95 * DAY,
    last_seen: NOW - 3 * HOUR,
    sessions: 64,
    total_laps: 1490,
    best_drift: 9310,
    best_lap_ms: 84880,
    podiums: 7,
    favourite_car: cars.r34,
    favourite_track: tracks.iro,
    drift_trend: [3200, 4100, 3900, 5200, 6100, 5800, 7200, 8100, 8900, 9310],
    last_result: driftRun("s-8401", 3 * HOUR, tracks.iro, cars.r34, 9310, 9310),
    results: [
      driftRun("s-8401", 3 * HOUR, tracks.iro, cars.r34, 9310, 9310),
      raceRun("s-8380", 2 * DAY, tracks.brands, cars.gt3a, 5, 18, 84880, 26),
      driftRun("s-8350", 4 * DAY, tracks.gunma, cars.r34, 8100, 9310),
      raceRun("s-8300", 7 * DAY, tracks.spa, cars.gt3a, 8, 20, 123640, 18),
    ],
    media: [],
    stream: { embed_url: "", status: "not_configured" },
  },
  {
    guid: "76561198066778899",
    name: "Ava Lindqvist",
    online: false,
    first_seen: NOW - 48 * DAY,
    last_seen: NOW - 12 * HOUR,
    sessions: 39,
    total_laps: 910,
    best_drift: 1240,
    best_lap_ms: 84020,
    podiums: 5,
    favourite_car: cars.gt3a,
    favourite_track: tracks.nords,
    drift_trend: [500, 700, 650, 900, 850, 1000, 1100, 1050, 1200, 1240],
    last_result: raceRun("s-8201", 12 * HOUR, tracks.nords, cars.gt3a, 3, 15, 391200, 7),
    results: [
      raceRun("s-8201", 12 * HOUR, tracks.nords, cars.gt3a, 3, 15, 391200, 7),
      raceRun("s-8180", 4 * DAY, tracks.brands, cars.gt3a, 6, 16, 84020, 24),
      raceRun("s-8150", 9 * DAY, tracks.spa, cars.gt3f, 4, 18, 122480, 20),
    ],
    media: [],
    stream: { embed_url: "", status: "not_configured" },
  },
  {
    guid: "76561198077889900",
    name: "Diego Santos",
    online: false,
    first_seen: NOW - 132 * DAY,
    last_seen: NOW - 2 * DAY,
    sessions: 88,
    total_laps: 1980,
    best_drift: 10870,
    best_lap_ms: 0,
    podiums: 0,
    favourite_car: cars.rx7,
    favourite_track: tracks.gunma,
    drift_trend: [4200, 5100, 4900, 6200, 7100, 6800, 8200, 9100, 9900, 10870],
    last_result: driftRun("s-8101", 2 * DAY, tracks.gunma, cars.rx7, 10870, 10870),
    results: [
      driftRun("s-8101", 2 * DAY, tracks.gunma, cars.rx7, 10870, 10870),
      driftRun("s-8080", 5 * DAY, tracks.ebisu, cars.rx7, 9100, 10870),
      driftRun("s-8050", 11 * DAY, tracks.iro, cars.e30, 8200, 10870),
    ],
    media: [],
    stream: { embed_url: "", status: "not_configured" },
  },
  {
    guid: "76561198088990011",
    name: "Priya Nair",
    online: false,
    first_seen: NOW - 27 * DAY,
    last_seen: NOW - 5 * DAY,
    sessions: 21,
    total_laps: 470,
    best_drift: 760,
    best_lap_ms: 85510,
    podiums: 2,
    favourite_car: cars.e30,
    favourite_track: tracks.brands,
    drift_trend: [200, 350, 300, 450, 500, 600, 650, 700, 720, 760],
    last_result: raceRun("s-8001", 5 * DAY, tracks.brands, cars.e30, 7, 16, 85510, 22),
    results: [
      raceRun("s-8001", 5 * DAY, tracks.brands, cars.e30, 7, 16, 85510, 22),
      raceRun("s-7980", 12 * DAY, tracks.spa, cars.gt3a, 9, 18, 124900, 16),
    ],
    media: [],
    stream: { embed_url: "", status: "not_configured" },
  },
];
