# Findings A1: setup wizard, Run setup again, lock screen

Scope: wait page, Take control, Restore from a backup, steps 1 to 4, Done, Run setup again, lock screen and PIN sign-in.
Code read at `/Users/seth/Desktop/sode/.claude/worktrees/review-shots` (origin/alpha-round2, 8e0969a7).

How the numbers were made: I booted Sode on port 41080 with a simulator on 41010, ran the wizard and the lock in headless Chrome at 1920x1080 DPR 1, and read `getBoundingClientRect` and text Range boxes. All px values are at 1080 unless a line says 4K. Root font is 16px at 1080 and 20px at 3840 (base.css). Token values used below: `--pb-space-3` 6px, `--pb-space-4` 8px, `--pb-space-7` 16px, `--pb-control-h-lg` 40px, `--pb-border-w` 1px. Scripts and raw output: `A1work/m.mjs`, `A1work/m2.mjs`, `A1work/out-1920.json`, `A1work/out2.json` in the review folder. Nothing in the repo was changed, and the servers I started are stopped.

Counts: 6 high, 15 medium, 8 low.

---

### A1-01: The site name and Theme color go back to "Sode" and blue during setup
- **Status:** Done in 4e903152.
- **Where:** Setup step 2 onward, after the Address changes. Visible in the owner's own shots: `012-setup-2-router-connected-1080.png` shows "Studio B" in teal; `014-setup-2-router-multi-destination-help-tip-1080.png`, `016-setup-3-access-standard-1080.png`, `017`, `018`, `019` show "Sode" in blue with the same answers.
- **Kind:** bug
- **Severity:** high
- **Problem:** Typing a new router address sends `setupRouter`. The server answers with a hello. `brand.js:16` (`on('helloCfg')`) then calls `showBrand(cfg.brandText, cfg.brandBorder)` with the saved config, which is still empty. Measured: before the address change the bar title is "Studio B" and `--pb-accent` is `#006d6a`; 4 s after typing `127.0.0.1:1` the title is "Sode" and the accent is the default `#3a44c4`, and it stays that way through step 3. The answers `d.brandText` and `d.brandBorder` are still right, so Finish saves teal and the page jumps back to teal on Done. Every reconnect during setup does the same.
- **Change:** `web/setupwizard.js:587`, in `on('hello', …)`, put the wizard's answers back after each hello. Inside the `if (S.setup) {` branch, before `paintDone();` add:
  ```js
  // The hello's config has no brand yet: keep the one the answers show.
  if (d) showBrand(d.brandText, d.brandBorder);
  ```
  `brand.js` listens on `helloCfg`; confirm it fires before `hello` (it did in my run, since the reset was visible). Add a browser check: pick Teal, change the address, wait for the hello, assert the bar title.
- **Also check:** `mode === 'again'` returns at the top of the same handler, so a hello during Run setup again also resets a changed name or color. Add the same line before that `return`.

### A1-02: Lock screen: the Name box is 80px wider than the PIN pad
- **Status:** Done in 4d3b65a6.
- **Where:** Lock screen, every state. `029-lock-screen-name-list-1080.png`, `031-lock-screen-pin-partial-1080.png`, `032-lock-screen-wrong-pin-crop-main-4k2x.png`.
- **Kind:** alignment
- **Severity:** high
- **Problem:** Name box x 790 to 1130 (340px). Pad x 830 to 1090 (260px). The box sticks out 40px on each side of the keys under it. `lock.css:15` says `--sode-lock-w` is "the pad and the name box", but `.sode-lock__namefield` (`lock.css:146`) uses its own width, `calc(var(--pb-control-h-lg) * 8.5)`.
- **Change:** `web/lock.css:145-146`. Current:
  ```css
  max-width: calc(100vw - var(--pb-space-9) * 2); /* the lock's own padding */
  width: calc(var(--pb-control-h-lg) * 8.5);
  ```
  New:
  ```css
  width: var(--sode-lock-w);
  ```
  If 260px is too narrow for a 24-character name at `--pb-text-xl` (a 24-character name measured 290px of text), raise the shared value instead: `lock.css:15` `--sode-lock-w: min(100%, calc(var(--pb-control-h-lg) * 8.5));`. Then the pad is 340px too and its keys become 105px wide. Either way one width.
- **Also check:** `.sode-lock__namefield` is a child of `.sode-lock`, where `--sode-lock-w` is set, so no other change is needed. The Step in pad uses the same nodes.

### A1-03: Lock screen: a blurred second copy of the site name shows above the real one
- **Status:** Done in 21426763.
- **Where:** Lock screen over the main page, every state, light and dark. `029`, `031`, `033`, `034`, `036` at 1080; `030-lock-screen-name-typed-dark.png`.
- **Kind:** readability
- **Severity:** high
- **Problem:** The main page's `.sode__mark` (y 167 to 233, 66px type) stays visible through the overlay (78% `--pb-surface-base` and a 12px blur). The lock's own brand is the same size at y 220 to 299 (y 158 with the name list open). So the page shows the name twice, one blurred, 53px apart, and with a long name the two overlap (`A1work/lk-1366-typed.png`).
- **Change:** Hide the page's own name while the overlay is on. The lock brand flies from that spot (`lockbrand.js`), so nothing is seen to vanish. Add to `web/lock.css` after line 53:
  ```css
  /* The lock's brand starts where the page's name is, so the page's name hides for as long as the overlay is on. */
  [data-sode-lock-on] :is(.sode__mark, .sode-m__mark) {
    visibility: hidden;
  }
  ```
  `visibility` keeps the box, so `logo()` in `lockbrand.js:6` still measures it for the way out.
- **Also check:** The other blurred shapes (Take button in orange, level chips in color) also read through the overlay. If the owner wants a calmer lock, raise `lock.css:46` from `78%` to `88%`.

### A1-04: Access step: column headings are left-aligned, their Yes and No are centred
- **Status:** Done in 2c86e004.
- **Where:** Setup step 3 and Run setup again step 3. `016-setup-3-access-standard-1080.png`, `017-setup-3-access-secured-picked-crop-sheet-4k2x.png`, `027`.
- **Kind:** alignment
- **Severity:** high
- **Problem:** Each preset heading starts at the left of its column (radio at x 775.1, 1025.0, 1274.9). The Yes and No marks below are centred in the column (icon at x 872.7, 1122.7, 1374.6). The heading sits 98 to 100px left of the values it names, so the eye does not read them as one column.
- **Change:** Left-align the marks under the radio. `web/setup.css:262-265`, current:
  ```css
  .sode-setup__sheet td {
    text-align: center;
    cursor: pointer;
  }
  ```
  New: the same rule with `text-align: left;`. `th` and `td` share `padding: var(--pb-space-1) var(--pb-space-4)`, so the mark's icon lands at x 775.1, the radio's left edge.
- **Also check:** none.

### A1-05: Access step: "Without sign-in:" sits 5px lower than the three preset names
- **Status:** Done in 2c86e004.
- **Where:** Setup step 3 heading row. `016-setup-3-access-standard-1080.png`, `016-setup-3-access-standard-crop-sheet-4k2x.png`.
- **Kind:** alignment
- **Severity:** high
- **Problem:** Corner text box y 240.9 to 258.9 (14px bold, `vertical-align: bottom`). Preset labels y 236.1 to 253.1 (13px bold, `vertical-align: top`, inside a 32px `.pb-check`). The baselines differ by about 5px in one row. Two sizes and two alignments in one heading row.
- **Change:** `web/setup.css:245-261`. In `.sode-setup__sheet thead th` change `vertical-align: top;` to `vertical-align: middle;`. In `.sode-setup__sheet thead .sode-setup__corner` delete `vertical-align: bottom;` and change `font-size: var(--pb-text-md);` to `font-size: var(--pb-text-sm);` so all four headings are 13px bold on one centre line.
- **Also check:** none.

### A1-06: Access step: row labels and their Yes or No are 1 to 2px apart, and it changes from row to row
- **Status:** Done in 2c86e004.
- **Where:** Setup step 3 body rows. `016-setup-3-access-standard-1080.png` (label y 277 against Yes y 276 in row 1, 305 against 303 in row 2).
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Row label text centre y 277.8; mark centre y 276.65 (1.15px higher). Cause: the marks are `inline-flex`, so the cell adds baseline space under them and the row grows to 27.25px. Fractional rows (264.9, 292.1, 319.4, 346.6 …) then round differently, so the offset is 1px in one row and 2px in the next.
- **Change:** `web/setup.css:278-283`, in `.sode-setup__yes, .sode-setup__no` change `display: inline-flex;` to `display: flex;` (keep `align-items` and `gap`). Give the table whole-pixel rows: add `line-height: 1.25rem;` to `.sode-setup__sheet` (`setup.css:230`). Rows become 2 + 20 + 2 + 1 = 25px and label and mark share one centre.
- **Also check:** After A1-04 the marks are left-aligned, so `display: flex` needs no `justify-content`.

### A1-07: Wizard footer: the error text sits 6.5px above the Next label
- **Status:** Done in df2c91c2.
- **Where:** Setup steps 1 and 2 footer with a problem shown. `009-setup-1-style-user-pin-mismatch-crop-foot-4k2x.png`, `008-setup-1-style-user-custom-color-caution-1080.png` ("Enter your name."), also "Saving…" and "Restoring…".
- **Kind:** alignment
- **Severity:** high
- **Problem:** `#setup-err` stretches to the footer row (y 561 to 593) and its text sits at the top (y 562 to 579). The Next label is at y 568.5 to 585.5. The red text is 6.5px higher than the button text beside it.
- **Change:** `web/setup.css:168-171`, add `align-self: center;` to `.sode-setup__err`.
- **Also check:** "Saving…" and "Restoring…" use the same span in error red although they are not errors. In `setupwizard.js:462` and `setuprestore.js:27` add class `is-busy` and a rule `.sode-setup__err.is-busy { color: var(--pb-text-muted); }`, cleared when the text changes.

### A1-08: Theme color swatches: Custom is off the pitch of the others
- **Status:** Done in 7f3c80ee.
- **Where:** Setup step 1. `005-setup-1-style-user-empty-1080.png`, `009-setup-1-style-user-pin-mismatch-crop-dialog-4k2x.png`.
- **Kind:** spacing
- **Severity:** medium
- **Problem:** Swatch boxes are 36px on a 42px pitch (x 380.8, 422.8, 464.8, 506.8, 548.8). The Custom box is at x 595.2, not 590.8: its label "Custom" is 44.8px wide and widens the item, so the gap before it is 10.4px against 6px. Labels also nearly touch: "Violet" ends at 541.6 and "Lilac" starts at 553.4.
- **Change:** Give every item one width that fits the longest label. `web/setup.css:178-188`, change `min-width: 2.25rem;` to `width: 3rem;`. Boxes stay 2.25rem and centre in the 3rem item, so all six sit on one 54px pitch.
- **Also check:** `pickerColors` can add custom-named colors; a name longer than about 7 characters still overflows 3rem. Add a `.sode-setup__swatch-name` rule (the class has none today): `overflow: hidden; text-overflow: ellipsis; max-width: 100%;`.

### A1-09: Lock screen: the PIN dots are 4px left of centre
- **Status:** Done in 43a1eb86.
- **Where:** Lock screen with digits typed. `031-lock-screen-pin-partial-1080.png`, `031-lock-screen-pin-partial-crop-dots-4k2x.png`, `036`.
- **Kind:** alignment
- **Severity:** medium
- **Problem:** `.sode-lock__dots` has `letter-spacing: var(--pb-space-4)` (8px). Letter spacing also follows the last dot, so the box (x 931.9 to 988.1) is centred but the dots inside it are not: two dots sit at x 942 and 970, centre 956 against the page centre 960.
- **Change:** `web/lock.css:197-202`, add `padding-inline-start: var(--pb-space-4);` to `.sode-lock__dots`, and `padding-inline-start: 0;` to `.sode-lock__dots.is-text` (`lock.css:128`), since "Incorrect" has normal spacing.
- **Also check:** `.sode-lock__brand` has `letter-spacing: var(--pb-tracking-label)` (5.28px at this size), so the site name is 2.6px left of centre for the same reason. Add `padding-inline-start: var(--pb-tracking-label);` at `lock.css:112`.

### A1-10: Lock screen: the site name, Name box and pad all jump 61px when the name list opens or closes
- **Status:** Done in ae790208.
- **Where:** Lock screen. Compare `029-lock-screen-name-list-1080.png` (brand top y 158) with `030` and `031` (brand top y 220).
- **Kind:** spacing
- **Severity:** medium
- **Problem:** `.sode-lock__main` is centred with `margin-block: auto` (`lock.css:94`). The name list is in flow (`lock.css:163`), so when it opens (120px for three users) the whole column moves up 61px and the pad moves down 61px. The keys move under the pointer between picking a name and pressing the first digit.
- **Change:** Let the list drop over the pad as it does in dialogs. In `web/lock.css:163-167` delete `position: static;` and `margin-top: var(--pb-space-1);` so the `.sode-namepick__list` rule at line 176 applies (absolute, `top: calc(100% + var(--pb-space-1))`); keep the `max-height`. The list closes on a pick, so it never hides a key the user needs.
- **Also check:** The comment at `lock.css:87-88` says the list pushes the pad down so both fit a 768px screen. With a drop-down list the pad no longer has to shrink. At 1366x768 with the list open the keys shrink from 64px to 42px today (`A1work/out2.json`, "1366 open").

### A1-11: Lock screen: PIN feedback is above the Name box, with a 139px empty band
- **Status:** Done in e7dbea5f.
- **Where:** Lock screen. `033-lock-screen-wrong-pin-again-1080.png` (empty band), `034-lock-screen-lockout-wait-1080.png` ("Try again in 30s" alone in the band).
- **Kind:** readability
- **Severity:** medium
- **Problem:** The body order is state line, dots, message, Name, pad (`lock.js:199`). The user fills Name first and the PIN second, but the PIN's dots and messages appear above the Name box. With no digits typed, three reserved rows leave 139px of nothing between the site name (bottom y 298.8) and the "Name" label (y 437.9). The lockout text is 13px, red, 126px above the keys it concerns.
- **Change:** `web/lock.js:199`, current:
  ```js
  padBody = el('div', 'sode-lock__body', state, dotsEl, msg, nameField, digits);
  ```
  New (entry order: who, then PIN, then keys):
  ```js
  padBody = el('div', 'sode-lock__body', nameField, dotsEl, state, msg, digits);
  ```
  The band between brand and Name falls to the body's 16px gap, and "Incorrect", the dots and the wait text sit right above the keys.
- **Also check:** Needs A1-10's drop-down list, or the open list would push the dots. The Step in pad (`modeLine`) uses the same body.

### A1-12: Lock screen: the Submit key is a text glyph, 40px high beside 64px keys, and almost invisible when off
- **Status:** Done in 17f83954.
- **Where:** Lock screen, bottom key. `029-lock-screen-name-list-crop-pad-4k2x.png`, `032-lock-screen-wrong-pin-crop-main-4k2x.png`, `030-lock-screen-name-typed-dark.png`.
- **Kind:** design-language
- **Severity:** medium
- **Problem:** Three things. (1) `lock.js:160` draws the key with the character `'✓'`, which Plex renders like a square-root sign. AGENTS.md: copy missing icons from upstream, never fake them; the wizard already uses `icon('check')`. (2) Its row is `var(--pb-control-h-lg)` (40px) under four 64px rows (`lock.css:220`). (3) Disabled, it has `opacity: 0.55` on top of a 38% text color and a `rgb(220,220,220)` border on a `rgb(228,228,228)` page, so the border and glyph nearly vanish. In dark it is a faint outline on black. A first-time user cannot see where to submit.
- **Change:** `web/lock.js:160`, current `checkKey = key('✓', 'enter', 'sode-lock__key--go');`. New: `checkKey = key(icon('check'), 'enter', 'sode-lock__key--go'); checkKey.append(' Sign in');` (import `icon` from `./core.js`), and set the `aria-label` to "Sign in". `web/lock.css:220`: `grid-template-rows: repeat(5, minmax(var(--sode-h-action), calc(var(--pb-control-h-lg) * 1.6)));` and line 221 `min-height: calc(var(--sode-h-action) * 5 + var(--pb-space-5) * 4);`. For the off state add `.pb-btn.sode-lock__key--go:disabled { opacity: 1; }` so only patchbay's disabled colors apply.
- **Also check:** `sode-lock__cancel` shares that row and gets the new height too. In Step in mode the word should be "Step in" (`lockrules.js:280`).

### A1-13: Router step: the Multi destination buttons jump down 23.5px when Router salvo is picked
- **Status:** Done in 7f3c80ee.
- **Where:** Setup step 2. Compare `012-setup-2-router-connected-1080.png` (buttons y 746) with `015-setup-2-router-router-salvo-group-crop-dialog-4k2x.png` (buttons y 770).
- **Kind:** bug
- **Severity:** medium
- **Problem:** The choice has no label and shares a `.sode-setup__row` (`align-items: end`) with the Router salvo group field, which has a label. When the field appears the row grows from 32px to 55.5px and the buttons drop to its bottom: y 746.3 to 769.8. The button the user just pressed moves out from under the pointer, and a 31.5px hole opens between the help sentence and the buttons.
- **Change:** `web/setupwizard.js:259-272`. Put the group field in its own row under the choice:
  ```js
  choice('fireMode', null, [['sequential', 'One after another'], ['native', 'Router salvo']], hint('fireMode')),
  ...(d.fireMode === 'native'
    ? [row(field('setup-group', 'Router salvo group', d.group, { type: 'number', min: '0', max: '127' }, hint('group'), 'Must be unused on the router'), el('div'))]
    : []),
  ```
- **Also check:** The tip "Must be unused on the router" is on the label only, where few users hover. Show it as the field's hint: "Use a group that nothing else on the router uses."

### A1-14: Two tips are badly placed: the help tip covers the buttons it explains, the Take control tip hangs outside the dialog
- **Status:** Done in 339e3e5f.
- **Where:** `014-setup-2-router-multi-destination-help-tip-1080.png`; `002-setup-wait-page-take-control-tip-1080.png`.
- **Kind:** readability
- **Severity:** medium
- **Problem:** (1) The help tip is 352px wide, centred under the help button, and covers the sentence under the heading and most of "One after another" and "Router salvo", the two choices it describes. (2) The Take control tip is centred under a button at the dialog's left edge, so it starts at x 336, 33px outside the dialog (x 369) and the page column.
- **Change:** (1) `web/setupwizard.js:110`, add `pb-tip--top pb-tip--start` to the help button's class list so the tip opens above the heading with its left edge on the button. tooltip.js reads both from patchbay's rules. (2) `web/setupwizard.js:533`, class `'pb-btn pb-btn--outline pb-tip--start'`.
- **Also check:** `cfg-rt-fireMode-help` in `web/index.html:530` has the same tip over a select; give it `pb-tip--top` too.

### A1-15: Wizard fields are far too wide, worst at 4K
- **Status:** Done in 7f3c80ee.
- **Where:** Steps 1 and 2. `005-setup-1-style-user-empty-4k.png`, `005-setup-1-style-user-empty-1080.png`, `012-setup-2-router-connected-1080.png`.
- **Kind:** readability
- **Severity:** medium
- **Problem:** At 1080 the server Name box is 1136px wide for at most 24 characters, and Address is 1136px for an IP and port. At 3840 the dialog is about 2380px wide and Name is about 2350px: one thin line across the screen with the Next button 2300px from the first field. PIN and Repeat your PIN are 369px each for 4 to 8 digits (about 790px each at 4K). On step 2 widths are mixed: Router title and Levels are half width (561px), Address is full width.
- **Change:** Cap the form column and use one field width. `web/setup.css` after line 141 add:
  ```css
  /* Fields keep a readable width on a wide dialog. */
  .sode-setup__section > .sode__field,
  .sode-setup__row {
    max-width: 48rem;
  }
  ```
  `web/setupwizard.js:214` wrap the server Name in a row like Router title: `row(field('setup-brandText', …), el('div'))`. `setupwizard.js:237` the same for Address. Then Name, Router title, Address and Levels share one width, and the three user fields fill the same 48rem.
- **Also check:** `.sode-setup__sheet` (Access table) is also about 2350px wide at 4K with labels 1500px from their marks; give it `max-width: 64rem;`. `setuprestore.js:95-96` stacks New PIN and Repeat your PIN at full width; put them in `sode-setup__row`.

### A1-16: The wizard names two settings differently from Settings
- **Status:** Dropped: "Site name" is not a glossary term; renaming needs the owner and a glossary row (second pass).
- **Where:** Step 1 heading and first label; step 2 last heading. `005-setup-1-style-user-empty-1080.png`, `012-setup-2-router-connected-1080.png`; Settings `web/index.html:340` and `:530`.
- **Kind:** text
- **Severity:** medium
- **Problem:** Wizard: heading "Sode Server Name and Theme Color" (title case, unlike every other heading here) over a field called "Name". Settings calls the same field "Logo text and page title". Step 1 then has a second field called "Name" (the user's). Wizard: "Multi destination take behavior"; Settings: "Multi destination" with option "Router salvo (stage, then Take all)".
- **Change:** `web/setupwizard.js:214`: heading `'Site name and Theme color'`, field label `'Site name'`. `web/index.html:340`: `Logo text and page title` becomes `Site name` (keep the old words findable in Settings search). `web/setupwizard.js:257`: heading `'Multi destination'`, help label `'About Multi destination'`. Add "Site name" to GLOSSARY.md if the owner accepts it.
- **Also check:** The chip label "Style & User" (`setupwizardrules.js:7`) names neither thing by its glossary word. Suggested: `'Site & user'`.

### A1-17: Access step: every row repeats "without signing in" under a heading that already says it
- **Status:** Done in 2c86e004.
- **Where:** Step 3 table. `016-setup-3-access-standard-1080.png`.
- **Kind:** text
- **Severity:** medium
- **Problem:** The corner says "Without sign-in:" and then four of six rows end in "without signing in" and two in "without a key". The words that differ are buried at the start of long lines. Row 3 is a different sentence shape from the rest.
- **Change:** `web/setupwizardrules.js:15-22` and `setupwizard.js:160`. Corner: `'A person who is not signed in can:'`. Rows:
  - `'Change routes'`
  - `'Change routes across routers'` (move up, beside its pair)
  - `'Change shared presets and filter sets'`
  - `'Open public panels'`
  - `'Read names and tally from another app, without a key'`
  - `'Take from another app, without a key'`

  "filter sets" is the glossary term; the current row says "filters".
- **Also check:** `accesspreset.js` shows the same rows in Settings; change both from the one list.

### A1-18: Other wizard and lock text that is long or unclear
- **Status:** Done in a8ecfe67.
- **Where:** Steps 1, 2, Done, wait page, lock.
- **Kind:** text
- **Severity:** medium
- **Problem and change (current, then new):**
  - `setupwizardrules.js:67` "Connection: Add your first router" becomes "Connection". The step is already "Router".
  - `setupwizard.js:258` "Some routers support staging changes for taking many routes simultaneously." becomes "How Sode sends a take with many destinations."
  - `setupwizardrules.js:87` (tip, 46 words) becomes "One after another: Sode sends each route alone, about 16 ms apart. Router salvo: the router holds all the routes and switches them together. The router must have salvos, and the group must be free."
  - `setupwizardrules.js:168` "Connected: 200 sources and 120 destinations discovered." becomes "Connected. 200 sources, 120 destinations."
  - `setupwizard.js:222` "This user will be the administrator." becomes "This user is the first Administrator." (glossary capital, present tense).
  - `setupwizard.js:226` "Repeat your PIN" becomes "Repeat PIN" (the label beside it is "PIN", not "Your PIN").
  - `setupwizardrules.js:76-80` "Congratulations! You are signed in as Ada and connected to the router at 127.0.0.1:18910 with Standard access permissions." becomes "Setup is complete." line break "You are signed in as Ada. Router 127.0.0.1:18910 is connected. Access: Standard access." The dialog title already says "Sode is set up". The not-connected form: "Router 127.0.0.1:18910 has not answered yet. Sode keeps trying."
  - `setupwizard.js:527` "Another browser took control. Your answers stay in this browser until you take control again." becomes "Another browser took control. This browser keeps your answers."
  - `accent.js:167` "…Selection can look like a state." becomes "…A selected item can look like a state."
  - `lock.js:172` key "Del" becomes "Delete" (STE rule 7, words in full; about 45px of text fits the 78.7px key at 14px).
  - `internal/app/pin.go:99,123` "try again in 30s" prints a Go duration ("30s", "1m30s"). Print seconds in words: `fmt.Errorf("wrong PIN; try again in %d seconds", int(w.Seconds()))`, so the lock shows "Try again in 30 seconds".
- **Also check:** `lockrules.js:116` comment quotes the old form. AB-17 set "Congratulations, then one sentence" in round 2, so the Done rewrite is the owner's call.

### A1-19: Step chips: Access gets a check mark before Finish, and Run setup again keeps "Style & User"
- **Status:** Done in 1736c83b.
- **Where:** `019-setup-1-style-user-revisited-crop-steps-4k2x.png`; `025-setup-again-1-style-1080.png`.
- **Kind:** bug
- **Severity:** medium
- **Problem:** (1) `setupwizard.js:313`: `done = !now && (i < at || i <= reached)`. After the user opens step 3 and goes back to step 1, measured classes are `is-now`, `is-done`, `is-done`: Access shows a check although Finish was never pressed, and the line after it stays grey. A visited step is not a completed step. (2) Run setup again has no Add a first user section but its first chip still reads "Style & User".
- **Change:** (1) `setupwizard.js:313`: `const done = !now && i < Math.max(at, reached);` so a step is done only when a later step was reached; the furthest visited step shows its number. Leave `chipGoes` as is so the chip still works. (2) `setupwizard.js:311`: `all.forEach(([key, name], i) => { const label = d.again && key === 'welcome' ? 'Site' : name; …` and use `label` on line 319.
- **Also check:** The check icon in a done chip is a 14px light stroke in a 32px box beside 13px bold digits; add `.sode-setup__dot .pb-icon { width: 1rem; height: 1rem; }` so done and numbered chips have equal weight.

### A1-20: The footer shows a red error before the user has reached the field
- **Status:** Done in df2c91c2.
- **Where:** Step 1. `008-setup-1-style-user-custom-color-caution-1080.png` ("Enter your name." in red after only the site name and a color were set).
- **Kind:** readability
- **Severity:** medium
- **Problem:** `setupwizard.js:411` writes `stepProblem` to the footer on every input. Typing the site name, which is optional, puts "Enter your name." in error red at the bottom right. Typing the user name then shows "A PIN has 4 to 8 digits." before the PIN box has focus. The user has made no mistake yet. The first view has no text at all, so a disabled Next gives no reason.
- **Change:** Show the reason as a muted hint always, and in red only for a real mistake. `setupwizard.js:350` and `:411`: write `stepProblem(d, at, max())` in both places, and toggle class `is-hint` unless the field at fault already has text: on step 1, `const real = !!d.pin2 || (!!d.pin && !/^\d*$/.test(d.pin));` then `$('setup-err').classList.toggle('is-hint', !real)`. CSS: `.sode-setup__err.is-hint { color: var(--pb-text-muted); }`.
- **Also check:** `role="alert"` on `#setup-err` (`index.html:81`) makes a screen reader speak on every key; `role="status"` fits a hint.

### A1-21: Restore from a backup with a new PIN: buttons move into the body and leave an empty footer
- **Status:** Done in f708aec4.
- **Where:** Setup, Restore from a backup, a file with no PINs (`setuprestore.js:71-110`). Not captured: `006-setup-restore-from-backup-1080.png` shows step 1 only, because the file dialog is native. Found by reading the code, not measured.
- **Kind:** edge case
- **Severity:** medium
- **Problem:** `askPin` and `showFailed` hide Back, Restore from a backup and Next, and put their own Back and Restore buttons at the bottom right of the body (`.sode-setup__restore`, `justify-content: flex-end`). The footer stays, 50px high, holding only the status text. So this page has Back at the right beside the primary button, where every other step has Back at the footer's left and the primary at its right, with an empty bar under them. The step chips also stay live: a chip click re-renders step 1 and drops the picked file with no word. The two PIN fields are full width (1136px).
- **Change:** Reuse the footer. In `setuprestore.js` `askPin`: keep `setup-back` and `setup-next` visible, set `$('setup-next').textContent = 'Restore'`, and route their clicks through a `restoreStep` flag that `setupwizard.js` checks at the top of its `setup-back` and `setup-next` handlers (`:444`, `:446`). Delete the `.sode-setup__restore` row. Put the two fields in `el('div', 'sode-setup__row', pinBox, pin2Box)`. Remove every chip's `data-go` while the restore page is up.
- **Also check:** `showFailed` ("Restored. 2 items not written: check panels and artwork.") has the same shape; its Open Sode button belongs in the footer's right slot. Text: "The restore is complete, but 2 items were not written. Check panels and artwork."

### A1-22: Router step: sections are only 8px further apart than fields
- **Status:** Done in 7f3c80ee.
- **Where:** Step 2. `012-setup-2-router-connected-1080.png`.
- **Kind:** spacing
- **Severity:** low
- **Problem:** Field to field: 8px. Heading to its first field: 8px. Section to section: 16px (313.2 to 329.2, 457.5 to 473.5, 680.8 to 696.8). With bold 14px headings and 13px labels, four sections read as one list of nine things. Step 1 has a rule between its two sections; step 2 has nothing.
- **Change:** `web/setup.css:121`, `margin-top: var(--pb-space-7);` becomes `margin-top: var(--pb-space-9);` (24px). Set `.sode-setup__hr { margin: var(--pb-space-9) 0 0; }` and delete the special case at lines 132-134 so step 1 has the same rhythm.
- **Also check:** The status line ("Connected…") is a sibling of the Address field at the same 8px gap as a new field, and looks the same for connected and no answer. Move it into the field as its last child so it sits 4px under the hint.

### A1-23: Done step: the Next steps column is not lined up with the text, and its links look disabled
- **Status:** Done in 7f3c80ee.
- **Where:** `020-setup-4-done-1080.png`, `021-setup-4-done-next-step-hover-crop-next-4k2x.png`.
- **Kind:** alignment
- **Severity:** low
- **Problem:** "NEXT STEPS" text top is y 231.9; the paragraph's first line is y 234.9 (3px lower). The column's last link ends at y 348.1 and the Open Sode button at y 333.7, so the two columns end 14px apart. The links are `--pb-text-muted` with an underline, the same grey as hints, so the four most useful follow-ups look like fine print. The column is fixed at 13rem on the far right, 940px from the text at 1080 and further at 4K.
- **Change:** `web/setup.css:288-323`. Grid: `grid-template-columns: minmax(0, 32rem) 13rem; justify-content: start; align-items: start;` so the column sits beside the text. `.sode-setup__next h3`: add `line-height: calc(var(--pb-text-md) * 1.6);` so its first line shares the paragraph's line box. `.sode-setup__next a`: delete the `color` rule so links use patchbay's link color. Add `.sode-setup__done p { margin-top: 0; }`.
- **Also check:** `.sode-setup__next` uses `border-left` and `padding-left`; the file elsewhere uses logical properties (`padding-inline`).

### A1-24: Raw px in setup.css where a token exists
- **Status:** Done in 7f3c80ee.
- **Where:** `web/setup.css` lines 74, 210, 226 (`outline: 2px solid …; outline-offset: 2px`), 238 and 303 (`1px solid var(--pb-edge)`), 276 (`inset 0 3px 0`).
- **Kind:** design-language
- **Severity:** low
- **Problem:** AGENTS.md: sizes in rem, tokens only. These do not grow with the root at 4K (root 20px): a 2px ring on a 45px swatch and a 3px picked-column bar on a 2380px table.
- **Change:** Lines 238 and 303: `var(--pb-border-w) solid var(--pb-edge)`. Lines 74, 210, 226: `outline: 0.125rem solid …; outline-offset: var(--pb-space-1);` (or patchbay's focus-ring token if 2.2.1 has one; I did not check). Line 276: `inset 0 var(--pb-space-2) 0 var(--pb-accent)` (4px).
- **Also check:** `lock.css:502` `font-size: 0.875rem` and `:539` `font-size: 1rem` (users and profiles dialogs, same file) should be `var(--pb-text-md)` and `var(--pb-text-lg)`.

### A1-25: The color caution wraps to two lines in a 1136px dialog
- **Status:** Done in 7f3c80ee.
- **Where:** Step 1 with a custom color near a state color. `008-setup-1-style-user-custom-color-caution-1080.png`.
- **Kind:** readability
- **Severity:** low
- **Problem:** `.sode__brand-caution` has `max-width: 32rem` (`base.css:450`), sized for Settings. In the wizard it wraps after "Selection can" (note box 512px wide, 39px high) with 620px of empty space to its right.
- **Change:** `web/setup.css`, add `.sode-setup .sode__brand-caution { max-width: none; }`. With A1-15's 48rem column it fits on one line.
- **Also check:** When the note appears the user section jumps down 45px while the native color picker is open. Reserve the line: in the wizard use `visibility: hidden` in place of `hidden` (`setupwizard.js:151`, `:399`).

### A1-26: Run setup again can show "Filled from SODE_ROUTER." for a value that is from the saved config
- **Status:** Done in 7f3c80ee.
- **Where:** `026-setup-again-2-router-1080.png` (the hint under Address).
- **Kind:** bug
- **Severity:** low
- **Problem:** `filled` (`setupwizard.js:46`) is set only in `startWizard` and never cleared. A page that ran first-time setup and then opens Run setup again without a reload still has the first run's `env`, so the Address hint says the value came from `SODE_ROUTER` although it is now the saved address. After a reload the hint is gone, so the same page shows two different things.
- **Change:** `web/setupwizard.js:626`, in `openAgain` add `filled = {};` before `render()`.
- **Also check:** none.

### A1-27: Lock screen at 4K: the pad is 8.5% of the screen width
- **Status:** Dropped: closed as not needed, since owner call 1 scales the root and the lock pad needs no vmin.
- **Where:** `029-lock-screen-name-list-4k.png`.
- **Kind:** readability
- **Severity:** low
- **Problem:** At 3840x2160 the pad is 325px wide and 430px high (keys 98x80px) in the middle of a 3840px screen; at 1080 it is 260px of 1920 (13.5%). The lock is the one page with nothing else on it, and a wall screen is read from a distance. The 125% root is the only growth.
- **Change:** Let the pad's width follow the short side of the window. `web/lock.css:15`: `--sode-lock-w: min(100%, max(calc(var(--pb-control-h-lg) * 6.5), 24vmin));` (518px at 2160 high; unchanged at 1080, where 24vmin is 259px). Raise the key row cap the same way at `lock.css:220`: `minmax(var(--sode-h-action), max(calc(var(--pb-control-h-lg) * 1.6), 6vmin))`.
- **Also check:** The "Name" label and state line stay 13px (16px at 4K) beside 22px input text; set `.sode-lock__namelabel { font-size: var(--pb-text-md); }`.

### A1-28: Lock screen name list with many users: 3.5 rows of 12, and the fourth is cut through its text
- **Status:** Done in 7d3aa156.
- **Where:** `A1work/lk-1920-open.png` (12 users, one with a 24-character name).
- **Kind:** edge case
- **Severity:** low
- **Problem:** The list is capped at `calc(var(--sode-h-action) * 3.5)` = 140px for 458px of names (`lock.css:165`), so a 12-user site scrolls a 140px window with the little scroll bar, and the half row cuts the fourth name through the middle of its letters. At 1080 there are 130px free under the pad.
- **Change:** `web/lock.css:165`: `max-height: min(calc(var(--sode-h-action) * 6.5), 40vh);`. With A1-10's drop-down list the extra rows cover the pad's first rows only while the list is open.
- **Also check:** A 24-character site name is 1036px wide on the lock at 66px and has no fit on desktop (`fitBrand` is phone only, `lockbrand.js:32`); under about 1090px of window it overflows and the lock scrolls sideways. Run `fitBrand` on every layout, or give the desktop brand `max-width: 100%; overflow-wrap: anywhere; white-space: normal; text-align: center;`. Tab order and focus rings on the pad are fine (2px accent outline on every key, measured).

### A1-29: Phone wait page: a box inside a box
- **Status:** Done in 7f3c80ee.
- **Where:** `003-setup-wait-page-phone-1080.png`.
- **Kind:** design-language
- **Severity:** low
- **Problem:** `desktopOnly()` brings its own 1px border and padding (`base.css:1379`). Inside the wait dialog it draws a bordered card inside the bordered modal for one sentence and one button.
- **Change:** `web/setup.css` after line 337: `.sode-setup__wait .sode__desktop-only { border: 0; padding: 0; }`.
- **Also check:** The wait dialog (no step chips) sits at y 108; the wizard dialog sits at y 179.5. Take control makes the dialog jump down 71px. Reserving the chips' height on the wait page would stop it.

---

## Shared causes

1. One width for the lock column. `--sode-lock-w` on both the Name box and the pad clears A1-02 and makes A1-27 a one-line change.
2. The lock's body order and in-flow name list. Making the list a drop-down and putting Name first clears A1-10, A1-11 and A1-28.
3. Trailing letter-spacing. One `padding-inline-start` equal to the spacing fixes the dots and the site name in A1-09.
4. Access table cell alignment. Left-aligned cells, `vertical-align: middle` headings and block-level marks clear A1-04, A1-05 and A1-06 together.
5. `#setup-err` does three jobs (hint, error, busy text) with one style and no vertical alignment. `align-self: center` plus `is-hint` and `is-busy` clears A1-07 and A1-20.
6. No width cap in the wizard. A 48rem form column and `row(…, el('div'))` for single fields clears A1-15 and A1-25 and most of the 4K emptiness.
7. A hello repaints from saved config while the wizard holds unsaved answers. Repainting from `d` after each hello clears A1-01; clearing `filled` in `openAgain` clears A1-26.
8. Wizard strings written apart from Settings. Taking labels from the Settings names (or one shared constant per label, as `FIRE_MODE_TIP` already does) clears A1-16 and keeps A1-17 and A1-18 from drifting again.