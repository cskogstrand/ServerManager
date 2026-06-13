Done:
- ~~Indicate running server on settings/instances; disable edit/delete buttons.~~ (was already implemented: status dot, disabled buttons, "Stop the server to edit or delete" hint)
- ~~Add toggle to start server on boot.~~ (per-instance `start_on_boot` toggle added)
- [x] Save protection if dirty on navigate on all forms and pages (useUnsavedGuard on Settings config/user, Content, instance + driver modals, Events builder; presets already had it)
- [x] Ability to set custom name on events (user_event.name column; editable in Events builder; shown in lists/queue/dashboard/detail; appended to lobby server name)
- [x] Servers on dashboard takes up too much space. We have the serverdetail page for a reason. Strip the serverlist page down. (dashboard now compact one-card-per-instance overview; map/live timing/grid editor/race control/console moved to /server/:id)
- [x] On the queue page, the Event group dropdown is only showing 3 of the 5 groups I have added. (Queue dropdown used `?filled=1`; category `filled` is 0 until a group is renamed, hiding created-but-unrenamed groups. Dropped the filter so all groups show, matching the Events page.)
- [ ] On the content page, the "installation" part should be moved to the configuration page.
