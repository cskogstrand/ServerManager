# Server Manager — Application Guide

Last updated: 2026-06-11 (post `ux-rewrite` cutover)

This document explains what Server Manager does, how every page works, the
mechanics that are easy to miss, and a prioritized list of usability
improvements. Companion document: `REFACTOR_PLAN.md` (migration history and
architecture decisions).

---

## 1. What the app is

Server Manager (SM) is a control panel for **Assetto Corsa dedicated racing
servers**. It wraps Kunos' `acServer` binary — which is configured purely
through two INI files and has no management interface of its own — in a web
UI that handles everything around it:

- importing and cataloguing game content (tracks, cars, weather),
- composing race configurations out of reusable presets,
- generating `server_cfg.ini` and `entry_list.ini`,
- starting/stopping the acServer OS process (one per *instance*),
- listening to the server's UDP plugin telemetry (sessions, players),
- rotating through a queue of events automatically.

It ships as a **single Go binary** with the entire frontend (a Vue 3 SPA),
database schema, and INI templates embedded. State lives in one SQLite file.
The web UI runs on `http://localhost:3030`; each managed game server uses its
own set of game ports (UDP/TCP 9600+, HTTP 8081+ by default).

Deployment options: native binary (Linux/Windows) or Docker
(`docker-compose.yml` for production behind traefik, `docker-compose.dev.yml`
for development). In Docker, the AC installation mounts at `/corsa` and
persistent data at `/appdata`.

### The mental model

```
Content (tracks/cars/weather)          imported once, cached in SQLite
        │
Presets (difficulty/sessions/time/class)   reusable building blocks
        │
Event  = track + class + sessions + time + difficulty (+ laps, strategy)
        │      grouped into Categories
Queue  = ordered list of events, per server instance
        │
Instance = one acServer process with its own ports + work dir
        │      start → render INIs → extract content → spawn process
        └── UDP plugin feeds live status; when the last session of an event
            ends, SM kicks players, marks the event finished, loads the next
            queued event and restarts — until the queue is empty.
```

---

## 2. Core concepts in detail

### Content caches
SM never reads the AC install at race time. Instead, **Rebuild cache** (or a
content upload) parses the install/archives and stores metadata in SQLite
(`cache_track`, `cache_car`, `cache_weather`): names, tags, pitboxes, track
length, car specs/torque curves, skin lists. Actual files used by a running
server are pulled from `smcontent.zip` (built during import, kept next to the
database) and extracted **per event** into the instance's work directory —
only the cars and track that event needs.

### Presets
Four independent preset types, each with full CRUD:

| Preset | Maps to | Notes |
|---|---|---|
| **Difficulty** | assists, realism, dynamic track, voting | `ABS/TC`: denied/factory/forced |
| **Sessions** | booking/practice/qualify/race blocks | each session individually toggleable |
| **Time & Weather** | sun angle + `WEATHER_n` sections | multiple weather panels; CSP mode unlocks 24h time + dates |
| **Car Class** | `entry_list.ini` | list of car+skin entries, each with a **count** (one row → N grid slots) |

A preset only becomes selectable in the event builder once it has been saved
at least once (`filled` flag) — a freshly created, never-saved preset would
render a broken config.

### Events & categories
An **event** references one track (key + layout config) and one preset of
each type, plus `race_laps` (0 = time-based race) and an **overflow
strategy**. A **category** groups events (e.g. "GT3 Championship") — you can
queue a single event or a whole category at once. Deleting presets that an
event still references is blocked (FK) and surfaces as "still in use".

### The queue & rotation
`server_event` rows are queue entries, ordered by `orderby`, scoped per
instance (`instance_id`). Lifecycle of an entry: *pending* → *active*
(`started_at` set) → *finished*. When acServer reports the end of the last
session (`ACSP_END_SESSION` with `currentSessionIndex == sessionCount-1`), SM:

1. kicks all players (only reliable way to signal a track change),
2. marks the entry finished,
3. takes the next unfinished entry for that instance,
4. re-renders configs, re-extracts content, restarts the process.

Empty queue → server stops. "Skip event" triggers the same rotation manually.
The rendered `server_cfg.ini`/`entry_list.ini` are also snapshotted into the
queue row for posterity.

### Instances (multi-server)
Each `server_instance` row is an independent acServer: its own lobby name,
game ports (UDP/TCP/HTTP), plugin port pair, queue, process, log buffer, and
UDP listener. Instance 1 keeps the historical `tmp/` work dir; instance N
runs from `tmp/instance_N/`. Port collisions are validated at create/edit;
edits and deletes are blocked while that instance runs. Docker port mappings
cover 9601-9609 and 8082-8090 for extra instances out of the box.

### Render pipeline (the heart)
`ConfigRenderer.renderIni` assembles everything:

- class entries are **expanded by their count** *before* capping, so caps act
  on real grid slots;
- `MAX_CLIENTS` = min(config max, track pitboxes, expanded entries);
- if entries exceed the cap, the **overflow strategy** applies (keep first N
  in list order, or random shuffle then cut);
- time-of-day becomes a `SUN_ANGLE` (vanilla AC only supports 08:00–18:00);
- CSP weather encodes time/multiplier/date into the weather *graphics name*
  (`3_clear_time=50400_mult=1`), and the track path gets a `csp/<version>/..`
  prefix plus letter-coded feature folders (B–H encode combinations of
  extended physics and hidden pits) that SM materializes as folder aliases.

### Live data (SSE)
One `GET /api/server/events` stream replaces all polling. Event types:
`snapshot` (on connect, per instance), `server` (running flipped), `players`,
`session` (full session info on change), `content_job` (import progress).
15-second heartbeat; slow clients drop messages rather than block the game
loop. The SPA holds it all in one Pinia store.

### Auth & security
Single admin user (default `admin`/`admin` — change it). JWT in an HttpOnly
`SameSite=Lax` cookie, 30-day expiry; passwords bcrypt-hashed. All mutating
API calls require a double-submit CSRF token (`csrf_token` cookie mirrored
into the `X-CSRF-Token` header — the SPA client does this automatically).
Server password / admin password for the *game* server are separate and live
in Configuration.

---

## 3. Page-by-page

### Login (`/login`)
Username + password → JSON login. On success the original target route is
restored. Sign-out lives at the bottom of the sidebar.

### Dashboard (`/`)
One card per instance:
- **Header**: status dot, lobby name, TCP port, live player count;
  Skip event (when one is active) and Start/Stop.
- **Current event**: track preview image, category, chips for class/sessions/
  time/weather. When idle, links to the queue.
- **Session**: type (Practice/Qualify/Race) with progress `n/total`, length
  (laps or minutes), elapsed clock, air/road temps, weather graphics id.
- **Grid**: entry list grouped by car model with counts (`ferrari_488 ×4`).
- **Console**: collapsible acServer output (last 200 lines, refreshes every
  5 s only while open).
- Public IP (polled every 5 min from api.ipify.org) shows in the header.

Start with an empty queue does nothing — queue first. The first-run banner
(shown until Configuration and the install path are saved) links to setup.

### Events (`/events`)
Categories in the left rail (create/delete inline, rename in the editor).
Events render as cards: track preview, preset chips, laps/timed badge.
Actions per card: **Queue** (one click, default instance), **Edit**, delete.

**The Builder** (side sheet): tap the track block to open the **track
picker** (searchable preview grid, layout configs listed separately, pitbox
counts visible); dropdowns for class/sessions/time/difficulty (only
completed presets appear); race laps (0 = use the session's race *time*);
overflow strategy. Saving updates the card grid immediately.

### Queue (`/queue`)
Instance tabs (with live status dots) when more than one instance exists —
each instance has a fully independent queue.
- Table: position, track + category, class, preset summary; state icons
  ▶ active / ✓ done / numbered pending.
- Pending rows: move up/down, remove. Active row: **Skip** (confirm; kicks
  players). **Clear completed** purges finished rows.
- Right card: add a single event (category → event) or *all* events of a
  category, to the selected instance.
- Start/Stop for the selected instance in the header. The table refetches
  automatically when SSE reports a start/stop/rotation.

### Content (`/content`)
- **Library**: tabs for tracks/cars/weathers with counts, search, preview
  thumbnails (tracks: preview image + layout + pitboxes; cars: first-skin
  shot + brand + skin count).
- **Installation**: AC install path with a **Check** button (validates that
  `<path>/server/acServer` exists), CSP requirement toggle with minimum
  build number and feature toggles (extended car/track physics, hide pits).
  Must be saved before anything can be imported or started.
- **Upload**: kind (track/car), archive file (zip/7z/rar ≤ 2 GB) *or*
  download URL, overwrite toggle. Upload shows a real progress bar; the
  import job then streams progress/phases over SSE in the **Import jobs**
  list (jobs are kept ~10 minutes after finishing). A completed import
  auto-refreshes the library.
- **Rebuild cache** re-scans everything (also what you run after pointing at
  a fresh install).

### Car Classes (`/presets/classes`)
Entry cards: car dropdown, skin dropdown + random-skin dice, live preview
image, **Number of cars** (grid slots; the header shows the total), reorder
arrows, remove. The grid-slot total is what counts against pitboxes.

### Difficulty (`/presets/difficulty`)
Identity / Assists / Realism / Dynamic track / Voting & blacklist cards.
Dynamic-track fields only appear when enabled. `Max collisions per km` of
-1 disables the limit.

### Sessions (`/presets/sessions`)
One card per session type, fields gated by the enable toggle. Qualify's "lap
completion limit" is the 120 %-rule (slower drivers may finish a started
lap). Race extras: overtime, wait time, reversed-grid count, pit window,
extra lap, join-during-session.

### Time & Weather (`/presets/time`)
Vanilla mode: one time of day (08:00–18:00) + multiplier. **CSP mode** moves
time onto each weather panel (full 24 h, multiplier up to 60, optional fixed
date — for special lighting/seasons). Weather panels are add/remove cards
with graphics, temperatures (road temp is a *differential* relative to
ambient), variations, and wind. Multiple panels = AC picks between them.

### Configuration (`/settings`)
The global server config (`user_config`): lobby identity (name, welcome
message, register-to-lobby, append event name / mod links), access (server
password, admin password, locked entry list), engine numbers (max clients,
result screen time, send rate, threads), auto-start-queue-on-launch.
**Ports are not here** — each instance owns its ports.

### Instances (`/settings/instances`)
Card per instance with all five ports. Create suggests the next free port
block; edit/delete disabled while running; the last instance cannot be
deleted. Port-collision errors come straight from the API.

### Preferences (`/preferences`)
Per-user units (km/h vs mph, °C vs °F) and password change (bcrypt-hashed;
old form-based flow used to store plaintext — fixed).

### About (`/about`)
Version, config + temp folder paths, downloads: `logfile.log`, `smdata.db`
(the database), `smcontent.zip` (the content bundle) — i.e. a manual backup
of everything that matters.

---

## 4. Nooks & crannies

Things that aren't obvious from the UI but matter:

- **`filled` flags**: presets/categories appear in builder dropdowns only
  after the first save; `cfg_filled`/`mod_filled` on the config row drive the
  first-run banner.
- **Count expansion vs. strategy**: a class with `5× CarA, 5× CarB` on a
  track with 8 pitboxes renders 8 slots — "First" keeps 5×A + 3×B, "Random"
  shuffles the expanded list first. `CARS=` in `server_cfg.ini` lists each
  expanded slot.
- **Config previews** (`/api/server/server_cfg.ini?id=N`,
  `entry_list.ini?id=N`) render with a throwaway renderer — they can't
  disturb a running server, but random strategy makes them nondeterministic.
- **Editing a running event** (`POST /api/server/current-event`) can rewrite
  the class entry list; it currently saves *expanded* entries, so counts in
  that class collapse to 1× each (the grid stays identical).
- **Track-change kick**: there is no graceful "track changing" message in
  vanilla AC — kicking everyone is the documented workaround.
- **Plugin ports never leave the machine**: `UDP_PLUGIN_LOCAL_PORT` (acServer
  side) and the SM listen port are loopback-only; only game UDP/TCP/HTTP need
  firewall/NAT/compose mappings.
- **Per-instance work dirs**: binary + config + per-event content are copied/
  extracted per instance; nothing is shared at runtime, so two instances can
  run different tracks from the same library.
- **Legacy `",string"` JSON quirks**: a few GET payloads (time weathers,
  category events) still serialize numbers as quoted strings; the SPA
  normalizes on load and posts through clean DTOs. Don't "fix" the tags
  without checking the dashboard current-event flow.
- **Logs**: everything is mirrored to stdout, `logfile.log` *and* an
  in-memory buffer; per-instance acServer output is captured separately and
  served in the dashboard console.
- **SQLite locking**: one writer at a time — don't run external `sqlite3`
  writes against a live server.
- **`-debug` flag**: serves the built SPA, schema, static assets, and INI
  templates from `src/embed` on disk instead of the embedded copies; it also
  skips opening the browser. This is what the dev Docker container runs.
- **Auto-start**: with the toggle on, the binary starts the default
  instance's queue at boot — combined with Docker `restart: unless-stopped`
  that gives unattended recovery.
- **Bookmarks**: pre-cutover `/app/...` URLs 301-redirect to the new roots.

---

## 5. Usability improvements (prioritized)

### Quick wins (high value, low effort)
1. **Toasts instead of inline notice rows.** Success/error currently renders
   as a banner at the top of each page — easy to miss after scrolling. A
   global toast stack (the Pinia store is already there) gives consistent
   feedback everywhere, including SSE-driven events ("Import finished",
   "Event rotated").
2. **Unsaved-changes guards.** Preset forms lose edits on navigation. Track
   a dirty flag per form (compare against the loaded snapshot) and use a
   route-leave confirm. The store-level pattern was already planned.
3. **Confirm dialogs in-app.** `window.confirm` works but looks foreign;
   reuse the `Modal` component for delete/skip confirmations with the
   consequence spelled out ("removes 3 queue entries").
4. **Empty states with calls to action.** "No presets yet" should offer the
   create action right there, not just an input in the corner; the dashboard
   idle card should offer "Queue an event" as a button.
5. **Loading skeletons.** Pages currently pop in when data arrives; skeleton
   cards (dashboard, library grids) remove the flash.
6. **Form validation before submit.** Required selects in the builder
   validate on save only; inline "required" markers and disabling the save
   button until valid is cheaper than the error roundtrip.
7. **Searchable dropdowns.** Car lists get long; the class editor's car
   select should be a combobox with search (the track picker already is).

### Structural improvements (medium effort)
8. **Inline preset creation in the Builder.** The original plan's promise:
   "create new class" from inside the event builder, opening the preset
   editor in a nested sheet and returning with it selected. The preset pages
   are already componentized around `usePresetPage` — extract the form bodies
   from the pages so the builder can host them.
9. **Drag-and-drop queue reorder** (and class entry reorder) with optimistic
   updates — replace the ↑/↓ buttons; one `PUT /api/queue/order` accepting
   the full id order would also remove the N-clicks-N-requests pattern.
10. **Event duplication & templates.** "Duplicate event", "duplicate
    category", and "save event as template" — most championships are
    variations of one setup.
11. **Onboarding wizard.** Replace the first-run banner with a 3-step
    wizard: install path → import/cache content → create first event. Each
    step exists; it's a guided wrapper.
12. **Queue ETA + scheduling.** Show estimated start time per queue entry
    (sum of session durations ahead of it); allow "start queue at 19:00"
    (server-side timer) for league nights.
13. **Grid editor on the dashboard.** Re-add the old UI's ability to tweak
    the *running* event (swap weather, adjust car list) from the dashboard
    with the restart prompt — the endpoint exists; while at it, make it
    count-aware so it stops collapsing class counts.
14. **Results & history.** acServer writes results JSON per session into the
    work dir; parse them into a `results` table and add a History page
    (winners, lap times, incidents per event). The queue rows already
    snapshot their configs — linking results to them gives full provenance.

### Bigger bets (high effort, high payoff)
15. **Live race control.** The UDP plugin already supports kick, broadcast
    chat, next/restart session, and admin commands — none are exposed. A
    race-control panel on the dashboard (driver list with ping/kick,
    broadcast box, next-session button) turns SM into a real stewarding
    tool. Driver join/leave events are already on the SSE stream.
16. **Real-time lap/position widget.** `ACSP_LAP_COMPLETED` and car updates
    arrive on the plugin socket; a minimal live timing table (position, last
    lap, gaps) on the dashboard would be the killer feature for league use.
17. **Mobile-first pass.** The layout is responsive but desktop-shaped;
    a bottom-tab navigation under 640 px, larger touch targets on queue
    actions, and sheets-as-default for pickers would finish the "one
    responsive UI" promise.
18. **Multi-user & roles.** The `users` table exists; add user management
    (admin vs. race-steward vs. read-only) — relevant once race control
    exists. Aud claim is already in the JWT.
19. **Backup/restore in-app.** About already serves the three files; add
    "download full backup" (zip of db + content) and an upload-restore flow
    with a confirmation diff ("this replaces 12 presets, 3 instances").
20. **Health panel.** Port reachability self-test (is 9600/udp actually
    reachable from outside? lobby registration succeeded?), disk space for
    the content zip, CSP version detection from the install — most "server
    not visible" support questions become self-service.

### Consistency debt to keep an eye on
- Legacy endpoints still answer in `{"success":false,...}` while new ones use
  the error envelope — converge on the envelope, then simplify the SPA client.
- `GET` payloads with quoted numbers (the `",string"` tags) should get clean
  DTO twins like the time-update endpoint, then the SPA normalizers can go.
- Native `go:embed` now owns schema, INI templates, favicon, and the SPA
  production bundle. If Go tooling fails after `make clean`, run
  `make webapp` first so `src/embed/webapp/dist` exists.
