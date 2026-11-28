# Findings C2

Scope: `/panels`, `/panels/editor`, `/panels/<id>` (Buttons and Grid). Code read at `/Users/seth/Desktop/sode/.claude/worktrees/review-shots` (origin/alpha-round2, 8e0969a7). Numbers are px at 1920x1080 DPR1, read from bounding boxes on a private copy of the seeded app (Sode on :46080, simulators on :46010 to :46012, all stopped again). The Grid prototype was served read-only from the `proto-grid` worktree on :46090 for the comparison. Nothing in the repo was changed.

Not verified: scene 57 (context menu) and 80 (Live Editor pick) did not capture their state, so the Smart menu is reviewed only from scene 54. Scene 166 and 167 (router down) show no fault mark on the panel at all; I did not reproduce a router-down state, so C2-41 is from the screenshots only.

Grid speed at 2900 x 990: 120 scroll steps of 400 px across and 150 px down gave median 17.8 ms, p95 26.1 ms, max 34.8 ms per frame in headless Chrome, with 4037 DOM nodes. No finding.

---

## /panels (list and detail)

### C2-01: Action buttons clip and overlap at 200% zoom
- **Where:** /panels detail pane at a 960 px viewport (200% of 1920). My capture `C2work/z-panels.png`.
- **Kind:** alignment / bug
- **Severity:** high (at 200%)
- **Problem:** "Launch" shows as "aunch", "Push to panels" as "Push to pane", "Send a copy" is cut, and Edit sits under Delete. The columns are `minmax(0, 13rem)`, so they shrink below their words, and Delete is a third `1fr` column that reaches 0.
- **Change:** `web/panels.css:563-572`. Replace
  `--sode-ov-act: minmax(0, 13rem); … grid-template-columns: repeat(2, var(--sode-ov-act));` and `.sode-ov__acts-main { grid-template-columns: repeat(2, var(--sode-ov-act)) 1fr; }`
  with one wrapping rule for both rows: `display: flex; flex-wrap: wrap; gap: var(--pb-space-3);` and `.sode-ov__acts .pb-btn { flex: 1 1 11rem; max-width: 13rem; }`, `.sode-ov__delete { flex: 0 0 auto; margin-inline-start: auto; }`.
- **Also check:** `.sode-ov__big` (`panels.css:534`) keeps two columns down to 52rem; at 960 px the right column is 190 px. Raise the one-column breakpoint to 64rem.

### C2-02: List rows have six different heights
- **Where:** /panels list; `26-panels-list-default-1080.png`, `-crop-list-4k2x.png`
- **Kind:** alignment
- **Severity:** high
- **Problem:** Row heights are 60, 60, 78, 58, 51, 58, 60, 108 px, because each row takes its thumbnail's height (a Buttons 8x4 picture is 48 px, a 2-row one 39, a Grid picture 46 to 96). Names do not sit on a regular pitch and the list looks ragged.
- **Change:** `web/panels.css:490` `.sode-ov__item`: add `min-height: 4.5rem;` and cap the list picture: `.sode-ov__thumb--sm { max-height: 3.75rem; overflow: hidden; }`; for a Grid picture in the list use the Small size only (`panels.css:662`, set `--sode-ov-cols` to at most 5 in `panelslist.js` when the thumb is `--sm`). Every row is then 72 px.
- **Also check:** the Grid thumbnails are left-aligned in a 96 px slot (`margin-inline-end: calc(6rem - var(--sode-ov-w))`), the Buttons ones fill it; centre the small ones with `margin-inline: calc((6rem - var(--sode-ov-w)) / 2)`.

### C2-03: "All Offline" for a panel that was never opened
- **Where:** /panels list, every row; `panelsrules.js:229-235`
- **Kind:** text
- **Severity:** medium
- **Problem:** A panel with no instance reads "All Offline", which says that instances exist and failed. The capital O is also wrong mid-phrase. The status is grey text with no icon, the same weight as "All Online".
- **Change:** `panelsrules.js:233-235`:
  `if (!known) return { text: 'Not open', tone: '' };`
  `'All Online'` → `'All online'`, `'Some Online'` → `'Some online'`, `'All Offline'` → `'All offline'`.
- **Also check:** the comment on line 229 ("No instance at all reads All Offline").

### C2-04: List subline is not a sentence and is cut
- **Where:** /panels list; "You, signs in as the user", "Ben, API client or the us…"; `panelsrules.js:189` (`actsShort`)
- **Kind:** text
- **Severity:** medium
- **Problem:** "You, signs in as the user" does not parse. The Both text is too long for the 20rem list and ends in an ellipsis on every such row.
- **Change:** `SHORT` table and its fallback in `panelsrules.js`: session → `'User sign-in'`, api → `'API key'`, both → `'Key or sign-in'`, public → `'Public'`. The row then reads "You · User sign-in". Join with `' · '` in place of `', '` in `panelslist.js` where owner and `actsShort` are joined.
- **Also check:** the same words in the detail (C2-05).

### C2-05: Detail facts read badly
- **Where:** /panels detail `dl`; `panelslist.js:157-159`, `panelsrules.js:144-151, 207`
- **Kind:** text
- **Severity:** medium
- **Problem:** "Acts as: the user (asks for a PIN at launch)", "an API client for anyone with the link", "an API client with a key, else as the user (a link without a key asks for a PIN)", "Owner: Ben (read only)". The label is "Acts as" here, "Authentication mode" in the editor.
- **Change:** label `'Acts as'` → `'Sign-in'` (the glossary lets the list keep "Acts as", but one word for both pages reads better; keep `Acts as` if the owner wants it). Values:
  - session: `The user. The panel asks for a PIN.`
  - api: `An API client. The link holds a key.`
  - both: `An API client with a key. Without a key, the user.`
  - public: `Public. Anyone with the link.`
  - owner: `Ben (you cannot edit this panel)`
  - `'Only the owner'` → `'Only you'` when the viewer is the owner.

### C2-06: Two "Notify when offline" boxes, one stranded at the far right
- **Where:** /panels detail with an open instance; `42-panels-open-now-instances-1080.png`; `panelsmon.js:196, 232`
- **Kind:** alignment / text
- **Severity:** medium
- **Problem:** The panel box (x 897) and the instance box (x 1595, right edge 1729) have the same words. The instance box is 458 px from its row's Reload button, alone at the page edge. At 4K it is 1200 px from the instance name.
- **Change:** `panelsmon.js:232` label → `Notify me when this panel is offline`; `panelsmon.js:196` label → `Notify me` with `data-pb-tip="Notify me when this instance is offline"`. In `web/panelmon.css` remove the `margin-inline-start: auto` on `.sode-ov__watchone` so it follows Reload at `var(--pb-space-3)`.

### C2-07: Open now rows do not share columns
- **Where:** /panels Open now; `42-…-crop-detail-4k2x.png`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Row 1: badge 52.8 px, name at x 955.6. Row 2: badge "No key" 54.2 px, name at x 957.0 (1.4 px off). Reload is at x 1063.6 in row 1 and 1048.4 in row 2 (15 px off). The "Offline" word trails after Reload in row 2. Row 1 is 63.5 px high (uptime strip), row 2 is 32.
- **Change:** `web/panelmon.css:47` `.sode-ov__inst`: `display: grid; grid-template-columns: 4.5rem minmax(6rem, max-content) 2rem 2rem max-content 1fr; align-items: center; gap: var(--pb-space-3);` and give `.pb-badge` there `justify-self: start`. Put the address (`::1`) and the "Offline" word in the last column.
- **Also check:** `h2.sode-ov__group` in `.sode-ov__openhead` is 2 px low (centre 413.4 against Reload all 411.4) from its `margin: space-4 0 space-2` (`panels.css:487`); add `.sode-ov__openhead > .sode-ov__group { margin: 0; }` in `panelmon.css`.

### C2-08: Raw address and "No key" need words
- **Where:** Open now row 2 ("No key  Desk  ::1  Reload  Offline"), Assign rows ("::1, Signed in as Ada, last seen 5 minutes ago")
- **Kind:** text
- **Severity:** low
- **Problem:** "::1" has no label and reads as a glitch. "No key" beside "Offline" gives two states with no order.
- **Change:** show the address as `from ::1` (`panelsmon.js` near line 148, `panelassignrules.js:54, 73`). Assign subline: `From ::1. Signed in as Ada. Last seen 5 minutes ago.`

### C2-09: Panel picture is inset from the title and caption
- **Where:** /panels detail left column
- **Kind:** alignment
- **Severity:** low
- **Problem:** Title and caption start at x 500.8. The picture's first square starts at 505 (1 px border + 4 px padding, `panels.css:634-640`), and the picture has a border that nothing else on the page has.
- **Change:** `.sode-ov__thumb:not(.sode-ov__thumb--sm) { border: 0; padding: 0; }`.

### C2-10: Delete confirm changes the button's place and size
- **Where:** /panels detail; `41-panels-delete-confirm-1080.png`; `panelslist.js:146`
- **Kind:** readability / design-language
- **Severity:** medium
- **Problem:** "Delete" (90 px, ghost) becomes "Delete? Press again" (178 px, filled grey) with no danger tone. Patchbay's rule is status with text and a clear danger action; a grey fill does not read as armed.
- **Change:** `panelslist.js:146`: armed text `Press again to delete`; armed class `pb-btn--danger` in place of `pb-btn--ghost`. Give `.sode-ov__delete` `min-width: 11rem; justify-content: flex-end;` so the left edge of the words does not jump.

### C2-11: Keyboard hint chips in Launch and Edit are drawn two ways
- **Where:** /panels detail; `44-…-crop-buttons-4k2x.png`
- **Kind:** design-language
- **Severity:** low
- **Problem:** `kbd.sode-ov__kbd` is 9.x px text in a white box on the blue Launch and on the grey Edit. On Launch the white box is the strongest thing in the button.
- **Change:** `panels.css:577`: add `background: transparent; border: var(--pb-border-w) solid currentColor; color: inherit; opacity: 0.8; font-size: var(--pb-text-xs);`.

### C2-12: Find with no match leaves the page empty
- **Where:** `35-panels-find-empty-1080.png`
- **Kind:** text / edge case
- **Severity:** low
- **Problem:** "No panel matches." at x 175 (6 px in from the Find box at 169) and the detail pane goes blank.
- **Change:** text → `No panel has "zzz" in its name.` with a `Clear` button; `.sode-ov__empty { padding: var(--pb-space-3) 0; }` (`panels.css:526`).

---

## /panels dialogs

### C2-13: Dead flex gaps inside tab panels (shared cause)
- **Where:** Share > People, Share > Link with key, Push > Push, and the editor's side panel
- **Kind:** spacing / bug
- **Severity:** high
- **Problem:** `.sode-ov__sharebody`, `.sode-ov__keytab` and `.sode-side__tab` set `display: flex; gap: …`, but each is also a `.pb-tabpanel.active`, and patchbay's `.pb-tabpanel.active { display: block }` (`patchbay.css:1217`) has higher specificity. The gaps never apply. Measured:
  - Share > People: button group bottom 416.3 = "Item link" label top 416.3 (0 px).
  - Link with key: last key row bottom 511.6 = "New link" top 511.6; field bottom 583.9 = mode group top; group bottom 615.9 = Make link top; Make link bottom 647.9 = "Allowed networks" top. Five stacked 0 px joins.
  - Push: list bottom 683.8 = "Push to 1 panel" top; the last row is cut by the button.
  - Editor side: each section's last control touches the next section's rule (0 px above the rule, 18 px below). "Item link" looks underlined (`49-…-crop-side-4k2x.png`).
- **Change:** `web/panels.css`, next to the existing override at line 1160:
  `.pb-tabpanel.active:is(.sode-ov__sharebody, .sode-ov__keytab, .sode-side__tab) { display: flex; }`
  and for Push add `#panel-push.active { display: flex; flex-direction: column; gap: var(--pb-space-4); }`.
- **Also check:** `#panel-sets`, `#panel-restore`, the Live Editor drawer tabs.

### C2-14: Dialog footers place Cancel and Close three ways
- **Where:** Share (Cancel at x 893.6, the middle), Send a copy (Close left at 672, Send right), Push, Assign, Templates (Close right at 1183); `panelshare.js:87`, `panelpush.js:198`, `panelassign.js:152`
- **Kind:** design-language
- **Severity:** high
- **Problem:** Patchbay: "Cancel left, confirm right, always" (`patchbay.css:1390`). The footers start with an empty note span, so `space-between` pushes Cancel to the centre of Share and Close to the right elsewhere.
- **Change:** `panelshare.js:87` → `foot: [btn('pb-btn--outline', 'Cancel', …), note, saveBtn]`; `panelpush.js:198` and `panelassign.js:152` → `foot: [btn('pb-btn--outline', 'Close', …), note]`. Same order in `editortemplates.js`.

### C2-15: Assign list rows are flush and their highlight has no padding
- **Where:** `40-panels-assign-dialog-1080.png`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Find box bottom 537.5 = first row top 537.5 (0 px). The row text starts at x 672, the Find box text at 680, so the names sit 8 px left of the text above. The selected row's grey fill and accent bar start under the first letter of "Desk". The Assign buttons look disabled (grey text on white) although they work.
- **Change:** `panels.css:617` `.sode-ov__cp`: add `padding: var(--pb-space-2) var(--pb-space-3);`; `#assign-list { margin-top: var(--pb-space-3); }`. Hint text (`panelassign.js:88-89`):
  - `This instance opens ${panel.name} immediately and gets a new key for it. Its key for the panel it shows now stays valid.`
  - `This instance opens ${panel.name} immediately and shows the sign-in. The person there types their PIN. Its key for the panel it shows now stays valid.`

### C2-16: Key rows have a ragged middle column and odd words
- **Where:** Share > Link with key; `164-panels-share-link-with-key-1080.png`; `panelkeyrules.js:52`, `panelkeytab.js:108, 314`
- **Kind:** alignment / text
- **Severity:** medium
- **Problem:** The "Made … Last used …" text starts at x 912.8 in two rows and 882.9 in the third, since it is right-packed by its length. "Last used never." is not English. "3 of 20 instances" does not say it is a limit. "Allowed networks (one per line, empty = any)" uses a symbol as a verb. "Save networks" is a full-width disabled bar that looks like a rule.
- **Change:** `panels.css:977` `.sode-ov__keyname { flex: 0 0 12rem; }` and give the middle span `flex: 1 1 auto;` so it starts on one edge. Text: `Last used ${…, 'never'}` → `Never used.` when there is no use, else `Last used 57 minutes ago.`; `${n} of ${max} instances` → `${n} instances of ${max} at most`; label → `Allowed networks`, hint under it `One on each line. Empty lets all addresses in.`; `Save networks` → `pb-btn--sm`, `align-self: flex-start`. Mode labels (`panelkeyrules.js:13-14`): `Named link` → `Link for one instance`, `Setup link, 1 hour` → `Setup link (1 hour)`.

### C2-17: Push list is cut and has no words for what Push does
- **Where:** `38-panels-push-dialog-1080.png`, `165-…`
- **Kind:** readability
- **Severity:** medium
- **Problem:** The list is `max-height: 18rem` (`panels.css:631`) and the ninth row is half hidden under the button. The big scroll bar thumb (22.4 px) is as wide as a checkbox column. The tab has no line that says Push replaces the layout of the ticked panels.
- **Change:** after C2-13 the cut goes. Add a hint above the list: `Push copies this panel's layout to the panels you tick. Each panel keeps a checkpoint of what it had.` Group rows by owner with the `sode-ov__group` heading instead of "(Ben)" after each name.

### C2-18: Share > People labels
- **Where:** `37-panels-share-dialog-1080.png`; `panellinkrow.js:66`, share host field
- **Kind:** text
- **Severity:** low
- **Problem:** "Item link" is glossary-correct, but "Address for other devices" with placeholder "192.168.1.20:8080" does not say why. The QR code has no caption.
- **Change:** label → `Address of Sode for other devices`; hint → `The link and the QR code use this address. Empty uses the address of this page.`; caption under the QR: `QR code of the Item link`.

---

## /panels/editor (Buttons)

### C2-19: Pages column is 2 px lower than the grid
- **Where:** editor main; `49-editor-buttons-panel-tab-1080.png`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Both headings are at y 122. The first page row starts at 143.6, the grid frame at 141.6.
- **Change:** `web/panels.css` near 1536: the pages nav uses `gap: var(--pb-space-3)` (6 px) and the frame `var(--pb-space-2)` (4 px). Set both to `var(--pb-space-2)`.

### C2-20: Side panel changes width between tabs
- **Where:** editor side; `49-…` against `50-…`, `58-…`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** The tab bar and fields are 365.6 px wide on Panel (the big scroll bar takes 22.4 px) and 388.0 px on Cell and Access. Every switch moves the right edge by 22.4 px. Top padding is 26 px, side padding 14 px.
- **Change:** `panels.css:1641` `.sode-ed__side`: always keep the bar's room: `padding-inline-end: calc(var(--sode-bigscroll-w) + var(--pb-space-5));` (use the bigscroll width variable), and `padding-block-start: var(--pb-space-5)` to match the sides.

### C2-21: "Show" heading is three times the size of every other label
- **Where:** Cell tab of an Info cell; `54-editor-buttons-cell-info-1080.png`
- **Kind:** design-language
- **Severity:** high
- **Problem:** The `legend.sode__field-label` is 36 px high (Bootstrap's `legend` size). Sibling labels are 19.5 px.
- **Change:** `panels.css:1663` add `#ed-cell-show > legend { float: none; font-size: inherit; line-height: inherit; margin: 0 0 var(--pb-space-2); padding: 0; width: auto; }`.
- **Also check:** every other `fieldset > legend.sode__field-label` in the editor and Share.

### C2-22: Cell tab heading is centred while all else is left
- **Where:** "Source at row 1, column 1"; `50-…-crop-side-4k2x.png`; `panels.css:1897`
- **Kind:** alignment
- **Severity:** low
- **Problem:** The one centred line in a left-aligned form; it changes x with each cell kind.
- **Change:** delete `.sode-side__kind { text-align: center; }` and make it the tab's heading: class `sode-side__h`, text `Source, row 1, column 1`.

### C2-23: Rule under controls, and the stray rule beside Font size
- **Where:** Panel tab; `49-…-crop-side-4k2x.png`
- **Kind:** spacing
- **Severity:** medium
- **Problem:** Besides C2-13, "Show the title bar | Font size 100 %" has a 2 px vertical rule that touches the checkbox label (label ends 1456.1, field starts 1460.1). Two unrelated settings share one row. `Mode` and "Authentication mode" button groups carry a bottom rule too.
- **Change:** `editorpanel.js`: put Font size on its own row under the checkbox and drop the divider class on `.sode-side__inline` (`panels.css`, the `border-inline-start` rule on `.sode-side__sizerow > .sode-side__inline`).

### C2-24: Editor labels
- **Where:** Panel, Cell and Access tabs; `editorpanel.js:131-147, 211`, `editorslug.js:26`, `editorfixed.js:72`, `editorcell.js:41`, `editordefault.js:64`, `editoraccess.js:76, 154, 170`, `editorties.js`
- **Kind:** text
- **Severity:** medium
- **Problem and change** (current → new):
  - `Id p-e5d39e14` / `Owned by user "Ada"` → `ID p-e5d39e14` / `Owner: Ada`
  - `Short name` (placeholder `studio-a`) → `Short name for the link`, hint `Letters, digits and hyphens. Empty uses the ID.`
  - `Take button behavior` with `None / Hold / Twice` → `Take guard` with `Press / Hold / Press twice`
  - `Fixed destinations`, hint `Off` → hint `None. The panel shows the destination that a person presses.`
  - `Stage several Destinations at the same time` → `Stage more than one destination`
  - `Tie lines`, `Off: the panel refuses a route across routers.` → `Off. The panel refuses a route that crosses routers.`
  - `Text override` → `Text on the cell`, hint `Empty shows the router name.`
  - `No Default artwork for this item yet.` → `This source has no Default artwork.` (name the kind)
  - `Client Panel p-bcb3df97. Rights.` → `API client: Panel p-bcb3df97. Its Rights:`
  - `Limit this panel's access` → `Limit the Rights of this panel`
  - `Who can use it:` + bullet list → `Shared with: Everyone` on one line; use the list only for named users.
  - `Automatically enter fullscreen` → `Go to full screen at the first press`
  - `Enforce landscape orientation` → `Keep the page in landscape`
  - `Ask users to set an instance name` → `Ask for an instance name` (the glossary's own term)
  - Mode `Stage + Take` on a Buttons panel and `Stage + Take + Confirm` on a Grid: keep, the glossary names them.

### C2-25: Access tab table and token typo
- **Where:** `59-editor-api-access-tab-crop-side-4k2x.png`; `panels.css:1900-1906`
- **Kind:** alignment / bug
- **Severity:** medium
- **Problem:** Labels at x 1326.8, values at x 1538.5: 212 px of space between "Sources" and "All", with a rule under the value cell only. `font-weight: var(--pb-weight-regular)` names a token that does not exist (tokens are `-normal`, `-medium`, `-bold`); it works only by accident. The "Who can use it:" hint is followed by a 24 px gap and a bullet that hangs left of the text column.
- **Change:** `.sode-side__table { width: max-content; } .sode-side__table th { font-weight: var(--pb-weight-normal); color: var(--pb-text-muted); padding-inline: 0 var(--pb-space-5); } .sode-side__table td { border: 0; padding-inline: 0; }`. `.sode-side__people { list-style: none; padding: 0; }`.
- **Also check:** `panels.css:937` and `:1533` use `font-weight: 600`; use `var(--pb-weight-bold)`.

### C2-26: Toolbar state and Save
- **Where:** editor toolbar; `49-…`, `64-editor-buttons-unsaved-1080.png`, my `z-editor.png`
- **Kind:** alignment / readability
- **Severity:** low
- **Problem:** "Saved" is plain grey text in a 101.9 px slot with Save 12 px after it; at 960 px the pair wraps alone to a second row under "Panel". Rename has an icon, the five other toolbar buttons have none.
- **Change:** `.sode-ed__bar > .sode-ed__grp:last-child { margin-inline-start: auto; }` so the pair stays right when it wraps; drop the icon from Rename or give Share, Copy link, Open panel and Live Editor their icons (`share-2`, `copy`, `external-link`, `play`).

### C2-27: Info cell text overflows its square
- **Where:** editor grid, Info cell "Panel Name · Main · Instance Name"; 1080 fits in three lines, at 960 px (`z-editor.png`) it is cut top and bottom
- **Kind:** edge case
- **Severity:** medium
- **Problem:** The square has no overflow rule for long text; the icon is pushed out of the top.
- **Change:** `.sode-ed__sq .sode-ed__text { display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 3; overflow: hidden; }` and show the parts on lines, as the panel page does ("Studio A / Main / Desk"), not joined with " · ".

### C2-28: Live Editor of a Buttons panel
- **Where:** `70-editor-buttons-live-editor-1080.png`
- **Kind:** text / alignment
- **Severity:** low
- **Problem:** "Panel Live Editor [Preview] Back to editing": the badge says Preview, but the glossary says takes there change the router. The Live grid is 1560 px wide, the editor grid before it was 896, so every cell jumps.
- **Change:** `editorlive.js:66-68`: title `Live Editor`, badge `pb-badge pb-badge--warn` with text `Live: takes change the router`, button `Back to the editor`.

---

## /panels/editor (Grid)

### C2-29: List toolbar buttons do not sit over the row buttons
- **Where:** Sources and Destinations lists; `71-…-crop-lists-4k2x.png`, `76-…-crop-lists-4k2x.png`
- **Kind:** alignment
- **Severity:** high
- **Problem:** Toolbar: pin 424.8, up 534.8, down 568.8, remove 644.8. Rows: pin 529.6, up 563.6, down 597.6, remove 631.6. So pin is 104.8 px off, up and down 28.8 px off, remove 13.2 px off. The toolbar runs to the column edge (676.8), the rows stop 11.2 px short for the scroll bar. When a list is too short to scroll (Destinations in scene 76) its rows run to the edge, so the two columns of one page differ.
- **Change:** `panels.css:1350` `.sode-edge__acts`: `display: grid; grid-template-columns: 1fr repeat(8, var(--pb-control-h-sm)); padding-inline-end: calc(var(--sode-littlescroll-w) + var(--pb-border-w) * 2);` and order the buttons so pin, up, down and remove are the last four, over the row's four. Keep the gutter on the list always: `.sode-edge__list { scrollbar-gutter: stable; }` or the bigscroll equal.

### C2-30: Row numbers push the names off one edge
- **Where:** Grid editor rows; "B.9 Replay A" name at x 91, "B.10 Replay B" at x 105 in the 2x crop
- **Kind:** alignment
- **Severity:** medium
- **Problem:** `.sode-pages__num { min-width: 1.25rem }` (`panels.css:1583`) fits "B.4" only. Two-, three- and four-digit numbers ("A.2500") widen it, so names start at a different x on each row.
- **Change:** `.sode-edge__row .sode-pages__num { min-width: calc(var(--sode-num-ch, 4) * 1ch + 0.5ch); text-align: end; font-variant-numeric: tabular-nums; color: var(--pb-text-muted); }` and set `--sode-num-ch` on the list from the longest label (`editorgridlist.js`).
- **Also check:** the router list under "Add from the router" has the same ragged names.

### C2-31: Heading row moves when the note shows
- **Where:** `71-…-1080.png` against `77-editor-grid-empty-1080.png`
- **Kind:** alignment
- **Severity:** low
- **Problem:** "Sources" is at y 162 with the follow-the-router note above it and at y 146 without. The note is one 11 px line the full width of both lists.
- **Change:** move the note under each heading as that list's hint, and shorten it: `Follows the router, in router order. Pins and artwork still apply.` (the Panel tab switch is already named "Sources follow the router").

### C2-32: Disabled list tools are a wall of faint icons
- **Where:** Follow-the-router lists; `71-…-crop-lists-4k2x.png`
- **Kind:** readability
- **Severity:** medium
- **Problem:** With follow on, every row shows three disabled icons (up, down, remove) at low contrast: 2900 rows of dead controls. Only pin works.
- **Change:** `editorgridlist.js`: when the edge follows the router, do not make the up, down and remove buttons (keep their columns empty so pin stays in place) and hide the toolbar's move and remove groups.

### C2-33: Empty Grid editor text
- **Where:** `77-editor-grid-empty-1080.png`, `73-…-crop-side-4k2x.png`; `editorgridlist.js:78`, `editorcell.js:145`, `editorrouterlist.js:25`
- **Kind:** text
- **Severity:** low
- **Change:** `Add from the router, or turn on Follow the router.` → `Add names from the list below, or turn on "Sources follow the router" in the Panel tab.` (and Destinations); `Select sources or destinations in the lists to set their artwork.` → `Select a source or destination in a list to set its artwork.`; placeholder `Find: name, 12 or 101-140` → `Name, number or range (101-140)`. The Cell tab on a Grid panel should be named `Artwork`, since a Grid has no cells (GLOSSARY: "A Grid panel has crosspoints, not cells").

---

## /panels/<id> Buttons panel

### C2-34: Text on armed, staged and pending cells fails contrast
- **Where:** `91-panel-buttons-staged-two-1080.png`, `173-…`, `90-panel-buttons-staged-dark.png`
- **Kind:** readability
- **Severity:** high
- **Problem:** These states set `color: var(--pb-text-strong)` on a fill that pulses up to the full state colour. Measured at the pulse peak:
  - light: armed Take rgb(20,20,20) on rgb(115,61,0) = 2.07:1; pending source on rgb(20,101,60) = 2.96:1; staged destination on rgb(194,67,16) = 4.10:1.
  - dark: armed Take white on rgb(239,182,75) = 1.83:1; pending source white on rgb(92,205,142) = 1.99:1; staged destination 3.32:1.
  "Take" is the one word that must read at the moment of the take.
- **Change:** `web/panels.css:205-217` and `:249-256`: replace `color: var(--pb-text-strong);` with `color: var(--pb-text-on-accent);` in `[data-s="staged"]`, `[data-s="pending"]` and `.sode-panel__take--armed` (white on these fills in light is 4.7 to 8.6:1; the dark token is dark text, about 10:1 on the amber).
- **Also check:** the Grid page's `.sode-gridpanel__take.sode-panel__take--armed` (`gridpanel.css:962`) already has light text in light theme; keep the two in step.

### C2-35: A failed take shows a 14 px cross for 2.1 seconds
- **Where:** fault panel, destination 7 (no confirmation); my capture `C2work/fault-failed.png`; `panels.css:229`, `panelview.js` (statusIcon)
- **Kind:** readability
- **Severity:** high
- **Problem:** After 4.1 s the cell gets `data-result="failed"`: a red inset ring on an orange cell and a small "x" after the name. It shows for 2137 ms, then the cell is plain "staged" again and Take is armed again. No words, no trace. The icon sits in the name's line, so the name moves about 9 px left of the lines under it. The icon has `aria-hidden="true"` and `aria-label="Failed"` at once, so a screen reader gets nothing.
- **Change:** keep the failed mark until the next press on that cell or on Take. Put the mark on its own line: `<small class="sode-panel__result">Failed: no answer from the router</small>` (icon `x` plus text; panel pages may show icon with optional text, and a failure deserves the text). Ring: `box-shadow: inset 0 0 0 var(--pb-space-2) var(--pb-status-error), inset 0 0 0 calc(var(--pb-space-2) + var(--pb-border-w)) var(--pb-surface-base);` so red separates from orange. Remove `aria-hidden` from the icon.
- **Also check:** Protected refusal (`96-…`) shows the same ring with no reason.

### C2-36: Title is 175 px off the grid edge and 8 px below the user chip
- **Where:** every Buttons panel page with a header; `87-panel-buttons-idle-1080.png`
- **Kind:** alignment
- **Severity:** high
- **Problem:** The grid starts at x 180. The title starts at x 355, because the header adds `padding-inline-start: calc(var(--sode-who-w) + var(--pb-space-4))` (`panels.css:30-32`) to clear the user chip, though at 1920 the chip (x 12 to 179) is outside the centred 1600 px column. The chip's text centre is y 28, the title's is y 36.
- **Change:** `panels.css:31`: `padding-inline-start: max(0px, calc(var(--sode-who-w, 0px) + var(--pb-space-4) + var(--pb-space-5) - max(var(--pb-space-8), (100vw - 100rem) / 2 + var(--pb-space-8))));` so the pad is only what the chip overlaps. Align tops: give `.sode-panels__head:not(.sode-bar)` `min-height: calc(var(--pb-control-h-sm) + var(--pb-space-2) * 2)` and set `.sode-panels` top padding to `var(--pb-space-4)` on panel pages, the chip's top.
- **Also check:** the device-name row (`103-…`) starts at x 180 while the title above it is at 355.

### C2-37: Panel is capped at 1600 px on a 4K screen
- **Where:** `87-panel-buttons-idle-4k.png`
- **Kind:** readability (4K)
- **Severity:** high (at 4K)
- **Problem:** On 3840x2160 the 8x4 grid is 1950 px wide and 975 px high: 26% of the screen, with 11 px names. A wall panel is the use this page is for. The fit needs the "Full width" header button, which a kiosk hides.
- **Change:** make full width the default for a panel page: `.sode-panels:has(> .sode-panel__grid:not(.sode-gridpanel)) { max-width: none; }` and size cells from the viewport (`grid-auto-rows: minmax(0, calc((100dvh - 8rem) / var(--rows)))`), with cell text in `cqmin` units as the artwork badge already does (`panels.css:442`). Keep the header button as "Narrow".

### C2-38: Cells are drawn three ways
- **Where:** bottom row; my `C2work/pb-row.png`
- **Kind:** design-language
- **Severity:** medium
- **Problem:** Source and Destination cells have a double frame. Next Page, Clear and Undo have a 1 px inset line 2 px in. Idle Take and the disabled Main and Info cells have no line, so Take looks 4 px larger than Clear beside it. Disabled cells are at 55% opacity, which also fades the Info cell that is only a label.
- **Change:** `panels.css:260` `.sode-panel__take:disabled`: add `box-shadow: inset 0 0 0 var(--pb-border-w) var(--pb-edge);`. Add `sode-panel__cell--info`, `--pageName` to the rule at `:232` that keeps label cells at `opacity: 1` with `color: var(--pb-text-muted)`.

### C2-39: Instance name prompt on a public kiosk
- **Where:** `111-panel-buttons-public-kiosk-1080.png`, `103-…`
- **Kind:** text / alignment
- **Severity:** medium
- **Problem:** A bare "Instance name" field with placeholder "Lobby tablet" and a blue Save spans the page above the grid. A person at a lobby screen does not know the word "instance". The row pushes the grid down 66 px, then the grid jumps up on Save.
- **Change:** label → `Name of this screen (shown on the panels page)`; placeholder `Lobby tablet` stays; add `Not now` beside Save. Limit the field to `max-width: 24rem`. Reserve nothing: show it as a `pb-banner` over the grid's top edge so the grid does not move.

### C2-40: Staged cell text
- **Where:** `90-…-crop-staged-cells-4k2x.png`
- **Kind:** readability
- **Severity:** low
- **Problem:** "DSM1 / Zoom 1 / → CAM 1": the present source is 85% opacity 11 px, the staged one bold. Nothing says which is now and which is next except the arrow.
- **Change:** keep the layout; set the present source line to `text-decoration: line-through` while staged, and the staged line to `var(--pb-text-sm)`.

### C2-41: Router down shows nothing on the panel (screenshots only)
- **Where:** `166-panel-buttons-router-down-1080.png`, `167-panel-grid-router-down-1080.png`
- **Kind:** edge case
- **Severity:** medium (not reproduced)
- **Problem:** With Router B's simulator stopped, the kiosk panel looks idle: names, no mark, Take ready. The Grid one shows the sign-in pad. I could not tell if the panel's cells were on Router B in that scene.
- **Change:** confirm with a panel whose cells are on the stopped router. If cells stay live, mark them: the `sode-panel__status-mark` icon (`panels.css:173`) plus `aria-disabled`, and a foot line `Router B is offline.`

---

## /panels/<id> Grid panel

### C2-42: A staged crosspoint is almost invisible
- **Where:** my `C2work/grid-two.png` (two staged, pointer away); `120-panel-grid-staged-two-1080.png`
- **Kind:** readability
- **Severity:** high
- **Problem:** The only bold ring in the screenshots is the cursor. A staged crosspoint away from the cursor shows a thin arc: the stage ring is "a pie of the staged levels" (`gridpanel.css:478-486`), so levels 1-2 of 17 give 2/17 of a ring, at `opacity: calc(0.25 + var(--pulse) * 0.75)`. On white it nearly vanishes. The prototype's staged mark is a full 4 px ring in `--pb-text-strong` with a 12% wash (`prototype/panels/grid/grid.css:114`, `.ga-pt.ga-ghost::after`).
- **Change:** `gridpanel.css` after line 489: `.sode-gridpanel__xp[data-s="staged"]::before { border: calc(var(--pb-border-w) * 4) solid var(--pb-text-strong); border-radius: 50%; background: color-mix(in oklab, var(--pb-text-strong) 12%, transparent); content: ""; grid-area: 1 / 1; height: 80%; width: 80%; }` and keep the level pie inside it at full opacity (`opacity: 1` on `.sode-gridpanel__stage`, scale the pulse instead). Give the staged row and column the prototype's band (`ga-band-r`, `ga-band-c`: 30% role colour mixed into the field).

### C2-43: Number and name touch in the Destination bar
- **Where:** `117-panel-grid-idle-1080.png`: "A.1PGM", "A.8Zoom Return"
- **Kind:** spacing
- **Severity:** high
- **Problem:** The number is a `::before` with `margin-inline-end: var(--pb-space-1)`, 1.8 px at this root (`gridpanel.css:335-342`). It reads as one word. Rows are centred, so the numbers start at x 32 to 57 and form no column.
- **Change:** line 341: `margin-inline-end: var(--pb-space-3);`. For the column: `:root:not([data-layout="phone"]) .sode-gridpanel__dst { justify-content: flex-start; padding-inline-start: var(--pb-space-4); }` and `.sode-gridpanel__dst[data-num]::before { min-width: calc(var(--sode-num-ch, 5) * 1ch); text-align: end; }`.

### C2-44: Grid is smaller than the prototype everywhere
- **Where:** all Grid screenshots against `C2work/protoA.png`
- **Kind:** design-language (owner's rule: match the prototype)
- **Severity:** high
- **Problem:** In rem (prototype root 16 px, page root 14.4 px):
  | Part | Prototype | Now |
  |---|---|---|
  | Crosspoint | 4.5rem (72 px) | 4rem (57.6 px) |
  | Destination bar width | 13rem (208) | 11.2rem (161.3) |
  | Source bar height | 7rem (112) | 6rem (86.4) |
  | Level pick | 2.6rem (41.6), 17 px mono | 2.125rem (30.6), 12.6 px sans |
  | Take | 12rem x 4rem, 22 px | 7rem x 4.4rem, 15.3 px |
  | Find boxes | 13 px text | 9.9 px text |
  | Labels (LEVELS, SOURCES, Select All, staged chip) | 11 to 13 px | 9.9 px |
  The comment at `gridpanel.css:463` still says "48 px in a 72 px crosspoint". 9.9 px text is under any readable size on a desk monitor.
- **Change:** `gridpanel.css:9-11`: `--sode-xp: 4.5rem; --sode-name-w: 13rem; --sode-name-h: 7rem;` (at this root 64.8, 187 and 100.8 px). `gridpanel.css:297` find input: `font-size: var(--pb-text-sm)`. Deck columns (`:1145`, `:1332`): `… auto 5rem 7rem` → `… auto 5rem 12rem`. Take `font-size: var(--pb-text-xl)`. Set every `--pb-text-xs` in the deck to `--pb-text-sm`, and the level picks to `--sode-pick: 2.6rem` with `font-family: var(--pb-font-num)`. Check 2500 x 750 speed again after it (fewer nodes on screen, so it should not be slower).

### C2-45: Deck rules have unequal gaps and the stacks split at different heights
- **Where:** toolbar band; `119-panel-grid-staged-crop-header-4k2x.png`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Rule 1 and rule 2 have 9 px on each side. Rule 3 has 3.6 px (`gridpanel.css:1335` sets `margin: 0`). The Levels input and Select All split at y 109.4/111.3; the two Find boxes beside them split at 107.7/109.5, so the seam steps by 1.7 px. The Find/Filter switch sits in its own bordered grey box (114 x 63) that looks like a button.
- **Change:** delete lines 1335-1337. Make the input rows equal: `.sode__input` height `var(--sode-pick)` and `--sode-all-h: var(--sode-pick)` so both stacks are two rows of one height with `row-gap: var(--pb-space-1)`. Remove the box from the switch (`:1310-1318`: `background: none; border: 0;`).

### C2-46: Find box shrinks when it has matches
- **Where:** `129-panel-grid-filter-on-1080.png`
- **Kind:** alignment
- **Severity:** medium
- **Problem:** With "CAM 1" typed, the Sources box is 155 px wide (x 452 to 607) and two step arrows take the rest; the Destinations box under it stays 201.6 px. The right edges no longer agree.
- **Change:** `gridpanel.css:705` `.sode-gridpanel__find-step`: place the steps inside the field (`position: absolute; inset-inline-end: var(--pb-space-6);`) beside the count, so the input keeps its width.
- **Also check:** "CAM 1" also marks CAM 10, 11, 12. Count text "1 of 12" is right, but a person who types an exact name expects one match first. Sort exact matches first.

### C2-47: "Find" overlaps its switch at 200%
- **Where:** my `C2work/z-grid.png` (960 px)
- **Kind:** alignment
- **Severity:** medium (at 200%)
- **Problem:** The bold "Find" word is drawn over the left edge of the switch box; the mode column is `auto` but the label is positioned by `::after` width tricks (`gridpanel.css:730-745`).
- **Change:** give `.sode-gridpanel__find-mode` `min-width: max-content` and remove `overflow: visible` on the side words; at under 64rem hide the words and keep the switch with `data-pb-tip`.

### C2-48: Result banner text is 9.9 px in a 63 px box
- **Where:** `122-panel-grid-taken-crop-banner-4k2x.png`
- **Kind:** readability
- **Severity:** medium
- **Problem:** "Taken: 1 route" is the smallest text on the page inside its largest free box (935 x 63).
- **Change:** `gridpanel.css:1357` add `font-size: var(--pb-text-md);` to `.sode-gridpanel__res .pb-alert`; text → `1 route taken`. For more routes: `3 routes taken`.

### C2-49: `.sode-gridpanel__held` is one class for two things
- **Where:** `gridpanel.css:380` (the "staged" word in a Destination name) and `:1366` (the count of held messages in the banner box)
- **Kind:** bug / rules that fight
- **Severity:** medium
- **Problem:** The second rule gives the row word `align-self: end; justify-self: end; margin: var(--pb-space-1); z-index: 1`. The word "staged" is also lower-case, orange and 9.9 px, the only lower-case status on the page.
- **Change:** rename the banner one to `.sode-gridpanel__msg-held` (CSS `:1366-1375` and `gridpanelheadview.js`). Row word: `gridpanelnodes.js:180` `'staged'` → `'Staged'`; `gridpanel.css:380` `font-size: var(--pb-text-sm); font-weight: var(--pb-weight-bold);`.

### C2-50: Empty Grid panel keeps the corner labels and all controls
- **Where:** `139-panel-grid-empty-1080.png`; `gridpanelhead.js:8`
- **Kind:** edge case / text
- **Severity:** low
- **Problem:** "SOURCES" and "DESTINATIONS" tabs float over nothing; Levels, Find and Take are all live. The message is centred at y 295 on a 930 px area.
- **Change:** hide `.sode-gridpanel__head` and disable the deck while empty. Text → `This panel has no sources or destinations.` with the button `Edit panel`; for a viewer who cannot edit: `Ask ${owner} to add them.`

### C2-51: Protected rows blend into the page
- **Where:** `117-…` (row A.13), `176-panel-grid-fault-protected-1080.png` (rows C.13, C.21), `138-…` (row B.13)
- **Kind:** readability
- **Severity:** medium
- **Problem:** A protected row's crosspoints use `--pb-surface-base` (`gridpanel.css:495`), the page colour. In a small grid (scene 176) the rows look like holes in the grid, and in scene 138 the last row looks like it is outside the grid. The pinned-column rule runs on through it.
- **Change:** `gridpanel.css:494-497`: `background: color-mix(in oklab, var(--pb-text) 8%, var(--sode-patch-field));` and keep the grid lines on it.

### C2-52: Take confirm box
- **Where:** `121-panel-grid-take-confirm-1080.png`
- **Kind:** text / readability
- **Severity:** low
- **Problem:** "Take 2 sources to 2 destinations: A.5 DSM3 and A.4 DSM2?" then each route shows the full 17-level picker twice (once for the new source, once for "REPLACING"), 9 px digits. "Press [F] to confirm, or escape to cancel." The destinations are listed in reverse of their numbers.
- **Change:** list by destination number. Lead: `Take 2 routes?` Per route: `A.4 DSM2: A.12 Zoom 2 replaces A.12 Zoom 2 on levels 1-2` (levels as text; show the picker only when levels differ between routes). Foot: `Press F to take. Press Esc to cancel.`

### C2-53: Keyboard list wording
- **Where:** `180-panel-grid-keys-dialog-1080.png`
- **Kind:** text
- **Severity:** low
- **Change:** `The corner` → `Go to the first source and destination`; `Stage the route, or the Diagonal of the picks. Staged already: Take` → `Stage the route (or the Diagonal of the picks). Press again to take.`; `Remove the line, then the picks, then the staged routes` → `Remove the Drawn line. Press again to remove the picks, then the staged routes.`; `Home. Not on a Kiosk panel` → `Go to Sode (not on a Kiosk panel)`. The key column is 228 px and the text starts at x 771; set the column to `max-content` so short rows do not leave a 150 px gap.

---

## Shared causes

1. **`.pb-tabpanel.active { display: block }` beats Sode's flex containers** (C2-13, C2-23): one selector in `panels.css` restores the gaps in Share, Link with key, Push and the whole editor side panel.
2. **`color: var(--pb-text-strong)` on pulsing state fills** (C2-34): switch the three state rules to `--pb-text-on-accent`; fixes light and dark.
3. **Footer built as `[note, button…]`** (C2-14): put the note between the buttons in `panelshare.js`, `panelpush.js`, `panelassign.js`, `editortemplates.js`.
4. **Grid size variables below the prototype** (C2-44, C2-48, C2-49 text sizes): three variables at `gridpanel.css:9-11` plus `--pb-text-xs` → `--pb-text-sm` in the deck.
5. **Rows sized by content, not by a shared grid** (C2-02, C2-07, C2-16, C2-29, C2-30, C2-43): give each list one column template (number column in `ch`, fixed action columns, stable scroll gutter).
6. **Header clears the user chip even when the chip is outside the column** (C2-36): one `max()` in `panels.css:31`.
7. **Fixed two-column grids with `minmax(0, …)`** (C2-01, C2-26, C2-47): let rows wrap at 200%.
8. **Bootstrap element defaults leaking in** (C2-21, C2-25): reset `legend`, `ul` and `th` inside `.sode-ed__side`.

---

Summary
- 53 entries: 14 high, 25 medium, 14 low (C2-01, C2-37 and C2-47 are high or medium only at 200% or 4K).
- Worst 1: a staged crosspoint on the Grid is a faint partial arc, and "Take" on an armed Buttons panel is 2.07:1 (light) and 1.83:1 (dark) at the pulse peak; a failed take shows a 14 px cross for 2.1 s (C2-42, C2-34, C2-35).
- Worst 2: one patchbay rule (`.pb-tabpanel.active`) removes every gap in Share, Link with key, Push and the editor side panel, and dialog footers put Cancel in three places (C2-13, C2-14).
- Worst 3: the Grid is 11 to 40% smaller than the prototype with 9.9 px text, "A.1PGM" has a 1.8 px gap, and the Buttons title sits 175 px off the grid edge (C2-44, C2-43, C2-36).
- Measuring scripts and captures are in `/private/tmp/claude-501/-Users-seth-Desktop-sode--claude-worktrees-happy-branch-merge-review-0c348b/1b1c74a8-cf3d-4546-aa99-2582e71b4297/scratchpad/review/C2work`; all servers I started on 46010 to 46090 are stopped.