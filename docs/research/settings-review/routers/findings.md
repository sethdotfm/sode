DISCLOSURE: This was written by an LLM.

# Settings > Routers: UX/UI review

Branch `alpha-round1` at ae0d7e15. Review only: no app code was changed.

## How it was tested

- 12 swp08sim routers on ports 9310 to 9321 (the 8910 range was in use by other agents). Each simulator had a `SIM_NAMES_FILE`, so destinations 100 and 101 on one router have the same names as sources 150 and 151 on the next router (tie line suggestions).
- Sode from the branch, `SODE_CONFIG` on a scratch file. The first run went through the setup wizard (Administrator "Seth" with a PIN). After that, generated `config.yaml` files (format 2, a `routers:` list) gave 1, 2, 6 and 12 routers, with two tie lines between each pair of neighbors.
- Router titles mix short and long ("Stage 2", "Production Control Room Backup"), and level counts mix 1, 2, 4, 8, 16 and 17. Router A was left without a title, as the wizard makes it.
- Down routers: the simulators of routers A, C and G were stopped with 12 routers.
- Headless Chromium through the DevTools protocol, 1920x1080 and 1280x800, signed in as an Administrator.

Screenshot numbers: 00 wizard, 01 one router, 02 add router, 03 two routers, 04 tie lines, 05 remove router, 06 review and errors, 07 six routers, 08 twelve routers, 09 routers down, 10 change an address, 11 to 15 single issues.

## Size of the Routers topic

Height of the Routers topic (from the "Routers" heading to "Access"), at 1920x1080:

| Routers | Height | Screens at 1080 | Screens at 800 |
| --- | --- | --- | --- |
| 1 | 1859 px | 1.7 | 2.3 |
| 2 | 2607 px | 2.4 | 3.3 |
| 6 | 4297 px | 4.0 | 5.4 |
| 12 | 6880 px | 6.4 | 8.6 |

The content column is 640 px wide at both window sizes. At 1920 about two thirds of the window is empty.

## Clicks for common tasks

Counted from Settings open and an Administrator signed in. "Type" is a field entry, not a click.

| Task | Steps |
| --- | --- |
| Add a router | Add router, type address (and title, levels), Save, Save 1 change, type PIN, Continue. 4 clicks. Every other setting of the new router (level offset, numbering, default levels) is in a different section further down. Most settings are not available at all (R-03). |
| Change the address of router B | Routers in the list on the left, type in the row, Save, Save 1 change, type PIN, Continue, "Save and reconnect to *router A's address*?" (R-05). 5 clicks. |
| Change the address of router A | "Router & matrix" in the list on the left (or the "Router" button in router A's row), type, Save, Save 1 change, type PIN, Continue, Save and reconnect. 5 clicks, in a different place from every other router. |
| Add a tie line by hand | Tie lines in the list on the left, scroll past every tie line and suggestion, From router, type destination number, To router, type source number, Add tie line. 3 to 4 clicks, and the person must know both numbers. Saves at once: no review, no PIN. |
| Accept a suggested tie line | Accept. 1 click, saves at once. |
| Remove a tie line | Remove. 1 click, saves at once, no confirm (R-18). |
| Remove a router (not A) | Remove…, Remove router B, Save, Save 1 change, type PIN, Continue. 5 clicks. Router A cannot be removed. |
| Reorder routers | Not possible in the UI (R-07). |
| Swap a number on router B | Not possible: Swap… works only on router A (R-06). |

## Findings

Severity: high = blocks a task, gives wrong information, or loses data; med = slow, confusing or inconsistent; low = polish.

### Router A is not like the others

**R-01. Router A has a different row in the Routers list. Severity: high.**
Screenshots: 03-two-routers-top-router-a-row-vs-router-b-block-1920.png, 07-six-routers-top-1920.png, 07-six-routers-top-1280.png.
Router A is one line: a ghost button with its title ("Router" when it has none), its address as read-only grey code, Swap…, and the Broken count. With two or more routers a Prefix field appears at the right end of that line, with its label above it, out of line with everything else. Every other router is a block with the heading "ROUTER B", Remove…, and editable Prefix, Title, Address and Matrix fields. Router A shows no "Router A" heading and has no Remove….
Direction: one row shape for every router. Things that are not about one router (Swap, Broken count) move out of router A's row.

**R-02. Router A's settings are in their own block at the bottom of the topic. Severity: high.**
Screenshots: 03-two-routers-router-levels-then-router-a-block-1920.png, 08-twelve-routers-router-a-block-at-bottom-1920.png, 01-one-router-router-a-block-1920.png.
Router A's Address, Name, Status label, Matrix, Levels, Level offset, Level numbering and number offset are under "Router > Router & matrix". Its name settings are under "Router names", its default levels and fire mode under "Takes", and its device number under "Protects". The block comes after the Routers list, Tie lines and the Router levels of every other router. With 12 routers it starts about 5900 px below the Routers list. The block heading is router A's title, or "Router" when it has no title. It never says "Router A". The Router levels note sends the reader there: "The levels of router A are in its own block below."
Direction: one place per router for all of its settings, the same for A and the others.

**R-03. Routers after the first cannot set most of their settings. Severity: high.**
Screenshots: 03-two-routers-router-a-names-takes-protects-1920.png, 11-router-b-matrix-16-refused-no-name-requests-field-1920.png.
`config.Router` has the same fields for every router, and `config.yaml` stores them for each one. In the UI, router B and later have only Prefix, Title, Address, Matrix, Levels, Level offset, Level numbering and Default levels. They have no Status label, Source and destination numbers, Name character limit, Name requests, Re-read every, Re-read routes every, Multi destination, Router salvo group or Sode's device number. The device number is 0 (off), so Sode cannot Protect a destination on router B to L. Router B also has no names status and no Re-read now.
Direction: give every router the full set of its settings. Settings that belong to the server and not to a router (see R-22) leave the router block.

**R-04. Router B cannot use a matrix above 15. Severity: med.**
Screenshot: 11-router-b-matrix-16-refused-no-name-requests-field-1920.png.
The Matrix field of router B accepts 0 to 255, and the page check lets 20 through. After Save, review and PIN, the server refuses: "Router B: matrix must be 0-15 with standard name requests." Name requests cannot be changed for router B (R-03), so the person cannot fix it.
Direction: follows from R-03. Also check the matrix range on the page against Name requests.

**R-05. The reconnect confirm names router A's address for a change on any router. Severity: high.**
Screenshots: 10-change-address-router-b-confirm-names-router-a-1920.png, 06-reconnect-confirm-names-router-a-only-1920.png.
A change to router B's address only asks "Save and reconnect to 127.0.0.1:9310?", which is router A's address. With both addresses changed, it names only router A's new address. `web/settingssave.js` passes `address: now.router` (router A's form field) to the review, and `web/savereview.js` puts it in the title. It also comes after the PIN, so the person is asked "are you sure" after they typed their PIN.
Direction: name each router that reconnects, by prefix and title, and its new address. Think about the order of the PIN and the confirm.

**R-06. Swap… works on router A only. Severity: high.**
Screenshot: 12-swap-dialog-no-router-choice-1920.png.
Swap… is on router A's row only. The dialog has no router choice, and `web/swap.js` always sends `router: routerData('').uid`, which is router A. A swap of a number on any other router is not possible.
Direction: let Swap choose the router, or put it on each router.

**R-07. Router A cannot be removed or moved, and the order cannot change. Severity: med.**
Screenshots: 03-two-routers-top-router-a-row-vs-router-b-block-1920.png, 05-remove-router-b-note-1920.png.
There is no reorder. Router A has no Remove…, and the server refuses it with "the first router stays: change its address in its own block" (`internal/app/routeradd.go`). docs/tielines-design.md, "Other stores" rule 4, says the same. The order also decides the default router of a new browser (tielines-design "Router prefix and typing" rule 5) and the order of the top bar and the Degraded note. To make another router first, a person must edit `config.yaml` by hand. A change of prefix does not change the order.
Direction: decide if "first" should mean anything. If it should, make it a setting that any router can have (for example "default router") and let routers be reordered.

**R-08. Level settings are split, and three different words name the same choice. Severity: med.**
Screenshots: 03-two-routers-router-levels-then-router-a-block-1920.png, 00-wizard-router-step-no-title-or-prefix-1920.png.
Router A: Levels and Level offset in "Router & matrix", Level numbering as three buttons "0–16 / 1–17 / V + 1–16", Default levels in "Takes". Other routers: all four in "Router levels", with Level numbering as a select "Start at 1 / Start at 0 / V, then 1". The setup wizard says "1 to 17 / 0 to 16 / V + 1 to 16". "Start at 1" and "Start at 0" are also the words for "Source and destination numbers" in router A's block, which is a different setting.
Direction: one control and one wording for Level numbering everywhere. Keep each router's level settings together.

**R-09. Title, Name or router name: one field, many names. Severity: med.**
Screenshots: 01-one-router-router-a-block-1920.png, 07-six-routers-tie-form-and-router-levels-1920.png, 09-down-main-page-for-comparison-1920.png.
The file key is `title`, the rows say "Title", router A's block says "Name" with the placeholder "Ultrix", the hello sends it as `routerName`, and the validation error says "router F: router name must be at most 32 characters". GLOSSARY.md calls it Router title, and "router name" is the glossary word for a source or destination name. A new router gets the title "Router B" by default (`routeradd.go`), but the wizard leaves router A blank, so router A shows as "Router". The tie line selects show "A" and "B Stage 2". The Degraded note says "Router A is down" and "Router C Stage 1 Small A is down".
Direction: one word (Router title) everywhere, and the same default for every router.

**R-10. The review dialog and the errors leave out router A's prefix. Severity: med.**
Screenshots: 06-review-router-a-unprefixed-vs-router-b-1920.png, 10-change-address-router-a-review-1920.png, 06-error-router-a-unprefixed-1920.png, 06-error-router-b-prefixed-top-1920.png.
A change on router B is "Router B: Address" under the group "ROUTERS". The same change on router A is "Address" under "ROUTER & MATRIX". The errors are "Not saved. Router B: router must be host:port…" and "Not saved. Router must be host:port…". `config.validateRouters` adds the prefix only for routers after the first. The field is called Address, but the error says "router must be host:port".
Direction: always name the router by prefix (and title) in review rows and errors. Say "address" in the address error.

**R-11. The API and the startup log speak of router A as "the router". Severity: med.**
No screenshot. Logs and responses from the 12-router run:

```
sode: http on :18090, router "127.0.0.1:9310" matrix 0, 17 levels (UI 1 → wire 0)
swp08: connected to 127.0.0.1:9312
names: 200 sources, 120 destinations        (once for each router, no prefix)
config: applied (reconnect=false)           (when Add router connected a new router)
```

`/api/health` with routers C and G down: `"state":"degraded"` and HTTP 503, but the top-level `"router":"up","targetAddress":"127.0.0.1:9310"` are router A's. With router A down too, the top level says `"router":"down"`. A monitor that reads the top-level `router` field gets the state of router A only. `GET /api/v1/status` has the same flat fields (`statusBody`, "The flat fields are of the first router"). In code, `RouterByUID("")` is router A, and `hiddenDstsRouter` blank means "the one router".
Direction: name each router by prefix and title in log lines. Decide if the flat health and status fields stay, and if they do, mark them as old in docs/api.md.

**R-12. In memory, router A is a different kind of value. Severity: med (root cause).**
No screenshot. `config.yaml` is even: one `routers:` list. In memory, `Config` embeds router A as `Router` and holds the others in `MoreRouters` (`internal/config/config.go`, `router.go`). The hello sends router A as flat keys (`router`, `routerName`, `matrix`, `levels`…). The Settings save keys are flat for router A and `router/<uid>/<attribute>` for the others. The HTML has fixed ids (`router-1`, `data-router="1"`, `cfg-router`). Most of R-01 to R-10 come from this split.
Direction: one list of routers in memory and on the wire, so the UI can draw every router the same way.

### Layout at scale

**R-13. The topic grows by about 230 px for each router, mostly empty space. Severity: med.**
Screenshots: 07-six-routers-top-1920.png, 08-twelve-routers-top-1920.png, 08-twelve-routers-router-levels-1920.png, 07-six-routers-top-1280.png.
Each router block is about 180 px for four fields: Prefix, Title and Address on one line and Matrix alone on the next. The Router levels block adds about 150 px more for each router, in a different section. A small grey caps label is the only thing between two routers in Router levels, so the fields of one router run into the next. 12 routers are 6.4 screens at 1920x1080 and 8.6 at 1280x800 (see the table above). The column stays 640 px wide.
Direction: a compact list of routers (one line each, with state) and a detail view for one router. The detail view can hold all of that router's settings.

**R-14. Settings > Routers shows no link state. Severity: med.**
Screenshots: 09-down-a-c-g-routers-list-shows-no-state-1920.png, 09-down-router-a-names-stale-count-1920.png, 09-down-main-page-for-comparison-1920.png.
With routers A, C and G down, the Routers list looks the same as when all are up. The main page shows "9/12 Routers" and one line for each down router. Router A's Router names still says "200 sources, 120 destinations", and Re-read now is greyed out with no reason. The Protects list still shows rows from routers that are down.
Direction: show each router's link state (with text, per patchbay) in the list, and a reason next to an action that is not available.

**R-15. The tie line list does not scale. Severity: med.**
Screenshots: 08-twelve-routers-tie-lines-1920.png, 07-six-routers-list-end-and-tie-lines-1920.png.
With 12 routers there are 24 rows in file order. They are not grouped by router pair, and there is no filter or sort. Each row shows "A.101 TIE AB 1 feeds B.151 TIE AB 1", with prefixes and no router titles. Edit and Remove come right after the text, so they are at a different x on each row. The add form is below every tie line and every suggestion, about 1100 px below the section heading with 12 routers.
Direction: a table with columns (name, from, to, state), grouped or filtered by router, with the add action at the top.

**R-16. Edit loads the tie line into the shared form far from its row. Severity: med.**
Screenshot: 04-tie-lines-edit-in-shared-form-1920.png.
Edit fills the form at the bottom of the section, below the suggestions. The row that is being edited has no mark. The button changes from "Add tie line" to "Save tie line", and Cancel appears.
Direction: edit in place, or in a dialog that names the tie line.

**R-17. To add a tie line by hand, the person must know both numbers. Severity: med.**
Screenshots: 04-tie-lines-manual-add-form-1920.png, 07-six-routers-tie-form-1280.png.
The form is From router, a number box, To router and a number box. The name of an end shows only after a valid number is typed. There is no search by name, as the patch boxes have. With 12 routers, From and To are two selects of 12 entries each.
Direction: let a person find each end by name or number, as in the patch boxes.

### Saves and destructive actions

**R-18. Remove on a tie line deletes it at once, with no confirm. Severity: high.**
Screenshot: 14-tie-line-removed-one-click-back-as-suggestion-1920.png.
One click on Remove saves the list without the tie line. There is no confirm, no undo and no review, and it is outside the one Save. Presets that store the tie line become Broken (tielines-design, "Presets" rule 5). The status "Removed TIE LA 2." shows under the add form, off screen with 12 routers. A tie line with matching end names comes back at once as a suggestion. Remove router does the opposite: it lists what will break and is staged for Save with a PIN.
Direction: confirm before removing, say what uses the tie line, and show the result next to the row.

**R-19. Two save models in one section. Severity: med.**
Screenshots: 04-tie-lines-suggestions-1920.png, 04-tie-lines-edit-in-shared-form-1920.png.
Tie lines (Add, Save tie line, Remove, Accept) save at once with the `tieLines` message, with no review and no PIN. Hop weight, in the same section, waits for the one Save, with review and PIN. The routers and their levels also use the one Save. "Add tie line", "Save tie line", each "Accept" and the footer "Save" are all `sode-btn--primary`, so a screen can show four blue main actions.
Direction: one save model for the topic, and one main action on screen.

**R-20. The duplicate error names the wrong problem. Severity: low.**
Screenshot: 04-tie-lines-duplicate-error-1920.png.
Adding the same destination and source a second time says "Not saved: two tie lines are named "CG3 Fill"." The real problem is that this cable is in the list already. Not tested: whether the same two ends with a different name are refused.
Direction: check the ends first and say "A.5 already feeds B.7 as CG3 Fill".

### Wording, navigation and empty states

**R-21. The section list on the left repeats "Routers" and names router A's block after its title. Severity: med.**
Screenshots: 01-one-router-top-1920.png, 08-twelve-routers-top-1920.png.
The list shows Routers > Routers, Tie lines, Router levels, then "Router" with Router & matrix, Router names, Takes and Protects under it. "Router" is router A's title, or the word "Router" when it has none. Routers B to L have no entries.
Direction: entries that match the new structure (for example Routers, Tie lines, then one entry for each router, or none).

**R-22. Router A's block holds things that are about every router. Severity: med.**
Screenshots: 08-twelve-routers-protects-all-routers-in-router-a-block-1920.png, 03-two-routers-top-router-a-row-vs-router-b-block-1920.png.
"Protects" in router A's block lists the Protects and Router locks of all 12 routers (A.13 to L.13), and its "Free Sode's Protects" acts on more than router A. The Broken and Name moved count in router A's row covers items of every router. The Status label is in router A's block but does nothing with two or more routers: `connview.js` reads it only when there is one router.
Direction: put server-wide lists and actions in a server-wide place. Hide or drop Status label when it has no effect.

**R-23. Settings search cannot tell routers apart. Severity: med.**
Screenshot: 13-settings-search-address-identical-hits-1920.png.
"address", "matrix", "prefix" and "title" each give 8 lines, all the same, such as "Address in Routers › Routers". There is no router prefix or title, and 8 is the limit, so the other routers are left out. Router A's field is called Name, so "title" does not find it. A search for a router title ("Stage 2") finds nothing.
Direction: name the router in each hit, and let a search for a router title or prefix find that router.

**R-24. Empty sections with one router. Severity: low.**
Screenshots: 01-one-router-top-1920.png, 01-one-router-top-1280.png.
With one router, Tie lines shows Hop weight and "Tie lines need two or more routers.", and Router levels shows "Sode has one router. Its levels are in its own block below." Those are two sections with nothing to do. Router A's Prefix field is hidden, and it appears only when a second router exists, in an odd place (R-01).
Direction: with one router, show the router and an Add router action, and hide or fold sections that need two routers.

**R-25. Add router: levels are not in the review, and defaults differ. Severity: low.**
Screenshots: 02-add-router-new-row-1920.png, 02-add-router-review-1920.png.
The new row has Levels with the placeholder "17" and no value. The review says "Stage 2 at 127.0.0.1:9311, matrix 0" and leaves out levels. The new router gets Sode's built-in defaults (`config.Defaults().Router`), not router A's settings. Router A's settings may not be the right model either, but the person cannot see or change most of them afterwards (R-03). Add router stays under the new row, so several new rows can be open at once.
Direction: show every value that will be saved, and let the person change it.

**R-26. The PIN recheck words. Severity: low.**
Screenshot: 02-add-router-pin-recheck-1920.png.
"Saving these changes: Critical. Enter your PIN." reads like a label, not a sentence. Continue is not styled as the main action.
Direction: a plain sentence that says why the PIN is needed, and the usual main action style.

**R-27. The setup wizard makes router A with no title and no prefix choice. Severity: low.**
Screenshot: 00-wizard-router-step-no-title-or-prefix-1920.png.
The Router step has Address, Levels, Level numbering, numbers and Multi destination. It has no title, so router A is "Router" everywhere afterwards (R-09), and there is no way to add a second router. The step label "Style & User" wraps under the dialog at 1920.
Direction: ask for a title. Maybe offer "Add another router" at the end.

**R-28. An open Settings page keeps old router rows after the router list changes. Severity: low.**
No screenshot kept. With unsaved edits in the Routers list, a hello with a different list (Sode restarted with another `config.yaml`) did not replace the rows: `load()` in `settingsrouterlist.js` skips when there are edits. The page kept "not an address" and its error on a router B that no longer existed. Router levels, a different save source, did show the new list. The two sections then disagreed.
Direction: when a router that has edits is gone, say so, and do not keep a row for it.

**R-29. Routers do not follow the pattern of the rest of Settings. Severity: low.**
Screenshot: 15-compare-access-users-manage-dialog-1920.png.
Users use "Manage users": a list and a detail editor. Routers use expanded inline forms. Tie lines use an inline list, a form and their own instant save. Three patterns for three lists of things.
Direction: pick one pattern for lists of items in Settings. The Users pattern fits routers and tie lines well.

## Top 10

1. R-02: router A's settings are in a separate block at the bottom of the topic, about 5900 px below the list with 12 routers.
2. R-03: routers after the first cannot set most of their settings. Protect on routers B and up is impossible (device number 0).
3. R-05: a change to router B's address asks "Save and reconnect to *router A's address*?".
4. R-18: tie line Remove deletes at once, with no confirm or undo, outside the one Save.
5. R-06: Swap… always acts on router A. Other routers cannot be swapped.
6. R-01: router A's row in the Routers list is a one-line summary with a floating Prefix field. The others are editable blocks.
7. R-07: no reorder, router A cannot be removed, and "first" is fixed by file order.
8. R-13: about 230 px for each router across two sections. 12 routers are 6.4 screens at 1080 and 8.6 at 800.
9. R-14: no link state in Settings > Routers. Down routers look like up ones, and router A shows a stale name count.
10. R-11: log lines name routers only by address. Health and status top-level `router` and `targetAddress` are router A's, and say "up" while Sode is Degraded.
