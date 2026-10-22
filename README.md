# Sode

Hyper-light, keyboard-first web patching for SW-P-08 routers (target: Ross Ultrix).

Type a **source**, Tab, a **destination**, Tab, **levels** (`1`, `1-3`, `4-17`, blank = default), Enter, then press the churning confirm key.

- Single static Go binary in a distroless Docker image
- Themed with [patchbay](https://github.com/sethdotfm/patchbay), vendored and embedded (no runtime fetches)
- Serves plain HTTP; put it behind your own TLS proxy (e.g. Caddy)

Status: planning. See [PLAN.md](PLAN.md).
