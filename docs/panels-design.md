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

## Build notes

- Vanilla ES modules and patchbay tokens only. The prototype used `pb-btn pb-xpt` cells, `pb-toolbar`, `pb-toast` and the Lucide sprite. Orange and the muted idle colours need new semantic tokens upstream, since `--pb-brand` is for navigation chrome.
- Status is icon first, with optional text. This is a documented exception to patchbay's "status always with text", and AGENTS.md needs a line for it.
- One animated root custom property (`@property --pulse`) drives every pulse, so cells stay in phase across re-renders.
- Settings for a panel (mode, guard, multi-destination, colours, levels, kiosk, access, sharing, idle clear) are not designed yet.
