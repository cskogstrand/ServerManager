# Fix Frontend Live Update Drift

## Summary
- Existing frontend tests pass: `npm run test --prefix webapp` ran 10 tests successfully.
- The store does update dynamically when an event reaches an existing instance.
- The refresh problem is still plausible because startup/reconnect can miss full state:
    - [App.vue](/Users/cskogstrand/privateprojects/ServerManager/webapp/src/App.vue:24) calls `server.load()` without awaiting it, then immediately opens SSE.
    - [server.ts](/Users/cskogstrand/privateprojects/ServerManager/webapp/src/stores/server.ts:201) drops snapshots/events if the instance is not loaded yet.
    - Page `detail` payloads and Pinia live state are only partially synced, so some UI can remain stale until refresh.

## Key Changes
- Add a store-level `bootstrapLiveState()` action:
    - Await `/api/instances`.
    - Fetch `/api/server/status?instance={id}` for each instance.
    - Sync `running`, `players`, `session`, `drivers`, `positions`, and `telemetry` into the store.
    - Clear `session`, `drivers`, `positions`, and online telemetry when an instance is stopped.
- Update [App.vue](/Users/cskogstrand/privateprojects/ServerManager/webapp/src/App.vue:20):
    - On login/session restore, `await server.bootstrapLiveState()` before `server.connect()`.
    - On SSE reconnect, run the same live-state refresh so missed events self-heal.
- Harden [server.ts](/Users/cskogstrand/privateprojects/ServerManager/webapp/src/stores/server.ts:198):
    - Do not silently lose unknown-instance snapshots; trigger a throttled bootstrap refresh.
    - Keep existing event-specific updates for low-latency `players`, `drivers`, `positions`, `session`, and `telemetry`.
- Update Server Detail and Dashboard:
    - Server Detail `fetchDetail()` must sync full live store fields, not only `drivers`, `positions`, and `telemetry`.
    - Use store `inst.session` for live session tiles/timeline where possible, with `detail.session` only as fallback.
    - Dashboard should render live session/player values from the store and keep `details` only for event metadata/preview/public IP.
- Add a low-rate recovery refresh:
    - On browser `visibilitychange` back to visible.
    - Every 15 seconds while at least one server is running.
    - This recovers dropped one-time `drivers`/`players` events without returning to heavy polling.

## Test Plan
- Add store tests for:
    - Bootstrap loads instances then hydrates full status.
    - Stopped instances clear stale drivers/positions.
    - Unknown-instance snapshot triggers one throttled refresh instead of being permanently lost.
    - `refreshInstanceStatus()` updates `players`, `drivers`, `positions`, and `telemetry`.
- Add page/store integration coverage for Server Detail using hydrated store state.
- Manual check:
    - Open app with a server already running and a driver connected; no refresh should be needed.
    - Disconnect/reconnect SSE or background the tab, then return; driver count and map should recover.
    - Join/leave a driver and verify player count, timing table, and map dot update live.

## Assumptions
- No backend SSE wire-shape change is required beyond the current `snapshot`, `players`, `drivers`, `positions`, `session`, and `telemetry` events.
- `/api/server/status` remains the authoritative fallback snapshot for recovering missed frontend events.
