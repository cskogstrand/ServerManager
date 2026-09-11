# Pitlane visual follow-up

11 September 2026. The earlier functional migration retained old presentation in the Drivers directory, both profile types and several detailed tools. This pass addresses those gaps in the production Vue app.

| Area | Finding and change |
| --- | --- |
| Drivers | Replaced the trophy heading, boxed score cards and separate desktop/mobile leaderboard markup with the concept's people directory: open statistics, circular portraits, quieter rows and pill filters. Search, URL filters, sorting and distinct account/guest links remain. |
| Account and guest profiles | Removed the gridded gradient hero, oversized badges and boxed metrics. Both use a spacious identity heading and an open statistics strip. Notes wrap; account GUID is available under Account details. Photo, history, attribution and media controls remain. |
| Shared components | Updated PageHeader, Card, Button, fields, avatars, empty states and session cards. Detailed editors now use the same type scale, panel spacing, borders and colors as Today and Garage. Consolidated duplicated theme tokens and removed unused Paddock shell CSS. |
| History and search | Updated the manual search heading and history metrics. Corrected Find a drive to open `/sessions/drives`, rather than the new session hub. |
| Detailed tools | Applied context headings to content, reusable setups, presets, running order, configuration, installation, instances, streaming diagnostics, maintenance and accounts. Updated standalone server-control and club-record headers and the sign-in copy. Existing domain editors and operations remain available. |
| Navigation | New route paths start at the top, browser history restores its saved position, and query-only filtering keeps the current position. Verified a directory-to-guest navigation that previously landed partway down the profile. |

## Verification for this pass

- Frontend: **26 files / 88 tests passed**, including a new directory test covering account/guest links, viewer restrictions, sorting and query filters.
- Vue type checking and Vite production build passed. The Go embed build passed in Docker and the disposable preview was refreshed on localhost:3031. The updated Drivers page was inspected from that embedded build.
- Account profile: no horizontal overflow at 375, 768, 1024 and 1440 CSS pixels. Light desktop and dark phone views inspected.
- Directory and guest profile: desktop and 375-pixel phone layouts inspected. Configuration and preset library fit at 375 pixels; history and its search destination inspected on desktop.
- Browser checks retained the guest/account destinations and confirmed the corrected history-search link and top-of-page profile navigation.
- Updated screenshots are in `pitlane-screenshots/README.md`. The screenshot data belongs to the disposable fixture.
- `git diff --check` passed. Generated TypeScript metadata was restored and the pre-existing staged/deleted artwork was preserved.

This was a frontend presentation pass. The earlier backend, protocol, migration and fullscreen results remain recorded in `PITLANE_IMPLEMENTATION_PROGRESS.md`; those complete suites were not repeated here. Physical simulator and external video acceptance remain separate.

## Complete profile body

The user correctly identified that the initial follow-up still retained the old composition below the profile statistics. The next pass replaces that body rather than only changing its tokens:

- Shared account/guest favourites use large installed car/track imagery and open captions. Missing data uses quiet text, without empty nested cards.
- Recent drives are open photographic rows. Details expand to readable phase results, lap tables, drift runs and captures. Tags, download/delete controls and reassignment remain available within the expanded drive.
- Personal highlights sit alongside the history, with larger media previews, readable captions and the existing viewer, download, deletion and attribution operations. The account camera and live-session sections follow the same spacing and hierarchy.
- Guest results and media use the same composition. Read-only guest media retains its driver credit and download/view actions.

Verification: **26 test files / 89 tests passed**. The added history regression exercises tagging, result rendering, media events, reassignment and deletion. Type checking, production frontend build and Docker Go embed build passed. Expanded history fits 375, 768, 1024 and 1440 CSS pixels without document overflow. Browser inspection covers the lower profile, expanded lap table, image viewer, light/dark phone views and guest layout. Port 3031 serves the refreshed isolated build.
