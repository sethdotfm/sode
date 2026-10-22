# Sode

Hyper-light, keyboard-first web patching for SW-P-08 routers (target: Ross Ultrix).

Type a **source**, Tab, a **destination**, Tab, **levels** (`1`, `1-3`, `4-17`, `1,3,5-8`; blank = default), Enter, then press the churning confirm key.

- Single static Go binary, ~15MB distroless image
- Themed with [patchbay](https://github.com/sethdotfm/patchbay), vendored and embedded
- Serves plain HTTP on :8080; put it behind your own TLS proxy (e.g. Caddy)

## Run

Against the built-in simulator:

```sh
docker compose --profile sim up --build
# open http://localhost:8080
```

Against a real router:

```sh
SODE_ROUTER=10.0.0.50:8910 SODE_DEBUG=true docker compose up --build
```

`SODE_DEBUG=true` logs every raw SW-P-08 frame in hex, which is the quickest way to see what the router is actually sending. Takes are logged as `TAKE client=… src=… dst=… levels=… result=…`.

## Settings (env)

| Var | Default | Meaning |
|---|---|---|
| `SODE_ROUTER` | (required) | Router `host:port` |
| `SODE_MATRIX` | `0` | SW-P-08 matrix number |
| `SODE_LEVEL_COUNT` | `17` | Number of levels (1 = video swatch, 2.. = audio swatches) |
| `SODE_LEVEL_WIRE_OFFSET` | `-1` | Wire level = UI level + offset. `-1`: UI 1 → wire 0. Use `0` if the router wants video on wire level 1 |
| `SODE_DEFAULT_LEVELS` | `1-<count>` | Used when the LEVELS box is blank |
| `SODE_NAME_LEN` | `12` | Name length to request: 4, 8, 12 or 16 |
| `SODE_NAMES` | `std` | `std` (cmd 100/102) or `ext` (cmd 0xE4/0xE6) name requests |
| `SODE_NUM_OFFSET` | `1` | Added to wire src/dst numbers for display (Ultrix shows 1-based) |
| `SODE_CONFIRM_LOCKOUT_MS` | `500` | Confirm key ignored for this long after the dialog opens |
| `SODE_CONFIRM_ENTER` | `false` | Also allow Enter to confirm |
| `SODE_DEBUG` | `false` | Hex-dump SW-P-08 traffic |
| `SODE_LISTEN` | `:8080` | HTTP listen address |

## Protocol notes

Crosspoints always use extended commands (0x82 connect, 0x84 connected, 0x95/0x97 tally dump), so source and destination numbers can go past 1024. A take only reports success once the router sends Connected for every requested level (3s timeout). Messages queued while the router is disconnected are discarded rather than sent after reconnect.

See [PLAN.md](PLAN.md) for the roadmap.
