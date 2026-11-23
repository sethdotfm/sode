DISCLOSURE: This was written by an LLM.

# Persisted and wire names

Audit of every name that is stored or sent and still carries a word from a `_Avoid_` line in `GLOSSARY.md`. Names in backticks are exact. The prose around them uses the glossary's words.

Scope: config keys (`config.yaml` and the `SODE_*` env that seeds it), history and Communication Log file fields, and `localStorage` keys. The scope also has WebSocket message types and the command kinds that the server logs. Names that only look close are listed apart at the end.

This list starts over for v2. v2 reads no v1 files, so it has no legacy names that are only read. On start, Sode moves a v1 install to `/data/v1/`, and the first v2 page load clears the v1 `sode-*` keys (`docs/agents/server.md`, config load).

Calls. Keep means the name stays for now and `AGENTS.md` records why. v2 renamed every other v1 name that carried a retired word, and the table "Names v2 renamed" lists them.

## Router references

Every stored source and destination is a router reference `{router, n, name}`: the router UID, Sode's number (0-based) and the router's name at save time. `shown` is never stored. Presets, Rights, panel cells, Grid edges, `fixedDests`, templates, checkpoints, Checkpoint sets and Default artwork hold one each. History and Communication Log rows hold `router` once per row, and `names/<router uid>.yaml` once per file. No store holds a bare number, and no loader reads one. A file reference with no router is refused (config.yaml) or skipped with a problem (the other stores). Only a request can send `{n}` with no router, which names the one router. Levels are UI levels 1..N; a preset item's levels belong to its source's router. Rights `levels` and a panel's `settings.levels` are lists under the router UID (`{"<uid>": [1, 2]}`), and a cell's `levels` belong to the cell's `router`. A file with a plain level list there is refused or skipped as a reference with no router is.

## Format markers

- `format` (config key, first in `config.yaml`): the format version, now `2`. A file without it is from v1. Sode refuses a higher number.
- `sode-format` (`localStorage` key, `2`): written by `web/prepaint.js` after it clears the `sode-*` keys of a v1 browser.

## Kept names

None. v2 removed the v1 settings secret and its names (its config key, its WebSocket messages and its `why` value) with Roles (ADR 0010).

These names refer to the router's own salvo, so they are correct and not part of this audit: `fireMode` (`native`), `salvoGroup`, `SODE_SALVO_GROUP`, `SIM_NO_SALVO` and `SIM_SALVO_ANNOUNCE`. The `kind` values `salvoStage`, `salvoStageExt` and `salvoGo` are also the router's.

## Names v2 renamed

v2 loads no v1 files, and a page whose build label differs from the hello's reloads itself. So these renames need no path that reads the old name.

| v1 name | v2 name | Where |
| --- | --- | --- |
| `batch` | `take` | WebSocket, browser to server |
| `recent` | `entry` | WebSocket, server to browser |
| `recent` field of `hello` | `history` | WebSocket |
| `refreshNames` | `rereadNames` | WebSocket, and the `why` value in Communication Log rows |
| `sent`, `sentAdd`, `sentPage` | `log`, `logAdd`, `logPage` | WebSocket |
| `userUnlock`, `userUnlockResult` | `userSignIn`, `userSignInResult` | WebSocket, and the `why` value in Communication Log rows |
| `protect`, and the `protect` field of `hello` | `protected` | WebSocket |
| `nameRefreshSeconds` | `nameRereadSeconds` | Settings message key (already the `config.yaml` key) |
| `SODE_NAME_REFRESH_S` | `SODE_NAME_REREAD_S` | env |
| `deviceDefaults` | `browserDefaults` | config key |
| `SODE_SENTLOG`, `SODE_SENTLOG_MB` | `SODE_COMMLOG`, `SODE_COMMLOG_MB` | env |
| `SIM_PANEL_DST`, `SIM_PANEL_SECONDS`, `SIM_PANEL_SILENT` | `SIM_EXTERNAL_DST`, `SIM_EXTERNAL_SECONDS`, `SIM_EXTERNAL_SILENT` | simulator env |
| `SIM_FOLLOWS` | `SIM_EXTERNAL_FOLLOW` | simulator env |
| `sode-device-defaults` | `sode-browser-defaults` | `localStorage` |

## Hello fields v2 removed

The hello's `routers` list carries the names, tally, levels and link of every router. So these fields of the first router left the hello (#812). The page reads them from the first element of `routers`.

| Removed field of `hello` | Where it is now |
| --- | --- |
| `srcs`, `dsts`, `at`, `savedAt` | `srcs`, `dsts`, `at`, `savedAt` of each router |
| `tally` | `tally` of each router |
| `status` | `status` of each router |
| `levels` | `levels` of each router |
| `prefix` | `prefix` of each router |
| `routerUid` | `uid` of each router: its UID in the settings |

The first router's Protects stay in `protected` and `protectBy`: the page keeps no Protect state of the other routers.

## Close names that are not retired

These carry no word from an `_Avoid_` line, so nothing is owed.

- `follow` and `followGroup` (history fields). The names hold no retired word. They point at the take that an External follow seems to trail, which is only ever a suspicion.
- `levelWireOffset` (Settings message key; `levelOffset` in `config.yaml`). It holds the offset between a Level and the router's own number on the wire. The Settings label says level offset.
- `wire` (Communication Log field). The router's own level number, kept apart from the UI Level on purpose.
- `locks`, `autoLockMinutes`, `lockState`. The Lock and Auto-lock.
- The `user` field of `userSignIn` and the hello's `users` ({name, len}). The name that signs in and the sign-in list. New names, no retired word.
- A user's `id` (config, `usersListResult`, the users save's rows), `tokens.json` `user` and a user key's `owner.user` in keys.json: the hidden user id.
- `activity` (browser to server, no fields). An open page sends it about every 30 seconds while someone uses it. It moves the session's idle clock for Auto-lock (`Activity` in `internal/app/idle.go`).

## Panel names (new in Panels, no retired word)

- Directory `panels/` beside the config: `<id>.yaml` per panel (id `p-` and 8 hex digits) and `<id>.art/<art id>` for artwork (`a-` and 8 hex digits). A file with a name and id that differ is skipped.
- Panel fields (YAML and JSON): `id`, `name`, `owner`, `acts`, `client`, `share`, `rows`, `cols`, `pages`, `settings` and `art`. `acts` is `session`, `api`, `both` or `public`. `share` has `mode` (`me`, `named` or `everyone`) and `users`. `owner` is a user id in the file and the live name in JSON. `canEdit` is output only. Fields not listed are kept as they are.
- Grid panels (ADR 0011): the panel fields `type` (`buttons` or `grid`, omitted or empty reads as `buttons`), `sources` and `destinations`. A Grid panel has no `rows`, `cols`, `cells` or `pages`. Each of `sources` and `destinations` is an edge:
  - `members`: the string `all`, or a list of router references in order.
  - `hide`: router references left out of an `all` edge. It is refused on a list.
  - `items`: a list of router references, each with `pinned` (any number for an edge), `art`, `artB` and `hideText`, written as on a cell.
  Checkpoint and Push layouts carry the same three fields. A layout of the other type is refused.
- Pages (ADR 0007): `pages` is a list of 1 to 16 Pages in order. Each Page has these fields:
  - `id`: 1 to 16 letters, digits, dashes and underscores. Sode makes 8 hex digits, and a Page sent without one gets one.
  - `name`: at most 40 characters, omitted when empty.
  - `enabled`: true when left out.
  - `cells`.

  A Page's number is its place in the list; its id never changes. `cells` at the top of a panel, template, checkpoint layout or set entry is the one grid from before Pages. It loads as one enabled Page whose id is the record id's 8 hex digits, and the next save writes only `pages`. Sode does not mirror `cells`, so an older Sode reads a panel saved with Pages as an empty grid. That older Sode keeps `pages` as an unknown field. A save from that older Sode writes its own `cells` beside them, and this Sode then loads the `pages` and ignores those `cells`. A body that sends `cells` without `pages` changes Page 1 only (a new panel gets one Page). Sode refuses a body with both.
- History entries: `panel` and `instance` name the panel page that made a take. `panel` is the panel's name at the time of the take, else its short name, and `instance` is the instance. `panelId` holds the panel's id, which the Undo cell matches on. Older entries have none of them, so their takes are never an Undo cell's to undo.
- Cell fields inside each Page's `cells` (stored by the panel page and the editor):
  - `r` and `c` (from 1), `rs`, `cs`.
  - `kind`: `source`, `destination`, `take`, `preset`, `clear`, `undo`, `current`, `panelName`, `instanceName`, `pageName`, `info`, `goToPage`, `nextPage` or `previousPage`.
  - `router`, `n` and `name` on a `source` or `destination` cell: its router reference. A cell with no `n` is unassigned.
  - `scope` (`everyone` or `me`), `id` (the preset's hidden id) and `name` on a `preset` cell.
  - `page` on a `goToPage` cell: its target Page's id. A missing Page is kept and reads as unassigned.
  - `show` on a `pageName` cell: a list of `number` and `name`; without it, `name`.
  - `show` on an `info` cell: a list of `panel`, `number`, `page` and `instance`; without it, all but `number`.
  - `caption`, `levels` (UI levels).

  Panel `settings` read so far:
  - `mode` (`take`, `instant` or `takeOnly`), `multi`, `levels`, `guard` (`hold` or `double`) and `resultText`.
  - `takeOnly` is Take mode with no confirm box, on a Grid panel. A Buttons panel reads it as `take`.
  - The server refuses a save with any other `mode` (400). A missing or empty `mode` reads as `take`. A file on disk loads as written.
  - `wrapPages`: wrap between Pages, stored only as `false`.
  - `askInstanceName`: Ask for an instance name, a boolean. A file without it reads as on, and the API sends it as `true`. So the editor stores it as chosen, `true` included.
  - `fixedDests` (Fixed destination): a list of router references `{router, n, name}` in the order chosen. `n` is a whole number from 0 to 65535, unique, at most 16. A save can leave `router` out (`{n}`); Sode stores references. A bare number is refused. It is removed when empty, never stored as `[]`. A file without it loads unchanged. Push leaves each target's own list, and a template carries it only with `dests`.

  The editor's sidebar writes only these and the look names below. It drops a setting that equals its default (`multi`, `titleBar` and `wrapPages` are on, `guard` is none, `fontSize` is 100). Cell `levels` is removed, never stored as `[]`, for every level. The editor sends `clientRights` only when the Access tab edited it. The page sends `Idempotency-Key` values that start with `panel-`.
- Panel field `rev` (in the file, in the API and as the page's poll signal) is the server's count of stored changes. It is omitted from the file while 0. A body's `rev` is ignored and a copy starts over.
- A panel's API client is an `apiClients` entry in the config with a `panel` field (the panel id), named `Panel <id>`. `clientRights` is the panel's request and response field for its Rights; it is never stored in the panel file. The Manage users list sends `panelClients` per user.
- Checkpoints: `panels/<id>.checkpoints`, a YAML list of at most 10. Each has `id` (`c-` and 8 hex digits), `at`, `by`, `why` and a `layout` of `rows`, `cols`, `pages`, `settings` and `extra`. An older one has `cells`. Checkpoint sets: `panelsets/everyone.yaml` and `panelsets/me-<hex of the user id>.yaml`. Each has `sets:` with `id` (`s-` and 8 hex digits), `name`, `scope` (`me` or `everyone`), `owner` and `entries`. Each entry has `panel`, `name` and the same layout fields. `api` row kinds `panelPush`, `panelRestore`, `panelSetCreate`, `panelSetUpdate`, `panelSetDelete` and `panelSetPush`. An Everyone set also accepts the body field `pin` (never stored), as a template does.
- A panel's own API client starts without the `history` scope; a client saved earlier keeps the scopes it has.
- API scopes `panels` and `panelsmanage`, and API error codes `not_owner`, `user_required`, `trusted_required`, `unsupported_type` and `internal`. `api` row kinds `panelCreate`, `panelUpdate`, `panelDelete`, `panelCopy`, `panelKey`, `panelKeyRotate`, `panelKeyRevoke`, `assign`, `rename`, `reload` and `publicBack`. An Assign's request body has `from`, `instance`, `keyless`, `name`, `as` (`client` or `user`) and `pin`. A rename's body has `instance`, `keyless`, `name` and `pin`. `POST /panels/{id}/instances/reload` takes `instance`, `keyless` or `all`, answers `panel` and `sent`, and refuses with the code `offline`. Both streams carry the reload line `t` `reload`; the keyless announcement also carries `t` `public`, which does not end it. `POST /panels/{id}/public` takes `back=1`. The `sessionStorage` key `sode-panel-back` holds the panel address segment of a page that reloads after a `public` line; it is no key. A name clash answers the error code `name_taken` with `error.suggest`. The instance stream's Assign line has `t` `assign`, `panel`, `slug`, `instance`, and `key` or `signIn` (`user`). Its rename line has `t` `rename` and `instance`. Panel `instances` rows carry `stream`, `page`, `addr`, `signIn`, `user`, `trusted` and `streamSince`. The keyless announcement is `GET /panels/{id}/instances/keyless` with `instance` and `ended`. Its beat line is `{"keyless":true}`, and an Assign names one with `keyless`. A panel read's `keyless` rows carry `keyless`, `device`, `addr`, `signIn` (`none` or `ended`), `stream`, `seen` and `streamSince`. Nothing of it is stored.
- `networks` on an `apiClients` entry (a panel's client only, set through `PUT /api/v1/panels/{id}/networks`): a list of canonical CIDR strings, such as `192.0.2.0/24` or `2001:db8::/32`. A single host is `/32` or `/128`. Empty or absent allows any address. Panel output fields `hasKey` and `clientNetworks` are never stored. Error codes `invalid_setup` and `network_not_allowed`, and `api` row kinds `panelSetupLink`, `panelSetup` and `panelNetworks`.
- Short name: the panel field `slug` (`panels/<id>.yaml`, JSON `slug`, omitted when empty). It has 1 to 32 lowercase letters, digits and dashes, is unique across panels, and is never `editor` or id-shaped. A file whose `slug` is invalid or already held by an earlier file loads without it and is listed in the panel problems. `/panels/<slug>` serves the panel page, and `GET /panels/<slug>` and `POST /panels/<slug>/setup` resolve it. Links: `?instance=<instance name>` on the panel page seeds the device name (never stored; `sode-panel-device` is still the stored one). A fragment `key=` or `setup=` can leave off its `sode_` or `sode_setup_` prefix. New panel keys and setup tokens are 16 random bytes (22 characters); older keys keep 43 and are read the same way.
- A setup link is a `keys.json` record with `"setup": true`, owned by the panel's client and named for its instance. It shows in no Settings key list, only in the panel's own instance list. Older records named `Setup link` expire within the hour. Its token starts `sode_setup_`. The panel page stores the key it gets in `localStorage` as `sode-panel-key-<panel id>` and forgets it on a 401. A plain `#key=` link's key goes under the same name when the panel opens with it, and leaves the address. It persists in that browser until the instance is rotated or revoked. Opened by short name, the page keys it by what the address holds, so `sode-panel-key-<slug>`, and also forgets it on a 404.
- Public panel: route `POST /panels/{id}/public` (also by short name, no body, no key). It answers `200` with `public` `false`, or `201` with `public` `true`, `key`, `expires` and `panel` (the panel id). The key starts `sode_pub.` and lives in memory only, never in `keys.json`; the page holds it in memory and stores nothing.
- Monitoring: `panelwatch.yaml` beside the config. It is a map of lowercased user name to watch keys, never part of a panel file. A key is a panel id for the whole panel, or `<panel id>/<device name lowercased>` for one instance. So a file from before instance watches loads unchanged.
  - Panel read fields `online`, `watching`, `watchingInstances`, `hiddenInstances`, `instancesOnline`, `instancesKnown` and `instances` are output only. `instances` has `device`, `seen`, `online` and `uptime`. `uptime` has `from`, `to`, `start` and `trans` of `t` and `s`.
  - Route `POST` and `DELETE /panels/{id}/instances/hide` (query `instance`, memory only).
  - Route `POST /panels/{id}/instances` (body `device`). A blank `device` is a 400 while `askInstanceName` is on. Otherwise a blank one names the instance by the caller's address.
  - Route `GET /panels/{id}/instances/stream` (query `device`, the same). It answers `online` and `graceSeconds` lines.
  - Route `POST` and `DELETE /panels/{id}/watch` (optional query `instance`).
  - Socket message `panelNotice` (`state` `lost` or `back`, `panel`, `name`, `device`, `secs`, `offline`).
  - Socket message `panelsOffline` (`offline`: `panel`, `name`, `device`, `since`, `instance` only for a watched device).
  - Socket message `panelAlarm` from the browser (`panel`, `instance`, `action` `clear` or `stop`). `clear` is kept in memory only and lasts until the panel's next notice. `stop` removes that watch.
  - The server messages go only to a connection signed in as a watcher and not under the Lock, and never in the hello.
  - `localStorage` key `sode-panel-device` (the instance's name).
  - `api` row kinds `panelLost` and `panelBack`, and a `command` row `panelAlarm`.
  - The `Actor` field `owner`, set only for a panel's own API client.
- `localStorage` key `sode-tieline-dismissed` (a JSON list of the two ends of each dismissed tie line suggestion, `router/n/router/n`).
- `localStorage` key `sode-panel-full` (`1` when the panel page runs at Full width).
- `localStorage` key `sode-panel-page-<panel id>`: the id of the Page the panel page last showed in that browser, read on open. A Page that is gone or disabled reads as the first enabled Page.
- Session key: socket messages `sessionKey` (browser to server, optional key name) and `sessionKeyResult`. Also the socket message `myRights` (server to browser, the signed-in user's own Rights, never in the hello). A session key is a `keys.json` record with `sess` (sha256 of the session token), so it ends with the session. Session mode panels ask for it named `Panel <id>`.
- A panel's keys are key-store entries owned by its client, one per named instance. The key name is the instance's device name (unique on the panel ignoring case, at most 40 characters). A panel holds at most 20 keys and pending setup links (`panelKeyMax`). A panel made before instances had one key named `Link`. It stays and is listed as the instance "Link", which rotates or revokes like any other. The `?instance=<name>` of a panel link is the same name. The `api` row detail for the key rows now ends with the instance name. History shows an action made with it as "Seth (Panel <id>)": the user who created it and the client's name. The verified `actor` is still the client with the instance's key name. A Session mode panel asks for its session key named `Panel <id>`, so it reads the same way (opened by short name, `Panel <slug>`). The link is `panels/<id>#key=<secret>`. The page also reads `kiosk=1` in the fragment and panel `settings.kiosk` (true hides the header).

- Default artwork: `artdefaults.yaml` beside the config, with `rev`, `defaults` and `images` (`id`, `type`, `size`). Each item in `defaults` has these fields:
  - `kind` (`source`, `destination`, `preset`).
  - `router` and `n` for a Source or Destination, `id` and `scope` for a Preset.
  - `name`: the preset's, or the router name when a Source or Destination default was set.
  - `art`, `artB`, `hideText`.

  Images sit in `artdefaults.art/<id>`, id `d-` and the first 8 hex digits of the content's SHA-256. Sode deletes an image that no default names on the next write. A default's `art` can be `lib:<id>`, stored as the reference with no image, so it draws nothing when the Icon library lacks that icon. Routes `/api/v1/default-artwork` and `/api/v1/default-artwork/images/{art}`, body fields `panel` and `pin` (never stored), the panel read's output field `artDefaults`, and `api` row kinds `artDefaultSet` and `artDefaultClear`.
- Icon library: `icons.yaml` beside the config, with `rev` and `icons` (each `id`, 8 random hex digits, `name`, `type` (`image/png` or `image/svg+xml`) and `size`). Files sit in `icons/<id>.png` or `icons/<id>.svg`, and `icons/drop-in/` accepts files to add. Routes `/api/v1/icon-library`, `/api/v1/icon-library/rescan`, `/api/v1/icon-library/{icon}` and `/api/v1/icon-library/{icon}/uses`. Upload and rename body fields `name`, `data` (base64) and `pin` (never stored). The `api` row kinds `iconAdd`, `iconRename`, `iconDelete` and `iconRescan`. A rescan answers `added` and `skipped` (each `file` and `reason`).
- Directory `templates/` beside the config: `me/<id>.yaml` and `everyone/<id>.yaml` per template (id `t-` and 8 hex digits), apart from the panel files. A file whose id or directory differs from its content is skipped. Template fields: `id`, `name`, `scope` (`me` or `everyone`), `owner`, `sources`, `dests`, `rows`, `cols`, `pages` and `settings`. With `sources`, Source and Preset cells keep their contents and captions; with `dests`, Destination cells do. `contents` is the single flag from before the split. A file or save that sets it loads as both `sources` and `dests`, and it is never written again. Routes `/api/v1/panel-templates`, body field `pin` (never stored), and `api` row kinds `templateCreate` and `templateDelete`.
- Panel look (stored in `settings` and `cells`):
  - `settings.titleBar`, `settings.fontSize`.
  - `settings.theme` (`light` or `dark`; missing means the browser's setting).
  - `settings.backdrop`: read-only legacy. It is no longer shown or used, so an old file still loads and its uploaded image stays in `art`.
  - `settings.wake`.
  - `settings.haptics`: read-only legacy. It is ignored, so an old file still loads.
  - `settings.fullscreen`, `settings.landscape`.
  - Cell `art`, `artB`, `fontSize` and `hideText`.

  `hideText` is true to hide the text under artwork, false only to show it against the item's Default artwork, and removed to follow the default. Level subsets show as lit level buttons in the editor (the main page's picker), and the stored `levels` stay as before (removed for every level). `art` holds `icon:<name>`, `lib:<id>` (an Icon library icon; an id the library does not hold saves and draws nothing) or an artwork id. `artB` is Icon B, one of `0` to `16` or `A` to `Z`, drawn beside Icon A. It is removed when cleared, and a cell without it loads as before.

## Enforcement

`internal/app/vocabulary_test.go` keeps the lists below. Each entry names a real kept name, and `TestAllowlistsMatchSomething` fails on an entry that exempts no line.

- `vocabAllow` exempts lines of docs, UI strings and web code that need a retired word. The `web/settingsindex.js` lines are search aliases, kept so that a search for the old word finds the setting.
- `identAllow` exempts web identifiers only. Its `unlock` entries are the `adminUnlock` command kind of old rows in `web/logkinds.js` and the `unlocked` sign-in state and bus event in `web/lock.js`.
