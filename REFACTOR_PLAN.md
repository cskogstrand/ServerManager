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
- [x] Full instance management UI (create/edit instances, per-instance dashboard, queue views) → delivered in Vue Phases 4–5.
- [x] Old UI start/stop only reaches the default instance → obsolete after Phase 6 cutover removed the old UI.

## 2b. Car count per class entry — IMPLEMENTED ✅

One class-entry row yields N grid slots instead of duplicating the car in the list.

- [x] Schema: `user_class_entry.car_count` (default 1, `ensureColumn` migration).
- [x] Render: entries expanded count-times in `ConfigRenderer.renderIni` *before* pitbox/max-clients capping and strategy shuffle; duplicate car content extracted once.
- [x] UI: "Number of cars" input per entry card in class editor; count round-trips through existing JSON form field. Verified end to end.
- [x] Dashboard car-modal still submits expanded single entries (collapses counts back to 1×) → obsolete after Phase 6 cutover; class count editing lives in the Vue class editor.

## 2c. Side fixes landed with the above ✅

- [x] Mutex around process handle/status (was unguarded global written by UDP goroutine).
- [x] Config previews (`/api/server/server_cfg.ini`, `entry_list.ini`) render with throwaway renderer — no longer mutate live state of a running server.
- [x] Queue move-up/move-down scoped per instance (was swapping across whole table).
- [x] Nil-deref fixed in public-IP poller error path.
- [x] UDP read errors handled (transient → retry, closed socket → goroutine exits).

---

## 3. Phase 0 — API completion (Go, prerequisite for SPA) — IMPLEMENTED ✅

- [x] JSON CRUD endpoints (`src/apicrud.go`) reusing the same Dbaccess functions the HTML routes use:
  - difficulties / sessions / times / classes / categories: `GET /api/<plural>` (+`?filled=1`), `POST /api/<plural>` `{name}`, `GET|PUT|DELETE /api/<singular>/:id` (time incl. weather panels, class incl. entries+count, category incl. nested events)
  - events: `GET /api/events`, `POST /api/events`, `GET|PUT|DELETE /api/event/:id` — clean DTO (`event_category_id`, `track_key`/`track_config`, plain-number ids), validated
  - config: `GET /api/config` (secret key redacted via `json:"-"`), `PUT /api/config`, `PUT /api/config/content` (CSP/install-path half)
  - user: `GET /api/user` (password never emitted), `PUT /api/user` — password bcrypt-hashed on change (the old form path stored it plaintext; API path fixes that)
- [x] GET-with-side-effects converted to POST: `server/start`, `server/stop`, `content/recache`, `queue/moveup|movedown|skipevent|clearcompleted`; old templates patched.
- [x] Error envelope `{"error": {"code", "message"}}` for all new endpoints (`apiError`/`apiDbError` in `src/apihelpers.go`; FK violations → 409 `in_use`). Legacy endpoints keep their shape until old UI dies.
- [x] SSE stream `GET /api/server/events` (`src/events.go`): per-instance snapshot on connect, then `session` / `players` / `server` (running) / `content_job` events; 15s heartbeat; slow consumers dropped, never block publishers.
- [x] CSRF: auth cookie now `SameSite=Lax`; double-submit `csrf_token` cookie + `X-CSRF-Token` header enforced on all mutating `/api` requests (`CsrfMiddleware`); cookie issued at login and back-filled on first safe request for existing sessions; old-UI fetch/XHR call sites patched (`smPost`/`smCsrf` helpers in `header.htm`, inline on standalone mobile pages).
- [x] Smoke-tested: 403 without token, full difficulty/session/category/user CRUD round-trips, bcrypt password change verified by re-login, SSE snapshot streams, GET queue mutations 404, POST works.

### Phase 0.5 — structural cleanup (opportunistic)

- [x] Thread-safety for server status / process handle (done via Instance mutex).
- [ ] Extract shared "load tracks/cars/weathers + demo data" helper (duplicated ~12× in `routes.go`).
- [ ] Wrap multi-step operations (`applyServerEvent`, content upload) in SQLite transactions.
- [x] Replace `go-assets-builder` with native `go:embed` (removes build dep + generated `assets.go`).
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

- [x] **Phase 0 — API completion** (section 3) + multi-server + car count (sections 2a/2b)
- [x] **Phase 1 — SPA scaffold** — `webapp/` with Vite 6 + Vue 3 `<script setup>` + TS strict + Tailwind 4 + Pinia + vue-query; slate tokens ported to `@theme` (`src/style.css`); base components `Button/Card/Modal/FormRow/Toggle/Sheet` (`src/components/ui/`); typed `api` client with envelope + CSRF handling (`src/lib/api.ts`); SSE subscription with reconnect (`src/lib/sse.ts`) feeding a Pinia server store (live instance status — polling gone); tygo generates `src/types/generated.ts` from `dbmodels.go` (`make types`); Vitest (7 tests, jsdom); Go serves the SPA at `/app` with index.html fallback for history routing (`routeSpa`); `make webapp` / `make webapp-dev` targets; docker dev image builds it on first boot. Proof-of-pipeline Dashboard page: live per-instance cards with start/stop over SSE.
- [x] **Phase 2 — Simple pages** — Login (JSON `POST /api/login` / `POST /api/logout` sharing cookie issuance with the form login), auth store + router guard (redirects to /login, preserves target), Server Configuration (full `user_config` form minus ports — instances own those now), Preferences (units + password change with confirm), About (`GET /api/about`: version/paths + download links). `Input`/`Select` added to the component set. The old admin page was a static placeholder (hardcoded table) — dropped, not ported.
- [x] **Phase 3 — Presets & content** — all four preset editors (Difficulty, Sessions, Time & Weather with dynamic panels, Car Classes with car/skin pickers + per-entry car count + reorder) on a shared `PresetShell` (list/create/delete) + `usePresetPage` controller; Content page: searchable track/car/weather library with thumbnails, upload (file or URL, overwrite toggle, XHR progress bar) and live import-job progress over SSE; `GET /api/cars|tracks|weathers` cache endpoints added; content store auto-refreshes caches when an import completes. Fixed along the way: legacy `",string"` json tags rejected proper JSON numbers — time updates now bind a clean DTO, the `Input` component emits real numbers, and the time editor normalizes legacy string-numbers on load.
- [x] **Phase 4 — Event Builder + Queue** — Events page: categories sidebar, events as preview cards with preset chips, one-click "Queue"; Builder in a side sheet: searchable visual track picker (preview grid), preset dropdowns (filled only), race laps / overflow strategy; events managed through the clean `/api/events` DTO (added `PATCH /api/category/:id` rename — the nested PUT deletes unsent events, SPA avoids it). Queue page: per-instance tabs with live status dots, queue table (active ▶ / done ✓ / pending with reorder + remove), skip with confirm, clear completed, start/stop per instance, add single event or whole category to any instance; refetches on SSE running-state changes. New API: `GET /api/queue?instance=N`, `DELETE /api/queue/:id`. Car-count editing lives in the class editor (Phase 3). Verified end-to-end including rendered `entry_list.ini` (count expansion → 4 slots) and per-instance ports in `server_cfg.ini`.
  - [ ] Polish (later): true inline preset creation inside the Builder (currently dropdown + link to the editor), drag-and-drop reorder (currently ↑/↓ buttons).
- [x] **Phase 5 — Dashboard + instance management** — Dashboard: one rich card per instance with current event (track preview + preset chips), live session detail (type, progress x/y, length, elapsed, temps, weather), grid summary grouped by car model with counts, collapsible console (last 200 lines, auto-refreshes every 5s only while open), skip-event and start/stop per instance, public IP; SSE drives status/players/session, detail payload refetches on running-state changes. Settings → Instances: card list with ports, create (suggests next free port block within the docker-mapped ranges), edit/delete guarded while running, port-collision errors surfaced from the API.
- [x] **Phase 6 — Cutover** — SPA serves at `/` (NoRoute fallback to index.html for client routes; `/api` misses return a JSON 404 envelope); `/app/*` 301-redirects for old bookmarks; deleted: `htm/` (all 20 templates), `css/` (extra.css + tailwind input), root `package.json`/npm setup, all `routes.go` page handlers + demo-data layer + HTML login (routes.go is now just the SPA handler, ~9.6k LOC removed net); `AuthenticateMiddleware` returns 401 JSON instead of redirecting; Makefile/Dockerfile.dev rebuilt around `webapp/` only (added `make test`); first-run setup banner in the SPA + Installation card (install path validation + CSP settings) on the Content page — that flow previously only existed in the old UI. Verified: `/`, deep links, `/app` redirects, 401/404 envelopes, login, status, hashed assets.
  - [x] `go:embed` swap: schema, INI templates, favicon, and Vite output now live under `src/embed`; Go serves native `embed.FS` and `go-assets-builder`/`src/assets.go` are gone.
- [x] **Phase 6.5 — UI system polish** — `ui-ux-pro-max` guided operations-dashboard pass: grouped desktop nav + mobile bottom nav, skip link, SVG icon system (removed text/emoji UI icons), shared page headers, stronger focus/hover states, denser cards/forms/toggles/modals/sheets, improved Dashboard/Queue/Event Builder/Content/Settings/About visual hierarchy. Verified with Vite build, Vitest, Docker Compose `make test`, and container-served bundle smoke check.

Effort: Phase 0 remainder 2 · scaffold 2 · simple pages 1 · presets/content 3–4 · builder/queue 3 · dashboard 2 · cutover 1 ≈ **14–15 sessions**.

---

## 6. Risks & mitigations

- [x] ~~CSRF gap goes live with SPA~~ — double-submit + SameSite=Lax landed in Phase 0.
- [x] ~~Demo mode lives in the HTML layer~~ — removed together with the old UI at cutover (it was a debug-only browse mode; reintroduce in the SPA later if wanted).
- [ ] Event Builder scope creep — build on existing preset semantics first; no data-model redesign in the same phase.
- [ ] No test suite exists — Vitest from Phase 1; Go `httptest` coverage for every endpoint touched in Phase 0.
- [x] ~~`ServerStatus` data races~~ — fixed with per-instance mutex.
