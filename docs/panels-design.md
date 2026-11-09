# Sode Panels: design

Validated by prototype on branch `prototype/panels` (three variants, kept as the primary source). Terms are in GLOSSARY.md under Panels. ADR 0006 covers who may create a panel's API client.

## What a panel is

A saved 4x8 grid of cells on its own URL (`/panels/<id>`), driving the router through `/api/v1` only. 4x8 is the default size, chosen so a panel maps one to one onto a hardware key deck later. Rows and columns become user settings, and merged cells stay inside the grid. The panels page `/panels` signs in like the rest of Sode and launches a user's panels. The editor, at `/panels/editor`, edits them live.

Storage: one YAML file per panel under `panels/`, with an `id` and a display name inside the file. A user makes their own panels with their existing Rights. A Trusted user may also make a panel that acts as its own API client, with Rights no wider than theirs. Panels can be shared use-only with named users or Everyone, and a user can save a copy to edit.

## Behaviour

- Pressing a Destination cell views it. The grid then shows what that destination has now: its current source is solid green and every other source is idle.
- Pressing a Source cell stages it for the viewed destination, and the staged source pulses green. Staged destinations pulse orange and show the staged source under their name.
- Take applies every staged change. Clear drops them all and the view. Pressing a staged destination again unstages just that one.
- Multi-destination is a panel setting, on by default. Off, one change is staged at a time.
- A Preset cell stages the preset's routes, so it can be previewed on the viewed destination, and works in the panel's mode. Take mode arms the Take cell. Instant mode makes pressing a Source or Preset the Confirm.
- A Take cell arms once something valid is staged. Optional guards are a hold or a double press.
- Current Destination shows only the source now on the viewed destination, under the caption "Currently".
- Idle cells are the active colours muted into the control surface, so user-chosen colours carry through. The pulse breathes between about 72% and 100% of the state colour on one shared 4-second cycle, with static rings so state survives reduced motion.
- After a take, a destination shows its Result as an icon (connected, already, failed). Result text is optional and off by default.
- Refused before arming, with an error: a Protected destination, a route outside the panel's Rights, and a Source and Destination with no common level. Levels are set per panel and per Source or Destination cell, and a take sends the intersection.
- A pending selection clears after 30 idle seconds. A saved edit refreshes open panels once no destination is viewed or after 10 seconds, with a banner and a 60 second snooze.

## Authentication

Session mode acts as the signed-in user, with that user's Rights, and the page uses a user key tied to the session. API mode uses the panel's own API client key. A kiosk needs no sign-in: by default the panel URL itself is the credential for that client's Rights. A secure mode exchanges a one-time setup link for a key stored in the browser.

## Editor

The editor is the page where a panel's grid and settings are made. It lives under the panels root as its own page (`/panels/editor`), apart from the pages for viewing, sharing and signing in to panels. The prototype is variant F on branch `prototype/panels` (`prototype/panels/editor/f`, served on :8116 by `serve-editor.mjs`).

- Layout: a toolbar with the rows and columns (typed or stepped), the panel picker in the middle (My panels, then Shared with me as use only, then New panel), Templates, Push, Test it live and Save. The grid is in the middle and the sidebar has Cell, Panel and Access tabs. The sidebar opens on Panel and switches to Cell when a square is selected. Cell holds the button type, a fuzzy picker for the contents, a caption that replaces the Router name, levels, font size and artwork. Panel holds identity, appearance (title bar, font size, backdrop image), behaviour, device and feedback, colours and updates. Access holds who the panel acts as, limits and sharing.
- Selecting: click a square, or drag across several. Selecting never changes the grid by itself.
- Smart menu: a small list sits beside the selection, to its right, else its left, else below it. Merge and Unmerge act on click. Type and Find widen on hover or focus. Type shows the cell kinds for the whole group, plus Empty to remove them. Find searches Sources, Destinations and Presets at once, with an icon for each category and filter chips to narrow to one. The results respect the signed-in user's Rights and Visibility: Hidden removes what is outside the Rights, Read-only greys it and marks it.
- Filling: choosing a result fills the group from that item onward in number order, using the fill order picked in Find (across, right to left, snake across, down, bottom to top, snake down, each shown as a small path icon). A preview shows in the grid before anything changes. Shift-click or Shift+arrows pick a set of results instead, and Enter or the Fill button then places those in order. With one square selected, a click fills it, moves the selection to the next square and moves the highlight to the next result. The menu stays put, so clicking the same spot again fills the next square.
- Keys: arrows move the selection and Shift extends it. Typing starts a search. Enter opens Find. Tab moves into the menu, arrows walk it, the arrow toward the flyout enters it and Esc backs out. Delete removes. Ctrl+J and Ctrl+K (Cmd on Mac) step a result list, as in the rest of Sode.
- Corner grip: dragging the grip at a selection's corner extends its numbering in that direction, with a tip showing the first and last name and the count.
- Unassigned cells: a cell with a kind but no Source, Destination or Preset is shown dashed. The toolbar counts them and a Next button selects the next one. A live Test leaves them out: Test it live saves any edits, then opens the saved panel in a new tab, and the panel page already drops cells without a Source, Destination or Preset.
- Templates: a template is a saved layout with button types, optionally the panel settings and the cell contents. By default cells come back unassigned, so a show's Sources and Destinations are filled in afterwards. Templates are saved to Me or Everyone, and saving to Everyone needs a Trusted user or the Advanced Settings PIN. Built in P16: files in `templates/me/` and `templates/everyone/` beside the config, routes `/api/v1/panel-templates` (not under `/panels/`, where `DELETE /panels/templates/{id}` would collide with `/panels/{id}/key`), and the server drops cell contents unless the save says `contents: true`, so an Everyone template never carries a Source or Destination by accident. A REST call has no Settings session to hold the PIN, so the PIN rides in the save body as `pin` and counts in the same wrong-PIN pool as Settings. The editor's Templates button (`web/editortemplates.js`) lists Me and Everyone, applies a template to the open grid as an unsaved change, and asks for the PIN only after the server refuses Everyone. An applied template's settings are staged by `stageSettings` in `web/paneleditor.js` and merged into the next save.
- Push: copies a panel's saved config to other panels. Each target keeps its own name, id and sharing, and updates the way a saved edit does. A checkpoint of each target is kept first. Targets are picked by group or one by one. A checkpoint set stores one saved layout per panel in a group, so one push gives EVS 1 its own and EVS 2 its own. A panel's owner can push to their own panels. A Trusted user can push to anybody's.
- Artwork: a cell can show a vendored icon, or an uploaded JPEG, PNG, GIF or SVG up to 1 MB. Images are shown dimmed behind the caption so the state colour still reads. SVG is only ever used as an image, so scripts in it never run.

## Panels page

`/panels` lists panels and launches them. The editor (`/panels/editor`) and a panel itself (`/panels/[id]`) are separate pages. The prototype is `prototype/panels/overview/c` (:8123 with `serve-overview.mjs`). Variants A (table) and B (tile gallery) are history.

- Layout: a list on the left, grouped under My panels and Shared with me, each row with a mini picture of the grid, its owner, whether it acts as a user or an API client, and an online dot. The right side shows the chosen panel: a large Launch button, Edit, Share, Link with key, Push to panels and Delete, then the instances open now and a Watch checkbox.
- A panel shared with you opens for use. It shows Save a copy instead of Edit.
- A Trusted user sees every user's panels, grouped by owner, and can edit, share, push to and watch any of them.
- Delete needs a second press.
- Share is a modal with two tabs. People sets Only me, Named users or Everyone. Link with key is for a panel that acts as an API client: the key sits in the URL fragment (`#key=`) so it never reaches a request line or proxy log, the link is masked until revealed, and the tab offers Copy link, a QR code, Open as kiosk, Rotate and Revoke (each confirmed). A panel that acts as a user has no key and asks for a PIN at launch.
- Rotating a key signs out open instances and kills old links. Revoking leaves the panel without a key.
- Share as built: `panelshare.js` opens the modal (`openShare(panel, {api, onSaved})`); another tab joins by pushing a function onto `shareTabs` that returns `{id, text, panel}` or null. People saves only the `share` field through `PUT /api/v1/panels/{id}`. The picker reads `GET /api/v1/panels/people` (names only, a user's key only). Pure rules (`shareDraft`, `pickList`, `shareBody`, `shareChanged`) are in `panelsrules.js`.

## Monitoring

Proposed, not prototyped.

- Each open panel page is an instance with a device name given the first time it opens. It sends a keepalive every 15 seconds. Sode keeps the last seen time and state for each instance, and a panel is online while at least one instance is.
- A user can watch panels. A user can watch the panels they own, and a Trusted user can watch any panel. The watch list is a set of checkboxes in the editor and in the My panels page.
- When a watched panel loses its last instance, or its keepalive is missed for more than 45 seconds, every watcher is notified: a toast and a badge in Sode, and a browser notification if the user allowed them. A second notice goes out when it comes back. The notice names the panel, the device and how long it was missing.
- Open questions, decided below: no expected instance count, and no alert for a key used from somewhere new.

## Monitoring (as built)

- Modules: `panelmon.go` (devices, watches, sweep, notices), `panelkeepalive.js` (device name form and the 15 s keepalive), `panelwatch.js` and `panelwatchrules.js` (toast, badge, browser notification, text), `panelsmon.js` (online mark, Open now list and Watch checkbox on the panels page), `panelmon.css`. Routes and fields are in docs/api.md.
- An instance is a device name, ignoring case, so a reload or a rebooted kiosk with the same name is the same instance and raises nothing. The name is asked once per browser (`sode-panel-device`). There is no leave message: a closed page is noticed after the 45 s miss. A panel is online while any device was heard within 45 s. Devices live in memory only, so a restart of Sode alerts nothing. A lost device is dropped after 24 hours.
- A sweep runs from `periodic` and tells each watcher once when a device passes 45 s, and once when it returns, naming panel, device and seconds, with `offline` true when no device is left. Each change is one `panelLost` or `panelBack` row in the Communication Log.
- Watchers: the owner or a Trusted user, checked again when a notice goes out. The list is `panelwatch.yaml` beside the config, never in a panel file, and the API only sends the caller's own `watching`. Notices go to connections signed in as the watcher and not under the Lock. `panelsOffline` rebuilds the badge after a reload or sign-in; a notice sent while nobody was signed in is not kept.
- Deleting a panel forgets its devices and watches. Making, rotating or revoking its key forgets its devices, so an ended kiosk does not alert.
- A panel client's keys share one rate limit that scales with its live devices, since each page polls every 2 s. Its streams are capped at 64 per key and not counted per address.
- Decided: no expected-instance count, and no alert for a key used from a new place.
- Not built here: the Watch checkbox in the editor (the editor page has no script yet; mount `watchBox` from `panelsmon.js`), Push (P17) and live update (P15).

## Panel page (as built)

- Modules: `panelmodel.js` (pure state model: view, stage, arm, Take, results, idle clear), `panelrules.js` (URL, API answers, cell checks, error text), `panelview.js` (draws the grid), `panelrun.js` (loads the panel, polls, sends the take), entry `panelpage.js`.
- Stored cell: `{ r, c, rs?, cs?, kind: source | destination | take | preset | clear | current, n?, scope?, name?, caption?, levels? }`, with `r` and `c` from 1. A Preset cell holds `scope` (`everyone` or `me`) and `name` instead of `n`; `current` is the Current Destination cell. Cells that are outside the grid, unknown, a Source or Destination without `n`, a Preset without both, or overlapping an earlier cell are not drawn. Settings read: `mode`, `multi`, `levels`, `guard` (`hold` or `double`, anything else is none).
- A Preset cell stages every route of the preset (`GET /presets`, polled with names) and views its first destination. One refused route (Protected, no common level) refuses the whole preset. With `multi` off a preset with more than one destination is refused. A taken route is sent as an ordinary `POST /takes`, so History shows routes, not the preset name. Clear and Current Destination have no stored fields beyond `caption`; Current is disabled.
- Instant mode: `startInstant` is called right after a Source or Preset press, and one route set is held at a time, so a new press replaces a failed one. The Take cell stays disabled. Guards apply to the Take cell only: `double` needs a second press within 1.5 s, `hold` 0.8 s of pointer or Enter and Space held (`guardDown`, `guardHeld`, `guardCancel`).
- A browser cannot send a Bearer header on `/api/v1/stream`, so the page polls `/tally` and `/protected` every 2 seconds and `/names` every 30. A take is `POST /takes`, which is its own Confirm, with an `Idempotency-Key`.
- Live update (P15): the page reads `GET /panels/{id}` every 5 seconds and compares `rev`, which `panelStore.writeLocked` moves on every stored change. A newer copy applies at once when no destination is viewed or staged, otherwise after 10 seconds, never during a take; a banner says so and Snooze 60 s holds it (the 10 seconds restart when the snooze ends). The rules are `panelupdate.js`, the poll and banner `panellive.js`. A Push (P17) only has to store the target panel through `panelStore.replace` and open panels follow.
- The page cannot read a panel's mode before it has a key. A key in the URL fragment (`#key=`) means API mode. Without one the page starts a session key (Session mode) and, if the panel then turns out to act as an API client, asks for its link with the key instead of running.
- A failed route stays staged after a take, so Take can be pressed again. A level with nothing routed is ignored when finding the current source. `GET /panels/<id>` carries the caller's own Rights as `rights`, and the page refuses a Source or Destination outside them (and a Preset outside them) before arming, as `outside-rights`; hidden Visibility makes that message name nothing. Levels the Rights exclude drop out of the take, and none left is `no-common-levels`. The server refuses the same route anyway, so keep `rightsAllow` in step with `config.Rights`. Setting `resultText` adds the Result word beside its icon. Kiosk (`settings.kiosk` or `#kiosk=1`) hides the header; `TestChromePanelKioskAndRefusal` covers it.

## Look and device settings (as built)

- Modules: `panellook.js` (pure: reads settings and cell artwork), `panelart.js` (artwork fetch), `panelfeel.js` (wake lock, haptics), with the page wiring in `panelrun.js` and `panelview.js`. Tests in `web/test/panellook.test.mjs` and `panelart.test.mjs`.
- Panel `settings`: `titleBar` (false hides the title bar, the header with the name and the Panels link; kiosk hides it too), `fontSize` (percent, 50 to 200, default 100), `backdrop` (an artwork id), `wake`, `haptics` (both true to switch on). Cell fields: `art` (`icon:<vendored icon name>` or an artwork id) and `fontSize` (percent, multiplies the panel's).
- Colours: an idle Source is `--pb-current-idle`, an idle Destination `--pb-view-idle`, both the state colour muted into `--pb-surface-control`. Viewed and Current are solid. Staged, pending and an armed Take swell between 72% and 100% of the state colour on `--pulse`, and each also has a static inset ring, so reduced motion (which freezes `--pulse` at 0.6) still shows state.
- Artwork: the route needs the Authorization header, so the page fetches it with the panel's key and shows a blob URL in an `<img>`. The backdrop is a CSS background from the same blob URL. SVG is never inlined. The CSP allows `blob:` for `img-src` only. A fetch that fails leaves the cell without artwork. An icon name missing from the vendored sprite draws nothing.
- Keep the screen awake: Screen Wake Lock, requested when the page opens and again whenever it becomes visible, since a browser drops the lock when the page hides. A refusal is ignored. Haptics: one 15 ms `navigator.vibrate` on each press the page accepts. Browsers without the API (iOS has no vibrate) do nothing. `featureSupport(navigator)` in `panelfeel.js` says what the browser has, for the editor to hide or disable the two switches. Result of the spec's proposal: built as described, with no extra settings.

## Push as built (P17)

- Server: `panelpush.go` (layout, checkpoints, set store) and `apipush.go` (routes). A layout is `rows`, `cols`, `cells`, `settings` and unknown fields; id, name, owner, `share`, `acts`, the API client and its Rights are never copied, and missing artwork is. `panelStore.applyLocked` writes the checkpoint before the panel, so a checkpoint that cannot be saved stops the Push, and the panel write moves `Rev` for open panels (P15).
- Reach: the pusher must be able to change the source and every target (`panelReach`); one refused target changes nothing. Checkpoints sit in `panels/<id>.checkpoints`, at most 10 per panel, and a restore keeps what it replaces. Sets are in `panelsets/` (`everyone.yaml`, `me-<hex of user>.yaml`); an Everyone set needs a Trusted user to save and to change someone else's.
- A group is the panels whose names differ only by a trailing number (EVS 1 to EVS 4 are "EVS"), worked out by the page (`pushGroups` in `panelpushrules.js`); the server only knows ids. `pushTargets` is the tested selection rule.
- Web: `panelpush.js` is the modal (Push, Checkpoint sets and Restore tabs), opened by Push to panels in the panels page's detail bar. The editor has no Push control yet.

## Build notes

- Editor Find: `findrules.js` is pure (entries with Rights and Visibility, ranking, the run a choice places, the plan and `applyFind`, which sets a Preset cell's `scope` and `name`). `editorfind.js` is the flyout in the smart menu: it plugs in through `addMenuSection` and `addKeyHook`, catches `myRights` at import time because the message can arrive before the editor loads, and previews by marking the grid's squares in place. Fill order is `setOrder(id)`, default `across`.

- Vanilla ES modules and patchbay tokens only. The prototype used `pb-btn pb-xpt` cells, `pb-toolbar`, `pb-toast` and the Lucide sprite. Orange and the muted idle colours need new semantic tokens upstream, since `--pb-brand` is for navigation chrome.
- Status is icon first, with optional text. This is a documented exception to patchbay's "status always with text", and AGENTS.md has a line for it.
- One animated root custom property (`@property --pulse`) drives every pulse, so cells stay in phase across re-renders.
- The settings in the editor's sidebar are a first pass for the prototype. Several of them, such as keeping the screen awake, haptics and the networks allowed, are placeholders and may change.
