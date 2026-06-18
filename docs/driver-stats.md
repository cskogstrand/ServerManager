# Driver Stats — backend

## Status

- ✅ **Persistence + aggregation API — built** (`src/driverstats.go`, schema in
  `src/embed/schema.sql`, hooks in `src/drivers.go` + `src/udpplugin.go`, routes in
  `src/main.go`, steward permission in `src/roles.go`). Endpoints:
  `GET /api/drivers`, `GET /api/drivers/:guid`,
  `GET|POST /api/drivers/:guid/avatar`, `GET /api/drivers/:guid/media/:file`.
  Not yet compiled here (no Go toolchain) — build/test in Docker.
- ✅ **Drift-spike auto-capture worker — built** (`src/drivercapture.go`). On a live
  drift score crossing `captureTriggerScore`, `driverDrift` submits one capture;
  the worker pulls a screenshot + short clip via ffmpeg from the driver's
  `driver_stream.stream_capture_url` (new column) and files them as `driver_media`.
  Throttled by: one capture/run, per-session cap, per-driver cooldown, global
  concurrency + per-minute ceiling, and top-by-score + most-recent retention.
  No-op when ffmpeg is absent or no capture URL is set. Reel renders real
  `<img>`/`<video>`; capture URL editable under Instances → Driver streams.
  Not compiled here (no Go toolchain) — build/test in Docker; ffmpeg must be on
  PATH in the container.

The list (`/drivers`) and detail (`/drivers/:guid`) pages were built
**frontend-first** against this contract, served by mock data only when the API
answers `404/501`. See:

- Contract types: `webapp/src/types/driverStats.ts`
- Data access + mock fallback: `webapp/src/lib/driversApi.ts` (falls back to mock
  only on `404/501`, so the pages light up automatically once the API answers)
- Pages: `webapp/src/pages/DriverStats.vue`, `webapp/src/pages/DriverDetail.vue`

Today everything driver-related is **in-memory and session-scoped** — `DriverState`
in `src/drivers.go` (name, guid, laps, lap times, `DriftLive/Last/Best`) is wiped
on every session reset and never persisted. Only `driver_stream` (guid → external
stream embed URL) survives. The work below adds the persistence + aggregation the
pages need, plus auto-capturing stream highlights on big drift spikes.

## 1. Persistence schema (add to `src/embed/schema.sql`)

```sql
CREATE TABLE IF NOT EXISTS driver (
  guid        TEXT PRIMARY KEY,
  name        TEXT NOT NULL,        -- last-seen display name
  first_seen  INTEGER NOT NULL,     -- epoch ms
  last_seen   INTEGER NOT NULL,
  avatar_path TEXT                  -- uploaded photo, NULL = use monogram avatar
);

CREATE TABLE IF NOT EXISTS driver_session (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  driver_guid   TEXT NOT NULL REFERENCES driver(guid),
  instance_id   INTEGER NOT NULL,
  server_event_id INTEGER,          -- the queued event, if any
  session_type  INTEGER NOT NULL,   -- AC: 0 booking,1 practice,2 qualify,3 race
  car_key       TEXT,
  skin_key      TEXT,
  track_key     TEXT,
  track_config  TEXT,
  joined_at     INTEGER NOT NULL,
  left_at       INTEGER,
  laps          INTEGER NOT NULL DEFAULT 0,
  best_lap_ms   INTEGER NOT NULL DEFAULT 0,
  finish_pos    INTEGER,            -- race only
  entrants      INTEGER,
  drift_best    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS driver_lap (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id    INTEGER NOT NULL REFERENCES driver_session(id),
  lap_number    INTEGER NOT NULL,
  laptime_ms    INTEGER NOT NULL,
  cuts          INTEGER NOT NULL DEFAULT 0,
  recorded_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS driver_drift_run (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id    INTEGER NOT NULL REFERENCES driver_session(id),
  score         INTEGER NOT NULL,   -- final score of the run (DriftLast)
  peak          INTEGER NOT NULL,   -- highest live value seen during the run
  ended_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS driver_media (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  driver_guid   TEXT NOT NULL REFERENCES driver(guid),
  session_id    INTEGER REFERENCES driver_session(id),
  kind          TEXT NOT NULL,      -- 'screenshot' | 'clip'
  path          TEXT NOT NULL,      -- on-disk file under the media dir
  caption       TEXT,
  captured_at   INTEGER NOT NULL,
  duration_s    INTEGER,            -- clips only
  trigger_score INTEGER,            -- drift score at the trigger
  trigger_delta INTEGER             -- jump that tripped the trigger
);
CREATE INDEX IF NOT EXISTS idx_driver_session_guid ON driver_session(driver_guid);
CREATE INDEX IF NOT EXISTS idx_driver_media_guid   ON driver_media(driver_guid);
```

## 2. Event hooks (in `src/drivers.go`, fed by `src/udpplugin.go`)

Each handler keeps doing its in-memory work, then writes through to SQLite
(`src/dbaccess.go`). Keep a live `driver_session.id` on each `DriverState` so laps
and drift runs attach to the right session row.

| Hook | Persist |
|------|---------|
| `driverJoin` | `UPSERT driver` (name, last_seen; first_seen if new) + open a `driver_session` row |
| `driverLap` | `INSERT driver_lap`; update `driver_session` laps/best_lap_ms |
| `driverDrift` (final, `live=false`) | `INSERT driver_drift_run` (score + tracked peak); bump `driver_session.drift_best` and `driver.best`-derived stats |
| `driverLeave` | set `driver_session.left_at` |
| session end / `resetDriverLaps` | finalize open sessions (set `finish_pos`/`entrants` for races from running order) |

## 3. Aggregation API (add handlers in `src/api.go`, register in `src/main.go`)

Match the JSON shapes in `webapp/src/types/driverStats.ts` exactly.

- `GET /api/drivers` → `{ drivers: DriverSummary[] }`
  - one row per `driver`; aggregate sessions/laps, `MAX(drift_best)`, `MIN(best_lap_ms>0)`,
    podium count (`finish_pos<=3` in races), favourite car/track (most-frequent
    `car_key`/`track_key` over `driver_session`), `last_result` (latest session →
    drift score or lap+pos), `drift_trend` (last ~10 `driver_drift_run.score`).
- `GET /api/drivers/:guid` → `DriverDetail` (summary + `results[]` newest-first +
  `media[]` + `stream`). Join `driver_stream` for the embed URL; reuse `streamHealth()`
  (already in `api.go`) for `stream.status`.
- `POST /api/drivers/:guid/avatar` (operate role) → multipart image; store under the
  media dir, set `driver.avatar_path`; return `{ avatar_url }`. The detail page already
  has the upload affordance wired to a local preview — point it here.
- `GET /api/drivers/:guid/media/:file` → serve a stored capture (auth-gated static).

## 4. Drift-spike auto-capture (screenshots + clips)

> Goal: grab a still + a short clip from a driver's stream when they land a **big**
> drift spike, with enough smarts that it captures the *moment*, not an endless reel.

### Capture source
Server-side capture needs a raw, server-reachable stream (HLS/RTMP/SRT) — platform
embeds (YouTube/Twitch iframes) can't be grabbed. Add an optional
`stream_capture_url` to `driver_stream`; capture is available only when it's set,
otherwise the spike is still logged but no media is produced.

Run a per-enabled-stream **rolling buffer** with `ffmpeg` (continuous segmenting,
e.g. keep the last ~30s) so a clip can include the *lead-up* to the spike, not just
the aftermath. On trigger: cut a clip (`-ss peak-8s -t ~15s`) and grab a frame
(`-frames:v 1`) into the media dir; `INSERT driver_media`.

### Trigger logic — peak-hold, not per-tick
Drift scores arrive as a stream of `live=` updates then a final `last=` (see
`parseDriftChat`/`driverDrift`). Capturing on every rising `live` tick would spam.
Instead **debounce to the run's peak**:

1. Track the rising `live` score per driver. Arm a candidate when `live` first
   crosses `MIN_SCORE` (floor, e.g. 2,000) **and** the jump since arming exceeds
   `MIN_DELTA` (e.g. +1,500).
2. Hold while the score keeps climbing. Fire **once** when the run settles — either
   the final `last=` arrives, or `live` stops increasing for `SETTLE_MS` (~1.5s) —
   capturing that run's **peak**.
3. After firing, **re-arm only** once the score drops below `peak × 0.6`
   (hysteresis), so one big run = one capture.

### Throttling (the "don't capture endlessly" guards)
- **Per-driver cooldown**: ≥ `COOLDOWN_MS` (e.g. 45s) between captures, hard floor.
- **Per-session cap**: ≤ `MAX_PER_SESSION` (e.g. 12) captures per driver per session.
- **Global limits**: a bounded worker pool for `ffmpeg` jobs + a server-wide
  captures-per-minute ceiling, so a busy lobby can't thrash disk/CPU.
- **Keep-the-best**: when the session cap is hit, replace the lowest-scoring stored
  capture only if the new spike beats it (so the reel trends toward personal bests).
- **Retention**: cap on-disk media per driver (e.g. keep top N by score + most
  recent M); prune oldest/lowest beyond that.

All thresholds belong in instance config (alongside `drift_score_enabled` /
`allow_wrong_way` in `src/dbmodels.go` + `schema.sql`) so an operator can tune or
disable capture per server.
