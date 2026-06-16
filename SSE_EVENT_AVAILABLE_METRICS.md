# SSE Event API

**SSE endpoint:** `GET /api/server/events` — `src/events.go:94`

**Envelope:**

```json
{
  "type": "...",
  "instance_id": 0,
  "ts": "...",
  "data": {}
}
```

## Driver Metrics

### Per-driver roster

`DriverState` — `src/drivers.go:11-21`

Emitted in:

- `drivers` events
- `snapshot` events

| Field | Type | Meaning |
|---|---:|---|
| `car_id` | `int` | Session car ID |
| `name` | `string` | Driver name |
| `car` | `string` | Car model |
| `skin` | `string` | Livery |
| `guid` | `string` | Driver GUID |
| `laps` | `int` | Completed laps |
| `last_lap_ms` | `uint32` | Last lap time |
| `best_lap_ms` | `uint32` | Best lap time |
| `connected` | `bool` | Connected now |

### Per-driver position / telemetry

`CarPositionState` — `src/positions.go:8-20`

Emitted in:

- `positions` events
- `snapshot` events

Frequency: ≤5Hz

| Field | Type | Meaning |
|---|---:|---|
| `car_id` | `int` | Session car ID |
| `x`, `y`, `z` | `float32` | World position, in meters |
| `velocity_x`, `velocity_y`, `velocity_z` | `float32` | Velocity vector, in m/s |
| `gear` | `int` | `0` = reverse, `1` = neutral, `2+` = forward |
| `engine_rpm` | `int` | Engine RPM |
| `normalized_spline_pos` | `float32` | Track position, `0..1` |
| `updated_at` | `int64` | Unix milliseconds |

## Session / Environment

### Session info

`SessionInfo` — emitted in `session` events

Source references:

- `src/udpplugin.go:373`
- `src/udpplugin.go:387`

Fields:

| Field | Meaning |
|---|---|
| `name` | Session name |
| `type` | Integer session type |
| `index` | Session index |
| `current_session_index` | Current session index |
| `session_count` | Total session count |
| `track` | Track name |
| `track_config` | Track configuration |
| `server_name` | Server name |
| `time` | Seconds remaining |
| `laps` | Lap count |
| `wait_time` | Wait time, in seconds |
| `ambient_temp` | Ambient temperature, in °C |
| `road_temp` | Road temperature, in °C |
| `weather_graphics` | Weather graphics identifier |
| `elapsed_ms` | Elapsed time, as `int32` milliseconds |

## Server / Players

### `server` event

| Field | Type | Meaning |
|---|---:|---|
| `running` | `bool` | Whether the server is running |

### `players` event

| Field | Type | Meaning |
|---|---:|---|
| `players` | `int` | Current player count |

### `snapshot` event

Bundles:

- `running`
- `players`
- `session`
- `drivers[]`
- `positions[]`
- `telemetry`

## Infrastructure / Plugin Health

### Telemetry snapshot

`TelemetrySnapshot` — `src/instance.go:46-53`

Emitted in:

- `telemetry` events
- `snapshot` events

| Field | Type | Meaning |
|---|---:|---|
| `udp_online` | `bool` | Whether UDP telemetry is online |
| `last_packet_ms` | `int64` | Unix ms timestamp of latest packet freshness |
| `last_driver_ms` | `int64` | Unix ms timestamp of latest driver freshness |
| `last_position_ms` | `int64` | Unix ms timestamp of latest position freshness |
| `plugin_listen_port` | `int` | Plugin listen port |
| `plugin_send_port` | `int` | Plugin send port |

## Content Jobs

Non-instance events use `instance_id = 0`.

### Content job

`ContentJob` — `src/contentjobs.go:14-34`

Emitted in `content_job` events.

| Field | Meaning |
|---|---|
| `id` | Job ID |
| `kind` | Job kind |
| `source` | Content source |
| `source_name` | Source display name |
| `source_url` | Source URL |
| `status` | Job status |
| `phase` | Current phase |
| `message` | Human-readable status message |
| `progress` | Progress, `0-100` |
| `downloaded_bytes` | Bytes downloaded |
| `download_total_bytes` | Total bytes to download |
| `files_written` | Number of files written |
| `imported_assets[]` | Imported asset list |
| `tracks_total` | Total tracks imported |
| `cars_total` | Total cars imported |
| `weathers_total` | Total weather assets imported |
| `started_at` | Job start timestamp |
| `updated_at` | Last update timestamp |
| `finished_at` | Job finish timestamp |

---

## Gaps Worth Noting

Currently unavailable:

- Sectors
- Live position / gap rank
- Tyre data
- Fuel data
- Temperature data
- Throttle, brake, or steering inputs

Derived values:

- **Speed** can be derived from the magnitude of `velocity_x`, `velocity_y`, and `velocity_z`.
- **Race order** can be derived by sorting on `laps + normalized_spline_pos`.
