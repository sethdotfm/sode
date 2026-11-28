# Big, Bloaty, and Broken

Alpha bugs for the multi-router build. Notes are the owner's, taken while poking at the software, then grilled here. Round 1 (AB-01 to AB-06) was on branch `alpha` and lives in this file on `alpha-notes`. Round 2 (AB-07 on) is on the Settings redesign, branch `settings/redesign` at 619e5871 (PR 822).

Test setup: two or three swp08sim routers and Sode. Round 2 was taken with router A on :8910, extra simulators on :9312 and :9313 (`SIM_PORT=9312 go run ./cmd/swp08sim`), and Sode on :8080 with a new config, so the setup wizard ran first. Locally, `.dev/run-alpha.sh` (not in git) runs router A on :8910, router B on :8911 and Sode on :8080. Elsewhere, run `SIM_PORT=8910 go run ./cmd/swp08sim` and `SIM_PORT=8911 go run ./cmd/swp08sim`, start Sode against the first, and add router B in Settings > Routers. For a third router (AB-01 chains), add `SIM_PORT=8912` as router C, with tie lines A→B and B→C in Settings > Routers > Tie lines.

## Workflow

1. **Capture.** Record each note as reported. Do not fix anything yet.
2. **Grill.** Go through the entries with the owner. Fill in Expected, Decisions and Open questions until none is open.
3. **Hand off.** A cloud agent (with subagents) works the entries marked `ready`. It follows AGENTS.md, makes one commit per entry, and fills in Proof and Notes.

Status values: `captured` → `grilling` → `ready` → `in progress` → `done` (or `wontfix`, `blocked`).

## Proof rules (for the agents)

The owner wants each fix proven, not claimed. For every entry:

- **Before and after.** Reproduce the bug first on the two-router sim, and say how. Then show it fixed.
- **UI entries:** headless Chrome screenshots at 1920×1080 desktop, before and after, saved under `docs/research/alpha-proof/<entry id>/` (not checked by the STE and vocabulary tests). For a layout or animation entry, also show `main` for comparison where the entry says "as before".
- **A test that fails before the fix and passes after.** A browser check (`TestChrome*`, docs/agents/browser-checks.md) for UI behavior, a node test (`web/test/*.test.mjs`) for pure rules, a Go test for server behavior.
- **Checks run.** `go test -short ./...`, `go vet ./...`, `node --test 'web/test/*.test.mjs'`, and only the browser checks added or changed (AGENTS.md: CI runs the rest). Paste the pass lines in Notes.
- **Notes.** What changed, file:line of the main change, the commit SHA, and anything left over or found on the way.

## Template

### AB-00: short title

- **Status:** captured
- **Area:**
- **Reported:** the owner's words
- **Now:** what the code does today (file:line)
- **Expected:**
- **Decisions:**
- **Open questions:**
- **Proof:**
- **Notes:**

---

# Round 2

Taken on 2026-11-27 from 28 browser annotations on the Settings redesign. Four areas: the setup wizard (AB-07 to AB-18), the top bar (AB-19, AB-20), Settings > Routers (AB-21 to AB-23) and the main page patch row (AB-24, AB-25). Grilled on 2026-11-27: the agent proposed answers, a Fable review settled them, and the owner settled AB-25. All are `ready`.

## Setup wizard

### AB-07: Setup steps row stacks and clips its labels; needs space below it

- **Status:** ready
- **Area:** First-run wizard steps row. web/setup.css (`.sode-setup__steps`, `.sode-setup__chip`, `.sode-setup__label`, `.sode-setup__line`), web/setupwizard.js (`renderSteps`, `swatches`).
- **Reported:** "Stacked and clipping content." "View screenshot to see the design horror." "Add padding below this before the modal." The screenshots show the step names ("Style & User", "Router", "Access", "Done") wrapped one word per line and cut off by the dialog's top edge. The connector lines meet the step boxes at odd places, and the step boxes have a double border.
- **Now:** The class `sode-setup__chip` has two meanings. web/setupwizard.js:291 uses it for the step button (a column with a 5rem width, `all: unset`, web/setup.css:46-54). web/setupwizard.js:109 and :118 use it for the color swatch box. The swatch rule at web/setup.css:166-175 comes later with the same selector. It sets `display: grid`, `width: 2.25rem`, `height: 2.25rem`, a border and a background. So it overrides the step button: the button is 2.25rem wide and 2.25rem high, the label wraps in 2.25rem, and the 2rem dot sits inside a bordered box (the double border). The labels spill below the box and the next block covers them. The steps row has `margin-bottom: var(--pb-space-7)` (web/setup.css:34), which does not cover the spilled label. This looks like a regression from the swatch work, not a missing margin. The wait page (AB-08) has no steps row.
- **Expected:** Step names show in full on one line each, not stacked and not clipped. The dialog starts below the steps row with space between them.
- **Decisions:** Owner, 2026-11-27, after a Fable review: Scope the step-button rules (`.sode-setup__steps .sode-setup__chip`) or rename the swatch class, whichever keeps identAllow clean; one patchbay space token below the row, enough that the dialog never touches the labels.
- **Open questions:** none.
- **Proof:** Screenshot before and after at 1920x1080 of each step (Style & User, Router, Access). A TestChrome* check (extend TestChromeSetupWizard) that every `.sode-setup__label` has a bounding box inside the steps row and one line high, and that the row's bottom is above the dialog's top.
- **Notes:**

### AB-08: "Another browser took control" loses the answers with no warning

- **Status:** ready
- **Area:** Setup wait page. web/setupwizard.js (`waitPage`, `toWait`, `onMsg('setupTaken')`, `claim`), internal/app/setup.go (`setupClaim`, `setupDropLocked`).
- **Reported:** "Gee thanks for not warning me before my answers were lost! If they're in local storage, why are they lost?" Text: "Setup runs in another browser" → "Setup is running in another browser".
- **Now:** The answers are the in-memory object `d` (web/setupwizard.js:30). There is no localStorage or sessionStorage in web/setupwizard.js, so the owner's premise does not hold. The server also writes nothing to disk before Finish (internal/app/setup.go:13). When another browser presses Take control, `setupClaim` (setup.go:129-148) moves the owner, and sends `setupTaken` to the old owner. The old page runs `toWait(true)` (web/setupwizard.js:545-547, :517-523). It sets `d = null` and shows "Another browser took control. Your answers are lost." (web/setupwizard.js:493). The first browser gets no warning before the loss and no veto. The Take control button only warns in a tip (web/setupwizard.js:500). The title is set at web/setupwizard.js:491.
- **Expected:** The owner wants a warning before answers are lost, and answers that survive. Wants the title to read "Setup is running in another browser".
- **Decisions:** Owner, 2026-11-27, after a Fable review: Keep each tab's answers in sessionStorage (never PIN or Repeat), refill them when the tab gets control back, clear them on Finish and Restore. No Confirm on the taker, since nothing is lost. Change the tip (web/setupwizard.js:500) and the "Your answers are lost" line (:493) to say the answers stay in that browser. Heading text: "Setup is running in another browser". Why sessionStorage: localStorage is shared by two tabs of one browser and leaves a name and router address on a shared machine.
- **Open questions:** none.
- **Proof:** A TestChrome* check with two browser tabs: type answers in A, take control in B, assert A's wording and that the warning or the saved answers behave as decided. Node test for the title text if it moves to a rules module. Screenshot before and after of the wait page.
- **Notes:**

### AB-09: Tooltips clip at the edge of a modal, everywhere

- **Status:** ready
- **Area:** Tooltips. web/vendor/patchbay/css/patchbay.css:1951-2000 (`[data-pb-tip]::after`), web/base.css:452-485 (`--sode-tip-x`, `data-sode-tip-flip`, `data-sode-tip-float`), web/tooltip.js (`fitTip`, `bounds`, `ROWS`, floating `cutTip`), web/test/tips.test.mjs. Seen on `#setup-take` in `#setup-wait` (web/setupwizard.js:500).
- **Reported:** "Tooltips clipping off the modal. This is a systemic problem I have been trying to chase for many versions. Fix this globally immediately."
- **Now:** A patchbay tip is a CSS pseudo-element (`::after` with `position: absolute` under its element, patchbay.css:1955-1976). `.pb-modal` has `overflow: hidden` (patchbay.css:1347) and `.pb-modal__body` has `overflow: auto` (patchbay.css:1374). So a tip near the modal's edge is cut off at the body. Sode's patch is web/tooltip.js: on mouseover and focusin, `fitTip` measures the tip, uses `bounds()` (the rectangle of each ancestor with overflow other than visible, and the window) to shift it sideways, flip it, or hide it and show the floating `cutTip` (a fixed element on `document.body`, so only the window can clip it). Flip needs room of tip height plus 8px on one side; float needs room on neither. Other places with the same cause (a CSS tip inside a clipping box): the lists in `ROWS` (web/tooltip.js:19: `.sode__sets`, `.sode__staged-list`, `.sode-pages__list`, `.sode-palette__list`, already floated), and every other `data-pb-tip` in a `.pb-modal` or in one of the 42 `overflow: hidden/auto/scroll` rules in web/*.css. Examples: the wizard's field label tip (`field(..., tip)`, web/setupwizard.js:50), the Settings form, the Grid popup, the Palette. I could not run the page (no server), so I did not reproduce the clip on `#setup-take`. Note: the dark bubbles in the screenshots for notes 5, 6 and 8 end in "..." and carry the owner's note text, so they are the annotation tool's own bubble, not a Sode tip. The real Sode tip for Take control ("Discards the other browser's answers") is not visible in the capture.
- **Expected:** Tips never clip at a modal's edge, on every page. Fixed globally, not per tip.
- **Decisions:** Owner, 2026-11-27, after a Fable review: One fix in Sode: draw every `data-pb-tip` in a fixed top layer on `document.body` from web/tooltip.js, and hide the vendored CSS `::after` tip from web/base.css. Keep the `(hover: none)` and `:focus-visible` gates (web/tooltip.js:86-106). No `showModal` or popover is in use, so a body layer is never under another top layer. Do not patch web/vendor/patchbay; file an upstream patchbay issue.
- **Open questions:** none.
- **Proof:** Reproduce first with a screenshot at 1920x1080 of the clipped tip. Then a TestChrome* check that hovers `#setup-take` and a tip in the last row of a modal and checks the tip's box lies inside the window and is not cut by any ancestor. Extend web/test/tips.test.mjs for the new rule.
- **Notes:**

### AB-10: Setup top bar is not the global header

- **Status:** ready
- **Area:** Setup page header. web/setup.css:1-7, web/index.html:36-45 (`#home-bar`), web/bar.css (`.sode-bar`, `.sode-home-bar`:351-370), web/desktop.css:3-9 (`.sode__mark`), web/homebar.js, web/panelsbar.js.
- **Reported:** "This does not match the global style header."
- **Now:** In setup, `html[data-sode-setup]` hides every child of `#home-bar` except `.sode__conn` (web/setup.css:2-4), so the title, Inbox, Grid button, Settings button and links are gone. Only the lock chip, the Server and Router status, the address and the line under the bar remain (see the capture). The page's own title is a separate large centered `h1.sode__mark` "Sode" inside `#setup` (web/index.html:68; web/setup.css:13-16), not the bar's `.sode-panels__title`. On Home the bar's title is hidden (bar.css:360) and the big mark carries the name. On /panels, /log and Settings the bar shows the name and links. The bar is placed by `--sode-head-edge` (bar.css:351-358), while the setup dialog uses `width: min(100%, 76rem)` (web/setup.css:9-12), so their edges do not line up (the bar is 1160px wide at x=309; the dialog at x=293).
- **Expected:** The setup header looks like the global header. The owner gave no detail.
- **Decisions:** Owner, 2026-11-27, after a Fable review: Use the bar that Settings and /log use: site name on the left (live from the Server Name box), status on the right, nav links and Lock hidden (no user exists yet). Drop the big centered `h1.sode__mark`; line the dialog up with `--sode-head-edge`. The main page bar is not the model, since it hides the site name (bar.css:360).
- **Open questions:** none.
- **Proof:** Screenshots at 1920x1080 of the setup header next to the Settings or /log header. A TestChrome* check that the bar's left and right edges equal the dialog's, if that is the wish.
- **Notes:**

### AB-11: Theme color: the heading and label words, and the choices look too alike

- **Status:** ready
- **Area:** Style & User step. web/setupwizard.js (`VIEW.welcome` :187-189, `swatches` :101-126), web/accent.js (`pickerColors`, `CAUTION_NOTE`), web/base.css:401-425 (`[data-brand=...]`), web/index.html:343 (Settings label), GLOSSARY.md, docs/color-roles.md.
- **Reported:** "All the color options, these are all very similar." Text, final: heading "Server Name and Color" → "Sode Server Name and Theme Color"; label "Border color" → "Theme color". (Annotations 11, 12 and 13: 11 → "Sode Server Name and Color", then 12 → "Sode Server Name and Theme Color".)
- **Now:** The heading is `section('Server Name and Color', ...)` (web/setupwizard.js:189). The label is `'Border color'` (web/setupwizard.js:126). Settings still says "Page border color" (web/index.html:343). The choices are Off, Blue, Teal, Sky, Slate, Violet and Custom (from `pickerColors` of `S.brandColors`). Blue is `--pb-signal-07`, Teal `--pb-signal-06`, Sky a hand-set hue between them (OKLCH 215, base.css:407-410), Slate `--pb-signal-14`. Each swatch is a 2.25rem box with its name under it (web/setup.css:150-175). Blue, Teal, Sky and Slate are close in hue and lightness. The caution note says a color near a state color can look like a state. Docs call this color "Page border" (docs/color-roles.md:24); GLOSSARY.md has no entry for "theme".
- **Expected:** Heading reads "Sode Server Name and Theme Color". Label reads "Theme color". The colors are easier to tell apart.
- **Decisions:** Owner, 2026-11-27, after a Fable review: Words: "Theme color" everywhere, Settings too (web/index.html:343 "Page border color"), and docs/color-roles.md:24 and :66. GLOSSARY row: "Theme color" is the server accent; "Theme" stays the light/dark setting; "Page border color" and "Border color" are _Avoid_. Heading is the owner's "Sode Server Name and Theme Color" (title case kept as written). Colors: retire Sky and Slate with the existing `Retired` flag (internal/config/config.go:96-105), so saved values still load, draw and stay offered while saved (`pickerColors`, web/accent.js:164). Add at most one or two colors that pass an OKLCH distance check against each other and against the state hues (green, amber, Protected pink, red). Add a node test for that check.
- **Open questions:** none.
- **Proof:** Screenshot before and after of the Theme color row at 1920x1080 (light and dark). A node test or TestChromeSetupWizard assertion on the two texts. Color-difference check (OKLCH distance between neighbours) if the set changes.
- **Notes:**

### AB-12: Administrator line: wording, and it shows on more than one step

- **Status:** ready
- **Area:** Style & User step, "Add a first user" section. web/setupwizard.js:195-205 (`section('Add a first user', 'Administrator', ...)`), `sectionAs` :81-84, web/setup.css:135-139 (`.sode-setup__sub`).
- **Reported:** Text: "Administrator" → "This user will be the administrator." And: "This is duplicated across pages for some reason."
- **Now:** The word "Administrator" is the `sub` text of the "Add a first user" section, once, on the Style & User step only (web/setupwizard.js:197). No other step passes it. The Router step's sub is "Some routers support staging changes for taking many routes simultaneously." (web/setupwizard.js:232). Annotation 15's capture shows the new sentence under "Multi destination take behavior" on the Router step. That is most likely the annotation tool's in-place copy edit being applied again, by its selector (`p.sode-setup__sub`), to another `p` after the step changed. Annotation 14 (the Connection heading) came from the same page in the same state. So this is probably not a Sode bug. Not confirmed in a browser. Also in this edit, annotation 10's context text reads "This use will be the administrator" (a typo mid-edit); the wanted text has "user".
- **Expected:** The line reads "This user will be the administrator." It shows only under "Add a first user".
- **Decisions:** Owner, 2026-11-27, after a Fable review: Text becomes "This user will be the administrator." The second copy on the Router step is the annotation tool putting the copy edit on the same selector again; Sode shows the line once. Check the Router step after a clean reload with annotations off; if it is clean, no change for duplication.
- **Open questions:** none.
- **Proof:** Node or TestChromeSetupWizard check that the sentence appears once on the Style & User step and on no other step. Screenshot of the Router step after a clean reload.
- **Notes:**

### AB-13: Name box should be a third width, same as the PIN and Repeat boxes

- **Status:** ready
- **Area:** Style & User step. web/setupwizard.js:195-205 (`field('setup-name', ...)`, `row(...)`), web/setup.css:122-128 (`.sode-setup__row`), web/base.css:1445 (short-value rule).
- **Reported:** "Make a third width, the same as the PIN and Repeat boxes." (on the Name box of "Add a first user")
- **Now:** The Name field is outside any row, so it fills the whole width (web/setupwizard.js:199; capture shows it as wide as the dialog). The PIN and Repeat your PIN fields are in `row(...)`, a two-column grid `1fr 1fr` (web/setup.css:123-128), so each is half width, not a third. The row comment says an empty second cell keeps a lone field at half width. The Router step's Levels and Router title also use half width.
- **Expected:** The Name box is a third of the width, the same width as the PIN and Repeat boxes.
- **Decisions:** Owner, 2026-11-27, after a Fable review: Name, PIN and Repeat sit in one row, a third each.
- **Open questions:** none.
- **Proof:** Screenshot before and after at 1920x1080. A TestChromeSetupWizard assertion that the widths of `#setup-name`, `#setup-pin` and `#setup-pin2` are equal within a pixel.
- **Notes:**

### AB-14: Connection heading names the first router

- **Status:** ready
- **Area:** Router step. web/setupwizard.js:208-215 (`section('Connection', ...)`), `VIEW.router` :207-249.
- **Reported:** Text: "Connection" → "Connection: Add your first router".
- **Now:** The section heading is the plain word "Connection" (web/setupwizard.js:210), above the Address field and the status line. The step is titled "Router setup" and has a "Router" section above it (the Router title field, :208).
- **Expected:** The heading reads "Connection: Add your first router".
- **Decisions:** Owner, 2026-11-27, after a Fable review: "Connection: Add your first router" on first run only. Run setup again keeps "Connection", since it edits an existing router.
- **Open questions:** none.
- **Proof:** Node or TestChromeSetupWizard text check on the Router step. Screenshot of the Router step.
- **Notes:**

### AB-15: Multi destination take behavior needs a help icon with a tooltip

- **Status:** ready
- **Area:** Router step. web/setupwizard.js:229-247 (`sectionAs('setup-fireMode-label', 'Multi destination take behavior', ...)`, `choice('fireMode', ...)`), web/importremap.js:152 (a help icon with a tip), web/vendor/patchbay/icons, web/tooltip.js, docs/agents/salvos.md.
- **Reported:** "Add question mark icon with tooltip explaining the difference."
- **Now:** The heading is a plain `h3` with the sub text "Some routers support staging changes for taking many routes simultaneously." The two choices are "One after another" (`sequential`) and "Router salvo" (`native`). Nothing explains the difference between them. The only help icon pattern in the code is `icon('circle-help')` with `pb-tip--end` (web/importremap.js:152). The facts for a tip are in docs/reference.md:115 and docs/agents/salvos.md: sequential sends one connect after another (about 16 ms each on the Ultrix); native uses the router's own salvos, a group send that needs a router that supports it, and a salvo group number that nothing else uses.
- **Expected:** A question-mark icon by the heading, with a tooltip that explains the difference between the two choices.
- **Decisions:** Owner, 2026-11-27, after a Fable review: A real focusable button (`pb-btn--ghost pb-btn--icon`, help icon) after the heading, with a `data-pb-tip` that says what each mode does. The same button goes next to the setting in Settings > Routers (web/index.html:524). The only help icon today (web/importremap.js:152) is a span that takes no focus; do not copy it. The agent writes the tip in STE; the owner checks the words at review.
- **Open questions:** none.
- **Proof:** Screenshot of the Router step with the tip open at 1920x1080. TestChromeSetupWizard check that the icon exists, is focusable, and its tip is not clipped (ties to AB-09). ste_test passes on the new words.
- **Notes:**

### AB-16: Access step: less spacing, alternating row colors

- **Status:** ready
- **Area:** Access step. web/setup.css:207-257 (`.sode-setup__sheet`, `.is-picked`), web/setupwizard.js:128-150 (`sheet`), web/vendor/patchbay/css/patchbay.css:1579 (`.pb-table tbody tr:nth-child(even)`).
- **Reported:** "Reduce spacing within modal and offer alternating line colors for easier reading."
- **Now:** Cells have `padding: var(--pb-space-3) var(--pb-space-4)` and a 1px bottom border each (web/setup.css:212-217). The modal body has `padding-top: var(--pb-space-5)` (web/setup.css:25-27). The table is custom (`.sode-setup__sheet`), not `.pb-table`, so it gets no row striping. The picked column has an accent wash (`.is-picked`, :243-245), which a row stripe would cross. The capture shows 7 rows (including the header) at about 55px each.
- **Expected:** Less spacing in the modal, and alternating line colors on the rows.
- **Decisions:** Owner, 2026-11-27, after a Fable review: About half the row padding and the gap above the table. Stripes from a Sode rule on `.sode-setup__sheet tbody tr:nth-child(even)` with a patchbay surface token, since patchbay 2.2.1 has no stripes (patchbay.css:1577-1579 gives odd and even rows one color). The picked column's wash stays on top of the stripes.
- **Open questions:** none.
- **Proof:** Screenshot before and after at 1920x1080 (light and dark). A TestChromeSetupWizard check on row height and that even rows have a different background from odd rows.
- **Notes:**

### AB-17: Done page text

- **Status:** ready
- **Area:** Done step. web/setupwizard.js:155-167 (`doneText`), :259 (`#setup-done-text`), :359-362 (`paintDone`), web/setup.css:269-272.
- **Reported:** Text: "You are signed in as Seth and connected to the router at 127.0.0.1:8910. You chose Open access." → "Congratulations!</br>You are signed in as Seth and connected to the router at 127.0.0.1:8910 with Open access permissions." Wanted text, as two lines:
  - Congratulations!
  - You are signed in as Seth and connected to the router at 127.0.0.1:8910 with Open access permissions.
- **Now:** `doneText()` returns one run of text nodes: "You are signed in as" (name in bold), "and connected to the router at" (address in mono), then "You chose" (preset name in bold). If the router has not answered it says ". The router at <addr> has not answered yet, and Sode keeps trying." The tag `</br>` in the owner's edit is not valid HTML; it means a line break. The new text has no "You chose" clause, so the "not answered" branch needs its own wording for the permissions part.
- **Expected:** Two lines: "Congratulations!" then the sentence ending in "with Open access permissions."
- **Decisions:** Owner, 2026-11-27, after a Fable review: Two lines: "Congratulations!", then "You are signed in as **Seth** and connected to the router at `addr` with Open access permissions." Keep the bold name and mono address. The full preset name as GLOSSARY.md:251 says ("Standard access permissions", "Secured access permissions"). Router not answered yet: "You are signed in as **Seth** with Open access permissions. The router at `addr` has not answered yet, and Sode keeps trying."
- **Open questions:** none.
- **Proof:** Node test of `doneText` (or a rules function) for linked and not-linked cases. Screenshot of the Done page at 1920x1080.
- **Notes:**

### AB-18: Done page next links: remove backup and tablet links, add useful ones

- **Status:** ready
- **Area:** Done step, Next steps list. web/setupwizard.js:184 (`nextLink`), :262-274, :394-402 (click handler), web/index.html (`fieldset[data-section="level-colors"]`:377, `routers`:402, `users`, `api`, `export-import`), web/setup.css:273-295.
- **Reported:** On "Download a backup": "Remove this, why would somebody need to download a backup of a completely brand new system?" On "Connect a tablet or another app": "This doesn't even make sense. Remove it." On the list: "Add more sensible options like adjust level colors and add another router."
- **Now:** The list has four links: "Add more users" (Settings section `users`), "Create your first panel" (`panels/editor`), "Connect a tablet or another app" (section `api`), "Download a backup" (section `export-import`) (web/setupwizard.js:269-272). A click on a link with a section opens Sode, emits `settingsOpen`, and scrolls to `fieldset[data-section="..."]` (web/setupwizard.js:394-400). Settings has sections `level-colors` (web/index.html:377) and `routers` (:402) that could take the new links. The router section's Add router is in Settings > Routers (settings redesign).
- **Expected:** Remove "Download a backup" and "Connect a tablet or another app". Add sensible links, named by the owner: adjust level colors and add another router.
- **Decisions:** Owner, 2026-11-27, after a Fable review: Remove "Download a backup" and "Connect a tablet or another app". List in this order: Add another router, Adjust level colors, Add more users, Create your first panel. Each opens Settings at its section; "Add another router" scrolls to `routers` and does not open the add flow (that would start an unsaved router).
- **Open questions:** none.
- **Proof:** TestChromeSetupWizard check: the Done list has no backup or tablet link, and each new link opens Settings at its section (level-colors, routers). Screenshot of the Done page at 1920x1080.
- **Notes:**

## Top bar, Settings > Routers and main page

### AB-19: Lock action belongs in the Lock drop-down

- **Status:** ready
- **Area:** Lock bar: `web/lockchip.js` (`buildChip`, `lockBtn`, `more`, `menu`, `item`), `web/lock.css` (`.sode-lock__who-*`). Same actions in Settings > My user (`web/changepin.js`).
- **Reported:** "Add the lock action to the lock dropdown. It is not immediately clear that clicking lock is a separate action." (annotation 22; the screenshot is the open-lock icon.)
- **Now:** The open-lock icon is its own button; a click calls `api.lock()` and the tip says "Lock (Ctrl+L)" (`lockchip.js:19-24`). The drop-down is opened by a separate chevron (`lockchip.js:29-36`) or by hover and focus. It holds only Sign out (`lockchip.js:58-61`) and Step in or Step out (`lockchip.js:62`, shown by `open()` at lines 93-99). Lock is not an item.
- **Expected:** Lock is an item in the Lock drop-down, so it is clear that Lock is a separate action.
- **Decisions:** Owner, 2026-11-27, after a Fable review: One menu button: the Lock icon and the chevron are one button that opens the menu. "Lock" is the first item with its Ctrl+L hint, then Sign out. No one-click lock on the icon; Ctrl+L stays the fast path. Rewrite the comment at web/lockchip.js:28 and update TestChromeChipMenu.
- **Open questions:** none.
- **Proof:** Screenshot of the open drop-down before and after at 1920×1080. Extend `TestChromeChipMenu` (`internal/app/chipmenu_browser_test.go`): the Lock item exists, locks the page and keeps the Ctrl+L hint.
- **Notes:**

### AB-20: Status bar tooltips no longer show router and server status with history

- **Status:** ready
- **Area:** Status bar `#conn` (`web/index.html:40`): `web/connview.js` (`paintConn`, `paintRouters`), `web/statusinfo.js` (`statusInfo`, `makeBox`, `show`, `strip`), `web/uptimebars.js`, `web/statusinforules.js`, `web/routerlinkrules.js`. Test: `internal/app/statusinfo_browser_test.go`.
- **Reported:** "There's an apparent and horrible regression where tooltips no longer show status of routers and server with history. We spent serious time developing that and I need it back immediately." (annotation 26)
- **Now:** The hover details are gone on `settings/redesign`. `statusInfo(root, open)` (`connview.js:22`) now builds a click popover, "Router list". Only the Routers button (`paintRouters`, `connview.js:28-41`) opens it, by click, with `toggle` (`statusinfo.js:288-296`). The list holds Server link, Server run time, and one row per router. A row opens on a click to show the address, the error and the last 24 hours (`statusinfo.js:110-114`, `136-179`). Nothing opens on hover or focus of the Server or Router status. The Server status has no action at all. The popover also needs a row click for the history, and it asks the history only for an open row (`statusinfo.js:226`).
  - Built (both on `origin/integration/v2`): `93404764` (2026-11-22) "Status details: Server run time, Router link and 24-hour strip on hover or focus" (new `statusinfo.js` with `pointerenter` and `focusin`, replaces the native `title`); `8d7ca2ad` (2026-11-22) "Status: Server and Router icons with state badges, ready for more routers" (Router link row per router). `e37d8f98` (2026-11-23, #784) gave each router its own 24-hour strip on the hover details.
  - Removed: `e9dbe236` (2026-11-26, "Status bar: one Routers icon that opens the router list"; message says "It replaces the hover details"). It rewrote `statusinfo.js` (316 lines changed), `connview.js`, `bar.css`, `degradedview.js`, `mobile.js` and `statusinfo_browser_test.go`. Follow-ups on the click list: `f53cc176`, `81ce61c0`, `0517bb8a`.
  - Branches: `origin/integration/v2` still has the hover tooltip with history (`git show origin/integration/v2:web/statusinfo.js` has `pointerenter`, `focusin`, LEAVE_MS). It is not in `origin/main` (neither `93404764` nor `8d7ca2ad` is an ancestor of main). `origin/settings/redesign`, `redesign-p6server`, `redesign-tielines`, `phase0` and `0c-statusbar` have the click list; `0a-names-safety` and `0b-protects` have the hover version.
- **Expected:** Hovering or focusing the Server and Router status shows the status of the server and the routers with their history again, as before.
- **Decisions:** Owner, 2026-11-27, after a Fable review: Hover or focus on the Routers and Server status shows a box that never scrolls: Server run time, then routers with their 24-hour strip, lost routers first, at most about 8 rows, then "and N more" that points to the click list. The click list stays for the full set and its actions. Build from the v2 code (93404764, 8d7ca2ad, e37d8f98 on `integration/v2`). No scroll bar in a hover box: it closes when the pointer leaves, the fault RECOMMENDATIONS.md:84 recorded.
- **Open questions:** none.
- **Proof:** Screenshots before and after at 1920×1080 with one and with three simulators. Restore or extend `TestChromeStatusInfoWithLogRights` and `TestChromeStatusInfoWithoutLogRights` (`internal/app/statusinfo_browser_test.go`; the v2 versions test hover) and keep `TestChromeRouterListManyRouters`. Node test for any new rule in `statusinforules.test.mjs`.
- **Notes:**

### AB-21: Level chips: same spacing for one- and two-digit levels

- **Status:** ready
- **Area:** Level chips of the router detail (`web/settingsrouterdetail.js:194-199`, `#router-chips`), `web/routers.css:214-223` (`.sode__rchips > .sode__slot`), `web/base.css:492-509` (`.sode__slot`). The same `.sode__slot` chip is in the History level bar and the Grid panel.
- **Reported:** "Ensure the same spacing between two-digit and single-digit number levels." (annotation 23; the screenshot shows the chip 15 wider than the single-digit chips above it.)
- **Now:** Each chip has `min-width: 1.5rem` and `padding: 0 var(--pb-space-1)` (`routers.css:219-223`), so a one-digit chip is exactly 1.5rem and a two-digit chip grows past it (25.2 px against 24 px in the annotation). The row of chips wraps, so the columns do not line up.
- **Expected:** One- and two-digit levels have the same spacing.
- **Decisions:** Owner, 2026-11-27, after a Fable review: Fix only `.sode__rchips > .sode__slot` (web/routers.css:219-223): drop the side padding and give the row one chip width from its widest label. The other slots are already fixed width (base.css:787-793). Levels go to 256, so the width comes from the labels, not a two-digit guess.
- **Open questions:** none.
- **Proof:** Screenshot before and after at 1920×1080 with 17 levels. A browser check (new `TestChrome*` name) that reads the widths of `#router-chips > .sode__slot` for levels 1 and 15 and expects them equal.
- **Notes:**

### AB-22: Routers list squeezes the router detail; put the A/B router selector at the top and give settings full width

- **Status:** ready
- **Area:** Settings > Routers: `web/index.html:418-420` (`#routers-panes`, `#routers-list`, `#router-detail`), `web/routers.css:26-60` and `246-258` (`.sode__rt`, `.sode__rlist`, the `@container (min-width: 36rem)` two-column grid `16rem minmax(0, 1fr)`), `web/settingsrouterlist.js` (rows, find, Up and Down keys), `web/bigscroll.js:18` (`.sode__rlist` in SCROLLERS).
- **Reported:** "Overall, this feels much better than before, but squishing the router config even tighter doesn't seem reasonable. Maybe we can put the AB selector at the top of the list and leave the full size for settings." (annotation 24)
- **Now:** From 36rem of section width, the list is a 16rem column on the left with the detail view in the rest (`routers.css:246-250`). The list stays sticky (`routers.css:251-254`). Below that width the list sits above the detail. In the annotation the detail fields are 640 px wide and the long fields wrap in one column pair.
- **Expected:** The router selector (A, B) sits at the top of the list, and the settings use the full width.
- **Decisions:** Owner, 2026-11-27, after a Fable review: A row of router chips across the top of Settings > Routers, wrapping as needed; the detail gets the full width. No bare state dot ("status always with text"): a chip shows state words only when the router is not connected ("B Stage 2, Lost"). The first chip keeps its Default mark; Edited and New marks show on the chip. Find and the problems filter sit at the end of the row. Left and Right move between chips. Reorder stays in the detail head. Take `.sode__rlist` out of SCROLLERS if the row wraps. Phone layout unchanged.
- **Open questions:** none.
- **Proof:** Screenshots before and after at 1920×1080 with 2 and with 24 routers. Browser checks: `TestChromeSettingsRouterReorder`, `TestChromeSettingsAddRemoveRouter` still pass; a new `TestChrome*` that checks the detail is full width and the selector is above it. Node test for any new list rule (`routerlistrules.test.mjs`).
- **Notes:**

### AB-23: Address field: try the link while typing and show its state before Save

- **Status:** ready
- **Area:** Router detail Connection group: `web/index.html:453-454` (`#cfg-rt-address`), `web/settingsrouterdetail.js` (`check` at 432, `paintHead` at 216, `addRouter` at 475, `errorsOf`), SW-P-08 client (`internal/app/routerunit.go:108`, `internal/app/settings.go:256`).
- **Reported:** "Attempt and show connection status immediately when typing in IP addresses so the user knows if they've done it correctly before saving." (annotation 25; the screenshot shows `localhost:9312` typed in the Address field with no state.)
- **Now:** Typing only checks the format of the address (`check` on focusout, `settingsrouterdetail.js:432-447`). A new router says "Sode connects to this router after you save." (`settingsrouterdetail.js:219`). The link state of a saved router comes from the live router unit (`linkOfRouter`), so Sode connects only after Save. No code path tries an address that is not saved: `grep` finds no dial or test message outside the router unit and its keepalive.
- **Expected:** While the person types an address, Sode tries to connect and shows the connection status at once, so the person knows the address is right before Save.
- **Decisions:** Owner, 2026-11-27, after a Fable review: Copy the wizard: its 600 ms pause (web/setupwizard.js:343-351) and its words (`routerLine`, web/setupwizardrules.js:127-131: "Connected: N sources and M destinations discovered." / "No answer yet."). The server does a real SW-P-08 name read, since a TCP connect to a wrong port also succeeds. Administrator only, as the Routers topic is (internal/app/settings.go:271). The test dials apart from the live link, one test at a time, and the next key cancels it. Correction to Now: the wizard already tries an unsaved address (`tryRouter` sends `setupRouter`, internal/app/setup.go:169-190), but it moves the live link, which this test must not do.
- **Open questions:** none.
- **Proof:** Browser check (new `TestChrome*`) against `swp08sim`: type its address into a new router, see Connected before Save; type a closed port, see the failure. Go test for the test request (reply states and the access rule). Screenshot of each state at 1920×1080.
- **Notes:**

### AB-24: Last chip too small to show its text

- **Status:** ready
- **Area:** Last and Next chips in a box head: `web/desktop.js:267-282` (chips), `web/base.css:1754-1783` (`.pb-chip.sode__last`, `.sode__last-text`), `web/base.css:1723-1745` (`.sode__box-head`, `[data-last]`), `web/index.html:130` (Levels head).
- **Reported:** "This box is currently too small to be useful as it can't even display the text that it holds." (annotation 27; the Last chip of the Levels box, text "Last 0–10", shows "Last C".)
- **Now:** The Levels head is one flex row of the label "Levels", the "Router A" text (`.sode__lvl-router`, `index.html:130`) and the `[data-last]` slot. The chip may shrink to nothing: `min-width: 0`, `max-width: 100%` on `.pb-chip.sode__last`, and `.sode__last-text` clips with an ellipsis (`base.css:1780-1785`). The Levels box is the narrow box of its group, so the chip shrinks to 50.5 px. The Router A text and the label take the room first (`base.css:1732-1745`, `.sode__box-router { flex: none }`).
- **Expected:** The Last chip shows the text it holds.
- **Decisions:** Owner, 2026-11-27, after a Fable review: Give the Last chip a minimum width that fits "Last 0–10"; hiding the Levels head's router label (AB-25) gives the room. A longer value is cut with an ellipsis; `cutOff` in web/tooltip.js:23-36 already shows the full text in a tip.
- **Open questions:** none.
- **Proof:** Screenshot before and after at 1920×1080 with two routers. A browser check (new `TestChrome*`, or an extension of `TestChromeMainNextChipNoKeys`) that `scrollWidth <= clientWidth` for `.sode__last-text` in the Levels head.
- **Notes:**

### AB-25: Router button goes under the left side of each Source and Destination box, out of Levels; Levels follow the Source box

- **Status:** ready
- **Area:** Main page boxes: `web/index.html:93` (`#src-router`), `110` (`#dst-router`), `130` (`#lvl-router`); `web/boxrouter.js` (`paintBoxRouter`, `wireBoxRouter`, `paintLevelsRouter`); `web/desktop.js:114-121` and `:660-661` (`lvlRouter`, `levelRouterOf`), `web/levels.js:131-134` (`levelRouterOf`); `web/base.css:1732-1743` (`.sode__box-router`, `.sode__lvl-router`); `web/desktop.css:98` (head height with a router button).
- **Reported:** "Move this to the under-left side of each source and destination box. Remove it from levels altogether. Levels should follow the source box." (annotation 28; the pointer is on the Destination's "Router A" button.) Then, on the Levels head's "Router A" label: "I see what you mean, I guess, since this is a read-only box, we can just hide it all together and make more room for the last levels." (annotation 29)
- **Now:** The Router button is in the head of the Source and Destination boxes, between the label and the Last and Next chips (`index.html:93`, `110`), and shows only with two or more routers (`boxrouter.js:27`). The Levels head has a text "Router A" (`#lvl-router`, `index.html:130`), not a button. It names the router of the typed levels, which today follows the first destination, else the router the destination text names, else the router of the source (`levels.js:128-134`).
  - Age: not in `origin/integration/v2` or `origin/main` (no `box-router`, `src-router` or `lvl-router` there). The Source and Destination buttons came with the multi-router work (`d321cbff`, 2026-11-23, "Routers 5: Prefix typing"), on this branch. The Levels text `#lvl-router` is new in this redesign: `3710fe22` (2026-11-27).
- **Expected:** The Router button sits under the left side of each Source and Destination box. The Levels box has no router text, and the Last chip uses the room.
- **Decisions:** Owner, 2026-11-27, after a Fable review: Each Router button moves to a line under the left side of its own box (Source, Destination). The read-only router label in the Levels head (`#lvl-router`) goes away, to give the Last chip room (owner's annotation 29: "since this is a read-only box, we can just hide it all together and make more room for the last levels"). Which router the levels come from does not change (still the destination, docs/tielines-design.md Levels rule 3); the owner does not mind which.
- **Open questions:** none.
- **Proof:** Screenshot before and after at 1920×1080 with two routers and with one. Browser check (new `TestChrome*`, near `TestChromeMainNextChipNoKeys`): the Router button of each box is below its input, left-aligned, and `#lvl-router` is gone; the level chips follow the rule that the owner picks. Node tests for `levelRouterOf` in `routerlevelrules.test.mjs` if the rule changes.
- **Notes:**
