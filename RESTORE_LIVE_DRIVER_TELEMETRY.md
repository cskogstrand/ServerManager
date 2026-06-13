# Restore Live Driver Telemetry

## Summary
- Root cause is upstream of the UI: `players`, `drivers`, and `positions` all come from the AC UDP plugin stream. The detail page already fetches `/api/server/status` and writes returned `drivers`/`positions` into the store.
- The app-generated config enables Kunos UDP plugin ports, but the current runtime is using AssettoServer and there is no `cfg/extra_cfg.yml`. AssettoServer controls features through `extra_cfg.yml`, and its binary exposes `EnableLegacyPluginInterface` for Kunos-compatible UDP plugin support. The app does not set that today.
- A second concrete bug can mask failures: [src/acprocess.go](/Users/cskogstrand/privateprojects/ServerManager/src/acprocess.go:50) starts the server but never waits on process exit, so the UI can show “running” after the server has already crashed.
- I could not inspect live June 13, 2026 telemetry because no Server Manager container was running during inspection, and the local log only had AssettoServer download entries from June 12, 2026. Official AssettoServer docs confirm `extra_cfg.yml` is created/used for feature control and config-error behavior: [intro](https://assettoserver.org/docs/intro/) and [configuration errors](https://assettoserver.org/docs/common-configuration-errors/).

## Key Changes
- Update AssettoServer provisioning in [src/assettoserver.go](/Users/cskogstrand/privateprojects/ServerManager/src/assettoserver.go:450) so a valid full `extra_cfg.yml` is available before launch and always sets:
    - `EnableLegacyPluginInterface: true`
    - existing relaxable `IgnoreConfigurationErrors` keys according to the current “allow mod content” setting.
- Add telemetry health state on each instance:
    - UDP plugin online/offline
    - last UDP packet time
    - last driver event time
    - last position update time
    - configured plugin bind/send ports
- Expose telemetry health from `/api/server/status` and SSE snapshots. Also include `drivers` in SSE snapshots, since [src/events.go](/Users/cskogstrand/privateprojects/ServerManager/src/events.go:106) currently snapshots `running`, `players`, `session`, and `positions` but not `drivers`.
- Fix server lifecycle in [src/acprocess.go](/Users/cskogstrand/privateprojects/ServerManager/src/acprocess.go:50): after `cmd.Start()`, run `cmd.Wait()` in a goroutine, clear `inst.cmd`, set UDP offline, clear players/drivers/positions, publish `running=false`, and log the exit status.
- Update [webapp/src/pages/ServerDetail.vue](/Users/cskogstrand/privateprojects/ServerManager/webapp/src/pages/ServerDetail.vue:581) to show a clear telemetry warning when the server is running but the plugin is offline or no packets have arrived, instead of only showing “no live telemetry.”

## Test Plan
- Backend unit tests:
    - `ensureAssettoServerExtraCfg` seeds/patches a full config and preserves existing keys.
    - `EnableLegacyPluginInterface: true` is present for AssettoServer runs.
    - process-exit handler clears running/player/driver/position state.
- Frontend/store tests:
    - SSE snapshot applies `players`, `drivers`, `positions`, and telemetry health.
    - Server detail renders telemetry warning when running but plugin offline.
- Manual acceptance:
    - Start the AssettoServer practice server, join one driver, verify `/api/server/status?instance=1` shows `players: 1`, one connected driver, and positions updating.
    - Verify SSE emits `positions` repeatedly and the Server Detail map dot moves on “Minecraft World.”
    - Kill or crash the dedicated server and confirm the UI flips to stopped with empty telemetry instead of staying falsely “running.”

## Assumptions
- The June 13, 2026 test was started through Server Manager using the configured `assettoserver` engine.
- If the stock `assetocorsa/server/cfg/server_cfg.ini` was launched manually instead, telemetry would also be absent because that file has `UDP_PLUGIN_LOCAL_PORT=0` and blank `UDP_PLUGIN_ADDRESS`.
