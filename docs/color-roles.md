# Color roles

Each color in Sode has one job. A normal, healthy state shows no color (dark cockpit). The reasons are in [ADR 0013](adr/0013-dark-cockpit-color-roles.md).

The rules:

1. A normal, healthy state shows no color.
2. Color shows only a state that needs attention or a change in progress.
3. Each color has one job.
4. Every colored state also has text, an icon or a shape. Patchbay's red and green are almost the same lightness, so this cue is what lets a color-blind user tell them apart.

There is no setting for a quieter or a louder mode.

Colors are of two kinds. **Identity** colors say what a thing is. They can always show. **State** colors say what is happening. They follow the rules above on every page.

## Identity

| Role | Token or class | Where it can show | Non-color cue |
|---|---|---|---|
| Source | `--pb-source` (sage) | Source ends, patch boxes and chips on the main page and every page that is not a panel page | The word Source, the box label, the "from" side of a route |
| Destination | `--pb-dest` (khaki) | Destination ends, patch boxes and chips, the plan items in a Confirm | The word Destination, the box label, the "to" side of a route |
| Level | `--pb-signal-*`, or the level's own color (`levelStyle` in `web/levels.js`) | Only inside a level chip that shows the level's label. Never as a state, a fill or an outline anywhere else | The level's label in the chip |
| Tie line | `--pb-tieline` (fern) | Tie lines only: the wires in the Tie line picker and the Via marks | The word Via, the tie line's name |
| Theme color | `--brand-color` from Branding (`[data-brand]` in `web/base.css`) | The border round the page: which server this is. Sode makes the accent from it (see Accent) | The site name in the title bar |
| Stepped in | `--pb-signal-10` (`.sode__stepframe`, `.sode__stepbar`) | The border and bar while a user is stepped in. No offered Branding color uses this hue | "Bo stepped in" with Step out in the bar |

The Communication Log (`/log`) and later monitoring pages show no identity fills, only state colors. A level chip with its label is the one exception.

## State

| Role | Token or class | Where it can show | Non-color cue |
|---|---|---|---|
| Failure (red) | `--pb-status-error`, `pb-badge--error`, `pb-note--error`, `pb-alert--error`, `pb-toast--error` | A lost link (banner and page outline), a failed or refused Take, invalid input, an offline panel page, a Degraded or Down router's icon. Nothing else is red | The banner's words and the wire glyphs, the failure text, the `circle-x` state glyph, "Offline", the invalid field's note |
| Caution (amber) | `--pb-status-warn`, `--pb-commit`, `pb-btn--commit`, `pb-badge--warn`, `pb-note--warn` | A warning, unsaved work, and every Take control (armed or not) on every page: the main page's Take and Confirm, the phone's Go, sheet and slider, the Grid popup's Take, the Buttons and Grid panels' Take, the staged label under a Grid panel destination. Also an Undo swipe and Save and reconnect. In the Tie line picker, the left bar and the count badge of a tie line whose take changes Affected destinations | The word Take with its key ("Take (T)"), the armed pulse, "Unsaved changes", "staged", the warning text, the `loader-circle` state glyph, the count badge ("3 change") |
| Routed (green) | `--pb-status-ok`, `pb-alert--ok` | The current source on a destination (panel pages), and a Take that just succeeded (a Take resolved banner, a panel cell's connected ring). In the Tie line picker, the left bar of a tie line that carries the source already: the source is the current source on its upstream destination. Nothing else is green | The source name in the cell, the banner's words ("Take resolved"), the History row's "ok", the picker's "Has it" with its check |
| Selection, focus and the main action (accent) | `--pb-accent`, `sode-btn--primary`, `pb-btn--selected` | The main action of a form or dialog, the selected item in a list (the panel on /panels and in the panel picker), focus rings, the Find result that was found | The button's words, `aria-current`, the focus ring's shape |
| Protected (magenta) | `--pb-protect` | Protected destinations (a Protect or a Router lock), as today | The shield icon and the word Protected or Router lock |
| Destination on a panel page (orange) | `--pb-view` | Panel pages only (see Panels) | The cell's name |

Healthy states are neutral:

- The Server and Router state icons when connected (`--pb-text`, with the `circle-check` glyph).
- The ok bars of the uptime graph (`--pb-text-muted`).
- The ok PIN dots on the lock screen ("Welcome").
- The panel status when ok, and the toast timer ring.
- Online on /panels.
- Approval cards that wait, are approved or are cancelled.
- A result that is not a Take, for example "Theme changed", "History cleared for everyone", "Import done", "Protected" or "Router names updated". Its banner has the kind `done` in `web/banner.js`: neutral, and it closes itself as a Take's does. The banner's countdown bar is the accent.
- The "Yes" marks in the setup wizard's presets table.

Info is neutral: info badges and notes are a plain `pb-badge` or text, with the badge's square. Your own Protect's chip is a plain `pb-badge` with its shield. A plain `pb-alert` or `pb-toast` is neutral too. Examples are a result banner of kind `info`, "Imported.", the Inbox toast and a panel back online. Patchbay makes them blue, so `web/base.css` sets `--pb-text-muted` and `--pb-surface-raised` for them. Blue has no state job.

Write each status class (`pb-btn--commit`, `pb-alert--warn`, `pb-badge--error`) in full, never from a template such as `` `pb-alert--${kind}` ``. Map a kind to a class with a table, so `web/test/colorroles.test.mjs` sees every color.

A Tie line picker shows a Protected or a Reserved tie line in grey (`--pb-text-disabled`). Its words say who set it and when, or which panel reserves it. A person cannot pick a Protected one, and a Reserved one asks one more time.

Each choice of the Tie line picker shows its state at a glance (`choiceLook` in `web/tiepickrules.js`). The bar is on the left and has the width `--pb-accent-bar-w`, as the bars of the source and destination chips.

- Has it: the tie line carries the source already. Routed bar and "Has it".
- In use: the take changes Affected destinations. Caution bar and a `pb-badge--warn` with the count ("3 change").
- Free: the take changes nothing else. No bar and no color.
- Best: the first choice that the planner ranks usable. A neutral double outline (`--pb-text`) and a plain `pb-badge` "Best". It can have any other state.

## Accent

With a theme color, the accent (`--pb-accent`, `--pb-accent-hover`, `--pb-accent-wash`, and the `--pb-accent-*` tokens that patchbay mixes from `--pb-accent`) is that color. This shows which server this is on every page that shows the brand: the main page, /log, /panels and the panel pages. With no theme color, the accent is patchbay's periwinkle.

The rule is in `web/accent.js` (tests in `web/test/accent.test.mjs`):

- Sode keeps the OKLCH hue and chroma of the theme color, and moves only its lightness, for each theme. It moves it only as far as necessary for two checks. Accent text has at least 4.5:1 on every surface of that theme (`--pb-surface-*`). `--pb-text-on-accent` has at least 4.5:1 on an accent fill. Light makes it darker, dark makes it lighter. The sRGB gamut can lower the chroma.
- The hover color is 0.07 darker (light) or lighter (dark), with the same check.
- The page border itself keeps its own color. Only the accent changes.
- The Step in frame and the red lost-link outline do not change.
- `brand.js` sets the tokens on the root when the hello comes, and keeps them in the `localStorage` key `sode-accent`. `prepaint.js` sets them from that key before the first paint, so a reload shows no periwinkle first. No theme color removes the key.

### Reserved hues

These OKLCH hues are measured from patchbay 2.2.1 in both themes:

| State | Hue |
|---|---|
| Failure (red) | 24 to 26 |
| Caution (amber, Take) | 59 to 81 |
| Tie line (`--pb-tieline`) | 127 to 129 |
| Routed (green) | 155 to 159 |
| Protected (magenta) | 347 to 350 |

A hue in a range or 30 degrees from it is in the reserved zone. An accent there can look like a state, or a selected tie line can look like any other tie line. The free band is about 189 to 317. Step in (`--pb-signal-10`, about 320) is in protected's zone. A grey (OKLCH chroma below 0.02) has no hue and is never in the zone.

Branding offers Blue, Teal, Violet and Lilac (`--pb-signal-16`, hue about 315), all in the free band, and Custom. In both themes, each offered color is at least 0.1 from each other one in OKLab distance (`distance` and `MIN_APART` in `web/accent.js`). So no two look alike (web/test/accent.test.mjs). Green, amber and pink are retired (`Retired` in `config.BrandColors`), since they look like states. Sky and Slate are retired, since they looked too much like Blue and Teal. A config with a retired color still loads and draws, and the picker shows it while it is the saved value. Sode never refuses a custom color. For a color in the reserved zone, Settings and the setup wizard show an amber caution note with an icon under the picker. The accent still uses the color.

### Buttons

- The main action of a form or dialog is `sode-btn--primary` (`web/base.css`): a `pb-btn` filled with `--pb-accent`, with `--pb-text-on-accent` text. It is meant to move into patchbay. `pb-btn--ok` is not used.
- A Save that switches to `pb-btn--commit` while changes are unsaved keeps doing that. With no unsaved changes it is `sode-btn--primary` (the Presets and Bookmarks editors). A failed or refused save shows in red in the note, not on the button.
- A button that only deletes or discards is `pb-btn--outline`. The Confirm (churning letter and lockout) guards it.
- A button that only opens something and saves nothing is a plain `pb-btn` (New panel on /panels).
- `pb-btn--danger` is left in one place, since it shows a failure: the panel editor's **Retry save** (`web/editortoolbar.js`), after a save failed twice.

## Panels

Panel pages (`/panels/<id>`, the Grid panel and the panel editor) keep their colors. A panel is an input device.

| Role | Token | Cue |
|---|---|---|
| Source cell, idle | `--pb-current-idle` (green wash) | The source name |
| Destination cell, idle | `--pb-view-idle` (orange wash) | The destination name |
| Current source | `--pb-status-ok`, solid | The source name; "current" for a screen reader |
| Viewed destination | `--pb-view`, solid | The destination name, the outline when viewed |
| Staged destination, pending source | The pulse between the idle and the state color, with a ring | The ring, "staged" |
| Grid panel source and destination bars | `--pb-source`, `--pb-dest` | The bar's words, "Sources" and "Destinations" |
| Take cell and Take button | `--pb-commit` (amber), with the armed pulse | The word Take |
| Grid panel level pie | The level colors | The level picks' labels |

Panel status marks (Broken, Name moved) are an icon with optional text (docs/panels-design.md).
