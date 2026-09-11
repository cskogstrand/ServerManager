# Pitlane production implementation plan

Prepared 11 September 2026. Repository: `/Users/cskogstrand/privateprojects/ServerManager`.

## 1. Outcome and scope

Replace the production frontend with the Pitlane experience demonstrated in `docs/pitlane-concept/`, preserving the original application's capabilities and connecting the new flows to real, persistent backend behavior. The result must feel simple to a club host holding a phone and remain useful to an operator managing several servers, simulator feeds, and shared rigs.

This is an implementation plan, not a request for another mockup. The concept is a design reference with simulated data and operations; the production application is the Vue SPA in `webapp/` and Go backend in `src/`. Necessary backend changes are part of implementing the experience. A page that saves a local draft and displays a success toast without performing its promised operation is not complete.

The user’s direction, in order:

1. Make the interface self-explanatory, with excellent defaults and little configuration work before a first drive. Hide routine mechanics behind simple actions, while explaining consequences.
2. Preserve simulator streams in **Session → Watch**, stream setup in that context, and a **Live** destination showing all connected feeds, including rigs without an active game connection.
3. Keep actual track maps with live driver positions, linked to the selected camera and timing row.
4. Give rigs and cameras dedicated management pages under Garage: stream setup, gear, display arrangement, location, notes, and driver associations. Rigs are equipment; drivers are people.
5. Give Cars & tracks a complete library and detail/manage pages. Visually distinguish **Your servers** from **The club** in Garage.
6. Put the complete original administration capability behind **Garage → Advanced**, with clearer language, custom configuration, diagnostics, and recovery tools.
7. Use the supplied transparent screen-setup PNGs everywhere these illustrations appear.
8. Preserve the latest large-screen Watch fix: balanced stream/map panels, a bounded video height, a much larger map, and compact secondary feeds in fullscreen.

“Pitlane” is the working UI identity. Use its branding in the interface; do not rename the binary, repository, configuration directories, or external integration paths as an incidental part of the UI migration.

## 2. Sources of truth and initial reading

Read these before implementing:

| Source | What it establishes |
| --- | --- |
| [Concept README](pitlane-concept/README.md) | Product purpose, daily flows, terminology, simplicity rules, backend gaps. |
| [Feature parity](pitlane-concept/FEATURE-PARITY.md) | Existing functionality that the simpler mock does not fully demonstrate. All existing capabilities must survive. |
| [Advanced coverage](pitlane-concept/ADVANCED-COVERAGE.md) | Catalogue of 53 tools across 14 sections; distinguishes real prototype interactions from simulated operations. |
| `docs/pitlane-concept/index.html`, `style.css`, `app.js` | Main visual and interaction reference. |
| `docs/pitlane-concept/watch.js`, `watch.css` | Watch, all-stream Live, map selection, missing-feed states, and latest fullscreen sizing. |
| `docs/pitlane-concept/garage.js`, `garage.css`, `advanced.js` | Rigs, content, Garage hierarchy, Advanced discovery and review. |
| [Application capabilities](../APP_CAPABILITIES.md), [application guide](../APP_GUIDE.md) | Domain overview. Verify claims against current source. |
| `webapp/src/router.ts`, `src/main.go`, `src/roles.go`, `src/routes.go` | Actual routes, API registration, permissions, SPA/deep-link behavior. |
| [Repository guidance](../CLAUDE.md), applicable skills/instructions | Working conventions and environment constraints. |

Resolve disagreement as follows: explicit user requirements and this plan’s recorded refinements govern the desired experience; running concept/source governs current visual detail; production code governs existing contracts, permissions, and semantics. Older planning documents and screenshots are context, not a reason to undo accepted refinements. In particular, early screenshots predate the transparent rig art and 4K Watch fix.

Preview the concept separately from production:

```sh
python3 -m http.server 8786 --bind 127.0.0.1 --directory docs/pitlane-concept
```

If that port is already serving the concept, reuse it. Walk through Today → New session → Drift/Race/Practice → setup → start/schedule; Session control → Next driver → Finish → recap → Run it again; Watch and Live; Drivers; Garage → rigs/content/Advanced. Exercise a missing camera, stale telemetry, an invalid grid, and a server conflict. Use the source when local UI access is unavailable, and record the visual verification limitation.

## 3. Architecture: keep the working foundations

### Existing stack and reusable seams

Keep Vue 3, TypeScript, Vue Router with history URLs, Pinia, TanStack Vue Query, Tailwind 4, Vite, Gin, and SQLite. No framework migration, second frontend application, new database, or generic workflow engine is needed. Use CSS and native browser capabilities where possible. Follow the repository’s Ponytail guidance without cutting required functionality or verification.

| Concern | Reuse or extend |
| --- | --- |
| App shell, authentication, roles, themes | `webapp/src/App.vue`, `stores/auth.ts`, `components/ui/*`, `lib/api.ts`, `router.ts` |
| Server truth and live recovery | `stores/server.ts`, `stores/feed.ts`, `lib/sse.ts`, `src/events.go`, `src/server.go`, `src/instance.go` |
| Session setup | `components/RaceSetupEditor.vue`, `lib/useRaceSetupDraft.ts`, `lib/useRaceSetupPreview.ts`, `lib/useSetupSummary.ts`, `components/presets/*` |
| Operating a server and its queue | `pages/ServerDetail.vue`, `pages/Queue.vue`, `lib/queueState.ts`, `src/api.go`, `src/scheduler.go` |
| Map, timing, and instruments | `pages/Broadcast.vue`, `lib/raceTelemetry.ts`, `src/positions.go`, `src/telemetryingest.go` |
| Playback | `components/WhepPlayer.vue`, `components/StreamWall.vue`, `components/StreamTheater.vue`, `lib/useWhep.ts`, `lib/useDriverStreams.ts` |
| Capture and media | `lib/useDriverCapture.ts`, `components/ManualRecordings.vue`, `components/MediaActions.vue`, `src/drivercapture.go`, `src/driverbuffer.go` |
| Drivers and history | `lib/driversApi.ts`, `lib/guestDriversApi.ts`, `pages/DriverDetail.vue`, `pages/GuestDriverDetail.vue`, `pages/SessionSearch.vue`, `pages/ResultsHistory.vue`, `src/drivers.go`, `src/guestdrivers.go`, `src/results.go` |
| Content | `stores/content.ts`, `pages/Content.vue`, `components/TrackPicker.vue`, `components/TrackImage.vue`, `src/contentjobs.go`, `src/contentupload.go`, `src/contentdownload.go` |
| Persistence and API types | `src/dbmodels.go`, `src/dbaccess.go`, `src/embed/schema.sql`, `webapp/src/types/generated.ts`, `tygo.yaml` |

Extract reusable behavior from large existing pages as they are migrated; do not first undertake an unrelated whole-repository refactor. Keep the shared SSE store and its reconnect/visibility recovery. Do not add one polling loop per new page or duplicate the server state in a new store. Scope queries, selections, mutations, and invalidation by their actual instance/session/source identifiers.

Port the concept into Vue components and production styles. Do not import the prototype’s global scripts, use its `innerHTML` rendering, or ship its `sessionStorage` domain model. Test fixtures may reuse illustrative scenarios, but must be explicitly enabled and isolated from real accounts and operations.

### Build boundary

Vite builds into `src/embed/webapp/dist`; Go embeds that output. Edit source in `webapp/`, not generated bundles. Regenerate model types with `make types` when changing the Go models that feed `tygo`. Do not hand-edit generated types. Preserve the production SPA fallback, missing-asset 404s, shell revalidation, and stale-chunk recovery. Vite-hashed assets replace the concept’s manual stylesheet cache versions.

## 4. Information architecture and route compatibility

Use four main destinations: **Today · Sessions · Live · Drivers**. Garage is secondary. Personal preferences and sign-out remain easy to find. Installation setup appears when required by actual readiness, with an appropriate explanation for users who cannot administer installation.

Proposed canonical route structure, to be finalized in the first milestone:

| Destination | Proposed production route | Required behavior |
| --- | --- | --- |
| Today | `/` | Current action, live/planned sessions, New session, useful recent activity; real empty states. |
| Sessions | `/sessions` | Drafts, planned, queued/live, completed; access to search drives and reusable setups. |
| New / edit session | `/sessions/new`, `/sessions/:sessionId/edit` | Experience choice, complete defaults, expandable details, authoritative review. |
| Session control / recap | `/sessions/:sessionId` | One durable driving-session identity; show the appropriate lifecycle view. |
| Watch | `/sessions/:sessionId/watch` | Camera, map, timing, instruments, stream setup, capture. |
| Historical stint search | `/sessions/drives` | Preserve the existing search filters and precise driver-stint links. |
| Live | `/live` | All configured/connected sources, session filters, entry to Watch and club display. |
| Drivers | `/drivers`, existing `/drivers/:guid`, `/guest-drivers/:id` | Unified presentation with distinct underlying identifiers and permissions. |
| Garage | `/garage` | Rigs & cameras, Cars & tracks, distinct server/club sections, Advanced. |
| Rigs / cameras | `/garage/rigs`, `/garage/rigs/:rigId` | Equipment and source management; standalone cameras accessible here too. |
| Content | `/garage/content`, typed detail route | Cars, tracks/layouts, weather, imports, metadata, management. |
| Advanced | `/garage/advanced/:section?` | Searchable tools, scope, settings, evidence, recovery. |

These are proposed paths, not existing endpoints. Define static routes before potentially conflicting dynamic identifiers. Content identity must include kind, key, and track layout where relevant; never assume a track display name identifies its layout.

Compatibility is a deliverable, not a cleanup task:

- `/sessions` currently opens `SessionSearch.vue`. Preserve old query/filter links by routing recognized legacy search parameters to `/sessions/drives`, and keep Search drives prominent in the new hub. Specify and test the actual query keys from the existing page. Do not silently discard them.
- `/server/:id` identifies an **instance**, not a driving session. Resolve its current session or show that instance’s state; never reinterpret that numeric ID as a session ID. Apply the same rule to `/server/:id/broadcast`.
- Preserve `/broadcast` auto-follow, `/leaderboard`, `/history`, `/events`, `/queue`, `/content`, all `/presets/*`, `/settings/*`, `/setup`, `/maintenance`, `/about`, login, access-denied, and driver/guest links through aliases, redirects, or deliberate compatibility views. Retain query strings and selected-instance context.
- Audit the existing `/app/*` redirect in `src/routes.go`: it currently derives a path without explicitly carrying the raw query. Include legacy parameter preservation in route tests.
- Keep login redirect-back behavior, role checks, deep-link reloads, and a useful not-found page. Never expose formerly restricted pages merely because they moved underneath Garage.

Keep existing implementations available during incremental migration. A temporary route into an old operational page is acceptable while building, but is not final parity. Remove old presentation only once its replacement passes the corresponding checks; do not maintain two permanent sets of domain logic.

## 5. Data and backend work the concept requires

The contracts below are proposed additions or extensions. Inspect current code before selecting exact names; do not treat these as APIs that already exist. Resolve them early and record the decision in the implementation progress file.

### A. A durable driving session

The backend has different concepts: `user_event` is reusable configuration; `server_event` is an instance queue/run record; a game session is a practice/qualifying/race phase; `driver_connection` is a connection span; `driver_session` is a driver’s phase segment. Preserve those meanings.

Introduce the smallest persistent driving-session aggregate needed to join the UI journey. Prefer extending current records where the lifecycle is truly compatible; otherwise add an additive `driving_session` table and explicit links. The contract needs:

- Stable ID, experience type, name, revision, lifecycle, chosen instance, scheduling information, configuration ownership, and created/started/ended times.
- Draft → planned/queued → starting → live → finished, with explicit cancelled/failed outcomes. Derive live state from confirmed server activity, not optimistic frontend state.
- An immutable effective configuration snapshot per execution, including generated configuration or its durable reference, content/layout identifiers, and applicable scoring rules.
- Explicit correlation from each actual execution to game phases, driver stints, results, and captures. A repeated event produces distinct executions; a queue entry or preset ID alone is not an execution identity.
- Existing history remains accessible. Unlinkable legacy records stay labelled legacy/unlinked; do not manufacture associations from matching names or approximate timestamps.

“Run it again” creates a new draft from the snapshot. Editing a session uses privately owned preset copies or equivalent isolated values by default. Current event duplication copies preset references; it does **not** provide isolation. Shared preset editing stays explicit, reports real usage, and honors existing role restrictions.

### B. Start exactly what the user reviewed

Current `Events.vue` adds the selected event to the end of the queue and starts from the first queued event. Its in-memory retry marker prevents some duplicates only while the page remains open. That behavior cannot back the new **Start session** promise.

Add a narrow server-side launch operation, for example `POST /api/driving-sessions/:id/start`, with a client idempotency key, expected revision, and explicit/automatically selected instance. It must:

1. Validate permissions, installed engine/content/layout, configuration, grid/pitbox/client limits, current queue/run mode, schedule, and instance availability.
2. Save isolated configuration and the execution intent transactionally. Reserve the target against concurrent operators; do not hold a database transaction open across process launch.
3. Start that exact reviewed execution using existing server lifecycle machinery. Never clear or reorder an unrelated queue silently. If the queue conflicts, return a specific choice: another compatible server, after current, or an explicitly reviewed plan change.
4. Persist the operation outcome so retrying after a timeout, refresh, or duplicate click returns the same operation rather than another enqueue/start. Report partial failure with a resumable state; reconcile process status on recovery.
5. Confirm started/failed from the backend, return session and instance IDs, and invalidate the relevant live/query state.

Reuse the existing renderer and readiness APIs (`/api/server/render-preview`, `/api/setup/summary`, `/api/server/readiness`). The final operation revalidates; frontend checks are only early feedback. Give field-specific blockers a direct remedy and preserve the draft while resolving them.

Automatic server selection must consider compatibility and reservations. Timed phases have estimates; lap-based races, open practice, and overruns have uncertain finish times. Do not promise a guaranteed free interval from a guessed duration. Store absolute schedule timestamps plus the chosen IANA time zone for presentation, and test DST/overrun/conflict behavior. Existing per-instance one-shot scheduling is not a multi-session reservation calendar.

### C. Rigs and source identity

Add persistent rig inventory: stable ID, name, location, display enum (`triple`, `single`, `wide`, `ultrawide`, `vr`, `custom`), notes, usual-driver reference, and gear rows. Gear includes type, name/model, notes, and stable ID. This is inventory; selecting VR or adding a wheelbase does not configure hardware or the game.

Represent each camera source independently, with stable ID, optional rig association, optional standalone/spectator-session association, player configuration, optional status URL, and separate recording configuration. Support more than one source per rig. Reuse existing `driver_stream` and instance spectator settings through an additive migration or compatibility adapter; preserve their IDs, URLs, settings, and existing playback. Do not infer a physical rig from a person’s name. Existing unbound sources may remain unassigned until the operator attaches them.

Keep technical game identity (GUID/connection/instance/car), physical rig, guest or account-driver identity, and SM login account distinct. Resolve current occupancy from authoritative live assignment; a usual driver is only a default. The prototype’s special `rig-01` behavior must become a general capability for any shared rig.

### D. Streams without game presence

`useDriverStreams.ts` currently sets `online` and effective health from `DriverState.connected`; status polling is per-instance and connected GUID. Extend this shared path and the backend so all configured sources can have health and playback independently of game presence. Use source IDs for selection and health; do not use a driver GUID as the sole camera key.

Separate **playback**, **telemetry**, and **recording** states, timestamps, and failure reasons. A failed poll means unknown/stale, not proven offline. Status may be unavailable for an embed that still plays. Use a real player/probe result for “Check connection”; successful URL validation alone must not become a green Connected badge.

Reuse native WHEP playback and sandboxed embeds. Preserve teardown, retry, mute, fullscreen, and media resource cleanup. The live wall should show all relevant feeds without opening an unbounded number of high-resolution decoders: use muted previews, pause offscreen playback, and keep the selected feed active. Playback URLs required by authorized viewers and privileged capture/probe credentials need distinct response handling; never place capture secrets in logs, recent-change diffs, or viewer diagnostics.

### E. Shared-rig handover and capture ownership

Build **Next driver** on `POST /api/server/assign-driver`, guest roster APIs, and the existing attribution/capture code. Define an authoritative boundary before changing ownership: complete or explicitly close the current run/recording, persist its original owner, then assign future telemetry and media to the next person. Handle in-flight automatic captures and manual recordings using ownership captured when their work began. Test reconnects and same-account guests.

Keep historical correction separate. “Next driver” must never rewrite past results. “Correct who was driving” must retain the existing scope, permission rules, and explicit affected records. Backend failure leaves the previous assignment visible and recoverable.

### F. Content metadata and Advanced extensions

Reuse the full installed-content service and async import jobs. Add local tags/notes/display overrides and reversible **archive** only as durable metadata separate from the content cache, so a rescan does not erase them. Archive hides an item from ordinary selection; it does not delete disk content or break an existing session. Usage and delete checks must be authoritative, not the prototype’s name matching.

Preserve raw generated-INI inspection immediately. For **custom INI overrides**, define a persisted, session-scoped draft/override contract and validation against the effective rendered configuration before allowing it to run. Maintain instance port, path, permission, content, and lifecycle invariants. Do not implement arbitrary filesystem editing or silently overwrite hand-written changes during the next render.

Implement proposed recovery tools as bounded operations with real evidence: content recache, identified failed-job/temporary-file cleanup, and recent changes to supported settings/metadata. A cleanup preview names targets and exclusions; applying it checks the preview is still current. An undo action is available only where the backend can reverse that specific change without losing later work. This does not require a universal transaction-history framework.

Backup scopes must match actual archive contents. The existing database restore is staged and applied on the next application start; do not label it an immediate restoration of the entire club. Recordings/media are not implicitly included in a database or content backup. Either implement an explicit media export or state that scope is unavailable.

## 6. Visual and interaction contract

- Preserve the concept’s warm neutrals, olive ink, terracotta actions, Manrope headings, DM Sans body, restrained borders, generous spacing, and confident plain-language headings. Extract reusable semantic tokens; avoid a generic dense administration dashboard or a gratuitous new design system.
- Match the main hierarchy and composition, then refine spacing for real content. Today is welcoming; operating views prioritize timing and decisions; Watch is a dark viewing surface. Both day and night themes need deliberate contrast.
- Use a single action hierarchy and shared primitives for buttons, fields, status, dialogs/sheets, empty/loading/error states, page headers, and scoped notices. Extend existing `components/ui/*` where practical.
- Use real track/car imagery and authoritative layout maps from existing APIs. Do not reuse the sample Silvia image as every production car’s photograph. Provide useful missing-image and missing-metadata states.
- Import the original six files under `assets/screen-setups/*-transp.png` into the production asset pipeline without regeneration or recoloring. The mapping is `triple → triple-screens`, `single → single-screen`, `wide → wide-screen`, `ultrawide → ultrawide`, `vr → vr`, `custom → custom`, all with `-transp.png`. Use one shared setup-art component in Edit/Add rig, rig cards, rig details, and Garage. See the latest `rigArt()` and `.rig-art` styling for framing. Keep decorative images empty-alt when adjacent text already names the setup.
- Do not ship the study bar, Reset demo, sample badges, fake health checks, fake clock, hardcoded people, simulated moving map markers, or prototype record/download behavior.
- Use helpful defaults from installed content and actual capabilities. New session offers Drift, Race, or Practice; track, cars, format, conditions, and timing expand when changed. Advanced options remain reachable in the relevant section.
- Review or confirm consequential actions with exact scope, effect, restart/disconnection consequences, and affected records. Routine reversible edits should not require repeated confirmation. Success reflects a backend result; failures retain useful input.
- Keyboard navigation, visible focus, labelled controls, focus restoration, Escape, dialog scroll, reduced motion, touch targets around 44 px, readable units, and contrast are required. Status cannot rely on color alone. The prototype’s smallest fonts are not a reason to ship unreadable controls.

### Watch fullscreen requirements

The current reference is `docs/pitlane-concept/watch.css`, whose stylesheet is versioned `v=9` in the prototype. At viewport widths of at least 1440 CSS pixels in fullscreen:

- Camera and map use equal-width columns, with matching panel heights of `clamp(360px, 48vh, 900px)` as the starting reference.
- The actual map scales to its panel while preserving projection and aspect ratio. Driver markers scale for legibility. Never restore the old 350 px map cap next to an unbounded video.
- Preserve the entire video image with appropriate letterboxing; do not stretch or crop away useful driving information. Apply the layout to actual video/embed wrappers, not only `<img>`.
- Secondary camera tiles use the available row space; timing and camera selection remain easy to find. Native fullscreen survives driver/feed selection and reactive updates; include a clear exit and handle `fullscreenchange`/Escape.
- Preserve the stacked mobile layout. Do not infer a 4K viewport from device-pixel ratio: test both 3840×2160 CSS pixels and common scaled-monitor viewports such as 1920×1080 and 2560×1440.

Measured prototype reference: at 3840×2160, stream/map panels are both about 1869×900, with the Rudskogen canvas about 529×836 instead of 196×310. At 2560×1440 they are about 1229×691; at 1920×1080, 909×518; at 1440×900, 669×432. These are composition targets, not brittle pixel assertions for a different production shell. The old 4K stream was approximately 2408×1355 next to a 350 px map panel.

## 7. Implementation milestones

Build vertical slices with real data. Each milestone must leave the app runnable and record its checks, unresolved dependencies, and exact next step. The complete request is all milestones, not just the first polished page.

### M0 — Baseline and contract decisions

- Read the sources above, inspect the current worktree, and establish frontend test/build results before editing. Do not change existing staged or unrelated files.
- Create `docs/PITLANE_IMPLEMENTATION_PROGRESS.md` containing milestone status, route compatibility map, API/schema decisions, risks, and validation evidence.
- Create a parity checklist that maps every current route and all 53 Advanced tools to a production destination, backend handler/capability, permission, and acceptance check. Use states such as existing, integration needed, backend addition needed, verified; never equate a rendered button with verification.
- Resolve durable session/execution identity, preset ownership, source/rig migration, exact-start behavior, and role mapping before building pages that depend on them.

**Exit:** a concrete implementation sequence and baseline; every original surface has an intended home. No production data has been altered.

### M1 — Pitlane shell and working navigation

- Build the shared tokens/primitives, responsive navigation, themes, auth/account entry, route scaffolding, and Garage hierarchy. Port the latest setup artwork.
- Connect Today to real instance/readiness/activity data. Implement honest empty, loading, partial-error, and unauthorized states.
- Add compatibility routes while existing operational pages remain available. Maintain one SSE subscription/recovery path.

**Exit:** signed-in users can navigate without dead ends; role-gated operations remain guarded; day/night and phone/desktop layouts visibly match Pitlane’s direction.

### M2 — Complete session creation through exact start

- Implement the durable aggregate and additive migrations, isolated setup persistence, and proposed launch operation from §5.
- Build experience selection → ready setup → expand track/cars/format/conditions → readiness review → start now/later/after current. Reuse the renderer and preset fields, including all supported advanced values.
- Preserve drafts through content import, navigation, validation, and failed requests. Use explicit save/recovery behavior; avoid persisting unfinished preset records as a side effect of opening an editor.
- Implement authoritative scheduling/reservation and conflict handling. Retain complete manual queue, repeat, and boot behavior.

**Exit:** selecting a session starts that exact session once, without interrupting another server or changing shared templates; invalid/unsupported configurations explain the remedy; refresh/retry cannot enqueue duplicates.

### M3 — Session operation, finishing, and real recaps

- Rebuild Session control around timing/drift board, phase, driver actions, Next driver, and Up next. Preserve stop/restart, skip, next/restart phase, chat/admin commands, kick, and live setup edits with precise restart effects.
- Complete queue ordering, removal, clear-completed, category enqueue, repeat freeze/unfreeze, and per-instance targeting.
- Wire lifecycle/phase/result correlation, handover boundaries, capture finalization, finish → recap, and Run it again. Recaps and live metrics must come from persisted/authoritative data.

**Exit:** an entire driving evening survives refresh and ends in the correct history. No cross-instance action or previous-driver attribution leak.

### M4 — Rigs, source migration, Watch, and all-stream Live

- Add rig/gear persistence and source-independent identities/health before claiming a complete live wall.
- Build rigs list/detail/Edit, standalone camera management, contextual stream setup/check/save from Watch and Live, and optional recording settings with direct diagnostics.
- Build Watch using existing projection/interpolation/ranking and WHEP/embed/capture implementations. Synchronize camera, driver, map marker, and board; handle spectators, missing feeds, stale telemetry, off-map positions, missing geometry, and no drivers.
- Implement all-source Live independent of game presence, fullscreen, audio/decoder discipline, and the large-screen contract above. Preserve the club leaderboard and automatic broadcast/auto-follow behavior in accessible destinations.

**Exit:** a connected idle rig can actually play in Live; all sessions use the correct map/layout; missing video never removes valid timing/position data; capture health is independent; fullscreen retains balance and selection at 4K.

### M5 — Drivers, detailed history, and media

- Apply one coherent profile layout to account drivers and guests while preserving identity and authorization differences.
- Migrate complete trends/statistics, avatar upload, favorites where supported, connections/stints/phases/laps/drift runs, tags/search filters, result files, clips/snapshots, playback/download/delete, and assignment controls.
- Keep all-server records, filters, clip links, and fullscreen leaderboard accessible. Integrate historical correction separately from live handover.

**Exit:** existing records/media remain reachable by old links and searchable in the new UI; repeated/shared-account sessions retain correct ownership and scoring semantics.

### M6 — Complete Cars & tracks workspace

- Port the full library: cars/skins/specifications, tracks/layouts/capacity/maps, weather/CSP, filtering, searching, details, and correct downloads.
- Integrate file/URL/batch import, kind detection, overwrite review, progress, reconnect/retry, and return to the invoking session picker without losing the draft.
- Add persistent local metadata/archive and authoritative usage; keep recache and actual disk deletion distinct, with the original lifecycle/dependency restrictions.

**Exit:** all installed content is manageable, jobs remain visible across navigation/reconnect, archiving is reversible, and no layout or image is silently substituted.

### M7 — Advanced parity and bounded recovery

- Integrate all catalogue entries into the 14 sections listed below; search must find nested setting labels as well as tool names. Reuse the same domain editors/actions as daily pages where the operation is the same.
- Scope every operation accurately: global installation/config, selected server, session draft, shared preset, person, source, or system data. A persistent server selector must not imply a global setting affects only that server.
- Preserve all original detailed fields and operations. Implement the specified custom/recovery extensions with persisted contracts and real validation, not generic form-to-JSON success stubs.
- Make diagnostics report measured evidence, timestamps, and a remedy. Offer recent-change review/undo only for supported reversible changes. Preserve staged restore semantics and truthful backup contents.

**Exit:** every parity checklist row names a tested production path; no missing feature is hidden behind an “Applied” toast or a link to a mock.

### M8 — Integration, polish, and cutover

- Run the acceptance suite below, resolve regressions, and perform visual comparison against the concept at matched sizes with representative real data.
- Verify clean install and upgrade from an existing SQLite fixture, repeated migration, legacy bookmarks, auth/session expiry, cached-tab recovery, and the built embedded SPA.
- Document the rollback boundary: which previous UI/backend build can read the upgraded database, which new data it cannot display, and how to preserve that data. Prefer additive, backward-compatible migrations; never drop new records or restore an old database merely to roll back presentation. Test the proposed rollback against fixtures.
- Remove superseded presentation only after parity gates pass. Keep necessary compatibility adapters. Keep the concept available as a design reference.
- Finish the progress/parity records and provide changed areas, exact commands/results, screenshots, migration notes, and remaining external verification needs.

**Exit:** the real application opens into Pitlane, every requested core flow works, and original capability is retained. A documented blocker is an incomplete item, not a waiver of the completion gate.

## 8. Advanced parity checklist categories

Use [the 53-tool catalogue](pitlane-concept/ADVANCED-COVERAGE.md#catalogue) for individual rows. At minimum, verify these families:

| Section | Production coverage required |
| --- | --- |
| Fix a problem | Readiness/start blockers, telemetry/map diagnosis, playback/recorder diagnosis, missing content. |
| Installation & engine | Installation validation/discovery, Kunos/AssettoServer behavior and installation, CSP requirements/versions/physics. |
| Server configuration | Lobby identity/joining/passwords, entry locking, capacities, network settings, welcome/download behavior. |
| Servers & ports | Instance CRUD, all port types/conflicts, last/running-instance restrictions, spectator slot, per-server driving/scoring rules. |
| Custom sessions | Events/groups, renderer/INI inspection and validated custom drafts, running controls and live overrides. |
| Presets & scoring | All four preset families, ordered weather, skins/ballast/restrictor/grid, drift modes, usage/share/copy/delete semantics. |
| Queue & automation | Order/add/remove/skip/clear/group, repeat and frozen queue, schedules, boot settings. |
| Streams & recording | Rig/standalone/spectator sources, independent health, raw recording sources, buffers/probes/triggers/cooldowns/limits. |
| Content maintenance | Full browse/metadata, imports/jobs/overwrite, index/image operations, usage checks and disk deletion. |
| People & access | SM accounts and roles, password resets, last-admin/self protections; distinct guest roster permissions. |
| Your preferences | Current account/password, measurement and temperature units, appearance/sign-out. |
| History & attribution | Stint/results search, full profiles/media/tags, ownership correction, records and spectator display. |
| Data & recovery | Accurate database/content/media export scope, restore staging/restart, bounded failed-job/temp cleanup. |
| System & logs | Version/paths/logs, timestamped telemetry evidence, supported recent changes and undo. |

The catalogue’s 180 labelled fields are a navigation/design inventory, not an API schema. Validate field names, enums, units, limits, nullability, and scope against the actual backend. Some source pages have more fields or behavior than the mock; preserve them too.

## 9. Acceptance and verification

### Required scenarios

| Scenario | Passing result |
| --- | --- |
| First use, installed content, incomplete setup | Only necessary questions; actual blockers lead to their remedy; no fake discovered installation. |
| Drift, Practice, Race; current server busy | Complete valid defaults, correct scoring/engine behavior, other session stays running, exact reviewed execution starts. |
| Double click, lost response, refresh, simultaneous operators | One operation/execution; safe reservation/conflict result; reliable retry/recovery. |
| Shared preset used by two sessions | Ordinary edits change only this session; explicit shared edit identifies actual dependents. |
| Invalid grid, missing layout/car, unsupported CSP | Backend rejects with actionable detail; no silent trimming/substitution presented as the intended setup. |
| Schedule, timezone/DST, overrun, repeat/manual queue | Correct target/order and honest start time; preserved queue on repeat; no silent interruption. |
| Two running instances and reconnect/background tab | State hydrates without manual reload, commands reach the selected instance, stale data is labelled. |
| Idle rig with healthy feed; standalone spectator | Visible/playable in Live without a game driver; correct session filters and independent source status. |
| Camera lost / telemetry stale / recorder unavailable | Three distinct states; working channels remain usable; contextual remedies. |
| Map across multiple actual layouts | Correct world projection, interpolation, off-map behavior, driver focus and race/drift ordering; no invented time gaps. |
| Handover during a run or recording | Prior results and in-flight media keep prior owner; next segment uses new owner; errors do not falsely update assignment. |
| 4K fullscreen and driver/source selection | Equal visual panels, bounded video, legible larger map, no exit/remount on selection, functional controls and Escape. |
| Rig CRUD, six displays, gear, multiple cameras | Persists through refresh; transparent supplied art everywhere; usual driver distinct from current occupant. |
| Content import with failure/retry/overwrite; archive/recache/delete | Accurate progress/usage, preserved drafts/metadata, explicit destructive scope, no unintended file deletion. |
| Viewer, steward, admin, expired login | UI and backend agree; no privileged source diagnostics/credentials exposed; correct redirect-back. |
| History/recap/media and old bookmarks | Correct IDs, owner, time, units, filters and files; legacy records remain available. |
| Backup/restore/cleanup/undo | Exact scope, real results, staged restore timing, concurrency-safe undo only where supported. |
| Existing database upgraded twice | Existing settings, presets, queues, users, guests, history, URLs and media preserved; migration is idempotent. |

Test the normal layout at 375, 768, 1024, and 1440 CSS-pixel widths, plus Watch fullscreen at 1440×900, 1920×1080, 2560×1440, and 3840×2160. Cover both themes, long names, empty libraries, many rigs/feeds, large driver lists, reduced motion, keyboard-only navigation, and dialog scrolling. Use viewport measurements as well as screenshots; verify the actual fullscreen element and loaded stylesheet. Do not mistake screenshot stitching artifacts or browser cache for the application layout.

### Commands and environments

From the repository root, establish and later repeat the relevant frontend checks:

```sh
npm --prefix webapp test
npm --prefix webapp run build
git diff --check
```

Extend existing Vitest suites around changed behavior: router/auth, server recovery, queue state, setup drafts/preview, telemetry math, dialogs, and user flows. Add focused backend tests for new migrations, exact-start idempotency/concurrency, source health, ownership boundaries, and recovery validation. CSS changes need browser layout checks; mocked network tests do not prove real WHEP playback or game-server launch.

`make test` currently builds the SPA, runs frontend tests, and runs Go vet; it does **not** run Go unit tests. Where an appropriate Go toolchain is available, run `go test ./...` and `go vet ./...` from `src/` after the SPA has been built. Repository guidance describes a Docker-based workflow and an environment without local Go; do not waste time searching for a binary. Use the configured Docker service when available, or report the exact external check that remains.

For an already-running approved development container, commands are:

```sh
docker compose -f docker-compose.dev.yml exec -T servermanager-dev sh -lc 'cd /go/src/app/src && go test ./...'
docker compose -f docker-compose.dev.yml exec -T servermanager-dev sh -lc 'cd /go/src/app/src && go vet ./...'
```

Inspect its state before using it. The existing dev Compose mounts `data/` and game content read/write; its `-debug` flag does not make those volumes disposable. For migration, launch, handover, restore, and deletion tests use a separate fixture database/content directory and isolated instance/ports. Do not run `make rundebug-docker-cold` or remove volumes as a testing shortcut. Do not stop an occupied game server to verify a redesign.

For visual development, `make webapp-dev` serves Vite on the configured development port (normally 5173) and proxies `/api` and `/static` to the Go service on 3030. The concept on 8786 is separate. Test the built app through Go too, including direct route reloads, assets, and download/media links. Check whether additional development proxy paths are needed for the existing `/dl/*` flow.

`node docs/pitlane-concept/check.cjs` validates only the standalone mock. It is useful to understand intended interactions, but never counts as production integration evidence.

## 10. Completion and handover discipline

During implementation keep `docs/PITLANE_IMPLEMENTATION_PROGRESS.md` current with decisions, completed milestones, precise blockers, modified contracts/migrations, parity status, and actual test results. Continue independent work when a simulator, recorder, or toolchain is unavailable. Do not report live playback/launch/capture as verified unless it was observed against a suitable real or controlled integration environment.

The final deliverable must include:

- The production Pitlane frontend and necessary backend/persistence changes, not modifications only to the mock.
- Completed route/capability mapping, migration and compatibility notes, and honest validation evidence.
- A runnable preview and representative screenshots of Today, creation/control, Watch, Live, Drivers, Garage, rig Edit/detail, library, and Advanced, including large-screen Watch and phone layouts.
- Clear explanation of any incomplete externally blocked checks. No silent exclusions of old features, fabricated successful operations, or blanket “all tested” claims.

Use [PITLANE_HANDOVER_PROMPT.md](PITLANE_HANDOVER_PROMPT.md) to start the implementation agent with this context.
