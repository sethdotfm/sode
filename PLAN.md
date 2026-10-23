# Sode — ergonomic SW-P-08 patching web app

## Context
Operators need a fast, keyboard-first way to patch a SW-P-08 router (target: Ross Ultrix) without a full control panel. Sode is a tiny Dockerized Go service: one Google-classic-minimal page, three big boxes (SOURCE → DESTINATION → LEVELS), fuzzy find, Tab to advance, Enter to confirm via a churning-key dialog. Themed with patchbay CSS (github.com/sethdotfm/patchbay, rules in its llms.txt). Priorities: hyper-light, efficient, reliable.

## Decisions (from Q&A)
- **Go**, single static binary, distroless image. Deps: `github.com/coder/websocket`, `gopkg.in/yaml.v3`. Nothing else.
- **HTTP only** (container listens :8080); user's Caddy terminates TLS on 192.0.2.10:443.
- **TCP only** to one SW-P-08 router. Always **extended commands** (16-bit src/dst, >1024). **Honor protects.** **Auto-reconnect** (exp. backoff, cap 10s) + keepalive/periodic tally re-sync to catch half-open TCP.
- **Names from router** (SW-P-08 name requests), cached in memory, refreshed on connect.
- **Level map user-definable**: UI level → (matrix, wire level, label). Default 17 levels, UI 1 = Video, 2–17 = Audio 1–16 (patchbay `pb-level--video|01…16`). Wire offset configurable (Ultrix 1-indexed vs 0-based) — verify against real Ultrix.
- **Global default levels** (blank LEVELS box ⇒ default, normally `1-17`). No per-user config.
- **Level syntax**: `1`, `1-3`, `4-17`, `1,3,5-8`, `!1` (all but 1), `1-8 !3` + config aliases (`v`, `st`, `a`, `all`…). Invalid ⇒ `pb-input--invalid`, Enter blocked.
- **Fuzzy**: number + name. Exact number ranks first, then prefix, then fuzzy subsequence on name. Small dropdown, arrows to pick, Tab accepts top/highlighted.
- **Confirm**: random letter (exclude l/i/o and previous letter), 500ms input lockout. Both configurable; option to allow Enter.
- **Tally, not a mess**: dest typed + source empty ⇒ box below with arrow diagram of sources currently feeding it, grouped by level runs (`Video ← 141 CG3 Fill · 2–17 ← 12 CAM 2`). Source typed + dest empty ⇒ `Feeding 42 "Zoom Return 1", 43 "Zoom Return 2", and 67 others`. Confirm dialog shows before → after.
- **Audit log + Recent/undo**: server-side, shared by all clients. Bottom of page shows a list styled like the inputs. Selecting one pre-fills the boxes with the prior source(s) and goes through the same confirm.
- **Config**: mounted `/data/config.yaml` + open `/settings` web page (auth deferred to a later version). Atomic write, hot reload, reconnect if router addr changes.
- **Patchbay vendored + `go:embed`**: css, needed Lucide symbols only, IBM Plex fonts self-hosted. Zero runtime external fetches (air-gap safe).
- **Repo** at `~/Desktop/sode`; remote added later.

## Layout
```
~/Desktop/sode/
  cmd/sode/main.go            wiring, flags/env, graceful shutdown
  cmd/swp08sim/main.go        fake router for dev/tests (names, tally, protects, connect)
  internal/swp08/
    frame.go                  DLE STX…DLE ETX, DLE stuffing, BTC, 2's-complement checksum, ACK/NAK
    msg.go                    encode/decode: ext interrogate/connect/tally/connected, tally dump,
                              src/dest name req+resp, protect tally/info
    client.go                 conn loop, ACK/NAK + retry, reconnect backoff, keepalive, event chan
  internal/router/state.go    names, tally[dst][level]src, protects; snapshot + delta subscribers
  internal/config/config.go   load/validate/save(atomic), level map, aliases, ParseLevels()
  internal/audit/audit.go     append-only JSONL (/data/audit.jsonl), ring of last N for "recent"
  internal/web/
    server.go                 static (embed), /ws, /settings GET/POST, /healthz
    hub.go                    ws broadcast, per-client send queue, slow-client drop
    protocol.go               JSON messages (below)
  web/
    index.html  app.js  app.css   (vanilla JS, no build step, target <20KB JS)
    settings.html  settings.js
    vendor/patchbay/{css/*, icons/lucide.svg (subset), fonts/*}
  Dockerfile                  golang:alpine build → gcr.io/distroless/static:nonroot
  compose.yaml                sode + optional swp08sim profile; volume ./data:/data
  .gitignore  README.md  config.example.yaml
```

## WebSocket protocol (JSON)
- S→C `hello`: levels map, aliases, default levels, confirm settings, names, tally, protects, recent, status
- S→C deltas: `status` {router: up|connecting|down, since}, `tally`, `names`, `protect`, `recent`, `config`
- C→S `take` {id, src, dst, levels[]} → S→C `takeResult` {id, ok, perLevel acks, error}. Success means a Connected (ext cmd) was received for every wire level before a timeout (~2s). On failure, no audit "ok".
- Server rejects takes to protected dests and to levels outside the map.

## Page (patchbay)
- Centered, Google-classic whitespace. Small `pb-status` in the top-right corner (dot + text, never color alone; hover for each link). A lost link (browser↔Sode or Sode↔router) shows a solid red banner pinned to the top ("Connection to Sode server interrupted" / "Connected to Sode server, router offline") with a browser → server → router map marking the lost link, a thick pulsing red outline round the window, and disables Patch/Undo/Retry.
- Three large `pb-input` boxes across. Dropdown is a `pb-menu`. Tally box underneath uses `pb-xpt--source/dest` styling. Recent list at bottom as input-styled rows.
- Keys: Tab/Shift+Tab move between boxes, ↑↓ pick, Enter → confirm modal (`pb-modal` + `pb-modal__header--warn`), churn key confirms, Esc cancels; Esc in an empty box clears all.
- Modal text: *Are you sure you want to patch Source 141 CG3 Fill to Destination 24 DSM5? Press **j** to confirm, or escape to cancel.* Plus a levels line and a before→after line.
- Theme follows OS via patchbay tokens. No per-theme styling.

## Milestones
1. **Init**: `mkdir ~/Desktop/sode && git init`, go mod, .gitignore, skeleton, vendor patchbay (css + icon subset + fonts), initial commit.
2. **SW-P-08 codec + tests**: golden frames, DLE escaping, checksum, ACK/NAK. Check the command numbers against the SW-P-08 spec and Ultrix docs before finalizing.
3. **Client + state + simulator**: reconnect/keepalive, name load, tally dump, protects.
4. **Web server + WS hub + audit**.
5. **Frontend**: boxes, fuzzy, level parser, confirm, tally panel, recent/undo, status diagram.
6. **Settings page + hot reload**.
7. **Docker**: image size check (target <15MB), compose with sim profile, README.

## Verification
- `go test ./...`: codec round-trips/golden bytes, level parser + aliases, fuzzy ranking, config atomic save, client against the in-process simulator (connect, drop/reconnect, NAK retry, protect refusal).
- `docker compose --profile sim up`: open the page in Chrome (claude-in-chrome) and run through the flow: fuzzy → Tab → levels → Enter → churn key → tally updates → recent → undo. Kill the sim to watch the status diagram go red and recover.
- Then point at the real Ultrix (user) to confirm the level offset and name commands.

## Open items to confirm during build
- Exact Ultrix SW-P-08 port, matrix number, and whether it serves names via extended name cmds.
- Max name length (4/8/12/16 chars) to request.
