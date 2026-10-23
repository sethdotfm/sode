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

# Local dev without Docker: simulator + server
go run ./cmd/swp08sim                          # :8910; SIM_SOURCES/SIM_DESTS/SIM_LEVELS, SIM_IGNORE_DSTS=9,12 (0-based, never confirm)
SODE_ROUTER=127.0.0.1:8910 SODE_DEBUG=true go run ./cmd/sode   # :8080; SODE_DEBUG hex-dumps SW-P-08
# Without SODE_CONFIG, config comes from SODE_* env and is not persisted.

docker compose --profile sim up --build        # server + simulator
SODE_ROUTER=10.0.0.50:8910 docker compose up -d --build   # real router
```

The web UI is embedded with `go:embed` (`web/web.go`), so frontend edits need the server restarted / rebuilt to show up.

## Key decisions

- Go, stdlib-first. Only deps: github.com/coder/websocket, gopkg.in/yaml.v3. Distroless static image, listens HTTP :8080 (user's Caddy does TLS).
- SW-P-08 over TCP to one router, **single matrix**. Always use **extended** commands (16-bit src/dst). Protects are read and honoured. Auto-reconnect + keepalive. Names come from the router and are re-polled (default 60s).
- Levels: wire = level + configurable offset. Levels are shown **only as numbered colour swatches / numbers**; there are no level labels (removed at the user's request).
- Config: `/data/config.yaml`, edited from the Settings dialog (no auth in v1). SODE_* env vars only seed the file on first start.
- Frontend: vanilla JS ES modules, no build step, embedded via go:embed. Follow patchbay rules exactly (https://github.com/sethdotfm/patchbay llms.txt): pb- classes only, tokens only, no Bootstrap utilities, no border-radius, status always with text. Patchbay is vendored under `web/vendor/patchbay`.
- Confirm dialog: random churning letter + lockout (default 500ms; Enter-to-confirm optional).
- History (JSONL) + shared server-side recent/undo list.
- Keep it light: no frameworks, no extra deps without asking.

## Architecture

- `internal/swp08`: framing (DLE/STX, checksums, ACK/NAK — `frame.go`), message builders/parsers (`msg.go`), and `Client` (`client.go`): one TCP session with reconnect, keepalive probe, ACK-gated writer. Messages sent while disconnected are dropped, not queued. Callbacks `OnUp`/`OnMsg`/`OnState` feed the app.
- `internal/app`: `App` owns all router state (names, tally, protect, keyed dst → **UI level**) under one mutex. Config lives in an immutable `runtime` behind an `atomic.Pointer`; `ApplyConfig` saves, swaps it and, if router-side fields changed, clears state and calls `SetTarget` to reconnect. `Run` ticks every 100ms: `flush()` batches tally/protect changes into broadcasts and finishes name refreshes (fresh maps swapped in once replies go quiet), `periodic()` re-polls names/dumps. Takes use `waiter`s that collect Connected/Tally per level (see the verified-behaviour notes below). `history.go` is the JSONL log with paging; a page never ends inside a group, and the browser shows a group as one folder (`histRows` in `core.js`).
- `internal/app/http.go`: `/` static, `/healthz`, `/ws`. One WebSocket per browser; JSON messages with a `t` field (client → server: `batch` (every take of one confirm; the server runs at most 8 at once and gives multi-take batches a shared history `group`), `history`, `refreshNames`, `clearHistory`, `config`; server → client: `hello` snapshot, `status`, `names`, `namesChecked`, `tally`, `protect`, history/results). Client IP honours `X-Forwarded-For` only from `trustedProxies`.
- `internal/config`: `Defaults` → `FromEnv` (seed) → `Normalize`/`Validate`; YAML load/save; `LevelMap` does the UI↔wire level mapping.
- `cmd/swp08sim`: fake router for dev; must keep mimicking real Ultrix quirks (e.g. no Connected reply for no-op connects).
- `web/`: `main.js` wires modules then calls `connect()`. `core.js` holds state `S`, the WebSocket and shared helpers, and emits events (`on('tally', …)` etc.); views render from `S`. `desktop.js` and `mobile.js` are the two layouts (`layout.js` picks one: auto on narrow touch screens, per-browser override in localStorage; `index.html` applies the same rule before first paint). `settings.js` is shared. Build DOM with `el()` — never `innerHTML` — because names come from the router.

## Verified against the real Ultrix (2026-10-22, 172.17.60.101:8910)
- Defaults are correct: UI level 1 → wire level 0 (`SODE_LEVEL_WIRE_OFFSET=-1`), display number = wire + 1 (`SODE_NUM_OFFSET=1`).
- Standard name requests (cmd 100/102) with 12-char names work. ~2690 sources, 720 destinations.
- Extended tally dump (0x95 → 0x97, 64 per message) works on all 17 levels; source 0xFFFF = nothing routed.
- Extended connect (0x82) is confirmed by 0x84 Connected; takes report ok.
- Routers only send Connected when a crosspoint *changes*. A connect that matches the current state gets no reply, so `take()` interrogates (0x81 → 0x83 Tally) levels already on the source, and any level still unconfirmed at the 3s timeout gets a 1s interrogate grace. Levels confirmed that way are reported as "already" (history `already`). The simulator mimics this.
- Extended protect dump (0x93 matrix, level, startDst(2) → 0x94 matrix, level, count, firstDst(2), count × 2 bytes): state in bits 4-5 of byte 1, device in the rest. Dest 65 was protected by device 9 on all levels. Single protect change messages (0x8B/0x8D/0x8F) not yet seen live.
