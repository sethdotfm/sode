# Findings C1

Scope: Communication Log `/log` (all states), the phone layout of the main page, Settings and `/log`, the server-down shots 900 to 904, and the non-page routes.
Code read at `/Users/seth/Desktop/sode/.claude/worktrees/review-shots` (origin/alpha-round2, 8e0969a7).

How the numbers were made: I copied the seeded data to `/private/tmp/claude-501/-Users-seth-Desktop-sode--claude-worktrees-happy-branch-merge-review-0c348b/1b1c74a8-cf3d-4546-aa99-2582e71b4297/scratchpad/review/C1work`, booted three simulators (45010 to 45012) and Sode (45080), and read `getBoundingClientRect` and text Range boxes in headless Chrome at 1920x1080 DPR 1 and at 375x812 DPR 3 (touch, mobile). All px values are at 1080 unless a line says phone. Root font is 16px in both. Token values used below: `--pb-space-2` 4px, `--pb-space-3` 6px, `--pb-space-4` 8px, `--pb-space-6` 14px, `--pb-space-7` 16px, `--pb-space-8` 20px, `--pb-border-w` 2px, `--pb-accent-bar-w` 4px (computed). Scripts and raw output are in that folder (`m1.mjs` to `m3.mjs`, `p1.mjs` to `p3.mjs`, `d1.mjs`, `m1-1080.json`). Nothing in the repo was changed. The servers I started are stopped.

Two fixes were tried live with an injected style and measured: C1-01 and C1-06. The other changes are computed from the CSS and not run.

Counts: 5 high, 14 medium, 7 low.

---

### C1-01: The Log table is wider than the page, so the Bytes column is cut off
- **Status:** Done in 4332de4a.
- **Where:** `/log`, every width. `01-log-default-crop-rows-4k2x.png`, `06-log-uptime-bar-picked-1080.png` (Bytes reads `82 00 02 00 (`), `01-log-default-4k.png`, `25-log-phone-phone3x.png`.
- **Kind:** alignment
- **Severity:** high
- **Problem:** `.sode-log__table td { white-space: nowrap }` (log.css:75) lets one long note set the width of the For column. Default kinds: table 1586px in a 1560px scroller, 26px cut. Show All: 1647px, 87px cut. The widest For cell was 819px ("Panel instance lost … missed its keepalive for 8s; no device is online"). The horizontal big scroll bar is at the bottom of the table (y 2790 on a 1080 screen), so the user sees cut hex bytes and no bar.
- **Change:** Let only the For cell wrap. `web/log.js:364-366`, current `el('td', null, why, …` becomes `el('td', 'sode-log__why', why, …`. Add to `web/log.css` after line 77:
  ```css
  .sode-log__table td.sode-log__why {
    min-width: 16rem;
    overflow-wrap: anywhere;
    white-space: normal;
  }
  ```
  Measured with this rule injected (as `td:nth-child(6)`): scroller 1560 of 1560, For column 744px, 999 of 1000 rows stay 22px, one row wraps to 41px.
- **Also check:** C1-07 (column widths). The phone needs more than this (C1-05).

### C1-02: With the Server link down, /log shows only a red icon
- **Status:** Done in 96dc1eb3.
- **Where:** `/log`, `/panels`, `/panels/editor` with Sode stopped. `902-log-server-down-1080.png`, `903-panels-server-down-1080.png`, `904-editor-server-down-1080.png`, against `900-…`, `901-…` and the main page.
- **Kind:** design-language
- **Severity:** high
- **Problem:** One state is drawn four ways. Main page: red frame and the banner "Lost link: Server link down". Panel pages: a cover that says "Offline. No contact with Sode. Presses do nothing." Editor: an 11px grey line "Sode is not answering. Reconnecting…". `/log` and `/panels`: nothing but the Server icon turned red, and the word beside it still reads "Server". Measured on `/log` 14 s after the stop: `--sode-down-h` is empty, no element with role alert or status shows, Load older and both Download buttons stay enabled, and the uptime summary stays as it was. On the phone `/log` the bar has no words at all, so a red icon is the only sign. This breaks "status always with text". `log.css:11` already keeps room for the banner (`--sode-down-h`), but no code draws one.
- **Change:** Draw the same banner on every page that uses the shared bar. `web/panelsbar.js:12-14`, current:
  ```js
  on('conn', () => {
    for (const c of conns) paintConn(c);
  ```
  New (import `connState`, `renderDown` and `$` from core.js):
  ```js
  on('conn', () => {
    for (const c of conns) paintConn(c);
    const box = $('conn-down');
    if (box) {
      const c = connState();
      document.documentElement.dataset.conn = c.down ? 'down' : 'up';
      renderDown(box, c);
    }
  ```
  Add `<div class="sode__down" id="conn-down" role="alert" hidden></div>` as the first child of `<body>` in `web/log.html` (before line 20), and the same line in the `/panels` and editor HTML. `.sode__down` is in base.css:192, which these pages load. In `web/log.js:490-493` also set `$('log-download-rows').disabled = $('log-download-full').disabled = S.wsState !== 'up'` and `$('log-older').disabled` the same way. Replace the editor's line (`editorrules.js:236`) with the banner, so one wording is left: "Lost link: Server link down".
- **Also check:** The panel pages' "Offline" cover is a different wording for the same state. That is the other reviewer's slice, so I only note it.

### C1-03: The uptime bars add 288 Tab stops before the filters
- **Status:** Done in 86aa9c37.
- **Where:** `/log` with 3 routers, 24 hours range. Not visible in a still shot.
- **Kind:** bug
- **Severity:** high
- **Problem:** Each bar is a `<button>` (uptimebars.js:9). I pressed Tab from the top: stops 9 to 296 are bars (96 bars for each of 3 routers). A keyboard user must press Tab 288 times to get from the range chips to Show. At 7 days it is 504.
- **Change:** One Tab stop for each strip, arrow keys inside it. `web/uptimebars.js`, after line 17 add `bar.tabIndex = -1;`. After the map, make the selected bar (else the last enabled bar) `tabIndex = 0`, and add one `keydown` listener in `log.js` on each `.sode-up__bars`: ArrowLeft and ArrowRight move focus to the previous or next enabled bar and swap the `tabIndex` values, Home and End go to the first and last.
- **Also check:** The header cells are also Tab stops (8). That is correct.

### C1-04: The four control rows start at four different x positions
- **Status:** Done in bebd1739.
- **Where:** `/log` filters. `01-log-default-1080.png`, `01-log-default-crop-filters-4k2x.png`.
- **Kind:** alignment
- **Severity:** high
- **Problem:** First control of each row: range chips x 280.2, Show chips x 212.1, Routers chips x 231.3, From field x 208.1. The gap from label to control is 14px, 8px, 8px and 6px. Cause: each label has its own width (97, 35, 54, 33px) and three different gap rules (`.sode-log__line` gap, legend margin plus fieldset gap, `.sode-log__when` gap).
- **Change:** One label column. `web/log.css`: delete lines 48-50 (a second `.sode-log__kinds > legend` rule that only adds `margin-right`). Add:
  ```css
  .sode-log {
    --sode-log-label-w: 6.5rem; /* "Router uptime", the widest label, is 97px */
  }
  .sode-log__uptime > .sode-log__line > h2,
  .sode-log__kinds > legend,
  .sode-log__when:first-child > .sode__label {
    flex: none;
    width: var(--sode-log-label-w);
  }
  .sode-log__kinds > legend {
    margin-right: calc(var(--pb-space-6) - var(--pb-space-2)); /* the fieldset gap adds --pb-space-2 */
  }
  .sode-log__when:first-child {
    gap: var(--pb-space-6);
  }
  ```
  Computed result: all four rows start their first control at x 286.8 (168.8 + 104 + 14). Put the Source and Destination fields under the same edge, or leave them at the page edge as the only full-width row.
- **Also check:** The router head lines ("ROUTER A 98.0% up") can keep their own flow, since their label is a name.

### C1-05: /log has no phone layout
- **Status:** Done in 86aa9c37.
- **Where:** `/log` at 375x812. `25-log-phone-phone3x.png`.
- **Kind:** alignment
- **Severity:** high
- **Problem:** `log.css` has no rule for `data-pb-layout="phone"`. Measured:
  - Title: 101px wide, shows "Sode C…". The page name cannot be read.
  - From field x 59.3 to 324.3. To field x 41.6 to 306.6. Left and right edges 17.7px apart.
  - Source and Destination fields end at x 276. The date fields end at 324 and 307. The page edge is 355. Three right edges.
  - "Download filtered rows" is right-aligned on the count row. "Download full log" wraps to the left of the next row.
  - Touch heights: chips 22px, Download buttons 24px, fields 30px.
  - The `|` between "Settings saves" and "Problems only" ends a row alone.
  - Table: 1586px wide in a 335px scroller. The Time column alone is 164px.
  - Bars: see C1-11.
- **Change:** Add to `web/log.css`:
  ```css
  [data-pb-layout="phone"] .sode-log__head { flex-wrap: wrap; }
  [data-pb-layout="phone"] .sode-log__title { flex: 1 0 100%; order: -1; }
  [data-pb-layout="phone"] .sode-log__when { flex: 1 0 100%; }
  [data-pb-layout="phone"] .sode-log__when > .sode__label { flex: none; width: 3rem; }
  [data-pb-layout="phone"] .sode-log__when > .pb-input,
  [data-pb-layout="phone"] .sode-log__expr { flex: 1 1 auto; max-width: none; width: auto; }
  [data-pb-layout="phone"] .sode-log__expr { flex-basis: 100%; }
  [data-pb-layout="phone"] .sode-log__chip,
  [data-pb-layout="phone"] .sode-log__filters .pb-btn { min-height: var(--pb-control-h); }
  [data-pb-layout="phone"] #log-download-rows,
  [data-pb-layout="phone"] #log-download-full { flex: 1 1 0; }
  [data-pb-layout="phone"] .sode-log__sep { display: none; }
  [data-pb-layout="phone"] .sode-log__table .sode__date,
  [data-pb-layout="phone"] .sode-log__table :is(th, td):nth-child(8) { display: none; }
  ```
  The last rule drops the date and the Bytes column on the phone. With C1-01 the For cell wraps, so the table gets much narrower. It still scrolls sideways with the native bar, which is the design.
- **Also check:** `/panels` on the phone uses the same bar. The other reviewer has it.

### C1-06: Rows with a level chip are 2px taller than the others
- **Status:** Done in 9d47aeca.
- **Where:** `/log` rows. `01-log-default-crop-rows-4k2x.png`, `07-log-all-kinds-1080.png`.
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Rows without a chip are 22px. Rows with a chip are 24px (507 of 1000 rows with Show All). The chip is `inline-flex`, 22px high, on the text baseline of a 19.5px line, so it pushes the row. The row pitch changes all the way down the table.
- **Change:** `web/log.css:94-97`, current:
  ```css
  .sode-log__lvl {
    min-width: calc(var(--pb-space-8) + var(--pb-space-1));
    padding: 0 var(--pb-space-2);
  }
  ```
  New:
  ```css
  .sode-log__lvl {
    height: var(--pb-space-7);
    min-width: calc(var(--pb-space-8) + var(--pb-space-1));
    padding: 0 var(--pb-space-2);
    vertical-align: middle;
  }
  ```
  Measured with this injected: every one-line row is 22px.
- **Also check:** none.

### C1-07: Column widths change with the rows on show
- **Status:** Done in 89ffb825 (by the measured Reason width).
- **Where:** `/log`. Compare `01-log-default-1080.png` (Time 164px), `08-log-problems-only-1080.png` (Time 300px, one row) and my server-down shot (Time 341px).
- **Kind:** alignment
- **Severity:** medium
- **Problem:** The table uses automatic layout, so each filter change, each Load older and each reconnect moves every column. With few rows the spare width is shared out and the Time header's sort mark sits 280px from the word "Time".
- **Change:** `web/log.css`, add:
  ```css
  .sode-log__table { table-layout: fixed; width: 100%; }
  .sode-log__table th:nth-child(1) { width: 10.5rem; }
  .sode-log__table th:nth-child(2) { width: 10rem; }
  .sode-log__table th:nth-child(3) { width: 3.75rem; }
  .sode-log__table th:nth-child(4),
  .sode-log__table th:nth-child(5) { width: 9rem; }
  .sode-log__table th:nth-child(7) { width: 11rem; }
  .sode-log__table th:nth-child(8) { width: 9.5rem; }
  .sode-log__table td { overflow: hidden; text-overflow: ellipsis; }
  ```
  The widths are the measured content widths with Show All (164, 154, 58, 111, 87, 98, 144px) rounded up. Not run. Check a row with "NAK, gave up after 3 tries" and a message longer than 7 bytes before you keep the last two widths.
- **Also check:** `.sode-log__name { max-width: 16rem }` (log.css:86-90) has no effect in a fixed table and can go.

### C1-08: The picked bar loses its color
- **Status:** Done in 86aa9c37.
- **Where:** `/log` after a click on a bar. `06-log-uptime-bar-picked-1080.png`, my `C1work/sel.png`.
- **Kind:** readability
- **Severity:** medium
- **Problem:** `.sode-up__bar--sel` draws an inset ring of `calc(var(--pb-border-w) * 3)` = 6px on each side (base.css:2135-2138). A 24 hours bar is 15.3px wide, so 3px of its color is left. A 7 days bar is 8.3px wide, so none is left: the picked bar is a black block and its Up, Partial or Down state is gone.
- **Change:** `web/base.css:2135-2138`, current:
  ```css
  .sode-up__bar--sel,
  .sode-up__bar--sel:hover {
    box-shadow: inset 0 0 0 calc(var(--pb-border-w) * 3) var(--pb-text-strong);
  }
  ```
  New:
  ```css
  .sode-up__bar--sel,
  .sode-up__bar--sel:hover {
    outline: var(--pb-border-w) solid var(--pb-text-strong);
    outline-offset: var(--pb-border-w);
    position: relative;
    z-index: 1;
  }
  ```
  The ring is then outside the bar and the bar keeps its whole color.
- **Also check:** In slot mode no range chip is pressed (measured: all four `aria-pressed="false"`), but the strip still shows the old range. Keep the chip of the strip's range pressed: `log.js:564`, use `slotMode?.prev.chip` before the default.

### C1-09: "No data" bars and their legend mark cannot be seen
- **Status:** Done in 86aa9c37.
- **Where:** `/log` uptime strip and legend, light and dark. `01-log-default-crop-uptime-4k2x.png`, `01-log-default-dark.png`.
- **Kind:** readability
- **Severity:** medium
- **Problem:** `.sode-up__bar--none` is `--pb-surface-control`, `rgb(220,220,220)`, on a page of `rgb(228,228,228)`. That is a contrast of about 1.07 to 1. The legend's "No data" square looks empty.
- **Change:** `web/base.css:2148-2150`, current `background: var(--pb-surface-control);` becomes:
  ```css
  .sode-up__bar--none {
    background: transparent;
    box-shadow: inset 0 0 0 var(--pb-border-w) var(--pb-edge-soft);
  }
  ```
  `.sode-up__bar--future` (2151-2154) uses the same ring in `--pb-surface-control`. Give it `border-bottom`-only or leave it blank, so "No data" and "not yet" still differ.
- **Also check:** The small strip on `/panels` uses the same class.

### C1-10: The bar tooltip covers the next router and runs off the edge
- **Status:** Done in 86aa9c37.
- **Where:** `/log`, hover on a bar. `05-log-uptime-bar-tip-1080.png`, my `C1work/tip.png` and `tip-first.png`.
- **Kind:** readability
- **Severity:** medium
- **Problem:** The tip is centred under the bar. On the newest bar it reaches x 1900 and its shadow goes under the page scroll bar (x 1897.6). On the oldest bar it starts at x 42, outside the content edge (168.8), and covers the next router's name and summary. A longer text ("…, 3 messages failed or needed a retry, 2 slow ACKs") is cut by the window. The text also says the date twice and uses an en dash where the Slot line uses "to": "2026-11-27 21:56 – 2026-11-27 22:11: router up 87.4% of the time".
- **Change:** `web/uptimebars.js`, in the map (line 8) take the index and the count: bars in the last third get `bar.classList.add('pb-tip--end')`, bars in the first third `pb-tip--start` (patchbay.css:1998-1999), and all get `pb-tip--top` (patchbay.css:1993) so the tip opens over the strip and not over the next router. `web/uptime.js:97`, current `` const when = `${fmt(s.from)} – ${fmt(s.to)}`; `` becomes:
  ```js
  const a = fmt(s.from), b = fmt(s.to);
  const when = `${a} to ${a.slice(0, 10) === b.slice(0, 10) ? b.slice(11) : b}`;
  ```
  Result: "2026-11-27 21:56 to 22:11: router up 87.4% of the time".
- **Also check:** Tips do not show on touch (touch.css:10-16), so on the phone a bar can only be read by a tap that filters the log.

### C1-11: On the phone each bar is 2.5px wide
- **Status:** Done in 86aa9c37.
- **Where:** `/log` phone. `25-log-phone-phone3x.png`.
- **Kind:** readability
- **Severity:** medium
- **Problem:** 96 bars in 335px: each is 2.5px with a 1px gap. The "No data" part reads as stripes, and a 2.5px button cannot be tapped on purpose.
- **Change:** Fewer, wider bars on a narrow strip. `web/uptime.js:25-28`, give `slotFor` a third argument `max = MAX_SLOTS` and use it in place of `MAX_SLOTS`. `web/log.js:588`, current `slot: slotFor(r.from, r.to)` becomes `slot: slotFor(r.from, r.to, Math.min(MAX_SLOTS, Math.floor(upBars.clientWidth / 8)))` (import `MAX_SLOTS`). At 335px that gives 41 slots at most, so 24 hours uses 1 hour bars of about 13px. The range chips then need the real slot in their label (`presetText`, uptime.js:23) or no slot at all (C1-17).
- **Also check:** `gap: 1px` in base.css:2114 is a magic px value. Use `var(--pb-border-w)` halved or a token.

### C1-12: Router names are written three ways, two of them doubled
- **Status:** Done in dc4f5fd2.
- **Where:** `/log` uptime heads and Routers chips ("Router B Router B (studio 2)"), the status hover box ("B Router B (studio 2)"), the Lost link banner on the phone ("Router Router A 127.0.0.1:45010"). `01-log-default-crop-uptime-4k2x.png`, `25-log-phone-phone3x.png`, my `C1work/down-log-hover.png` and `down-phone-main.png`.
- **Kind:** text
- **Severity:** medium
- **Problem:** `routerName` (routerlinkrules.js:118) drops the title only when it is exactly `Router <prefix>`. A title that starts with those words is doubled. The banner (core.js:615) writes the word "Router" and then the title, so the default title "Router A" gives "Router Router A". With the Server link down the banner also names only the first router, though three are configured.
- **Change:** `web/routerlinkrules.js:118`, new:
  ```js
  export const routerName = (r) => {
    const head = `Router ${r.prefix}`;
    const t = (r.title || '').trim();
    return !t || t === head ? head : t.startsWith(head) ? t : `${head} ${t}`;
  };
  ```
  This gives "Router A", "Router B (studio 2)", "Router C (MCR)". `web/core.js:614-615`, use `routerName(r)` and `routerName(FIRST)` for the node labels, and at core.js:542 add `routers: list.length > 1 ? list : undefined` so the banner shows every router. Use `routerName` in the hover box (`statusinfo.js` `makeHover`) too.
- **Also check:** The `router` column of the download files uses the prefix only. That is fine.

### C1-13: API and refused rows show code names
- **Status:** Done in 75515b1a.
- **Where:** `/log` For column. `01-log-default-1080.png` ("uptime ::1", "log ::1"), `05-log-uptime-bar-tip-1080.png` ("panelKey"), `902-log-server-down-1080.png` ("keyCreate"), `16-log-settings-save-open-1080.png` ("panelDelete", "panelCreate").
- **Kind:** text
- **Severity:** medium
- **Problem:** `WHY_LABEL` (logkinds.js:22-78) has no words for most API whys, so the row shows the wire name. The same column shows "Communication Log downloaded" one row lower.
- **Change:** Add to `WHY_LABEL` in `web/logkinds.js`:
  ```js
  uptime: 'Uptime read',
  log: 'Log read',
  read: 'Log read through the API',
  refused: 'Refused',
  keyCreate: 'API key made',
  keyRevoke: 'API key revoked',
  keyEnded: 'API key ended',
  keyExpired: 'API key expired',
  panelKey: 'Panel key made',
  panelKeyRevoke: 'Panel key revoked',
  panelCreate: 'Panel made',
  panelUpdate: 'Panel saved',
  panelCopy: 'Panel copied',
  panelDelete: 'Panel deleted',
  panelRestore: 'Panel restored',
  panelPush: 'Panel pushed',
  panelNetworks: 'Panel networks saved',
  authFailed: 'API sign-in failed',
  endedKeyUsed: 'Ended API key used',
  ```
  Then do the rest of the list in `internal/app` (`panelSet*`, `template*`, `icon*`, `artDefault*`, `myClient*`, `tieTake`, `guestApi`, `keylessScopes`, `logApiReads`, `apiLimits`, `requireHttps`, `panelSetupLink`). Add a node test that fails when a why the server writes has no label.
- **Also check:** The download CSV uses the same table, so it gets the words too.

### C1-14: The "Router" column holds the result, and "For" holds three things
- **Status:** Done in 223efbba.
- **Where:** `/log` table head. Every log shot.
- **Kind:** text
- **Severity:** medium
- **Problem:** Under "Router" the rows read "Locked", "Done", "Sent", "Failed", "Connected", "Started". Beside a Routers filter and router names, the reader expects a router name there. The header came from "the router's answer" (ACK), but most rows on show are events.
- **Change:** `web/log.html:89`, current `<th scope="col" data-sort="result">Router</th>` becomes `<th scope="col" data-sort="result">Result</th>`. `web/log.html:88`, `For` becomes `Reason`. Keep the CSV headers in `logfile.js` (`LOG_COLS`) in step if they use the same words.
- **Also check:** GLOSSARY.md has no entry for either word, so no row changes.

### C1-15: Bars have no accessible name
- **Status:** Done in 86aa9c37.
- **Where:** `/log` uptime bars. Not visible.
- **Kind:** bug
- **Severity:** medium
- **Problem:** `uptimebars.js:13` sets `aria-label` from `bar.title`, but the text is in `data-pb-tip` (line 10) and `title` is empty. Measured: `aria-label=""` on every bar. A screen reader gets 288 unnamed buttons.
- **Change:** `web/uptimebars.js:13`, current `bar.setAttribute('aria-label', bar.title);` becomes `bar.setAttribute('aria-label', bar.dataset.pbTip);`.
- **Also check:** none.

### C1-16: Text in the For column is 4px off its header when the row has no reason
- **Status:** Done in f3d74dec.
- **Where:** `/log`, Router link and Sode rows. `902-log-server-down-1080.png`, `16-log-settings-save-open-1080.png` (the addresses "127.0.0.1:38912" sit right of "panelCreate").
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Header text x 687.9. Cell text x 691.9. `.sode-log__why-part` (log.css:148-150) always has `margin-inline-start: var(--pb-space-2)`, also when it is the first thing in the cell.
- **Change:** `web/log.css:148-150`, add:
  ```css
  .sode-log__why-part:first-child {
    margin-inline-start: 0;
  }
  ```
- **Also check:** none.

### C1-17: Wording around the uptime strip
- **Status:** Done in 86aa9c37.
- **Where:** `/log`. `06-log-uptime-bar-picked-1080.png`, `20-log-download-hover-…`, my `C1work/slot.png` and `7d.png`.
- **Kind:** text
- **Severity:** medium
- **Problem:** One thing has two names: the chip tip says "Bar = 1 hour", the note says "Slot", the legend tip says "part of the slot". The chip label "7 days (1 hour)" does not say what the hour is, and its tip repeats it. The note's button "Back" reads like the browser's Back. The count "77 of 6,559 in log" does not say what the 77 are.
- **Change:**
  - `web/uptime.js:23`: `` `${p.label} (${slotLabel(…)})` `` becomes `p.label`. Chips read "1 hour", "24 hours", "7 days", "30 days".
  - `web/log.js:551`: `` `Bar = ${…}` `` becomes `` `Each bar is ${slotLabel(slotFor(0, p.span))}` ``.
  - `web/log.js:635`: `el('strong', null, 'Slot')` becomes `el('strong', null, 'One bar')`, and `log.js:632` `'Back'` becomes `'Show all bars'`.
  - `web/log.html:43`: "Down for part of the slot, or messages failed or were slow" becomes "Down for part of the bar's time, or messages failed or were slow".
  - `web/log.js:451`: `` `${shown} of ${total} in log` `` becomes `` `${shown} rows shown, ${total} in the log` ``.
- **Also check:** `#log-slot` has class `pb-note` but computes to no padding and no border, so it reads as loose text between the filters and the table. Give it `pb-note--info` or the padding of the other notes.

### C1-18: Phone main page: clear button 3px high, "Edit" alone in the middle
- **Status:** Done in af401ed7.
- **Where:** Main page, phone, top. My `C1work/p-main.png` (the owner's shots 146, 147 and 149 are one and the same view, scrolled to the end of History).
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Filter fields: y 170 to 200 (30px). The clear button beside them: y 170 to 194 (24px). Its centre is 3px above the fields' centre, because `.sode__search` uses `align-items: flex-start` (history.css:585) and touch.css raises only the fields. In the Bookmarks head, "Edit" sits at x 157.7, in the middle between the label (ends x 86) and "All presets" (starts x 267.9), tied to neither. The Bookmarks head is 32px high and the History head 30px.
- **Change:** `web/touch.css:4-6`, current selector `.sode__search-box` becomes `.sode__search` (the variable then reaches the button too), and add inside the same media rule:
  ```css
  .sode__search > [data-search-clear] {
    height: var(--sode-search-h);
    width: var(--sode-search-h);
  }
  ```
  Check that `.sode__search-box` still gets the value (history.css:607 sets it on the box, so move that line to `.sode__search`). For "Edit": in `mobile.css` add `[data-layout="phone"] .sode__presets > .sode__label { margin-inline-end: auto; }` so Edit and All presets sit together at the right.
- **Also check:** Shot 149 ("palette") shows no palette. The phone bar has no command key button, so the scene did nothing. That is a capture note, not a page fault.

### C1-19: Settings list: sections are 2px left of their topic
- **Status:** Done in 89514b4e.
- **Where:** Settings list, phone and desktop. `151-settings-phone-top-phone3x.png`.
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Topic "Preferences" has padding-left 16px (text x 32). Sections "This browser" and "My user" have `padding-inline-start: var(--pb-space-6)` = 14px (text x 30), from base.css:1368-1370. The child is 2px left of its parent. It reads as a mistake, not as an indent.
- **Change:** `web/base.css:1369`, `padding-inline-start: var(--pb-space-6);` becomes `padding-inline-start: var(--pb-space-9);` (24px). `web/base.css:1374` (the router entries), `var(--pb-space-8)` becomes `var(--pb-space-10)` (32px).
- **Also check:** On the phone the "Other settings need a computer." box starts at y 222.5, the same y where the list ends: 0px gap, while the box has 8px inside. Add `margin-top: var(--pb-space-4)` to `.sode__desktop-only` (base.css:1379). The list rows are also inset 4px (x 16 to 359) from the search field and the box (x 12 to 363).

### C1-20: "View Logs" names the Communication Log a second way
- **Status:** Done in 0af0fa68.
- **Where:** Settings foot, phone and desktop. `151-settings-phone-top-phone3x.png`.
- **Kind:** text
- **Severity:** low
- **Problem:** `about.js:19` labels the link "View Logs" (title case, plural). The same link in Settings is "Communication Log" (index.html:435), which is the GLOSSARY term. Next to it, "Report a bug" is sentence case.
- **Change:** `web/about.js:19` and `:24`, `'View Logs'` becomes `'Communication Log'`. Keep the external-link icon.
- **Also check:** `settingsindex.js:361` finds the button by id, so search still works.

### C1-21: Numbers in the summary and the count use the text font
- **Status:** Done in 2486dd68.
- **Where:** `/log` uptime summaries and row count. `01-log-default-crop-uptime-4k2x.png`.
- **Kind:** design-language
- **Severity:** low
- **Problem:** "98.0% up, 1 drop" and "77 of 6,559 in log" compute to IBM Plex Sans. The rule is `--pb-font-num` for numbers. The time and hex cells name `--pb-font-mono` (log.css:80), which is for code and hex. The time is a number.
- **Change:** `web/log.css:113-116`, add `font-family: var(--pb-font-num);` to `.sode-log__up-sum`. Add `#log-count { font-family: var(--pb-font-num); }`. Split log.css:78-82 so `.sode-log__time` uses `var(--pb-font-num)` and `.sode-log__hex` keeps `var(--pb-font-mono)`.
- **Also check:** none.

### C1-22: The problem stripe is 2px from the time
- **Status:** Done in 3169d5af.
- **Where:** `/log` problem rows. `01-log-default-1080.png`, `08-log-problems-only-crop-problem-rows-4k2x.png`.
- **Kind:** spacing
- **Severity:** low
- **Problem:** The stripe is 4px (`--pb-accent-bar-w`) inside a cell with 6px padding, so the text starts 2px after it.
- **Change:** `web/log.css:91-93`, add a rule: `.sode-log__table :is(th, td):first-child { padding-inline-start: calc(var(--pb-accent-bar-w) + var(--pb-space-3)); }`. Every row then starts 10px in and the stripe has 6px of air.
- **Also check:** none.

### C1-23: A typed "|" stands in for a divider
- **Status:** Done in 817c4c71.
- **Where:** `/log` Show row, between "Settings saves" and "Problems only". `01-log-default-crop-filters-4k2x.png`.
- **Kind:** design-language
- **Severity:** low
- **Problem:** `log.js:100` makes a span with the character `|` at 13px beside 11px chips. Its box is 19.5px high in a 22px row (y 272.25), and on the phone it wraps to the end of a row alone.
- **Change:** `web/log.js:100`, `el('span', 'sode-log__sep', '|')` becomes `el('span', 'sode-log__sep')`. `web/log.css:55-58`, new:
  ```css
  .sode-log__sep {
    align-self: stretch;
    border-inline-start: var(--pb-border-w) solid var(--pb-edge-soft);
    margin: 0 var(--pb-space-2);
  }
  ```
- **Also check:** The bar's own rules (`.sode-bar__rule`) are drawn the same way.

### C1-24: Phone Settings foot: the path is cut and cannot be read
- **Status:** Done in b9a5faa1.
- **Where:** Settings, phone. `151-settings-phone-top-phone3x.png`.
- **Kind:** edge case
- **Severity:** low
- **Problem:** "Saved to /private/tmp/claude-501…" The path is 1183px of text in a 213px box. Tips do not show on touch, so the rest cannot be reached. The Back button's icon starts at x 18 while the page edge is x 12 (the button has `margin-left: -8px` and 12px padding).
- **Change:** In `mobile.css` add `[data-layout="phone"] #settings-path { display: none; }` (the path is server detail that a phone user cannot act on), or let it wrap under the row with `overflow-wrap: anywhere`. For Back, change the margin to `calc(var(--pb-space-5) * -1)` so the icon starts at x 12.
- **Also check:** The gap from the Settings head to the search field is 44px on the phone, and the same 44px before the save bar. `var(--pb-space-7)` is enough on a phone.

### C1-25: Unknown paths answer with bare text
- **Status:** Done in b9ef0573.
- **Where:** `/no-such-page`. `161-route-not-found-1080.png`.
- **Kind:** edge case
- **Severity:** low
- **Problem:** The reply is the Go default, `404 page not found`, as plain text. A user who mistypes `/logs` or `/panel` gets no Sode page and no way back.
- **Change:** In the HTTP mux's fallback (internal/app, the handler that calls `http.NotFound` for page paths), serve a small embedded page with status 404 for GET requests that accept `text/html`: title "Sode", text "This page does not exist.", and a link "Open Sode" to `./`. Keep the plain reply for `/api/`.
- **Also check:** `/healthz`, `/api/health`, `/manifest.webmanifest` and `/icon.png` are correct. Nothing user-visible is wrong there.

### C1-26: Lock screen name box on the phone
- **Status:** Done in 4d3b65a6.
- **Where:** `148-main-phone-signed-out-phone3x.png`, `22-log-signed-out-1080.png`.
- **Kind:** alignment
- **Severity:** low
- **Problem:** Same fault as A1-02: the Name box is wider than the PIN pad (phone: box x 57 to 867 against pad x 143 to 781 in shot pixels). No new cause.
- **Change:** The A1-02 change (`lock.css:145-146`, `width: var(--sode-lock-w)`) clears it here too.
- **Also check:** none.

## Shared causes

- `log.css` has no label column and no phone rules. One label width (C1-04) and one phone block (C1-05) clear the ragged left and right edges on both layouts.
- `white-space: nowrap` on every cell plus automatic table layout. The wrap rule (C1-01) and fixed widths (C1-07) clear the cut Bytes column, the jumping columns and most of the phone's sideways scroll.
- Pages on the shared bar never call `renderDown`. One change in `panelsbar.js` (C1-02) gives `/log`, `/panels` and the editor the main page's Lost link banner and one wording.
- Router names are built in three places. `routerName` fixed once and used everywhere (C1-12) clears the uptime heads, the Routers chips, the hover box and the banner.
- `uptimebars.js` makes every bar a full button with a centred tip. Roving focus, the `aria-label` fix and edge-aware tips (C1-03, C1-10, C1-15) are all in that one 21-line file.
- `WHY_LABEL` is not checked against the whys the server writes. One test (C1-13) keeps code names out of the log.

Summary
26 entries: 5 high, 14 medium, 7 low.
Worst 1: the Log table is wider than the page at every size, so the Bytes column is cut and its scroll bar is off screen (C1-01, fix measured).
Worst 2: with the Server link down, /log, /panels and the editor show only a red icon and no words, and one state has four designs (C1-02).
Worst 3: the uptime bars put 288 Tab stops before the filters, and the four filter rows start at x 280, 212, 231 and 208 (C1-03, C1-04).
The phone /log has no layout of its own: title cut to "Sode C…", three right edges, 2.5px bars (C1-05, C1-11).