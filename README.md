# Sode

Hyper-light, keyboard-first web patching for SW-P-08 routers (target: Ross Ultrix).

Type a **source**, Tab, a **destination**, Tab, **levels** (`1`, `1-3`, `4-17`, `1,3,5-8`; blank = default), Enter, then press the churning confirm key.

To patch one source to several destinations, press `+` (or the + button) instead of Tab after each destination: it becomes a chip and the box starts over. Each destination can only be added once; Backspace in the empty box takes the last one back to edit. The takes land in history as one folder with an overall status and **Undo all**; open it to undo or retry destinations one by one. Protected destinations in the set are refused and show as failed; the rest go ahead.

- Single static Go binary, ~15MB distroless image
- Themed with [patchbay](https://github.com/sethdotfm/patchbay), vendored and embedded
- Serves plain HTTP on :8080; put it behind your own TLS proxy (e.g. Caddy)
- Phones get a text-message style layout: type a source, Next, a destination (+ for another), Next, levels, Go, then slide to confirm. Swipe a recent take or folder left to undo. Settings → This device can force the phone or desktop layout.

## Run

Against the built-in simulator:

```sh
docker compose --profile sim up --build
# open http://localhost:8080
```

The simulator can fake a router that never confirms some destinations, to exercise timeouts and Retry: set `SIM_IGNORE_DSTS=9,12` (0-based destination numbers) on the `swp08sim` container.

Against a real router (first start):

```sh
SODE_ROUTER=10.0.0.50:8910 docker compose up -d --build
```

## Settings

Settings live in `/data/config.yaml` on the `sode-data` volume and are edited from the **Settings** button at the bottom of the page. Saving applies immediately; changes to the router, matrix, levels or name options reconnect and reload names and routes.

On first start, when there is no config file yet, it is created from these seed variables. After that the file wins and the variables are ignored.

| Seed var | YAML key | Default | Meaning |
|---|---|---|---|
| `SODE_ROUTER` | `router` | | Router `host:port`. Empty = idle until set in Settings |
| `SODE_MATRIX` | `matrix` | `0` | SW-P-08 matrix (0-15 with standard name requests) |
| `SODE_LEVEL_COUNT` | `levels` | `17` | Number of levels; 1 gets the video swatch, the rest audio |
| `SODE_LEVEL_WIRE_OFFSET` | `levelWireOffset` | `-1` | Wire level = level + offset. `-1`: level 1 → wire 0 |
| `SODE_DEFAULT_LEVELS` | `defaultLevels` | `1-<levels>` | Used when the levels box is blank |
| `SODE_NAME_LEN` | `nameLength` | `12` | Name length to request: 4, 8, 12 or 16 |
| `SODE_NAMES` | `nameRequests` | `std` | `std` (cmd 100/102) or `ext` (cmd 228/230) |
| `SODE_NUM_OFFSET` | `numberOffset` | `1` | Added to router numbers for display (Ultrix shows 1-based) |
| `SODE_CONFIRM_LOCKOUT_MS` | `confirmLockoutMs` | `500` | Confirm key ignored for this long after the dialog opens |
| `SODE_CONFIRM_ENTER` | `confirmEnter` | `false` | Also allow Enter to confirm |
| `SODE_NAME_REFRESH_S` | `nameRefreshSeconds` | `60` | Re-read names this often (0 = off); also a button in Settings |
| `SODE_TALLY_RESYNC_S` | `tallyResyncSeconds` | `300` | Re-read all routes and protects this often (0 = off) |
| | `levelColors` | `[]` | Per-level `#rrggbb`, level 1 first; blank keeps the default (the Ultrix colours for 1-17, built-in swatches after). Settings only, no seed var |
| `SODE_TRUSTED_PROXIES` | `trustedProxies` | | IPs/CIDRs whose `X-Forwarded-For` is believed (your Caddy). Others are logged by peer address |

Process settings stay in env: `SODE_LISTEN` (`:8080`), `SODE_CONFIG` (`/data/config.yaml` in the image), `SODE_HISTORY` (`/data/history.jsonl`), and `SODE_DEBUG=true` to hex-dump all SW-P-08 traffic.

## Protocol notes

Crosspoints always use extended commands (0x82 connect, 0x84 connected, 0x95/0x97 tally dump), so source and destination numbers can go past 1024. A take only reports success once the router sends Connected for every requested level (3s timeout). Messages queued while the router is disconnected are discarded rather than sent after reconnect.

Protects are read with the extended protect tally dump (0x93 → 0x94). The Ultrix packs each destination as two bytes: protect state in bits 4-5 of the first, device number in the rest. Takes to a protected destination/level are refused before anything is sent.

Names refresh into a fresh list that is swapped in once replies stop, so names deleted on the router disappear. The router does not announce name changes, so this polling is the only way to see them.

See [PLAN.md](PLAN.md) for the roadmap.
