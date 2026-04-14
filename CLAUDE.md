# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

**Server Manager (SM)** is a web-based control panel for managing Assetto Corsa dedicated racing servers. It is a single Go binary with all assets (HTML templates, CSS, images) embedded at build time. The web UI runs on `http://localhost:3030`.

## Commands

All build and dev commands go through `make`:

| Command | Description |
|---------|-------------|
| `make deps` | Install npm and Go dependencies |
| `make run` | Dev mode: regenerate assets, then `go run .` from `src/` |
| `make build` | Build Linux binary to `bin/sm_linux` |
| `make buildwin` | Cross-compile Windows binary to `bin/sm_win.exe` |
| `make all` | Clean full build for both platforms |
| `make package` | Production package (buildwin + build) |

**No test suite exists.** There are no lint or test make targets.

The build pipeline always runs before Go compilation:
1. Compile TailwindCSS (`css/input.css` → `css/main.css`)
2. Embed assets via `go-assets-builder` (generates `src/embedded.go`)
3. Compile Go binary with version ldflags

## Architecture

### Entry Points
- `src/main.go` — Gin router setup, middleware (JWT cookie auth), graceful shutdown, public IP polling goroutine
- `src/routes.go` — HTML page handlers (GET/POST returning full pages)
- `src/api.go` — REST API handlers under `/api/*` (JSON responses, JWT-authenticated)

### Key Packages / Files
| File | Responsibility |
|------|---------------|
| `src/server.go` | `ServerStatus` struct — server state, start/stop logic, track change on queue rotation, CSP alias management |
| `src/acprocess.go` | Spawns/kills the AC server OS process |
| `src/configrenderer.go` | Renders `server_cfg.ini` and `entry_list.ini` from user presets using Go text/templates |
| `src/contentparser.go` | Parses AC content directory (tracks, cars, weather JSONs) |
| `src/dbaccess.go` | All SQLite CRUD — caches (track/car/weather), user configs, presets, event queue |
| `src/dbmodels.go` | Struct definitions for DB rows |
| `src/udpplugin.go` | UDP listener goroutine receiving telemetry/events from AC process |
| `src/zipfile.go` | Archive extraction for mod management |

### Frontend
Templates in `htm/` are loaded from the embedded filesystem. Pages use **Alpine.js** for reactivity and **TailwindCSS** for styling. **Chart.js** is used for metrics visualizations on the status page. There is no build step for JS — it loads from CDN in dev and should be embedded or CDN in production.

### Database
Embedded SQLite (`schema.sql` defines the schema). Two categories of tables:
- **Caches**: `track`, `car`, `weather` (parsed from AC content directory)
- **User data**: `users`, `user_config`, plus preset tables for difficulty, session, time/weather, car classes, event categories, event queue (`server_event`)

### Server Lifecycle (Queue → Running)
1. User queues an Event (POST `/queue`) → `server_event` table
2. `GET /api/server/start` → fetches first queued event
3. `ConfigRenderer` assembles presets into INI files
4. `acprocess.go` spawns AC binary
5. `udpplugin.go` goroutine receives AC UDP events
6. On session end → dequeue next event → loop; on queue empty → stop

### CSP (Custom Shaders Patch) Support
When an event requires CSP, the app creates filesystem symlinks/aliases for track folders (`cspTrackBase()`, `ensureCspTrackAliases()` in `server.go`). This is handled transparently.

### Asset Embedding
`go-assets-builder` embeds all `htm/`, `css/`, and static files into `src/embedded.go` during build. In dev (`make run`), assets are re-embedded before `go run`. Never edit `src/embedded.go` manually.

## Stack

- **Go 1.23.2** (Gin web framework, golang-jwt/v5, bcrypt, SQLite3, archiver)
- **Alpine.js** — frontend reactivity
- **TailwindCSS 4** — utility CSS (compiled via npm/npx)
- **SQLite** — embedded DB, no external database
- **go-assets-builder** — embeds assets into binary
- **go-winres** — Windows binary resources for cross-compilation

## Platform Notes

Config/data directory resolution: `XDG_CONFIG_HOME` on Linux, `APPDATA` on Windows. Temp files use `/tmp` vs `TEMP` env var. Cross-compilation for Windows uses `x86_64-w64-mingw32` toolchain (`CGO_ENABLED=1` required for SQLite).
