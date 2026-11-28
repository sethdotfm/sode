# Findings A2: main page, top bar, banners

Code read at `/Users/seth/Desktop/sode/.claude/worktrees/review-shots` (origin/alpha-round2). Numbers are CSS px at 1920x1080, DPR 1, read from bounding boxes in a booted copy (Sode on 42080, sims 42010 to 42012, seeded data from the 1080 capture). "Verified" means I injected the CSS change and measured again. Entries that rest on a screenshot only say so.

Not checked: keyboard mode key notes. `088-main-keyboard-mode-keys-*` shows no key notes, and my own Tab press did not set `data-keys-on` either, so the capture did not reach that state.

---

### A2-01: Routes under Source and under Destination start at different heights
- **Where:** main page with Source and Destination picked; `047-main-source-destination-picked-crop-boxes-4k2x.png`, `068-...-1080.png`, `080-...-1080.png`, `097-...-1080.png`, `099-...-1080.png`
- **Kind:** alignment
- **Severity:** high
- **Problem:** "Sending to" starts at y=358.0, "Receiving from" at y=364.0. 6 px apart, on every page state that shows both. Cause: `.sode__group--dst` also carries the class `.sode__group`, which is the Settings fieldset (`web/base.css:952`, `gap: var(--pb-space-5)` = 12 px). The group is a row subgrid of `.sode__boxes` (row-gap 0), so its own 12 px row gap moves its second row down by half the difference. This is the "boxes sitting at different heights" note.
- **Change:** `web/desktop.css:40` add `row-gap: 0;` to `.sode__group--dst`. Verified: both start at y=358.0. Better: rename the patch row's class (`sode__group` to `sode__boxgroup` in `web/index.html:108,129` and `web/desktop.css`) so the Settings fieldset rules (border, flex column, gap, padding) never reach the patch row.
- **Also check:** `.sode__group--levels` gets the same 12 px row gap (computed `row-gap: 12px`); it has one row now, so nothing shows, but a second row would. A2-10 is the same class of fault on the column axis.

### A2-02: History filter boxes are 32 px, the buttons beside them 40 px
- **Where:** History tools row, every state; `037-main-empty-1080.png`, `072-...-crop-search-4k2x.png`, `074-...-1080.png`
- **Kind:** alignment / bug
- **Severity:** high
- **Problem:** Filter by Source and Filter by Destination are 32 px high (y 523.5 to 555.5). The Clear button beside them is 40 px (to 563.5) and so is Filters. All three are top-aligned, so the two buttons hang 8 px under the boxes, and the centre lines are 539.5 and 543.5. `web/base.css:127` means to make the boxes 40 px (`--sode-search-h: var(--sode-h-action)`), but its selector has the weight of one class (the page part is in `:where()`), and `web/history.css:607` sets the same variable with the same weight and loads later. The base.css rule never applies.
- **Change:** `web/history.css:606-607`: delete `--sode-search-h: var(--pb-control-h-sm);` from `.sode__search-box` and write the fallback at the three uses: `height: var(--sode-search-h, var(--pb-control-h-sm));` (line 617), `top: var(--sode-search-h, var(--pb-control-h-sm));` (line 631), `top: calc(var(--sode-search-h, var(--pb-control-h-sm)) / 2);` (line 715). Verified with an equal override: boxes, Clear and Filters all y=633.0 to 673.0.
- **Also check:** the phone view's search boxes (the variable is shared).

### A2-03: Last and Next chips touch
- **Where:** box heads after a take; `064-main-last-next-chips-empty-boxes-crop-boxes-4k2x.png`, `-crop-src-`, `-crop-dst-`
- **Kind:** spacing
- **Severity:** high
- **Problem:** `[data-last]` has `gap: var(--pb-space-3)` (6 px), but the Last chip ends at x=672.1 and the Next chip starts at 673.1: 1 px, so the two 2 px edges read as one 5 px bar. The slot is sized by a hidden ghost (`web/desktop.js:281`, plain text "Next"), but the real chip has a bold `.sode__tag` and a 4 px flex gap before its zero-width `.sode__next-open`. The real chip is 48.3 px, the slot about 43 px, and the chip hangs out of the slot to the left (it is `right: 0`).
- **Change:** `web/desktop.js:281`: build the ghost like the chip: `el('span', 'pb-chip sode__last sode__next-size', el('span', 'sode__tag', 'Next'))`. `web/base.css:1793`: add `gap: 0;` to `.pb-chip.sode__next`, and add `padding-left: var(--pb-space-2);` to `.sode__next-in` (line 1834) so the open chip keeps its space after the tag.
- **Also check:** Destination head (same code). Levels has no Next.

### A2-04: The "Lock menu" tip covers the first menu item
- **Where:** top bar, hover on the lock button; `041-topbar-lock-menu-hover-1080.png`, `-crop-menu-4k2x.png`
- **Kind:** bug
- **Severity:** high
- **Problem:** On hover the tip "Lock menu" shows first, then the drop-down opens under it. The tip stays and hides the word "Lock" and its icon (tip box at about x 369 to 438, y 48 to 72; item 1 is y 52 to 84). `web/lock.css:360` hides `[data-pb-tip]::after` while `.is-menu` is set, but tips are no longer drawn as `::after`: `web/tooltip.js` draws one fixed element and reads the `::after` rules only when the tip opens. A tip that is already open is not checked again.
- **Change:** in `web/lockchip.js`, where the menu opens (the block at line 102 that adds `is-menu`), close the open tip: export a `hideTip()` from `web/tooltip.js` and call it there. Or remove the tip from the button while the hover menu is possible: the menu itself names the button's function.
- **Also check:** any other control that opens a popup on hover or click while its tip shows: the Routers status (its hover box replaces the tip, fine), Next chips, the `+` button.

### A2-05: Redo and Undo do not line up down the History list
- **Where:** History rows; `069-history-with-entries-*`, `071-...-1080.png`, `084-...-1080.png`
- **Kind:** alignment
- **Severity:** high
- **Problem:** The actions are packed to the right, and the Undo label has three lengths. Redo starts at x=1389 beside "Undo" (63 px), 1378 beside "Undo 3" (74 px) and 1372 beside "Undo all" (81 px). The column of Redo buttons steps 17 px left and right down the list.
- **Change:** `web/history.css:46` (`.sode__hist-actions`): give the buttons fixed tracks: `display: grid; grid-auto-flow: column; grid-auto-columns: 5.25rem; justify-content: end;` and `.sode__hist-actions > .pb-btn { justify-content: center; width: 100%; }`. 5.25rem holds "Undo all". Then Redo is always x=1353 to 1437 and Undo 1443 to 1521 at the present gap.
- **Also check:** the opened rows of a stacked entry (`091-...-1080.png`): their Undo ends at x=1513, 8 px left of the parent row's 1521, because the child rows are inset. Take the inset out of the right side so one Undo column runs down the list.

### A2-06: At 3840 px the page is small and the row has stranded parts
- **Where:** all pages at 4K with no OS scale; `037-main-empty-4k.png`, `069-history-with-entries-4k.png`
- **Kind:** readability
- **Severity:** high
- **Problem:** `web/base.css:13-20` stops the root at 125% at 3840 px. Body text is then 16.25 px, labels and set chips 13.75 px, level squares 27.5 px, on a 3840 px canvas. The page takes 2460 px (64%). The Levels text box grows to about 508 px for the text "1-17 (All)", while the picks under it stay 9 columns (about 268 px) and Select None ends far from any edge.
- **Change:** owner's decision on the size; a concrete option: `font-size: min(150%, 100% + (100vw - 2400px) / 180);` (24 px root at 3840, body 19.5 px) and `--sode-main-w: max(75rem, 100vw - var(--sode-page-bar) - 69rem)` kept, which then gives a 2184 px page of 91rem. For the Levels group, cap the text box: `web/desktop.css:60` `grid-template-columns: minmax(6rem, 14rem) auto; justify-content: start;` so the box does not stretch past its content.
- **Also check:** `AGENTS.md` says the UI scales with html font-size 125%; `web/base.css:5-7` says 100%. One of them is out of date.

### A2-07: The name list in the Approval popup is cut by the dialog
- **Where:** Approval popup; `089-approval-popup-clear-history-crop-dialog-4k2x.png` (screenshot only)
- **Kind:** bug
- **Severity:** high
- **Problem:** The match list under Name opens inside the dialog body, which scrolls. The third name (Cleo) is cut by the footer and the body shows a big scroll bar for a list of three names.
- **Change:** in the popup built by `web/approval.js` (its prompt), give the body room for the list instead of a scroll: `min-height` of the Name box plus 4 rows (`calc(var(--sode-h-field) + 4 * var(--pb-control-h) + var(--pb-space-8))`), or open the list in the dropdown layer as the patch boxes do (`.sode__menu-pop`, `z-index: var(--pb-z-dropdown)`) with `overflow: visible` on the body while it is open.
- **Also check:** the lock screen's Name list (other reviewer) and the Step in popup, which use the same Name box.

### A2-08: Lost link banner says "Router A Router A" and runs the reasons together
- **Where:** Lost link banner; `098-link-state-all-routers-down-lost-link-banner-crop-banner-4k2x.png`, `099-...-1080.png`, `100-link-state-server-down-1080.png`
- **Kind:** text / bug
- **Severity:** high
- **Problem:** (1) Each router node reads "Router A Router A 127.0.0.1:18910": `web/core.js:613` writes `Router ${r.prefix} ` and then the title, and the default title is "Router A". With one router (`core.js:614`) it reads "Router Router A". (2) The reasons of three routers are joined with `\n` (`core.js:550`) but shown in an inline `<code>`, so they run on as one line with no break. (3) Title "Lost link: every Router link down" has a capital R in the middle of a sentence; the other title is "Server link down".
- **Change:** `core.js:613`: `node(routerIcon(), links[1], r.title && r.title !== \`Router ${r.prefix}\` ? \`${r.prefix}: ${r.title} \` : \`Router ${r.prefix} \`, el('code', null, r.addr || '?'))`; line 614 the same with `FIRST`. `web/base.css` `.sode__down-why > code` (near line 326): add `white-space: pre-line;` so each router's reason has its own line. `core.js:550` title: `Lost link: every router link is down`; `core.js:546`: `Lost link: the router link is down`; `core.js:542`: `Lost link: the server link is down`.
- **Also check:** the phone banner uses the same node code.

### A2-09: Router list has no row lines, and its rows are 4 px off the lines above
- **Where:** click Routers; `040-topbar-router-list-open-crop-list-4k2x.png`, `096-link-state-degraded-router-list-crop-list-4k2x.png`
- **Kind:** bug / alignment / design-language
- **Severity:** medium
- **Problem:** (1) `web/bar.css:274` and `:282` use `var(--pb-border)`. No such token exists in patchbay or Sode, so both border declarations are invalid and the list has no top line and no lines between routers. (2) "Server link" starts at x=1124.8, "Router A" at 1128.8; "Connected" ends at 1512.8, the box content at 1516.8: the row button has `padding: var(--pb-space-2)` on all sides. (3) The hover box and the list show the same facts in two layouts: the hover box puts the state in the value column at x=1226.6, the list puts it at the right edge.
- **Change:** `bar.css:274,282`: `var(--pb-edge-soft)`. `bar.css:294` (`.sode-routerlist__head`): `padding: var(--pb-space-2) 0;` and move the hover fill to a negative-margin pseudo-element or accept a flush fill. For (3), put the state in the same value column as the lines above: make `.sode-routerlist__head` use the two columns of `.sode-statusinfo__rows` (`display: grid; grid-template-columns: subgrid` is not available across the `<ul>`, so set `.sode-routerlist__name { flex: 0 0 var(--sode-status-label-w) }` from the measured label column, or simply left-align the state after a fixed 7rem name column).
- **Also check:** run the token scan for the whole web folder: `--pb-border` was the only undefined `--pb-*` name in desktop, bar, base, history, tiepicker, approvalcards, palette and lock CSS.

### A2-10: Gap between the Levels box and Take is 9 px; the code says 14
- **Where:** patch row; `037-main-empty-crop-boxes-4k2x.png`
- **Kind:** spacing
- **Severity:** medium
- **Problem:** Levels ends at x=1332.6, Take starts at 1341.6: 9 px. Destination to `+` is 4 px and Take to Clear is 4 px, Source to the arrow is 14 px. `.sode__group--levels` sets `column-gap: var(--pb-space-6)` (14 px), but the Levels box is a column subgrid and takes `gap: var(--pb-space-2)` (4 px) from `.sode__box` (`web/desktop.css:93`). A subgrid with a smaller gap grows each track by half the difference, so the text box runs 5 px into the gap.
- **Change:** `web/desktop.css:62` (`.sode__group--levels > .sode__box--levels`): add `column-gap: var(--pb-space-6);`. Verified: the text box ends at 1327.6 and the gap is 14.0 px. If the three joined controls should read as one group instead, use `var(--pb-space-2)` in both places to match `+` and Clear.
- **Also check:** the picker row under it uses the same subgrid.

### A2-11: Source and Destination keep 66 px free at the right for a key note that does not show
- **Where:** patch boxes outside keyboard mode; all main page shots
- **Kind:** edge case
- **Severity:** medium
- **Problem:** `web/history.css:692` gives every box that contains a key note `padding-right: 3em`. At the box's 22 px text that is 66 px (computed on `#src` with `data-keys-on` off). The note is hidden then (`history.css:665`), so a long name is cut 66 px before the box edge. `web/desktop.css:77` already takes the pad away, but for the Levels box only. The History filter boxes lose 39 px the same way.
- **Change:** replace `desktop.css:77-79` with one rule for all: `html:not([data-keys-on]) :is(.sode__keyed, .sode__search-box) > .pb-input { padding-right: var(--pb-space-3); }`. Keep 3em only under `html[data-keys-on]`.
- **Also check:** the Destination box's armed hint (`.sode__armed`, `right: 3em`, `desktop.css:182`) uses the same number.

### A2-12: One row holds controls of three or four heights
- **Where:** Bookmarks strip and History tools; `037-main-empty-1080.png`, `061-...-crop-strip-4k2x.png`, `075-...-crop-sets-4k2x.png`
- **Kind:** alignment / design-language
- **Severity:** medium
- **Problem:** Bookmarks row: heading text 25.5 px, Edit and All presets 40 px (y 390 to 430), bookmark chips 32 px (y 393 to 425). History tools: filter boxes 32, Clear 40, set chips 22.5 (the "All" chip 24.5), Filters 40. `web/base.css:23-25` says nothing used live is lower than the action height (40 px), yet the chips a person presses live are 32 and 22.5. The "All" chip is 2 px taller than the other set chips because `.sode__sets` stretches it to the run's height, and the run has `padding-bottom: var(--pb-space-1)` for its scroll bar (`web/history.css:474-481`).
- **Change:** (1) `history.css:467` (`.sode__sets`): add `align-items: center;` (verified: All and the others get one height). (2) Pick one height for the live chips and use it for both strips: `#patch-view :is(.sode__presets, .sode__sets-bar) .sode__set { min-height: var(--sode-h-action); }` in `history.css` after line 509, and drop the 2 px difference between `.sode__chipbtn` and the chips. If 40 px set chips are too heavy, state the exception in the base.css comment and make Edit, All presets, Clear and Filters `var(--sode-h-compact)` in these two rows instead, so each row has one height. (3) Text size: bookmark chips are 13 px, set chips and the buttons 11 px; use `var(--pb-text-sm)` for all four.
- **Also check:** the `|` separators in both strips are text characters with a transparent 2 px border for size (`history.css:567`); a `border-left: var(--pb-border-w) solid var(--pb-edge-soft)` rule of fixed height reads better and scales.

### A2-13: Last chips are compact while everything they sit over is large
- **Where:** box heads; `064-...-crop-boxes-4k2x.png`, `068-...-crop-levels-4k2x.png`
- **Kind:** readability
- **Severity:** medium
- **Problem:** The chips are 32 px high with 13 px text (`web/base.css:1766-1772`, `--sode-h-compact`, `--pb-text-sm`) over a 64 px box with 22 px text. They are pressed live, so by base.css's own rule they are actions (40 px). This is the owner's "Last chip too small" note; the chip grew from the earlier size but is still the smallest pressable part of the row.
- **Change:** `base.css:1766`: `font-size: var(--pb-text-md); height: var(--sode-h-action);` and `base.css:1728` (`.sode__box-head`): `min-height: var(--sode-h-action);`. The four blank heads in the row use the same class, so the row keeps one head height.
- **Also check:** the Levels Last chip has `min-width: 5rem` (`desktop.css:295`); at the larger text "Last 1–4" still fits (80 px now).

### A2-14: Approval cards sit on the page scroll bar
- **Where:** bottom right cards; `091-approval-card-on-approver-main-page-1080.png`, `090`, `093` (screenshot, plus computed variables)
- **Kind:** bug
- **Severity:** medium
- **Problem:** `.pb-toast-stack` is fixed at `right: var(--pb-space-5)` (12 px, patchbay.css:2205). The page's big scroll bar is 22.4 px wide (`--sode-page-bar: 1.4rem` while it has room). The card's right edge and shadow lie about 10 px over the bar.
- **Change:** `web/base.css` (next to the `.pb-toast` rule at line 683): `.pb-toast-stack { right: calc(var(--pb-space-5) + var(--sode-page-bar)); }`. `base.css:180` already does this for another fixed part.
- **Also check:** result toasts on other pages use the same stack.

### A2-15: The Last chip's tip opens over the text in the box
- **Where:** hover or focus on a Last chip; `065-main-last-chip-hover-clip-tip-4k2x.png`, `068-...-1080.png`, `070-...-1080.png`, `071-...-1080.png`
- **Kind:** readability
- **Severity:** medium
- **Problem:** The tip "Last source (⌃s)" opens under the chip, which is on the box. It covers the right part of the typed name (y 230 to 256 in a box that starts at y=230). It also stays while the chip keeps focus after a press, as three of the shots show.
- **Change:** `web/desktop.js:267` and `:277`: add the class `pb-tip--top` to both chips (the tip then opens over the head, where the page is empty), and blur the chip after `useLast` moves focus to the box.
- **Also check:** "Clear the search" tip over the first History row (`076-...-1080.png`) and "Dismiss" under the result banner (`080`): both open down over content; `pb-tip--top` fits both.

### A2-16: Top bar: Inbox is a size smaller than its neighbours, and the right end is not flush
- **Where:** top bar; `037-main-empty-crop-bar-4k2x.png`, `098-...-crop-bar-4k2x.png`, `100-...-crop-bar-4k2x.png`
- **Kind:** alignment / design-language
- **Severity:** medium
- **Problem:** (1) Inbox is `pb-btn--sm`: 11 px text and a 14 px icon. "Signed in as", Server and Routers are 13 px with 18 px icons. One bar, two text sizes. (2) The left ghost button's box starts on the page edge (x=368.8). At the right, the word "Routers" ends at 1520.8 and its button at 1522.8, 6 px short of the bar's line end (1528.8): `#conn-addr` is an empty flex item that still takes one gap. (3) In the Lost link states the status is hidden, and the rule after Inbox is left alone at the right end with nothing after it.
- **Change:** (1) `web/inbox.js` (the button with class `sode-inbox__btn`): drop `pb-btn--sm`, use `pb-icon` not `pb-icon--sm`. (2) `web/bar.css`: `.sode__conn-addr:empty { display: none; }`. (3) `bar.css`: `.sode-bar__rule--end:has(+ .sode__conn[hidden]), .sode-home-bar:not(:has(.sode__conn > :not([hidden]))) > .sode-bar__rule--end { display: none; }` (match it to how connview hides the status in the down state).
- **Also check:** the lock button's icon is `pb-icon--sm` (14 px) beside 18 px tool icons; the four tool buttons are 32 px with 4 px gaps while the status items have a 6 px gap.

### A2-17: Dialog buttons are drawn four ways
- **Where:** `043-topbar-inbox-open`, `059-presets-popup-all`, `063-bookmarks-editor`, `078-history-filters-dialog`, `082-protect-picker-two-picked`, `054-tie-picker-a-to-b` (1080 shots)
- **Kind:** design-language
- **Severity:** medium
- **Problem:** Save is teal (primary) in Edit Bookmarks and brown (commit) in Filters. Close is teal in All Presets and grey in Inbox. Cancel is a grey fill everywhere except the Protect picker, where it is a ghost with no fill. The confirm's Take is 48 px beside a 32 px Cancel; the Tie line picker's Take is 40 px beside a 32 px Cancel.
- **Change:** one rule set: commit colour only for a button that changes the router (Take, Protect); `pb-btn--primary` for Save; plain for Close and Cancel. So: Filters Save to primary (`web/filters.js`), All Presets Close to plain (`web/presetpopup.js`), Protect picker Cancel to the plain fill (`web/protectpick.js`). Give the Tie line picker's Cancel `height: var(--sode-h-action)` to match its Take (`web/tiepicker.css:177`).
- **Also check:** the Approval popup's disabled Approve has a double edge; other disabled buttons have a single one.

### A2-18: Tie line picker: five edges inside 12 px, and a mostly empty 80vh dialog
- **Where:** `054-tie-picker-a-to-b-1080.png`, `056-tie-picker-a-to-c-hop-1-crop-dialog-4k2x.png`
- **Kind:** readability
- **Severity:** medium
- **Problem:** A focused step with one Best choice draws, from the outside in: the teal focus outline, the step's 2 px edge, the Best double outline (6 px, `web/tiepicker.css:110-113`), and the choice's own edge. The Protect button's lower edge touches the top of that outline. The light green bar at the left of a choice (`.sode-tiepick__wire`) has no label. The dialog is always `min(80vh, 60rem)` high (`tiepicker.css:5`); with one tie line 70% of it is blank and Cancel and Take are 700 px from the choice.
- **Change:** mark Best with the badge only (it already says "Best"): delete `tiepicker.css:110-113`. Add `margin-bottom: var(--pb-space-3)` under `.sode-tiepick__stephead`. Size the dialog to its content: `height: auto; max-height: min(80vh, 60rem);`. Give the wire a `data-pb-tip` ("Free", "In use") or remove it, since the row already has that word.
- **Also check:** the same picker inside Panels (other slice).

### A2-19: Protected destination: loose level strips and an icon with no text
- **Where:** `080-main-take-to-protected-refused-1080.png`, `084-main-destination-protected-by-sode-1080.png`, `-crop-dst-4k2x.png` (screenshots)
- **Kind:** alignment / design-language
- **Severity:** medium
- **Problem:** Under Destination, a Router lock shows a second level strip with nothing beside it and the badge on its own line below. A Sode Protect shows the strip with the badge beside it, then one icon-only button alone on a third line at x=809, off every edge. Two layouts for one fact, and an icon-only control with no text, which patchbay does not allow outside Panels.
- **Change:** in `web/protects.js` (the part that paints `#dst-under`): always put the badge beside its level strip (the 084 layout), and put the release action in the same line as a text button: `Free` with the icon, class `pb-btn pb-btn--ghost pb-btn--sm`.
- **Also check:** the Protect picker's title bar has a lone shield icon at the right (`081`), also with no text.

### A2-20: Status and result texts
- **Where:** History rows, result banner, empty states, popups (file names in each line)
- **Kind:** text
- **Severity:** medium
- **Problem and change, one line each:**
  - `web/history.js:239,256`: status `ok` beside `External`. New: `OK` (and `Failed` for `failed`).
  - `web/historyrows.js:80` and the child rows of a stacked entry (`091-...-1080.png`): the button says `Before` on plain rows, `Source` / `Sources` on child rows of a Protect. New: `Before` on all.
  - `web/tieflow.js:198`: `A.13 CAM 1 to B.14 MV 1 Via TIE 1`; the plain take says `A.13 CAM 1 → A.1 PGM (levels 1–4)`. New: `${r.label} via ${names.join(', ')}` and the arrow form in the label, so both read `A.13 CAM 1 → B.14 MV 1 via TIE 1`.
  - `web/protects.js:361-363`: title `2 Protected`, text `Every level.` New title: `2 destinations protected`; text: `On every level.`
  - `web/inboxdialog.js:126`: `Inbox empty`; History says `History is empty`. New: `The Inbox is empty` and `History is empty` stay parallel, or `Inbox is empty`.
  - `web/presetpopup.js:34` and `web/bookmarks.js:113`: title and tip `All Presets`, button `All presets`. New: `All presets` in all three.
  - `web/protectpick.js:81`: `Enter goes on.` New: `Enter continues.`
  - `web/searchbox.js:150` with `:21`: `0 found, filters ignored` beside `No matches.` and a Clear search button, three parts for one fact. New: show only `No matches (filter sets not applied).` in the list and leave the note empty at 0.
  - `web/approvalcards.js:186`: `Ben asks from ::1: Clearing History. 1:57 left`. New: `Ben asks to clear History (from ::1). 1:57 left`; needs `m.what` as a verb phrase, or `Ben, from ::1: Clearing History. 1:57 left`.
  - `web/approval.js:53`: `Clearing History: Approval from an Administrator.` New: `Clearing History needs Approval from an Administrator.`
  - `web/plans.js:301`: `Sode is not connected to router A.` New: `Sode is not connected to Router A.` (the page writes `Router A` on its buttons).
  - Tie line picker (`054`): `Source, router A` / `Step 1: router A to router B`. New: `Source on Router A`, `Step 1: Router A to Router B`.
  - Match list foot (`044`): `↑ ↓ or ⌘j/k`. New: `↑ ↓ or ⌘J ⌘K`.
  - Sign out note, `web/lockchip.js:62`: `Every tab that shares this sign-in`. New: `Signs out every tab of this browser`.
- **Also check:** the degraded note (`094-...-crop-note-4k2x.png`): `Router B is down. You cannot patch its 120 destinations now: B.1 PGM to B.120 MV 107.` Check "patch" against GLOSSARY.md; a shorter form: `Router B is down. Its 120 destinations (B.1 to B.120) cannot take a route.`

### A2-21: The Router button under a box can disagree with the box
- **Where:** `097-link-state-degraded-box-on-lost-router-1080.png`, `049-main-router-button-under-destination-hover-crop-dst-4k2x.png`
- **Kind:** readability
- **Severity:** medium
- **Problem:** With `B.14 MV 1` in the Destination box, the button under it says `Router A`. The button is the default router for a number typed with no prefix (`web/boxrouter.js:26-32`), but it reads as a label of what is in the box. Its meaning is only in the tip.
- **Change:** `web/boxrouter.js:29`: `b.textContent = \`No prefix: ${rt.prefix}\`` (or `Default: Router A`), and the tip: `A number with no prefix is on Router ${rt.prefix}. Press to change the router.` ("Select to change" is not what a press does).
- **Also check:** the button is 11 px muted text in a ghost button with no edge, and Show all at the right is the same; both look like captions, not buttons, until hover. A `pb-btn--outline` edge would say they can be pressed.

### A2-22: Disabled buttons are drawn three ways
- **Where:** `037-main-empty-1080.png` (Clear, the search Clear), `099-link-state-take-while-down-1080.png` (Take, Redo, Undo)
- **Kind:** design-language
- **Severity:** low
- **Problem:** Disabled Clear is a pale outlined box. Disabled Take is bare grey text with no box at all, so the row seems to lose a part. Disabled Redo and Undo are white outlined boxes, brighter than their enabled grey fill, so on the Lost link page the disabled column draws the eye more than the enabled one did.
- **Change:** one disabled look in `web/base.css` for the patch page: keep the control's own fill at `opacity: 0.4` and no edge change: `#patch-view .pb-btn:disabled { background-color: var(--pb-surface-hover); border-color: transparent; opacity: 0.4; }`, and give `.pb-btn--commit:disabled` the commit fill at the same opacity so Take keeps its place and shape.
- **Also check:** dark theme: `037-main-empty-dark.png`.

### A2-23: Select None is a stranded two-line button
- **Where:** under the Levels box; `037-main-empty-crop-boxes-4k2x.png`
- **Kind:** alignment
- **Severity:** low
- **Problem:** The button is 52 px wide (x 1396.1 to 1448.1). Its right edge meets nothing: the Levels box ends at 1332.6, Take at 1469.6, Clear at 1528.8. Its text is 11 px on two lines.
- **Change:** `web/desktop.css:307`: `--sode-all-w: auto;` is not possible with the fit sum, so set it to the remaining width: in `.sode__box--levels > .sode__picker` add `justify-content: space-between;` and in `.pb-btn.sode__all` (line 337) `width: auto; flex: 1 1 var(--sode-all-w); max-width: none;` so the button ends on the group's right edge (1528.8, under Clear) and its text fits on one line (`web/index.html:138`: one span "Select none").
- **Also check:** the same picker with 2 or 30 levels (`--sode-bar-cols`).

### A2-24: The Server hover box cuts the error with no way to read it there
- **Where:** `095-link-state-degraded-routers-hover-crop-tip-4k2x.png`
- **Kind:** readability
- **Severity:** low
- **Problem:** `Error  dial tcp 127.0.0.1:18911: connect: connection r…`. The cut is by design (`web/bar.css:262`), but the part that is cut is the reason ("refused"), and the address at the front is already known.
- **Change:** in `web/statusinfo.js` (the hover row for the error), show the text after the last `: ` first: `connection refused (127.0.0.1:18911)`.
- **Also check:** the uptime strip in the same box is all grey with one orange bar at the end and no legend; a first reading is "down all day". `Last 24 hours: 95.7% up, 2 drops` explains it, but grey for "no data yet" and orange for "now" are not named anywhere.

### A2-25: Protect picker cuts its last row in half
- **Where:** `081-protect-picker-all-destinations-crop-dialog-4k2x.png`, `082-...-1080.png` (screenshot)
- **Kind:** alignment
- **Severity:** low
- **Problem:** The list ends in the middle of row "A.19 MV 6", directly over the key hint line.
- **Change:** `web/protectpick.js` list height: a whole number of rows, `max-height: calc(var(--pb-control-h) * 18)` in place of the present height, as `.sode__menu` does (`web/desktop.css:156`).
- **Also check:** the Source "Show all" list (`046`), same pattern.

### A2-26: Space between the three groups of the patch row is 68 px and 34 px
- **Where:** patch row; `037-main-empty-1080.png`
- **Kind:** spacing
- **Severity:** low
- **Problem:** Source to Destination is 68 px (14 + the 40 px arrow + 14). Destination's `+` to Levels is 34 px (`web/desktop.css:20`, an empty 6 px track plus two 14 px gaps). The row looks pushed to the right.
- **Change:** owner's decision. To even it: make the empty track `calc(var(--sode-h-action) )` wide (40 px, the arrow's width), `grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr) var(--sode-h-action) minmax(min-content, 1fr);`. Both spaces are then 68 px.
- **Also check:** the flipped order (`data-flip`).

### A2-27: Not saved banner pushes the site name down and its link is a chip
- **Where:** `101-banner-not-saved-env-only-mode-1080.png`, `-crop-top-4k2x.png`
- **Kind:** design-language
- **Severity:** low
- **Problem:** The banner is in the flow between the bar and the site name, so the whole page starts 54 px lower than with a config file. "How to fix this" is a `pb-chip` used as a link (`web/index.html:63`), the only chip on the page that leaves the page.
- **Change:** `index.html:63` and `:755`: `class="pb-btn pb-btn--outline pb-btn--sm"`. Text: `Not saved: SODE_CONFIG is not set. A restart loses settings and users.`
- **Also check:** the Degraded note (`094`) is centred text over the site name with no box; it and this banner are two looks for a page-level warning. Give the Degraded note the same `pb-alert pb-alert--warn` box.

---

## Shared causes

1. **`.sode__group` is two things** (Settings fieldset and patch row group). Clears A2-01 and guards A2-10. Rename the patch row's class.
2. **Subgrid children with their own `gap`.** A subgrid whose gap differs from its parent's shifts its tracks by half the difference. Clears A2-01 and A2-10. Rule: every subgrid in `desktop.css` states both gaps.
3. **A `:where()` rule set before a plain rule of the same weight never wins** (`base.css:127` against `history.css:607`). Clears A2-02. Check the other `:where(...#patch-view)` rules at `base.css:121-129` against later plain rules.
4. **No single height for one row.** `--sode-h-action` (40) is forced on every `.pb-btn` of the patch page, while chips and boxes in the same rows keep 32, 24.5 and 22.5. Clears A2-02, A2-12, A2-13 and part of A2-17. Decide one height per row and state it in the base.css comment.
5. **Tips open down by default**, onto the content the control acts on, and an open tip is not checked again when a menu opens. Clears A2-04 and A2-15.
6. **Fixed parts at the right edge that do not add `--sode-page-bar`.** Clears A2-14.
7. **Text built from parts** (prefix plus title, "what" plus sentence). Clears A2-08 and half of A2-20. Build each line as one sentence with the name tested for the default.
