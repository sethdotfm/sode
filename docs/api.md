# Sode API guide

A short guide to `/api/v1`. The contract is the OpenAPI file Sode serves at `/api/v1/openapi.yaml`, open even while the API is off. It lists every path, field and error code. Settings and the rest of the server are in [reference.md](reference.md).

## Turn it on

The API is off until an administrator ticks *API enabled* in Settings (advanced). While off, every call answers 503 `api_off`, except the OpenAPI file and `/api/health`. The same area sets:

- *Rate limit*: requests a minute per key (default 600, 0 means none). A call with no key is limited per address.
- *Take cap*: API takes in flight across all callers (default 4). The router answers one message at a time, so over the cap a take gets 429 `take_cap`.
- *Guest API*: lets a call with no key in, with Guest's scopes only.
- *Log API reads*: one Communication Log row for every read. Off keeps the log to commands, refusals and key events.

## Keys

A key is `sode_` plus 32 random bytes, sent as `Authorization: Bearer <key>`. A key is never accepted in a query string. Sode shows it once when it is made and stores only a hash in `keys.json` beside `config.yaml`, so a leaked config file leaks no working key. Each key has a key name such as "Panel 21", an optional expiry, a created time, a last used time and a use count. History and the Communication Log show the actor with the key name, for example "PanelsBot (Panel 43)". A caller may also send `X-Sode-Via`, free text that the log shows beside the key name marked unverified.

Two kinds of owner:

- **A user's key** acts as that user, with that user's rights and Me presets, and no narrowing. A user makes it in Settings > My user > **My API keys**, a dialog next to Change my PIN. It asks for the user's own PIN, in the same pool and with the same waits as Change my PIN. It dies when the user changes their PIN, is renamed or is removed (ADR 0003), the same moment their session tokens die. Revoke and expiry end it too.
- **An API client's key** carries the client's own rights and scopes. An administrator makes a client and its keys in Manage users, on the API clients tab, with the Advanced Settings PIN. Changing a user's PIN does not touch it. It ends on revoke, expiry, or removing or renaming its client.

Per owner, a user may hold 8 keys and a client 20 (`maxUserKeys`, `maxClientKeys`). An administrator lists every key and revokes one or all keys of an owner with the Advanced Settings PIN. A stream open on a key that is revoked, expires or loses its owner is closed, and the Communication Log's disconnect row says why.

No PIN is ever sent over the API, and there is no API sign-in. The Lock and auto-lock never apply to a key.

## Scopes, rights and visibility

**Scopes** say which areas a caller may use: `names`, `tally`, `protected`, `status`, `presets`, `history`, `layouts`, `log` and `take`. A user's key has all but `log`. An API client and Guest have the scopes set for them, and Guest starts with `names`, `tally`, `protected` and `status`. A call without the scope gets 403 `scope`.

**Rights** say which routes a caller may command: allowlists of sources, destinations, levels and presets, set on each user, each API client and Guest. An unset list means everything, so an old file changes nothing. A take with any route outside the rights is refused whole before anything is sent (403 `outside_rights`, no History entry, an `api` row). The browser obeys the same check. Undo checks the destination and level but not the Before source it restores. Protected destinations are still refused per route.

**Visibility** is part of the rights, read-only (the default) or hidden. Read-only shows everything and refuses commands outside the rights. Hidden removes out-of-rights sources and destinations from names, tally, protected, History, Groups and the stream for that caller, and Undo, Redo and Retry cover only visible routes. Hidden applies to API callers only, because the browser connection already broadcasts names, tally and History to every browser (ADR 0004). The Communication Log ignores visibility.

## Numbers

Sources and destinations are the router's own numbers, 0-based. `/names` carries `numberOffset`; add it to show the number a person sees. Levels are UI levels, from 1, the same numbers the rights lists use. A source of -1 means nothing is routed.

## Endpoints

All paths are under `/api/v1`. Errors are `{"error":{"code","message"}}`.

| Path | Scope | What it does |
| --- | --- | --- |
| `GET /names`, `/tally`, `/protected`, `/status` | the same name | Names, what each destination is routed from per level, Protected destinations, Server link and Router link |
| `GET /presets` | `presets` | Everyone presets, plus Me presets for a user's key. Never This browser |
| `GET /history?before=&n=` | `history` | History, newest first. A page never ends inside a Group |
| `GET /history/groups/last` | `history` | The latest Group with its entries, oldest first |
| `GET /layouts` | `layouts` | Panel layouts, empty until the panel page exists |
| `GET /log?before=&n=` | `log` | The Communication Log |
| `POST /takes` | `take` | A take by routes: `{"routes":[{"src":1,"dst":2,"levels":[1]}]}`, 1 to 200 routes |
| `POST /presets/{scope}/{name}` | `take` | Fire a preset, scope `everyone` or `me` |
| `POST /history/{id}/undo`, `/redo`, `/retry` | `take` | Undo, Redo or Retry one History entry |

Known v1 limits. Undo, Redo and Retry work on one entry at a time. To undo a Group, call `GET /history/groups/last`, then undo each returned entry. Single-route takes and External entries are never returned there. There are no settings, users, keys or lock endpoints.

## Takes

A take request is its own Confirm: nothing asks a person, so guard the button on your side. The request waits and returns when the router has answered or timed out every route. It is always 200 once the take ran, even when a route failed, so read each result: `connected`, `already` or `failed`, with the History `entry` id. Refusals before anything is sent are errors.

The worst case for one route is about 35 seconds: check 1 s, send queue up to 30 s, route 3 s, grace 1 s. More than 8 routes run 8 at a time and take longer. Common proxies cut a request at 60 seconds, so raise the proxy's read timeout for `/api/v1`. Sode sets a write deadline on each take request and no server-wide write timeout, so `/ws` and the stream are not cut.

A route the router confirms after the take gave up is a Late Result. The response already carries the entry ids, and the stream sends an `entryUpdate` for each, so match updates by `id`.

**Idempotency-Key.** An optional header (1 to 128 printable characters). A repeat of the same request with the same key from the same API key within 10 minutes returns the first answer with `Idempotent-Replay: true`, so retrying after a timeout or a proxy cut does not take twice. The same key with a different request is 422 `idempotency_mismatch`. A request that did nothing (bad input, rate limit, take cap) is not remembered.

**Large takes.** A take of 6 or more routes skips the Before check, to spare the router, and the response carries `checkSkipped: true`. Each entry's Prev then comes from Sode's cache, so a later Undo may restore less accurately. Send fewer than 6 routes per request to keep the check.

## Stream

`GET /stream?topics=names,tally,status,history,results` upgrades to a WebSocket that only sends. A browser page cannot set the Bearer header, so use a client library. Each topic needs the scope of the same read (`results` needs `history`). The first frame is `hello`, then one `snapshot` each for `names`, `tally` and `status`, then changes: `names`, `tally` (only changed routes), `status`, `entry`, `entryUpdate` and `historyCleared`. `ping` is a heartbeat every 5 seconds. A slow client is disconnected. Connects and disconnects are `api` rows with the key name.

## Require HTTPS

Settings > Proxy > Require HTTPS (ADR 0005) refuses every request except `/healthz` and `/api/health` unless it arrives from a trusted proxy with `X-Forwarded-Proto: https`. API paths answer 403 `https_required`. It needs `trustedProxies` set, and a save that turns it on must itself arrive over HTTPS. See [reference.md](reference.md#behind-a-proxy) for the recovery flag.

## The Communication Log

Every API command, refusal, key made, revoked or expired, rights edit, stream connect and disconnect, and the API and Require HTTPS switches write an `api` row. Failed keys write one row per address per minute with a count. No key, key hash or PIN is ever written.
