# Sode — notes for Claude

Full plan and rationale: PLAN.md. Key decisions:

- Go, stdlib-first. Only deps: github.com/coder/websocket, gopkg.in/yaml.v3. Distroless static image, listens HTTP :8080 (user's Caddy does TLS).
- SW-P-08 over TCP to one router. Always use **extended** commands (16-bit src/dst). Honor protects. Auto-reconnect + keepalive. Names come from the router.
- Level map is user-definable: UI level -> (matrix, wire level, label). Default 17 levels, UI 1 = Video, 2-17 = Audio 1-16. Ultrix is 1-indexed; wire offset must be configurable.
- Frontend: vanilla JS, no build step, embedded via go:embed. Follow patchbay rules exactly (https://github.com/sethdotfm/patchbay llms.txt): pb- classes only, tokens only, no Bootstrap utilities, no border-radius, status always with text.
- Confirm dialog: random churning letter + 500ms lockout (configurable; Enter-to-confirm allowed via config).
- Audit log (JSONL) + shared server-side recent/undo list. Config in /data/config.yaml, editable at /settings (no auth in v1).
- Keep it light: no frameworks, no extra deps without asking.

## Verified against the real Ultrix (2026-10-22, 172.17.60.101:8910)
- Defaults are correct: UI level 1 → wire level 0 (`SODE_LEVEL_WIRE_OFFSET=-1`), display number = wire + 1 (`SODE_NUM_OFFSET=1`).
- Standard name requests (cmd 100/102) with 12-char names work. ~2690 sources, 720 destinations.
- Extended tally dump (0x95 → 0x97, 64 per message) works on all 17 levels; source 0xFFFF = nothing routed.
- Extended connect (0x82) is confirmed by 0x84 Connected; takes report ok.
