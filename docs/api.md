# Sode API guide

A guide to `/api/v1`. The contract is the OpenAPI file Sode serves at `/api/v1/openapi.yaml`, open even while the API is off. It lists every path, field and error code. Settings and the rest of the server are in [reference.md](reference.md).

## Turn it on

The API is off until an administrator ticks *API enabled* in Settings (advanced). While off, every call answers 503 `api_off`, except the OpenAPI file and `/api/health`. The same area sets:

- *Rate limit*: requests a minute per key (default 600, 0 means none). A call with no key is limited per address.
- *Take cap*: API takes in flight across all callers (default 4). The router answers one message at a time, so over the cap a take gets 429 `take_cap`.
- *Guest API*: lets a call with no key in, with Guest's scopes only.
- *Log API reads*: one Communication Log row for every read. Off keeps the log to commands, refusals and key events.

## Keys

A key is `sode_` plus 43 URL-safe characters (32 random bytes). Treat it as opaque. Send it as `Authorization: Bearer <key>`. A key is never accepted in a query string. Sode shows it once when it is made and stores only a hash in `keys.json` beside `config.yaml`, so a leaked config file leaks no working key. Each key has a key name such as "Panel 21", an optional expiry, a created time, a last used time and a use count. History and the Communication Log show the actor with the key name, for example "PanelsBot (Panel 43)". A caller may also send `X-Sode-Via`, free text up to 32 characters that the log shows beside the key name marked unverified.

Two kinds of owner:

- **A user's key** acts as that user, with that user's rights and Me presets, and no narrowing. A user makes it in Settings > My user > **My API keys**, a dialog next to Change my PIN. It asks for the user's own PIN, in the same pool and with the same waits as Change my PIN. It dies when the user changes their PIN, is renamed or is removed (ADR 0003), the same moment their session tokens die. Revoke and expiry end it too.
- **An API client's key** carries the client's own rights and scopes. An administrator makes a client and its keys in Manage users, on the API clients tab, with the Advanced Settings PIN. Changing a user's PIN does not touch it. It ends on revoke, expiry, or removing or renaming its client.

Per owner, a user may hold 8 keys and a client 20. An administrator lists every key and revokes one or all keys of an owner with the Advanced Settings PIN. A stream open on a key that is revoked, expires or loses its owner is closed, and the Communication Log's disconnect row says why.

No PIN is ever sent over the API, and there is no API sign-in. The Lock and auto-lock never apply to a key.

## First call

These examples assume Sode at `http://sode.local:8080` (put TLS in front, see Require HTTPS below) and a key in `$KEY`. Read the names first:

```sh
curl -s -H "Authorization: Bearer $KEY" http://sode.local:8080/api/v1/names
```

```json
{"numberOffset":1,"sources":[{"n":0,"name":"CAM 1"},{"n":1,"name":"CAM 2"}],"destinations":[{"n":0,"name":"MON A"},{"n":1,"name":"MON B"}],"namesAt":"2026-11-08T09:30:00Z"}
```

`n` is the router's number, 0-based. With `numberOffset` 1 the router's screen calls CAM 1 source 1, but you send `"src":0`. Levels are never offset: send `"levels":[1]` for the level a page calls level 1. Now take CAM 2 to MON A on level 1. The `Idempotency-Key` makes a retry after a timeout safe:

```sh
curl -s -X POST http://sode.local:8080/api/v1/takes \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -H "Idempotency-Key: panel-21-0001" \
  -d '{"routes":[{"src":1,"dst":0,"levels":[1]}]}'
```

```json
{"ok":true,"checkSkipped":false,"results":[{"id":1,"src":1,"dst":0,"levels":[1],"result":"connected","ok":true,"entry":412}]}
```

Read `results[].result` (`connected`, `already` or `failed`), not just the status: a take that ran is always 200. An error has one shape, here for a key without the `take` scope:

```json
{"error":{"code":"scope","message":"this key lacks the \"take\" scope"}}
```

## Scopes, rights and visibility

**Scopes** say which areas a caller may use: `names`, `tally`, `protected`, `status`, `presets`, `history`, `layouts`, `log`, `take`, `panels` (read panels) and `panelsmanage` (create, change and delete panels, and artwork). A user's key has all but `log`, always: the log scope can be given only to an API client or Guest. An API client and Guest have the scopes set for them, and Guest starts with `names`, `tally`, `protected` and `status`. A call without the scope gets 403 `scope`.

**Rights** say which routes a caller may command: allowlists of sources, destinations, levels and presets, set on each user, each API client and Guest. An unset list means everything, so an old file changes nothing. A take with any route outside the rights is refused whole before anything is sent (403 `outside_rights`, no results, no History entry, an `api` row). The browser obeys the same check, with a failed Result per route. Undo checks the destination and level but not the Before source it restores. Protected destinations are still refused per route.

**Visibility** is part of the rights, read-only (the default) or hidden. Read-only shows everything and refuses commands outside the rights. Hidden removes out-of-rights sources and destinations from names, tally, protected, History, Groups and the stream for that caller, and Undo, Redo and Retry cover only visible routes. A preset that names any hidden source or destination is not listed and cannot be fired (404, as for an unknown preset), a tally change with a hidden source is left out of the stream as it is of `GET /tally`, and a 403 `outside_rights` says what kind is outside the rights without the number. Hidden applies to API callers only, because the browser connection already broadcasts names, tally and History to every browser (ADR 0004). The Communication Log ignores visibility.

## Numbers

Sources and destinations are the router's own numbers, 0-based. `/names` carries `numberOffset`; add it to show the number a person sees. Levels are UI levels, from 1, the same numbers the rights lists use. A source of -1 means nothing is routed. If a route omits `levels` or sends `[]`, Sode uses the *Default levels* setting, which no endpoint reports, so always send `levels`.

## Endpoints

All paths are under `/api/v1`.

| Path | Scope | What it does |
| --- | --- | --- |
| `GET /names`, `/tally`, `/protected`, `/status` | the same name | Names, what each destination is routed from per level, Protected destinations, Server link and Router link (`router` is `up`, `connecting` or `down`, plus `routerUp` and an `error` flag; `address` and the `err` text only with the `log` scope, since they name the router) |
| `GET /presets` | `presets` | Everyone presets, plus Me presets for a user's key. Never This browser |
| `GET /history?before=&n=` | `history` | History, newest first, 50 per page by default, 500 at most. A page never ends inside a Group |
| `GET /history/groups/last` | `history` | The latest Group with its entries, oldest first |
| `GET /layouts` | `layouts` | Panel layouts, empty until the panel page exists |
| `GET /log?before=&n=` | `log` | The Communication Log, 100 per page by default, 500 at most |
| `POST /takes` | `take` | A take by routes: `{"routes":[{"src":1,"dst":2,"levels":[1]}]}`, 1 to 200 routes |
| `POST /presets/{scope}/{name}` | `take` | Fire a preset, scope `everyone` or `me` |
| `POST /history/{id}/undo`, `/redo`, `/retry` | `take` and `history` | Undo, Redo or Retry one History entry |
| `GET /stream` | per topic | A WebSocket of live changes, see Stream |
| `GET /panels`, `GET /panels/{id}` | `panels` | The panels the caller owns or may use, or all of them for a Trusted user. `canEdit` is false for use only |
| `POST /panels` | `panelsmanage` | Create a panel. A user's key only: the user owns it |
| `PUT /panels/{id}`, `DELETE /panels/{id}` | `panelsmanage` | Change or delete. The owner, or a Trusted user for anyone's |
| `POST /panels/{id}/copy` | `panelsmanage` and `panels` | Save a copy: a private panel the caller owns, from any panel the caller may use |
| `POST /panels/{id}/key`, `DELETE /panels/{id}/key` | `panelsmanage` | Make or rotate, or revoke, the key of a panel that acts as an API client. Owner or Trusted, a user's key. The secret comes back once, with `link` (`panels/<id>#key=<secret>`, relative to Sode's root) |
| `POST /panels/{id}/artwork`, `GET` and `DELETE /panels/{id}/artwork/{art}` | `panelsmanage` (`panels` to read) | Artwork: the raw image as the body, JPEG, PNG, GIF or SVG up to 1 MB, its type decided by the bytes |

A panel with `acts: api` has an API client of its own, `Panel <id>`, made with the panel by a Trusted user (403 `trusted_required` otherwise). Its `clientRights` default to the creator's and can be no wider than the creator's rights (403 `outside_rights`, nothing created or changed); the check runs whenever a save sets them to something other than what is stored, so a panel an administrator widened still saves. Its scopes start as `names`, `tally`, `protected`, `status`, `presets`, `history`, `layouts`, `take` and `panels`. Only the Advanced Settings PIN widens it, in Manage users, where it shows on the API clients tab with its panel and does not count toward the 12-client limit. Switching the panel to `session` or deleting it removes the client and ends its keys and streams. In History its actions read like any client's, "Panel p-1a2b3c4d (Wall tablet)"; a user's key named "Panel 21" reads "Seth (Panel 21)". Changing a user's Rights lists the panel clients they created, in the Manage users list (`panelClients`, with `wider` when the client now exceeds the user) and in the `rightsEdit` row.

A panel's key is one key on its client: making a new one ends the old key and its streams (`rotated: true`), and revoking leaves the panel and client without a key. Open the `link` and the page uses the key as its Bearer key with no sign-in, and takes as `Panel <id>` with the client's Rights. The fragment is never sent to Sode, so the key is in no request line or log row; a key in a query string is ignored. A panel with `settings.kiosk` set, or a link ending `&kiosk=1`, hides Sode's header.

Panels need the API switch and use the same scopes, rate limit and errors as every other call. A panel the caller may not use is 404, never 403. Artwork is served with headers that stop a script in an SVG from running; show it as an image.

There are no settings, users, keys or lock endpoints.

## Errors

Every error is `{"error":{"code","message"}}`. A 401 carries `WWW-Authenticate: Bearer`, and a wrong method on a known path is 405 with `Allow`. 429 from the rate limit carries `Retry-After` in seconds.

| Status | Code | Cause | What to do |
| --- | --- | --- | --- |
| 503 | `api_off` | The API switch is off | Ask an administrator |
| 401 | `key_required` | No key, and Guest API is off | Send a key |
| 401 | `invalid_key` | The key is unknown, malformed, or its owner is gone | Make a new key |
| 401 | `key_expired` | The key's expiry has passed | Make a new key |
| 403 | `scope` | The key lacks the scope this path needs | Add the scope to the client |
| 403 | `outside_rights` | A route or preset is outside the key's rights; nothing was sent | Change the request or the rights |
| 403 | `https_required` | Require HTTPS is on and the request did not arrive securely | Use the HTTPS URL |
| 403 | `not_owner` | A panel the caller may only use | Save a copy, or ask the owner |
| 403 | `user_required` | Creating or copying a panel with a key that is not a user's | Use a user's key |
| 403 | `trusted_required` | Making a panel an API client, or changing its rights, as a user who is not Trusted | Ask a Trusted user |
| 400 | `bad_request` | A parameter or body is not valid | Fix the request |
| 413 | `too_large` | The body is over 64 KiB (256 KiB for a panel, 1 MB for artwork) | Send less |
| 415 | `unsupported_type` | Artwork that is not a JPEG, PNG, GIF or SVG | Send one of those |
| 500 | `internal` | A panel could not be written to disk | Check the disk and the logs |
| 404 | `not_found` | No such path, History entry or visible preset, or scope `me` on a client or Guest key | Check the id, name or scope |
| 405 | `method_not_allowed` | The path does not take this method | Use a method in `Allow` |
| 409 | `conflict` | Redo of a failed entry, or Retry of one that succeeded; a panel over its limits; a key for a panel that does not act as an API client | Use the other verb, or switch the panel to `api` |
| 422 | `nothing_to_do` | The plan has no route left to run: nothing to undo, a Redo of a route with no source, or a preset whose parts are all gone from the router | Nothing to do |
| 422 | `idempotency_mismatch` | The Idempotency-Key was used for a different request | Use a new key |
| 429 | `rate_limited` | Too many requests, or ten bad keys from this address in a minute | Wait `Retry-After` seconds |
| 429 | `take_cap` | The global cap on API takes in flight is reached | Retry in a second or two (no `Retry-After`) |
| 429 | `too_many_streams` | The key or the address already has 8 open streams | Close one |

## Takes

A take request is its own Confirm: nothing asks a person, so guard the button on your side. The request waits and returns when the router has answered or timed out every route. It is always 200 once the take ran, even when a route failed, so read each result: `connected`, `already` or `failed`, with the History `entry` id. Refusals before anything is sent are errors.

The worst case for one route is about 35 seconds: check 1 s, send queue up to 30 s, route 3 s, grace 1 s. More than 8 routes run 8 at a time and take longer. Common proxies cut a request at 60 seconds, so raise the proxy's read timeout for `/api/v1`.

A route the router confirms after the take gave up is a Late Result. The response already carries the entry ids, and the stream sends an `entryUpdate` for each, so match updates by `id`.

**Idempotency-Key.** An optional header, 1 to 128 printable characters with no spaces. A repeat of the same request with the same key from the same API key within 10 minutes returns the first answer with `Idempotent-Replay: true`, so retrying after a timeout or a proxy cut does not take twice. The same key with a different request is 422 `idempotency_mismatch`. A request that did nothing (bad input, rate limit, take cap) is not remembered, except an `outside_rights` refusal, which a repeat with the same key replays for the same 10 minutes, even if the rights have widened since. A call without an API key (a keyless Guest) cannot send the header: guests behind one address would share answers, so it is 400 `bad_request`.

**Large takes.** A take of 6 or more routes skips the Before check, to spare the router, and the response carries `checkSkipped: true`. Each entry's Prev then comes from Sode's cache, so a later Undo may restore less accurately. Send fewer than 6 routes per request to keep the check.

**Undo, Redo and Retry** work on one entry at a time. To undo a Group, call `GET /history/groups/last`, then undo each returned entry. Single-route takes and External entries are never returned there.

## Stream

`GET /stream?topics=names,tally,status,history,results` upgrades to a WebSocket that only sends: anything the client writes is ignored. A browser page cannot set the Bearer header, so use a client library. Each topic needs the scope of the same read (`results` needs `history`); a topic outside the key's scopes is 403 `scope`, and an unknown or missing topic is 400.

Every frame is JSON `{"t","topic","data"}`. The first is `hello`, then one `snapshot` each for `names`, `tally` and `status`, with `data` shaped like the REST read. A `history` subscriber gets no snapshot: call `GET /history` first and bridge by id. `results` sends only `entryUpdate`, which suits a panel that fires takes and wants nothing but Late Results.

```json
{"t":"hello","data":{"v":1,"topics":["tally","results"],"numberOffset":1}}
{"t":"snapshot","topic":"tally","data":{"levels":[{"level":1,"label":"1"}],"routes":[{"dst":0,"level":1,"src":3}]}}
{"t":"tally","topic":"tally","data":{"routes":[{"dst":0,"level":1,"src":1}]}}
{"t":"entryUpdate","topic":"results","data":{"id":412,"time":"2026-11-08T09:31:02Z","client":"PanelsBot (Panel 43)","actor":{"kind":"client","name":"PanelsBot","key":"Panel 43"},"src":1,"srcName":"CAM 2","dst":0,"dstName":"MON A","levels":[1],"prev":[[1,3]],"ok":true,"late":[1]}}
```

Changes follow as `names`, `tally` (only changed routes), `status` (the same fields as `GET /status`), `entry`, `entryUpdate` (the whole Entry again) and `historyCleared`. `ping` is a heartbeat every 5 seconds, ignore it.

A rights or scope change applies to an open stream at once, to later frames only: a client keeps what it already holds until it reconnects. A stream whose key lost a topic's scope, or whose API switch went off, is closed. A key or an address can hold 8 streams at a time, and a ninth gets 429 `too_many_streams`. A client that stops reading is dropped when a frame write stalls for 5 seconds or 256 frames are queued. Connects and disconnects are `api` rows with the key name, and a disconnect row says why.

## Require HTTPS

Settings > Proxy > Require HTTPS (ADR 0005) refuses every request except `/healthz` and `/api/health` unless it arrives from a trusted proxy with `X-Forwarded-Proto: https`. API paths answer 403 `https_required`. It needs `trustedProxies` set, and a save that turns it on must itself arrive over HTTPS. See [reference.md](reference.md#behind-a-proxy) for the recovery flag.

## The Communication Log

Every API command, refusal, key made, revoked or expired, rights edit, stream connect and disconnect, and the API and Require HTTPS switches write an `api` row. Failed keys write one row per address per minute with a count. No key, key hash or PIN is ever written.
