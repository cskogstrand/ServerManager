# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working Style

Always use the **ponytail** skill/plugin when working in this repo: prefer the laziest solution that works — stdlib and native platform features before dependencies, one line before fifty, deletion over addition, no speculative abstractions.

## What This Project Is

**Server Manager (SM)** is a web-based control panel for managing Assetto Corsa dedicated racing servers. It is a single Go binary with the Vue SPA, schema, INI templates, and static assets embedded at build time. The web UI runs on `http://localhost:3030`.

## Environment Note (read first)

The **Go toolchain is NOT installed in the Claude Code environment.** Do not search the filesystem for a `go` binary and do not try to run `go`, `make build`, `make run`, or `make test` locally — they will fail. The user builds and runs the app in Docker. To verify Go changes, ask the user to run the build/tests in their Docker environment. Frontend (`npm`/Vite) IS available, so `cd webapp && npm run build` can be used to type-check and build the SPA.

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
| `make test` | Build SPA, run Vitest, then run Go vet |

The build pipeline always runs before Go compilation:
1. Build the Vue SPA with Vite into `src/embed/webapp/dist`
2. Embed assets with native Go `embed` from `src/embed`
3. Compile Go binary with version ldflags

## Architecture

### Entry Points
- `src/main.go` — Gin router setup, middleware (JWT cookie auth), graceful shutdown, public IP polling goroutine
- `src/routes.go` — SPA fallback and legacy `/app/*` redirect handlers
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
The frontend lives in `webapp/` and is a Vue 3 + TypeScript SPA built by Vite. Production assets are generated into `src/embed/webapp/dist` and served by the Go binary. In local UI development, use `make webapp-dev` for Vite and `make rundebug` for the Go API/static backend.

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
Native Go `embed` embeds `src/embed/schema.sql`, `src/embed/ini`, `src/embed/favicon.ico`, and the generated `src/embed/webapp/dist` tree. `make webapp`, `make run`, `make build`, and `make test` rebuild the SPA before Go commands that need embedded UI assets.

## Stack

- **Go 1.23.2** (Gin web framework, golang-jwt/v5, bcrypt, SQLite3, archiver)
- **Vue 3 + TypeScript** — frontend SPA
- **TailwindCSS 4** — utility CSS (compiled by Vite)
- **SQLite** — embedded DB, no external database
- **go-winres** — Windows binary resources for cross-compilation

## Platform Notes

Config/data directory resolution: `XDG_CONFIG_HOME` on Linux, `APPDATA` on Windows. Temp files use `/tmp` vs `TEMP` env var. Cross-compilation for Windows uses `x86_64-w64-mingw32` toolchain (`CGO_ENABLED=1` required for SQLite).
