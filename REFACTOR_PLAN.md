# Server Manager — Frontend Rewrite & Backend Refactor Plan

Last updated: 2026-06-11 · Branch: `ux-rewrite`
Scope: Replace the Go-template + Alpine.js frontend with a TypeScript SPA (Vue 3), complete the JSON API in Go, and redesign the UX around the actual workflow (build event → queue → run). The Go backend stays.

Legend: `[x]` done · `[ ]` open

---

## 1. Framework Decision — SETTLED

- [x] **Backend stays Go.** The hard parts are what Go is best at and Node is worst at:
  - Spawning/killing the `acServer` OS process (`src/acprocess.go`)
  - UDP binary protocol listener, ACSP telemetry (`src/udpplugin.go`)
  - Single-binary deployment with embedded assets
  - SQLite via CGO, cross-compiled to Windows
- [x] **Frontend: rewrite as TypeScript SPA, not in-place refactor.** Evidence: mobile is a forked UI (1,316 LOC duplicating desktop), ~1,900 LOC inline untyped Alpine JS, hand-rolled state (JSON.stringify dirty-tracking, 3× 1s polling loops), modal markup copy-pasted 8+ times. In-place cleanup ≈ 70% of a rewrite with none of the type safety.
- [x] **Stack confirmed**: Vite + **Vue 3** `<script setup>` + TS, TanStack Query (server state), Pinia (client state), Tailwind 4 with tokens ported from `extra.css` (keep slate dark theme, Plus Jakarta Sans, soft blue, 8px radius), tygo for Go→TS types, SSE for realtime. New `webapp/` dir in this repo; Go serves embedded `webapp/dist`.

---

## 2a. Multi-server hosting — IMPLEMENTED ✅

Run several acServer processes side by side on different ports, each with its own queue.

- [x] Schema: `server_instance` table (name, udp/tcp/http ports, plugin port pair); `server_event.instance_id` scopes the queue. Default instance seeded from legacy `user_config` ports — existing installs migrate automatically.
- [x] Runtime: `src/instance.go` — `Instance` bundles process handle, log buffer, UDP plugin connection, status and config renderer (all formerly package globals). `InstanceManager` syncs runtime set with DB, owns lifecycle (UDP goroutine per instance, rebind on port change, teardown on delete).
- [x] Isolation: instance 1 keeps historical `tmp/`; instance N runs from `tmp/instance_N/` with own cfg + content + binary copy. `server_cfg.ini` ports/plugin addresses templated per instance.
- [x] API: `GET/POST /api/instances`, `PUT/DELETE /api/instances/:id` — port-collision validation, edits/deletes refused while running, last instance protected. All `/api/server/*` + `/api/queue/*` accept `?instance=N`, default = lowest id (old UI unaffected). New `POST /api/queue/event/:id`, `POST /api/queue/category/:id`.
- [x] Old UI: instance selector on queue page when >1 instance exists.
- [x] Docker: compose files map port ranges 9601-9609 (tcp+udp) and 8082-8090 for extra instances.
- [x] Smoke-tested live: create/update/delete, per-instance status + work dirs, collision rejection, delete-last guard.
- [ ] Full instance management UI (create/edit instances, per-instance dashboard, queue views) → Vue, Phases 4–5.
- [ ] Old UI start/stop only reaches the default instance (others via API until Vue dashboard).

## 2b. Car count per class entry — IMPLEMENTED ✅

One class-entry row yields N grid slots instead of duplicating the car in the list.

- [x] Schema: `user_class_entry.car_count` (default 1, `ensureColumn` migration).
- [x] Render: entries expanded count-times in `ConfigRenderer.renderIni` *before* pitbox/max-clients capping and strategy shuffle; duplicate car content extracted once.
- [x] UI: "Number of cars" input per entry card in class editor; count round-trips through existing JSON form field. Verified end to end.
- [ ] Dashboard car-modal still submits expanded single entries (collapses counts back to 1×) → fix in Vue phase.

## 2c. Side fixes landed with the above ✅

- [x] Mutex around process handle/status (was unguarded global written by UDP goroutine).
- [x] Config previews (`/api/server/server_cfg.ini`, `entry_list.ini`) render with throwaway renderer — no longer mutate live state of a running server.
- [x] Queue move-up/move-down scoped per instance (was swapping across whole table).
- [x] Nil-deref fixed in public-IP poller error path.
- [x] UDP read errors handled (transient → retry, closed socket → goroutine exits).

---

## 3. Phase 0 — API completion (Go, prerequisite for SPA)

- [ ] JSON CRUD endpoints for everything form-POST-only today: user, config, difficulty, class, session, time, event, event category. Extract shared logic from `routes.go` handlers so old UI keeps working during migration.
- [ ] Convert remaining GET-with-side-effects queue endpoints (`moveup`, `movedown`, `skipevent`, `clearcompleted`) to POST/PUT/DELETE.
- [ ] Consistent error envelope `{"error": {"code", "message"}}`; replace generic `routeDbError` 500.
- [ ] SSE stream `GET /api/server/events`: status, session changes, content-job progress (sources: UDP plugin events, `ContentJobs`). Per-instance events tagged with instance id.
- [ ] CSRF protection for mutating endpoints (double-submit cookie) — **must land before any mutating SPA page ships**. Keep JWT HttpOnly cookie auth.

### Phase 0.5 — structural cleanup (opportunistic)

- [x] Thread-safety for server status / process handle (done via Instance mutex).
- [ ] Extract shared "load tracks/cars/weathers + demo data" helper (duplicated ~12× in `routes.go`).
- [ ] Wrap multi-step operations (`applyServerEvent`, content upload) in SQLite transactions.
- [ ] Replace `go-assets-builder` with native `go:embed` (removes build dep + generated `assets.go`).
- [ ] (Optional, deferred) Service-layer extraction from 1,615-LOC `dbaccess.go`.

---

## 4. UX Redesign (target design)

Core problem: app is organized around DB tables, not the user's task. To run a race: five preset pages → assemble event → queue → start. Every CRUD action reloads the page. Mobile is a second app.

Principles: event-centric · one responsive UI · live (SSE) not polled-and-reloaded · inline creation, never navigate away mid-flow.

```
┌─ Dashboard — one card per instance: live status, players, current event,
│              start/stop; queue with drag-reorder and "up next"
├─ Events — list + Event Builder (track → classes → sessions → time/weather
│           → difficulty), presets selectable as cards, "create new" inline
├─ Content — track/car/weather library, uploads with SSE progress
├─ Presets — power-user direct access (same components the Builder uses)
└─ Settings — config, users, admin, server instances (ports, create/delete)
```

| Current pain | Fix |
|---|---|
| 5 preset pages to set up one event | Event Builder with inline preset creation |
| Queue reorder = GET + full reload | Drag-and-drop, optimistic, PUT |
| 1000ms polling ×3 | Single SSE subscription in a Pinia store |
| Unsaved-changes snapshot hack | Store-level dirty state + route-leave guards |
| Mobile = separate templates | Responsive: sidebar → bottom nav, pickers → sheets |
| Pickers reimplemented 2× | One `<ContentPicker>` component |
| Upload via raw XHR + polling | Upload component, job status over SSE |
| One server only | Instance cards, per-instance queues, instance settings |

---

## 5. Migration Phases

Each phase ships independently; old UI keeps working until Phase 6.

- [ ] **Phase 0 — API completion** (section 3) — *multi-server + car count already landed*
- [ ] **Phase 1 — SPA scaffold**: `webapp/` (Vite + Vue 3 + TS + Tailwind 4 + Pinia + vue-query); port `extra.css` tokens to `@theme`; base components (`Button`, `Card`, `Modal`, `FormRow`, `Toggle`, `Sheet`); tygo in Makefile; typed `apiClient` (error envelope + CSRF); Vite dev proxy → :3030; SPA served at `/app` until cutover. Vitest from day one.
- [ ] **Phase 2 — Simple pages**: login, initial config, about, admin, user settings.
- [ ] **Phase 3 — Presets & content**: preset editors as reusable components (Builder reuses them); content library + upload with SSE progress.
- [ ] **Phase 4 — Event Builder + Queue**: wizard with inline preset creation; queue with drag-reorder, optimistic updates, per-instance assignment/filtering; car-count editing in class/entry components.
- [ ] **Phase 5 — Dashboard**: SSE-live multi-instance dashboard (card per instance, start/stop each); instance management in Settings; port of `server.htm` last, when all components exist.
- [ ] **Phase 6 — Cutover**: SPA at `/`; delete `htm/` (~5,800 LOC), mobile templates, `routes.go` page handlers (~1,000 LOC), Alpine/Chart.js CDN refs; swap to `go:embed`.

Effort: Phase 0 remainder 2 · scaffold 2 · simple pages 1 · presets/content 3–4 · builder/queue 3 · dashboard 2 · cutover 1 ≈ **14–15 sessions**.

---

## 6. Risks & mitigations

- [ ] CSRF gap goes live with SPA — Phase 0 item, blocks first mutating SPA page.
- [ ] Demo mode (`isDemoRequest`, `withDemo*` in `routes.go`) lives in the HTML layer — API endpoints need the same injection or demo mode silently breaks in the SPA.
- [ ] Event Builder scope creep — build on existing preset semantics first; no data-model redesign in the same phase.
- [ ] No test suite exists — Vitest from Phase 1; Go `httptest` coverage for every endpoint touched in Phase 0.
- [x] ~~`ServerStatus` data races~~ — fixed with per-instance mutex.
