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
| Page border | `--brand-color` from Branding (`[data-brand]` in `web/base.css`) | The border round the page: which server this is | The site name in the title bar |
| Stepped in | `--pb-signal-10` (`.sode__stepframe`, `.sode__stepbar`) | The border and bar while a user is stepped in. No Branding color uses this hue | "Bo stepped in" with Step out in the bar |

The Communication Log (`/log`) and later monitoring pages show no identity fills, only state colors. A level chip with its label is the one exception.

## State

| Role | Token or class | Where it can show | Non-color cue |
|---|---|---|---|
| Failure (red) | `--pb-status-error`, `pb-badge--error`, `pb-note--error`, `pb-alert--error`, `pb-toast--error` | A lost link (banner and page outline), a failed or refused Take, invalid input, an offline panel page, a Degraded or Down router's icon. Nothing else is red | The banner's words and the wire glyphs, the failure text, the `circle-x` state glyph, "Offline", the invalid field's note |
| Caution (amber) | `--pb-status-warn`, `--pb-commit`, `pb-btn--commit`, `pb-badge--warn`, `pb-note--warn` | A warning, unsaved work, and every Take control (armed or not) on every page: the main page's Take and Confirm, the phone's Go, sheet and slider, the Grid popup's Take, the Buttons and Grid panels' Take, the staged label under a Grid panel destination. Also an Undo swipe and Save and reconnect | The word Take with its key ("Take (T)"), the armed pulse, "Unsaved changes", "staged", the warning text, the `loader-circle` state glyph |
| Routed (green) | `--pb-status-ok`, `pb-alert--ok` | The current source on a destination (panel pages), and a Take that just succeeded (a Take resolved banner, a panel cell's connected ring). Nothing else is green | The source name in the cell, the banner's words ("Take resolved"), the History row's "ok" |
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

Info is neutral: info badges and notes are a plain `pb-badge` or text, with the badge's square. Blue has no state job.

A Tie line picker shows a Protected or a Reserved tie line in grey (`--pb-text-disabled`). Its words say who set it and when, or which panel reserves it. A person cannot pick it.

### Buttons

- The main action of a form or dialog is `sode-btn--primary` (`web/base.css`): a `pb-btn` filled with `--pb-accent`, with `--pb-text-on-accent` text. It is meant to move into patchbay. `pb-btn--ok` is not used.
- A Save that switches to `pb-btn--commit` while changes are unsaved keeps doing that.
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
