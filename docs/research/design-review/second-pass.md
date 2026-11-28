# Second pass (Fable), 2026-11-28

Read-only review of the six findings files and the two owner calls. CSS values here are computed, not measured.

## Owner calls

### 1. 4K: keep scaling by root font (option a), but steepen the rule that already exists. Do not use B2-27's clamp, and do not treat column widening (b) as the 4K answer.

Facts first, since the review has them wrong:
- `web/base.css:5-20`: root is 100% up to 2400 px wide, then grows 1px per 360 px to 125% (20px) at 3840. Grid panel page and its Live Editor: 90%, growing to 112.5%. Phone is excluded from the media rule and stays 100%.
- The 125% in the brief comes from an older AGENTS.md. On the reviewed branch, AGENTS.md line 39 and `docs/agents/web-layout.md:4` describe exactly the rule above (commits 9cbfd934, 7ce5d383). Code and docs agree.
- The 4k2x shots (1920x1080 CSS px, DPR 2) have the same layout as the 1080 shots, only sharper. That is a 4K desk monitor at 200%, and it looks right: the page fills the window (compare `A/069-history-with-entries-4k2x.png`).
- The 4k DPR1 shots (3840 CSS px) show the problem: the main page takes 64% of the width with 16.25px body text, Settings is an 800px column, and a Buttons panel is 1950px of 3840. This case is a wall TV or a 32"+ monitor with no OS scale. It is not the usual 4K desk.
- A 4K desk at 150% is 2560x1440 CSS px. Nobody captured that size (see Gaps). The root there is 16.4px today.

Reasoning:
- At 150% a 32" 4K screen with no OS scale shows 13px text at the same physical size as a 24" 1080p screen (19.5px x 0.184mm = 3.6mm, against 13px x 0.277mm). 125% leaves it 17% smaller. On a 55" wall TV it gives 6mm text.
- B2-27's `clamp(100%, 0.42vw + 0.5rem, 200%)` gives a 16.06px root at 1920 (fractional rems on the main target), grows every window between 1920 and 2400 that the owner chose to hold at 100%, and replaces a documented rule.
- Widening columns only (B1-32, A1-15) leaves 4K DPR1 text at 16px. Those caps are good fixes at 1080 and should go ahead as their own entries.
- Width-only scaling over-scales ultrawides: 3440x1440 gets 118% today and would get 136%. Bounding by height fixes that.
- It stays inside "UI scales with html font-size, never zoom".

Exact change, `web/base.css:13-20`:
```css
@media (min-width: 2400px) {
  html:not([data-layout="phone"]) {
    font-size: clamp(100%, 100% + (min(100vw, 177.78vh) - 2400px) / 180, 150%);
  }
  :root:not([data-layout="phone"]):has(.sode-panels .sode-gridpanel, .sode-ed[data-live="grid"]) {
    font-size: clamp(90%, 90% + (min(100vw, 177.78vh) - 2400px) / 200, 135%);
  }
}
```
Result: 16px up to 2400, 16.9px at 2560x1440 and 3440x1440, 24px at 3840x2160 (Grid 21.6px). No step at 2400.

Also needed:
- Update the comment above the rule, AGENTS.md line 39, `docs/agents/web-layout.md:4`, `docs/panels-design.md:68`.
- `internal/app/gridpanelscale_browser_test.go` pins the old numbers.
- Check 2560x1440, 3440x1440, 3840x2160, and the Grid speed at 2500x750.
- A1-27 (lock pad in vmin) becomes unnecessary.
- A2-06's Levels box cap (`desktop.css:60`) is still worth doing.

### 2. Panel width: Full width by default, fitted to the window height. Keep the button and its present words.

Reasoning:
- `docs/panels-design.md:287`: "A panel without a title bar, or in kiosk, has no button." So the pages the feature exists for (kiosk, wall tablet) are held at the 100rem cap with no way out. `87-panel-buttons-idle-4k.png` shows a quarter-screen panel.
- A Grid panel is already always full width (`panels-design.md:77`).
- Full width alone is not enough. The grid is `width: 100%` plus an `aspect-ratio` set in `panelview.js:95`, so a 6x4 panel at full width on 1080p is 1272px high and scrolls. It must fit both ways.
- "Narrow" is a new word. `panelfull.js:18` already has "Normal width" and "Full width".

Exact change:
- `web/panelfull.js:20-22`: `let on = true;` and `on = localStorage.getItem(KEY) !== '0';`. The key `sode-panel-full` and its values `1`/`0` stay, so no persisted name changes. Update `docs/persisted-names.md:154` to say that no value means Full width.
- In `initChrome`'s `apply()`, force the class when the button is not reachable: `ui.main.classList.toggle('sode-panels--full', stored || kiosk || ui.head.hidden)`.
- Fit the height. Where `panelview.js:95` sets `aspectRatio`, also set `--cols` and `--rows`. In `panels.css` after line 139: `.sode-panel__grid { margin-inline: auto; max-width: calc((100dvh - var(--sode-panel-chrome, 6rem)) * var(--cols) / var(--rows)); }`. The 6rem (header plus padding; less in kiosk) needs measuring.
- Let text grow with the cell, keeping the per-panel Font size as the multiplier: in `panels.css:349-359` use `max(var(--pb-text-md), 6.5cqi)` in place of `var(--pb-text-md)` on the cell's text children. 6.5cqi equals today's 13px in a 199px cell. Do not use C2-37's plain `cqmin`, which would change what the saved Font size means.
- Do C2-36 (title offset) after this, since the chip overlap changes.
- Update `docs/panels-design.md:287`.

## Wrong or doubtful findings

- **README cause 10**: "root stops growing at 1920", "text is 16px at 3840" and "AGENTS.md says 125%" are all wrong (see call 1). Correct the paragraph.
- **B2-27**: "computed root is 16px", "640px column" and "13px text" at 3840 are 1080 numbers. The real ones are 20px, 800px, 16.25px (B1-32 and A2-06 have them right). Drop its clamp, replace with call 1.
- **A2-06**: the "AGENTS.md is out of date" note is stale. Drop the note, keep the Levels cap.
- **A2-17**: names `pb-btn--primary`, which does not exist. AGENTS.md says `sode-btn--primary`. Correct it.
- **C1-17**: `pb-note--info` does not exist in patchbay 2.4.0. Use plain `pb-note` padding.
- **B1-09**: `--pb-leading-body` does not exist. It works only through the `1.5em` fallback, so write `1.5em`.
- **A1-24**: it says "patchbay 2.2.1, I did not check" for a focus token. The vendored version is 2.4.0, and B1-22 found that patchbay already draws the ring on `[tabindex]`. Delete the custom outlines instead of rewriting them in rem.
- **C2-24**: "Ask for an instance name (the glossary's own term)" is wrong. GLOSSARY.md:360-361 fixes the shown text as "Ask users to set an instance name". Drop that line or change the glossary row with it.
- **C2-24 and README "Settled"**: "Take button behavior" is not a glossary term (no row), so "Take guard" is a plain owner wording call, not a glossary conflict. Check that `None/Hold/Twice` are labels only before any change.
- **C2-05**: "Acts as" to "Sign-in" goes against GLOSSARY.md:350-351 (the term is Authentication mode; the list keeps Acts as). If one word is wanted, it is "Authentication mode".
- **C2-01, C2-47**: "200% zoom" means a 960px viewport from the reviewer's own capture, not the 4k2x set. Keep them as narrow-window findings at medium.
- **C1-07**: not run. `table-layout: fixed` cancels C1-01's `min-width`, and the ellipsis rule would cut hex bytes. Do C1-01 first, and C1-07 only after measuring.
- **A1-10 (and A1-11, A1-28, which depend on it)**: it reverses a deliberate design. `lock.css:87-88` says the in-flow list and shrinking pad exist to fit 768px. It is plausible but is an owner call, and lock browser checks will need changes.
- **A2-01**: the `row-gap: 0` fix is verified. The "better" rename to `sode__boxgroup` touches tests and the identifier scan. Do the one-line fix only.
- **A2-13**: checked "Last 1–4" at the larger text. AB-24 requires "Last 0–10" to fit. Re-measure the `5rem` min-width at `--pb-text-md`.
- **A2-22 against A1-12**: one adds `opacity: 0.4` to disabled buttons, the other removes opacity so "only patchbay's disabled colors apply". Pick patchbay's own disabled look for both.
- **README cause 1 and B1-14 option 2**: `pb-btn--sm` at full height is by design (`base.css:109-117`, the three control heights). Restoring patchbay's small height breaks that rule. Use B2-15's fix only: drop `--sm` from standalone buttons.

Confirmed against code: A1-01, A1-02, A1-04/05, A2-02, A2-04, A2-08, A2-09, B1-01, B1-23, B1-28, B2-01, B2-05, B2-06, C1-01, C1-15, C2-13, C2-25, C2-34. For C2-34, the 72% pulse floor keeps `--pb-text-on-accent` readable at both ends.

## Conflicts

- **A1-18 against owner text.** The README settled only AB-11 and AB-17. Also the owner's own words:
  - "Connection: Add your first router" (AB-14)
  - "This user will be the administrator." (AB-12)
  - "Connected: N sources and M destinations discovered." (AB-23 says copy the wizard's words)
  
  Drop those three rewrites.
- **A1-18 against B1-19.** Both reword the same string (`routerLine`, `setupwizardrules.js:168`, shared by `routertry.js`), each differently. Keep the owner's text. B1-19's error mapping can stay.
- **B1-16 against AB-22.** AB-22 puts Find and the problems filter at the end of the chip row. B1-16 moves them to the bar. Owner call.
- **A2-21 against AB-25.** The Router button's place is settled. Changing its label to "No prefix: A" is a new owner call.
- **C2-14 against B2-18.** C2-14 puts a lone Close on the left. B2-18 pushes a lone Close to the right with an empty span. Pick one (right is what most dialogs do today) and implement it once in `web/modal.js:16` with `:only-child { margin-inline-start: auto }`, not per dialog.
- **B2-10 against C1-19.** Same sidebar indent fix with different router values (`space-9 + space-5` against `space-10`). Use C1-19's.
- **A1-24 also-check against B2-26.** Same lines (`lock.css:502`, `:539`), different tokens. B2-26 wins.
- **A2-02 against C1-18.** A2-02 deletes `history.css:607`. C1-18 moves the same line to `.sode__search`. They need one combined change.
- **A1-16.** "Site name" is not in GLOSSARY.md, and the owner's heading says "Sode Server Name". Renaming Settings' "Logo text and page title" needs the owner and a glossary row.
- **C2-39.** "Name of this screen" replaces the glossary term Instance. Owner call.
- **C2-44 and call 1.** Both change Grid sizes. Do C2-44 after the root change, then check the speed once.

## Merge these

- A1-02 + C1-26 (lock Name box width): stream 3.
- A1-10/A1-28 + A2-07 (name list, shared `.sode-namepick__list`): stream 3 owns it. Stream 4 only checks the Approval popup after.
- A2-08 + C1-12 (router names): one `routerName` in `routerlinkrules.js`, used by `core.js:613-614`, statusinfo and /log. Stream 1. Drop A2-08's own `prefix: title` form. Its `pre-line` and title text parts stay.
- B2-10 + C1-19: stream 1.
- B1-14 + B2-15 + B2-24 + A2-16(1) (`pb-btn--sm`): one decision in stream 1. Streams 4, 5, 6 remove the class in their own files.
- A2-17 + B1-05 + B1-30 + B2-18 + C2-14 (dialog buttons and footer order): one rule in stream 1 (`modal.js`, button classes). The others apply it.
- A2-04 + A2-15 + A1-14 + B2-22 + C1-10 (tips over their own control): put `hideTip()` and a default "open up when the tip would cover the control's field" in `tooltip.js`, stream 1. Then the per-control `pb-tip--top` edits mostly go away.
- B2-06 + B2-07 + B2-19 (check groups): stream 1 for the rule, stream 6 for the markup.
- A1-17 + the Access rows in `accesspreset.js`: one list in `setupwizardrules.js:15-22`, stream 2.
- A1-14 also-check + B1-09 (Settings fireMode help button): stream 5.
- B1-29 + B2-29 (Save review spacing): stream 5.
- B1-31 + C1-24 (cut settings path): stream 5 fixes the cut, stream 7 the phone rule.
- A1-24 + B1-22 (focus rings and raw px): stream 1.
- A2-09 + C2-25 + B1-09 (`--pb-*` names that do not exist): add one node test that every `var(--pb-*)` in `web/*.css` is defined. Stream 1.
- C1-02 + the panel "Offline" cover + the editor's "not answering" line: one wording, owned by stream 7, but it edits stream 8's files.

## Gaps

- **Sizes never captured:**
  - 2560x1440, the most common desk size (QHD, and 4K at 150%)
  - 3440x1440
  - anything between the phone and 1920 (1280x800 and 1366x768 touch panels, tablets, the `base.css:897` breakpoint at about 1380px)
  
  Nearly all layout numbers are from 1920 only.
- **Touch on the desktop layout:** no pass for hit sizes on a 1080p touch screen, and none for the big scroll bar rule (`SCROLLERS`) against native bars.
- **Phone:**
  - Settings area B not captured.
  - History on the phone not captured.
  - Panel pages have 8 phone shots (113-116, 142-145), but C2 has no phone finding and did not read `gridpanelphone.css`.
- **Dark theme:** shots exist, but only C2-34 and C1-09 measured contrast. No pass on level chip colors, disabled text or state colors in dark.
- **Keyboard:**
  - Key notes in keyboard mode not reached (A2).
  - Palette on the phone showed nothing.
  - No focus order or focus ring pass outside the lock pad and /log.
  - Reduced motion not checked.
- **Whole features with no finding:**
  - Grid popup on the main page
  - Palette
  - preset and filter set Share links
  - Broken and Name moved items, and Swap results
  - Compact view
  - native fire mode on the main page
  - editor Templates and Checkpoints
  - the panel live-update countdown banner
- **States not reached:**
  - Step in and Step out
  - an Approval after Approve
  - results of confirmed Remove router, Free, Clear History and Remove tie line
  - Import with a file loaded
  - Restore with a new PIN (A1-21 is from code only)
  - editor unsaved state
  - Smart menu, Live Editor pick
  - router down on a panel (C2-41 unproven)
  - lockout countdown

## Plan changes

- **"Each workstream owns its files" is not true.** Shared files:
  - `web/base.css`: streams 1, 4, 5, 6, 7
  - `web/index.html`: 2, 4, 5, 6
  - `web/lock.css`: 2 (A1-24), 3, 6
  - `web/history.css`: 4, 7
  - `web/core.js`: 1, 4, 6
  - `web/panels.css`: 8, 9
  - `web/settingssave.js`: 5, 6
  - `web/setupwizardrules.js`: 2, 5
  - `web/tooltip.js`: 2, 4, 7
  - the Not saved banner: 4 (A2-27) and 6 (B2-02)
  
  Either assign each shared entry to one stream as in "Merge these", or expect hand merges. Most are different hunks.
- **Stream 1 must merge before the others start**, not "run first or merge first". It should also take `modal.js`, `tooltip.js`, the `--sm` decision, the token test and the 4K rule. Both owner calls must be settled before stream 1 and stream 9 start.
- **Stream 4's file list is wrong.** A2 edits `base.css` (A2-03, A2-08, A2-13, A2-14, A2-22), `core.js`, `tooltip.js`, `approval.js`, `protects.js`, `tiepicker.css`. Move A2-08 to stream 1.
- **Order inside streams:**
  - A1-10, then A1-11, then A1-28
  - B1-01, then B1-02
  - B2-05 (stream 1), then B2-25
  - C1-01, then C1-07
  - C2-13, then C2-17
  - C2-37, then C2-36
  - call 1, then C2-44, then the 2500x750 speed check
  - stream 3 before A2-07
- **Go changes hide in web streams:** A1-18 (`pin.go`), C1-13 (the why list plus a test), C1-25 (404 page). Those streams must run `go test -short ./...`.
- **Every stream:**
  - grep `TestChrome*` and `web/test` for each string or selector it changes
  - run the STE and vocabulary tests
  - add a `changelog.d` fragment
  - branch into an `integration/*` branch so each PR runs the light CI, with one full run at the end
- **Add a stream 10** for the gap captures (2560x1440, 1280x800 touch, phone Settings, dark contrast) before streams 4 to 9 finish, so new findings land in the same round.