# Sode

Hyper-light, keyboard-first web patching for SW-P-08 routers (built and tested on a Ross Ultrix).

**Version:** Beta 1.3

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

**Last take:** Ctrl+s, Ctrl+d and Ctrl+f fill Source, Destination or Levels from your last take; each box shows it as a small chip you can click.

**Recent** lists every take, shared by all browsers and kept in a log. Undo restores what the router actually had before. Routes made elsewhere (panels, Companion) appear too, marked **panel**, and can be undone the same way; changes the router made without announcing them are marked **found**. Destinations that follow another (a multiviewer PiP following a DSM) are shown as **follows** of the take; undo leaves them out unless you tick the box in the confirm. Routes changed while Sode was disconnected aren't logged.

**Search and filters.** Filter by Source / Destination above Recent, and the **Filters** button (`/` in keyboard mode) scope what Recent shows, per browser. Filters can be named sets, shared with everyone from Settings, or shared as a link. They use one small expression language:

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
| `SODE_NUM_OFFSET` | `numberOffset` | `1` | Added to router numbers for display (Ultrix shows 1-based) |
| `SODE_CONFIRM_LOCKOUT_MS` | `confirmLockoutMs` | `500` | Confirm key ignored this long after the dialog opens |
| `SODE_CONFIRM_ENTER` | `confirmEnter` | `false` | Also allow Enter to confirm |
| `SODE_NAME_REFRESH_S` | `nameRefreshSeconds` | `60` | Re-read names this often (0 = off) |
| `SODE_TALLY_RESYNC_S` | `tallyResyncSeconds` | `300` | Re-read all routes and protects this often (0 = off) |
| `SODE_TRUSTED_PROXIES` | `trustedProxies` | | IPs/CIDRs whose `X-Forwarded-For` is believed (your proxy) |
| `SODE_PIN_MAX_TRIES` | `pinMaxTries` | `10` | Wrong settings PINs in a row before settings lock until reset or restart. `0` = never |

Settings-only keys (no seed var): `brandText` (replaces "Sode" in the header and page title, up to 24 characters), `brandBorder` (`blue`, `teal`, `green`, `amber`, `violet`, `pink`; a border round the window, off by default), `routerName`, `routerShow` (`address`, `name`, `none`), `levelColors`, `filterSets`, `hiddenDsts`, `deviceDefaults` (theme, layout, clock, flip, scrollTop, searchFilters: what a browser follows until it chooses its own under Settings → This device) and `settingsPin`.

Process settings stay in env: `SODE_LISTEN` (`:8080`), `SODE_CONFIG` (`/data/config.yaml` in the image), `SODE_HISTORY` (`/data/history.jsonl`), `SODE_SENTLOG` (`/data/sent.jsonl`), `SODE_SENTLOG_MB` (`64`) and `SODE_DEBUG=true` to hex-dump all SW-P-08 traffic.

### Settings PIN

Settings → Access can set a PIN (4-64 characters) asked every time Settings opens; patching stays open. Wrong tries count for everyone: from the 5th in a row there's a wait (30 seconds, doubling, up to an hour), and after `pinMaxTries` settings stay locked until the PIN is reset or Sode restarts. Only a salted PBKDF2 hash is stored.

Forgot it?

```sh
docker compose exec sode /sode -reset-pin && docker compose restart sode
```

or delete the `settingsPin:` line from `/data/config.yaml` and restart.

## Monitoring (Uptime Kuma)

`GET /api/health` reports the server and the router apart. It needs no PIN and shows no settings:

```json
{"server":"up","router":"up","addr":"10.0.0.1:8910","version":"Beta 1.3","uptimeSeconds":86400,"browsers":2}
```

`router` is `up`, `connecting` or `down`; `err` appears when the router link has a problem. The status code is 200 only while the router is up and 503 otherwise, so a plain HTTP(s) monitor alerts when the router is lost (a dead server shows as no answer). For a JSON Query monitor, use `router` as the key and `up` as the expected value. `/healthz` is unchanged.

## Communication log

`/log` (**Open Log** in Settings, available even while Settings are locked) lists every message Sode wrote to the router, newest first and live, with what it was for and whether the router ACKed it. It's read-only. Kept in `SODE_SENTLOG` up to `SODE_SENTLOG_MB`; without it the last 20,000 messages stay in memory.

## Protocol notes

Crosspoints use extended commands (0x82 connect, 0x84 connected, 0x95/0x97 tally dump). A take succeeds once the router sends Connected for every requested level (3s timeout); routers send nothing for a connect that changes nothing, so Sode interrogates and counts those as "already". Protects are read with the extended protect dump (0x93 → 0x94) and takes to protected destinations are refused before anything is sent. Messages queued while disconnected are discarded. Names are polled, since the router doesn't announce renames.

## Development

```sh
go test ./...                                        # also run by the Dockerfile
go run ./cmd/swp08sim                                # simulator on :8910
SODE_ROUTER=127.0.0.1:8910 go run ./cmd/sode         # server on :8080
node --test 'web/test/*.test.mjs'                    # browser-side tests
```

The simulator takes `SIM_*` variables to fake misbehaving routers (`SIM_IGNORE_DSTS`, `SIM_SILENT_DSTS`, `SIM_SLOW_DSTS`, `SIM_FOLLOWS`, `SIM_PANEL_SECONDS`, `SIM_PANEL_SILENT`); see `cmd/swp08sim`. The web UI is embedded, so restart after frontend edits.

## Releasing

Bump `Version` in `internal/app/version.go` and the `**Version:**` line above, then run `go test ./internal/app -run TestVersionNoted` (with `SODE_RELEASE_TAG=v<version>` it also checks the tag).

Pushing a `v<version>` tag makes CI check it and publish the multi-arch image to ghcr as `latest` and the tag name; then make a GitHub release from the tag.

## License

Public domain ([Unlicense](LICENSE)). The vendored Lucide icons keep their own license in `web/vendor/patchbay/icons/`.
