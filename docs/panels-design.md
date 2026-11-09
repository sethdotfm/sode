# Sode Panels: design

Validated by prototype on branch `prototype/panels` (three variants, kept as the primary source). Terms are in GLOSSARY.md under Panels. ADR 0006 covers who may create a panel's API client.

## What a panel is

A saved 4x8 grid of cells on its own URL (`/panels/<id>`), driving the router through `/api/v1` only. 4x8 is the default size, chosen so a panel maps one to one onto a hardware key deck later. Rows and columns become user settings, and merged cells stay inside the grid. The editor page `/panels` signs in like the rest of Sode, edits panels live and launches a user's panels.

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
- Unassigned cells: a cell with a kind but no Source, Destination or Preset is shown dashed. The toolbar counts them and a Next button selects the next one. A live Test leaves them out.
- Templates: a template is a saved layout with button types, optionally the panel settings and the cell contents. By default cells come back unassigned, so a show's Sources and Destinations are filled in afterwards. Templates are saved to Me or Everyone, and saving to Everyone needs a Trusted user or the Advanced Settings PIN.
- Push: copies a panel's saved config to other panels. Each target keeps its own name, id and sharing, and updates the way a saved edit does. A checkpoint of each target is kept first. Targets are picked by group or one by one. A checkpoint set stores one saved layout per panel in a group, so one push gives EVS 1 its own and EVS 2 its own. A panel's owner can push to their own panels. A Trusted user can push to anybody's.
- Artwork: a cell can show a vendored icon, or an uploaded JPEG, PNG, GIF or SVG up to 1 MB. Images are shown dimmed behind the caption so the state colour still reads. SVG is only ever used as an image, so scripts in it never run.

## Monitoring

Proposed, not prototyped.

- Each open panel page is an instance with a device name given the first time it opens. It sends a keepalive every 15 seconds. Sode keeps the last seen time and state for each instance, and a panel is online while at least one instance is.
- A user can watch panels. A user can watch the panels they own, and a Trusted user can watch any panel. The watch list is a set of checkboxes in the editor and in the My panels page.
- When a watched panel loses its last instance, or its keepalive is missed for more than 45 seconds, every watcher is notified: a toast and a badge in Sode, and a browser notification if the user allowed them. A second notice goes out when it comes back. The notice names the panel, the device and how long it was missing.
- Open questions: whether a panel can say how many instances it expects, so a drop below that count alerts, and whether watching also covers a panel's API client key being used from somewhere new.

## Build notes

- Vanilla ES modules and patchbay tokens only. The prototype used `pb-btn pb-xpt` cells, `pb-toolbar`, `pb-toast` and the Lucide sprite. Orange and the muted idle colours need new semantic tokens upstream, since `--pb-brand` is for navigation chrome.
- Status is icon first, with optional text. This is a documented exception to patchbay's "status always with text", and AGENTS.md needs a line for it.
- One animated root custom property (`@property --pulse`) drives every pulse, so cells stay in phase across re-renders.
- The settings in the editor's sidebar are a first pass for the prototype. Several of them, such as keeping the screen awake, haptics and the networks allowed, are placeholders and may change.
