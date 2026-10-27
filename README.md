# Sode

Hyper-light, keyboard-first web patching for SW-P-08 routers (target: Ross Ultrix).

Type a **source**, Tab, a **destination**, Tab, **levels** (`1`, `1-3`, `4-17`, `1,3,5-8` or `1+3+5-8`; `!` leaves levels out: `!1` is every level but 1, `1-8!3` is 1-8 without 3; words work too: `t`, `thru` or `through` for `-`, `n` or `not` for `!`, `and` for `,`, as in `1 thru 8 not 3`; on a phone's number pad `.` for `,` and `..` for `-`, as in `1..8.10`; blank = default; with V + A numbering also `v,a1-a8`), Enter, then press the churning confirm key.

Ctrl+S, Ctrl+D and Ctrl+F (Control on a Mac too) fill Source, Destination(s) or Levels from your last take and move on (Source to Destination, Destination back to Source). After a reload, they use the newest Sode take in history. Each box shows that value as a small chip by its label (Levels only when they aren't the default); clicking it does the same.

To patch one source to several destinations, press `+` (or the + button) instead of Tab after each destination: it becomes a chip and the box starts over. Each destination can only be added once; Backspace in the empty box takes the last one back to edit. The takes land in history as one folder with an overall status and **Undo all**; open it to undo or retry destinations one by one. Protected destinations in the set are refused and show as failed; the rest go ahead.

**Other panels (beta).** Routes made elsewhere (Ultrix panels, Companion) show up in Recent too, with a **panel** status, and can be undone like any take. A panel take arrives as one change per level, so Sode collects them until they stop for half a second, then logs one entry per destination and source. A burst that changes several destinations becomes one folder. If the same destination is punched on the same levels again within 30 seconds, with nothing else routed there in between, the punches show as one row ("3 changes"); its Before lists each change, and Undo goes back to the state before the first punch.

The router also moves destinations by itself, such as a multiviewer PiP that follows a DSM. When the same source lands on another destination within half a second of a Sode take, Sode logs it as a **follow** of that take ("+1 follow") rather than a row of its own. Before lists the follows. Undo leaves them out unless you tick "Also undo … suspected automation" in the confirm (Space on desktop), because undoing the take lets the router redo its own automation. Each take of a Sode salvo has its own follows and its own Undo. A panel salvo of one source to several destinations stays one folder. Beta limits: routes changed while Sode was disconnected are not logged, and a change that arrives before Sode has read the router's routes has no Before, so it can't be undone.

**Search.** Filter by Source and Filter by Destination above Recent work like the patch boxes. Typing lists matching sources or destinations, including names logged in history, so a source the router has since renamed is still found. Pick one with ↑↓ (or Alt+J/K) and Enter, Tab or `+`, and Recent shows every entry for it, from the whole log. Each pick becomes a chip, so you can add more, and Recent shows entries for any of them. Backspace in an empty box removes the last pick. A take also shows if one of its follows matches. You can type an expression instead (`1-50`, `MV, !PGM`, `/^DSM/`), which applies as typed. Search ignores Filters unless Settings → This device → **Search respects filters** is on. Esc closes the list, then clears the box. On desktop, Shift+S and Shift+D jump to them from keyboard mode.

**Filters.** The **Filters** button in Recent's header scopes what Recent shows *in this browser*. It reads "Filters · 2" in blue while any are on.
- Pick which entries to show: Sode takes, panel changes, and panel changes to globally hidden destinations.
- Narrow sources and destinations, either as rule rows (Visual) or typed (Code). Both write the same language.
- Everything applies as you type, with a live count of what matches.
- **Share link** copies a URL that applies the same filters in another browser. The URL is cleaned once read, and a banner offers Undo.
- Desktop keyboard mode opens Filters with `/`.
- **Filter sets** are named filters, shown as quick buttons beside Filters (e.g. **All**, **All except shader**, **My destinations**). Click one to switch. A `*` means you've edited it since; click it again to go back.
  - The buttons come in groups separated by `|`: **All**, then the sets for everyone, then this browser's own.
  - Every set is managed in the Filters dialog's Set row, whichever group it's in:
    - **Save as…** names the current filters "For this browser" or "For everyone".
    - **Save** updates the set you've edited.
    - **Rename** and **Delete** (which asks twice) work on any set.
    - Sets for everyone are settings: with a settings PIN, the dialog asks for it the first time, and locks again when it closes.
    - Every browser gets sets for everyone at once. A browser using one follows changes to it, unless it has edited it.
    - Settings → History lists the sets for everyone.
  - A shared link carries the set's name, and the banner offers to save it as a set in that browser.

| Typed | Matches |
|---|---|
| `65` | number 65 |
| `1-50`, `1 thru 50`, `1 to 50` | numbers 1 to 50 |
| `PIP` | names containing PIP (any case) |
| `'NOT USED'` | names containing that text, taken literally (for text that looks like a number, has a comma, or starts with "not"; `''` for a quote inside) |
| `"CAM 1"` | the name exactly (any case), so not CAM 10 |
| `/^DSM\d/` | names matching the regex (any case unless flags are given) |
| `!65`, `!PIP`, `not 'PIP'` | excludes (`not` only before a number, quote or regex, so a bare `NOT USED` is a name) |

Terms are separated by commas. A name matches if it fits any plain term and no `!` term, and only `!` terms means everything but those. So `MV, !MV 1` means every multiviewer except the ones whose names contain "MV 1".

**Globally hidden destinations** (Settings → History) uses the same language to hide very noisy destinations, such as a shader's monitor, from everyone's Recent. Changes to them are still recorded, and a station whose Filters include globally hidden destinations (say the EIC's) sees and can revert everything.

- Single static Go binary, ~15MB distroless image
- Themed with [patchbay](https://github.com/sethdotfm/patchbay), vendored and embedded
- Serves plain HTTP on :8080; put it behind your own TLS proxy (e.g. Caddy)
- Phones get a text-message style layout: type a source, Next, a destination (+ for another), Next, levels, Go, then slide to confirm. While you type, a summary of the patch stays at the top and Currently shows what's on the destination. Leave Source blank and press Next to pick the destination first. Swipe a recent take or folder left to undo. Settings → This device (each choice can also stay on **Global**, the default set for everyone in Defaults for every device) can put Destination before Source (on desktop the arrow between the boxes does the same), force the phone or desktop layout, pick a 24-hour (default), 12-hour or browser-language clock, and turn off scrolling back to the top when a box is focused.

## Run

Against the built-in simulator:

```sh
docker compose --profile sim up --build
# open http://localhost:8080
```

The simulator can fake a router that never confirms some destinations, to exercise timeouts and Retry: set `SIM_IGNORE_DSTS=9,12` (0-based destination numbers) on the `swp08sim` container. `SIM_FOLLOWS=2>40,3>41` makes destination 40 follow 2 and 41 follow 3, like a multiviewer PiP following a DSM. `SIM_PANEL_SECONDS=5` makes it route something by itself every 5 seconds, like a panel. Every other punch goes to `SIM_PANEL_DST` (default 3), so repeated punches can be seen merging. With `SIM_PANEL_SILENT=1` those routes aren't announced, as the Ultrix sometimes doesn't announce routes, so Sode only finds them at the next tally resync and logs them as `found`.

Against a real router (first start):

```sh
SODE_ROUTER=10.0.0.50:8910 docker compose up -d --build
```

## Settings

Settings live in `/data/config.yaml` on the `sode-data` volume and are edited from the **Settings** button at the bottom of the page (a contents list beside or above the groups jumps to each one). Saving applies immediately; changes to the router, matrix, levels or name options reconnect and reload names and routes.

On first start, when there is no config file yet, it is created from these seed variables. After that the file wins and the variables are ignored.

| Seed var | YAML key | Default | Meaning |
|---|---|---|---|
| `SODE_ROUTER` | `router` | | Router `host:port`. Empty = idle until set in Settings |
| `SODE_MATRIX` | `matrix` | `0` | SW-P-08 matrix (0-15 with standard name requests) |
| `SODE_LEVEL_COUNT` | `levels` | `17` | Number of levels; 1 gets the video swatch, the rest audio |
| `SODE_LEVEL_WIRE_OFFSET` | `levelWireOffset` | `-1` | Wire level = level + offset. `-1`: level 1 → wire 0 |
| `SODE_DEFAULT_LEVELS` | `defaultLevels` | `1-<levels>` | Used when the levels box is blank. Always 1-based in the file; Settings shows it in the chosen numbering |
| `SODE_LEVEL_NUMBERING` | `levelNumbering` | `1` | How levels are shown and typed, for everyone: `1` (1-17), `0` (0-16) or `va` (V, 1-16: like `0` but level 0 shows as V, and `v` can be typed) |
| `SODE_NAME_LEN` | `nameLength` | `12` | Name length to request: 4, 8, 12 or 16 |
| `SODE_NAMES` | `nameRequests` | `std` | `std` (cmd 100/102) or `ext` (cmd 228/230) |
| `SODE_NUM_OFFSET` | `numberOffset` | `1` | Added to router numbers for display (Ultrix shows 1-based) |
| `SODE_CONFIRM_LOCKOUT_MS` | `confirmLockoutMs` | `500` | Confirm key ignored for this long after the dialog opens |
| `SODE_CONFIRM_ENTER` | `confirmEnter` | `false` | Also allow Enter to confirm |
| `SODE_NAME_REFRESH_S` | `nameRefreshSeconds` | `60` | Re-read names this often (0 = off); also a button in Settings |
| `SODE_TALLY_RESYNC_S` | `tallyResyncSeconds` | `300` | Re-read all routes and protects this often (0 = off) |
| | `routerName` | | Friendly name for the router in the status card, e.g. `Ultrix` (up to 32 characters). Settings only |
| | `routerShow` | `address` | What the status card shows for the router: `address`, `name` (falls back to the address when no name is set) or `none`. The disconnected banner always shows the address. Settings only |
| | `levelColors` | `[]` | Per-level `#rrggbb`, level 1 first; blank keeps the default (the Ultrix colors for 1-17, built-in swatches after). Settings only, no seed var |
| `SODE_TRUSTED_PROXIES` | `trustedProxies` | | IPs/CIDRs whose `X-Forwarded-For` is believed (your Caddy). Others are logged by peer address |
| | `filterSets` | `[]` | Named Recent filters offered to every browser as quick buttons: `name`, `src`, `dst` (filter expressions), `sode`, `ext`, `hidden`. Settings only |
| | `hiddenDsts` | | Filter expression (see Filters) for destinations whose changes from other panels every browser's Recent hides unless its Filters show them. Still recorded. Settings only |
| | `deviceDefaults` | `theme: system`, `layout: auto`, `clock: "24"`, `flip: false`, `scrollTop: true`, `searchFilters: false` | What Settings → This device follows in a browser that hasn't chosen its own: theme (`system`, `light`, `dark`), layout (`auto`, `phone`, `desktop`), clock (`"24"`, `"12"`, `locale`), Destination before Source, scroll to the top on focus, search respects filters. A browser's own choice wins; **Global** in its dropdown goes back to this. Settings → Defaults for every device; applying never reconnects. Settings only |
| `SODE_PIN_MAX_TRIES` | `pinMaxTries` | `10` | Wrong settings PINs in a row, from anyone, before settings lock until the PIN is reset or Sode restarts. `0` = never |
| | `settingsPin` | | Hash of the settings PIN (set from Settings → Access). Settings only, no seed var |

### Settings PIN

Settings → Access can set a PIN or password (4-64 characters) that is asked for every time the Settings view opens; closing it locks again. Patching itself stays open. Wrong tries count for everyone, whatever address they come from: from the 5th in a row there's a wait (30 seconds, doubling each time, up to an hour), and after `pinMaxTries` (default 10) settings stay locked until the PIN is reset or Sode restarts. Only a salted PBKDF2 hash is stored, as `settingsPin` in the config file.

Forgot it? Either:

```sh
docker compose exec sode /sode -reset-pin && docker compose restart sode
```

or delete the `settingsPin:` line from `/data/config.yaml` and restart. The running server keeps the PIN in memory, so the restart is needed either way.

Process settings stay in env: `SODE_LISTEN` (`:8080`), `SODE_CONFIG` (`/data/config.yaml` in the image), `SODE_HISTORY` (`/data/history.jsonl`), `SODE_SENTLOG` (`/data/sent.jsonl`) and `SODE_SENTLOG_MB` (`64`, the sent log's total size on disk), and `SODE_DEBUG=true` to hex-dump all SW-P-08 traffic.

## Sent log

`/log` (**Open Log** in Settings, there even while Settings are locked) lists every message Sode wrote to the router, newest first and live: connects, interrogates, dumps, name requests and keepalives. Each row has the level, destination and source (with names as they were when it was sent), what it was for (a take, undo or retry, with the browser's address; the check before a take; a resync; a name refresh; a link check) and what the router made of it: ACK, ACK after retries, NAK, no ACK, or never sent because the link was down. It's read-only. It opens with the newest 100 entries; **Load older** reads 100 more. **From**/**To** limit it to a time range, and setting From reads the log back as far as it needs to cover the range. By default it shows connects only; the filters (kinds, problems only, source and destination expressions in Recent's filter language) are kept per browser. The log is `SODE_SENTLOG`, kept to `SODE_SENTLOG_MB` across it and one older file (`sent.1.jsonl`); without `SODE_SENTLOG` it keeps the last 20,000 messages in memory. A damaged log file doesn't stop Sode: it's renamed to `….bad-<date>` and a fresh one starts.

## Protocol notes

Crosspoints always use extended commands (0x82 connect, 0x84 connected, 0x95/0x97 tally dump), so source and destination numbers can go past 1024. A take only reports success once the router sends Connected for every requested level (3s timeout). Messages queued while the router is disconnected are discarded rather than sent after reconnect.

Protects are read with the extended protect tally dump (0x93 → 0x94). The Ultrix packs each destination as two bytes: protect state in bits 4-5 of the first, device number in the rest. Takes to a protected destination/level are refused before anything is sent.

Names refresh into a fresh list that is swapped in once replies stop, so names deleted on the router disappear. The router does not announce name changes, so this polling is the only way to see them.

See [PLAN.md](PLAN.md) for the roadmap.
