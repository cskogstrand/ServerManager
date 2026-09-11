# Pitlane — a fresh frontend proposal

The app helps people run Assetto Corsa sessions together. Its operator prepares a driving experience, brings drivers onto a server, manages what happens on track, and keeps the results. It also supports friends sharing a simulator, drift scoring, camera feeds, and recorded highlights. Those uses deserve equal consideration alongside conventional races.

My proposal is to make **the session** the centre of the product. One thing moves from draft, to planned, to live, to finished. Its settings, drivers, results, and recordings travel with it. “Pitlane” is a working name for this design direction, not a proposed mandatory rebrand.

## Explore the concept

Open `index.html` directly, or from the repository root run:

```sh
python3 -m http.server 8786 --bind 127.0.0.1 --directory docs/pitlane-concept
```

Then open <http://127.0.0.1:8786>.

Suggested walkthrough:

1. **New session → Drift → Start session.** The complete setup is ready by default. A free server is selected, and the current session stays running.
2. Reset the demo. Open **Thursday drift club → Next driver → Alex**. New runs belong to the person taking the wheel.
3. **Finish session → Finish & see recap → Run it again.** The same session carries its story from preparation to history.
4. **New session → Race → Later.** Scheduling is part of the session, with an explicit time zone.
5. **Try first use** in the study bar. Installation discovery leads directly to the first driving experience.
6. **Live → Thursday drift club**, or **Open session → Watch**. Pick a driver on the map, board, or camera strip. Their camera and position follow together. Try Erik’s disconnected feed and Leo’s missing stream.
7. **Watch → Set up streams → Add simulator**. Use the sample source, check it, and save it. Recording expands only when needed. **Live** also shows the lounge simulator with no active session.
8. **Watch → Record clip**, then return to Session control and hand Rig 01 to Alex. The previous clip stays with Nora; the camera label changes to Alex. Open Saved moments or Nora’s profile to find the demo capture.
9. **Garage → Rigs & cameras → Rig 01 → Edit rig**. Choose triple, single, wide, ultrawide, VR, or custom. Add and edit gear, notes, and attached camera feeds.
10. **Garage → Cars & tracks**. Browse Cars, Tracks, and Weather. Open an item to inspect usage, edit tags, archive/restore it, or use it in a session. Add a sample pack to try import review and overwrite handling.
11. **Garage → Advanced**. Search for ports, multiplier, or a missing stream. Review a custom setup, try a port conflict, inspect an INI draft, or stage and undo a sample restore.
12. Try a 24-car grid on Rudskogen, or a two-hour session starting now. The concept blocks an invalid grid or a conflicting server allocation and tells you what to change.

The top study bar is for evaluating the prototype. It would not ship in the product. All people, activity, times, metrics, and operations are illustrative. The demo clock is Thursday 10 September 2026 at 18:42, local Oslo time. Images come from the project's existing game content. Fonts have system fallbacks.

## Four daily destinations

| Destination | The question it answers | What belongs here |
| --- | --- | --- |
| **Today** | What is happening, and what should I do next? | Current sessions, the next planned session, a strong New session action, recent favourites, a highlight. |
| **Sessions** | What are we driving, and what did we do? | Drafts, upcoming sessions, live sessions, finished sessions, complete settings, recaps, replaying a previous combination. |
| **Live** | What can I watch right now? | All connected simulator and spectator feeds, session watching with linked camera/map/timing, contextual stream setup, manual capture. |
| **Drivers** | Who is here, and how are they doing? | One profile per person, results, progress, clips, and account/rig associations. |

**Garage** is a secondary destination with dedicated **Rigs & cameras**, **Cars & tracks**, and **Advanced** workspaces. Servers and The club have distinct bordered sections and headings. A rig owns its display setup, gear, notes, and source links; drivers remain separate people. Advanced has 53 searchable tools across 14 sections, including 180 labelled settings from the existing administration surfaces and proposed recovery workflows. Advanced controls stay discoverable here and inside their relevant session section. A professional operator can still inspect the exact configuration.

The first implementation should preserve existing deep links and translate their destinations into this model. “Session” in the interface means the complete driving experience; practice, qualifying, and race are **phases**, avoiding the current naming ambiguity.

## The flows I would build

| Job | Proposed flow | Work the app handles |
| --- | --- | --- |
| First drive | Connect installation → choose experience → start | Scan content, establish safe identity/access defaults, select unused ports, create a usable server, check readiness. Ask for missing information only when it is actually missing. |
| Casual drift evening | Drift → optionally change track → start | Appropriate car collection, scoring, accessible assists, a valid grid, daylight, available server, result recording. Clips only when a recording source is healthy. |
| Organised race | Race → review track/grid/format → start or schedule | Practice/qualifying/race phases, capacities, session timings, and conflicts. Fine details expand in place. |
| Another track afterwards | Use a previous session → After current | Add the exact session to the current server's plan. Keep the current drive intact. Show the upcoming order inside that server's live session. |
| Shared simulator | Open session → Next driver → choose or add a person | End the attribution segment, switch the identity for future runs, and attach new clips to that person. Preserve previous results. |
| Manage a live session | Open session → timing or drift board | Prioritise drivers and current phase. Maps and conditions expand on demand; messages, disconnects, next phase, and finishing remain contextual. |
| Watch a session | Open session → Watch | Camera, actual circuit map with driver markers, timing, and instruments stay linked. Setup and capture actions are contextual for authorised operators. Video and telemetry have independent failure states. |
| Watch all simulators | Live → select feed or session | Keep connected feeds visible even without drivers in a game. Show source health, filter by session, and open setup from the affected feed. |
| Connect a simulator | Watch or Live → Set up streams → name + player link → check → save | Associate the source with a driver or spectator session. Expand optional recording settings; distinguish playback readiness from recorder readiness. |
| Finish | Finish → recap | Preserve results, useful moments, and the exact setup. Offer Run it again. |
| Missing content | Change track/car → add content → return to session | Detect the archive type, show import progress, validate what arrived, and preserve the draft while it runs. |

## Simplicity rules

- **A usable default beats an empty form.** Start with Drift, Race, or Practice. Base defaults on installed content and actual server capabilities. A recommendation should never silently add unavailable content.
- **Save a whole experience.** Reuse is “Run it again.” Preset families and categories need not be prerequisites or navigation items. Advanced users can explicitly share a configuration; ordinary edits affect only this session.
- **Choose a server when it matters.** Automatically pick a free compatible server for the requested interval. Identify the selection before starting. When resources conflict, offer a later time, a different server, or a place after the current session.
- **Hide mechanics, expose consequences.** “Starts on Club 02.” “This disconnects six drivers.” “New scores belong to Alex.” “Requires a session restart.” These facts are more useful than configuration terminology.
- **Automate routine work, not consequential guesses.** Do not interrupt an occupied server, reattribute past results, relax access, or overwrite content without an explicit decision. A disabled action explains the exact blocker and how to resolve it.
- **Show quiet health.** No permanent wall of technical indicators. If telemetry is stale, keep the last timing values and state the last update time; do not label a server stopped without evidence. Content and recording failures stay close to the affected task.
- **People have one profile.** Keep the existing identity distinctions internally. Steam GUIDs and local guest identities must not be merged merely because their names match.
- **Design for the host holding a phone.** Labelled navigation, visible actions, large touch targets, native dialogs, keyboard support, and no hover-only operating controls.

## Visual direction

Warm neutral surfaces, dark olive text, restrained terracotta actions, generous type, and track photography make the app feel like a place to enjoy driving. The homepage is welcoming; the live page becomes more compact and puts timing first. Photography moves into an optional circuit preview while operating. Watch uses a dark viewing surface, with the camera as the main focus and the actual track map beside it. Live is a simple camera wall. Day and night themes share the same hierarchy. Decorative telemetry, repeated KPI cards, and permanent configuration chrome are omitted.

## What the existing implementation supports

This proposal was grounded in the capabilities reference, current route map and Vue pages, setup and configuration APIs, queue behaviour, results and driver history, guest attribution, drift scoring, and capture code. A browser walkthrough checked the running homepage and the current new-race form. It was not a live operational test of the real servers.

| Proposal | Existing foundation | Work required for a dependable product |
| --- | --- | --- |
| Ready-made session | Event and preset CRUD, setup summary, configuration renderer | Compose defaults in one editor. For isolation, clone or privately own underlying presets. Avoid creating unfinished records before final save. |
| One-action start | Queue APIs, process start, render preview, instance state | An idempotent orchestration operation that saves, validates, targets the exact event, and starts without duplicate enqueues. Partial failures must offer a safe retry. A frontend click sequence alone is insufficient. |
| Automatic server choice | Instance list, suggested ports, per-instance schedules | Reserve resources atomically, handle simultaneous operators, verify installed engine/content, and recheck before launch. Conflict checks in this mock use a small fixed calendar only. |
| Session-centric history | Queue/event data, result files, persistent driver sessions | A durable identity linking the proposed complete session, its immutable settings snapshot, phases, drivers, and clips. Current backend “session” records are not all the same thing. |
| Safe Next driver | Guest roster, live-car guest assignment, historical attribution | Define handover at a run boundary and verify capture/score attribution across that boundary. Keep people/accounts distinct and preserve historical ownership. The prototype switches immediately after instructing the host to finish the run. |
| All connected camera sources | Configured driver streams, WHEP player/embed, per-instance stream health | Decouple camera health and playback eligibility from a driver’s game connection. Represent a simulator independently from the person using it; preserve spectator sources. |
| Linked Watch | Broadcast maps, timing, native WHEP playback, theater and manual capture | Reuse actual track-layout metadata and telemetry interpolation, keep stale/off-map states, enforce roles, and make player/recorder health independent. |
| Automatic clips | Capture worker, rolling buffers, run-to-media links | Surface source health and limits. An enabled checkbox is not proof that a camera can record. Extend automatic capture beyond drift only if useful triggers exist. |
| Driver invitation page | Public content downloads; authenticated live and profile pages | A deliberately scoped joining view, working join links, required-content information, and explicit treatment of private/password-protected sessions. This is a new product surface. |
| One profile interface | Account drivers and guest-driver APIs | A frontend view model with role-consistent access. Do not guess identity matches or expose operator controls to viewers. |
| Expert configuration | Existing settings/preset/stream/maintenance APIs | Preserve advanced functionality behind clear progressive disclosure, with truthful scope and restart explanations. |

The concept deliberately does not claim that public joining, conflict-free booking, atomic launch, or run-boundary handover are already solved. “Magic” should be reliable orchestration with legible outcomes.

## Prototype scope and validation

- Changes are confined to this concept directory. The production Vue frontend, Go services, database, presets, and live servers were not changed.
- Navigation, session setup, capacity/conflict checks, starting, scheduling, handover, driver creation, search, finishing, reuse, sample content import, stream setup, camera filtering/selection, linked map positions, telemetry-delay states, and simulated captures are interactive. State stays in browser session storage; Reset demo restores the examples.
- Camera views are explicitly labelled sample frames. Stream checks validate fields and simulate connectivity; no entered URL is fetched. Captures create local demo records, not video/image files. Inviting, backup/recovery, access, and server connection details remain explanatory panels. There are no network API operations or real operational actions. A results export contains demo data only.
- Scheduled time does not advance automatically in the demo. It demonstrates planning and conflict handling, not a background scheduler. Advanced options shown in explanatory panels are illustrative.
- A runnable scenario check uses the project's existing jsdom dependency: `node docs/pitlane-concept/check.cjs`. It covers the core session flows plus all-source/connected/standby filters, stream validation, linked camera/map focus, missing/offline/spectator views, shared-rig capture ownership, session-end recording boundaries, persistence, all three track maps, and animation cleanup/telemetry delay. Map updates use a controlled test clock.
- Before the Watch/Live expansion, browser layout checks covered ten destinations at 375, 768, 1024, and 1440 CSS pixels, with no horizontal overflow. Desktop and phone theme checks and a targeted keyboard/dialog walkthrough were also performed. These are prototype checks, not a complete accessibility audit. Fresh browser checks after the Garage expansion covered Garage, Rigs, rig details, content library/details, Advanced, Watch, and Live at the same four widths, with no document overflow or broken images. Radio selection/cancel, dialogs, phone layouts, and themes were inspected.

See [ADVANCED-COVERAGE.md](ADVANCED-COVERAGE.md) for the complete Garage/Advanced capability catalogue, what is interactive, what remains a mapped design, and backend integration boundaries.

See [FEATURE-PARITY.md](FEATURE-PARITY.md) for the repository-backed comparison, the restored Watch/Live capabilities, and the remaining functionality that should survive the redesign.

The first production slice I would implement is **experience → ready setup → exact session start → useful live view**. That removes the largest cognitive burden immediately. The shared-rig handover and automatic recap would follow as the most distinctive daily benefits.
