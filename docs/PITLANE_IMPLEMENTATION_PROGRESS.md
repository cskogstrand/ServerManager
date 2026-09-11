# Pitlane implementation progress

Updated 11 September 2026. The production Vue/Go implementation and local fixture checks are complete. Real simulator/CSP/spectator-client and external WHEP interoperability remain unverified external acceptance items; they are not waived or represented as passing.

## Milestones

| Milestone | Implementation / validation |
| --- | --- |
| M0 Baseline/contracts | Complete. Read concept README, FEATURE-PARITY, ADVANCED-COVERAGE, implementation plan and CLAUDE.md. No AGENTS.md found. Preserved the six pre-existing staged/deleted `*-removebg-preview.png` files. Baseline: 24 frontend files / 83 tests and production build passed. |
| M1 Shell/navigation | Complete. Today · Sessions · Live · Drivers, secondary Garage; account/preferences, both themes, mobile navigation, empty/error/role states, shared SSE recovery. Warm concept tokens and supplied transparent artwork. |
| M2 Session creation/exact launch | Complete. Installed defaults, whole-value validation, isolated presets, shared renderer preview, persisted request fingerprints/revisions/reservations, exact queue target, now/after/later, conflicts/retry/start failures and recovery. |
| M3 Operation/recap | Complete. Existing server/queue/phase commands retained; owned live edits create a new private version and restart creates a new execution. Exact ACSP phase/result association; finish/recap/Run again exercised through the browser. Real simulator command effects remain external. |
| M4 Rigs/sources/Watch/Live | Complete locally. Persistent rigs/gear and source identities, optimistic conflicts, usual/current driver, six existing PNGs, source-independent health/playback/capture. Actual local embed, ffmpeg capture, map/telemetry fixture, 4K native fullscreen and driver selection verified. Physical/WHEP sources remain external. |
| M5 Drivers/history/media | Complete locally. Existing full profiles/search/stints/results/media retained; guest profiles readable by viewers, roster mutations role restricted. Authoritative handover blocks motion/active drift, preserves prior segment and in-flight media. Historical corrections remain explicit. Camera galleries reuse playback/download/delete/guest-assignment controls. |
| M6 Content | Complete locally. Complete original file/URL/batch import UI and jobs, cars/specs/curves/skins, track layouts/maps/capacity, weather, local metadata/archive/usage and distinct disk deletion. Real ZIP retry/overwrite/recache/archive/undo/deletion checks passed. |
| M7 Advanced/recovery | Complete locally. 53 tools / 14 sections, nested-label search, explicit scope/roles and source audit. Actual configuration/preset/instance/account/backup/cleanup/undo operations exercised. Restore integrity/staging/application tested in isolated SQLite fixtures. |
| M8 Cutover/verification | Local checks complete: embedded SPA/assets/deep links, original-schema upgrade twice/read compatibility, frontend/backend suites, race detector, vet, build, desktop/phone/themes and native fullscreen. External integration items are listed below. No deployment performed. |

## Contract decisions

- `/sessions` now means a driving aggregate. Old `q`/`tag` bookmarks go to `/sessions/drives` with the full query/hash. `/server/:id` remains an instance ID; connections, phases, executions, queue items and results retain separate identities. `/app/*` preserves raw queries.
- Saved ordinary sessions own immutable values. Shared library usage excludes private snapshots. Library saves explain actual dependents, while duplication creates independent values.
- Exact launch uses backend serialization, SQLite immediate transactions, durable fingerprints/revisions and per-instance reservations. It revalidates actual engine/content/capacity/ports and starts the reviewed queue item. Due plans wait through overruns; automatic starts and legacy operations use the same instance boundary.
- Source identity is independent of GUID or physical rig. Legacy `driver:N` and `spectator:N` settings remain adapters. New `source:N` cameras can play/record while no game is connected. Status response, playable transport, game telemetry and recorder freshness are distinct states.
- Handover preserves old result segments, pins recordings at request time and finalizes them at the boundary. Clip encodes trim to the requested interval. An actual MPEG-TS regression revealed unreliable EOF seeking and MJPEG pixel-format behavior; forward decoded seeking now produces real JPEGs, with a runnable ffmpeg regression.
- Content metadata/archive survive rescans. Shared/execution dependencies protect disk deletion. Cleanup acts only on its unchanged preview. Undo only supports an unchanged latest content-metadata revision. Restores remain staged until restart.
- Original detailed pages share Pitlane tokens, auth, existing editors and the same API; the standalone concept remains a reference only. No fake roster/media/default-score fallback is used when APIs fail. Driver tag/deletion/avatar failures report failure and preserve saved state.
- The rig account picker filters guest records out of the combined driver directory. An unset usual driver correctly displays “Anyone in the club”; selecting a guest uses its separate identity.
- An idle server with an empty manual queue opens the driving-session builder with that server preselected. It no longer asks operators to create shared presets before preparing a private session. Existing queued/repeat start and running controls remain available.
- Explicitly discarding a session clears its browser recovery copy. Only dirty edits are cached; saved sessions do not reappear as a new unsaved draft. Refresh recovery remains available.
- Final embedded-browser checks covered recovery in a fresh tab, discard → new experience selection, save draft → leave without a discard prompt → new experience selection, and the selected-server entry point.

## Exact validation results

| Check | Result |
| --- | --- |
| `npm --prefix webapp test` | 25 files, **87 tests passed**. Existing auth/SSE recovery, user flows, queue, telemetry, forms/dialogs plus source race/error and source-notification routing regressions. |
| `npm --prefix webapp run build` | Passed Vue type checking and Vite production build into the Go embed directory. |
| `PITLANE_LEGACY_SCHEMA=/tmp/pitlane-legacy-schema.sql go test ./... -count=1` in Docker | **62 tests passed**, no skipped tests in the recorded run. Includes actual ffmpeg decode/encode, independent running processes, concurrent identical retries, immutable live revisions, schedule overrun, handover, source scope/roles, backup/restore and migration. |
| Same suite with `-race` | Passed. Build assets first; an intermediate overlapping Vite/Go compilation encountered disappearing hashed assets and was rerun successfully after the build completed. |
| `go vet ./...` in Docker | Passed. |
| `go build -o /tmp/pitlane-preview .` in Docker | Passed; the browser served the Go-embedded SPA on localhost:3031. |
| Guarded localhost HTTP integration | **16 checks passed**; exact names and timestamp in `pitlane-integration-results.json`. Covers all 14 Advanced category foundations and real mutations described in the parity record. |
| Legacy database compatibility | Original schema from `a97dcbab9965b01601409c522365ee02814660f5`, upgraded twice and reapplied successfully. Existing settings/guest row and new driving-session row preserved. Full old-backend operation is not implied. |
| Normal responsive widths | 375, 768, 1024, 1440 CSS pixels: document scroll width equalled viewport width. Phone and desktop light/dark screenshots inspected. |
| Dense content / dialogs | 13 rigs, 13 cameras and 35 drivers, with long names/notes and ten gear items per added rig: no horizontal overflow at 375px. The edit sheet scrolled internally (735px visible / 4971px content). Driver search reduced the directory to the expected record. Temporary layout rows were removed afterward. |
| Keyboard / motion | Browser keyboard entry, visible focus, skip link, Enter activation, Shift-Tab/Tab dialog wrapping, Escape and trigger-focus restoration passed. The embedded stylesheet contains the reduced-motion override for transitions, animations and smooth scrolling; OS reduced-motion emulation was unavailable. |
| Worktree hygiene | `git diff --check` passed. Generated TypeScript build metadata restored; unrelated staged artwork preserved. |
| Native Watch fullscreen | Verified `document.documentElement.matches(':fullscreen')` at all sizes below, actual embedded video URL present, driver selection retained native fullscreen. |

Fullscreen panel measurements (stream and map were equal):

| CSS viewport | Each panel width | Each panel height |
| --- | ---: | ---: |
| 3840×2160 | 1880 | 900 |
| 2560×1440 | 1240 | 691.195 |
| 1920×1080 | 920 | 518.398 |
| 1440×900 | 680 | 432 |

Browser journey: authenticated fixture → installed content recache → rig/source setup → practice defaults/review → exact launch → actual UDP driver/position/lap ingestion → Watch and recording → finish → exact result file/media recap → Run again → saved draft → second browser launch/finish through the embedded build. The HTTP journey additionally tests an idle-source JPEG and an in-flight clip crossing a guest handover. Source notification links open the source gallery rather than treating its ID as a driver GUID.

The in-app browser's raster capture above the physical display crops/stitches fixed content. The delivered fullscreen raster is the valid 1440 CSS-pixel view. 4K acceptance evidence uses the actual fullscreen state and DOM dimensions rather than a misleading stitched image.

## Remaining external acceptance

- Run a compatible Kunos/AssettoServer binary with a real game client, then verify CSP requirements, spectator joining and in-game admin command effects. The available binary is x86 and the isolated Docker runtime is ARM64; the clearly labelled Python UDP fixture is not a game server.
- Test an actual external WHEP/MediaMTX endpoint and physical rig cameras, including network loss and reconnect. The local embed/MP4/MPEG-TS recorder path is verified.
- Drive other installed layouts with a real client to complement numerical projection/interpolation/ranking tests and the observed Drift map.

## Delivery records

- `PITLANE_PARITY.md`: all 53 tools, original routes, retained control audit and verification boundaries.
- `PITLANE_MIGRATION.md`: additive schema, permissions, backup/restore, rollback limits and reproducible isolated preview commands.
- `pitlane-integration-results.json`: executed HTTP checks.
- `pitlane-screenshots/README.md`: representative inspected screenshots.

Production preview: http://localhost:3031. Vite preview: http://localhost:5174. Fixture credentials: admin/admin. No real simulator session or production database/content was modified, no changes were staged/committed, and no deployment was performed.
