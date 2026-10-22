# Sode — notes for Claude

Full plan and rationale: PLAN.md. Key decisions:

- Go, stdlib-first. Only deps: github.com/coder/websocket, gopkg.in/yaml.v3. Distroless static image, listens HTTP :8080 (user's Caddy does TLS).
- SW-P-08 over TCP to one router, **single matrix**. Always use **extended** commands (16-bit src/dst). Protects are read and honoured. Auto-reconnect + keepalive. Names come from the router and are re-polled (default 60s).
- Levels: wire = level + configurable offset. Levels are shown **only as numbered colour swatches / numbers**; there are no level labels (removed at the user's request).
- Config: `/data/config.yaml`, edited from the Settings dialog (no auth in v1). SODE_* env vars only seed the file on first start.
- Frontend: vanilla JS ES modules, no build step, embedded via go:embed. `core.js` holds state, the WebSocket and shared helpers and emits events; `desktop.js` and `mobile.js` are the two layouts (`layout.js` picks one: auto on narrow touch screens, per-browser override); `settings.js` is shared. Follow patchbay rules exactly (https://github.com/sethdotfm/patchbay llms.txt): pb- classes only, tokens only, no Bootstrap utilities, no border-radius, status always with text.
- Confirm dialog: random churning letter + lockout (default 500ms; Enter-to-confirm optional).
- History (JSONL) + shared server-side recent/undo list.
- Keep it light: no frameworks, no extra deps without asking.

## Verified against the real Ultrix (2026-10-22, 172.17.60.101:8910)
- Defaults are correct: UI level 1 → wire level 0 (`SODE_LEVEL_WIRE_OFFSET=-1`), display number = wire + 1 (`SODE_NUM_OFFSET=1`).
- Standard name requests (cmd 100/102) with 12-char names work. ~2690 sources, 720 destinations.
- Extended tally dump (0x95 → 0x97, 64 per message) works on all 17 levels; source 0xFFFF = nothing routed.
- Extended connect (0x82) is confirmed by 0x84 Connected; takes report ok.
- Routers only send Connected when a crosspoint *changes*. A connect that matches the current state gets no reply, so `take()` interrogates (0x81 → 0x83 Tally) levels already on the source, and any level still unconfirmed at the 3s timeout gets a 1s interrogate grace. Levels confirmed that way are reported as "already" (history `already`). The simulator mimics this.
- Extended protect dump (0x93 matrix, level, startDst(2) → 0x94 matrix, level, count, firstDst(2), count × 2 bytes): state in bits 4-5 of byte 1, device in the rest. Dest 65 was protected by device 9 on all levels. Single protect change messages (0x8B/0x8D/0x8F) not yet seen live.
