# Design review, 2026-11-28

Branch reviewed: `alpha-round2` at 8e0969a7 (the Settings redesign plus Alpha Notes round 2).

How: three Sonnet agents took about 1,955 headless Chrome screenshots of every page and state they could reach: 1920x1080, 3840x2160, 1920x1080 at 200%, dark, phone at 375x812. Six Fable agents then reviewed one slice each. They read the screenshots and the code, and measured layout with bounding boxes in a seeded copy of the app. Screenshots are not in git. They are in the session scratchpad under `review/A`, `review/B` and `review/C`, each with an `index.md`.

Every entry gives the file:line and the exact new code or text. Status: captured, not yet checked by the owner.

| Slice | File | Entries | High | Medium | Low |
| --- | --- | --- | --- | --- | --- |
| A1 Setup wizard, lock screen | findings-A1.md | 29 | 6 | 15 | 8 |
| A2 Main page, top bar, banners | findings-A2.md | 27 | 8 | 13 | 6 |
| B1 Settings: Routers, Tie lines, Protects, Save | findings-B1.md | 32 | 7 | 17 | 8 |
| B2 Settings: everything else | findings-B2.md | 31 | 6 | 19 | 6 |
| C1 Communication Log, phone, server down | findings-C1.md | 26 | 5 | 14 | 7 |
| C2 Panels list, editor, panel pages | findings-C2.md | 53 | 13 | 26 | 14 |
| **All** | | **198** | **45** | **104** | **49** |

## Causes shared across slices

Fix these first. Each one clears entries in several slices.

1. **`pb-btn--sm` is full height.** Sode sets `--pb-control-h-sm` to the normal height, so a small button only gets 11px text in a 32px box. Small and normal buttons then sit side by side at one height (B1-14, B2-15, B2-24, A2-13).
2. **Flex gaps that never apply.** Patchbay's `.pb-tabpanel.active { display: block }` beats Sode's flex containers, which removes every gap in Share, Link with key, Push and the editor side panel (C2-13, C2-23). A modifier rule placed before its base rule loses the same way (B2-05), and `.sode__field`'s 4px gap leaks into subgrids (B2-08, A2-01, A2-10).
3. **Rows sized by their content, not one grid.** Table and list rows that end in an `auto` track, or that are wrapping flex lines, put each row's columns somewhere else: Tie lines heads up to 77px off (B1-01), Protects, Accept/Dismiss, Open now rows, Grid editor toolbars 105px off, log columns (B1-04, C2-07, C2-29, C2-30, C1-07).
4. **Nodes added next to a field become grid items.** `after()` inserts the preset badges and refused notes into `.sode__fields` grids and breaks the layout (B2-01).
5. **Check groups have no layout rule.** Wrapped checks are indented 12px, and gaps differ from 0 to 24px (B2-06, B2-07, B2-19).
6. **Settings sidebar children are 2px left of their parent** (B2-10, C1-19).
7. **Router names are built in three places.** That gives "Router A Router A" in the Lost link banner and doubled names on /log (A2-08, C1-12).
8. **The lock Name box is 80px wider than the PIN pad** (A1-02, C1-26).
9. **Facts kept only in tooltips.** Tips are not shown on touch screens, and some tips cover the field they describe (B2-22, B2-23, A1-14, C1-10).
10. **4K at DPR 1.** The root is 100% up to 2400px wide, then grows to 125% (20px) at 3840 (`base.css:5-20`, as AGENTS.md says). A 4K desk monitor at 200% (the 4k2x shots) looks right. A wall TV or a big monitor with no OS scaling (3840 CSS px) gets 16.25px text and an 800px Settings column (A2-06, B1-32, A1-15, A1-27, C2-37). See owner call 1.

## The worst problems, by slice

- **A1:** while setup runs, the site name and Theme color go back to "Sode" and blue after each hello (A1-01). The lock screen shows a blurred second copy of the site name (A1-03). The Access table headings are about 100px off their Yes/No values (A1-04).
- **A2:** the routes under Source and Destination are 6px apart (A2-01). The History filter boxes are 32px beside 40px buttons (A2-02). The Lock menu tip stays over the menu (A2-04).
- **B1:** the Tie lines table heads are 24 to 77px off their columns, and the State text wraps with one orphan character (B1-01, B1-02). The Save review shows a cut sentence for a removed router (B1-28).
- **B2:** the Access preset badges break the Locks grid (B2-01). The Not saved banner covers Settings search (B2-02). Manage users rows clip their buttons (B2-03).
- **C1:** the Log table is wider than the page, so Bytes is cut (C1-01). With the Server link down, /log, /panels and the editor show only a red icon (C1-02). The uptime bars add 288 Tab stops (C1-03).
- **C2:** a staged Grid crosspoint is a faint arc, and "Take" on an armed Buttons panel is 2:1 contrast (C2-42, C2-34). A failed take shows a 14px cross for 2 seconds (C2-35). The Grid is 11 to 40% smaller than the prototype, with 9.9px text (C2-44).

## Second pass

A second Fable pass checked the findings against the code (`second-pass.md`). It confirmed 18 of the high entries. It corrected or dropped others: wrong token or class names (A2-17, B1-09, C1-17), glossary clashes (C2-05, C2-24), and B2-27's 4K numbers. It found conflicts between entries, 15 groups of duplicates to merge, gaps in coverage, and file overlaps in the plan. Read `second-pass.md` before you start any stream. It takes priority over the slice files where they disagree.

## Settled without the owner

- The owner's words win over a reviewer's rewording. These stay: AB-11's heading "Sode Server Name and Theme Color", AB-17's Done text, AB-14's "Connection: Add your first router", AB-12's "This user will be the administrator.", and AB-23's "Connected: N sources and M destinations discovered." Drop the A1-16 rename, those three lines of A1-18, and B1-19's rewording (its error mapping stays).
- AB-22 and AB-25 are settled. Drop B1-16's move of Find, and A2-21's new Router button label.
- Glossary terms stay: drop C2-24's "Ask for an instance name", C2-05's "Sign-in" and C2-39's "Name of this screen". "Take guard" (C2-24) is not a glossary term, but it is a new wording with no need behind it, so drop it.
- Dialog footers: a lone Close or Done goes on the right, with one rule in `web/modal.js` (`:only-child { margin-inline-start: auto }`), not per dialog. This replaces C2-14 and B2-18.
- `pb-btn--sm` stays full height, by design (`base.css:109-117`). Drop it only from standalone buttons (B2-15), not B1-14's option 2.
- Disabled buttons use patchbay's own disabled look (A1-12 over A2-22).
- A1-10's drop-down name list goes ahead, with a check that the pad still fits at 1366x768. Its order is A1-10, A1-11, A1-28.
- The Grid matches the `prototype/panels` sizes (owner's rule), so C2-44 goes ahead after owner call 1. Check the speed at 2500x750 after it.

## Owner calls, with Fable's recommendation

1. **4K scaling: keep scaling by the root font, and make the existing rule steeper.** Do not use B2-27's clamp. The root becomes 150% at 3840x2160 (24px; Grid 21.6px). It stays 16px up to 2400, and ultrawides are bounded by height. Change `web/base.css:13-20` to `clamp(100%, 100% + (min(100vw, 177.78vh) - 2400px) / 180, 150%)` (Grid: `clamp(90%, 90% + (min(100vw, 177.78vh) - 2400px) / 200, 135%)`). Update AGENTS.md, docs/agents/web-layout.md, docs/panels-design.md and `gridpanelscale_browser_test.go`. Check at 2560x1440, 3440x1440 and 3840x2160.
2. **Buttons panel width: full width by default, fitted to the window height.** A kiosk or a panel with no title bar has no button today, so wall panels are stuck at the 1600px cap. Default `sode-panel-full` to on when it has no value (the key and its values do not change). Force it on in kiosk mode and with no title bar. Cap the grid's width by the window height so it does not scroll. Let cell text grow with the cell, with the panel's Font size as the multiplier. Keep the existing "Normal width" and "Full width" words. Then do C2-36.

## Proposed parallel work

Stream 1 must merge before the others start. It also takes `modal.js`, `tooltip.js`, the `--sm` decision, a test that every `var(--pb-*)` is defined, the 4K rule and `routerName` (A2-08 and C1-12). The streams still share some files (`base.css`, `index.html`, `lock.css`, `history.css`, `panels.css`, `settingssave.js`, `setupwizardrules.js`), so give each shared entry to one stream as `second-pass.md` lists, and expect a few hand merges. Streams that change Go (A1-18 `pin.go`, C1-13, C1-25) run `go test -short ./...`. Each stream greps `TestChrome*` and `web/test` for the strings and selectors it changes, runs the STE and vocabulary tests, and adds a `changelog.d` fragment.

| # | Stream | Entries | Main files |
| --- | --- | --- | --- |
| 1 | Shared base rules, 4K | shared causes 1, 2, 5, 6, 7, 10; A2-06, B2-05/06/08/09/10/15/16/27, C1-19 | web/base.css, web/core.js, web/routerlinkrules.js |
| 2 | Setup wizard | A1-01, A1-04 to A1-08, A1-13 to A1-26, A1-29 | web/setup*.js, web/setup.css |
| 3 | Lock screen | A1-02, A1-03, A1-09 to A1-12, A1-27, A1-28, C1-26 | web/lock.js, web/lock.css (lock part) |
| 4 | Main page, top bar, banners | A2 | web/desktop*, history*, bar.css, statusinfo.js, lockchip.js |
| 5 | Settings: Routers, Tie lines, Protects, Save | B1 | web/routers*, settingstielines.js, settingsprotects*.js, savereview.js |
| 6 | Settings: the rest, user dialogs | B2 (not the base rules) | web/users*.js, profiles.js, accesspreset.js, lock.css (dialog part) |
| 7 | Communication Log, phone | C1 | web/log*, uptime*, mobile.css, touch.css, panelsbar.js |
| 8 | Panels list, dialogs, editor | C2-01 to C2-33 | web/panels*.js, editor*.js, panels.css |
| 9 | Panel pages: Buttons and Grid | C2-34 to C2-53 | web/panelview.js, gridpanel*.js, gridpanel.css |

| 10 | Gap captures and review | 2560x1440, 3440x1440, 1280x800 and 1366x768 touch, phone Settings and History, dark contrast, keyboard focus order, reduced motion, features with no finding (second-pass.md, Gaps) | scratchpad only; findings go in this folder |

Run stream 10 while streams 4 to 9 run, so its findings land in the same round.
