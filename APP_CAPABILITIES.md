# Server Manager — Capabilities & Data Reference

A reference for a UX designer. It describes **what the application can do**, **what data is available**, and **what the backend exposes** — deliberately without describing how the current interface looks or is laid out. Use it to design a fresh UX from the underlying capabilities up.

---

## 1. What the application is

Server Manager (SM) is a web-based control panel for running **Assetto Corsa** dedicated racing servers. Assetto Corsa is a PC racing simulator; people host multiplayer servers that others join to race on. Configuring such a server by hand means editing INI files and managing a folder of car/track "content". SM replaces that with a single web application.

A user of SM is a **server operator or race organizer**, not a driver. They use SM to:

- Point SM at an Assetto Corsa installation and import its cars, tracks and weather.
- Build reusable **presets** for the rules of a race (difficulty, sessions, time/weather, car classes).
- Compose those presets into **race setups (events)**, grouped into **categories**.
- **Queue** events and **start/stop** the actual game server, which then appears in the public server browser for drivers to join.
- **Monitor** a running server live — who is connected, lap times, car positions, telemetry health.
- **Control** a running race — send chat, force the next session, restart, kick players, run admin commands.
- Run **multiple independent server instances** at once (different ports/queues), optionally on a schedule or auto-repeating one event.
- Manage **users** of SM itself (admin / steward / viewer roles).
- Offer **mod downloads** and **livestream embeds** so spectators and joining players have what they need.

It is a single binary serving a JSON REST API plus a real-time event stream. Everything below is reachable through that API.

---

## 2. Core domain concepts (the mental model)

These are the nouns the whole product is built from. A good UX hinges on making these relationships legible.

| Concept | What it is |
|---------|-----------|
| **Content** | The raw assets imported from an Assetto Corsa install: **cars**, **tracks** (each with one or more layout *configs*), and **weather** types. Read-mostly catalog. |
| **Difficulty preset** | A named bundle of realism/assist rules (ABS, traction control, tyre wear, fuel use, damage, penalties, voting/kick rules, etc.). |
| **Session preset** | A named bundle defining which session phases run and how long: Booking, Practice, Qualify, Race — plus race specifics (extra lap, pit window, reversed grid, wait times). |
| **Time/Weather preset** | A named time-of-day + one or more weather panels (graphics, ambient/road temp + variation, wind, CSP time/date). |
| **Car class preset** | A named set of car entries: which car + skin, how many of each, optional ballast. Defines the grid. |
| **Event (race setup)** | One race definition = a track/layout + one of each preset (difficulty, session, time, class) + race laps/strategy + optional name. The central composed object. |
| **Category (event group)** | A named folder grouping related events. Can be duplicated wholesale. |
| **Queue (server_event)** | The ordered list of events waiting to run on a given instance. Rotates as sessions finish. |
| **Instance** | One runnable server process with its own ports, queue, run mode, schedule, streaming and spectator config. There is always at least one (the default). |
| **Run mode** | An instance is either in **manual queue** (advance through queued events) or **repeat event** (re-run one chosen event forever). |
| **User** | An SM operator account with a role: **admin** (everything), **steward** (operate running servers/queues), **viewer** (read-only). |
| **Driver** | A person connected to a *running* game server. Not an SM account — derived live from telemetry. |

Key relationships: an **event** references exactly one of each preset and one track; **categories** contain events; presets are **shared** across many events (editing one ripples to all that use it — usage counts are exposed for warnings); the **queue** holds events scheduled to run on an **instance**.

---

## 3. Capability areas

Each area lists what the operator can do and the concrete API/data behind it.

### 3.1 Content library (cars, tracks, weather)

What the operator can do: browse the imported catalog; inspect details and imagery; import more content (file upload or URL, sync or background job with progress); rebuild the cache after changing the AC install; delete individual items.

- `GET /api/cars`, `GET /api/tracks`, `GET /api/weathers` — full catalog lists.
- `GET /api/car/:key` — single car incl. **power & torque curves** (chartable arrays) and description.
- Imagery endpoints (return images directly):
  - `GET /api/car/image/:car/:skin` — car livery/preview.
  - `GET /api/track/preview/:track[/:config]` — track photo.
  - `GET /api/track/outline/:track[/:config]` — track outline.
  - `GET /api/track/map/:track[/:config]` — track map image, plus `GET /api/track/mapmeta/...` returning map geometry (width/height/offsets/scale) for overlaying live car positions.
  - `GET /api/weather/preview/:weather`.
- `POST /api/content/upload` — upload an archive or supply `archive_url`; URL imports run as background jobs.
- `GET /api/content/jobs/active`, `GET /api/content/jobs/:id` — import job status: phase (downloading/extracting/recaching/completed/failed), progress %, bytes downloaded, files written, imported asset keys, resulting totals.
- `POST /api/content/recache` — re-scan the install; returns track/car/weather counts.
- `DELETE /api/track/:key`, `DELETE /api/car/:key`, `DELETE /api/weather/:key` — remove one item (disk + cache).

Data available per item:
- **Car**: key, name, brand, class, tags, description, specs (bhp, torque, weight, top speed, acceleration, power-to-weight, fuel range), power/torque curves, list of skins (key + name).
- **Track**: key, layout config, name, description, tags, country, city, length, width, pitbox count, run type.
- **Weather**: key, name.

### 3.2 Presets (the reusable rule bundles)

For **each** of the four preset families — difficulties, sessions, times, classes — the same CRUD shape exists:

- `GET /api/<family>` — list (optionally only "filled"/usable ones via `?filled=1`).
- `POST /api/<family>` — create by name.
- `GET /api/<family-singular>/:id` — full detail.
- `PUT /api/<family-singular>/:id` — update all fields.
- `DELETE /api/<family-singular>/:id`.

Families & routes: `difficulties/difficulty`, `sessions/session`, `times/time`, `classes/class`.

- `GET /api/presets/usage` — how many events reference each preset, per family, keyed by preset id. Powers "used by N events" and edit warnings.

Notable field sets the UI must expose:
- **Difficulty**: assists (ABS/TC/stability/autoclutch/tyre blankets), virtual mirror, fuel rate, damage multiplier, tyre wear rate, allowed tyres out, max ballast, start rule, gas penalty toggle, dynamic track + preset, session start grip, randomness, session grip transfer, lap gain, kick quorum, vote duration, voting quorum, blacklist mode, max contacts per km.
- **Session**: per-phase enabled flag + duration for Booking/Practice/Qualify/Race; practice/qualify/race "is open"; qualify max wait %; race extra lap, over-time, wait time, reversed-grid positions, pit window start/end.
- **Time/Weather**: time string, time-of-day multiplier, CSP enabled, plus an ordered list of **weather panels** (graphics key, base ambient/road temp, ambient/road variation, wind min/max speed, wind base + variation direction, CSP time/date/multiplier).
- **Class**: ordered list of entries (car key, skin key, ballast, count). The sum of counts is the requested grid size.

### 3.3 Race setups (events) and categories

- Categories: `GET /api/categories`, `POST /api/categories`, `GET /api/category/:id` (with nested events), `PUT /api/category/:id`, `PATCH /api/category/:id` (rename only), `POST /api/category/:id/duplicate`, `DELETE /api/category/:id`.
- Events: `GET /api/events`, `POST /api/events`, `GET /api/event/:id`, `PUT /api/event/:id`, `DELETE /api/event/:id`.
- An event payload = category, name, track key + layout config, the four preset ids, race laps, strategy.

**Preview before committing** — a critical capability for the UX:
- `POST /api/server/render-preview` — render the actual `server_cfg.ini` + `entry_list.ini` for a *saved or unsaved/draft* event, returning: rendered grid count, requested grid size, track pitbox capacity, max clients, and human-readable **warnings/errors** (e.g. "grid exceeds pitboxes — trimmed", "grid exceeds max clients", "no cars in grid"). Nothing is persisted.
- `GET /api/server/entry_list.ini`, `GET /api/server/server_cfg.ini` — render the raw INI for a given event id/instance (inspect/debug).

### 3.4 Queue & running the server

The queue is per-instance and ordered; running rotates through it (or repeats one event).

- `GET /api/queue?instance=N` — queue rows with display names and, where computable, an **estimated duration in minutes** per event (sum of timed sessions), started-at, finished flag.
- `POST /api/queue/event/:id?instance=N` — add one event; `POST /api/queue/category/:id?instance=N` — add a whole category.
- Reordering: `POST /api/queue/moveup/:id`, `POST /api/queue/movedown/:id`, `PUT /api/queue/order` (full ordered id list).
- `POST /api/queue/skipevent?instance=N` — jump to next event on a live server.
- `POST /api/queue/clearcompleted`, `DELETE /api/queue/:id`.
- (Queue mutations are blocked while an instance is in repeat mode — surfaced as a conflict the UI should explain.)

Lifecycle controls:
- `POST /api/server/start` / `POST /api/server/stop` (per instance) — start blocks briefly until the telemetry plugin is confirmed online.
- `GET /api/server/status?instance=N` — the big live status object (see §4).

### 3.5 Live race control (only while running)

All target a running instance over the game's UDP plugin:

- `POST /api/server/broadcast` — send a chat message to everyone.
- `POST /api/server/next-session` — advance to the next session phase.
- `POST /api/server/restart-session` — restart the current session.
- `POST /api/server/kick` — kick a driver by car id.
- `POST /api/server/admin-command` — run an arbitrary admin command string.
- `POST /api/server/current-event` — **live-edit the running event**: change track, car class & entries, time, weather, CSP time/multiplier — then optionally restart immediately; response indicates whether a restart is required vs. was performed.

### 3.6 Server instances (multi-server)

- `GET /api/instances` — every instance with live state: ports (udp/tcp/http/plugin/plugin-listen), running?, player count, run mode, repeat event (with track/category/class), scheduled start, start-on-boot, streaming + spectator config.
- `POST /api/instances`, `PUT /api/instances/:id`, `DELETE /api/instances/:id` (can't edit/delete while running; can't delete the last one; port-conflict validated).
- `PUT /api/instances/:id/runmode` — switch manual-queue ↔ repeat-event.
- `PUT /api/instances/:id/schedule` — set/clear a one-shot scheduled start (unix timestamp); a scheduler starts the instance at that time.
- Per instance: **start-on-boot** auto-starts from its queue when SM launches.

### 3.7 Setup & readiness (first-run / health)

- `GET /api/server/readiness` — facts for a setup checklist: install path set?, acServer binary found?, base config saved?, mod config filled?, content counts, preset counts per family, event count, per-instance port/queue/running state, any port conflict.
- `GET /api/setup/summary` — everything a guided first-run needs in one call: the above plus a config snapshot (name, has-password, has-admin-password, engine, max clients, lobby registration), usable preset lists, event groups, suggested next-free ports for a new instance, and a normalized list of **blocking issues** each keyed to the setup step that fixes it, plus a `can_start` boolean.
- `POST /api/validate/installpath` — check a candidate AC install path contains the server binary.

### 3.8 Configuration

- `GET /api/config`, `PUT /api/config` — base server config: lobby name, password, admin password, register-to-lobby, locked entry list, result screen time, ports, client send interval, threads, max clients, welcome message, append-event-name, append-mod-links, mod download URL, **server engine** (stock *Kunos* vs. *AssettoServer*), relax-checksums, auto-start.
- `PUT /api/config/content` — the content/CSP half: install path and CSP flags (required, version, physics cars/tracks, hide pit).
- `GET /api/server/engine` — selected engine + whether the AssettoServer binary is installed; `POST /api/server/assettoserver/install` — download/extract it.

### 3.9 Mods, downloads & streaming

- **Public (unauthenticated) downloads** so joining players can fetch missing mods: `GET /dl/car/:key`, `GET /dl/track/:key`.
- **Per-instance livestream embed**: enable + embed URL + optional status URL; `GET /api/instances/:id/stream/status` reports live/offline/not-configured/unknown.
- **Spectator car**: an instance can reserve a spectator slot (driver name, GUID, car, skin).
- **Per-driver streams**: map a driver GUID → display name + embed/status URL. Full CRUD at `/api/driver-streams[...]`; `GET /api/instances/:id/driver-streams/status` returns stream health for currently-connected drivers, so a viewer UI can show "who is live".

### 3.10 SM user management & account

- `GET /api/users`, `POST /api/users`, `PUT /api/users/:name/role`, `PUT /api/users/:name/password`, `DELETE /api/users/:name` (admin only; can't demote/delete the last admin or delete self).
- `GET /api/user`, `PUT /api/user` — current account (password change, measurement unit, temp unit preferences).
- `POST /api/login`, `POST /api/logout` (cookie + CSRF).
- Roles: **admin** = full; **steward** = operate running servers and queues; **viewer** = read-only. The UX should reflect role-gated actions.

### 3.11 Maintenance & diagnostics

- `GET /api/about` — version, config & temp folder paths.
- `GET /api/server/logfile` — download the SM log.
- `GET /api/server/smdata` — download the SQLite database (backup).
- `GET /api/server/smcontent` — download the packaged content archive.
- `POST /api/maintenance/restore` — restore a database backup (staged, applied on next start).

---

## 4. Live data (real-time)

There is a single **Server-Sent Events** stream: `GET /api/server/events`. On connect it sends a snapshot per instance, then pushes deltas; it heartbeats every 15s. Event types and their payloads:

| Event type | Payload |
|------------|---------|
| `snapshot` | Per-instance initial state: running, players, session, drivers, positions, telemetry. |
| `server` | `running` true/false. |
| `players` | Current connected player count. |
| `session` | Session info (see below). |
| `drivers` | Full driver roster. |
| `positions` | All car positions (throttled to ~5/s). |
| `telemetry` | Telemetry-link health. |
| `content_job` | Content import job progress (not instance-specific). |

The same data is also pollable via `GET /api/server/status`. Its shape:

- **is_running**, **players** (count), **public_ip**, log text tail, working-dir + cfg/entry paths.
- **session**: name, type (Booking/Practice/Qualify/Race) + id, session index / current index / count, track + config, server name, time, laps, wait time, ambient & road temp, weather graphics, elapsed ms.
- **current_event**: the event currently applied — id, name, category, track (+ key/config), difficulty, session, class (+id), time (+id), weather (+key), started-at, finished.
- **drivers[]**: per car — car id, driver name, car model, skin, GUID, laps completed, last lap (ms), best lap (ms), connected flag. (Powers live timing and kick-by-car-id.)
- **positions[]**: per car — x/y/z, velocity x/y/z, gear, engine RPM, normalized spline position (0–1 around the lap), updated-at. (Combine with track map meta for a live map.)
- **telemetry**: udp-online flag, last-packet / last-driver / last-position timestamps, plugin ports. Lets the UI distinguish "running but no telemetry" (misconfigured) from "running, nobody connected yet".
- **current_cars[]**: the cars actually in the rendered entry list (model + skin + count).

---

## 5. Cross-cutting facts worth designing around

- **Composition with sharing.** Presets are reused across many events; the app exposes usage counts so the UX can warn that editing a preset affects every event using it.
- **Preview-before-run is first-class.** The backend can render the exact server files for any draft and report grid size vs. pitbox/max-client limits with plain-language warnings — ideal for a confidence step before queueing.
- **Two engines.** Stock *Kunos* server vs. *AssettoServer* (a drop-in with extra features like installing missing content for joining players). The choice affects available behavior.
- **CSP (Custom Shaders Patch).** Some content/weather requires CSP; there are CSP flags and CSP-specific time/weather fields. The app handles the plumbing, but the UX must let operators set CSP options and understand CSP-required content.
- **Multi-instance everywhere.** Almost every server/queue/control endpoint takes an `instance` parameter. Status, queues, run modes, schedules, streaming and players are all per-instance.
- **Roles gate actions.** Viewer (read-only), steward (operate/queue), admin (everything). Action availability should adapt to role.
- **Live editing.** A running race can be reconfigured on the fly (track/class/time/weather) with an explicit "needs restart vs. restarted" outcome.
- **Background work.** Content imports and the like run asynchronously with progress; the UX needs a place to surface job progress and completion.
- **Setup is a guided, checklist-shaped problem.** The readiness/summary endpoints already model first-run as discrete, individually-fixable blocking issues keyed to steps.
- **Units are a user preference.** Each account stores measurement and temperature unit preferences; values like temps, lengths and speeds should respect them.
