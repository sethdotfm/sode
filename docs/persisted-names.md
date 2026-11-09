DISCLOSURE: This was written by an LLM.

# Persisted and wire names

Audit of every name that is stored or sent and still carries a word from a `_Avoid_` line in `GLOSSARY.md`. Names in backticks are exact. The prose around them uses the glossary's words.

Scope: config keys (`config.yaml` and the `SODE_*` env that seeds it), history and Communication Log file fields, `localStorage` keys, WebSocket message types, and the command kinds the server logs. Names that only look close are listed apart at the end.

Calls. Keep means the name stays and `AGENTS.md` records why. Legacy means an old name that is only read, never written, so old files load (as `levelTypedBase` and the `grids:` keys do). No rename is approved, so every call below is keep or legacy.

## Config keys

| Name | Retired word | Call | Why |
| --- | --- | --- | --- |
| `settingsPin` | Settings PIN | Keep | Holds the Advanced Settings PIN hash. A rename needs a fallback read, a fixture file, and the `-reset-pin` and import paths changed, and it buys nothing a user sees. |
| `nameRefreshSeconds`, `SODE_NAME_REFRESH_S` | refresh | Keep | The Settings label already says re-read. Old `config.yaml` files and deploy env hold the key. |
| `salvos` | salvo (for presets) | Legacy | Pre-1.5 key. `Normalize` moves it into `presets`. Never written. |
| `deviceDefaults` | This device | Keep | Only the phrase "This device" is retired, and the key holds the defaults This browser falls back on. Listed because the word is close. |

The names `fireMode` (`native`), `salvoGroup` and `SODE_SALVO_GROUP` refer to the router's own salvo, so they are correct and not part of this audit.

## Simulator env

| Name | Retired word | Call | Why |
| --- | --- | --- | --- |
| `SIM_PANEL_DST`, `SIM_PANEL_SECONDS`, `SIM_PANEL_SILENT` | panel (change) | Keep | Simulator settings only, set in compose files and scripts. They make an External. |
| `SIM_FOLLOWS` | follows | Keep | Simulator setting that makes External follow cases. |
| `SIM_NO_SALVO`, `SIM_SALVO_ANNOUNCE` | none | n/a | The router's own salvo. |

## Communication Log env

| Name | Retired word | Call | Why |
| --- | --- | --- | --- |
| `SODE_SENTLOG`, `SODE_SENTLOG_MB` | sent log | Keep | Process env in deployed compose files. The file it names holds Communication Log rows. |

## History and Communication Log files

| Name | Retired word | Call | Why |
| --- | --- | --- | --- |
| `salvo` (history entry field) | salvo (for presets) | Legacy | Old files call the preset name `salvo`. `Entry.UnmarshalJSON` loads it into `preset`. Never written. |
| `why` values `refreshNames`, `unlock`, `userUnlock` (Communication Log event rows) | refresh, unlock | Keep | Stored in rows already written. The page shows them through the command-kind labels in `web/log.js`. |
| `kind` values `salvoStage`, `salvoStageExt`, `salvoGo` | none | n/a | Messages to the router's own salvo. |

## `localStorage` keys

| Name | Retired word | Call | Why |
| --- | --- | --- | --- |
| `sode-salvos`, `sode-salvo-pins` | salvo (for presets) | Legacy | `migrateLegacyKeys` in `web/presetstore.js` copies them to `sode-presets` and `sode-preset-pins` once and removes them. |
| `sode-lock-token`, `sode-lock-never` | none | n/a | Not retired. `web/lock.js` deletes them on load, as leftovers of a removed sign-in. Listed so nobody reads the delete as a miss. |
| `sode-device-defaults` | This device | Keep | Same reason as `deviceDefaults`. `web/prepaint.js` reads it before first paint, so a rename needs both pages changed together. |

## WebSocket types and fields

A browser can keep an old page open across a server upgrade, so a renamed type would break it until a reload. These keep their names.

| Name | Retired word | Call | Why |
| --- | --- | --- | --- |
| `batch` (browser to server) | batch | Keep | Carries every take of one confirm. |
| `recent` (server to browser, and the `recent` field of `hello`) | Recent | Keep | A new History entry or an update to one. |
| `refreshNames` (browser to server) | refresh | Keep | Asks for a router name re-read. |
| `sent` (browser to server), `sentAdd`, `sentPage` | sent log | Keep | The Communication Log feed and its pages. |
| `unlock`, `unlockResult` | unlock | Keep | Entering the Advanced Settings PIN. |
| `userUnlock`, `userUnlockResult` | unlock | Keep | A user signing in from the lock screen. |
| `protect` (server to browser, and the `protect` field of `hello`) | protect (as a noun) | Keep | The set of Protected destinations. |
| `locked` (server to browser) | locked | Keep | Says the Advanced Settings PIN is needed. It is not about a destination, so the glossary's use of the word does not apply. |

## Close names that are not retired

These carry no word from an `_Avoid_` line, so nothing is owed.

- `follow` and `followGroup` (history fields). The names hold no retired word. They point at the take that an External follow seems to trail, which is only ever a suspicion.
- `levelWireOffset`. It holds the offset between a Level and the router's own number on the wire. The Settings label says level offset.
- `wire` (Communication Log field). The router's own level number, kept apart from the UI Level on purpose.
- `locks`, `autoLockMinutes`, `lockState`. The Lock and Auto-lock.
- `signInMode` (`pin` or `username`, also in `lockState` as `mode` and in `usersListResult`), a user's `username`, and the `user` field of `userUnlock`. The Sign-in mode, the username and the name a username signs in with. New names, no retired word.

## Panel names (new in Panels, no retired word)

- Directory `panels/` beside the config: `<id>.yaml` per panel (id `p-` and 8 hex digits) and `<id>.art/<art id>` for artwork (`a-` and 8 hex digits). A file with a name and id that differ is skipped.
- Panel fields (YAML and JSON): `id`, `name`, `owner`, `acts` (`session` or `api`), `client`, `share` (`mode` `me`, `named` or `everyone`, and `users`), `rows`, `cols`, `cells`, `settings`, `art`. `canEdit` is output only. Fields not listed are kept as they are.
- Cell fields inside `cells` (stored by the panel page and the editor): `r` and `c` (from 1), `rs`, `cs`, `kind` (`source`, `destination`, `take`, `preset`, `clear`, `current`), `n` (the router's number), `scope` (`everyone` or `me`) and `name` on a `preset` cell, `caption`, `levels` (UI levels). Panel `settings` read so far: `mode` (`take` or `instant`), `multi`, `levels`, `guard` (`hold` or `double`) and `resultText`. The editor's sidebar writes only these and the look names below, and drops a setting that equals its default (`multi` and `titleBar` are on, `guard` is none, `fontSize` is 100). Cell `levels` is removed, never stored as `[]`, for every level. The editor sends `clientRights` only when the Access tab edited it. The page sends `Idempotency-Key` values that start with `panel-`.
- Panel field `rev` (in the file, in the API and as the page's poll signal): the server's count of stored changes, omitted from the file while 0. A body's `rev` is ignored and a copy starts over.
- A panel's API client is an `apiClients` entry in the config with a `panel` field (the panel id), named `Panel <id>`. `clientRights` is the panel's request and response field for its Rights; it is never stored in the panel file. The Manage users list sends `panelClients` per user.
- Checkpoints: `panels/<id>.checkpoints` (YAML list, at most 10, each `id` `c-` and 8 hex digits, `at`, `by`, `why` and a `layout` of `rows`, `cols`, `cells`, `settings`, `extra`). Checkpoint sets: `panelsets/everyone.yaml` and `panelsets/me-<hex of the lowercased user name>.yaml`, each `sets:` with `id` (`s-` and 8 hex digits), `name`, `scope` (`me` or `everyone`), `owner` and `entries` (`panel`, `name` and the same layout fields). `api` row kinds `panelPush`, `panelRestore`, `panelSetCreate`, `panelSetUpdate`, `panelSetDelete` and `panelSetPush`. An Everyone set also takes the body field `pin` (never stored), as a template does.
- A panel's own API client starts without the `history` scope; a client saved earlier keeps the scopes it has.
- API scopes `panels` and `panelsmanage`, API error codes `not_owner`, `user_required`, `trusted_required`, `unsupported_type` and `internal`, and `api` row kinds `panelCreate`, `panelUpdate`, `panelDelete`, `panelCopy`, `panelKey`, `panelKeyRotate` and `panelKeyRevoke`.
- `networks` on an `apiClients` entry (a panel's client only, set through `PUT /api/v1/panels/{id}/networks`): a list of canonical CIDR strings, such as `192.0.2.0/24` or `2001:db8::/32`; a single host is `/32` or `/128`. Empty or absent allows any address. Panel output fields `hasKey` and `clientNetworks` are never stored. Error codes `invalid_setup` and `network_not_allowed`, and `api` row kinds `panelSetupLink`, `panelSetup` and `panelNetworks`.
- A setup link is a `keys.json` record with `"setup": true`, owned by the panel's client and named `Setup link` (it shows in no key list); its token starts `sode_setup_`. The panel page stores the key it gets in `localStorage` as `sode-panel-key-<panel id>` and forgets it on a 401.
- Monitoring: `panelwatch.yaml` beside the config (a map of lowercased user name to panel ids, never part of a panel file). Panel read fields `online`, `watching` and `instances` (`device`, `seen`, `online`) are output only. Routes `POST /panels/{id}/instances` (body `device`) and `POST` and `DELETE /panels/{id}/watch`. Socket messages `panelNotice` (`state` `lost` or `back`, `panel`, `name`, `device`, `secs`, `offline`) and `panelsOffline` (`offline`: `panel`, `name`, `device`, `since`), sent only to a connection signed in as a watcher and not under the Lock and never in the hello. `localStorage` key `sode-panel-device` (the screen's name). `api` row kinds `panelLost` and `panelBack`. The `Actor` field `owner`, set only for a panel's own API client.
- `localStorage` key `sode-panel-full` (`1` when the panel page runs at Full width).
- Session key: socket messages `sessionKey` (browser to server, optional key name) and `sessionKeyResult`, and `myRights` (server to browser, the signed-in user's own Rights, never in the hello). A session key is a `keys.json` record with `sess` (sha256 of the session token), so it ends with the session; Session mode panels ask for it named `Panel <id>`.
- A panel's key is a key-store entry owned by its client, named `Link`. History shows an action taken with it as "Seth (Panel <id>)", the creating user and the client's name; the verified `actor` is still the client with key `Link`. A Session mode panel asks for its session key named `Panel <id>`, so it reads the same way. The link is `panels/<id>#key=<secret>`; the page also reads `kiosk=1` in the fragment and panel `settings.kiosk` (true hides the header).

- Directory `templates/` beside the config: `me/<id>.yaml` and `everyone/<id>.yaml` per template (id `t-` and 8 hex digits), apart from the panel files. A file whose id or directory differs from its content is skipped. Template fields: `id`, `name`, `scope` (`me` or `everyone`), `owner`, `contents` (cells keep their Sources, Destinations, Presets and captions), `rows`, `cols`, `cells`, `settings`. Routes `/api/v1/panel-templates`, body field `pin` (never stored), and `api` row kinds `templateCreate` and `templateDelete`.
- Panel look (stored in `settings` and `cells`): `settings.titleBar`, `settings.fontSize`, `settings.backdrop`, `settings.wake`, `settings.haptics`; cell `art` and `fontSize`. `art` holds `icon:<name>` or an artwork id.

## Enforcement

`internal/app/vocabulary_test.go` keeps the lists below. Each entry names a real kept name, and `TestAllowlistsMatchSomething` fails on an entry that exempts no line.

- `vocabAllow` exempts lines of docs, UI strings and web code that need a retired word. The `web/settingsindex.js` lines are search aliases, kept so that a search for the old word finds the setting.
- `identAllow` exempts web identifiers only. Its entries are the names in the tables above (`m.recent`, the command kinds in `web/log.js`, and the `unlock` functions in `web/pin.js`).
