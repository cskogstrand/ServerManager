# Keep the capability. Simplify the path.

The first concept made Watch too shallow. Simulator video, driver positions, and capture controls are core parts of this app. They should be available together, with stream setup reachable from the place where someone discovers a missing feed.

This comparison is against the current repository implementation, including its broadcast, streaming, driver, queue, content, and administration screens. It is a feature and interaction review, not an operational test of real simulators.

## Incorporated in the interactive concept

| Capability | New home and behaviour | Prototype coverage |
| --- | --- | --- |
| Simulator streams while watching a session | **Session → Watch**. A featured camera, the session's camera tiles, a track map, and timing share one screen. | Interactive camera selection; labelled sample frames. |
| See all connected feeds | **Live** in the main navigation. Starts with connected sources; includes cameras with no active driver/session. Filter by session or show all sources to diagnose missing feeds. | Five initially connected sources, one disconnected source, and an idle simulator. |
| Configure a stream where it is needed | **Watch → Set up streams**, or **Stream setup** beside the selected camera. The same manager is available in Live and Garage. | Add/edit a name, player URL, driver association, optional spectator session, and optional recording source. Validate fields, simulate a connection check, then save locally. No URLs are contacted. |
| See driver positions on the actual circuit | **Watch → On the circuit**. Numbered markers correspond to timing rows. Selecting a marker, driver, or camera synchronises the other views. | Moving sample positions use the installed track's `map.png`, `map.ini`, and sampled `fast_lane.ai` coordinates for Rudskogen, Ebisu Minami, and Drift Playground. These are simulated positions, not live telemetry. |
| Follow a driver | Select their timing row or map marker. Keep their position visible even without a configured camera. | Camera, selected marker, and speed/gear/RPM stay associated. Selecting a spectator camera clears driver telemetry. |
| Distinguish camera loss from telemetry loss | “No signal” belongs to the camera. “Position updates delayed” belongs to the map and instruments. | Disconnected-camera and missing-stream states; a telemetry-delay scenario holds positions independently. |
| Fixed spectator / trackside feed | Link a camera to a session without assigning a driver. | Selectable alongside simulator cameras. Reserving the actual AC spectator slot remains advanced setup work. |
| Take a snapshot or record a clip | **Watch → Snapshot / Record clip → Saved moments**. Capture appears when the source can record; playback alone does not imply recording support. | Simulated captures appear in the moment list and the relevant driver profile. Automatic-highlight preference is modelled, but no media file is created. |
| Shared simulator attribution | **Session control → Next driver**. The camera stays attached to Rig 01 while the displayed identity changes. | An active demo recording is saved to the previous driver and stopped at handover. Finishing a session also saves its outstanding demo clips. Production still needs an authoritative run boundary. |
| Larger spectator view | **Watch → Fullscreen**, or enlarge a tile from Live. | Native fullscreen request for Watch; an enlarged sample frame for a single feed. |

The deliberately simple setup is **name + player link → check → save**. Driver association places the feed into the right session automatically. Recording opens as an optional section because it may require a different source. In production, connection results must come from actual playback/recorder health, and only administrators should see or edit source credentials.

## Remaining gaps that should survive the redesign

These are recommendations for the full frontend, **not features already implemented by this mock**.

| Priority | Existing capability | Where I would put it |
| --- | --- | --- |
| Before replacement | Per-server queue reordering, skip/remove/clear, group enqueue, scheduled starts, auto-start, repeat mode with a frozen manual queue | **Session control → Up next** and **Sessions → Plan**. Expand the plan into an ordered list. “Repeat this session” shows that queued sessions are waiting and provides a clear return to the queue. The mock currently demonstrates basic scheduling only. |
| Before replacement | Restart/stop operations, next phase, live settings, commands, and their restart consequences | **Session control**. Keep common actions visible; place rare commands in More controls. State precisely what restarts or disconnects drivers. The mock covers only part of this operational set. |
| Before replacement | Telemetry interpolation, missing map/layout metadata, off-map direction/distance, accurate race ordering, live/best/last drift metrics, lap counts and gaps | **Watch** and the live timing board. Preserve the current telemetry logic rather than deriving real positions or timing from the prototype. Never manufacture second-based race gaps from normalised track position alone. The mock demonstrates linked positions and a subset of metrics. |
| Before replacement | All-server standings with discipline, track, car, recency filters and clip playback | **Live → Club records**, also reachable from **Drivers**. Keep a dedicated fullscreen leaderboard for the second screen. A session recap is insufficient for this use. |
| Before replacement | Searchable driver stints, tags, individual drift runs, detailed result history, media links, and historical guest attribution corrections | **Sessions → Search drives**, with results opening the correct person's stint. Keep “Who was driving?” correction separate from “Next driver”: one repairs history, the other changes future ownership. |
| Before replacement | Driver and guest profiles with trends, favourites, avatar uploads, complete history, media download/delete and assignment controls | **Drivers → person**. One consistent profile layout, retaining the different account/guest permissions and identifiers. The concept's profile is currently a sketch plus the new demo captures. |
| Before replacement | Recorder availability, rolling buffers, stale segments, source probes, logs, capture triggers, cooldowns, clip limits | **Stream setup → Recording → Diagnose**. Put an actionable summary beside the failed feed; expand the existing technical evidence when needed. A green camera badge must never stand in for recorder health. |
| Before replacement | Complete track/car/weather library, layout metadata, skins, vehicle specifications, filters, archive/URL batch import, progress/retry, overwrite and deletion | **Garage → Cars & tracks**, with a contextual picker during session setup. A simple “Add content” entry can preserve the full import workflow underneath. The mock imports one predefined sample pack. |
| Before replacement | Detailed phase, weather/time, difficulty, car-class, grid, drift scoring and preset configuration | Expand the relevant **session setup section**. Keep reusable collections/templates in Garage or the session library. Clearly distinguish editing this session from editing a shared template; show exact generated configuration and readiness details on demand. |
| Before replacement | Installation and server CRUD, engine/CSP compatibility, port assignment, credentials, run modes and boot behaviour | **Garage → Servers → a server**. Summarise readiness; expose the actual settings underneath. The mock shows connection details only. |
| Before replacement | Login, admin/steward/viewer boundaries, account preferences, units, password and user management | Account menu for personal preferences; **Garage → Access** for administration. Viewers see Watch and records; hosts see operating actions; admins manage stream configuration and credentials. The current mock presents the administrator's view. |
| Before replacement | Backups with exact scope, staged database restore and restart timing | **Garage → Backup & recovery**. Keep settings/database, content, and recordings explicit. Existing restore stages a database for the next restart; the design must not imply an instant complete restore. |
| Valuable follow-up | Automatic broadcast follows the busiest server and falls back to the club leaderboard when no drivers are connected | **Live → Club display**. Offer “Follow the action” with a visible current-session label and a manual pin. The new mock uses explicit session selection. |
| Valuable follow-up | A television view that stays useful without an operator | A compact display mode with timing and map readable from a distance, optional camera rotation, and a clear exit. In production keep stream-wall audio muted by default and allow audio from only the selected feed. The mock's fullscreen page still includes host controls. |

## Three underlying decisions to get right

1. **A camera is a source, a simulator is a seat, and a driver is a person.** Keep those associations explicit. The current stream plumbing primarily keys by driver GUID and uses driver presence to permit playback; its health poll is scoped to connected drivers on one server. The proposed all-camera wall needs source health independent of game presence, plus a durable rig-to-driver handover model. This is more than adding a route.
2. **A session needs durable ownership of its settings, phases, stints, results, and media.** The current event, queue item, game phase, and driver-session records have different meanings. Giving them one visual container should not erase those distinctions or break historical attribution.
3. **Video, telemetry, and recording can each fail independently.** Preserve separate truth for each and show the relevant remedy. Use the existing WHEP player/embed implementations, actual map metadata and interpolation, and recorder checks. The concept's successful check is deliberately only a simulation.

## Source trail

- [Broadcast.vue](../../webapp/src/pages/Broadcast.vue): camera/card/map coordination, telemetry instruments, capture controls, off-map indicators, fullscreen and auto-follow.
- [raceTelemetry.ts](../../webapp/src/lib/raceTelemetry.ts): world-to-map projection, interpolation, ranking and gap semantics.
- [useDriverStreams.ts](../../webapp/src/lib/useDriverStreams.ts), [StreamWall.vue](../../webapp/src/components/StreamWall.vue), [StreamTheater.vue](../../webapp/src/components/StreamTheater.vue): stream identity, health, driver presence, playback and enlargement.
- [StreamingCapture.vue](../../webapp/src/pages/StreamingCapture.vue), [StreamsDebug.vue](../../webapp/src/pages/StreamsDebug.vue): source configuration, spectator slots, capture options and diagnostics.
- [Queue.vue](../../webapp/src/pages/Queue.vue), [ServerDetail.vue](../../webapp/src/pages/ServerDetail.vue): per-instance operation and queue/repeat behaviour.
- [BroadcastLeaderboard.vue](../../webapp/src/components/BroadcastLeaderboard.vue), [SessionSearch.vue](../../webapp/src/pages/SessionSearch.vue): club comparisons and historical discovery.
- [DriverDetail.vue](../../webapp/src/pages/DriverDetail.vue), [GuestDriverDetail.vue](../../webapp/src/pages/GuestDriverDetail.vue): detailed profile, media and attribution workflows.
- [Content.vue](../../webapp/src/pages/Content.vue), [Maintenance.vue](../../webapp/src/pages/Maintenance.vue), [router.ts](../../webapp/src/router.ts): content operations, recovery semantics, account/admin surfaces and route access boundaries.
