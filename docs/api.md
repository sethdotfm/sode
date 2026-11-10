# Sode API guide

A guide to `/api/v1`. The contract is the OpenAPI file Sode serves at `/api/v1/openapi.yaml`, open even while the API is off. It lists every path, field and error code. Settings and the rest of the server are in [reference.md](reference.md).

## Turn it on

The API is off until an administrator ticks *API enabled* in Settings (advanced). While off, every call answers 503 `api_off`, except the OpenAPI file and `/api/health`. The same area sets:

- *Rate limit*: requests a minute per key (default 600, 0 means none). A call with no key is limited per address.
- *Take cap*: API takes in flight across all callers (default 4). The router answers one message at a time, so over the cap a take gets 429 `take_cap`.
- *Guest API*: lets a call with no key in, with Guest's scopes only.
- *Log API reads*: one Communication Log row for every read. Off keeps the log to commands, refusals and key events.

## Keys

A key is `sode_` plus 43 URL-safe characters (32 random bytes). Treat it as opaque. Send it as `Authorization: Bearer <key>`. A key is never accepted in a query string. Sode shows it once when it is made and stores only a hash in `keys.json` beside `config.yaml`, so a leaked config file leaks no working key. Each key has a key name such as "Panel 21", an optional expiry, a created time, a last used time and a use count. History and the Communication Log show the actor with the key name, for example "PanelsBot (Panel 43)". A caller may also send `X-Sode-Via`, free text up to 32 characters that the log shows beside the key name marked unverified. A panel page sends `X-Sode-Panel` (the panel id) and `X-Sode-Instance` (its instance name, up to 40 characters), so its rows read `panel <short name> (<name>), instance <instance>`; a panel's own key proves the panel, a user's key does not and the tag is marked unverified.

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
| `GET /panels`, `GET /panels/{id}` | `panels` | The panels the caller owns or may use, or all of them for a Trusted user. `canEdit` is false for use only. `GET /panels/{id}` also takes the panel's short name (`slug`) in place of the id |
| `GET /panels/people` | `panels` | The names of Sode's users, for the Share picker. A user's key only (403 `user_required`). Sharing itself is the `share` field of `PUT /panels/{id}`: `{mode: me\|named\|everyone, users[]}`, names kept only for `named`, and a change reaches a user's next list or launch |
| `POST /panels` | `panelsmanage` | Create a panel. A user's key only: the user owns it. `settings` is free-form, except `settings.fixedDests`, the Fixed destination: a list of at most 16 different destination numbers (0-based), checked on every save (400 `bad_request`). Empty or absent means off |
| `PUT /panels/{id}`, `DELETE /panels/{id}` | `panelsmanage` | Change or delete. The owner, or a Trusted user for anyone's |
| `POST /panels/{id}/copy` | `panelsmanage` and `panels` | Save a copy: a private panel the caller owns, from any panel the caller may use |
| `GET /panel-templates`, `GET /panel-templates/{id}` | `panels` | Templates: the Everyone ones, plus a user's own Me ones. Another user's Me template is 404 |
| `POST /panel-templates` | `panelsmanage` | Save a template of a layout: rows, columns and each cell's type, levels and look. Cell contents (`n`, `scope`, `name`, `caption`) are dropped unless asked for: `sources` keeps those of Source and Preset cells, `dests` those of Destination cells (the older `contents: true` means both), so cells come back unassigned; `settings` are kept when sent, and `settings.fixedDests` only with `dests`, so a template never carries a destination by accident. Scope `me` or `everyone`; `everyone` needs a Trusted user, or the Advanced Settings PIN as `pin` in the body (403 `trusted_required`, a wrong PIN counts toward its lockout). A user's key only. Names are unique per place (409 `conflict`) |
| `DELETE /panel-templates/{id}` | `panelsmanage` | The user who saved it, or a Trusted user for an Everyone template |
| `GET /default-artwork`, `GET /default-artwork/images/{art}` | `panels` | Default artwork: one shared set, `defaults[]` of `kind` (`source`, `destination`, `preset`), `n` or `scope` (`everyone` only; Me presets have no default) and `name`, `art`, `artB` and `hideText`. A Source or Destination default's `name` is the router name when it was set. A Hidden key sees only items inside its Rights, and an image only while a default it sees names it. `GET /panels/{id}` carries the defaults of the items its cells show as `artDefaults`; a cell's own artwork wins, and a cell never stores a default |
| `PUT /default-artwork`, `DELETE /default-artwork` | `panelsmanage` | Set or clear one item's default; the JSON body names the item (`DELETE` takes a body too). A user's key that is Trusted, or the Advanced Settings PIN as `pin` (403 `trusted_required`, a wrong PIN counts toward its lockout). Set needs Icon A or B; a panel's artwork id is copied from the panel named in `panel` and comes back as a `d-` id, under the same 2 MB and 1024 by 1024 limits as an upload and 64 MB for all Default artwork images (409 `conflict`). An Icon library icon (`lib:<id>`) is kept as the reference and never copied (404 `not_found` when the library lacks it); deleting it from the library leaves the default drawing no Icon A. Open panels update with no panel save |
| `GET /icon-library`, `GET /icon-library/{icon}` | `panels` | The Icon library: one install-wide set, `icons[]` of `id`, `name`, `type` (`image/png` or `image/svg+xml`) and `size`, and each icon's file with the same headers as a panel's artwork. A cell's Icon A names one as `art: "lib:<id>"`; an id the library does not hold saves and draws nothing (the file route is 404) |
| `POST /icon-library` | `panelsmanage` | Add an icon: JSON `{name, data, pin?}` with the file base64 in `data`. A user's key that is Trusted, or the Advanced Settings PIN as `pin` (403 `trusted_required`). PNG or SVG by content (415 `unsupported_type`), under the artwork limits (2 MB, 413 `too_large`; a PNG at most 1024 by 1024, 400 `bad_request`) and 64 MB for the whole library (409 `conflict`). The name is 1 to 40 characters; one another icon has, ignoring case, is stored as "Name 2" and so on, so the answer carries the stored name. Answers 201 with the icon |
| `PATCH /icon-library/{icon}`, `DELETE /icon-library/{icon}` | `panelsmanage` | Rename (JSON `{name, pin?}`, answers the icon; the id stays, a clash becomes "Name 2") or delete (optional JSON `{pin}`, answers 204). A user's key that is Trusted, or the Advanced Settings PIN as `pin` (403 `trusted_required`). Cells that name a deleted icon keep `lib:<id>` and draw no Icon A |
| `POST /icon-library/rescan` | `panelsmanage` | Add the files in `icons/drop-in/` beside the config, as Sode does at start (optional JSON `{pin}`). A user's key that is Trusted, or the Advanced Settings PIN as `pin` (403 `trusted_required`). Each file has the upload's limits and is named after its file name ("Name 2" on a clash); an added file leaves the directory. Answers 200 with `added[]` (icons) and `skipped[]` of `file` and `reason`; a skipped file stays. Links are never followed |
| `GET /icon-library/{icon}/uses` | `panelsmanage` | `{cells, panels}`: how many cells across every panel name `lib:<icon>`, and on how many panels. Counts only |
| `GET /panels/{id}/keys` | `panelsmanage` | The instances of a panel that acts as an API client: `keys[]` (`id`, `name`, `created`, `lastUsed`, `online`, and for a pending setup link `setup: true` and `expires`) and `max`. Owner or Trusted, a user's key. Never a secret |
| `POST /panels/{id}/key`, `DELETE /panels/{id}/key?instance=<name>` | `panelsmanage` | Make or rotate, or revoke, the key of one named instance of a panel that acts as an API client. Owner or Trusted, a user's key. `instance` is required (400 `bad_request`), in the body for POST. The secret comes back once, with `link` (`panels/<id>?instance=<name>#key=<secret>`, relative to Sode's root). With `{"setup": true}` in the body it returns a one-time setup link for that instance instead (secure mode, below) |
| `PUT /panels/{id}/networks` | `panelsmanage` | Set the networks a panel's key may be used from: `{"networks": ["192.168.1.0/24"]}`. Owner or Trusted, a user's key |
| `POST /panels/{id}/setup` | none | Trade a setup token for the panel's key. No key is sent: the token is the credential |
| `POST /panels/{id}/push` | `panelsmanage` | Push: copy the panel's saved grid, `settings` and unknown fields to the panels in `targets`. Each keeps its name, id, owner, sharing and `settings.fixedDests`. `acts`, the API client and its rights are never copied. Needs the right to change the source and every target (owner, or Trusted for anybody's); one refused target means none is changed. A checkpoint of each target is kept first, and open panels update like a saved edit. Artwork the target lacks is copied while it fits under the target's 64 MB total, and the ids that did not fit come back as `artSkipped` on that target (a panel Copy reports the same field). Answers `{pushed:[{panel, rev, checkpoint, artSkipped?}]}` |
| `GET /panels/{id}/checkpoints`, `POST /panels/{id}/checkpoints/{cp}/restore` | `panels`, `panelsmanage` | A panel's checkpoints, newest first, at most 10 (the oldest is dropped). Restore puts one back and keeps what it replaces as a new checkpoint. Owner or Trusted |
| `GET`, `POST /panel-sets`; `PUT`, `DELETE /panel-sets/{sid}` | `panels`, `panelsmanage` | Checkpoint sets: one saved layout per panel. `POST {name, scope: me\|everyone, panels[]}` saves those panels' current layouts; `everyone` needs a Trusted user, or the Advanced Settings PIN as `pin` in the body (403 `trusted_required`, a wrong PIN counts toward its lockout). `PUT {name?, panels?, pin?}` renames or saves the layouts again. Me sets are the owner's. An Everyone set can be used by any user and changed or deleted by its owner, a Trusted user, or anyone who sends the PIN as `pin` in the body (`DELETE` takes a JSON body for it). The PIN is never stored |
| `POST /panel-sets/{sid}/push` | `panelsmanage` | Give each panel in the set its own layout, with a checkpoint of each first. `{targets?}` narrows it to some of the set's panels. A panel that no longer exists comes back in `skipped` |
| `POST /panels/{id}/instances` | `panels` | Keepalive of one open panel page, body `{"device":"Lobby tablet"}`, every 15 seconds. Anyone who may use the panel |
| `GET /panels/{id}/instances/stream?device=` | `panels` | Held connection of one open panel page, so Sode sees a closed page after 8 seconds, not 45. Send the Authorization header with `fetch`. Answers `application/x-ndjson`, a line `{"online":true,"graceSeconds":8}` at once and every 10 seconds, until either side closes it. An Assign sends one last line with `t` `assign` and ends it, and a rename one with `t` `rename`. A panel read's `instances` rows then carry `page`, and from that stream `addr`, `signIn` (`client`, `user` or `none`), `user`, `trusted` (that user is Trusted now) and, while it is held, `streamSince`. The keepalive above stays as the fallback |
| `GET /panels/{id}/instances/keyless?instance=&ended=1` | none | Held connection of a panel page with no working key, a Keyless instance (ADR 0008). No Authorization header; `{id}` is a panel id or short name, and the answer is the same whether or not it exists (400 `bad_request` for a malformed one). `instance` is the name the page holds, if any; `ended=1` says its key answered `key_revoked` or `key_expired`. Answers `application/x-ndjson`, `{"keyless":true}` at once and every 10 seconds, and nothing else until an Assign sends its one last line, as on the instance stream. Sode keeps at most 1000 in memory for 24 hours, oldest dropped. A panel read's `keyless` rows (for a caller who may edit it) carry `keyless` (the id an Assign names), `device` (its own name, else the last name a stream from its address carried, else the address), `addr`, `signIn` (`none` or `ended`), `stream`, `seen` and `streamSince` |
| `POST /panels/{id}/assign` | `panelsmanage` | Assign an open instance to this panel: `{"from": "<panel id>", "instance": "<device>", "pin": "<Advanced Settings PIN>", "as": "client"}`. A user's key, Trusted or with the PIN (403 `trusted_required`). The instance must hold `/panels/{from}/instances/stream` now (409 `conflict`). `as` is `client` (a key) or `user` (the device's own sign-in); left out, `client` where the target has an API client, else `user`, and the other on a panel that is not `both` is 409 `conflict`. For `user` no key is made and no name or count is checked: the last line is `{"t":"assign","panel","slug","instance","signIn":"user"}`. For `client` the target must not be at its 20 instances (409 `conflict`). A target that already holds that name, as a key or as any instance Sode holds for it, answers 409 `name_taken` with `error.suggest`, a free name; send one as `name` and the instance uses it on the target. The target's client gets a key named for the instance (or `name`), sent only as the last line `{"t":"assign","panel","slug","instance","key"}` of the newest stream the instance holds; the answer is `{from, panel, instance}`. When no stream wrote the line, the key is revoked and the answer is 409 `conflict`. A keyless instance is named by `{"keyless": "<id>"}` in place of `from` and `instance`, may be assigned to the panel it shows, and gets a key only from an address inside the target client's allowed networks (403 `network_not_allowed`; an empty list allows any). Writes one `api` row `assign` |
| `POST /panels/{id}/instances/rename` | `panelsmanage` | Rename an open instance: `{"instance": "<its name now>", "name": "<new name>", "pin": "<Advanced Settings PIN>"}`. A user's key: the panel's owner, a Trusted user, or the PIN (403 `trusted_required`). The instance must hold `/panels/{id}/instances/stream` now (409 `conflict` once it timed out). The name is its device name, up to 40 characters; a name the panel holds, as a key or as any instance Sode knows, answers 409 `name_taken` with `error.suggest`, and the same name is 400 `bad_request`. Sode renames that instance's key, moves its watches and sends `{"t":"rename","instance":"<new name>"}` as the stream's last line; the answer is `{panel, from, instance}`. Writes one `api` row `rename` |
| `POST /panels/{id}/watch`, `DELETE /panels/{id}/watch` | `panels` | Watch or stop watching, the whole panel or with `?instance=<device>` one instance. A user's key, for a panel the user owns, or any panel for a Trusted user |
| `POST`, `DELETE /panels/{id}/instances/hide?instance=` | `panels` | Hide one instance from the caller's panels page, or show it again. Same callers as watching. It ends no key and no watch; it lasts in memory until the device is next heard after it was lost |
| `POST /panels/{id}/artwork`, `GET` and `DELETE /panels/{id}/artwork/{art}` | `panelsmanage` (`panels` to read) | Artwork: the raw image as the body, JPEG, PNG, GIF or SVG up to 2 MB, at most 64 MB in all per panel (an upload past it is 409 `conflict`; there is no file-count limit), its type decided by the bytes. A PNG, JPEG or GIF with a width or height over 1024 pixels is refused (400 `bad_request`); SVG has only the size limit. Sode never resizes, and artwork stored earlier is kept as it is |

A panel with `acts: api` or `acts: both` has an API client of its own, `Panel <id>`, made with the panel by a Trusted user (403 `trusted_required` otherwise). The two differ only in the page: `api` opens only from a link with a key, while `both` falls back to Session mode (a sign-in) when the page has no key, and switching between them keeps the client and its keys. Its `clientRights` default to the creator's and can be no wider than the creator's rights (403 `outside_rights`, nothing created or changed); the check runs whenever a save sets them to something other than what is stored, so a panel an administrator widened still saves. Its scopes start as `names`, `tally`, `protected`, `status`, `presets`, `layouts`, `take` and `panels`, so its key does not read History. Only the Advanced Settings PIN widens it, in Manage users, where it shows on the API clients tab with its panel and does not count toward the 12-client limit. Switching the panel to `session` or deleting it removes the client and ends its keys and streams; switching between `api` and `both` keeps them. In History its actions read "Seth (Panel p-1a2b3c4d)", the user who made it and the panel client's name (`actor.owner` has the user; `actor.name` and `actor.key` stay the client and its key name), and a user's key named "Panel 21" reads "Seth (Panel 21)". Changing a user's Rights lists the panel clients they created, in the Manage users list (`panelClients`, with `wider` when the client now exceeds the user) and in the `rightsEdit` row.

A panel's client holds one key per named instance, at most 20 instances (keys and pending setup links together; 409 `conflict` past that). The name is the instance's device name, unique on the panel ignoring case, at most 40 characters. Making a link for a name that already holds a key rotates that instance only: its old key and streams end (`rotated: true`) and the other instances keep theirs. `DELETE` with `?instance=` revokes one instance and leaves the panel, the client and the other keys. Open the `link` and the page uses the key as its Bearer key with no sign-in, and takes as `Panel <id>` with the client's Rights. The fragment is never sent to Sode, so the key is in no request line or log row; a key in a query string is ignored. Sode's own links are shorter: `panels/<slug or id>#key=<secret without its sode_ prefix>`, with an optional `?instance=<instance name>` before the fragment that names the instance so the page skips asking. A key made for a panel is 128 random bits (22 characters after the prefix), and so is a setup token; keys made earlier keep their length and still work, with or without the prefix in the fragment. A panel with `settings.kiosk` set, or a link ending `&kiosk=1`, hides Sode's header. `GET /panels/{id}` and the list give an editor `hasKey` (any live key; a pending setup link does not count) and `clientNetworks`; the secret is never read back.

Secure mode: `POST /panels/{id}/key` with `{"instance": "<name>", "setup": true}` ends that instance's key and any older setup link for the name, and returns `setup` (a token), `expires` (one hour on), `instance` and `link` (`panels/<id>?instance=<name>#setup=<token>`). The panel page posts the token to `POST /panels/{id}/setup` (body `{"setup": "<token>"}`) and gets `{key, id, instance}` once; the key carries the instance's name. It keeps the key in the browser, drops `#setup=` from the address and runs as the client. The token works once; a used, expired or replaced one is 401 `invalid_setup`. Wrong tokens count against the same ten-per-minute address limit as wrong keys. A new key, a revoke for the same instance or deleting the panel also ends a pending setup link, and a setup token is never accepted as a Bearer key.

Allowed networks: `PUT /panels/{id}/networks` replaces a list kept on the panel's API client (`networks` in the config). Each entry is a CIDR (`192.168.1.0/24`, `2001:db8::/32`) or a bare address for one host (`/32` or `/128`); entries are stored masked, at most 16, and `[]` lifts the limit. A request with the client's key from an address outside the list is 403 `network_not_allowed` before anything else runs, on every route, and is not counted as a failed key. The address is the same one the rate limit and the log use (`X-Forwarded-For` only through a trusted proxy). A setup exchange from outside is refused the same way and leaves the link usable. An open panel page whose key is refused stops polling and shows the message.

**Monitoring.** An open panel page is an instance, named by its device (asked once in the browser), and sends a keepalive every 15 seconds. The page also holds `GET /panels/{id}/instances/stream`. A device that held a stream and closed it is lost 8 seconds later, so a closed page is noticed quickly and a reload with the same name raises nothing. A page whose stream cannot stay open falls back to the keepalive timing. A panel is `online` while a device has a stream and was heard in the last 45 seconds, or lost its stream less than 8 seconds ago, or never held a stream and was heard in the last 45 seconds; an editor also sees `instances` (device, `seen`, `online`, and `uptime`: the device's `up` and `down` changes over the last 24 hours as Unix milliseconds, with `start` the state at `from`) for every device that has opened it since Sode started. Every reader gets `instancesOnline` and `instancesKnown`, counted for the caller: the devices heard since Sode started and the ones the caller watches by name, less the ones the caller hid. The panels page reads All Online, Some Online or All Offline from them. Devices live in memory, are told apart by name ignoring case, and at most 100 are kept per panel (409 `conflict`). A device silent for more than 45 seconds, or a panel that loses its last device, tells each watcher once, and a second time when the device returns. A watch is for the whole panel or, with `?instance=<device>` on the watch routes, for one device named ignoring case; a user may hold both. Someone who watches only a device is told only about it, and its notice never has `offline` true. Panel reads carry `watching` (the whole panel) and `watchingInstances` (lowercase device names), both the caller's own. Sode turns on TCP keepalive on every connection it accepts (2 s idle, 1 s apart, 3 misses), so a device that vanished without closing, such as a power cut or lost Wi-Fi, ends its stream within about 5 seconds and is lost 8 seconds after that. Behind a proxy that holds the connection to Sode open the probes only reach the proxy, so such a device falls back to the 45 second miss. The Sode page receives `panelNotice` (`state` `lost` or `back`, `panel`, `name`, `device`, `secs`, `offline` true when no device is left) and `panelsOffline` (the alarms that are on now: `panel`, `name`, `device`, `since`, and `instance` for a watched device), shows a toast, a banner at the top of the Sode page (a badge on the panels pages) and a browser notification if the person allowed them. The banner has Clear, which offers "Clear alarm for now" and "Stop monitoring this panel": the page sends `panelAlarm` (`panel`, `instance` for a watched device or empty, `action` `clear` or `stop`) on the socket. `clear` hides that outage from the sender's banner until the panel's next notice, and `stop` ends that watch like `DELETE /panels/{id}/watch`. The messages go only to a connection signed in as a watcher and not under the Lock, and `watching` is only ever the caller's own. A watcher who stops being the owner or a Trusted user is no longer told. Deleting a panel forgets its devices without a notice, and making, rotating or revoking an instance's key forgets that instance's device the same way. A panel client's keys share one rate limit that grows with the number of live devices, and its streams are capped at 64 per key rather than 8 per key and address. Nothing watches for a key used from a new place.

Panels need the API switch and use the same scopes, rate limit and errors as every other call. A panel the caller may not use is 404, never 403. Artwork is served with headers that stop a script in an SVG from running; show it as an image.

There are no settings, users, keys or lock endpoints.

## Errors

Every error is `{"error":{"code","message"}}`. A 401 carries `WWW-Authenticate: Bearer`, and a wrong method on a known path is 405 with `Allow`. 429 from the rate limit carries `Retry-After` in seconds; make no call to Sode before it has passed.

`key_revoked` and `key_expired` are final. Sode answers them for a key that worked once, so a client that gets one must stop sending that key: retrying cannot help. They do not count toward the ten bad keys per address, so a forgotten tablet or tab still sending an old key does not make every other page at its address wait. Sode remembers the last 1024 ended keys, in `keys-ended.json` beside the config, and knows a session key from Sode's own pages by its shape (`sode_ses.` and 43 characters), so those read as ended after a restart too. A session key is 256 random bits, so not counting a guess of one weakens nothing. An address that keeps sending ended keys gets 429 `rate_limited` past 120 in a minute.

| Status | Code | Cause | What to do |
| --- | --- | --- | --- |
| 503 | `api_off` | The API switch is off | Ask an administrator |
| 401 | `key_required` | No key, and Guest API is off | Send a key |
| 401 | `invalid_key` | The key is unknown or malformed | Check the key; ten of these from one address in a minute make it wait |
| 401 | `key_revoked` | The key worked once and was ended: revoked, rotated, its session ended, or its owner was removed or changed PIN | Stop using it and make a new key |
| 401 | `key_expired` | The key's expiry has passed | Stop using it and make a new key |
| 401 | `invalid_setup` | A setup link that was used, expired or replaced | Make a new setup link |
| 403 | `network_not_allowed` | A panel's key used from outside its allowed networks | Use the key from an allowed network, or change the list |
| 403 | `scope` | The key lacks the scope this path needs | Add the scope to the client |
| 403 | `outside_rights` | A route or preset is outside the key's rights; nothing was sent | Change the request or the rights |
| 403 | `https_required` | Require HTTPS is on and the request did not arrive securely | Use the HTTPS URL |
| 403 | `not_owner` | A panel the caller may only use | Save a copy, or ask the owner |
| 403 | `user_required` | Creating or copying a panel with a key that is not a user's | Use a user's key |
| 403 | `trusted_required` | Making a panel an API client, or changing its rights, or saving a template or checkpoint set to Everyone without the Advanced Settings PIN, as a user who is not Trusted | Ask a Trusted user |
| 400 | `bad_request` | A parameter or body is not valid | Fix the request |
| 413 | `too_large` | The body is over 64 KiB (256 KiB for a panel, 2 MB for artwork) | Send less |
| 415 | `unsupported_type` | Artwork that is not a JPEG, PNG, GIF or SVG | Send one of those |
| 500 | `internal` | A panel could not be written to disk | Check the disk and the logs |
| 404 | `not_found` | No such path, History entry or visible preset, or scope `me` on a client or Guest key | Check the id, name or scope |
| 405 | `method_not_allowed` | The path does not take this method | Use a method in `Allow` |
| 409 | `conflict` | Redo of a failed entry, or Retry of one that succeeded; a panel over its limits; a short name another panel has; a key for a panel that does not act as an API client; a panel already at 20 instances | Use the other verb, switch the panel to `api`, or revoke an instance |
| 409 | `name_taken` | An Assign whose target, or a rename whose panel, already has an instance of that name; `error.suggest` is a free one | Send again with that name, or another, as `name` |
| 422 | `nothing_to_do` | The plan has no route left to run: nothing to undo, a Redo of a route with no source, or a preset whose parts are all gone from the router | Nothing to do |
| 422 | `idempotency_mismatch` | The Idempotency-Key was used for a different request | Use a new key |
| 429 | `rate_limited` | Too many requests, or ten bad keys from this address in a minute | Wait `Retry-After` seconds |
| 429 | `take_cap` | The global cap on API takes in flight is reached | Retry in a second or two (no `Retry-After`) |
| 429 | `too_many_streams` | The key or the address already has 8 open streams (64 for a panel's own key) | Close one |

## Takes

A take request is its own Confirm: nothing asks a person, so guard the button on your side. The request waits and returns when the router has answered or timed out every route. It is always 200 once the take ran, even when a route failed, so read each result: `connected`, `already` or `failed`, with the History `entry` id. Refusals before anything is sent are errors.

The worst case for one route is about 35 seconds: check 1 s, send queue up to 30 s, route 3 s, grace 1 s. More than 8 routes run 8 at a time and take longer. Common proxies cut a request at 60 seconds, so raise the proxy's read timeout for `/api/v1`.

A route the router confirms after the take gave up is a Late Result. The response already carries the entry ids, and the stream sends an `entryUpdate` for each, so match updates by `id`.

**Idempotency-Key.** An optional header, 1 to 128 printable characters with no spaces. A repeat of the same request with the same key from the same API key within 10 minutes returns the first answer with `Idempotent-Replay: true`, so retrying after a timeout or a proxy cut does not take twice. The same key with a different request is 422 `idempotency_mismatch`. A request that did nothing (bad input, rate limit, take cap) is not remembered, except an `outside_rights` refusal, which a repeat with the same key replays for the same 10 minutes, even if the rights have widened since. A call without an API key (a keyless Guest) cannot send the header: guests behind one address would share answers, so it is 400 `bad_request`.

**Large takes.** A take of 6 or more routes skips the Before check, to spare the router, and the response carries `checkSkipped: true`. Each entry's Prev then comes from Sode's cache, so a later Undo may restore less accurately. Send fewer than 6 routes per request to keep the check.

**Undo, Redo and Retry** work on one entry at a time. To undo a Group, call `GET /history/groups/last`, then undo each returned entry. Single-route takes and External entries are never returned there.

## Stream

`GET /stream?topics=names,tally,status,history,results` upgrades to a WebSocket that only sends: anything the client writes is ignored. A browser page cannot set the Bearer header, so use a client library. Each topic needs the scope of the same read (`results` needs `history`); a topic outside the key's scopes is 403 `scope`, and an unknown or missing topic is 400.

Every frame is JSON `{"t","topic","data"}`, and every frame but `hello` and `ping` carries `topic`, so match on it alone. Changes use the topic they belong to: `names`, `tally`, `status`, and `history` for `entry` and `historyCleared`. `entryUpdate` has `history` when you chose it, otherwise `results`. The first frame is `hello`, then one `snapshot` each for `names`, `tally` and `status`, with `data` shaped like the REST read. A `history` subscriber gets no snapshot: call `GET /history` first and bridge by id. `results` sends only `entryUpdate`, which suits a panel that fires takes and wants nothing but Late Results.

```json
{"t":"hello","data":{"v":1,"topics":["tally","results"],"numberOffset":1}}
{"t":"snapshot","topic":"tally","data":{"levels":[{"level":1,"label":"1"}],"routes":[{"dst":0,"level":1,"src":3}]}}
{"t":"tally","topic":"tally","data":{"routes":[{"dst":0,"level":1,"src":1}]}}
{"t":"entryUpdate","topic":"results","data":{"id":412,"time":"2026-11-08T09:31:02Z","client":"PanelsBot (Panel 43)","actor":{"kind":"client","name":"PanelsBot","key":"Panel 43"},"src":1,"srcName":"CAM 2","dst":0,"dstName":"MON A","levels":[1],"prev":[[1,3]],"ok":true,"late":[1]}}
```

Changes follow as `names`, `tally` (only changed routes), `status` (the same fields as `GET /status`), `entry`, `entryUpdate` (the whole Entry again) and `historyCleared`. `ping` is a heartbeat every 5 seconds, ignore it.

A rights or scope change applies to an open stream at once, to later frames only: a client keeps what it already holds until it reconnects. A stream whose key lost a topic's scope, or whose API switch went off, is closed. A key or an address can hold 8 streams at a time, and a ninth gets 429 `too_many_streams`. A panel's own API client key is the exception: its kiosks share one key, so it may hold 64 and its streams do not count toward an address's 8. A client that stops reading is dropped when a frame write stalls for 5 seconds or 256 frames are queued. Connects and disconnects are `api` rows with the key name, and a disconnect row says why.

## Require HTTPS

Settings > Proxy > Require HTTPS (ADR 0005) refuses every request except `/healthz` and `/api/health` unless it arrives from a trusted proxy with `X-Forwarded-Proto: https`. API paths answer 403 `https_required`. It needs `trustedProxies` set, and a save that turns it on must itself arrive over HTTPS. See [reference.md](reference.md#behind-a-proxy) for the recovery flag.

## The Communication Log

Every API command, refusal, key made, revoked or expired, rights edit, stream connect and disconnect, and the API and Require HTTPS switches write an `api` row. Failed keys write one row per address per minute with a count (`authFailed`), and so do calls with a key that ended (`endedKeyUsed`). No key, key hash or PIN is ever written. A row a panel page caused, including a lost or returned instance, names the panel by its short name (else its id) and the instance in its note.
