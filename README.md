# Sode

Hyper-light, keyboard-first web patching for SW-P-08 routers (built and tested on a Ross Ultrix).

**Version:** Beta 1.5

Type a **source**, Tab, a **destination**, Tab, **levels**, Enter, then press the churning confirm key. Press `?` in keyboard mode (Esc with every box empty) for the shortcut list.

- Single static Go binary, small distroless image, plain HTTP on :8080 (put your own TLS proxy in front).
- Always extended SW-P-08 commands (16-bit source/destination), protects honored, auto-reconnect.
- Names come from the router and are re-read periodically.
- Phones get a text-message style layout; the desktop is keyboard-first.
- Themed with [patchbay](https://github.com/sethdotfm/patchbay), vendored and embedded: no external fetches.

## Run

`compose.yaml`:

```yaml
services:
  sode:
    image: ghcr.io/sethdotfm/sode:latest
    restart: unless-stopped
    ports: ["8080:8080"]
    volumes: ["sode-data:/data"]
    environment:
      SODE_ROUTER: 10.0.0.50:8910   # your router's SW-P-08 host:port

volumes:
  sode-data:
```

```sh
docker compose up -d     # then open http://localhost:8080
```

To try it without a router, run the bundled simulator from a checkout: `docker compose --profile sim up --build`.

**Install as an app.** Sode is a web app: use *Install* in Chrome or Edge, or *Add to Home Screen* in Safari on an iPhone or iPad, and it opens in its own window. Chrome and Edge only offer Install over HTTPS (or on localhost), so put your TLS proxy in front first. The app name and icon follow **Logo text** and **Border colour** in Settings, so several instances stay apart on a home screen. Android and desktop Chrome pick up a changed name or icon on their own; on iOS, remove the app and add it again. It still needs the server to run, so there is no offline mode.

**Behind a proxy.** Set `trustedProxies` (`SODE_TRUSTED_PROXIES`) to the proxy's address so history shows real client IPs; the server logs a warning when it sees `X-Forwarded-For` and the setting is empty. The proxy must pass WebSocket connections (`/ws`) and keep them open while idle for at least 15 seconds. The server sends a small heartbeat every 5 seconds and the page treats a connection that has been silent for 15 seconds as dead: it reconnects, and a patch is refused ("NOT sent") if the connection has been quiet for more than about 7 seconds, rather than being lost.

## Using it

**Levels** accept `1`, `1-3`, `1,3,5-8` (or `+`), `!1` (all but 1), `1-8!3`, and words: `thru`/`t` for `-`, `not`/`n` for `!`, `and` for `,`. On a phone's number pad `.` is `,` and `..` is `-`. Blank means the default levels. With V + A numbering, `v,a1-a8` also works.

**Several destinations:** press `+` (or Ctrl+Enter) after each destination instead of Tab. The takes land in history as one folder with **Undo all**.

**Presets.** Buttons above Recent that take a saved set of sources to destinations in one confirm. **Edit** (or **Presets…** when there are none) opens the editor: each take is a source, destinations and levels (numbers or names; blank levels = the default levels), and **Fill from patch boxes** or **Fill from latest folder in Recent** adds takes for you. Save for **This browser** or **Everyone** (Settings PIN needed, like filter sets). They are stored as numbers, so a rename on the router changes nothing, and a number the router no longer has is left out and listed in the confirm. A preset is one batch, so it is one folder in Recent and Undo all undoes it, marked "Preset: name" (quick search finds it by that name too). Only **pinned** presets are buttons above Recent; **Ctrl+g** (**⌘g** on a Mac; the **Presets** button on a phone) opens a popup that lists all of them with a search box: arrows or Ctrl+j/k to move, Enter fires through the usual confirm, Esc closes. Each row has **Pin/Unpin** for this browser and, for everyone's presets, **Pin for everyone** (Settings PIN); the editor has a Pinned checkbox. In keyboard mode (Esc with the boxes empty) **p** jumps to the pinned presets. A preset is one confirm of several takes, so it is sent the way the **Fire confirms of several takes** setting says (Settings → Takes; see `SODE_FIRE_MODE` above).

**Last take:** Ctrl+s, Ctrl+d and Ctrl+f fill Source, Destination or Levels from your last take; each box shows it as a small chip you can click. Ctrl+Shift+s and Ctrl+Shift+d do the same with the next source and destinations (every destination shifted), for patching a run in order: Cam 1 → Rec 1 + Rec 5, then Cam 2 → Rec 2 + Rec 6. Each box also has a small "Next" chip that opens to show the value when you hover it or hold Shift. Settings → This device → "Next means" picks the next number or the next match of the name's text.

**Recent** lists every take, shared by all browsers and kept in a log. Undo restores what the router actually had before. In a row's Before panel (and the steps of a stacked row) each source is a button that fills Source with it and moves on to Destination. Routes made elsewhere (panels, Companion) appear too, marked **panel**, and can be undone the same way; every row has **Redo** (or **Retry** if its take failed; **Redo all** on a folder), which takes that line's route again, and the confirm says when the destination already has it (keyboard mode: `r`); changes the router made without announcing them are marked **found**. Destinations that follow another (a multiviewer PiP following a DSM) are shown as **follows** of the take; undo leaves them out unless you tick the box in the confirm. Routes changed while Sode was disconnected aren't logged.

**Search and filters.** Filter by Source / Destination above Recent, and the **Filters** button (`/` in keyboard mode) scope what Recent shows, per browser. Filters can be named sets, shared with everyone from Settings, or shared as a link. Click several set buttons in Recent's header to stack them (click one again to take it off; none left is All). Under Settings → This device, **Stacked filter sets** says whether an entry must match all of them (the default) or any one. They use one small expression language:

| Typed | Matches |
|---|---|
| `65` | number 65 |
| `1-50`, `1 thru 50`, `1 to 50` | numbers 1 to 50 |
| `PIP` | names containing PIP (any case) |
| `'NOT USED'` | names containing that text, taken literally (`''` for a quote inside) |
| `"CAM 1"` | the name exactly (any case), so not CAM 10 |
| `/^DSM\d/` | names matching the regex (any case unless flags are given) |
| `!65`, `!PIP`, `not 'PIP'` | excludes |

Terms are separated by commas. A name matches if it fits any plain term and no `!` term; only `!` terms means everything but those.

**Globally hidden destinations** (Settings → History) uses the same language to hide noisy destinations, such as a shader's monitor, from everyone's Recent. They're still recorded.

## Settings

Edited from the **Settings** button at the bottom of the page, stored in `/data/config.yaml`. Saving applies at once; changes to the router, matrix, levels or name options reconnect and re-read names and routes. When there's no config file yet, it is seeded from these variables, then the file wins:

| Seed var | YAML key | Default | Meaning |
|---|---|---|---|
| `SODE_ROUTER` | `router` | | Router `host:port`. Empty = idle until set in Settings |
| `SODE_MATRIX` | `matrix` | `0` | SW-P-08 matrix (0-15 with standard name requests) |
| `SODE_LEVEL_COUNT` | `levels` | `17` | Number of levels; 1 gets the video swatch, the rest audio |
| `SODE_LEVEL_WIRE_OFFSET` | `levelWireOffset` | `-1` | Wire level = level + offset. `-1`: level 1 → wire 0 |
| `SODE_DEFAULT_LEVELS` | `defaultLevels` | `1-<levels>` | Used when the levels box is blank. Always 1-based in the file |
| `SODE_LEVEL_NUMBERING` | `levelNumbering` | `1` | How levels are shown and typed: `1` (1-17), `0` (0-16) or `va` (V, 1-16) |
| `SODE_NAME_LEN` | `nameLength` | `12` | Name length to request: 4, 8, 12 or 16 |
| `SODE_NAMES` | `nameRequests` | `std` | `std` (cmd 100/102) or `ext` (cmd 228/230) |
| `SODE_FIRE_MODE` | `fireMode` | `sequential` | How a confirm of several takes is sent: `sequential` (one connect after another, ~16 ms per crosspoint on the Ultrix) or `native` (a SW-P-08 group salvo, cmd 120/121; needs a router with group salvos, Ross Ultrix verified). Measured on the Ultrix 2026-11-02: a group Go announces its crosspoints about 4 ms apart (51 in ~210 ms) against ~16 ms apiece for ordinary connects (51 in ~840 ms); whether the switch itself is simultaneous is unproven. Level 17 and numbers past 1024 still go one after another for now. If the router doesn't acknowledge the salvo, Sode clears the salvo group and sends the takes one after another as before |
| `SODE_SALVO_GROUP` | `salvoGroup` | `127` | The router's salvo group (0-127) native salvos use. The router's staged list is shared by every controller, so pick one nothing else uses |
| `SODE_NUM_OFFSET` | `numberOffset` | `1` | Added to router numbers for display (Ultrix shows 1-based) |
| `SODE_CONFIRM_LOCKOUT_MS` | `confirmLockoutMs` | `500` | Confirm key ignored this long after the dialog opens |
| `SODE_CONFIRM_ENTER` | `confirmEnter` | `false` | Also allow Enter to confirm |
| `SODE_NAME_REFRESH_S` | `nameRefreshSeconds` | `60` | Re-read names this often (0 = off) |
| `SODE_TALLY_RESYNC_S` | `tallyResyncSeconds` | `300` | Re-read all routes and protects this often (0 = off) |
| `SODE_TRUSTED_PROXIES` | `trustedProxies` | | IPs/CIDRs whose `X-Forwarded-For` is believed (your proxy) |
| `SODE_PIN_MAX_TRIES` | `pinMaxTries` | `10` | Wrong settings PINs in a row before settings lock until reset or restart. `0` = never |
| `SODE_MAX_PIN_LEN` | `maxPinLen` | `8` | Longest user PIN (4-12 digits). The lock pad stops there and submits on its own only at the longest length in use; shorter PINs are sent with the check key or Enter. Also set in Settings > Users |

Settings-only keys (no seed var): `brandText` (replaces "Sode" in the header and page title, up to 24 characters), `brandBorder` (`blue`, `teal`, `green`, `amber`, `violet`, `pink`; a border round the window, off by default), `routerName`, `routerShow` (`address`, `name`, `none`), `levelColors`, `filterSets`, `hiddenDsts`, `deviceDefaults` (theme, layout, clock, flip, scrollTop, searchFilters, setStack (`and` or `or`), nextMode (`number` or `fuzzy`): what a browser follows until it chooses its own under Settings → This device) and `settingsPin`.

Process settings stay in env: `SODE_LISTEN` (`:8080`), `SODE_CONFIG` (`/data/config.yaml` in the image), `SODE_HISTORY` (`/data/history.jsonl`), `SODE_SENTLOG` (`/data/sent.jsonl`), `SODE_SENTLOG_MB` (`64`), `SODE_UPTIME` (`/data/uptime.jsonl`; without it the router uptime is kept in memory only) and `SODE_DEBUG=true` to hex-dump all SW-P-08 traffic.

### Settings PIN

Settings → Access can set a PIN (4-64 characters) asked every time Settings opens; patching stays open. Wrong tries count for everyone: from the 5th in a row there's a wait (30 seconds, doubling, up to an hour), and after `pinMaxTries` settings stay locked until the PIN is reset or Sode restarts. Only a salted PBKDF2 hash is stored.

Forgot it?

```sh
docker compose exec sode /sode -reset-pin && docker compose restart sode
```

or delete the `settingsPin:` line from `/data/config.yaml` and restart.

### Users and locks (optional)

Off by default: with no users and no locks Sode behaves as before. Settings → Users (a summary and a Manage users… button that opens its own dialog, with its own Save) adds people (a name and a 4-12 digit PIN each; PINs must differ, since the PIN alone says who) and turns on locks per page type: the main screen and the Communication Log. A locked page opens behind a lock screen with a number pad; it unlocks the moment the typed PIN matches, no Enter. The server enforces it per WebSocket connection: until a connection unlocks as a user it is refused takes, name refreshes and clearing history, and (on the log) reading the sent log and uptime graph. Saving settings, filter sets, presets and users needs the settings PIN (or, for settings, filter sets and presets, a trusted user, below), not just any unlocked user, so a locked wall panel can still be set up with the PIN. `/healthz` and `/api/health` stay open. The user's name replaces the IP in history and in the log.

- **Lock again:** Ctrl+L, the Lock button in the corner, or after the idle minutes set in Settings (0 = never). A dropped connection also locks, since unlocking belongs to the connection.
- **Trusted users:** each user can be marked Trusted (Settings → Users). While unlocked as a trusted user, saving the Settings form, filter sets and presets needs no settings PIN. Clearing history, adding or removing users and locks, and setting or removing the settings PIN still need the settings PIN, and so does changing the router address, matrix, levels or level offset, how names are read, the fire mode and salvo group, trusted proxies or the PIN attempt limit (anything that reconnects or weakens a guard). Unticking Trusted takes effect at once.
- **Never log out:** a user marked Never log out gets a random session token when they unlock; the browser keeps it (localStorage) and the server keeps only its hash in `tokens.json` next to the config file (up to 8 per user, the oldest dropped); a token not used for 90 days expires, and a page that stays connected counts as using it. A reload or a dropped connection does not log them out and the idle timer skips them. Ticking the box while they are already logged in, or changing their own PIN, gives their open page a token at once, so the next reload keeps them in. Lock, Log out and Ctrl+L revoke the token. Changing that user's PIN, removing the user or unticking the box ends every one of their sessions. A wall panel logs in once as a trusted user who never logs out.
- **Lock screen without a PIN:** where no PIN lock is on, **Ctrl+L** (or Settings → This device → Lock screen) covers the page until you press the letter it shows, like a take's confirm ("Press J to unlock"), after the confirm lockout; on a touch screen tap Unlock. It guards against keyboard mashing and is not a PIN: it is only this browser page, and a reload clears it. The auto-lock minutes (Settings → Users) put it up when the page is idle, too.
- **Wrong PINs** are counted in two separate classes, each for everyone. The settings PIN is one class, with the settings PIN rules above (waits from the 5th, permanent lock after `pinMaxTries`). User PINs are the other: waits from the 5th that double up to 5 minutes, never a permanent lock, so strangers' guesses can't shut the operators out. A right user PIN does not reset the count (so holding one PIN doesn't give free guesses at the others); the count is forgotten after an hour without a wrong PIN. A success or lockout in one class never changes the other. A PIN of a length no user has is just a wrong try, but the page only sends lengths some user has; with users of different lengths a PIN is sent half a second after the last digit.
- **Settings PIN required:** users and locks are administered with the settings PIN, so it must be set first, and a lock can't be on without one. `sode -reset-pin` therefore turns the page locks off too. If everyone is locked out otherwise, stop Sode and set `locks:` to all `false` in `/data/config.yaml`. Users are `users: [{name, pin (hash), len, trusted, persistent}]`; set PINs in Settings, not by hand.
- **Limits:** a lock guards actions (and, for the log lock, reading the log), not data: names, routes, recent history, user names and PIN lengths keep streaming to a locked browser, by design. Idle auto-lock is a client-side convenience in the page, not enforced by the server.

## Monitoring (Uptime Kuma)

`GET /api/health` reports the server and the router apart. It needs no PIN and shows no settings:

```json
{"server":"up","router":"up","targetAddress":"10.0.0.1:8910","version":"Beta 1.3.1","uptimeSeconds":86400,"browsers":2}
```

`router` is `up`, `connecting` or `down`; `err` appears when the router link has a problem. The status code is 200 only while the router is up and 503 otherwise, so a plain HTTP(s) monitor alerts when the router is lost (a dead server shows as no answer). Uptime Kuma counts a 503 as down even for a JSON Query monitor, so to watch the server alone, add `503` to that monitor's accepted status codes. For a JSON Query monitor, use `router` as the key and `up` as the expected value. `/healthz` is unchanged.

## Communication log

`/log` (**Open the Communication Log** in Settings > History, also shown while Settings are locked) lists every message Sode wrote to the router, newest first and live, with what it was for and whether the router ACKed it. It's read-only. Settings has a search box above its section list that finds sections and single settings by name or by common words ("password", "dark", "proxy"). Kept in `SODE_SENTLOG` up to `SODE_SENTLOG_MB`; without it the last 20,000 messages stay in memory.

Besides messages to the router the log has event rows (#139), which show in the default view and have their own filter chips: **Browsers** (a browser's WebSocket connecting or dropping, with its address), **Commands** (what people fire that changes something: takes, undos and retries, refreshing names, clearing history, saving settings, filter sets or presets, unlocking and locking the settings PIN: whether the PIN was right, never the PIN itself), and **Router link** (the router link going down once per outage, coming up, being changed in Settings, and Sode starting and stopping). ACKed messages also show how long the router took.

The **Router uptime** graph at the top is one strip with toggles for 1 hour, 24 hours (the default view), 7 days and 30 days, each showing its bar length, e.g. "24 hours (15 min)". A toggle also sets the log's From time to that long ago, so the table shows the same period; Clear times undoes it. Setting From or To by hand shows that range instead. Click a bar to see just that slot: the log's times become the bar's, the table is sorted newest first, and a banner says so; **Back** restores the times and sort you had. Each bar is a slot (a minute for an hour, 15 minutes for a day, an hour for a week; hover for the details): green when the router link was up the whole slot, yellow when it was down for part of it or messages failed, needed a retry or took over 300 ms to ACK, red when it was down or Sode wasn't running, grey with nothing recorded. The record is kept for 400 days in `SODE_UPTIME`; Sode not running is found from a stamp it rewrites every minute, so a crash shows as down from the last minute it was alive.

## Protocol notes

Crosspoints use extended commands (0x82 connect, 0x84 connected, 0x95/0x97 tally dump). A take succeeds once the router sends Connected for every requested level (3s timeout); routers send nothing for a connect that changes nothing, so Sode interrogates and counts those as "already". Protects are read with the extended protect dump (0x93 → 0x94) and takes to protected destinations are refused before anything is sent. Messages queued while disconnected are discarded. Names are polled, since the router doesn't announce renames.

## Development

```sh
go test ./...                                        # also run by the Dockerfile
go run ./cmd/swp08sim                                # simulator on :8910
SODE_ROUTER=127.0.0.1:8910 go run ./cmd/sode         # server on :8080
node --test 'web/test/*.test.mjs'                    # browser-side tests
```

The simulator takes `SIM_*` variables to fake misbehaving routers (`SIM_IGNORE_DSTS`, `SIM_SILENT_DSTS`, `SIM_SLOW_DSTS`, `SIM_FOLLOWS`, `SIM_PANEL_SECONDS`, `SIM_PANEL_SILENT`, `SIM_NO_SALVO=1` for a router without group salvos, and `SIM_SALVO_ANNOUNCE=1` to announce a group Go's routes with Connected, as the Ultrix sometimes does); see `cmd/swp08sim`. The web UI is embedded, so restart after frontend edits.

## Releasing

Bump `Version` in `internal/app/version.go` and the `**Version:**` line above, and give [CHANGELOG.md](CHANGELOG.md) a `## Beta <version>` section (drop "(unreleased)" from the top one), then run `go test ./internal/app -run TestVersionNoted` (it checks the README line and the changelog heading; with `SODE_RELEASE_TAG=v<version>` it also checks the tag and refuses an "(unreleased)" heading).

Pushing a `v<version>` tag makes CI check it and publish the multi-arch image to ghcr as `latest` and the tag name; then make a GitHub release from the tag.

## AI disclosure

Sode is written with AI assistance: much of the code, tests and documentation is produced by Claude (Anthropic) through Claude Code, working from issues and direction written by the maintainer, who reviews it and tests it against a real router. Commits made this way carry a `Co-Authored-By: Claude` trailer. Treat it like any software that has not been audited: read it, and try it on a non-critical router first.

## License

Public domain ([Unlicense](LICENSE)). The vendored Lucide icons keep their own license in `web/vendor/patchbay/icons/`.
