# Findings B2

Slice: every Settings section except Routers, Tie lines and Protects. Code read at /Users/seth/Desktop/sode/.claude/worktrees/review-shots (origin/alpha-round2). Numbers are CSS px at 1920x1080 DPR1, read from bounding boxes in a live copy on port 44080 (seed `full`, plus `envonly`). At this viewport 1rem = 16px, so `--pb-space-5` = 12px, `--pb-space-3` = 6px, `--pb-space-2` = 4px. All servers I started are stopped. Measurement scripts and raw dumps are in /private/tmp/claude-501/-Users-seth-Desktop-sode--claude-worktrees-happy-branch-merge-review-0c348b/1b1c74a8-cf3d-4546-aa99-2582e71b4297/scratchpad/review/B2work (m1.txt page, m2.txt and m3.txt dialogs).

Not checked: phone layout, 200% zoom, the Import tab with a file loaded, and the setup wizard steps behind "Run setup again" (only the entry row).

---

### B2-01: Access preset badges break the Locks grid
- **Where:** Access > Locks after "Apply an Access preset" > Stage changes; 06-access-apply-preset-staged-1080.png, -secured-, -standard-
- **Kind:** bug
- **Severity:** high
- **Problem:** `paintPresetMarks` inserts the badge after the `.sode__field` or `.pb-check` (`at.after(mark)`). After a `.sode__field` inside `.sode__fields` the badge becomes a third grid item: the grid goes from 2 columns of 314px to 3 of 205.3px and the badge stretches to a grey box of 205.3 x 117.6px at x=1063.5. After the "Main page" check the badge starts at x=712.4, which is exactly the label's right edge (0px gap), and sits 10px lower than the check row (y=391.0 against 380.9). It also pushes "Communication Log" to a new line. `.sode__preset-mark` has no CSS rule anywhere.
- **Change:** web/settingssave.js:179-181. Current: `if (src.node) at.after(mark); else at.append(mark);`. New: put the badge inside the label: `const host = at.matches('.pb-check') ? at.querySelector('.pb-check__label') : at.querySelector(':scope > label, :scope > .sode__label') || at; host.append(mark);`. Add to web/base.css next to `.sode__refused`: `.sode__preset-mark { margin-inline-start: var(--pb-space-3); vertical-align: middle; }`.
- **Also check:** `.sode__refused` notes use the same `after()` at settingssave.js:164 and will also become grid items inside `.sode__fields` (Branding, Confirm and results, Locks). Give them `grid-column: 1 / -1`.

### B2-02: The Not saved banner covers the Settings search box
- **Where:** env-only mode, Settings at scroll top; 15-envonly-first-view-1080.png
- **Kind:** bug
- **Severity:** high
- **Problem:** `.sode__notsaved` is sticky with z-index 100 at y=108 to 142. `.sode__toc` is `position: fixed; top: calc(10vh + var(--sode-down-h))` = 108, and `--sode-down-h` counts only fixed banners (core.js `publishDownHeight`), so it is 0. The search input (y=108 to 140) is fully under the banner: `elementFromPoint` on the input returns the banner text. The sidebar list then seems to start with no search box, and the Settings title sits 54px lower than the sidebar top.
- **Change:** web/core.js `publishDownHeight`: count a showing sticky banner too, or (smaller) in web/base.css:908 add the banner height: set `--sode-notsaved-h` from notsaved.js (`b.offsetHeight` when shown, 0 when hidden) and use `top: calc(10vh + var(--sode-down-h, 0px) + var(--sode-notsaved-h, 0px))`.
- **Also check:** any other in-flow banner above the form (`.sode__noadmin`, `.sode__broken`) for the same overlap.

### B2-03: Manage users rows do not fit: clipped buttons and role select
- **Where:** Manage users > Users tab; 08-users-manage-users-tab-1080.png, 08-users-bad-user-rows-1080.png, 08-users-remove-row-1080.png
- **Kind:** alignment
- **Severity:** high
- **Problem:** The 9-column grid needs more than the 936px body. "Sign out everywhere" is 89.5px wide with 106px of content: it shows as "n out everywh". The role select is 103.5px and shows "Administr…", "Standard …". After the first click on Remove the button text becomes "Confirm remove" in a 60.6px column and shows as "onfirm remov". A new row has no Sign out button, so its Remove floats with a hole before it.
- **Change:** web/lock.css:461 `width: min(60rem, …)` to `min(68rem, calc(100vw - var(--pb-space-8) * 2))`. web/lock.css:492 `grid-template-columns: minmax(8rem, 1fr) minmax(8rem, 1fr) repeat(7, auto)` to `minmax(8rem, 1fr) minmax(7rem, 1fr) max-content repeat(6, max-content)` and add `.sode__users-row .pb-btn { white-space: nowrap; }`. Reserve the Remove column for its longer text: in users.js:176-187 keep the button label "Remove" and show the confirm state with the footer note already there plus `pb-btn--danger`, or set `min-width: 8rem` on that button. Shorten users.js:198 `'Sign out everywhere'` to `'Sign out all'` with tip "Signs this user out of every browser".
- **Also check:** the same table below 50rem, where rows wrap.

### B2-04: Profile dialog has large empty blocks
- **Where:** My profile and Edit Profile; 08-my-user-profile-1080.png, 08-users-edit-profile-trusted-1080.png, 08-users-people-edit-profile-1080.png
- **Kind:** spacing
- **Severity:** high
- **Problem:** `.sode__profiles-body` is an auto-fit grid and User defaults spans 2 rows. The one-line intro is a `p.pb-note`, but the full-width rule names only `.sode__hint` (lock.css:564), so the note takes a cell and stretches to 386.5px tall. "Presets" starts at y=480 and "Filter sets" at y=949 with nothing between. In the 3-column case (Trusted) the note sits alone in column 1 and Filter sets starts 430px lower.
- **Change:** web/lock.css:559-570. New: `.sode__profiles-body { align-items: start; display: grid; gap: var(--pb-space-3) var(--pb-space-7); grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); } .sode__profiles-body > :is(.sode__hint, .pb-note) { grid-column: 1 / -1; } .sode__profiles-sec--prefs { grid-column: 2; grid-row: 2 / span 4; }` and keep the one-column media rule. Presets, Filter sets and Broken then stack in column 1 next to User defaults.
- **Also check:** the "Swap…" button touches the "No Broken or Name moved items" line (0px gap, y=1087.6): add `margin-top: var(--pb-space-3)`.

### B2-05: My user buttons: row 1 is spread, row 2 is packed
- **Where:** Preferences > My user; 01-preferences-my-user-buttons-crop-4k2x.png, 01-preferences-my-user-1080.png
- **Kind:** spacing
- **Severity:** high
- **Problem:** Row 1 gaps are 26.8px, row 2 gaps are 12px. `.sode__names--start { justify-content: flex-start }` is at base.css:1528, before `.sode__names { justify-content: space-between }` at base.css:1547, so the modifier loses. Row 2 looks right only because Lock's `margin-left: auto` takes the free space.
- **Change:** web/base.css: move the `.sode__names--start` rule below `.sode__names` (after line 1553).
- **Also check:** every other `.sode__names--start` user (grep shows the class is set in index.html:267 only).

### B2-06: Wrapped check rows are indented 12px and gaps differ
- **Where:** Access > Locks > Page locks, Access > API > Keyless scopes, Users > New user defaults; 06-access-locks-1080.png, 06-access-api-1080.png
- **Kind:** alignment
- **Severity:** high
- **Problem:** base.css:1970 `.sode__field [role="group"] .pb-check + .pb-check { margin-left: var(--pb-space-5) }` also applies to a check that wraps to a new line: "Panels", "Panel layouts" and "Set and free Sode's Protects" start at x=640.8, the first line starts at 628.8. In `.sode__scopes` the margin adds to the flex gap, so scopes are 24px apart while Page locks are 15.1px apart and the same scope list in My API clients is 12px apart.
- **Change:** web/base.css:1970-1972: delete the rule. Add `.sode__field [role="group"] { display: flex; flex-wrap: wrap; gap: var(--pb-space-2) var(--pb-space-5); }`.
- **Also check:** `.sode__preset-users` (B2-07), Manage users `.sode__users-opts .sode__checks`.

### B2-07: Access preset dialog: labels touch
- **Where:** Apply an access preset; 06-access-apply-preset-dialog-1080.png
- **Kind:** spacing
- **Severity:** medium
- **Problem:** `.sode__preset-users` has no CSS. "Administrator [You]" ends at x=820.9 and the Trusted check starts at 820.9; Trusted ends at 889.4 and Operator starts at 889.4. The two fields also touch: the preset buttons end at y=518.0 and the label "SET NEVER SIGN OUT AND NO AUTO-LOCK FOR" starts at 518.0. "Tick all" is a 576px wide button for a 2-word action.
- **Change:** web/lock.css, add: `.sode__preset-users { display: flex; flex-wrap: wrap; gap: var(--pb-space-2) var(--pb-space-5); } .sode__preset-users .pb-badge { margin-inline-start: var(--pb-space-3); }`. web/accesspreset.js:26-32: pass the body as one column with a gap (`el('div', 'sode__keys-new', pickBox, userBox)`), and put Tick all in a `sode__users-acts` div so it keeps its text width.
- **Also check:** B2-06, same cause family (check groups with no gap rule).

### B2-08: Preferences grid: 4px column gap, label touches the select
- **Where:** Preferences > This browser; 01-preferences-this-browser-fields-crop-4k2x.png, 01-preferences-this-browser-4k.png
- **Kind:** spacing
- **Severity:** medium
- **Problem:** The grid asks for a 12px column gap (base.css:1998), but each row is a subgrid and `.sode__field { gap: var(--pb-space-2) }` replaces it. "Searching history respects filters" ends at x=824.4 and its select starts at 828.4 (4px). The select ends at 1142.3 and "Save as my default" starts at 1146.3 (4px).
- **Change:** web/base.css:2001-2006: add `column-gap: var(--pb-space-5);` to `:root:not([data-layout="phone"]) .sode__prefgrid > .sode__field`.
- **Also check:** the Guest view (13-guest-first-view-1080.png), where the third column is empty and the select grows to 360px; give the select column a fixed `22rem`.

### B2-09: Half-width fields end 6px past the two-column grid edge
- **Where:** Look and feel > History, Access > Locks (Guest role), API (Rate limit, API takes), Panels; 02-look-history-1080.png, 06-access-locks-1080.png
- **Kind:** alignment
- **Severity:** medium
- **Problem:** `.sode__field--half` controls are `width: 50%` = 320px, right edge 948.8. The first column of a `.sode__fields` grid in the same section is 314px, right edge 942.8. In Locks the Guest role select is 6px wider than the Page locks column above it; in History the Hidden destinations box is 6px wider than "Logo text" in Branding above.
- **Change:** web/base.css:1447-1449: `width: 50%` to `width: calc(50% - var(--pb-space-5) / 2)`.
- **Also check:** Routers > Status label (other slice) uses the same class.

### B2-10: Sidebar: sections are indented less than their topic
- **Where:** Settings sidebar, every shot
- **Kind:** alignment
- **Severity:** medium
- **Problem:** Topic text starts at x=428.8 (`.pb-tab` padding `--pb-space-7`), section text at 426.8 (`padding-inline-start: var(--pb-space-6)`, base.css:1369), router text at 432.8. A child sits 2px left of its parent, so the tree reads as a flat list with a wobble.
- **Change:** web/base.css:1368-1375: `.sode__toc .sode__toc-sec { padding-inline-start: var(--pb-space-9); }` and `.sode__toc .sode__toc-router { padding-inline-start: calc(var(--pb-space-9) + var(--pb-space-5)); }`.
- **Also check:** the tabs are inset 4px in the 200px box (x=412.8) while the search input starts at 408.8.

### B2-11: Search list: selected row shifts 4px, "No match" moves the sidebar
- **Where:** Settings search; 09-search-router-1080.png, 09-search-nomatch-1080.png, 09-search-keyboard-pick-1080.png
- **Kind:** alignment
- **Severity:** medium
- **Problem:** The selected hit gets `border-inline-start` (base.css:875), so its text starts at x=420.8 and the other rows at 416.8; text jumps sideways as the arrow keys move. "No match" is in the flow: the nav moves from y=140.0 to 172.5 when it shows.
- **Change:** web/base.css:875-878: replace the border with `box-shadow: inset var(--pb-accent-bar-w) 0 0 0 var(--pb-accent);` and add `background: var(--pb-surface-control);`. For the hint, add `#settings-find-none { background: var(--pb-surface-high); border: var(--pb-border-w) solid var(--pb-edge); inset-inline: 0; margin: 0; padding: var(--pb-space-2) var(--pb-space-3); position: absolute; z-index: 2; }` so it draws where the list draws.
- **Also check:** none.

### B2-12: Rights tab: fields touch, two gap rules
- **Where:** Manage users > Rights; 08-users-rights-tab-1080.png, 08-users-edit-rights-operator-1080.png
- **Kind:** spacing
- **Severity:** medium
- **Problem:** Sources, Destinations, Levels and Presets are `.sode__rights-list` blocks 8px apart with a 6px label gap. Visibility, Protect, Use tie lines and No auto-lock are bare `.sode__field`s with 0px between them: Visibility's select ends at y=757.1 and the "Protect" label starts at 757.1; a hint ends at 833.1 and the next label starts at 833.1. The label gap there is 4px. The heading "User: Administrator" repeats the select right above it.
- **Change:** web/usersrights.js:33: wrap the editor body in a column, `el('div', 'sode__keys-new', …)` (flex column, gap `--pb-space-4`), and drop `margin-bottom` from `.sode__rights-list` (lock.css:645-649). Set `.sode__rights-head { gap: var(--pb-space-2) }` so label gaps match `.sode__field`. Remove the repeated `h3`.
- **Also check:** the list inputs under "All" look disabled but show an example value "1-8, 12": set the placeholder only when the mode needs a list.

### B2-13: Export tree labels read "Everyone's defaults Trusted Approval"
- **Where:** Server > Export & Import > Export, Settings expanded; 07-server-export-import-expanded-1080.png
- **Kind:** bug
- **Severity:** medium
- **Problem:** exportdialog.js:10 reads `legend.textContent`, which includes the role badge and the hidden Approval badge. Every one of the 12 rows shows "… Administrator Approval" or "… Trusted Approval".
- **Change:** web/exportdialog.js:10: `?.textContent` to `?.firstChild.textContent.trim()` (the same read as settingssave.js:91).
- **Also check:** settingsindex.js and the save review, wherever a legend is read as a name.

### B2-14: Theme color preview: three boxes, two sizes
- **Where:** Look and feel > Branding; 02-look-branding-custom-crop-crop-4k2x.png, 02-look-branding-crop-crop-4k2x.png
- **Kind:** alignment
- **Severity:** medium
- **Problem:** The custom picker and the border swatch are 44px (`--pb-space-10 + --pb-space-5`), the app icon is 48px (`3rem`). Tops are at y=1509.4 and 1507.4. Nothing says what the two boxes are.
- **Change:** web/base.css:435-444: set swatch and `.sode__colorpick` (base.css:1479-1485) to `3rem` like the icon. Add a hint after `#brand-preview` in index.html:349: `<span class="sode__hint">Page border and app icon</span>`.
- **Also check:** the setup wizard uses the same preview.

### B2-15: Mixed 11px and 13px buttons in one view
- **Where:** History (Clear History 13px) next to Level colors (Reset to default 11px); Users (Manage users 13px, Edit Profile 11px); Manage users (Add user, Add API client, Make key 11px; Done 13px); My API clients (Make key 13px)
- **Kind:** design-language
- **Severity:** medium
- **Problem:** `--pb-control-h-sm` is set to the normal height (base.css:114), so `pb-btn--sm` only shrinks the text to 11px in a 32px button. The same action is drawn two ways: "Make key" is 11px outline in Manage users (usersclients), 13px outline in My API clients, 13px primary in the footer of My API keys.
- **Change:** drop `pb-btn--sm` from standalone section buttons: index.html:383 (`cfg-colors-reset`), users.js:286 (Profiles rows), users.js "Add user", usersclients.js:195 and its Make key, profiles.js "Add preset" and "Add filter set". Keep `--sm` only inside dense table rows (Manage users rows). Make "Make key" the same in all three places: 13px, `sode-btn--primary`, last in its form.
- **Also check:** B2-16.

### B2-16: Full-width buttons
- **Where:** Access > Locks "Apply an Access preset…" (640px), Access > Users "Manage users" (640px), preset dialog "Tick all" (576px); 06-access-locks-1080.png
- **Kind:** design-language
- **Severity:** medium
- **Problem:** These stretch because they are direct children of a column `.sode__field`. Every other section button (Clear History, Export & Import…, Run setup again…) keeps its text width.
- **Change:** web/base.css: add `.sode__field > .pb-btn { align-self: flex-start; }`.
- **Also check:** none.

### B2-17: Manage users jumps when the tab changes
- **Where:** Manage users; 08-users-manage-users-tab-1080.png, 08-users-api-clients-tab-1080.png, 08-users-rights-tab-1080.png
- **Kind:** readability
- **Severity:** medium
- **Problem:** The dialog is centred on its height. Its top is at y=329 (Users), 248 (API clients) and 73 (Rights), so the tab strip moves 256px under the pointer.
- **Change:** web/lock.css: `.sode__scrim > .sode__filters.sode__users-dlg { align-self: flex-start; margin-top: 8vh; }` and `.sode__users-dlg .pb-modal__body { min-height: 30rem; }`.
- **Also check:** Export & Import (Export tab is 680px tall, Import tab is 180px).

### B2-18: Lone Close button is left in some dialogs and right in others
- **Where:** My API clients (Close left, x=652), Manage users (Done right), Profile (Close right), Export & Import (Close left); 08-my-user-clients-1080.png, 08-users-manage-users-tab-1080.png
- **Kind:** design-language
- **Severity:** low
- **Problem:** The footer is space-between, so a single button lands left unless a note element comes before it. Two words are used for the same action: Done and Close.
- **Change:** web/myclients.js:60: `foot: [el('span'), btn('pb-btn--outline', 'Close', …)]`. web/users.js: rename "Done" to "Close".
- **Also check:** every `foot:` with one button (editorlib.js:106).

### B2-19: My API clients: scopes come after the button that uses them
- **Where:** My user > My API clients; 08-my-user-clients-1080.png
- **Kind:** bug
- **Severity:** medium
- **Problem:** Order on screen: Name, [Make API client], then the scope checks with 0px gap (button ends y=423.0, scopes start 423.0), no label. A reader presses the button before seeing the scopes. The "For" select is empty and "Make key" is disabled with no reason shown.
- **Change:** web/myclients.js:45: `el('div', 'sode__keys-new', field('myclient-name', 'Name', nameIn), el('div', 'sode__field', el('span', 'sode__label', 'API scopes'), scopeBox), el('div', null, addBtn))`. Hide the "Make a key" block while there is no client and show `Make an API client first.` in its place.
- **Also check:** none.

### B2-20: Copy this browser dialog says Copy, the button says Move
- **Where:** My user > Copy this browser to My user; 08-my-user-copy-browser-1080.png
- **Kind:** text
- **Severity:** medium
- **Problem:** Title "Copy this browser to My user", primary button "Move", switch "Keep in This browser" (off). An empty dialog shows three stacked hints: "This browser", "No filter sets or presets", "Nothing will change."
- **Change:** web/browsercopy.js: make the button follow the switch: "Copy" when Keep is on, "Move" when off, and default the switch to on so the title is true. With nothing to copy, show one line: `This browser has no filter sets, presets or settings to copy.`
- **Also check:** the My user button text (index.html:274).

### B2-21: Import tab is blank
- **Where:** Export & Import > Import, both dialogs; 07-server-export-import-import-1080.png, 08-my-user-own-import-1080.png
- **Kind:** readability
- **Severity:** medium
- **Problem:** The tab body is empty (the body is 30px of white under the tabs). The only control is "Choose a file…" in the footer.
- **Change:** web/importtab.js: add a first-state paragraph in the panel: `Choose a Sode export file. You see what it holds before anything changes.`
- **Also check:** none.

### B2-22: Tooltip covers the field it describes
- **Where:** Look and feel > Confirm and results; 02-look-confirm-tip-1080.png. Also Access > Locks tips (06-access-locks-tip-1080.png)
- **Kind:** readability
- **Severity:** medium
- **Problem:** The label tip "Delay before the Confirm letter and OK work." draws over the number input below the label (y=110 to 136 against the input at 111 to 143). Key facts are tip-only: "0 = off", "0 = stays open. Failures always stay.", "Needs a user with a PIN".
- **Change:** web/index.html:391-397, 631, 636, 678: move these to a visible `<span class="sode__hint">` under the control, as Routers does ("0 is off."). Text: `Delay before the Confirm letter and OK work.`, `0 keeps it open. A failure banner always stays.`, `0 is off.`, `Needs a user with a PIN.`
- **Also check:** tooltip.js placement for labels: prefer `pb-tip--top`.

### B2-23: Invalid level color shows no message
- **Where:** Look and feel > Level colors; 02-look-level-colors-invalid-1080.png
- **Kind:** edge case
- **Severity:** medium
- **Problem:** Typing `#zzz` sets `pb-input--invalid`, but the focus ring hides it and the swatch silently falls back to the default color. The message comes only at Save. Unset positions 9 to 17 show the default as grey placeholder text that looks like a disabled value.
- **Change:** web/settingsform.js:176-179: on invalid input set a hint under the grid (`<p class="sode__hint sode__error-text" id="cfg-colors-err" role="status">`) to `Position 1 color must be hex, like #3a7bd5.` Add under the legend hint: `Grey text is the default color.`
- **Also check:** the Hidden destinations error in settingsform.js:111 says "Globally hidden destinations", but the label is "Hidden destinations in History". Use the label text.

### B2-24: "Save as my default" does not look like a button
- **Where:** Preferences > This browser; 01-preferences-this-browser-1080.png, 01-preferences-save-as-default-hover-1080.png
- **Kind:** readability
- **Severity:** medium
- **Problem:** Eleven ghost buttons at 11px with letter spacing read as a column of labels. Nothing shows which rows already have a default of mine.
- **Change:** web/devicesettings.js:45: use `pb-btn--outline pb-btn--sm` and shorter text `Save for me` (tip: "Save as my default in every browser"); when a default is set show `Clear mine`.
- **Also check:** column width in base.css:1998 after the text change.

### B2-25: Clear History confirm buttons are 3px apart
- **Where:** Look and feel > History, after Clear History; 02-look-history-clear-confirm-1080.png
- **Kind:** spacing
- **Severity:** low
- **Problem:** Cancel ends at x=701 and "Clear History for everyone" starts at 705: the two sit in a `<span>` and only white space separates them. Other button rows use 12px.
- **Change:** web/index.html:369-372: remove the wrapping `<span>` and add `sode__names--start` to `#clear-step2` (needs B2-05). Give the destructive button `pb-btn--danger`.
- **Also check:** none.

### B2-26: Manage users column heads
- **Where:** Manage users > Users; 08-users-manage-users-tab-1080.png
- **Kind:** design-language
- **Severity:** low
- **Problem:** Only "Name" and "PIN" have heads, at 14px from a literal `font-size: 0.875rem` (lock.css:502), while field labels are 13px medium. The Role column has no head.
- **Change:** web/lock.css:501-504: `font-size: var(--pb-text-sm); font-weight: var(--pb-weight-medium);`. web/users.js:44: add `el('span', null, 'Role')`.
- **Also check:** lock.css:539 `.sode__profiles-h { font-size: 1rem }` is off the type scale (16px): use `var(--pb-text-lg)` or `--pb-text-md` bold, the same as section legends.

### B2-27: Settings at 3840x2160 is a 640px column
- **Where:** every 4k shot; 01-preferences-this-browser-4k.png
- **Kind:** readability
- **Severity:** medium
- **Problem:** At 3840 wide and DPR 1 the form is 640px (17% of the width) with 13px text and 11px hints. Nothing scales with the viewport: the computed root is 16px.
- **Change:** web/base.css:6: `font-size: 100%` to `font-size: clamp(100%, 0.42vw + 0.5rem, 200%)` (16px at 1920, about 24px at 3840), or the owner's rule of 125% plus the clamp. Check 2500x750 before use.
- **Also check:** all pages; this is one change for the whole app. AGENTS.md says the root is 125%, the build measures 100%.

### B2-28: Env-only My user shows only a Lock button
- **Where:** env-only, Preferences > My user; 15-envonly-my-user-1080.png, 15-envonly-first-view-1080.png
- **Kind:** edge case
- **Severity:** low
- **Problem:** With no users the account line is empty and every button but Lock is hidden. The section is a legend and one button at the right edge.
- **Change:** web/devicesettings.js:125: when there are no users show `No users yet. Add one in Access > Users.` in `#account-name`.
- **Also check:** Lock with no users does nothing useful; hide it in this state.

### B2-29: Save review dialog: three left edges
- **Where:** Save > Review; 08-users-save-error-1080.png
- **Kind:** alignment
- **Severity:** low
- **Problem:** Table head and rows start at x=678, the topic "Access" and "USERS" at 673, "BACKUP" and its buttons at 672. Read from the screenshot, not measured live.
- **Change:** web/base.css `.sode__review-topic` and the section row: use the cell padding of the table (`padding-inline: var(--pb-space-3)`), and give the Backup block the same inset.
- **Also check:** owned by the save flow; may overlap the Routers reviewer.

### B2-30: Text fixes
- **Where:** Settings labels and dialogs
- **Kind:** text
- **Severity:** medium
- **Problem and change (current to new):**
  - index.html:689 `New-panel Authentication mode` to `Authentication mode for new panels`.
  - index.html:238, 310 `Searching history respects filters` to `Search in History uses filters` ("History" has a capital in the two labels near it).
  - index.html:230 `Destination before Source` to `Destination before source`.
  - index.html:242, 250 `Stacked filters behavior`, `Next match behavior`: these are glossary terms, keep. No change.
  - index.html:395 `Success banner closes after (s)` to `Close a success banner after (s)`.
  - profiles.js:123 `Not set uses the global default.` to `Not set: Everyone's default applies.` ("global default" is not a glossary term; the scope is Everyone).
  - accesspreset.js:28 title `Apply an access preset` to `Apply an Access preset` (the button says "Access preset").
  - accesspreset.js:19 `Set Never sign out and No auto-lock for` to `Also change these users` with a hint `The preset sets their Never sign out and No auto-lock.`
  - users.js:207, 214, 286 `Edit Profile`, `Edit Rights` to `Edit profile`, `Edit rights` (sentence case, as "Manage users", "Add user").
  - users.js:189 `Remove "Operator": Save deletes their PIN and Profile.` to `Save removes "Operator", their PIN and their Profile.`
  - index.html:703 Require HTTPS hint (41 words, 3 ideas) to: `Sode refuses a request that a trusted proxy did not mark X-Forwarded-Proto: https. The health endpoints stay open. Add a trusted proxy first, and save from an HTTPS address. To turn it off from the console: sode -reset-https.`
  - usersclients.js:189 (one 60-word paragraph) to three sentences on separate lines: `An API client is an app or panel with its own name in History.` / `It has its own API scopes (here) and Rights (Rights tab).` / `A rename ends its keys. A user key ends when that user changes their PIN, so give a wall panel a client key.`
  - Guest view: `Not signed in` stands next to "Sign in" and "Step in" with no word on what a guest can do; add hint `You act as a guest, with Guest Rights.`
- **Also check:** run `go test ./internal/app -run 'TestSTE|TestRetiredWords' -v` after the edits.

### B2-31: Uppercase micro labels used as field labels and as row names
- **Where:** Access > Users profile rows (ADMINISTRATOR, TRUSTED, OPERATOR), Page locks, New user defaults, Keyless scopes; Profile dialog (THEME, LAYOUT …); 06-access-users-1080.png, 08-users-edit-profile-trusted-1080.png
- **Kind:** design-language
- **Severity:** low
- **Problem:** `.sode__label` (11px uppercase muted) names a check group in one place, a user in another (user names in capitals: "ADMINISTRATOR" is the user, not the role) and a select in the Profile dialog. The same 11 settings are drawn as 13px medium labels in Everyone's defaults and as uppercase labels in bordered boxes in the Profile dialog.
- **Change:** web/users.js:284-288: draw the user name as plain text (`el('span', null, n)`), not `sode__label`. web/profiles.js: build User defaults with `sode__fields` and `sode__field` like Everyone's defaults, without `.sode__profiles-row` boxes.
- **Also check:** none.

---

## Shared causes

1. Check groups have no layout rule of their own (B2-06, B2-07, B2-19, part of B2-01): one `[role="group"]` flex rule with `gap: var(--pb-space-2) var(--pb-space-5)` clears four entries.
2. Rule order and inherited `gap` (B2-05, B2-08): a modifier written before its base, and `.sode__field`'s 4px gap leaking into a subgrid.
3. Nodes inserted with `after()` next to a field become grid items (B2-01, and `.sode__refused`): insert inside the label, or span all columns.
4. `pb-btn--sm` changes only the font size because `--pb-control-h-sm` equals the normal height (B2-15, B2-24, B2-26): keep `--sm` for table rows only.
5. Column `.sode__field` stretches its direct buttons (B2-16): `align-self: flex-start`.
6. Half-width is 50% while grid columns are (100% - 12px) / 2 (B2-09).
7. Dialog width and position are per-dialog literals (B2-03, B2-17, B2-04): Manage users needs 68rem and a fixed top.
8. Facts kept only in tooltips (B2-22, B2-23, B2-24): use a visible hint under the control.
9. No viewport scaling of the root font (B2-27).

---

31 entries: 6 high, 18 medium, 7 low.
Worst 1: Access preset badges become grid items and break the Locks layout (B2-01).
Worst 2: The Not saved banner hides the Settings search box in env-only mode (B2-02).
Worst 3: Manage users rows overflow, so "Sign out everywhere", "Confirm remove" and the role select are clipped (B2-03).
Close behind: the Profile dialog's empty blocks (B2-04) and the My user and check-row spacing bugs (B2-05, B2-06), each a one-rule CSS fix.