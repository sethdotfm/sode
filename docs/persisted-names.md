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
- A panel's API client is an `apiClients` entry in the config with a `panel` field (the panel id), named `Panel <id>`. `clientRights` is the panel's request and response field for its Rights; it is never stored in the panel file. The Manage users list sends `panelClients` per user.
- API scopes `panels` and `panelsmanage`, API error codes `not_owner`, `user_required`, `trusted_required`, `unsupported_type` and `internal`, and `api` row kinds `panelCreate`, `panelUpdate`, `panelDelete` and `panelCopy`.

## Enforcement

`internal/app/vocabulary_test.go` keeps the lists below. Each entry names a real kept name, and `TestAllowlistsMatchSomething` fails on an entry that exempts no line.

- `vocabAllow` exempts lines of docs, UI strings and web code that need a retired word. The `web/settingsindex.js` lines are search aliases, kept so that a search for the old word finds the setting.
- `identAllow` exempts web identifiers only. Its entries are the names in the tables above (`m.recent`, the command kinds in `web/log.js`, and the `unlock` functions in `web/pin.js`).
