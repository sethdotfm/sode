DISCLOSURE: This was written by an LLM.

# Sode

[![CI](https://github.com/sethdotfm/sode/actions/workflows/ci.yml/badge.svg)](https://github.com/sethdotfm/sode/actions/workflows/ci.yml) [![rc](https://github.com/sethdotfm/sode/actions/workflows/rc.yml/badge.svg)](https://github.com/sethdotfm/sode/actions/workflows/rc.yml) [![Release](https://img.shields.io/github/v/release/sethdotfm/sode?include_prereleases)](https://github.com/sethdotfm/sode/releases) [![Go version](https://img.shields.io/github/go-mod/go-version/sethdotfm/sode)](go.mod) [![License](https://img.shields.io/github/license/sethdotfm/sode)](LICENSE)

**Version:** Beta 1.8.1

Sode is a hyper-light, keyboard-first web page for patching an SW-P-08 router, built and tested on a Ross Ultrix. It is for people who want to make routes fast from any browser on the network, on a desktop keyboard or a phone. You type a source, Tab, a destination, Tab, levels, Enter, then press the churning confirm key.

## Features

- Sode is a single static Go binary in a small distroless image, serving plain HTTP on :8080.
- It always uses extended SW-P-08 commands with 16-bit source and destination numbers, honors protected destinations and reconnects on its own.
- Names come from the router and are re-read every 60 seconds by default.
- The desktop layout is keyboard-first, with a shortcut list on `?`, and phones get a text-message style layout.
- History lists every take for all browsers, including routes made from panels, and undo restores what the router had before.
- Presets fire a saved set of takes in one confirm, sequentially or as a native fire mode group send.
- Filters and named filter sets limit what History shows, and you can share them with everyone or as a link.
- Optional users, PIN locks and per-user profiles cover shared wall panels and several users.
- A communication log at `/log` shows every message sent to the router and a router uptime graph.
- An optional authenticated API (`/api/v1`, off by default) with keys, rights and a live stream lets other apps read state and fire takes. See [docs/api.md](docs/api.md).
- `/api/health` reports the server and the router separately for monitors such as Uptime Kuma.

## Quickstart

Save this as `compose.yaml`:

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

To run from source with the simulator:

```sh
go run ./cmd/swp08sim                                # simulator on :8910
SODE_ROUTER=127.0.0.1:8910 go run ./cmd/sode         # server on :8080
```

Sode serves plain HTTP, so put your own TLS proxy in front. Behind a proxy, set `trustedProxies` so history shows real client IPs.

## Configuration

You change settings in the page, with the **Settings** button at the bottom. Sode stores them in `/data/config.yaml` and applies a save at once. On first start, with no config file yet, Sode seeds the file from `SODE_*` variables such as `SODE_ROUTER`. After that the file wins.

A few process settings stay in the environment: `SODE_LISTEN` (`:8080`), `SODE_CONFIG` (`/data/config.yaml` in the image), `SODE_HISTORY` (`/data/history.jsonl`), `SODE_SENTLOG` (`/data/sent.jsonl`), `SODE_SENTLOG_MB` (`64`), `SODE_UPTIME` (`/data/uptime.jsonl`) and `SODE_DEBUG=true` to hex-dump all SW-P-08 traffic.

If you forget the Advanced Settings PIN, run `docker compose exec sode /sode -reset-pin && docker compose restart sode`.

## More documentation

- [docs/api.md](docs/api.md) is the guide to the API: keys, scopes, rights, endpoints, the stream and timeouts.
- [docs/reference.md](docs/reference.md) has the full settings table, the SW-P-08 protocol notes, proxy and app-install notes, and details on levels, presets, History, filters, users and locks, profiles, monitoring, the communication log, the simulator and image tags.
- [RELEASING.md](RELEASING.md) has the release steps, and [CHANGELOG.md](CHANGELOG.md) lists what changed in each release.

## Development

```sh
go test ./...                                        # also run by the Dockerfile
node --test 'web/test/*.test.mjs'                    # browser-side tests
```

The web UI is embedded, so restart after frontend edits. The simulator takes `SIM_*` variables to fake misbehaving routers. See `cmd/swp08sim`.

## Releasing

You cut releases by tag and promote them without a rebuild. Bump `Version` in `internal/app/version.go`, the `**Version:**` line above and the `## Beta <version>` heading in CHANGELOG.md, which `TestVersionNoted` checks. RELEASING.md has the rest.

## AI disclosure

Sode is written with AI assistance. Claude (Anthropic), through Claude Code, produces much of the code, tests and documentation, working from issues and direction written by the maintainer. The maintainer reviews it and tests it against a real router. Commits made this way carry a `Co-Authored-By: Claude` trailer. Treat Sode like any software that has not been audited. Read it, and try it on a non-critical router first.

## License

Public domain ([Unlicense](LICENSE)). The vendored Lucide icons keep their own license in `web/vendor/patchbay/icons/`.
