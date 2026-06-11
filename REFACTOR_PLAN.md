# Server Manager — Frontend Rewrite & Backend Refactor Plan

Date: 2026-06-11 (updated same day: Vue 3 confirmed; multi-server + car-count features added and implemented)
Scope: Replace the Go-template + Alpine.js frontend with a TypeScript SPA (Vue 3), complete the JSON API in Go, and redesign the UX around the actual workflow (build event → queue → run). The Go backend stays.

## Status

- **DONE (Phase 0 head start, on branch `ux-rewrite`)**: multi-server instances (backend + API + minimal old-UI hooks), car count per class entry (backend + old class editor UI). See sections 2a/2b.
- **NEXT**: remaining Phase 0 items (JSON CRUD for presets/config/users, CSRF, SSE), then Phase 1 scaffold.

---

## 1. Framework Decision

### Keep Go for the backend — do NOT port to TypeScript/Node

The backend's hard parts are exactly the things Go is best at and Node is worst at:

| Concern | Where | Why Go wins |
|---|---|---|
| Spawning/killing the `acServer` OS process | `src/acprocess.go` | `exec.Command` + cross-platform binary selection already works on Win/Linux |
| UDP binary protocol listener (ACSP telemetry) | `src/udpplugin.go` (451 LOC) | Byte-level parsing of a binary game protocol; goroutine model fits |
| Single-binary deployment with embedded assets | `src/embedded.go`, Makefile | One file to ship; Node equivalents (pkg/bun build) are heavier and flakier, especially with native SQLite |
| SQLite via CGO, cross-compiled to Windows | `src/dbaccess.go`, mingw toolchain | Already solved; would need re-solving with better-sqlite3 + electron-builder-style packaging |

A TS backend rewrite would cost weeks and the end state would be operationally worse. **TypeScript goes where it actually pays off: the frontend**, which is currently ~1,900 LOC of untyped, untestable inline JavaScript.

### Frontend: rewrite as a TypeScript SPA (don't refactor in place)

Evidence from the scan that in-place refactor ≈ 70% of a rewrite anyway:

- **Mobile is a forked UI, not responsive CSS**: `mobile.htm` (362) + `mobile_cars.htm` (424) + `mobile_track.htm` (286) + `mobile_weather.htm` (244) = 1,316 LOC duplicating desktop logic (~90% overlap with `server.htm`'s dashboard object).
- **~1,900 LOC of inline Alpine.js** across 16 `x-data` objects, no separate JS files, no types, no tests possible.
- **Hand-rolled state management**: snapshot/dirty-tracking via `JSON.stringify` in `server.htm:141-212`, custom DOM events as a state bus (`sm-content-counts` in `content.htm:26`), three independent 1000ms polling loops.
- **Modal markup copy-pasted 8+ times** across class/time/event/session/difficulty/server/mobile templates.
- **CSS already SPA-ready**: `extra.css` (3,584 LOC) is a self-contained design system (`--c-*` tokens, `pw-*` components) that bypasses Tailwind — it ports cleanly.

### Recommended stack

| Layer | Choice | Rationale |
|---|---|---|
| Build | **Vite** | Fast, trivial dev proxy to `:3030`, static `dist/` output for embedding |
| Framework | **Vue 3 + `<script setup>` + TS** | Closest mental model to Alpine (`x-data`→`ref/reactive`, `x-show`→`v-show`, `@click` identical) — fastest port of existing logic. React + TanStack Query + Zustand is a fine swap if preferred; nothing below depends on the choice. |
| Server state | **TanStack Query (vue-query)** | Kills the hand-rolled fetch/merge/poll logic; caching, invalidation, optimistic updates for free |
| Client state | **Pinia** | Dirty tracking, modal cascade, queue editing |
| Styling | **Tailwind 4 with `@theme` tokens ported from `extra.css`** | Keep the existing slate design system (per established design direction: `#0d1117` bg, Plus Jakarta Sans, soft blue accent, 8px radius); delete the 44 `!important` overrides |
| Go↔TS types | **tygo** (generate TS interfaces from `src/dbmodels.go` structs) | One source of truth; regenerated in `make run` |
| Realtime | **SSE endpoint from Go** | Replaces all three 1s polling loops; UDP listener already produces the events |

Project layout: new `webapp/` directory in this repo (not a separate project — shares Makefile, schema, release pipeline). Go serves `webapp/dist` embedded; API stays at `/api/*`.

---

## 2a. Multi-server hosting — IMPLEMENTED

Run several acServer processes side by side on different ports, each with its own queue.

- **Schema**: new `server_instance` table (name + udp/tcp/http ports + plugin port pair); `server_event.instance_id` scopes the queue per instance. The default instance (id 1) is seeded from the ports historically stored in `user_config`, so existing installs migrate automatically.
- **Runtime**: `src/instance.go` — `Instance` bundles the OS process handle, captured log output, UDP plugin connection, status and config renderer (all formerly package globals). `InstanceManager` syncs runtime instances with the DB and owns lifecycle (UDP listener goroutine per instance, rebind on port change, teardown on delete).
- **Isolation**: instance 1 keeps the historical `tmp/` work dir; instance N runs from `tmp/instance_N/` with its own `cfg/`, extracted content and binary copy. `server_cfg.ini` ports and plugin addresses are templated per instance.
- **API**: `GET/POST /api/instances`, `PUT/DELETE /api/instances/:id` (port-collision validation; edits/deletes refused while running; last instance protected). All existing `/api/server/*` and `/api/queue/*` endpoints accept `?instance=N` and default to the lowest-id instance, so the old UI keeps working unchanged. New `POST /api/queue/event/:id` and `/api/queue/category/:id`.
- **Old UI**: queue page gets an instance selector when more than one instance exists. Full instance management UI (create/edit instances, per-instance dashboard tabs, per-instance queue views) is **Vue work — added to Phases 4 and 5**.
- **Side fixes**: per-instance mutex around process handle/status (was an unguarded global written by the UDP goroutine); config previews (`/api/server/server_cfg.ini`, `entry_list.ini`) now render with a throwaway renderer instead of mutating live state; public-IP poller no longer dereferences a nil response on error.

## 2b. Car count per class entry — IMPLEMENTED

One class-entry row can produce N grid slots ("5× this car/skin") instead of duplicating the car in the list.

- **Schema**: `user_class_entry.car_count` (default 1, `ensureColumn` migration).
- **Render**: entries are expanded count-times in `ConfigRenderer.renderIni` *before* the pitbox/max-clients capping and strategy shuffle, so caps apply to real grid slots.
- **UI**: "Number of cars" input on each entry card in the class editor; count round-trips through the existing JSON form field. The dashboard's car-list modal still submits expanded single entries (collapsing a class edited there back to counts is Vue-phase work).

## 2. Backend Refactor (prerequisite, Go)

The SPA can only be as good as the API. Currently ~60% of functionality is HTML-form-only.

### Phase 0 — API completion
1. **Add JSON CRUD endpoints** for everything that is form-POST-only today: user, config, difficulty, class, session, time, event, event category (handlers in `src/routes.go` contain the logic; extract into shared functions called by both old routes and new `/api/*` handlers so old UI keeps working during migration).
2. **Fix GET-with-side-effects**: queue moveup/movedown/skip/clear are GET endpoints (`src/api.go`) — convert to POST/PUT/DELETE.
3. **Consistent error envelope**: `{"error": {"code", "message"}}` with correct status codes; replace the generic `routeDbError` 500 (`src/routes.go:1024`).
4. **SSE stream** `GET /api/server/events`: push server status, session changes, content-job progress. Source: `udpplugin.go` events + `ContentJobs`.
5. **Auth for SPA**: keep JWT HttpOnly cookie (works fine for same-origin SPA). Add CSRF protection for mutating endpoints (double-submit cookie) — currently absent.

### Phase 0.5 — structural cleanup (do opportunistically while touching handlers)
- Extract shared "load tracks/cars/weathers + demo data" helper (currently duplicated ~12× in `routes.go:221-280`).
- Mutex around `ServerStatus` (written by UDP goroutine, read by handlers, currently unsynchronized) and around the process handle in `acprocess.go`.
- Wrap multi-step operations (`applyServerEvent` in `api.go:372-429`, content upload) in SQLite transactions.
- Optional, not blocking: service-layer extraction from the 1,615-LOC `dbaccess.go`. Defer unless it blocks API work.
- Replace `go-assets-builder` with native `go:embed` (available since Go 1.16) — removes a build dependency and the generated 187-LOC `assets.go`.

---

## 3. UX Redesign

### Core problem
The app is organized around **database tables**, not the user's task. To run a race you visit five preset pages (difficulty, session, time/weather, class, category), then assemble them on the event page, then queue, then start. Every CRUD action is a full page reload. Mobile is a separate, second app.

### Design principles
1. **Event-centric**: the primary object is "an event I want to run." Everything else is supporting material reachable from that flow.
2. **One responsive UI** — delete the mobile fork.
3. **Live, not polled-and-reloaded**: SSE-driven status, optimistic updates, no `window.location` reloads.
4. **Inline creation**: never force navigation away from a flow to create a dependency.

### New information architecture

```
┌─ Dashboard (default view)
│   ├─ Server status card (live via SSE: session, players, time remaining)
│   ├─ Queue (drag-to-reorder, inline skip/remove, "up next" preview)
│   └─ Start/Stop with confirm
├─ Events
│   ├─ Event list (cards with track/car/weather summary chips)
│   └─ Event Builder (wizard or single scrollable form):
│        track picker → car classes → sessions → time/weather → difficulty
│        Each step shows existing presets as selectable cards
│        + "create new" inline (drawer/modal) without leaving the builder
├─ Content (tracks/cars/weather library, upload with progress via SSE)
├─ Presets (power-user direct access to difficulty/session/time/class —
│   same components the builder uses, just standalone)
└─ Settings (config, users, admin)
```

### Concrete UX fixes mapped to current pain

| Current pain | Fix |
|---|---|
| 5 separate preset pages to set up one event | Event Builder with inline preset selection/creation |
| Queue reorder = GET request + full page reload (`queue.htm:45-65`) | Drag-and-drop with optimistic reorder, PUT to API |
| 1000ms polling ×3, status flickers, modal-open guards (`server.htm:103-116`) | Single SSE subscription in a Pinia store |
| Unsaved-changes snapshot hack (`server.htm:141-212`) | Form-level dirty state from the store; route-leave guards |
| Mobile = separate templates | Responsive layout: sidebar collapses to bottom nav, pickers become full-screen sheets on small viewports |
| Track/car pickers reimplemented 2× (desktop modal + mobile page) | One `<ContentPicker>` component, responsive presentation |
| Upload via raw XMLHttpRequest, jobs polled (`content.htm:286-332`) | Upload component with progress events; job status over SSE |

---

## 4. Migration Phases

Each phase ships independently; old UI keeps working until Phase 6.

**Phase 0 — Backend API completion** (section 2). ~Go-only, no UI risk.

**Phase 1 — SPA scaffold**
- `webapp/` with Vite + Vue 3 + TS + Tailwind 4 + Pinia + vue-query.
- Port `extra.css` tokens into Tailwind `@theme`; build `Button`, `Card`, `Modal`, `FormRow`, `Toggle`, `Sheet` components reproducing the `pw-*` design system.
- tygo wired into Makefile; typed `apiClient` wrapper (fetch + error envelope + CSRF header).
- Dev: Vite proxy → `:3030`. Prod: Makefile builds `webapp/dist`, Go embeds and serves it at `/` (old templates move to `/legacy/*` during migration, or SPA mounts at `/app` — pick one; recommend SPA at `/app` until Phase 6 flip).

**Phase 2 — Simple pages** (proves the pipeline end-to-end)
- Login, initial config, about, admin, user settings.

**Phase 3 — Presets & content**
- Preset editors (difficulty, session, time/weather, class) as reusable components — built *as components first* because the Event Builder reuses them.
- Content library + upload with SSE job progress.

**Phase 4 — Event Builder + Queue** (the UX centerpiece)
- Event Builder wizard with inline preset creation.
- Queue with drag-reorder, optimistic updates, **per-instance queue assignment and filtering**.

**Phase 5 — Dashboard**
- Live status via SSE, start/stop, session/player info, Chart.js (or replace with lighter sparklines) — port of `server.htm`, hardest page, done last when all components exist.
- **Multi-server dashboard**: one card/tab per instance (status, players, current event, start/stop each), plus instance management (create/edit/delete instances, port settings) in Settings.

**Phase 6 — Cutover & deletion**
- SPA moves to `/`; delete `htm/` (5,776 LOC), `extra.css` overrides, mobile templates, `routes.go` page handlers (~1,000 LOC), Alpine/Chart.js CDN references.
- Swap `go-assets-builder` → `go:embed`.

### Rough effort (sessions ≈ focused work blocks)

| Phase | Estimate |
|---|---|
| 0 — API completion | 2–3 |
| 1 — Scaffold + design system | 2 |
| 2 — Simple pages | 1 |
| 3 — Presets & content | 3–4 |
| 4 — Event Builder + queue | 3 |
| 5 — Dashboard | 2 |
| 6 — Cutover | 1 |
| **Total** | **~14–16** |

---

## 5. Risks & mitigations

- **CSRF gap goes live with the SPA** — Phase 0 item 5 must land before any mutating SPA page ships.
- **`ServerStatus` data races** exist today; SSE fan-out will read it more often — fix the mutex in Phase 0.5 before Phase 5.
- **Demo mode** (`isDemoRequest`, `withDemoTracks/Cars/Weathers` in `routes.go:34-153`) is implemented in the HTML layer — the API endpoints need the same demo-data injection or demo mode silently breaks in the SPA.
- **Scope creep in the Event Builder** — build it with existing preset semantics first; don't redesign the preset data model in the same phase.
- **No test suite exists** — add Vitest from Phase 1 (component + store tests), and Go httptest coverage for every endpoint touched in Phase 0. The rewrite is the cheapest moment to gain tests.
