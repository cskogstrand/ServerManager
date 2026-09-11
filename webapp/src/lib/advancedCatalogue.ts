// Production capability catalogue. Settings retain the roles enforced by the API.
export const advancedSections = [
  {
    "id": "health",
    "name": "Fix a problem",
    "summary": "Start with the symptom. Follow the evidence to the fix.",
    "icon": "info",
    "group": "START HERE"
  },
  {
    "id": "installation",
    "name": "Installation & engine",
    "summary": "Install paths, content discovery, AssettoServer, and CSP.",
    "icon": "road",
    "group": "CONFIGURE"
  },
  {
    "id": "configuration",
    "name": "Server configuration",
    "summary": "Identity, access, networking behaviour, and limits.",
    "icon": "sliders",
    "group": "CONFIGURE"
  },
  {
    "id": "servers",
    "name": "Servers & ports",
    "summary": "Independent servers, connections, and spectator slots.",
    "icon": "screen",
    "group": "CONFIGURE"
  },
  {
    "id": "custom",
    "name": "Custom sessions",
    "summary": "Build from scratch, organise setups, inspect the exact files.",
    "icon": "flag",
    "group": "CUSTOMISE"
  },
  {
    "id": "presets",
    "name": "Presets & scoring",
    "summary": "Every assist, phase, weather panel, grid entry, and drift rule.",
    "icon": "sliders",
    "group": "CUSTOMISE"
  },
  {
    "id": "automation",
    "name": "Queue & automation",
    "summary": "Order the evening, repeat a session, or start on schedule.",
    "icon": "repeat",
    "group": "CUSTOMISE"
  },
  {
    "id": "capture",
    "name": "Streams & recording",
    "summary": "Players, recorders, buffers, triggers, and source diagnostics.",
    "icon": "camera",
    "group": "MAINTAIN"
  },
  {
    "id": "content",
    "name": "Content maintenance",
    "summary": "Import, inspect, re-index, and remove content deliberately.",
    "icon": "car",
    "group": "MAINTAIN"
  },
  {
    "id": "access",
    "name": "People & access",
    "summary": "Operator accounts, roles, and account recovery.",
    "icon": "people",
    "group": "MANAGE"
  },
  {
    "id": "preferences",
    "name": "Your preferences",
    "summary": "Units and your own account settings.",
    "icon": "sun",
    "group": "MANAGE"
  },
  {
    "id": "history",
    "name": "History & attribution",
    "summary": "Find stints, correct ownership, and manage captured moments.",
    "icon": "clock",
    "group": "MANAGE"
  },
  {
    "id": "recovery",
    "name": "Data & recovery",
    "summary": "Backups, staged restores, and careful cleanup.",
    "icon": "repeat",
    "group": "MAINTAIN"
  },
  {
    "id": "system",
    "name": "System & logs",
    "summary": "Version, paths, logs, telemetry, and recent changes.",
    "icon": "info",
    "group": "MAINTAIN"
  }
] as const;
export const advancedTools = [
  {
    "id": "readiness",
    "section": "health",
    "title": "Check why a server won’t start",
    "description": "Installation, binary, content, configuration, grid, ports, and queue readiness in one check.",
    "scope": "Measured diagnostics",
    "path": "/setup",
    "role": "admin",
    "labels": [],
    "source": "SetupWorkbench.vue"
  },
  {
    "id": "telemetry",
    "section": "health",
    "title": "Drivers are connected, but the map is empty",
    "description": "Inspect plugin ports, the last packet, positions, and the selected track layout.",
    "scope": "Measured diagnostics",
    "path": "/server",
    "role": "viewer",
    "labels": [],
    "source": "ServerDetail.vue"
  },
  {
    "id": "stream-health",
    "section": "health",
    "title": "A stream is missing or won’t record",
    "description": "Check video playback and the recorder separately. See a specific next step.",
    "scope": "Measured diagnostics",
    "path": "/settings/streams",
    "role": "admin",
    "labels": [],
    "source": "StreamsDebug.vue"
  },
  {
    "id": "content-health",
    "section": "health",
    "title": "A car or track is missing",
    "description": "Review imports, layout metadata, and the content index before rescanning.",
    "scope": "Measured diagnostics",
    "path": "/garage/content",
    "role": "admin",
    "labels": [],
    "source": "Content.vue"
  },
  {
    "id": "install",
    "section": "installation",
    "title": "Installation & discovery",
    "description": "Validate the Assetto Corsa folder, inspect the server binary, and discover content.",
    "scope": "Installation",
    "path": "/settings/installation",
    "role": "admin",
    "labels": [
      "Install path",
      "Rescan content after validation"
    ],
    "source": "InstallationSettings.vue"
  },
  {
    "id": "engine",
    "section": "installation",
    "title": "Dedicated server engine",
    "description": "Choose stock Kunos or AssettoServer. Inspect installation before changing engine.",
    "scope": "All servers · next start",
    "path": "/settings",
    "role": "admin",
    "labels": [
      "Server engine",
      "Install missing engine binary",
      "Relax missing content checksums"
    ],
    "source": "SettingsConfig.vue"
  },
  {
    "id": "csp",
    "section": "installation",
    "title": "Custom Shaders Patch",
    "description": "Required version, extended physics, and pitbox behaviour.",
    "scope": "All servers · next start",
    "path": "/settings/installation",
    "role": "admin",
    "labels": [
      "Require CSP",
      "Minimum CSP build",
      "Extended car physics",
      "Extended track physics",
      "Hide pitboxes"
    ],
    "source": "InstallationSettings.vue"
  },
  {
    "id": "identity",
    "section": "configuration",
    "title": "Lobby identity & joining",
    "description": "Server name, welcome message, public listing, and content download links.",
    "scope": "All servers · next start",
    "path": "/settings",
    "role": "admin",
    "labels": [
      "Server name",
      "Welcome message",
      "Register in public lobby",
      "Append session name",
      "Append mod links",
      "Mod download URL"
    ],
    "source": "SettingsConfig.vue"
  },
  {
    "id": "server-access",
    "section": "configuration",
    "title": "Joining rules & server passwords",
    "description": "Public or private joining, admin password, and a locked entry list.",
    "scope": "All servers · next start",
    "path": "/settings",
    "role": "admin",
    "labels": [
      "Server password",
      "Admin password",
      "Locked entry list"
    ],
    "source": "SettingsConfig.vue"
  },
  {
    "id": "limits",
    "section": "configuration",
    "title": "Limits & network behaviour",
    "description": "Clients, result screen, update frequency, threads, and startup behaviour.",
    "scope": "All servers · next start",
    "path": "/settings",
    "role": "admin",
    "labels": [
      "Maximum clients",
      "Result screen seconds",
      "Client send interval Hz",
      "Threads",
      "Auto-start queue on launch"
    ],
    "source": "SettingsConfig.vue"
  },
  {
    "id": "instance",
    "section": "servers",
    "title": "Edit server & port assignments",
    "description": "Name, game ports, HTTP port, and the telemetry plugin pair. Check conflicts before saving.",
    "scope": "Selected server · must be stopped",
    "path": "/settings/instances",
    "role": "admin",
    "labels": [],
    "source": "SettingsInstances.vue"
  },
  {
    "id": "new-instance",
    "section": "servers",
    "title": "Add an independent server",
    "description": "Give it a name. Review a free set of ports before creating it.",
    "scope": "New server",
    "path": "/settings/instances",
    "role": "admin",
    "labels": [],
    "source": "SettingsInstances.vue"
  },
  {
    "id": "delete-instance",
    "section": "servers",
    "title": "Remove a server",
    "description": "Only stopped servers can be removed. Keep at least one server and review queued sessions.",
    "scope": "Selected server",
    "path": "/settings/instances",
    "role": "admin",
    "labels": [],
    "source": "SettingsInstances.vue"
  },
  {
    "id": "spectator",
    "section": "servers",
    "title": "Reserve a spectator slot",
    "description": "Lock a name, GUID, car, and skin to the stream client.",
    "scope": "Selected server · next start",
    "path": "/settings/streaming",
    "role": "admin",
    "labels": [
      "Reserve a locked spectator slot",
      "Spectator name",
      "Spectator GUID",
      "Car key",
      "Skin key"
    ],
    "source": "StreamingCapture.vue"
  },
  {
    "id": "instance-driving",
    "section": "servers",
    "title": "Server-specific driving rules",
    "description": "Drift HUD, scoring mode, and permission to drive the wrong way.",
    "scope": "Selected server · next start",
    "path": "/settings/instances",
    "role": "admin",
    "labels": [
      "Enable drift scoring HUD",
      "Scoring mode",
      "Allow wrong-way driving"
    ],
    "source": "SettingsInstances.vue"
  },
  {
    "id": "custom-session",
    "section": "custom",
    "title": "Build a completely custom session",
    "description": "Choose every phase, assist, weather panel and grid entry in an isolated driving session.",
    "scope": "New draft · no running session changes",
    "path": "/sessions/new",
    "role": "steward",
    "labels": [
      "Session name",
      "Track",
      "Layout",
      "Driving type",
      "Group",
      "Difficulty preset",
      "Phase preset",
      "Time and weather preset",
      "Car class preset",
      "Grid size",
      "Duration minutes",
      "Race laps",
      "Strategy"
    ],
    "source": "SessionEditor.vue + private session API"
  },
  {
    "id": "event-groups",
    "section": "custom",
    "title": "Organise setups & groups",
    "description": "Create, rename, duplicate, or delete groups and their saved setups.",
    "scope": "Reusable setups · shared preset references",
    "path": "/events",
    "role": "admin",
    "labels": [
      "Group name",
      "Action",
      "New name",
      "Include setups when duplicating"
    ],
    "source": "Events.vue"
  },
  {
    "id": "raw-config",
    "section": "custom",
    "title": "Inspect or customise generated files",
    "description": "Review generated server_cfg.ini and entry_list.ini. Save validated driving overrides in this session only.",
    "scope": "Selected server · draft only",
    "path": "/sessions/new",
    "role": "steward",
    "labels": [],
    "source": "SessionEditor.vue + shared ConfigRenderer"
  },
  {
    "id": "live-controls",
    "section": "custom",
    "title": "Operate a running session",
    "description": "Broadcast chat, next phase, restart phase, kick a driver, or use an admin command.",
    "scope": "Selected live server",
    "path": "/server",
    "role": "steward",
    "labels": [
      "Action",
      "Driver or car ID",
      "Message or command"
    ],
    "source": "ServerDetail.vue"
  },
  {
    "id": "live-edit",
    "section": "custom",
    "title": "Change a running setup",
    "description": "Track, layout, grid, time and weather—with an explicit restart decision.",
    "scope": "Selected live server · restart may disconnect drivers",
    "path": "/server",
    "role": "steward",
    "labels": [
      "Track key",
      "Layout",
      "Car class",
      "Grid entries",
      "Time",
      "Weather",
      "CSP date",
      "Time multiplier",
      "Apply at"
    ],
    "source": "ServerDetail.vue"
  },
  {
    "id": "difficulty",
    "section": "presets",
    "title": "Assists, realism & track grip",
    "description": "Every driving aid, penalty, consumption rate, vote rule, and dynamic-track setting.",
    "scope": "Shared preset · usage confirmation before save",
    "path": "/presets/difficulty",
    "role": "admin",
    "labels": [
      "Preset name",
      "ABS",
      "Traction control",
      "Stability control",
      "Auto clutch",
      "Tyre blankets",
      "Virtual mirror",
      "Fuel rate percent",
      "Damage percent",
      "Tyre wear percent",
      "Allowed tyres out",
      "Maximum ballast kg",
      "Start rule",
      "Gas penalty",
      "Maximum contacts per km",
      "Dynamic track",
      "Dynamic track preset",
      "Session start grip percent",
      "Randomness",
      "Session grip transfer percent",
      "Lap gain",
      "Kick quorum percent",
      "Voting quorum percent",
      "Vote duration seconds",
      "Blacklist mode",
      "Duplicate",
      "Shared usage"
    ],
    "source": "PresetDifficulty.vue"
  },
  {
    "id": "phases",
    "section": "presets",
    "title": "Booking, practice, qualifying & race",
    "description": "Phase durations, joining rules, race overtime, reverse grids, and pit windows.",
    "scope": "Shared preset · usage confirmation before save",
    "path": "/presets/sessions",
    "role": "admin",
    "labels": [
      "Preset name",
      "Booking enabled",
      "Booking minutes",
      "Booking open to join",
      "Practice enabled",
      "Practice minutes",
      "Practice open to join",
      "Qualifying enabled",
      "Qualifying minutes",
      "Qualifying open to join",
      "Race enabled",
      "Race minutes",
      "Race open to join",
      "Qualifying maximum wait percent",
      "Extra lap",
      "Race overtime seconds",
      "Race wait seconds",
      "Reversed grid positions",
      "Pit window start lap",
      "Pit window end lap",
      "Duplicate",
      "Shared usage"
    ],
    "source": "PresetSession.vue"
  },
  {
    "id": "weather-panels",
    "section": "presets",
    "title": "Time & ordered weather panels",
    "description": "Time of day, temperatures, variation, wind, and CSP time/date settings.",
    "scope": "Shared preset · usage confirmation before save",
    "path": "/presets/time",
    "role": "admin",
    "labels": [
      "Preset name",
      "Time of day",
      "Time multiplier",
      "Enable CSP time",
      "Weather panels",
      "Duplicate",
      "Shared usage"
    ],
    "source": "PresetTime.vue"
  },
  {
    "id": "grid",
    "section": "presets",
    "title": "Car classes, liveries & ballast",
    "description": "Build an ordered grid with explicit car/skin keys, counts, and ballast.",
    "scope": "Shared preset · usage confirmation before save",
    "path": "/presets/classes",
    "role": "steward",
    "labels": [
      "Preset name",
      "Grid entries",
      "Duplicate",
      "Shared usage"
    ],
    "source": "PresetClass.vue"
  },
  {
    "id": "drift-scoring",
    "section": "presets",
    "title": "Drift scoring modes",
    "description": "Thresholds, weights, multiplier growth, collision rules, and run-reset timers.",
    "scope": "Shared scoring mode · usage confirmation before save",
    "path": "/presets/drift-scoring",
    "role": "admin",
    "labels": [
      "Preset name",
      "Minimum speed kmh",
      "Minimum angle degrees",
      "Angle weight",
      "Speed weight",
      "Proximity weight",
      "Proximity range metres",
      "Gain pace seconds",
      "Multiplier cap",
      "Reset multiplier",
      "Multiplier reset seconds",
      "Collision resets score",
      "Car collision resets score",
      "Reset score",
      "Score reset seconds",
      "Duplicate",
      "Shared usage"
    ],
    "source": "DriftScoringModes.vue"
  },
  {
    "id": "preset-library",
    "section": "presets",
    "title": "Manage reusable presets",
    "description": "Find by family, inspect usage, duplicate, rename, or delete an unused preset.",
    "scope": "Selected preset",
    "path": "/presets",
    "role": "steward",
    "labels": [
      "Family",
      "Preset name",
      "Action",
      "New name"
    ],
    "source": "PresetTemplates.vue"
  },
  {
    "id": "queue-editor",
    "section": "automation",
    "title": "Edit the running order",
    "description": "Reorder, add a setup or group, skip an event, remove a queued item, and clear completed runs.",
    "scope": "Selected server",
    "path": "/queue",
    "role": "steward",
    "labels": [],
    "source": "Queue.vue"
  },
  {
    "id": "repeat",
    "section": "automation",
    "title": "Repeat a session",
    "description": "Repeat one setup continuously. The manual queue waits until repeat mode ends.",
    "scope": "Selected server",
    "path": "/queue",
    "role": "steward",
    "labels": [
      "Run mode",
      "Repeat setup"
    ],
    "source": "Queue.vue"
  },
  {
    "id": "schedule",
    "section": "automation",
    "title": "Scheduled starts & boot behaviour",
    "description": "Set or clear a one-shot start and decide what happens when Server Manager launches.",
    "scope": "Selected server",
    "path": "/settings/instances",
    "role": "admin",
    "labels": [
      "Start on boot",
      "Scheduled start",
      "Clear scheduled start",
      "Time zone"
    ],
    "source": "SettingsInstances.vue + Queue.vue"
  },
  {
    "id": "source-library",
    "section": "capture",
    "title": "Manage every stream source",
    "description": "Rig feeds, standalone cameras, player/status links, and enable/disable controls.",
    "scope": "Rig inventory and individual sources",
    "path": "/garage/rigs",
    "role": "admin",
    "labels": [],
    "source": "Rigs.vue + pitlane_inventory.go"
  },
  {
    "id": "capture-settings",
    "section": "capture",
    "title": "Photos, clips & automatic highlights",
    "description": "Global enablement, triggers, clip length, cooldowns, and per-session limits.",
    "scope": "Capture subsystem",
    "path": "/settings/streaming",
    "role": "admin",
    "labels": [
      "Enable screenshots",
      "Enable clips",
      "Automatically save drift runs",
      "Minimum drift score",
      "Clip length seconds",
      "Cooldown seconds",
      "Maximum clips per driver per session"
    ],
    "source": "StreamingCapture.vue"
  },
  {
    "id": "recorder-diagnostics",
    "section": "capture",
    "title": "Recorder health, buffers & probes",
    "description": "ffmpeg availability, pullable sources, buffered segments, stalled workers, and recent logs.",
    "scope": "Measured diagnostics",
    "path": "/settings/streams",
    "role": "admin",
    "labels": [],
    "source": "StreamsDebug.vue"
  },
  {
    "id": "stream-status",
    "section": "capture",
    "title": "Source status & advanced links",
    "description": "Separate the player, health endpoint, raw capture URL, and enabled state.",
    "scope": "Selected source",
    "path": "/garage/rigs",
    "role": "admin",
    "labels": [
      "Source",
      "Enabled",
      "Player URL",
      "Status URL",
      "Recording source"
    ],
    "source": "Rigs.vue + source health API"
  },
  {
    "id": "content-browser",
    "section": "content",
    "title": "Browse all content",
    "description": "Cars, tracks, layouts, weather, tags, brands, liveries, and usage.",
    "scope": "See tool for scope",
    "path": "/garage/content",
    "role": "admin",
    "labels": [],
    "source": "Content.vue"
  },
  {
    "id": "content-import",
    "section": "content",
    "title": "Archive & URL imports",
    "description": "Review type detection, file/URL imports, progress, retries, and overwrite choices.",
    "scope": "See tool for scope",
    "path": "/garage/content",
    "role": "admin",
    "labels": [],
    "source": "Content.vue"
  },
  {
    "id": "content-metadata",
    "section": "content",
    "title": "Inspect complete content metadata",
    "description": "Car power/torque curves, specs, skins, track layout geometry, pitboxes, and weather keys.",
    "scope": "Content library · inspection",
    "path": "/garage/content",
    "role": "admin",
    "labels": [],
    "source": "Content.vue"
  },
  {
    "id": "recache",
    "section": "content",
    "title": "Rebuild the content index",
    "description": "Rescan the installation after a manual file change. Content files remain intact.",
    "scope": "Content index",
    "path": "/garage/content",
    "role": "admin",
    "labels": [],
    "source": "Content.vue"
  },
  {
    "id": "delete-content",
    "section": "content",
    "title": "Remove content from disk",
    "description": "Inspect session and preset usage first. Deleting files requires a deliberate final choice.",
    "scope": "Selected content item",
    "path": "/garage/content",
    "role": "admin",
    "labels": [],
    "source": "Content.vue"
  },
  {
    "id": "users",
    "section": "access",
    "title": "Accounts & roles",
    "description": "Create accounts, change administrator/steward/viewer access, reset passwords or remove an account. Last-admin and self protections apply.",
    "scope": "Operator accounts",
    "path": "/settings/users",
    "role": "admin",
    "labels": [
      "Account name",
      "Action",
      "Role"
    ],
    "source": "Users.vue"
  },
  {
    "id": "account-reset",
    "section": "access",
    "title": "Reset an operator password",
    "description": "An admin-only account action with a clearly identified recipient.",
    "scope": "Selected operator account",
    "path": "/settings/users",
    "role": "admin",
    "labels": [
      "Account name",
      "New password",
      "Confirm password"
    ],
    "source": "Users.vue"
  },
  {
    "id": "units",
    "section": "preferences",
    "title": "Measurement & temperature units",
    "description": "Choose the units used on timing, maps, and weather views.",
    "scope": "Your account only",
    "path": "/preferences",
    "role": "viewer",
    "labels": [
      "Measurement units",
      "Temperature units"
    ],
    "source": "SettingsUser.vue"
  },
  {
    "id": "own-account",
    "section": "preferences",
    "title": "Your password & account",
    "description": "Change your password or sign out without affecting other operators.",
    "scope": "Your account only",
    "path": "/preferences",
    "role": "viewer",
    "labels": [
      "Action",
      "New password",
      "Confirm password"
    ],
    "source": "SettingsUser.vue + Login.vue"
  },
  {
    "id": "find-stint",
    "section": "history",
    "title": "Find a driving stint or result",
    "description": "Search by driver, track, tag, or date; inspect lap results and individual drift runs.",
    "scope": "See tool for scope",
    "path": "/sessions/drives",
    "role": "viewer",
    "labels": [],
    "source": "SessionSearch.vue + ResultsHistory.vue"
  },
  {
    "id": "driver-records",
    "section": "history",
    "title": "Driver profiles, progress & media",
    "description": "Drivers and guests, avatars, trends, favourites, sessions, photos, clips, and downloads.",
    "scope": "See tool for scope",
    "path": "/drivers",
    "role": "viewer",
    "labels": [],
    "source": "DriverDetail.vue + GuestDriverDetail.vue"
  },
  {
    "id": "attribution",
    "section": "history",
    "title": "Correct who was driving",
    "description": "Correct a historical stint or capture. This changes the selected history and is separate from handing over a live rig.",
    "scope": "Selected historical stint only",
    "path": "/drivers",
    "role": "admin",
    "labels": [
      "Driver or guest",
      "Stint or session",
      "Assign to"
    ],
    "source": "DriverDetail.vue + GuestDrivers.vue"
  },
  {
    "id": "media-library",
    "section": "history",
    "title": "Tags & captured media",
    "description": "Tag a stint, download a capture, remove a media item, or inspect capture ownership.",
    "scope": "Selected history item",
    "path": "/drivers",
    "role": "steward",
    "labels": [
      "Item",
      "Action",
      "Tag or media filename"
    ],
    "source": "DriverDetail.vue"
  },
  {
    "id": "leaderboards",
    "section": "history",
    "title": "Club records & spectator display",
    "description": "All-server drift/lap comparisons with track, car, recency, and clip links; automatic broadcast follow.",
    "scope": "Display preferences",
    "path": "/leaderboard",
    "role": "viewer",
    "labels": [
      "View",
      "Discipline",
      "Track filter",
      "Car filter",
      "Recency",
      "Rotate cameras"
    ],
    "source": "BroadcastLeaderboard.vue + Broadcast.vue"
  },
  {
    "id": "backup",
    "section": "recovery",
    "title": "Back up the club",
    "description": "Export the database with settings and history, or the server content archive. Physical recordings require a separate filesystem backup.",
    "scope": "Database and server-content exports",
    "path": "/maintenance",
    "role": "admin",
    "labels": [],
    "source": "Maintenance.vue"
  },
  {
    "id": "restore",
    "section": "recovery",
    "title": "Stage a database restore",
    "description": "Review what will be replaced. Apply on the next application restart, preserving the previous database.",
    "scope": "Database · next restart",
    "path": "/maintenance",
    "role": "admin",
    "labels": [],
    "source": "Maintenance.vue"
  },
  {
    "id": "cleanup",
    "section": "recovery",
    "title": "Clean up failed jobs & temporary files",
    "description": "Preview exact abandoned upload files and remove only the unchanged preview. Content, recordings and restore files are excluded.",
    "scope": "Abandoned upload files older than 24 hours",
    "path": "/garage/recovery",
    "role": "admin",
    "labels": [],
    "source": "Recovery.vue + pitlane_recovery.go"
  },
  {
    "id": "system-info",
    "section": "system",
    "title": "Version, paths & support information",
    "description": "Inspect application version, config/temp paths, selected engine, and runtime capabilities.",
    "scope": "Measured diagnostics",
    "path": "/about",
    "role": "viewer",
    "labels": [],
    "source": "About.vue"
  },
  {
    "id": "logs",
    "section": "system",
    "title": "Application logs & telemetry details",
    "description": "Read server output and inspect telemetry packet times. Stream diagnostics include recorder logs.",
    "scope": "Measured diagnostics",
    "path": "/server",
    "role": "viewer",
    "labels": [],
    "source": "ServerDetail.vue + About.vue"
  },
  {
    "id": "change-history",
    "section": "system",
    "title": "Recent changes & undo",
    "description": "Inspect saved metadata changes and undo an unchanged latest revision. Server operations and deleted files cannot be undone here.",
    "scope": "Local content metadata only",
    "path": "/garage/recovery",
    "role": "admin",
    "labels": [],
    "source": "Recovery.vue + pitlane_content.go"
  }
] as const;
