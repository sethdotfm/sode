# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Sode — notes for Claude

Keyboard-first web patching for one SW-P-08 router (Ross Ultrix). Full plan and rationale: PLAN.md; user-facing settings table and protocol summary: README.md.

## Commands

Go is not installed on the dev Mac; builds and tests normally go through Docker (the Dockerfile runs `go test ./...` before building). With Go available:

```sh
go test ./...                                  # all tests (also run by the Dockerfile build)
go test ./internal/app -run TestWaiterAlreadyRouted -v   # one test
go vet ./...
docker run --rm -v "$PWD/web":/w node:22-alpine node --test '/w/test/*.test.mjs'   # browser-side tests (filter language, histRows)

# Local dev without Docker: simulator + server
go run ./cmd/swp08sim                          # :8910; SIM_SOURCES/SIM_DESTS/SIM_LEVELS, SIM_IGNORE_DSTS=9,12 (0-based, never confirm), SIM_PANEL_SECONDS=5 (routes by itself like a panel), SIM_FOLLOWS=2>40 (dst 40 follows dst 2)
SODE_ROUTER=127.0.0.1:8910 SODE_DEBUG=true go run ./cmd/sode   # :8080; SODE_DEBUG hex-dumps SW-P-08
# Without SODE_CONFIG, config comes from SODE_* env and is not persisted.

docker compose --profile sim up --build        # server + simulator
SODE_ROUTER=10.0.0.50:8910 docker compose up -d --build   # real router
```

The web UI is embedded with `go:embed` (`web/web.go`), so frontend edits need the server restarted / rebuilt to show up.

## Key decisions

- Go, stdlib-first. Only deps: github.com/coder/websocket, gopkg.in/yaml.v3. Distroless static image, listens HTTP :8080 (user's Caddy does TLS).
- SW-P-08 over TCP to one router, **single matrix**. Always use **extended** commands (16-bit src/dst). Protects are read and honored. Auto-reconnect + keepalive. Names come from the router and are re-polled (default 60s).
- Levels: wire = level + configurable offset. Internally levels are UI 1..N everywhere (history, `defaultLevels` in YAML); `levelNumbering` (`1` / `0` / `va` = V, 1…16, i.e. `0` with 0 shown as V; no "A", so chips stay two characters) only changes how they're shown (`Level.Label` from `LevelMap`) and typed (`parseLevels` in `core.js`). `levelTypedBase` is a dropped setting kept only so old config files still load (`KnownFields`). Levels are color swatches with those labels; no free-text level names (removed at the user's request).
- Config: `/data/config.yaml`, edited from the Settings dialog. SODE_* env vars only seed the file on first start. Optional settings PIN: PBKDF2 hash in `settingsPin` (`json:"-"`, never sent to browsers; `SaveSettings` carries it over on saves), unlocked per WebSocket connection *for that hash* (changing or removing the PIN relocks everyone; an unlock with no PIN set counts for nothing) and relocked when Settings closes. `unlock` is handled in line in the read loop so a following `lock` wins; wrong PINs are counted globally (not per IP), one check at a time, waits doubling from 30s after the 5th, and `pinMaxTries` (default 10, 0 = never) in a row locks settings until `-reset-pin` or restart. A non-hash `settingsPin` stops startup. `SaveSettings` runs one at a time (`cfgMu`) and re-checks the lock. `sode -reset-pin` clears it (restart needed).
- Frontend: vanilla JS ES modules, no build step, embedded via go:embed. Follow patchbay rules exactly (https://github.com/sethdotfm/patchbay llms.txt): pb- classes only, tokens only, no Bootstrap utilities, no border-radius, status always with text. Patchbay is vendored under `web/vendor/patchbay`.
- Confirm dialog: random churning letter + lockout (default 500ms; Enter-to-confirm optional).
- History (JSONL) + shared server-side recent/undo list. Routes made elsewhere (beta, issue 14) are logged as `ext` entries (`ext.go`). These are Connected messages that none of our takes sent: `own` holds each take's (dst, wire level, src) for `ownFor`. Changes are collected into bursts, ending after 500ms quiet or 3s total, and logged as one entry per dst+src. A burst across several dsts shares a `group`. The browser handles display in `histRows`:
  - Follows (#53): an `ext` change of the same src to another dst within 500ms of a take is a follow. The take can be ours (`waiter.cause`, set from its confirms) or an `ext` take, earlier or in the same burst (`a.causes`, `extEntries`). The follow is written with `follow` = that take's ID and isn't grouped. A burst that might follow one of our takes still in flight waits for it to be recorded (`extAwaitsTake`). `histRows` attaches follows to their take as `e.follows`. Undo plans carry them as `plan.extra` (one per dst, the oldest), sent only if the confirm's box is ticked.
  - It merges repeated same-dst/same-levels `ext` entries within 30s into one row (newest, with the oldest's `prev`, `merged` = count, `chain` = the entries). Any other entry on that dst breaks the chain, even a hidden one.
  - It then applies this browser's Recent filter (`core.js`: `localStorage['sode-filter']`, `setFilter`). The filter covers kinds (Sode, panel, globally hidden) plus `src`/`dst` expressions.
  - Folders keep every entry in `f.entries`, so Undo all covers the whole confirm, and the visible ones in `f.shown`.
  - The expression language is `web/filter.js`: pure, with no imports. The server's `hiddenDsts` uses the same language and is evaluated only in browsers.
  - Named filter sets come from three places: the built-in All, `cfg.filterSets` (Settings, global) and `localStorage['sode-filter-sets']`. The active one is `sode-filter-active` `{scope, name, base}`. `followGlobal()` on hello applies a changed global set, unless the browser edited the filter since picking it (filter ≠ base). Set names are unique per scope, ignoring case.
  - Quick search (`setSearch`, the `[data-search]` boxes): not kept, and doesn't merge rows. It reads back through the log in 500-entry pages (`history` takes `n`, capped at 500 by the server). It ignores the filter unless `localStorage['sode-search-filters']` is `on`.
  - `web/filters.js` holds the dialog (Visual/Code editors, live preview), share links (`?f=` base64url JSON, applied and stripped in `main.js`) and the empty-state note.
- Keep it light: no frameworks, no extra deps without asking.

## Architecture

- `internal/swp08`: framing (DLE/STX, checksums, ACK/NAK — `frame.go`), message builders/parsers (`msg.go`), and `Client` (`client.go`): one TCP session with reconnect, keepalive probe (an interrogate, sent on connect and after 10s without rx). Up is reported, and `Send` accepts messages, only after the router's first reply (a hung router still accepts TCP). A settings change (`SetTarget`) reconnects without reporting down. A probe with no reply at all drops the link (silent router down in ~13s); any other unanswered message just triggers a probe, so a command the router ignores doesn't drop a healthy link. Rx times are monotonic offsets (`lastRx`), ACK-gated writer. Messages sent while disconnected are dropped, not queued. Callbacks `OnUp`/`OnMsg`/`OnState` feed the app.
- `internal/app`: `App` owns all router state (names, tally, protect, keyed dst → **UI level**) under one mutex. Config lives in an immutable `runtime` behind an `atomic.Pointer`; `ApplyConfig` saves, swaps it and, if router-side fields changed, clears state and calls `SetTarget` to reconnect. `Run` ticks every 100ms: `flush()` batches tally/protect changes into broadcasts and finishes name refreshes (fresh maps swapped in once replies go quiet), `periodic()` re-polls names/dumps. Takes use `waiter`s that collect Connected/Tally per level (see the verified-behavior notes below). `history.go` is the JSONL log with paging; a page never ends inside a group, and the browser shows a group as one folder (`histRows` in `core.js`).
- `internal/app/http.go`: `/` static, `/healthz`, `/ws`. One WebSocket per browser; JSON messages with a `t` field (client → server: `batch` (every take of one confirm; the server runs at most 8 at once and gives multi-take batches a shared history `group`), `history`, `refreshNames`, `clearHistory`, `config` (+ optional `pin` / `clearPin`), `filterSets` (+ `sets`, `id`; just the sets for everyone, from Recent's Filters, PIN-guarded like `config`, answered with `filterSetsResult`), `unlock` (+ `id`, echoed in `unlockResult` so the browser drops stale replies), `lock` — `config` and `clearHistory` are refused while a PIN is set and the connection isn't unlocked for it; server → client: `hello` snapshot, `status`, `names`, `namesChecked`, `tally`, `protect`, `unlockResult`, `locked` (also sent when a PIN change voids an unlock), history/results; `hello` carries `pinSet`). Client IP honors `X-Forwarded-For` only from `trustedProxies`.
- `internal/config`: `Defaults` → `FromEnv` (seed) → `Normalize`/`Validate`; YAML load/save; `LevelMap` does the UI↔wire level mapping.
- `cmd/swp08sim`: fake router for dev; must keep mimicking real Ultrix quirks (e.g. no Connected reply for no-op connects).
- `web/`: `main.js` wires modules then calls `connect()`. `core.js` holds state `S`, the WebSocket and shared helpers, and emits events (`on('tally', …)` etc.); views render from `S`. `desktop.js` and `mobile.js` are the two layouts (`layout.js` picks one: auto on narrow touch screens, per-browser override in localStorage; `index.html` applies the same rule before first paint). `settings.js` is shared, and so are `filters.js` (Recent's Filters dialog, filter sets, the Filter by Source/Destination dropdowns) and `filter.js` (the filter language; no imports, tested in Node under `web/test/`). Build DOM with `el()` — never `innerHTML` — because names come from the router. Desktop keyboard mode: Esc with every box empty (or Esc on a button in Recent, landing on its row) focuses the history list (`#history`) and sets `keysOn`, cleared by any click, so clicking in Recent never arms the single keys (`onListKey` in `desktop.js`: s/d/f boxes, j/k rows, J next group, K latest take, h/l or Shift+Enter/Enter close/open, u/r, `,` settings, `/` Filters, Shift+S/Shift+D the Filter by boxes, `?` the shortcut list in `index.html`, also Ctrl+? / ⌘? from anywhere). Ctrl+S/D/F (Control on a Mac too) fill a patch box from the last take from anywhere (`useLast`; this browser's last take, else the newest Sode take in history), and each box shows it as a "Last:" chip by its label. A small `.sode__key-note` shows each box's keyboard-mode key while it's empty; folders open as the cursor enters and close as it leaves (`autoOpen`).

## Verified against the real Ultrix (2026-10-22, 172.17.60.101:8910)
- Defaults are correct: UI level 1 → wire level 0 (`SODE_LEVEL_WIRE_OFFSET=-1`), display number = wire + 1 (`SODE_NUM_OFFSET=1`).
- Standard name requests (cmd 100/102) with 12-char names work. ~2690 sources, 720 destinations.
- Extended tally dump (0x95 → 0x97, 64 per message) works on all 17 levels; source 0xFFFF = nothing routed.
- Extended connect (0x82) is confirmed by 0x84 Connected; takes report ok.
- Routers only send Connected when a crosspoint *changes*. A connect that matches the current state gets no reply, so `take()` interrogates (0x81 → 0x83 Tally) levels already on the source, and any level still unconfirmed at the 3s timeout gets a 1s interrogate grace. Levels confirmed that way are reported as "already" (history `already`). The simulator mimics this.
- 2026-10-23 18:01:40: the router sent 0x84 to Sode for a route Sode didn't make. PIX 03 went to DSM 03 and MV08 PIP015, and Sode logged it as an `ext` burst. Where it came from (panel, Companion or another client) wasn't identified.
- Extended protect dump (0x93 matrix, level, startDst(2) → 0x94 matrix, level, count, firstDst(2), count × 2 bytes): state in bits 4-5 of byte 1, device in the rest. Dest 65 was protected by device 9 on all levels. Single protect change messages (0x8B/0x8D/0x8F) not yet seen live.
