# Server Setup UX Overhaul Plan

## Summary
The current UX is technically capable but organized around implementation concepts: content, config, presets, event groups, events, queue, instances, and run modes. A new user has to understand that dependency graph before they can start a server. Editing has the same problem: the user must know whether to edit an event, a preset, a queue row, an instance, or the running server.

Refactor the UX around two jobs instead:

- **Set up and run a server**: one guided workbench from login to first running server.
- **Build, edit, and reuse race setups**: one event editor reused from Setup, Events, Queue, and Server Detail.

Keep the existing backend model where practical, but stop exposing it as the primary workflow.

## Key Changes

### 1. Replace `/setup` With A Setup Workbench
Turn `ServerSetup.vue` from a passive readiness checklist into the first-run command center.

Layout:
- Top: compact readiness bar with `Install`, `Content`, `Server`, `Race`, `Run` status.
- Left rail: non-blocking step list with pass/fail/resume state.
- Main panel: active setup step editor.
- Right panel: live “what will run” summary with server name, instance, track, grid size, sessions, weather, run mode, and rendered-config warnings.
- Footer: persistent `Back`, `Save draft`, `Preview`, `Start server` / `Queue event` actions.

Flow:
1. **Install & Content**: set AC path, validate it, show content counts, rebuild/import content without leaving setup.
2. **Server Identity**: set lobby name, access password, admin password, engine, and common limits. Keep advanced engine fields collapsed.
3. **Instance**: create or choose the server instance, show suggested ports, warn about Docker port ranges, validate conflicts inline.
4. **Race Setup**: choose track, cars/grid, sessions, time/weather, difficulty, laps, overflow strategy in one editor. Presets appear as reusable templates, not required page visits.
5. **Run**: choose `Start now`, `Queue`, `Schedule`, or `Repeat continuously`.

Success criteria:
- A fresh install can reach a running server from `/setup` without navigating to Content, Presets, Events, Queue, or Instances.
- Every failed readiness item has an inline fix, not just a link elsewhere.
- Users can leave and resume setup from the same step.

### 2. Create A Shared Race Setup Editor
Extract the Events builder into a reusable `RaceSetupEditor` used by:
- Setup Workbench first event step.
- Events page create/edit.
- Queue page “add/edit queued event”.
- Server Detail “edit current/next race setup”.

Behavior:
- First-class fields: event name, group/series, track/layout, grid, sessions, time/weather, difficulty, race length, overflow strategy.
- Presets are optional accelerators:
    - `Use saved preset`
    - `Customize for this event`
    - `Save as reusable preset`
- When editing a shared preset, show a scope choice:
    - `Only this event` creates a copy and updates the event.
    - `All events using this preset` edits the shared preset.
- Add a review section with pitbox warnings, grid count, `MAX_CLIENTS`, missing content, and rendered INI preview before save/start.

### 3. Rework Existing Pages Around Tasks
**Events stays, but becomes the race setup library.**
- Primary action becomes `New race setup`, available even when no group is selected.
- Event groups move to filters/organization, not the gate before creating an event.
- Add search, group filter, run-mode chips, and compact/list view toggle.
- Event cards show actionable summaries: track image, grid count vs pitboxes, sessions, weather, difficulty, queued/repeating status.
- Actions: `Edit`, `Queue`, `Start on instance`, `Repeat`, `Duplicate`, `Save as template`.

**Queue stays, but becomes Run Plan.**
- Rename visible page title to `Run Plan`; keep `/queue` route for compatibility.
- Layout: instance tabs at top, timeline/list in main area, right panel for `Add race setup`, `Start now`, `Schedule`, `Repeat`.
- Show queue ETA per row using session duration totals where possible.
- In repeat mode, show the pinned event and a clear `Switch to manual queue` action; do not show disabled queue controls as the main content.

**Dashboard stays, but idle servers need stronger next actions.**
- Idle instance cards should show `Start setup`, `Queue race`, or `Start repeat` depending on readiness.
- Add visible “next blocker” when start would fail.
- Link `Edit run setup` from current/upcoming event into the shared editor.

**Server Detail stays, but editing gets unified.**
- Replace narrow `Edit grid & weather` with `Edit race setup`.
- First prompt asks scope: `Apply after restart`, `Restart now`, or `Edit reusable source`.
- Reuse the shared editor and keep current quick grid/weather editing as a fast path inside it.

**Content stays, but becomes Library & Content.**
- Setup owns first-run install path and cache rebuild.
- Content page focuses on browsing, importing, rebuild history, and content health.
- Add clearer empty states: “No tracks cached” should offer `Set install path`, `Rebuild cache`, and `Upload content`.

**Preset pages stay as Advanced Templates.**
- Keep routes for power users.
- Improve `PresetShell` with search, usage counts, duplicate, and “used by N events” warnings.
- Add clone-before-edit affordance when changing shared presets.

**Settings and Instances stay, but are no longer first-run requirements.**
- Server Configuration becomes advanced global defaults.
- Instances page remains for multi-server management, streams, spectator slots, and port tuning.
- Add a small setup-health badge linking back to `/setup` when configuration blocks running.

## API / Interface Changes
Use existing APIs for most saves. Add only helper endpoints where they remove fragile frontend orchestration:

- `GET /api/setup/summary`: extends readiness with current config, instances, preset lists, event groups, content counts, suggested next instance ports, and blocking issues.
- `POST /api/server/render-preview`: accepts either `event_id` or an unsaved draft payload and returns `server_cfg`, `entry_list`, warnings, grid count, pitbox cap, and render errors.
- Optional later: `POST /api/setup/finish` to queue/start/schedule/repeat the selected event in one request after all referenced objects already exist.

Frontend additions:
- `RaceSetupEditor.vue`
- `SetupWorkbench.vue`
- `useRaceSetupDraft.ts`
- `useSetupSummary.ts`
- Shared types for draft event, draft presets, validation issues, preview result, and run action.

## Test Plan
- Backend tests for setup summary, render preview for saved and draft events, port conflicts, missing preset/content failures, and repeat/manual queue guards.
- Vue/Vitest tests for setup flow states: empty install, valid path/no cache, content cached/no event, complete setup, repeat mode, and shared-preset edit scope.
- Component tests for `RaceSetupEditor`: required fields, inline preset/custom mode, pitbox warnings, save disabled states, and preview rendering errors.
- Manual/browser verification at 375, 768, 1024, and 1440 px for `/setup`, `/events`, `/queue`, `/server/:id`, `/content`, and preset pages.
- Acceptance test: fresh database user can create first runnable server from `/setup` without using sidebar navigation.

## Assumptions
- Keep the existing dark operations-console visual system, 8px radii, compact cards, and current component library.
- Keep existing routes for compatibility; change page titles and primary workflows where needed.
- Do not redesign the database schema in this pass. The UX can hide presets/event groups without removing those concepts from storage.
- Default run action in setup is `Start now` when readiness is complete; otherwise `Queue` is offered only after an event exists.
