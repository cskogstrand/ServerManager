# Live Spectator Stream via External GPU Stream Host

## Summary

- Add live video support by embedding a WebRTC player in Server Manager, with the actual Assetto Corsa rendering done on a separate GPU PC/VM.
- Do not attempt to render video inside the current Docker container; this repo only runs the headless dedicated server/AssettoServer process.
- First version is "embed/status only": SM stores stream URLs, shows stream health, and optionally reserves a spectator slot. Starting AC/OBS/MediaMTX on the stream PC stays outside SM.

## Key Changes

- Extend `server_instance` with stream/spectator fields: enabled flag, WebRTC embed URL, optional status URL, spectator enabled flag, spectator driver name, GUID, car key, and skin key.
- Update the instance APIs/types so `/api/instances`, instance create, and instance update include the new fields. Add `GET /api/instances/:id/stream/status` for optional health checks with short timeout, no credential forwarding, and strict `http/https` URL validation.
- Update config rendering in `src/configrenderer.go` and `src/embed/ini/entry_list.ini` so a configured spectator slot is appended with `SPECTATOR_MODE=1`.
- Reserve capacity safely: if spectator mode is enabled, keep total `MAX_CLIENTS` within configured max and track pitboxes by reducing normal race slots by one. If no race slot remains, block server start with a clear error.
- Update the Vue app:
  - Add a Stream/Spectator section to instance settings.
  - Add a 16:9 stream panel on the dashboard for configured instances, using the WebRTC embed URL in an iframe plus an "Open stream" fallback link.
  - Show health as `Live`, `Offline`, `Unknown`, or `Not configured`.

## External Stream Host

- The stream PC/VM runs the full Assetto Corsa client, joins the configured server as the spectator account/slot, and uses AC camera controls to spectate drivers.
- OBS, MediaMTX, or an equivalent WebRTC publisher exposes a browser-playable WebRTC page or WHEP/player URL.
- The stream URL must be reachable from the browser. If SM is served over HTTPS, the stream player must also be HTTPS/WSS to avoid mixed-content blocking.

## Test Plan

- Add Go tests for spectator entry generation: disabled output unchanged, enabled slot appended, max clients adjusted, and no-capacity start fails clearly.
- Add API tests for instance stream fields, URL validation, default migrations, and stream status timeout/error handling.
- Add frontend tests for settings save/load and dashboard states: unconfigured, configured unknown, live, offline, iframe fallback.
- Run `make test` and manually verify one configured stream instance in the dashboard with a mocked/local stream player URL.

## Assumptions

- Target is actual video, WebRTC playback, separate GPU PC/VM, and embed/status only.
- SM will not launch AC, select cameras, run OBS, or relay media in this first version.
- The spectator slot is for allowing the stream client into locked-entry servers; camera behavior remains controlled on the stream PC.
