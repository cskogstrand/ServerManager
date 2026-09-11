# Pitlane production screenshots

Captured on 11 September 2026 from the actual Vue SPA backed by the isolated Go/SQLite installation. These are browser captures, not the standalone concept. Names, drivers and video belong to the disposable fixture. Track imagery is copied installed content; display illustrations use the six existing `assets/screen-setups/*-transp.png` files.

The native Watch capture uses a real local video embed and the installed Drift map. Its positions/timing come through the protocol fixture. The test video is an ffmpeg pattern, not a physical camera or game render. See [verification results](../PITLANE_IMPLEMENTATION_PROGRESS.md) for the separate 4K native-fullscreen measurements and external acceptance limits.

## Today and session creation

Phone, light theme, 375 CSS pixels. Ready-state actions use actual installation/content readiness.

![Today on phone](today-phone-light.jpg)

Desktop session creation, 1440 CSS pixels. Installed defaults, expandable driving settings and the exact-configuration review share one journey.

![Session creation](session-creation-light.jpg)

An idle server with no queued event offers the same session builder, with that server preselected. Missing map/current-event state is explicit.

![Idle session control](session-control-empty-light.jpg)

Finished session with persisted executions, attributed driver results and captured moments.

![Session recap](recap-light.jpg)

## Watch and Live

Native fullscreen, 1440×900 CSS pixels. The video and map each measure 680×432; the layout also passed 1920×1080, 2560×1440 and 3840×2160 measurements. Driver selection kept native fullscreen active. Stale telemetry is labelled separately from source playback.

![Native fullscreen Watch](watch-native-fullscreen-1440.jpg)

Phone, dark theme. An enabled source remains visible without a connected game driver; playback opens on selection.

![Live on phone](live-phone-dark.jpg)

## Drivers and Garage

Driver directory with actual ingested protocol laps and distinct guest identities.

![Drivers](drivers-light.jpg)

Garage, light theme, with equipment, collection and Advanced destinations and separate server/club sections.

![Garage](garage-light.jpg)

Saved rig detail distinguishes usual driver from current game assignment and lists its attached camera.

![Rig detail](rig-detail-light.jpg)

Native rig editor with all six supplied transparent display arrangements.

![Six screen arrangements](add-rig-six-arrangements.jpg)

## Library and Advanced

Track detail in dark theme: actual map/layout, local notes/tags, archive and usage controls.

![Content detail](content-dark.jpg)

Advanced search on a phone in dark theme. Search includes nested field labels across the 53-tool / 14-section catalogue.

![Advanced on phone](advanced-phone-dark.jpg)
