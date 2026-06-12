# Server Manager - Usability and UX Refactor Plan

Last updated: 2026-06-11

Scope: turn the current SPA into a clearer server-setup and race-operations
workflow. This plan includes the prioritized usability improvements from
`APP_GUIDE.md` section 5 onward, plus the requested UX changes around server
setup, Events vs. categories, admin navigation, and automatic event repeat.

Legend: `[x]` done, `[ ]` open

---

## 1. Product Direction

The app should be organized around the user's job:

1. Configure where Assetto Corsa lives.
2. Import/cache content.
3. Create reusable presets.
4. Build a runnable event.
5. Choose how a server runs that event: manual queue, scheduled queue, or
   automatic repeat.
6. Monitor and control the live server.

The UI should make the next step obvious. Users should not need to understand
the database tables behind presets, categories, queue rows, and server events.

---

## 2. Terminology and Information Architecture

### 2.1 Separate "Event" from "Category"

Current problem: Events and category feel mixed. The page starts with category
selection/creation, so it can look like an "Event" is only a name. A runnable
event actually needs track, cars/classes, sessions, difficulty, time/weather,
race laps, and overflow strategy.

- [x] Rename the current event category concept in the UI to **Event Group**,
      **Series**, or **Collection**. Recommended: **Event Groups** because it
      is explicit and neutral. (Chose **Event Groups**.)
- [x] Make **Event** mean only one runnable race definition:
      - track and layout/config
      - car class / grid entries
      - session preset
      - time and weather preset
      - difficulty preset
      - race laps / timed race settings
      - overflow strategy when cars exceed pitboxes
      - optional event name/description
- [x] Update page copy and button labels:
      - "Create event group" for grouping.
      - "Create event" for runnable race setup.
      - "Queue event" for manual queueing.
      - "Repeat event" for auto-repeat mode. (Added in Phase D.)
- [x] Keep existing API/table names initially if that reduces risk, but add
      clean frontend names and DTOs so the user model is correct.
- [ ] Later, consider backend/API rename aliases:
      - `/api/event-groups` as a friendly alias over categories.
      - `/api/events` remains runnable events.

### 2.2 Move Content Under Admin

Current problem: Content is an operational prerequisite, but not part of daily
race operation once setup is done.

- [x] Move the **Content** navigation item from Operate to Admin.
- [x] Keep `/content` as the route for backwards compatibility.
- [x] In mobile nav, replace Content with a more frequent operation item if
      needed, such as Instances or Settings. (Mobile tabs: Dashboard, Events,
      Queue, Configuration, Content.)
- [ ] Treat content import/cache as an admin/setup workflow and also expose it
      inside the first-run/onboarding wizard.

### 2.3 Target Navigation

Recommended primary nav:

- Operate
  - Dashboard
  - Events
  - Queue
  - Race Control (future)
  - History (future)
- Build
  - Event Groups
  - Presets
- Admin
  - Server Setup
  - Instances
  - Content
  - Users/Roles
  - Backup/Restore
  - Health
  - About

---

## 3. More User-Friendly Server Setup Flow

### 3.1 First-Run Wizard

Replace the passive first-run banner with a guided wizard.

- [ ] Step 1: Installation path
      - validate `server/acServer`
      - explain Docker path expectations
      - save CSP requirements when relevant
- [ ] Step 2: Content cache
      - show track/car/weather counts
      - offer rebuild cache
      - offer upload/import if content is missing
- [ ] Step 3: Server instance
      - create or confirm default instance
      - show port block and collision checks
      - explain UDP/TCP/HTTP/plugin ports in plain language
- [ ] Step 4: First event
      - pick track
      - pick or create class
      - pick or create sessions
      - pick or create time/weather
      - pick difficulty
- [ ] Step 5: Run mode
      - manual queue
      - start now
      - schedule start
      - repeat this event continuously

Acceptance criteria:

- [ ] A new install can get from login to a running server without visiting
      five unrelated pages.
- [ ] Each step has a clear success state and next action.
- [ ] Users can leave and resume setup without losing progress.

### 3.2 Server Setup Page

Create a single **Server Setup** admin page that unifies the current scattered
setup concerns.

- [ ] Show setup health at the top:
      - install path valid/invalid
      - content cache status
      - instance port status
      - default run mode
      - queue status
- [ ] Link each failed check to the exact editor section.
- [ ] Provide "Test server readiness" action:
      - can render config
      - can render entry list
      - acServer binary exists
      - ports are not duplicated across instances

---

## 4. Event Builder Refactor

### 4.1 Event List and Event Group Layout

- [ ] Split the Events page into two visual levels:
      - left or top: Event Groups/Series
      - main: runnable Event cards for the selected group
- [ ] Add a clear "New event" primary action in the main event area.
- [ ] Add "New group" as a secondary action in the group list.
- [ ] Do not open the group-name editor when the user intends to create a
      runnable event.
- [ ] Event cards should show track preview, class/grid count, session,
      time/weather, difficulty, and run mode.

### 4.2 Full Event Editing

- [ ] Event editor must set all runnable fields:
      - event name
      - event group
      - track/layout
      - car class/grid
      - session preset
      - time/weather preset
      - difficulty preset
      - race laps/timed race behavior
      - overflow strategy
- [ ] Inline validation:
      - disable save until required fields are valid
      - show missing fields next to the relevant controls
      - warn when grid count exceeds pitboxes
- [ ] Preview generated output before queueing:
      - compact `server_cfg.ini` summary
      - compact `entry_list.ini` summary
      - pitbox/max-clients warnings

### 4.3 Inline Preset Creation

- [ ] From the event editor, support "Create new" for:
      - car class
      - session preset
      - time/weather preset
      - difficulty preset
- [ ] Open nested sheets or focused sub-editors.
- [ ] Return to the event editor with the new preset selected.
- [ ] Reuse existing preset form bodies rather than duplicating logic.

### 4.4 Duplication and Templates

- [x] Duplicate event.
- [ ] Duplicate event group.
- [ ] Save event as template.
- [ ] Create event from template.
- [ ] Support "same event, different track" workflows for championship setup.

---

## 5. Queue, Scheduling, and Auto-Repeat

### 5.1 Manual Queue Improvements

- [ ] Add drag-and-drop queue reorder with optimistic updates.
- [ ] Add `PUT /api/queue/order` accepting the full ordered id list.
- [ ] Keep keyboard-accessible up/down controls as fallback.
- [ ] Add queue ETA per row.
- [ ] Add "start queue at time" scheduling.

### 5.2 Server Auto-Repeat Mode

Requested behavior: a server should be able to auto-queue/re-run the same
event it is already running, disabling manual queueing.

Proposed model: per-instance **Run Mode**.

- [ ] Add run mode to `server_instance`:
      - `manual_queue`
      - `repeat_event`
      - later: `scheduled_queue`
- [ ] Add `repeat_event_id` to `server_instance` or a dedicated
      `server_instance_run_mode` table.
- [ ] When `repeat_event` is enabled:
      - the selected event is rendered and started
      - on event/session completion, the same event is applied again
      - manual queue add/reorder/remove controls are disabled for that instance
      - Queue page shows a locked/repeat state instead of an editable queue
- [ ] Dashboard shows:
      - "Repeat mode"
      - repeated event name/track
      - action to stop repeat
      - action to switch back to manual queue
- [ ] Events page adds:
      - "Run repeatedly on..." action
      - instance picker
- [ ] API safeguards:
      - reject manual queue mutations for repeat-mode instances with 409
      - allow explicit "switch to manual queue" endpoint/action
      - do not silently delete existing queue rows when switching modes
- [ ] Decide queue preservation behavior:
      - recommended: keep manual queue rows paused/hidden while repeat mode is
        active, then restore them when switching back.

Acceptance criteria:

- [ ] A user can set one server to repeat one event forever.
- [ ] Manual queue controls for that server are visibly disabled and API
      mutations are rejected.
- [ ] Other instances can still use manual queues.
- [ ] Stopping the server does not forget the selected repeat event.

---

## 6. Quick Wins from APP_GUIDE.md

These are high-value, lower-risk improvements.

- [x] Toasts instead of inline notice rows.
      - [x] global toast stack (`stores/toast.ts` + `Toaster.vue`)
      - [x] success/error handling through a shared store
      - [ ] SSE-driven toasts for import finished and event rotated
- [ ] Unsaved-changes guards.
      - dirty flag per form
      - route-leave confirmation
      - shared helper for form snapshot comparison
- [x] Confirm dialogs in-app.
      - [x] replace `window.confirm` (Events/Queue/Dashboard)
      - [x] reuse `Modal` (`ConfirmDialog.vue` + `stores/confirm.ts`)
      - [x] spell out consequences, e.g. "removes 3 queue entries"
- [x] Empty states with calls to action.
      - [x] no presets -> create preset (event builder warns + links to Build)
      - [x] idle dashboard -> create/queue/run event
      - [x] empty content -> set install path or import content
- [ ] Loading skeletons.
      - [ ] dashboard cards
      - [x] event card grid
      - [ ] content library grids
      - [ ] preset editor shell
- [ ] Form validation before submit.
      - [ ] required markers
      - [x] disable save until valid (event builder)
      - [x] inline errors for missing fields (event builder)
- [ ] Searchable dropdowns.
      - car select in class editor
      - preset selects in event builder
      - event/group selectors where lists can grow

---

## 7. Structural Improvements from APP_GUIDE.md

- [ ] Inline preset creation in the Builder.
      - extract preset form bodies
      - host them in nested sheets
      - return with the new preset selected
- [ ] Drag-and-drop queue reorder and class entry reorder.
      - optimistic updates
      - full-order API endpoint
      - keyboard fallback remains
- [ ] Event duplication and templates.
      - duplicate event
      - duplicate group
      - save event as template
- [ ] Onboarding wizard.
      - install path -> import/cache content -> create first event -> run mode
- [ ] Queue ETA and scheduling.
      - estimate start time from session durations
      - allow server-side scheduled start
- [ ] Grid editor on the dashboard.
      - tweak running event
      - swap weather
      - adjust car list
      - restart prompt
      - count-aware so class counts do not collapse
- [ ] Results and history.
      - parse acServer result JSON
      - add `results` table
      - add History page
      - link results to queue snapshots/config provenance

---

## 8. Bigger Bets from APP_GUIDE.md

- [ ] Live race control.
      - driver list
      - ping/kick
      - broadcast chat
      - next/restart session
      - admin commands
- [ ] Real-time lap/position widget.
      - use ACSP lap completed and car updates
      - show position, last lap, and gaps
- [ ] Mobile-first pass.
      - improve bottom-tab navigation under 640 px
      - larger touch targets
      - sheets as default picker pattern
      - verify no horizontal scroll
- [ ] Multi-user and roles.
      - admin
      - race steward
      - read-only
      - protect admin-only routes/actions
- [ ] Backup/restore in-app.
      - download full backup zip
      - restore upload flow
      - confirmation diff before replacing data
- [ ] Health panel.
      - external port reachability
      - UDP reachability
      - lobby registration status
      - disk space
      - CSP version detection

---

## 9. Consistency Debt from APP_GUIDE.md

- [ ] Converge legacy endpoint errors on the new error envelope:
      `{"error": {"code", "message"}}`.
- [ ] Add clean DTO twins for GET payloads with quoted numbers from legacy
      `",string"` JSON tags.
- [ ] Remove SPA-side normalizers after DTO cleanup.
- [ ] Keep native `go:embed` asset workflow documented:
      - schema, INI templates, favicon, and SPA bundle live under `src/embed`
      - after `make clean`, run `make webapp` before raw Go tooling that needs
        embedded UI assets

---

## 10. Recommended Implementation Order

### Phase A - Information Architecture and Labels

- [x] Move Content under Admin in navigation.
- [x] Rename category UI to Event Groups.
- [x] Update Events page copy so "event" clearly means runnable race setup.
- [x] Add page-level empty states and calls to action.

Why first: low risk, resolves immediate confusion, and improves every later
feature discussion.

**Phase A status: done.** Also landed early: toast stack, in-app confirm
dialog, empty/skeleton components, and single-event duplication.

### Phase B - Server Setup Wizard

- [ ] Build the first-run wizard.
- [ ] Add Server Setup admin page with setup health checks.
- [ ] Reuse existing config/content/instance/event editor pieces.

Why second: this creates the friendly setup flow and reduces support friction.

### Phase C - Event Builder Completion

- [ ] Redesign Events page around Event Groups + Event cards.
- [ ] Ensure event editor exposes all runnable fields.
- [ ] Add inline validation.
- [ ] Add inline preset creation.
- [ ] Add event duplication/templates.

Why third: events become the central object users understand and run.

### Phase D - Run Modes and Auto-Repeat

- [ ] Add per-instance run mode to DB/API.
- [ ] Implement repeat-event engine behavior.
- [ ] Disable manual queue controls and mutations for repeat-mode instances.
- [ ] Add Dashboard/Events/Queue controls for switching modes.

Why fourth: auto-repeat touches backend lifecycle, queue semantics, and UI
state, so it should land after terminology and event editing are clear.

### Phase E - Queue, Scheduling, and Race Operations

- [ ] Drag-and-drop reorder.
- [ ] Queue ETA and scheduled starts.
- [ ] Dashboard grid editor.
- [ ] Race control panel.
- [ ] Live timing widget.

Why fifth: improves live operations after the setup and event model are stable.

### Phase F - Administration and Reliability

- [ ] Results/history.
- [ ] Multi-user roles.
- [ ] Backup/restore.
- [ ] Health panel.
- [ ] API consistency cleanup.

Why last: these are high-value but depend on stable core workflows.

---

## 11. Open Design Decisions

- [ ] Final label for categories:
      - recommended: Event Groups
      - alternatives: Series, Collections, Championships
- [ ] Whether event name should be required or auto-generated from
      track/class/session.
- [ ] Whether repeat mode should repeat the full event or only the current
      session inside the event.
- [ ] What happens to existing queue rows when repeat mode is enabled:
      - recommended: preserve but disable/hide while repeat mode is active
- [ ] Whether scheduled starts should be part of queue mode only or a separate
      per-instance run mode.

