# Garage: capability map and prototype scope

The Garage concept now has three distinct workspaces: **Rigs & cameras**, **Cars & tracks**, and **Advanced**. Servers and The club are separate bordered sections with their own headings, accents, and actions.

## Rigs are equipment, drivers are people

Each rig has a stable identity, name, location, display arrangement, gear list, notes, and attached camera sources. Display choices are Triple screens, Single (16:9), Wide (21:9), Ultrawide (32:9), VR, and Custom. Gear supports wheelbases, wheels, pedals, shifters, handbrakes, cockpit/seat, screens, VR headsets, PC, motion, haptics, and other equipment. All of these are inventory notes; the prototype does not configure a simulator PC.

Attached cameras take their displayed driver from the rig. Rig 01 retains the shared-seat handover flow and closes the previous driver's recording at the boundary. Standalone cameras remain separate. Stream setup is available on the rig profile and in Watch. This needs a production rig model: the current app mainly associates streams with driver GUIDs, and has no full equipment inventory.

## A real content workspace

The library has Cars, Tracks, and Weather views, search, detail pages, tags and notes, usage, liveries, layout maps, a reviewed file/URL sample import, overwrite handling, and reversible archiving. It reads selected filenames only and never uploads or fetches them. Images and records are illustrative; the car cards explicitly label their shared collection image. Missing imported-track geometry prevents the concept from silently creating a session on a different track.

Production content metadata comes from the existing service. Rich specifications/curves, batch jobs, exact download operations, disk deletion, and complete filters are represented in Advanced but need the existing backend integration. Archiving and editable local labels are proposed additions. The prototype's usage display is a sample approximation, not an authoritative dependency scan.

## Advanced: 53 tools, 14 sections, 180 labelled settings

Advanced is an administrator-view interaction study. Search finds tool names, descriptions, and nested setting labels. A persistent server selector states the current target. Editors use native fields and progressive disclosure, with a review step before local changes. Private preset copies are the default; a shared edit shows its effect on other setups.

**What is executable here:** local settings drafts and reviews, rig and gear editing, source linking, content metadata/import/archive simulation, adding/editing/removing eligible demo servers with port checks, custom session drafts, basic INI draft validation, queue order/add/remove and repeat gating, sample cleanup/restore states, sample manifest downloads, and undo for recent supported changes.

**What is a mapped design surface:** actual server start/stop/live commands, account/password changes, shared preset CRUD and backend usage resolution, real import/recorder health checks, media operations, historical corrections, all-server records, engine installation, filesystem cleanup and database restore. Their inputs and effects are reviewable; applying them stores a local concept draft or opens the relevant concept page. It does not execute the production operation. Daily history/profile pages still need their full production detail.

A production rebuild should preserve the original role gates, usage constraints, precise telemetry and result semantics, server targeting, and backend confirmation rules. The prototype's diagnostic results are labelled samples. Password entries are never retained in its settings or change history.

## Catalogue

| Section / tool | Existing source or extension | Representation |
| --- | --- | --- |
| **Fix a problem** · Check why a server won’t start | SetupWorkbench.vue | Sample diagnosis + evidence + next step |
| **Fix a problem** · Drivers are connected, but the map is empty | ServerDetail.vue | Sample diagnosis + evidence + next step |
| **Fix a problem** · A stream is missing or won’t record | StreamsDebug.vue | Sample diagnosis + evidence + next step |
| **Fix a problem** · A car or track is missing | Content.vue | Sample diagnosis + evidence + next step |
| **Installation & engine** · Installation & discovery | InstallationSettings.vue | Labelled editor → review → local draft |
| **Installation & engine** · Dedicated server engine | SettingsConfig.vue | Labelled editor → review → local draft |
| **Installation & engine** · Custom Shaders Patch | InstallationSettings.vue | Labelled editor → review → local draft |
| **Server configuration** · Lobby identity & joining | SettingsConfig.vue | Labelled editor → review → local draft |
| **Server configuration** · Joining rules & server passwords | SettingsConfig.vue | Labelled editor → review → local draft |
| **Server configuration** · Limits & network behaviour | SettingsConfig.vue | Labelled editor → review → local draft |
| **Servers & ports** · Edit server & port assignments | SettingsInstances.vue | Local server editor with port/lifecycle checks |
| **Servers & ports** · Add an independent server | SettingsInstances.vue | Local server editor with port/lifecycle checks |
| **Servers & ports** · Remove a server | SettingsInstances.vue | Usage/lifecycle check; no disk deletion |
| **Servers & ports** · Reserve a spectator slot | StreamingCapture.vue | Labelled editor → review → local draft |
| **Servers & ports** · Server-specific driving rules | SettingsInstances.vue | Labelled editor → review → local draft |
| **Custom sessions** · Build a completely custom session | Events.vue | Labelled editor → review → local draft |
| **Custom sessions** · Organise setups & groups | Events.vue | Labelled editor → review → local draft |
| **Custom sessions** · Inspect or customise generated files | Events.vue + render-preview API | Basic INI editor + review |
| **Custom sessions** · Operate a running session | ServerDetail.vue | Labelled editor → review → local draft |
| **Custom sessions** · Change a running setup | ServerDetail.vue | Labelled editor → review → local draft |
| **Presets & scoring** · Assists, realism & track grip | PresetDifficulty.vue | Labelled editor → review → local draft |
| **Presets & scoring** · Booking, practice, qualifying & race | PresetSession.vue | Labelled editor → review → local draft |
| **Presets & scoring** · Time & ordered weather panels | PresetTime.vue | Labelled editor → review → local draft |
| **Presets & scoring** · Car classes, liveries & ballast | PresetClass.vue | Labelled editor → review → local draft |
| **Presets & scoring** · Drift scoring modes | DriftScoringModes.vue | Labelled editor → review → local draft |
| **Presets & scoring** · Manage reusable presets | PresetTemplates.vue | Labelled editor → review → local draft |
| **Queue & automation** · Edit the running order | Queue.vue | Interactive sample queue |
| **Queue & automation** · Repeat a session | Queue.vue | Labelled editor → review → local draft |
| **Queue & automation** · Scheduled starts & boot behaviour | SettingsInstances.vue + Queue.vue | Labelled editor → review → local draft |
| **Streams & recording** · Manage every stream source | StreamingCapture.vue | Linked Rigs workspace |
| **Streams & recording** · Photos, clips & automatic highlights | StreamingCapture.vue | Labelled editor → review → local draft |
| **Streams & recording** · Recorder health, buffers & probes | StreamsDebug.vue | Sample diagnosis + evidence + next step |
| **Streams & recording** · Source status & advanced links | StreamingCapture.vue | Labelled editor → review → local draft |
| **Content maintenance** · Browse all content | Content.vue | Relevant daily concept workspace |
| **Content maintenance** · Archive & URL imports | Content.vue | Sample import review |
| **Content maintenance** · Inspect complete content metadata | Content.vue | Expandable metadata inspection |
| **Content maintenance** · Rebuild the content index | Content.vue | Preview + sample repair state |
| **Content maintenance** · Remove content from disk | Content.vue | Usage/lifecycle check; no disk deletion |
| **People & access** · Accounts & roles | Users.vue | Labelled editor → review → local draft |
| **People & access** · Reset an operator password | Users.vue | Labelled editor → review → local draft |
| **Your preferences** · Measurement & temperature units | SettingsUser.vue | Labelled editor → review → local draft |
| **Your preferences** · Your password & account | SettingsUser.vue + Login.vue | Labelled editor → review → local draft |
| **History & attribution** · Find a driving stint or result | SessionSearch.vue + ResultsHistory.vue | Relevant daily concept workspace |
| **History & attribution** · Driver profiles, progress & media | DriverDetail.vue + GuestDriverDetail.vue | Relevant daily concept workspace |
| **History & attribution** · Correct who was driving | DriverDetail.vue + GuestDrivers.vue | Labelled editor → review → local draft |
| **History & attribution** · Tags & captured media | DriverDetail.vue | Labelled editor → review → local draft |
| **History & attribution** · Club records & spectator display | BroadcastLeaderboard.vue + Broadcast.vue | Labelled editor → review → local draft |
| **Data & recovery** · Back up the club | Maintenance.vue | Scoped sample manifest export |
| **Data & recovery** · Stage a database restore | Maintenance.vue | Staged sample restore + undo |
| **Data & recovery** · Clean up failed jobs & temporary files | New concept workflow | Preview + sample repair state |
| **System & logs** · Version, paths & support information | About.vue | Sample diagnosis + evidence + next step |
| **System & logs** · Application logs & telemetry details | ServerDetail.vue + About.vue | Sample diagnosis + evidence + next step |
| **System & logs** · Recent changes & undo | New concept workflow | Recent local changes + undo |

## Original route coverage

| Original area | Place in the concept |
| --- | --- |
| Setup workbench, readiness | Advanced → Fix a problem; first-use setup |
| Installation, engine, CSP | Advanced → Installation & engine |
| Global server settings | Advanced → Server configuration |
| Server instances, ports, spectator slots | Advanced → Servers & ports |
| Events, categories, rendered file previews | Advanced → Custom sessions; Sessions |
| Difficulty, phases, weather/time, car-class presets, drift scoring | Advanced → Presets & scoring |
| Queue, schedules, repeat, boot behaviour | Advanced → Queue & automation |
| Dashboard, server controls, live overrides | Today / Session control; Advanced → Custom sessions |
| Broadcast, streams, fullscreen | Live / Watch; Rigs; Advanced → Streams & recording |
| Content library, imports, metadata, recache, removal | Cars & tracks; Advanced → Content maintenance |
| User administration, account preferences, login/sign-out | The club; Advanced → People & access / Your preferences; production authentication must remain |
| Driver/guest profiles, avatars, history, tags, attribution, media | Drivers / Sessions; Advanced → History & attribution |
| All-server leaderboard and auto-follow broadcast | Advanced → History & attribution; full display surface still to be integrated |
| Backup, restore, version, paths, logs | Advanced → Data & recovery / System & logs |

## New recovery and customisation proposals

- Inspect scope before cleanup; separate temporary files, recordings, content, and database.
- Preserve a reviewable recent-change trail with an explicit undo boundary.
- Include media as an explicit backup scope, rather than implying the database backup includes it.
- Allow custom generated-file drafts, with authoritative server validation before they can run.

The standalone prototype makes no production API calls. It does not install software, edit game files, change accounts, delete real data, or restart any game server. Only the localhost prototype preview server was restarted for verification.

## Validation

Run `node docs/pitlane-concept/check.cjs` from the repository root. It covers the earlier session/watch flows plus equipment CRUD, duplicate rig names, display choices, stream-to-rig ownership, content import/overwrite/archive, every Advanced tool opening, nested-field search, review-before-apply, port conflicts, running-server protection, custom/INI drafts, cleanup/restore undo, queue ordering/repeat and persistence.

Browser verification checked nine destinations at 375, 768, 1024 and 1440 CSS pixels with no horizontal document overflow or broken images. Targeted dialog, keyboard, theme and visual checks supplement this; it is not a complete accessibility audit.
