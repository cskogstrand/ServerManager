# Handover prompt for the implementation agent

Copy the prompt below into a new agent working in this repository. The plan contains the detailed scope and acceptance criteria; this prompt establishes how to execute it.

---

Implement the Pitlane UI as the real production frontend for ServerManager.

Repository: `/Users/cskogstrand/privateprojects/ServerManager`

Read and execute `/Users/cskogstrand/privateprojects/ServerManager/docs/PITLANE_IMPLEMENTATION_PLAN.md`. Start by reading the concept README, FEATURE-PARITY.md, and ADVANCED-COVERAGE.md under `docs/pitlane-concept/`, then inspect the actual frontend/backend source and applicable repository guidance. This is an implementation task. Do not stop after producing another plan, a reskin of the homepage, or another standalone mockup.

The product manages Assetto Corsa driving sessions, servers, drivers and guests, shared simulators, live cameras, drift/race results, content, and recorded moments. The design should make running a club feel effortless: strong defaults, obvious next actions, progressive disclosure, and precise explanations of consequences. Preserve the approved Pitlane visual direction—warm surfaces, olive ink, restrained terracotta, clear type and generous spacing—rather than substituting a generic administration dashboard. “Pitlane” is the working interface identity, not an instruction to rename infrastructure.

The standalone concept in `docs/pitlane-concept/` is the interaction and visual reference. The application you must change is the Vue 3/TypeScript SPA in `webapp/`, backed by Go/Gin/SQLite in `src/`. Necessary backend additions are included in the task. Keep the existing stack, authentication, role enforcement, API client, shared live/SSE recovery, and tested telemetry/playback/capture foundations. Follow the repository’s Ponytail guidance: reuse before rebuilding, native mechanisms before dependencies, and no unrelated architectural rewrite.

The required experience is:

- Main navigation **Today · Sessions · Live · Drivers**, with **Garage** secondary.
- One driving-session journey: choose Drift/Race/Practice → receive valid installed-content defaults → review/change only what is needed → start that exact session or schedule/queue it → operate → finish → recap → Run it again.
- Watch with actual camera playback, actual track-layout map and live positions, timing and telemetry linked to the selected driver, contextual stream setup, and real capture controls.
- Live showing all connected simulator and standalone/spectator feeds, even when nobody is connected to a game. Playback, telemetry, and recording health are separate.
- Rigs & cameras list/detail/edit under Garage: persistent rig identity, location, six display arrangements, gear, notes, usual/current-driver distinction, and attached sources. Shared rigs work generally, without hardcoded demo IDs.
- Complete Cars & tracks library with detail/manage pages, layouts/skins/weather, actual imports/progress, metadata, usage, archive and distinct disk deletion.
- Clearly separated Your servers and The club sections, plus searchable **Garage → Advanced** preserving every original capability. Use the 53-tool/14-section catalogue and audit original pages for anything the mock omitted.
- Complete driver/guest profiles, history/search/tags/attribution/media, queue/repeat/schedules, server operations, accounts/preferences, club records/broadcast, backups and staged restore.

Two recent refinements are mandatory:

1. Use the six existing `assets/screen-setups/*-transp.png` files in every screen-setup illustration, including Edit/Add rig, rig cards/detail, and Garage. Use the latest concept source, not older screenshots. Do not regenerate those images.
2. On large fullscreen Watch views, give stream and map equal-width panels and matching, bounded heights. The current reference uses `clamp(360px, 48vh, 900px)` at widths ≥1440 CSS px, a map that grows with its panel, and compact secondary feeds. Validate real video/embed wrappers at 3840×2160 and scaled 4K sizes; keep fullscreen stable when selecting another driver. The old tiny-map/huge-video proportions must not return.

Important engineering traps to resolve, not paper over:

- The current `/sessions` route searches historical driver stints. Preserve old filter links when adding the new session hub. Instance IDs, event IDs, queue IDs, executions, game phases, and driver stints are different identities.
- Current “Queue & start” appends an event and can start an earlier queue item. Implement the plan’s backend-owned exact-start/idempotency/conflict behavior; a chain of frontend requests or an in-memory duplicate guard is insufficient.
- Duplicating an event currently shares preset references. Ordinary session edits must be isolated; shared edits need accurate usage and explicit intent.
- Current stream health/playability is tied to connected driver GUIDs. A live wall independent of game presence requires a real source model/health extension, not just a new route.
- Handover changes future ownership at an authoritative boundary. Preserve previous results and in-flight recording ownership. Historical correction is a separate action.
- The mock’s sample checks, local success messages, simulated media, raw-INI editor, cleanup and undo are not production implementations. Build truthful persisted operations and validation. Never silently claim a source connected, a server started, or data restored.

Work through the plan’s milestones, keeping the app runnable. Maintain `docs/PITLANE_IMPLEMENTATION_PROGRESS.md` with decisions, status, route/feature parity, tests, and concrete blockers/next steps. Inspect existing worktree changes before editing and preserve unrelated/staged work. Existing detailed pages may remain accessible during migration; the finished interface must have coherent Pitlane flows and complete capability, not permanent links into unfinished old UI.

Use judgment on routine implementation decisions and continue autonomously. Do not repeatedly ask for permission for normal reversible repository work. If evidence requires changing a proposed contract, document the reason and preserve the user-facing promise. Do not reduce the scope merely because a backend extension is needed.

Validate using existing frontend tests/builds, targeted regression tests for changed logic, appropriate Go tests/vet in the available environment, and browser checks against real APIs. The plan specifies commands and scenarios. The prototype’s `check.cjs` is not a production test. Existing Docker dev volumes contain real data/content; use isolated fixtures for migration, server-control, restore and deletion tests. Do not interrupt active sessions or publish/deploy changes as an incidental verification step.

Before calling the work complete, exercise the full daily journey and every parity category; inspect desktop/phone layouts in both themes and native fullscreen at 4K. Include authentication/roles, empty/error/stale states, multi-instance isolation, repeat/scheduling, import retry, shared-rig attribution, real streams/capture where available, and legacy deep links. Preserve the Go-embedded production build pipeline and verify built asset/deep-link behavior.

Deliver the implemented application, a runnable preview, representative screenshots, migration notes, completed parity/progress records, and exact validation results. State any external integration checks that remain blocked honestly. A polished mock with missing functionality is not completion. Aim for an interface that is as simple and considered as the concept, backed by the full capability and reliability of the application.
