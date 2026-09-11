# Pitlane migration and preview

## Deployment boundary

This change updates the real Vue SPA and Go backend together. Vite still emits into `src/embed/webapp/dist`; Go embeds that directory. Build the SPA **before** Go compilation/tests; do not run Vite's output replacement concurrently with the Go embed scanner.

No deployment or production-data migration was performed during implementation. All process, import, deletion, handover and restore mutations used `pitlane-fixture` with `/tmp/pitlane-data` and `/tmp/pitlane-fixture/install`. Existing staged screen-art changes were preserved.

## Additive schema

Startup creates `rig`, `camera_source`, `source_rig`, `content_metadata`, `driving_session`, `driving_setup`, `driving_execution`, `instance_launch`, `execution_phase`, `execution_result`, `capture_job` and `metadata_change`. Additive columns associate driver connections, phase summaries, drift runs and media with executions; media has independent source identity and pinned guest ownership. Launches persist the exact request fingerprint and outcome. Queue reservation triggers protect a prepared launch, including legacy queue writers.

The upgrade preserves original users, configuration, presets, queues, guests, driver GUIDs, connections, result files and media paths. Legacy cameras are projected as `driver:N` and `spectator:N`, keeping their saved URLs. New sources use `source:N`. No rigs or historical session/result relationships are guessed from names, GUIDs or approximate timestamps. Existing uncorrelated history remains in Drivers, driving-stint search and legacy result files.

Every saved driving-session version owns a whole-value setup and private renderer records. Ordinary edits and Run again do not change shared presets. Executions preserve effective INIs, scoring and setup snapshots. Shared-library saves report current dependents and require explicit intent. Supported custom INI driving values are session-scoped; unsafe/unsupported keys and invalid values are rejected.

Startup marks interrupted prepared/live executions and capture jobs failed instead of claiming they resumed. Planned/queued sessions remain durable and are revalidated before launch. Scheduling stores an absolute instant plus IANA zone; the UI displays the local choice and zone. Queue and overrun behavior never promises a guaranteed start interval.

## Permissions

The existing viewer/steward/admin model remains authoritative. Viewers read daily screens, profiles, recaps and media. Stewards operate sessions, queues, handover, capture, guests and the existing car-class capability. Administrators manage infrastructure, source configuration, shared non-class presets, content and recovery.

Privileged source/status/capture URLs are redacted from viewer/steward source responses. Global configuration, raw configuration downloads, database/content backups, application logs, stream debug and recovery are admin-only. Guest profiles are readable by viewers; roster changes remain operator-only. Frontend controls and route guards follow backend permissions.

## Backup and restore

The database download uses SQLite `VACUUM INTO` to produce a consistent snapshot. The server-content archive is a separate export. Physical recordings, avatars and the complete installation are **not** contained in the database: back up their directories separately.

Restore uploads must pass SQLite header/integrity checks and contain the ServerManager core tables. A corrupt upload cannot replace an existing valid staged file. A valid upload is staged as `smdata.db.restore`, applied only at application restart, and the previous database remains `smdata.db.prev`. Stop servers before staging. The UI reports staging separately from application. Metadata undo is limited to an unchanged saved revision. Cleanup previews only unchanged abandoned upload files older than 24 hours; content, recordings, databases, restore files, directories, symlinks and active imports are excluded.

## Rollback

The pre-cutover schema from commit `a97dcbab9965b01601409c522365ee02814660f5` was applied to an isolated database, upgraded twice, and applied again successfully. Legacy settings and guest rows survived, and the new driving-session row remained present. This proves schema/read compatibility; it is not a claim that the old backend understands Pitlane orchestration.

The safe presentation rollback keeps the new backend and its database, and rebuilds it with the previous SPA. That UI will not display new rigs, standalone sources, driving aggregates or execution recaps. Do not operate old queue/start controls while a Pitlane launch is pending. Stop/resolve new sessions first. A full old-backend rollback requires stopping operations and preserving the entire upgraded database and media directory; the old build cannot reconcile new reservations, planned sessions or ownership. Do not drop additive tables or restore an old database merely to change presentation.

## Current runnable previews

- Go-embedded production SPA: http://localhost:3031
- Vite frontend using the same fixture API: http://localhost:5174
- Fixture login: `admin` / `admin` (disposable test installation only).
- Actual local MP4/embed/MPEG-TS test source: http://localhost:8787/player

The server is labelled **Protocol fixture · not a game server**. Its Python executable sends real AC UDP packets and exact result-file notifications, but is not a simulator. The available Kunos binary is x86 and the container is ARM64. No physical simulator, CSP client, AssettoServer client or external WHEP endpoint was available for final interoperability checks.

## Reproduce in a fresh isolated container

Prerequisites: repository dependencies installed, Docker, and `servermanager-servermanager-dev:latest` built from `Dockerfile.dev`. Run from the repository root. Use another container name if `pitlane-fixture` already exists; the provided scripts deliberately target this fixture name and temporary data path.

```sh
npm --prefix webapp run build
docker run -d --name pitlane-fixture --publish 127.0.0.1:3031:3030 \
  --mount type=bind,source="$PWD",target=/go/src/app \
  --workdir /go/src/app/src servermanager-servermanager-dev:latest \
  sh -c 'mkdir -p /tmp/pitlane-data && sleep 86400'
docker exec pitlane-fixture apk add --no-cache ffmpeg python3
docker exec -d pitlane-fixture sh -c 'GIN_MODE=release /usr/local/go/bin/go run . -debug -p /tmp/pitlane-data > /tmp/pitlane-preview.log 2>&1'
```

After first boot creates the temporary database, initialize copied display metadata:

```sh
docker exec pitlane-fixture python3 /go/src/app/scripts/pitlane-fixture-init.py
python3 scripts/pitlane-media-fixture.py
```

In the fixture UI, rebuild the content cache, create a rig and camera with account `pitlane-fixture-1`, player `http://localhost:8787/player`, status `http://host.docker.internal:8787/status`, and capture `http://host.docker.internal:8787/stream.ts`. Enable capture in Garage → Advanced → Streams & recording. The media fixture's HTTP server is a development service and must not be published as a production camera service.

Then run:

```sh
python3 scripts/pitlane-integration-check.py
PITLANE_API_URL=http://127.0.0.1:3031 npm --prefix webapp run dev -- --host 127.0.0.1 --port 5174
```

The integration script refuses any installation path other than `/tmp/pitlane-fixture/install`, uses only localhost:3031, and writes its result record under `docs/`. It intentionally creates fixture accounts/drafts/history. It never stages a live restore. Backup/restore application is tested in separate temporary SQLite unit fixtures.

## Validation commands

```sh
npm --prefix webapp test
npm --prefix webapp run build
git show a97dcbab9965b01601409c522365ee02814660f5:src/embed/schema.sql > /tmp/pitlane-legacy-schema.sql
docker cp /tmp/pitlane-legacy-schema.sql pitlane-fixture:/tmp/pitlane-legacy-schema.sql
docker exec pitlane-fixture sh -c 'PITLANE_LEGACY_SCHEMA=/tmp/pitlane-legacy-schema.sql /usr/local/go/bin/go test ./... -count=1'
docker exec pitlane-fixture sh -c 'PITLANE_LEGACY_SCHEMA=/tmp/pitlane-legacy-schema.sql /usr/local/go/bin/go test ./... -race -count=1'
docker exec pitlane-fixture /usr/local/go/bin/go vet ./...
docker exec pitlane-fixture /usr/local/go/bin/go build -o /tmp/pitlane-preview .
git diff --check
```
