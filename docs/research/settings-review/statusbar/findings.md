# Status bar and router health with many routers

Review of `alpha-round1` at ae0d7e15. Review only: no app code was changed.

Owner's question: "How does the status bar work if there are a ton of routers?"

## Short answer

The status bar itself never wraps or overflows on the desktop. Up to 3 routers it shows one icon for each router ("Router A", "Router B", "Router C"). With 4 or more it collapses to a single icon, "Routers" with an up/total count such as `19/24`, coloured by the worst state (`ROUTER_ICONS_MAX = 3` in `web/routerlinkrules.js`). So the bar stays the same width for 4 routers and for 24. The cost is that the bar no longer says **which** router is in trouble. The user has to read the Degraded note under the bar, or open the hover details, which grow to 4,255 px tall at 24 routers. The surfaces that list every router (Lost link banner, details, Degraded note) do not scale. The phone header already overflows at 3 routers.

## Setup

- swp08sim, one per router, `SIM_PORT=47310..47333` (`SIM_LEVELS=4 SIM_SOURCES=40 SIM_DESTS=30`). Ports 8910 and 8911 were already taken by another session on this host, and a first run at 9310+ collided with another agent's simulators, so every shot here is from the 47310+ run.
- Down router: address `127.0.0.1:47399`, where nothing listens (connection refused). "Connecting" router: `10.255.255.1:9`, a black hole. After the first dial timeout it also shows as down (see SB-15), so the files named `connecting` show it red.
- Sode built from the branch. One scratch `config.yaml` (format 2) for each scenario, with Administrator Ana and all page locks off. Headless Chromium 153 driven over CDP, at 1920×1080, 1280×800 and 390×844 (mobile emulation, touch).
- Titles: long (up to 30 characters, such as "MCR Production Ultrix Main Rtr") or short ("Main", "Aud", "TX"). Each `statusLabel` value (`address`, `name`, `none`) was tested with 1 router and with `name` on 2+.
- File names: `nNN-<state>--<view>.png`, where NN is the router count.

## Findings

| ID | Sev | Finding |
|---|---|---|
| SB-01 | high | With 4+ routers the bar does not say which router is in trouble |
| SB-02 | high | The Lost link banner breaks with many routers |
| SB-03 | high | The phone header pushes Settings off the screen at 3 routers |
| SB-04 | high | The status details do not scale: 4,255 px tall at 24 routers, narrow value column |
| SB-05 | med | No click-through from the status to Settings > Routers or /log, and Settings shows no link state |
| SB-06 | med | The same router is named five different ways |
| SB-07 | med | The Status label setting stops working at 2 routers, with no note |
| SB-08 | med | Router 1 is special in Settings and stays the default box router while it is down |
| SB-09 | med | The Degraded note grows one full sentence for each down router |
| SB-10 | med | Compact view cannot show "only the icon of each down router" with 4+ routers |
| SB-11 | med | On the phone there is no way to see per-router state other than the Degraded note |
| SB-12 | low | `19/24` in red reads as a fault count, and down and connecting are merged |
| SB-13 | low | Healthy routers show amber in the uptime strips after a restart |
| SB-14 | low | The single-router and multi-router Lost link banners name the router differently |
| SB-15 | low | A router that never answered looks the same as one that dropped |

### SB-01: With 4+ routers the bar does not say which router is in trouble (high)

Screenshots: `n24-only-router1-down--1280x800.png`, `n24-only-router14-down--1280x800.png`, `n04-B-down-D-connecting--1920x1080.png`, `n08-C-F-down-longtitles--1920x1080.png`, `n16-3down-1connecting--1920x1080.png`, `n24-5down-longtitles--1920x1080.png`, `n04-all-up--1280x800.png`, `n16-all-up--1280x800.png`.

Problem: From 4 routers up, the bar shows one red icon and `23/24 Routers`. A down router A (the default router of the boxes) and a down router N give the same bar, pixel for pixel. The bar's job, to show at a glance which link is in trouble, now falls to the Degraded note under it, which moves the page down (SB-09). The jump from 3 separate icons to 1 icon between 3 and 4 routers is also abrupt. Adding a fourth router removes information the user had a moment before.

Possible direction: Keep the aggregate for the healthy state, but name the lost routers in the bar when one or more are lost (their prefixes, for example "B E L +2"). Or keep one icon for each lost router and collapse only the healthy ones.

### SB-02: The Lost link banner breaks with many routers (high)

Screenshots: `n24-all-down-lostlink--1920x1080.png`, `n24-all-down-lostlink--1280x800.png`, `n24-all-down-lostlink--phone-390x844.png`.

Problem: When every router is down, the banner draws one node for each router in a single flex row. At 24 routers each node is about 60 px wide. Titles and `127.0.0.1:47399` break mid-word, one or two characters per line on the phone. The banner is 465 px tall at 1920, 597 px at 1280 and 835 px (the whole screen) on the phone. On the desktop the Reason block is one long paragraph ("Router A: dial tcp … Router B: dial tcp …") with no line breaks. The phone keeps the breaks but lists 24 lines. All of this sits in a fixed red banner above the page.

Possible direction: With more than about 3 routers, group the router end of the map into one node ("24 routers") and list them below in a compact table or `details` element. Give each error its own line and collapse identical errors.

### SB-03: The phone header pushes Settings off the screen at 3 routers (high)

Screenshots: `n03-B-down-C-connecting--phone-390x844.png` (compare `n02-B-down-longtitles--phone-390x844.png`).

Problem: The phone header keeps a word beside each router icon ("Router A", "Router B", "Router C"). With 3 routers the row needs more than 390 px. The Grid popup button is cut at the edge (x 369 to 399), and the Settings button sits at x 407 to 437, outside the screen. The page does not scroll sideways (`overflow` hides it), so Settings cannot be reached from the header. With 2 routers it fits, and with 4+ the aggregate fits again.

Possible direction: On the phone, drop the words when there is more than one router, or start the aggregate at 2 routers.

### SB-04: The status details do not scale (high)

Screenshots: `n24-5down-longtitles--details-hover-1280x800.png`, `n24-5down-longtitles--details-hover-fullpage-1280.png` (1280×4296), `n16-3down-1connecting--details-hover-1280x800.png`, `n02-B-down-longtitles--details-hover-1280x800.png`, `n01-up-label-address--details-hover-1280x800.png`.

Problem: The hover details (statusinfo.js) are the only per-router view reachable from the bar. They show a block for each router: link row, address, error, "link up for…" and a 24-hour strip. Measured heights: 181 px for 1 router, 459 px for 2, 801 px for 4, 1,406 px for 8, 2,850 px for 16 and 4,255 px for 24, against an 800 px window. The box has no inner scroll. To read it, the user must scroll the page while the pointer stays on it, because the box closes 200 ms after the pointer leaves. Routers are in config order, so a down router can be 20 blocks down. The label column takes most of the 340 px width when titles are long. The value column is then about 60 px: "Connected" breaks as "Connecte/d", `127.0.0.1:47310` breaks after the third dot, and the error wraps one word per line.

Possible direction: Sort lost routers first, or show a compact one-line-per-router summary with the full block only for lost routers. Give the box a max height and its own scroll. Put the title on its own line so values keep their width.

### SB-05: No click-through from the status, and Settings shows no link state (med)

Screenshots: `n24-5down--settings-routers-list-1280x800.png`, `n24-5down--settings-routers-list-middle-1920x1080.png`, `n24-5down--log-page-1280x800.png`.

Problem: The status block (`#conn`) has no click action. The details hold text only, with no link to the router's Settings block or to /log. In the other direction, Settings > Routers does not show whether a router is up. Router B (down, `127.0.0.1:47399`) has the same row as router C (up). So a user who sees "B is down" and wants to check its address has to open Settings, scroll the Routers list and match the prefix by eye. The /log page has the best per-router overview (an uptime strip for each router), but nothing in the bar points to it.

Possible direction: Make each router line in the details (and each lost router in the bar, per SB-01) a link to that router's row in Settings > Routers. Show the link state (Connected or Down, with the error) on each router row in Settings. Add a "Communication Log" link in the details.

### SB-06: The same router is named five different ways (med)

Screenshots: `n24-5down-longtitles--1920x1080.png`, `n24-5down-longtitles--details-hover-1280x800.png`, `n24-5down--log-page-1280x800.png`, `n24-5down--settings-routers-list-1280x800.png`, `n24-5down--settings-router1-block-1920x1080.png`.

Problem: For the same router:
- Bar: "Router B" (1 to 3 routers), or only "Routers 19/24". The title never shows with 2+ routers.
- Details: "Router link B Studio 4 Gallery Audio Router", then "Router B link up for…".
- Degraded note: "Router B Studio 4 Gallery Audio Router is down."
- /log: "ROUTER B STUDIO 4 GALLERY AUDIO ROUTER" (upper case).
- Settings list: "ROUTER B" with no title in the row heading. Router 1's row shows only its title ("MCR Production Ultrix Main Rtr"), with no "Router A".
- Settings sidebar and the router 1 block heading: router 1's title only. The other routers are not in the sidebar.
- Field labels: router 1's title field is "Name", the other routers' is "Title". GLOSSARY.md calls it Router title.

A user who reads "Router B" in the bar finds no "Router B" in the Settings sidebar, and does not find router 1 under "Router A" at all.

Possible direction: Pick one display form ("B · Studio 4 Gallery Audio Router", say) and use it in every surface. Rename router 1's "Name" field to "Title".

### SB-07: The Status label setting stops working at 2 routers, with no note (med)

Screenshots: `n01-up-label-address--1280x800.png`, `n01-up-label-name-longtitle--1280x800.png`, `n01-up-label-none--1280x800.png`, `n02-up-label-name--1280x800.png`, `n24-5down--settings-router1-block-1920x1080.png`.

Problem: `statusLabel` (Settings: "Status label", options Address / Name / Nothing) works only while there is exactly one router (`list.length === 1 ? routerTitle() : …` in connview.js and mobile.js). With 2 routers and `statusLabel: name` on both, the bar shows only "Router A  Router B". Settings still offers the field in router 1's Router & matrix section, with no note that it does nothing. Routers 2+ have no such field, although the YAML key is per router. The option word "Nothing" is `none` in the file and in docs/reference.md.

Possible direction: Hide or disable the field when there are 2+ routers, with a one-line reason. Or move it to a server-wide setting that also applies to the multi-router bar (for example, show titles beside the prefixes when there is room).

### SB-08: Router 1 is special in Settings and stays the default box router while it is down (med)

Screenshots: `n24-5down--settings-routers-list-1280x800.png`, `n24-5down--settings-router1-block-1920x1080.png`, `n24-only-router1-down--1280x800.png`, `n02-A-router1-down-longtitles--1920x1080.png`.

Problem: In the status bar, router 1 is shown the same as the others when there are 2+ routers (`n02-A-router1-down…` shows a plain red "Router A"). Elsewhere it is different:
- Settings: router 1's row is a read-only summary (title, address, Swap…, the Broken count and a Prefix field). Routers 2+ get editable Prefix, Title, Address and Matrix rows with Remove…. Only router 1 has its own block (Router & matrix, Router names, Takes, Protects) and a sidebar entry.
- Main page: with router A down, the Source and Destination boxes still say "Router A" by default (`n24-only-router1-down`). The first thing a user types goes to the one router that cannot patch.
- Protects follow router 1's link only (`firstLinked()`), and the single-router status label comes from router 1 only.

Possible direction: Give every router the same row and block shape in Settings. When the default router of a box is lost, say so beside the box, or offer to switch.

### SB-09: The Degraded note grows one full sentence for each down router (med)

Screenshots: `n24-5down-longtitles--1920x1080.png`, `n24-5down-longtitles--phone-390x844.png`, `n16-3down-1connecting--phone-390x844.png`, `n24-5down--settings-routers-phone-390x844.png`.

Problem: Each lost router adds the whole sentence "Router X <title> is down. You cannot patch its destinations now." Measured heights: 5 lost routers make 98 px on the desktop and 195 px on the phone, and the note pushes the patch boxes down by that much. The note has no limit, so 20 lost routers would make about 400 px on the desktop. It also shows inside the phone Settings view, above the Settings title. A router that never connected shows "its destinations" with no names, because Sode never read them.

Possible direction: Past 2 or 3 lines, use one summary sentence ("5 routers are down: B, E, L, R, W. You cannot patch their destinations.") with an expand control. Keep the per-destination detail for a single router.

### SB-10: Compact view cannot show "only the icon of each down router" with 4+ routers (med)

Screenshots: `n08-compact-view-2down--1280x800.png`, `n08-compact-view-2down--1920x1080.png`, `n02-compact-view-B-down--1280x800.png`.

Problem: GLOSSARY.md says that in Compact view, while Sode is Degraded, "only the icon of each Router that is down shows". That holds with 2 routers (a red "Router B" alone). With 8 routers the only icon is the aggregate "6/8 Routers", so Compact view also cannot say which router is down. In both shots the Degraded note sits at the left edge, level with the title, apart from the centred Compact layout.

Possible direction: Follow SB-01 (lost routers named in the bar), so that Compact view can keep its rule. Centre the note, or place it under the title, in Compact view.

### SB-11: On the phone there is no way to see per-router state other than the Degraded note (med)

Screenshots: `n24-5down-longtitles--phone-390x844.png`, `n08-C-F-down-longtitles--phone-390x844.png`, `n04-B-down-D-connecting--phone-390x844.png`.

Problem: On the phone the details become a native `title` attribute (statusinfo.js `sync()`). A touch screen never shows a `title`. With 4+ routers the header shows only `19/24 Routers`. Settings on the phone has no Routers topic ("Other settings need a computer"). So a phone user can see which routers are down only in the Degraded note, and cannot see addresses or errors at all.

Possible direction: Make the phone status icon a button that opens a sheet with the per-router list (the same compact list as SB-04).

### SB-12: `19/24` in red reads as a fault count, and down and connecting are merged (low)

Screenshots: `n24-5down-longtitles--1920x1080.png`, `n16-3down-1connecting--1920x1080.png`.

Problem: The count is routers up out of total, drawn in red when any router is lost. A quick glance reads "19 problems". The word beside it is "Routers", not "connected". The spoken name says "19 of 24 routers connected", but the visible text does not. Connecting and down both count as "not up" with one colour (the worst), so 3 down plus 1 connecting is the same as 4 down.

Possible direction: Show the lost count when there is one ("5 down"), and the up count only when all are healthy, or say "19 of 24 up".

### SB-13: Healthy routers show amber in the uptime strips after a restart (low)

Screenshots: `n24-5down--log-page-1280x800.png`, `n24-5down-longtitles--details-hover-1280x800.png`.

Problem: Right after the server starts, each healthy router's last slot in the 24-hour strip is amber (the connecting moment falls in the current slot). On /log with 24 routers, that gives 19 amber blocks for routers that are fine. Docs/color-roles.md keeps a healthy state colourless. This is seen after every restart, the moment an operator is most likely to check.

Possible direction: Do not colour a slot amber for a first connect that succeeded, or weigh the slot by the time spent connecting.

### SB-14: The single-router and multi-router Lost link banners name the router differently (low)

Screenshots: `n01-down--1280x800.png`, `n24-all-down-lostlink--1920x1080.png`.

Problem: With one router the banner says "Lost link: Router link down" and the node shows "Router Studio Ultrix 127.0.0.1:47399", from router 1's title (`S.cfg.routerName`). With 2+ it says "Lost link: every Router link down" and a node for each router with its prefix. While the banner shows, the bar's status block is hidden (`#conn` is `display: none`), so the banner is the only status. This is consistent within each case, but the naming rule changes with the count (see SB-06).

Possible direction: Use the same router display form as SB-06 in both banners.

### SB-15: A router that never answered looks the same as one that dropped (low)

Screenshots: `n03-B-down-C-connecting--1280x800.png`, `n16-3down-1connecting--details-hover-1280x800.png`.

Problem: A router at an address that never answers is "connecting" (amber) only until the first dial times out. After that it has an error and shows as down (red), with the same word as a router that was up and dropped. A wrong address in a new setup and an outage in a working one look the same in the bar and in the Degraded note.

Possible direction: Separate "Never connected" (a setup problem, with the address shown) from "Down" (lost after being up) in the details and the note. The bar colour can stay the same.

## What works

- The desktop bar itself never wraps, truncates or overflows at any count. Its widest is 337 px (3 routers), and the aggregate is 183 to 199 px.
- One router: the label (address or name) sits after the icon. A 30-character title fits at 1280, and on the phone it is cut with an ellipsis.
- The Degraded note names each lost router by prefix and title, so the information is on the page, though not in the bar.
- /log gives a good per-router overview, one uptime strip for each router (but see SB-05 and SB-13).
