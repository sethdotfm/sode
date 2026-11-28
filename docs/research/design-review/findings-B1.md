# Findings B1

Slice: Settings > Routers (chips row, detail view, Address states, levels, level chips, level colors, Move, Swap, Remove, Add, 1 and 12 routers), Tie lines, Protects, Exit warning, Save review, PIN recheck.

Method: read the B screenshots (1080, 4k, 4k2x crops, dark), read the code in `/Users/seth/Desktop/sode/.claude/worktrees/review-shots/web`, and booted the same binaries with the `full` seed on ports 43080 and 43010 to 43012 to read bounding boxes with playwright-core at 1920x1080 DPR1. All px numbers below are measured at 1080 unless marked "from screenshot". At 1080 the tokens resolve to `--pb-space-1` 2px, `-2` 4px, `-3` 6px, `-4` 8px, `-5` 12px. Servers and Chrome are stopped. Work files are in `/private/tmp/claude-501/-Users-seth-Desktop-sode--claude-worktrees-happy-branch-merge-review-0c348b/1b1c74a8-cf3d-4546-aa99-2582e71b4297/scratchpad/review/B1work`.

Not measured live: the 12-router and 1-router fixtures, the Address states other than timeout, and the Free dialogs. Those entries come from the screenshots and the CSS.

## Tie lines

### B1-01: Tie lines table heads do not sit over their columns
- **Where:** Settings > Routers > Tie lines, every state with rows; `04-ties-overview-1080.png`, `04-ties-list-crop-crop-4k2x.png`, `12-many-tie-lines-1080.png`, `04-ties-add-duplicate-dark.png`
- **Kind:** alignment
- **Severity:** high
- **Problem:** Head cells start at x 636.8, 781.4, 941.2, 1101.0. Row cells start at x 636.8, 757.6, 891.0, 1024.3. "From (destination)" is 23.8px right of its column, "To (source)" 50.2px, "State" 76.7px. Cause: each row is its own grid and the last track is `auto`. The head's last cell is empty (0px) and a row's last cell holds Edit and Remove (103px), so the `fr` tracks resolve differently.
- **Change:** `web/base.css:1233`, `.sode__trow`. Current: `grid-template-columns: minmax(6rem, 0.9fr) minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1fr) auto;`. New: `grid-template-columns: minmax(6rem, 0.9fr) minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1.2fr) 6.5rem;` so the last track is fixed for head and rows. Add `.sode__trow > .sode__prow-act { justify-content: flex-end; }` (already true through `.sode__prow-act`).
- **Also check:** `.sode__prow` (`base.css:1062`) ends in `minmax(5rem, auto)`. It lines up today only because Free (41px) is under 5rem. A wider last cell (the "Try again" button after a failed Free, `settingsprotects.js:129`) will shift that row's columns the same way. Make it a fixed `6rem`.

### B1-02: State text wraps with one orphan character
- **Where:** Tie lines table rows; `04-ties-overview-1080.png`, `04-ties-list-crop-crop-4k2x.png`
- **Kind:** readability
- **Severity:** high
- **Problem:** "Carries A.101 TIE AB 1" needs 123px of text in a 125.3px column and breaks to two lines with a lone "1" on line 2. The name column holds a 52px name in 112.8px. Every seeded row wraps.
- **Change:** The `1.2fr` State track in B1-01 gives the column about 150px. Also keep the number and name together: in `web/settingstielines.js` near line 100 (the Carries text) wrap the carried item in a span with `white-space: nowrap`, so a long name wraps as a unit below "Carries".
- **Also check:** `.sode__tend-line` (number and name on one line, no ellipsis, `min-width: 0`): a 16-character name overflows the 125px column. Add `overflow: hidden; text-overflow: ellipsis; white-space: nowrap` to `.sode__tend-line` and `.sode__tend-title`, and a `title` attribute.

### B1-03: Add tie line button floats between two rows of tools
- **Where:** Tie lines tools bar; `04-ties-tools-crop-crop-4k2x.png`, `04-ties-overview-1080.png`, `12-many-tie-lines-1080.png`
- **Kind:** alignment
- **Severity:** high
- **Problem:** The find box is at y 850.9 to 882.9 and the router chips at y 888.9 to 920.9. Add tie line is at y 869.9 to 901.9, centred on the pair, so its top is 19px below the find box top and it shares no edge or centre line with either. The find box is 320px wide in a 368px span, so the chips always wrap below it. With 12 routers the count line moves to the right of the block and is also centred on nothing.
- **Change:** `web/index.html:581`. Move the button out of the nested tools: one row `[Add tie line] [find] ......` and the router filter on the next row with the count. Markup: `<div class="sode__ptools"><button id="tieline-new">…</button><span class="sode__ptools" id="tielines-tools"></span></div>` becomes `<div class="sode__ptools sode__ptools--top"><button id="tieline-new">…</button></div>` with `tools.replaceChildren(find, filter)` in `settingstielines.js:368` changed to put `find` beside the button and `filter` in the `.sode__pbar` row before `#tielines-count`. CSS: `.sode__ptools--top { align-items: flex-start; }`.
- **Also check:** Protects tools (`#protects-tools`) wrap the same way but have no button beside them, so they look right.

### B1-04: Suggested tie lines: Accept and Dismiss do not form a column
- **Where:** Suggested tie lines list; `04-ties-suggestions-crop-crop-4k2x.png`, `04-ties-overview-1080.png`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Each row is a wrapping flex line, so the buttons start where the text ends. Row 1 Accept starts at x 880.5, row 2 at 879.7. With the seed names the jitter is 1px and reads as sloppy. With real names of different lengths the buttons will scatter by tens of px.
- **Change:** `web/base.css:1216`, `.sode__tielines-row`. Current: `display: flex; flex-wrap: wrap; gap: var(--pb-space-4);`. New: `display: grid; grid-template-columns: minmax(6rem, 12rem) minmax(0, 1fr) auto auto; gap: var(--pb-space-2) var(--pb-space-4);` with the buttons in the two `auto` tracks at the right edge. Give the list a subgrid, or fixed tracks as written, so every row shares the columns.
- **Also check:** One-off tie lines in use (`#tielines-oneoffs-list`) uses the same row class.

### B1-05: Two Confirm button sizes in one page
- **Where:** Tie line Remove confirm and Protects Free confirm; `04-ties-remove-confirm-crop-crop-4k2x.png`, `05-protects-free-one-confirm-1080.png`
- **Kind:** design-language
- **Severity:** medium
- **Problem:** Tie line confirm: "Remove (Y)" is 143.6 x 48px at 17px text (`sode__confirm-ok`), Cancel is 32px high. Protects confirm: "Free (P)" is 80.7 x 32px at 13px text, no `sode__confirm-ok`. `base.css:686` says every churning-letter Confirm button has one size on every page.
- **Change:** `web/settingsprotects.js` near line 113, where `ok` is made: add the class `sode__confirm-ok`. Raise the confirm row: `base.css:1108`, `.sode__prow[data-confirm] { min-height: 3.5rem; }` to `min-height: calc(var(--sode-h-confirm) + var(--pb-space-4) * 2);`.
- **Also check:** `settingsprotectsfree.js` (Free Sode's Protects dialog) and the Clear History confirm.

### B1-06: Tie line error shows twice, and the words are unclear
- **Where:** Add tie line with a duplicate name; `04-ties-add-duplicate-1080.png`, `04-ties-add-duplicate-dark.png`
- **Kind:** text
- **Severity:** medium
- **Problem:** "Not saved: two tie lines are named "TIE AB 1"." shows in red inside the editor and again in plain text under the table (`#tielines-status`). The reader has not made two tie lines; one exists.
- **Change:** `web/tielinerules.js:53`: `two tie lines are named "${name}"` to `another tie line has the name "${name}"`. In `web/settingstielines.js:281` and `:286`, do not write the status line while the editor is open and shows the same error. Same rewording for line 46: `${shown(t.from)} already feeds ${shown(t.to)} as …` is good, keep.
- **Also check:** `tielinerules.js:14` to `18`, `:51`: lower-case fragments after "Not saved:". "a tie line goes from one router to a different router" should be "From and To must be on different routers".

### B1-07: Tie line editor fields have unequal heights
- **Where:** Add tie line editor, empty; `04-ties-add-empty-crop-crop-4k2x.png`
- **Kind:** spacing
- **Severity:** low
- **Problem:** From and To field boxes are 59.5px high, Tie line name is 55.5px. The 4px comes from the flex gap before the empty match list. The buttons row sits 6px under From and To but 10px under the name box.
- **Change:** `web/base.css` after line 1302: `.sode__tfind > .sode__menu:empty, .sode__tfind > [hidden] { display: none; }` (use the selector that matches the empty list node in `tielineedit.js`).
- **Also check:** Edit in the row (`04-ties-edit-crop-crop-4k2x.png`).

### B1-08: Tie lines intro and empty states
- **Where:** `11-one-tie-lines-crop-crop-4k2x.png`, `15-envonly-tie-lines-1080.png`, `04-ties-find-none-crop-crop-4k2x.png`
- **Kind:** text
- **Severity:** low
- **Problem:** "Tie lines need two or more routers. Add router in Routers adds one." and "No tie lines. Add tie line adds one." use a button name as the subject of a sentence, which does not read on the first pass.
- **Change:** `web/index.html:573`: `Tie lines need two or more routers. To add a router, use Add router in Routers.` `web/settingstielines.js:447`: `No tie lines. To add one, use Add tie line.`
- **Also check:** none.

## Routers

### B1-09: Multi destination label is 2px lower than the labels beside it, and its help button overlaps the select
- **Where:** Router detail > Takes; `03-routers-A-group-takes-crop-crop-4k2x.png`, `03-routers-A-group-levels-1080.png`
- **Kind:** alignment
- **Severity:** high
- **Problem:** "Multi destination" text top is y 1394.4. "Router salvo group" and "Re-read routes every (s)" are at y 1392.4. The wrapper `.sode__label-help` is 23.5px high against 19.5px labels, and the subgrid aligns labels to the row end, so the short labels sit lower in their box than the centred one. The 32px help button (y 1387.1 to 1419.1) reaches 4.2px into the select (top 1414.9), so its hover fill covers the select border.
- **Change:** `web/base.css:1432`, `.sode__help`. Current: `margin-block: calc(-1 * var(--pb-space-3));`. New: `height: var(--pb-leading-body, 1.5em); margin-block: 0; min-height: 0; width: 1.5em;` so the button is one line high (19.5px) and the wrapper stays 19.5px. If the 32px target must stay, set `.sode__label-help { height: 1.5em; }` and keep the negative margin at `calc((1.5em - var(--pb-control-h)) / 2)`.
- **Also check:** every other `.sode__help` (Export & Import, Access).

### B1-10: Router name and its action buttons share no centre line
- **Where:** Router detail head; `03-routers-detail-head-crop-4k2x.png`, `03-routers-overview-A-1080.png`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Name text centre is y 487.9 (25.5px line). Button centre is y 491.9 (32px). 4px apart, with `align-items: flex-start`. The name looks pushed up beside the buttons.
- **Change:** `web/routers.css:167`, add `.sode__rname { min-height: var(--pb-control-h); display: flex; align-items: center; }` so the name line is 32px and centres on the buttons. Keep `.sode__rhead { align-items: flex-start }` so the link state stays under the name.
- **Also check:** New router head ("Router D (new)" with Cancel, `03-routers-add-new-1080.png`), one-router head.

### B1-11: A long Router title wraps in a 208px box while the buttons keep 419px
- **Where:** Router detail head with a 32-character title (live test, title "Studio A Core Main Production XYZ")
- **Kind:** edge case
- **Severity:** medium
- **Problem:** `.sode__rhead-name` has `flex: 1 1 12rem`, the actions do not shrink, so the name gets 208.6px and wraps to two lines (51px) though the row is 640px wide. With `overflow-wrap: anywhere` a long word breaks mid-word.
- **Change:** `web/routers.css:160`: `.sode__rhead-name { flex: 1 1 100%; }` and `.sode__racts { flex: 1 1 100%; }`, so the name has the full row and the actions sit on the next row, left edge on the shared edge. This also removes B1-10.
- **Also check:** the chip handles it (title max 14rem with ellipsis), but the chip has no `title` attribute, so a cut title cannot be read. Add `title` in `settingsrouterlist.js` where `.sode__rrow-title` is made.

### B1-12: Two gaps between a control and its hint, and two gaps between fields, in one group
- **Where:** Router detail > Levels, Router names, Protects; `03-routers-A-group-levels-crop-crop-4k2x.png`
- **Kind:** spacing
- **Severity:** medium
- **Problem:** Inside `.sode__fields` the hint is 2px under the control (Level offset: input bottom 925.4, hint top 927.4). Outside it the hint is 4px under (Default levels: 1099.4 to 1103.4; Level numbering: 1015.4 to 1019.4). Between blocks: Level offset hint to "Level numbering" is 16px (8px field margin plus 8px group gap), Level numbering hint to "Default levels" is 8px. Levels (no hint) to "Level numbering" is 34.5px.
- **Change:** `web/routers.css:278`: `margin-top: var(--pb-space-1)` to `margin-top: var(--pb-space-2)` (matches the 4px flex gap of `.sode__field`). `web/routers.css:270`: remove `margin-bottom: var(--pb-space-4);` from `.sode__rgroup .sode__fields > .sode__field` and set `row-gap: var(--pb-space-4)` on `.sode__rgroup .sode__fields` instead of `row-gap: 0`, so the last row adds no extra 8px. The subgrid rows then need `grid-row: span 4` items to keep `row-gap` only between fields: set `row-gap: 0` on the field and add the gap as `padding-top` on every field after the first row, or simpler, keep the margin and add `.sode__rgroup .sode__fields { margin-bottom: calc(-1 * var(--pb-space-4)); }`.
- **Also check:** Connection group (Name character limit is alone on row 2).

### B1-13: Facts rows mix box heights of 28.5px and 32px
- **Where:** Router names facts row; `03-routers-A-group-names-crop-crop-4k2x.png`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** "Re-read 21:26:29" is a bordered box 28.5px high. "Re-read now" beside it is 32px. Both are bordered boxes with 11px text, 8px apart, so the 3.5px difference shows. The time box looks like a control but is not one.
- **Change:** Drop the box. `web/index.html:531`: `<span class="sode__chipbox" id="router-names-at">` to `<span class="sode__hint" id="router-names-at">`, and write the text as "Last re-read 21:26:29" (`settingsrouterdetail.js`, where `router-names-at` is set). Order: count, button, then the two hints.
- **Also check:** `.sode__chipbox` uses elsewhere (`base.css:1988`).

### B1-14: `pb-btn--sm` is the same height as a full button
- **Where:** Router head actions, Show in Protects, Show in Tie lines, table row buttons, filter chips, Accept and Dismiss
- **Kind:** design-language
- **Severity:** medium
- **Problem:** Every `pb-btn--sm` measures 32px high, the same as a normal button, with 11px text and 6px side padding. Add router (13px text) and Move up (11px text) are the same height one row apart, so the small text reads as a mistake and long labels look cramped ("Communication Log" 126.9px with 8px text inset). In rows of 19.5px text the 32px buttons force the row height (facts rows are 32px for one line of text).
- **Change:** Decide one. Either use normal buttons in the head (remove `pb-btn--sm` on `web/index.html:433` to `437`), or give small buttons the patchbay small height where Sode overrides it. Check which Sode rule sets `min-height` on `.pb-btn--sm` (patchbay's own `.pb-btn--sm` is shorter) and remove it for non-touch layouts.
- **Also check:** this is shared with the rest of Settings (other reviewer's slice).

### B1-15: One router: Add router sits alone at the right, with no count
- **Where:** `11-one-router-routers-1080.png`, `11-one-router-routers-crop-crop-4k2x.png`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** With one router the count line and the chips row are empty, so the bar holds only Add router at the right edge, 16px above the router name. It reads as stranded.
- **Change:** `web/settingsrouterlist.js:69`: always write the count, "1 router. 1 connected." Or with one router move Add router into `.sode__racts` in the head, before Swap.
- **Also check:** `15-envonly-routers-1080.png`.

### B1-16: 12 routers: find box and checkbox hang at the right under the chips
- **Where:** `12-many-routers-1080.png`, `12-many-routers-bar-crop-crop-4k2x.png`, `12-many-find-none-1080.png`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** From the screenshot: the chips fill two rows from x 629, then "Find a router" starts at x 883 and the checkbox label ends at 1269, right-aligned on a third row. With no match, "No router matches." and Show all routers sit left on one row and the find box stays right on the next. Three left-aligned rows and one right-aligned row.
- **Change:** `web/index.html:416` to `423`: move `.sode__rfind` out of `#routers-list-wrap` into `.sode__rbar`, between the count and Add router, so the order is count, find, checkbox, Add router on one bar. Remove `margin-left: auto` for it (`routers.css:15`) and keep it only on the Add router span.
- **Also check:** the find box is 12rem wide here and 20rem in Tie lines and Protects. Use one width.

### B1-17: Router count ignores a new router
- **Where:** `03-routers-add-new-1080.png`
- **Kind:** text
- **Severity:** low
- **Problem:** After Add router the bar says "3 routers. 3 connected." above four chips.
- **Change:** `web/settingsrouterlist.js:69`: append the unsaved count: "3 routers. 3 connected. 1 not saved."
- **Also check:** after Remove (chip marked "Remove on Save").

### B1-18: Remove router note: list indent, blank lines, and words
- **Where:** `03-routers-remove-confirm-1080.png`, live removed state
- **Kind:** text
- **Severity:** medium
- **Problem:** "Remove router B removes 3 tie lines:" repeats the verb. The list has the browser's 32px indent and 16px paragraph margins inside the note (text at x 660.8 against the 628.8 edge). After Remove the note reads "Router B goes when you save. The save asks for your PIN again." then the same "Remove router B removes…" line again. Keep is a ghost button, so it has no box beside the boxed Remove router B.
- **Change:** `web/routerlistrules.js:65`: head to `If you remove router ${prefix}, Sode ${parts}:` with parts "removes 3 tie lines" / "makes 2 presets Broken". `web/settingsrouterdetail.js:302`: `Router ${prefix} is removed when you save. Save asks for your PIN again.` `:315`: Keep to `pb-btn--outline` with the label `Keep router ${prefix}`. CSS `base.css:1017` `.sode__routers-uses`: `margin: var(--pb-space-2) 0; padding-left: var(--pb-space-6);` and `.sode__rnote p { margin: 0 0 var(--pb-space-2); }`.
- **Also check:** `settingsrouterdetail.js:310`: "Looking for what names router B…" to "Looking for items that use router B…".

### B1-19: Address check line mixes two type styles and grows the row
- **Where:** `03-routers-address-refused-crop-crop-4k2x.png`, `03-routers-address-timeout-crop-crop-4k2x.png`, `03-routers-address-same-as-other-router-crop-crop-4k2x.png`
- **Kind:** readability
- **Severity:** low
- **Problem:** "No answer yet. Sode keeps trying." in Plex Sans, then "dial tcp 127.0.0.1:1: connect: connection refused" in mono on two more lines in a 205px column. The row grows by up to 50px and pushes Name character limit down, so the form jumps as the check runs. The Go error text is not operator language. "Connected: 200 sources and 120 destinations discovered." uses "discovered", which no other text uses.
- **Change:** `web/routertry.js`: map the common errors to words: refused to "The router refused the connection.", timeout to "No answer from this address.", and keep the raw text in a `title`. "Connected: 200 sources and 120 destinations." Let the hint span the row so it stays on one line: `routers.css` add `.sode__rgroup .sode__fields > .sode__field > .sode__rtry { grid-column: 1 / -1; width: max-content; max-width: 40rem; }` is not possible in a subgrid cell, so instead move `#cfg-rt-address-try` out of the field to a full-width line under the Connection fields (`index.html:462`).
- **Also check:** the same address on two routers gives no warning (`03-routers-address-same-as-other-router`); it shows "Connected". Add "Router A uses this address also."

### B1-20: Level chips are 24px here and 22px in Level colors; rows do not end on the edge
- **Where:** `03-routers-A-level-chips-crop-4k2x.png`, `03-routers-levels-40-crop-crop-4k2x.png`, Look and feel > Level colors
- **Kind:** design-language
- **Severity:** low
- **Problem:** The same level slot is 24 x 24px in the router detail (`routers.css:233`, `height: 1.5rem`... measured 24) and 22 x 22px in Level colors. With 40 levels a row holds 24 chips and ends at x 1252, 17px short of the 1268.8 edge.
- **Change:** `web/routers.css:232`: remove `height` and the `max(1.5rem, …)` width floor; use the shared slot size `calc(var(--pb-space-8) + var(--pb-space-1))` from `base.css:487` for both, and `width: max(calc(var(--pb-space-8) + var(--pb-space-1)), calc(var(--sode-rchip-ch, 1) * 1ch + var(--pb-space-2)))`.
- **Also check:** Level colors slots 9 to 17 show the default as grey placeholder beside typed values 1 to 8 (noted in notes.md). Show "Default" as a hint, or fill the value.

### B1-21: Routers bar text
- **Where:** Routers section; `03-routers-overview-A-1080.png`
- **Kind:** text
- **Severity:** low
- **Problem:** "No Broken or Name moved items" has no verb and no period, and sits between the Status label hint and the count with no label. "Status label" hint: "The text next to the Routers icon in the status bar, for every router." is fine.
- **Change:** `web/brokencount.js:59`: `No item is Broken or Name moved.` Group heading hint `index.html:456`: "A change here reconnects this router only." to "A change here reconnects only this router."
- **Also check:** Prefix hint "One or two letters. B.69 is number 69 on this router." to "One or two letters. For example, B.69 is number 69 on router B."

### B1-22: Chip focus ring differs from every other control
- **Where:** `web/routers.css:77`
- **Kind:** design-language
- **Severity:** low
- **Problem:** `.sode__rrow:focus-visible { outline: 2px solid var(--pb-accent); outline-offset: 2px; }`. Patchbay's rule (`patchbay.css:69`) already gives `[tabindex]` the same outline with `outline-offset: 1px`. Measured: chip offset 2px, buttons 1px. Lines 87 and 90 use `inset 3px 0 0` in px.
- **Change:** delete `routers.css:77` to `80`. Lines 87 and 90: `3px` to `var(--pb-accent-bar-w)`.
- **Also check:** a selected chip has an accent border, an inset accent ring and a wash; with focus it shows three accent lines. Acceptable, but check in dark.

## Swap dialog

### B1-23: Find items and Only Broken touch, and sit on different centre lines
- **Where:** `03-routers-swap-dialog-1080.png`
- **Kind:** alignment
- **Severity:** high
- **Problem:** Find items is at x 652 to 750.2, y 583.8 to 615.8. The Only Broken label starts at x 750.2 (0px gap) at y 579.3 to 611.3, so the checkbox centre (595.3) is 4.5px above the button centre (599.8). The button top is 4.5px under the fields grid. The two are inline children of the body with no wrapper (`web/swap.js:194` to `196`). New number is alone on a second row at the left.
- **Change:** `web/swap.js:194`: wrap them: `el('div', 'sode__ptools', btn('pb-btn--outline', 'Find items', find, 'swap-find'), el('label', 'pb-check', broken, el('span', 'pb-check__label', 'Only Broken')))` and add `margin-top: var(--pb-space-4)` through a class. Set the grid to four columns in this dialog so Router, Swap, Old number, New number share one row: `.sode__swap .sode__fields { grid-template-columns: 1.4fr 1fr 1fr 1fr; }`.
- **Also check:** title is "Swap source or destination" but the button is "Swap…" and the footer says "Stage 0 items" when nothing is found. Disabled label should be "Stage items".

## Protects

### B1-24: Protects list keeps a fixed height and shows a large empty box
- **Where:** `05-protects-overview-1080.png`, `15-envonly-tie-lines-1080.png` (Protects under it), `11-one-protects-1080.png`
- **Kind:** spacing
- **Severity:** high
- **Problem:** `.sode__plist` has `height: clamp(16rem, calc(100vh - 31rem), 44rem)` (`base.css:1055`), measured 584px. With 6 rows there is 80px of blank bordered space; with one router and 2 rows (envonly shot) about 375px of the box is empty. The Tie lines list uses `max-height` with the same clamp and has no blank space.
- **Change:** `web/base.css:1055`: `height:` to `max-height:`.
- **Also check:** bigscroll on `.sode__plist` when the list is shorter than the box.

### B1-25: "Router settings" is a full-width ghost button with centred text
- **Where:** Protects group head for a router with device 0; `05-protects-overview-1080.png`, `05-protects-free-one-confirm-1080.png` (hover shows the box)
- **Kind:** alignment
- **Severity:** medium
- **Problem:** The button is 584px wide (x 676.8 to 1260.8) with its label centred at x 925 to 1012, under a hint that starts at x 676.8. It reads as loose centred text. The head grows from 42px to 94.5px for it.
- **Change:** `web/base.css:1149`, `.sode__pgroup-notes`: add `align-items: flex-start; gap: var(--pb-space-1);`. Better: put the note and the link on one line: `flex-direction: row; flex-wrap: wrap; align-items: center; gap: var(--pb-space-3);`. In `web/settingsprotects.js:185` use `pb-btn--outline pb-btn--sm` and the label "Open router settings".
- **Also check:** the hint "Protect is off for this router: Sode's device number is 0." to "Protect is off for this router, because Sode's device number is 0."

### B1-26: Protects heading, counts and Levels column
- **Where:** `05-protects-overview-1080.png`, `05-protects-tools-crop-crop-4k2x.png`
- **Kind:** text
- **Severity:** medium
- **Problem:** The legend "Protects" is followed by an 11px upper-case label "PROTECTS AND ROUTER LOCKS" that repeats it. "6 on 3 routers. Sode set 1." has no noun. "2 in all. Sode set 1, other devices 1." also. The Levels column is empty in every seeded row, so the head stands over nothing. "Free Sode's on A…" is hard to read; the ellipsis reads as a cut label, and "Device 77" before it has no explanation.
- **Change:** `web/index.html:606`: remove the `.sode__label` span (keep `data-label` for search). `web/protectlistrules.js:50`: `${n} Protects and Router locks on ${routers}. Sode set ${k}.` `:94`: `${g.total} here. Sode set ${g.sode}. Other devices set ${g.other}.` `web/settingsprotects.js:171`: `Free Sode's Protects on ${rt.prefix}…`. "Device 77" to "Sode is device 77". Levels cell: write "All" when the Protect covers every level.
- **Also check:** filter chips put the count inside the button with a 4px gap ("All 6", "Mine 0"); the count is the same weight as the label. Give `.sode__pcount` `color: var(--pb-text-muted)` on unselected buttons, or write "All (6)" as the select does for 12 routers.

### B1-27: Router filter changes form and width with many routers
- **Where:** `12-many-protects-1080.png`, `12-many-protects-tools-crop-crop-4k2x.png`, `12-many-tie-lines-tools-crop-crop-4k2x.png`
- **Kind:** design-language
- **Severity:** low
- **Problem:** Over `ROUTER_BUTTONS_MAX` the chips become a select. In Protects the select is 640px wide, under a 320px find box. In Tie lines it is 318px.
- **Change:** `web/base.css` after line 1038: `.sode__ptools > .pb-select-wrap { max-width: 20rem; width: 100%; }`.
- **Also check:** none.

## Save bar, review and PIN recheck

### B1-28: Review row for a removed router shows a cut sentence
- **Where:** Save review after Remove router (live; not in the B shots)
- **Kind:** bug
- **Severity:** high
- **Problem:** Row: Setting "Remove B Studio B Core", Was "Kept", Now "Remove router B removes 3 tie lines:". The Now cell reuses the note's list head (`settingsrouterdetail.js:579`), ends in a colon, and the list is not there.
- **Change:** `web/settingsrouterdetail.js:577` to `582`: build the Now text from the uses, not the head: `Removed. Also removes 3 tie lines.` / `Removed. Makes 2 presets Broken and removes 3 tie lines.` / `Removed.` Label (line 568) to `Router ${loadedName(uid)}`, Was to `In use`.
- **Also check:** `:563` "Router order. Default router: A → C" uses an arrow; write "Default router: was A Studio A Core, now C Playout C".

### B1-29: "Backup" heading touches the review table
- **Where:** Save review with a Critical change (live); layout of `03-save-review-dialog-1080.png`
- **Kind:** spacing
- **Severity:** medium
- **Problem:** The table ends at y 557.7 and the `h3.sode__label` "Backup" box starts at y 557.7 with a 13.2px line, so its text top (556.7) is 1px above the table's bottom rule. Gap is 0px. Under it the gap to the buttons is 8px.
- **Change:** `web/base.css` near line 1001: `.pb-modal__body > h3.sode__label { margin: var(--pb-space-5) 0 var(--pb-space-2); line-height: var(--pb-leading-tight, 1.5); }`.
- **Also check:** the "Interruptions" heading (`savereview.js:66`) has the same class. In the table, the topic row "Routers" and the section row "ROUTERS" repeat one word on two lines; when topic and section have the same name, show one.

### B1-30: PIN recheck words and button
- **Where:** `03-save-pin-recheck-1080.png` (the shot missed the dialog), live `pin.png`
- **Kind:** text
- **Severity:** medium
- **Problem:** Body: "Saving these changes: Critical. Enter your PIN." in 11px hint text. "Critical" is a rule name, not a reason. Continue is an outline button with a ring, while the step before used the filled "Save 1 change"; the primary action changes look between two steps of one flow.
- **Change:** `web/recheck.js:51`: `` `${what} needs your PIN again. Enter your PIN.` `` giving "Saving these changes needs your PIN again. Enter your PIN." Use body text size, not `.sode__hint`. Make Continue `sode-btn--primary` and label it "Save".
- **Also check:** `settingsprotectsfree.js:57`: "Sode asks your PIN again after this step." to "Sode asks for your PIN again after this step."

### B1-31: Save bar: the path is cut at the end, and the Exit warning changes the bar
- **Where:** `03-routers-savebar-crop-crop-4k2x.png`, `03-routers-exit-warning-crop-crop-4k2x.png`, `03-routers-save-bad-address-1080.png`
- **Kind:** readability
- **Severity:** medium
- **Problem:** "Saved to /private/tmp/claude-501/-Users-seth-Desktop-sode--claude-work…": the `code` box is 1182.8px wide in a 502px slot and the ellipsis removes the file name, the part that matters. On Exit with changes the bar shows "Unsaved changes in Routers" in amber, Exit becomes "Discard and exit" (53.9px to about 122px wide), and the path hint goes, so the row changes shape. On a save error the bar shows the red line and "Unsaved changes" together: "A setting has an error. A Studio A Core: Address. Enter the address as host:port, for example 10.0.0.50:8910." Three sentences, the middle one is not a sentence.
- **Change:** `web/base.css` add `#settings-path > code { direction: rtl; text-align: left; unicode-bidi: plaintext; }` or cut the path in the middle in `settingssave.js:233`. Error line (`settingsrouterdetail.js:612`): `Router A Studio A Core, Address: enter the address as host:port, for example 10.0.0.50:8910.` Exit warning (`settingssave.js:191`): `Routers has unsaved changes. Save, or press Discard and exit.` Keep the path hint in place while the warning shows.
- **Also check:** "Applied 20:49:39; not saved: " (`settingssave.js:230`) uses a semicolon; write "Applied 20:49:39. Not saved to the file: ".

### B1-32: At 4K the Settings column uses about a fifth of the screen
- **Where:** `03-routers-overview-A-4k.png`, every 4k shot in the slice
- **Kind:** readability
- **Severity:** medium
- **Problem:** From the 4k shot: the content column is about 800px of 3840px, text about 16px. The router chips, tables and the 12-router list are no easier to read on a large screen, and the Tie lines and Protects tables keep the narrow columns that cause B1-02.
- **Change:** owner decision, shared with the other Settings reviewer: let the Routers, Tie lines and Protects sections use a wider measure (for example `max-width: 60rem` for `fieldset[data-topic="routers"]`) while form sections keep 40rem.
- **Also check:** all of Settings.

## Shared causes

1. Row grids that end in an `auto` track (`.sode__trow`, `.sode__prow`): a fixed last track clears B1-01 and B1-02 and protects the Protects table.
2. Rows built as wrapping flex lines where columns are meant (`.sode__tielines-row`, `.sode__ptools` nested in `.sode__ptools`, `.sode__rhead`, the swap dialog body): a grid or one wrapper clears B1-03, B1-04, B1-10, B1-11, B1-16, B1-23.
3. `pb-btn--sm` at 32px height: one decision clears B1-14 and eases B1-13 and the facts rows.
4. Text reused from another place (`usesNote().head` in the review, the note head repeated after Remove, the tie error in two places): clears B1-06, B1-18, B1-28.
5. Hint and label spacing set per container (`.sode__rgroup .sode__fields` overrides against `.sode__field` gap, `h3.sode__label` with no margin, `.sode__help` negative margin): clears B1-09, B1-12, B1-29.
6. Fixed sizes where the content should decide (`.sode__plist` height, 22px against 24px slots, find box 12rem against 20rem): clears B1-20, B1-24, B1-27.

## Summary

32 entries: 7 high, 17 medium, 8 low.
Worst 1: Tie lines table heads are 24px, 50px and 77px off their columns, and every State cell wraps with one orphan character (B1-01, B1-02).
Worst 2: the Save review shows the cut sentence "Remove router B removes 3 tie lines:" as the new value of a removed router (B1-28).
Worst 3: controls that share a row but no line: Add tie line between two tool rows, Find items touching Only Broken 4.5px off centre, Multi destination label 2px low with its help button over the select (B1-03, B1-23, B1-09).
Also high: the Protects list is a fixed 584px box that stays mostly empty with few rows (B1-24).