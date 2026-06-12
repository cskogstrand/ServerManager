# Live Spectator Stream via External GPU Stream Host

## Summary

- Add live video support by embedding WebRTC players in Server Manager, with rendering/publishing done outside the headless dedicated-server container.
- Do not attempt to render video inside the current Docker container; this repo only runs the headless dedicated server/AssettoServer process.
- First version is "embed/status only": SM stores stream URLs, shows stream health, and optionally reserves a spectator slot. Starting AC/OBS/MediaMTX on stream PCs stays outside SM.
- Support both a fixed spectator/camera stream and optional streams published by current drivers.

## Key Changes

- Extend `server_instance` with stream/spectator fields: enabled flag, WebRTC embed URL, optional status URL, spectator enabled flag, spectator driver name, GUID, car key, and skin key.
- Update the instance APIs/types so `/api/instances`, instance create, and instance update include the new fields. Add `GET /api/instances/:id/stream/status` for optional health checks with short timeout, no credential forwarding, and strict `http/https` URL validation.
- Add a per-driver stream mapping keyed by driver GUID, with display name, enabled flag, WebRTC embed URL, and optional status URL. Active driver rows in the dashboard should show a watch action only when a mapped stream exists for that connected GUID.
- Add CRUD endpoints for driver stream mappings, plus `GET /api/instances/:id/driver-streams/status` for batch health checks of currently connected drivers.
- Update config rendering in `src/configrenderer.go` and `src/embed/ini/entry_list.ini` so a configured spectator slot is appended with `SPECTATOR_MODE=1`.
- Reserve capacity safely: if spectator mode is enabled, keep total `MAX_CLIENTS` within configured max and track pitboxes by reducing normal race slots by one. If no race slot remains, block server start with a clear error.
- Update the Vue app:
  - Add a Stream/Spectator section to instance settings.
  - Add a 16:9 stream panel on the dashboard for configured instances, using the WebRTC embed URL in an iframe plus an "Open stream" fallback link.
  - Add a Driver Streams management section where admins can register stream URLs by driver GUID.
  - Add watch buttons to connected drivers in Live Timing and open driver streams in an inline dashboard panel or modal.
  - Show health as `Live`, `Offline`, `Unknown`, or `Not configured`.

## External Stream Host

- The stream PC/VM runs the full Assetto Corsa client, joins the configured server as the spectator account/slot, and uses AC camera controls to spectate drivers.
- Driver PCs may also publish their own cockpit/chase-cam feeds to the same WebRTC infrastructure.
- OBS, MediaMTX, or an equivalent WebRTC publisher exposes browser-playable WebRTC pages or WHEP/player URLs.
- The stream URL must be reachable from the browser. If SM is served over HTTPS, the stream player must also be HTTPS/WSS to avoid mixed-content blocking.

## Test Plan

- Add Go tests for spectator entry generation: disabled output unchanged, enabled slot appended, max clients adjusted, and no-capacity start fails clearly.
- Add API tests for instance stream fields, driver stream mappings, URL validation, default migrations, and stream status timeout/error handling.
- Add frontend tests for settings save/load and dashboard states: unconfigured, configured unknown, live, offline, iframe fallback, and driver stream watch actions.
- Run `make test` and manually verify one configured spectator stream plus one connected-driver stream in the dashboard with mocked/local stream player URLs.

## Assumptions

- Target is actual video, WebRTC playback, separate GPU/driver PCs, and embed/status only.
- SM will not launch AC, select cameras, run OBS, or relay media in this first version.
- The spectator slot is for allowing the stream client into locked-entry servers; camera behavior remains controlled on the stream PC.
- Driver streams are matched by GUID so the dashboard only shows a stream action for the connected driver who owns that feed.
