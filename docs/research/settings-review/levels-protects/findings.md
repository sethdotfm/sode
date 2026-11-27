# Settings review: levels and Protects

Track: the levels and Protect parts of Settings, per router and across routers. Review only, no app code changed.

## Setup

- Branch `alpha-round1` at ae0d7e15 (Sode Beta 1.8.3).
- Four swp08sim routers, so that a router with many levels sits next to routers with few:

| Router | Title | Port | Levels | levelOffset | levelNumbering | defaultLevels | protectDevice |
|---|---|---|---|---|---|---|---|
| A | Main Ultrix | 8920 | 32 | 0 | 1 | 1-32 | 200 |
| B | Studio B | 8921 | 17 | -1 | 1 | 1-4 | 201 |
| C | Edit suite | 8922 | 4 | 0 | 0 | 1 (shows as 0) | 0 (Protect off) |
| D | MCR | 8923 | 2 | 0 | va | blank (all) | 202 |

- Router-side holds from `SIM_PROTECTS`: 40 destinations on A (every 3rd, kinds 1 and 2, devices 9, 12, 55, 112, 300) and 24 on B (every 5th, devices 7, 9, 44), plus the simulator's default Router lock on dst 12 of every router.
- Sode-side Protects as Administrator "Ada": 80 on A through the Protect picker (one range pick), 7 on B and 4 on D one at a time from the route view.
- Peak: 158 rows in Settings > Routers > Main Ultrix > Protects (A 120, B 32, C 1, D 5).
- Headless Chromium at 1920x1080, and 1280x800 where layout matters. Screenshots are numbered in the order of the walk-through.

## Numbers at scale

- Protects list at 158 rows: 5,282 px tall (the Protects fieldset 5,493 px). That is about 5 screens at 1920x1080 and about 10 screens at 1280x800 (16 rows a screen, shot 19). The whole Settings page grows to 12,682 px.
- After Free Sode's Protects, 67 router-side rows stay (about 2,200 px), and none of them has an action.

## Clicks for common tasks

| Task | Steps today |
|---|---|
| Find one destination's Protect in Settings | Settings, Routers, tree "Main Ultrix > Protects", then scroll. No find box. A Protect on router D is at the end of the list, about 5,000 px down. |
| Free one Sode Protect from Settings | Free (the page jumps about 5,300 px to the Confirm at the end of the list), Confirm (letter key or button). No result shows. Scroll back about 5,300 px to go on. |
| Free every Sode Protect | Free Sode's Protects, Confirm, PIN recheck, Continue: 4 steps plus the PIN. Then the page moves to the Server topic. |
| Protect many destinations on router A | Palette (Ctrl+Shift+P), "protect", Enter, click, Shift+click, Protect N, Confirm: about 7, for any count. |
| Protect destinations on router B, C or D | Not possible in the picker. Per destination: type it, Enter, shield, Confirm (4 steps each). |
| See or change Sode's device number for router B, C or D | Not possible in Settings. Only by hand in config.yaml. |

## Findings

Severity: high = blocks or misleads a common task at scale; med = friction or inconsistency that users will hit; low = polish.

### LP-01 Protects list has no find, filter, grouping or count (high)
Screenshots: 08, 09, 10, 19.
Problem: Every hold on every router is one flat list, in destination order. At 158 rows it is about 5 screens tall at 1080p and 10 at 1280x800. There is no find box, no filter (Sode's own, mine, other devices, Router locks), no count, and no paging. To find one destination, the user scrolls and reads. This is the "Protects run right off the screen" report.
Direction: give the list a bounded height with its own big scroll bar (bigscroll.js), a find box and filter chips like the Protect picker has (All, Sode's, Router locks), and a count per router.

### LP-02 Free on a row opens its Confirm at the end of the list (high)
Screenshot: 11.
Problem: There is one Confirm box under the whole list. Free on A.2 (scrollY 4,887) moves the page to scrollY 10,234 to show it. The user loses the row they acted on. After the Confirm the page stays at the end.
Direction: confirm in place (in the row or in a dialog), and keep the scroll position.

### LP-03 Free from Settings gives no visible result (high)
Screenshot: 12.
Problem: A single Free sends its result to the result banner, and that banner is on Home, which is hidden while Settings shows. The banner "Protect freed, D.2 PVW, every level" was in the DOM with `offsetParent` null. The only sign is that the row goes, about 5,000 px above. A refusal ("Free Protect failed") is invisible too.
Direction: show the result in Settings, near the row or in the Protects status line that Free Sode's Protects already uses.

### LP-04 Routers after the first have no Protects settings (high)
Screenshots: 05, 06, 22.
Problem: The Routers list gives router B, C and D only Prefix, Title, Address and Matrix. Router levels gives their levels. No part of Settings shows or edits their `protectDevice`. Router C has 0 (Protect off), and nothing tells the user: Home shows no shield on C.5 and no reason (shot 22). Router B's 201 can only be found in config.yaml. The tree shows a full block (Router & matrix, Router names, Takes, Protects) for the first router only.
Direction: one block of the same sections for each router, or one per-router row in the Protects section with that router's device number and "Protect off" state.

### LP-05 The Protects list for all routers sits in the first router's block (high)
Screenshots: 08, 09, 01, 13.
Problem: The list and Free Sode's Protects are under "Main Ultrix > Protects", but they show and act on all four routers. "Sode's device number" just above it is router A's only, with no router name. The Palette and Settings search name the place "Routers > Protects", with no router. A user who wants router D's Protects has no reason to look under Main Ultrix.
Direction: move the cross-router list to a Routers-level section of its own (next to Router levels and Tie lines), and keep per-router fields in per-router blocks.

### LP-06 Router boundaries in the list are only a prefix change (med)
Screenshots: 09, 10.
Problem: Rows go from A.120 to B.2 with no heading, no gap and no count. The router title (Studio B, MCR) is not shown at all in the list. With 120 rows for A, the user cannot see where B starts without reading every prefix.
Direction: a heading per router (prefix, title, link state, count), optionally collapsible.

### LP-07 Rows are ragged and wordy (med)
Screenshots: 08, 19.
Problem: There are no columns. The levels text, the badge and the Free button start where the name ends, so they move from row to row. Rows with Free are taller than rows without it. "Levels 1-32" repeats on almost every row and says nothing when it is every level. The badge "Protected by another system (device 300)" is long and it is the same on dozens of rows.
Direction: a table-like grid (destination, levels, held by, action) with fixed columns. Show levels only when they are not every level. Shorter badge text, with the device number in its own column.

### LP-08 Router-side holds dominate and give no next step (med)
Screenshots: 10, 25.
Problem: Holds from other devices and Router locks are most of the list (67 of 158 here, and all 67 after Free Sode's Protects). They have no action and no hint of what to do (which panel or device to go to, or that Sode cannot free them). "Another system" does not say if it is a panel, DashBoard or another Sode.
Direction: group or filter them apart from Sode's own Protects; one line of help on who can free them; the device number as a label the Administrator can name.

### LP-09 Holds stay on show while the router link is down (med)
Screenshot: 26.
Problem: With every Router link down, the list still shows all 67 holds from before, with no mark that they are old. A user can read them as live.
Direction: mark a router's rows as last known (with a time) when its link is down, or fold them under the router heading with the link state.

### LP-10 Free Sode's Protects: no count, no router choice, result is easy to miss (med)
Screenshots: 23, 24, 25.
Problem: The Confirm says "Free every Protect with Sode's device number on each router that is connected?" It does not say how many (89 here) or which routers (C has Protect off). There is no way to free on one router only. After the PIN recheck the list shrinks by about 3,000 px and the page lands in the Server topic. The result is small hint text, "Sode's Protects freed: 89 freed." ("freed" two times), with no count per router.
Direction: show the count per router in the Confirm, offer per-router scope, keep the scroll position, and give a clear result line per router.

### LP-11 Empty state keeps an action with nothing to do (low)
Screenshot: 27.
Problem: "No Protects or Router locks." is clear. But Free Sode's Protects stays enabled, and the old "89 freed" status line stays under it.
Direction: disable or hide the button when Sode holds nothing, and clear the old status.

### LP-12 Protect picker covers the first router only (med)
Screenshot: 02.
Problem: The Palette's Protect row opens a picker for router A only. There is no router choice. Protects on B, C and D are one destination at a time from the route view (4 steps each).
Direction: a router selector (or router chips) in the picker, with the same range pick.

### LP-13 Level settings are split across sections and use different controls (med)
Screenshots: 06, 07, 20.
Problem: Router A's Levels, Level offset and Level numbering are in "Main Ultrix > Router & matrix", and its Default levels is in "Takes". For B, C and D all four are in "Router levels", whose note says "The levels of router A are in its own block below." Router A's Level numbering is a button group with a range preview ("0-31 / 1-32 / V + 1-31"). The others use a dropdown with other words ("Start at 1 / Start at 0 / V, then 1"). Router A has a live offset hint ("Level 1 = level 0 on the router"). B, with offset -1, has none.
Direction: one set of level fields per router, in the same place and with the same controls and hint for every router.

### LP-14 Save review and config file use internal level numbers (med)
Screenshot: 17. Server log for the config case.
Problem: Router D shows "V" in Default levels, but the review says "Router D: Default levels, was 1-2, now 1". Router C (numbering 0) shows "0" but stores "1". A hand edit of `defaultLevels: "0"` for router C fails on start: `router C: default levels: "0" is outside 1-4`. The user sees one numbering, and the review and the file use another.
Direction: show review values in the router's own numbering. Document (or accept) the router's numbering in config.yaml.

### LP-15 Level validation only on Save, one error at a time, mixed wording (med)
Screenshots: 14, 15, 16.
Problem: "5-40" on a 17-level router, "4" on a 0-3 router and "2" on a V/1 router show no inline error. Save shows one error at a time in the footer, and the page does not move to the field. Router A: `Default levels: "33" is not a level or range here.` (no router name). Router B: `Router B: Default levels is not a level or range of this router.` (no value). Neither gives the valid range.
Direction: validate on blur next to the field, list all errors on Save with links to the fields, and use one message form that names the router, the value and the valid range.

### LP-16 Settings search and Palette cannot tell routers apart (med)
Screenshots: 13, 01.
Problem: Search "level" gives "Level offset, in Routers > Router levels" three times and "Level numbering, in Routers > Router levels" three times, with no router name. Palette rows "Sode's device number" and "Protects and Router locks" say "Routers > Protects" with no router.
Direction: put the router prefix and title in each per-router result.

### LP-17 Level colors are by position, labelled 1-32, and shared by every router (med)
Screenshots: 18, 21, 22.
Problem: Level colors has 32 slots labelled 1-32. Each router takes its levels from these slots by position, so router C's level "0", router A's level "1" and router D's level "V" all get the same green. The labels do not match routers that number from 0 or V. Across routers, the same color means different labels, and a user cannot tell from a chip which router's level it is.
Direction: label slots as positions or show each router's labels under them; consider the router prefix where levels of two routers can show together.

### LP-18 Home: the router label does not follow the destination (med)
Screenshot: 22.
Problem: With destination C.5 (or D.11, B.3) chosen, the Levels box shows that router's levels (0-3, V/1, 1-17), but the label over the Destination box still says "Router A". Levels from different routers are told apart only by the count and labels of the chips.
Direction: the router label follows the chosen destination, and the Levels box names its router.
Note: Home is next to this track, so it is reported briefly.

### LP-19 32 levels do not fit on Home (med)
Screenshot: 21.
Problem: The Levels box shows levels 1-9 and 17-25 only; 10-16 and 26-32 are behind a small horizontal scroll bar. In the route view the level bars are cut at 15 and 31, and the Free Protect shield is pushed into a small inner scroller below the fold. With 32 levels the user may not find the Free Protect control.
Direction: wrap level chips to more rows (or a compact "1-32, all" form), and never put the Protect control inside a clipped scroller.

### LP-20 History cuts the level bar at 30 of 32 (low)
Screenshot: 04.
Problem: The Protect row of 80 destinations shows levels 1-14 and 17-30; 15, 16, 31 and 32 are cut off with no mark.
Direction: show "every level" in words, or wrap.

### LP-21 "V" wording is unexplained and has two forms (low)
Screenshots: 06, 07, 10.
Problem: "V + 1-31" in one control and "V, then 1" in the other mean the same thing. "Levels V-1" in the Protects list reads as "V minus 1". "V" is not explained in these sections.
Direction: one wording, a short tip that V is the video level, and "V to 1" or "V, 1" in lists.

### LP-22 Blank Default levels becomes an explicit list on the next save (low)
Screenshot: 06.
Problem: Router D had `defaultLevels: ""` (all). The field shows "V-1", and the next save wrote `1-2`. If the router later gets more levels, the default no longer means all.
Direction: keep blank as "all" (placeholder "All levels"), and save it blank.

### LP-23 Sode's device number field: size and help (low)
Screenshot: 08.
Problem: A 0-4095 number takes the full form width. "0 = off" and the panel warning are only in a tooltip. The field does not say which router it is for.
Direction: a short number field with the router name, and the help as visible hint text.

### LP-24 Router levels blocks run together (low)
Screenshots: 06, 20.
Problem: Router B's Default levels sits right above the small "ROUTER C: EDIT SUITE" label with no gap or rule, so the blocks are hard to tell apart, more so at 1280x800.
Direction: a heading and spacing per router, like the rules in the Routers list.

### LP-25 Sidebar tree highlight lags behind the content (low)
Screenshot: 19.
Problem: At 1280x800 with the Protects heading at the top of the content, the tree still marks "Takes".
Direction: check the scroll-spy threshold for short viewports.

## Screenshots

01 Palette search "protect" (rows say "Routers > Protects", no router).
02 Protect picker, router A only.
03 Confirm for 80 Protects from the picker.
04 History Protect row, level bar cut at 30.
05 Settings > Routers list: B, C, D get four fields; only Main Ultrix has a block in the tree.
06 Router levels for B, C, D (dropdowns, no hints).
07 Router A's level controls (button group, live hint), shown with V numbering and offset -1 (not saved).
08 Protects list, top: ragged rows.
09 Protects list, A to B boundary with no heading.
10 Protects list, end: Free Sode's Protects after about 5,000 px.
11 Free on A.2 moved the page about 5,300 px to the Confirm.
12 After Free: no result in Settings.
13 Settings search "level": same rows three times.
14 Invalid Default levels on B, C, D: no inline error.
15 Save error for router A.
16 Save error for router B, other wording, page not moved.
17 Save review with internal level numbers.
18 Level colors, 32 slots labelled 1-32.
19 Protects list at 1280x800: 16 rows a screen.
20 Router levels at 1280x800.
21 Home, router A: 32 levels, chips hidden, Free Protect clipped.
22 Home, router C: Protect off with no reason; "Router A" label with C.5.
23 Free Sode's Protects Confirm.
24 After Free Sode's Protects: page in the Server topic.
25 Result line and the 67 router-side rows left.
26 Protects list while every Router link is down.
27 Empty state.

## Not covered

- Owner labels for Protects of a second Sode user, and the Approval path for freeing them (only one Administrator was used).
- Phone layout.
- Routers with 64 to 256 levels.
