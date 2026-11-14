DISCLOSURE: This was written by an LLM.

# Data inventory and dead-end prerequisites

Research for sethdotfm/sode#376, a child of the Settings, auth and config overhaul map (#375). Source: this repo at commit 85244ff. Every claim cites a file and line. A claim the code does not settle on its own is marked **(inferred)**.

Terms used here:

- Site-bound: the value means something only against one router (source, destination or level numbers, the router's address, the router's names) or one install (panel ids, user names, PIN hashes).
- Portable: the value means the same on any Sode.
- "The PIN" means the Advanced Settings PIN (`settingsPin`).
- "Trusted or PIN" means a Trusted user, or a connection holding the Advanced Settings PIN.

## Part 1. What Sode holds

### Where it lives

With `SODE_CONFIG` set, everything lives beside the config file. The image sets `SODE_CONFIG=/data/config.yaml` and puts History, the Communication Log and uptime in `/data` too (Dockerfile:23). Without `SODE_CONFIG`, config comes from `SODE_*` env and nothing persists (cmd/sode/main.go:56). The panel store then has no directory either (internal/app/panelstore.go:283-285), so panels, keys and the rest are memory only **(inferred for the stores other than panels, which follow the same `cfgPath == ""` pattern)**.

| Kind | Path | Owner in code | Written by |
| --- | --- | --- | --- |
| Config | `config.yaml` | internal/config/config.go:25-87 | `config.Save` (config.go:662) |
| Previous config | `config.yaml.prev` | config.go:667 | every `Save` copies the old file first |
| History | `history.jsonl` (`SODE_HISTORY`) | internal/app/history.go:18-43, :103 | takes, undo, ext |
| Communication Log | `sent.jsonl` (`SODE_SENTLOG`, cap `SODE_SENTLOG_MB`) | internal/app/sentlog.go:140 | every command and event |
| Uptime | `uptime.jsonl` (`SODE_UPTIME`) | internal/app/uptime.go:16 (400 days kept) | router link changes |
| API keys | `keys.json` | internal/app/apikeys.go:104-122, :161 | key create, revoke, expiry, setup links, session keys |
| Ended keys | `keys-ended.json` | apikeys.go:130-134, :182 | last 1024 ended key hashes |
| Session tokens | `tokens.json` | internal/app/tokens.go:35-42, :69 | persistent and tab sign-ins, held Locks |
| Panels | `panels/<id>.yaml` | panelstore.go:80-102, :286 | panel editor, Push, restore, user renames |
| Panel artwork | `panels/<id>.art/<art id>` | panelstore.go:457, :478 | artwork upload, Copy, Push |
| Push checkpoints | `panels/<id>.checkpoints` (at most 10) | internal/app/panelpush.go:68, :125 | Push, restore |
| Checkpoint sets | `panelsets/everyone.yaml`, `panelsets/me-<hex of user>.yaml` | panelpush.go:178, :184-200, :217 | Checkpoint set routes |
| Templates | `templates/me/<id>.yaml`, `templates/everyone/<id>.yaml` | internal/app/paneltemplates.go:46-60, :164-168 | Templates button in the editor |
| Default artwork | `artdefaults.yaml`, `artdefaults.art/` | internal/app/artdefaults.go:26-27, :49-57, :92-96 | Cell tab, Set as default |
| Icon library | `icons.yaml`, `icons/<id>.png|svg`, `icons/drop-in/` | internal/app/iconlib.go:26-27, :41-57; iconlibdrop.go:62 | upload, rename, delete, rescan |
| Panel watches | `panelwatch.yaml` | internal/app/panelmon.go:28, :39, :169, :208 | Watch on /panels |

docs/persisted-names.md has the field-level detail for every panel-side file. This note does not repeat it.

#### Held only in memory (lost on restart, so nothing to export)

- Public panel keys (`App.public`, docs/agents/panels.md, Keys).
- Panel monitor devices, uptime strips, hides and alarm clears (panelmon.go:54-160; docs/agents/panels.md, Monitoring).
- Tokenless sessions and the idle clock of a token (docs/agents/users-locks.md, Server idle clock).
- The PIN try pools for the Advanced Settings PIN and user PINs (docs/agents/users-locks.md, PIN pools).
- Router names, tally and Protected destinations (app.go:155-159), which Sode re-reads from the router.

### config.yaml, section by section

Who can change a field comes from its struct tag (config.go:25-87; rules in docs/agents/settings-saves.md):

- `guard:"open"`: the Settings form, Trusted or PIN (policy.go:126, `config` is needTrusted).
- `guard:"router"` and `guard:"pin"`: the Settings form, PIN only. A Trusted user's save that changes one is refused (internal/app/settings.go:210-216).
- `owned:"true"`: never through the Settings form. Each has its own save path, listed in the table.

| Section | Keys | Who changes it | Site-bound or portable |
| --- | --- | --- | --- |
| Router link | `router`, `matrix`, `levels`, `levelWireOffset`, `nameLength`, `nameRequests` | PIN (`router`), reconnects | Site-bound: address and router geometry |
| Native salvo | `fireMode`, `salvoGroup` | PIN | Site-bound: the router's salvo group is shared by all its clients (config.go:38) |
| Display of the router | `routerName`, `routerShow`, `numberOffset`, `levelNumbering`, `defaultLevels`, `levelColors` | Trusted or PIN | Site-bound. `levelColors` and `defaultLevels` index levels (config.go:59-62, :371-372) |
| Branding | `brandText`, `brandBorder` | Trusted or PIN | Portable, though it names a site in practice |
| Takes and timing | `confirmLockoutMs`, `resultDismissSeconds`, `confirmEnter`, `nameRefreshSeconds`, `tallyResyncSeconds` | Trusted or PIN | Portable |
| History | `hiddenDsts` | Trusted or PIN | Site-bound: a filter expression over destination names or numbers (config.go:48-49) |
| Proxy | `trustedProxies`, `requireHttps` | PIN | Site-bound: network addresses |
| Access | `settingsPin` (owned, the hash), `pinMaxTries`, `locks`, `autoLockMinutes` | PIN; `settingsPin` through the PIN fields of the Settings form | `settingsPin` is a hash, portable as a value but personal to an install |
| Filter sets | `filterSets` (owned) | Trusted or PIN, `filterSets` message (policy.go:129) | Site-bound: expressions over names or numbers |
| Presets | `presets` (owned) | Trusted or PIN, `presets` message (policy.go:134) | Site-bound: source and destination numbers and UI levels (internal/config/presets.go:23-36) |
| Device defaults | `deviceDefaults` (owned) | Trusted or PIN (docs/agents/settings-saves.md) | Portable |
| Users | `users` (owned), `maxPinLen`, `signInMode` | PIN, and a PIN must be set: `users` message (internal/app/users.go:405-416) | See Users below |
| API | `api` (owned): `enabled`, `rateLimit`, `takeCap`, `guestEnabled`, `logReads`, `maxUserKeys`, `maxClientKeys` | PIN, and a PIN must be set: `apiSettings` message (internal/app/apisettings.go:19-26) | Portable |
| API clients | `apiClients` (owned, never in JSON) | PIN, in the `users` message; panel clients by the panel editor | Mixed, see below |
| Guest | `guestRights`, `guestApiScopes` (owned) | PIN, in the `users` message | `guestRights` site-bound, scopes portable |
| Legacy | `salvos`, `grids`, `gridConfirm`, `hideDsts`, `levelTypedBase`, `locks.grid` | read only | n/a |
| Unknown keys | `Extra` on every struct (config.go:85-86) | kept through every save | Whatever a newer release wrote |

#### Users (internal/config/users.go:12-29)

| Field | Site-bound or portable |
| --- | --- |
| `name`, `username` | Portable as text, but every cross-reference below uses the name |
| `pin` (hash), `len` | A PBKDF2 hash. Portable as a value. `keys.json` and `tokens.json` are tied to it |
| `trusted`, `persistent`, `noAutoLock` | Portable |
| `rights` (config/api.go:50-57) | Site-bound: source and destination numbers, UI levels, preset names |
| `profile` (config/profile.go:17-24): `prefs`, `filterSets`, `presets`, `pinOrder` | `prefs` portable; sets and presets site-bound |

#### API clients (config/api.go:117-126)

`name`, `scopes`, `rights`, `panel`, `networks`. A plain client is made in Manage users > API clients. A panel's own client has `panel` set, is named `Panel <id>` (panelclient.go:17), is made and removed with its panel, and does not count toward the 12-client limit (config/api.go:213-222, MaxAPIClients at :40). `rights` is site-bound, `networks` is site network, `panel` is a panel id on this install.

### Browser storage

Per browser, per origin, so per install. Nothing here reaches the server except where noted. The table is grouped; docs/persisted-names.md and docs/agents/settings-saves.md list the per-key rules.

| Keys | File | What it holds | Site-bound or portable |
| --- | --- | --- | --- |
| `sode-theme`, `sode-layout`, `sode-clock`, `sode-flip`, `sode-scroll-top`, `sode-search-filters`, `sode-set-stack`, `sode-next-mode` | web/prefs.js:10-15, web/prepaint.js | This browser prefs | Portable |
| `sode-device-defaults`, `sode-profile-prefs` | web/prepaint.js:15, :24 | Caches of server values for first paint | Copies; no export value |
| `sode-presets`, `sode-preset-order`, `sode-preset-pins` | web/presetstore.js:19-21 | This browser presets and pin order | Presets site-bound (numbers) |
| `sode-filter`, `sode-ext`, `sode-filter-sets`, `sode-filter-active`, `sode-filter-stack`, `sode-filter-mode` | web/filterstore.js:10, :23, :61-62, :89; web/filters.js:39 | This browser filter sets and the live filter | Site-bound (expressions) |
| `sode-log-filter`, `sode-log-sort` | web/log.js:25, :336 | /log view | Portable |
| `sode-session` (local), `sode-tab-session` (session) | web/device.js:11 | Sign-in tokens | Install-bound: valid only against this server's `tokens.json` |
| `sode-last-user` (local), `sode-held-user` (session) | web/signin.js:15-16 | Username prefill, whose Lock | Install-bound |
| `sode-panel-key-<id or slug>` | web/panelkeyrules.js:100, web/panelsetup.js:11-79 | A panel instance's API key | Install-bound |
| `sode-panel-device` | web/panelkeepalive.js:9 | Instance name | Install-bound |
| `sode-panel-page-<id>` | web/pagekeep.js:3 | Last Page shown | Install-bound |
| `sode-panel-full` | web/panelfull.js:6 | Full width | Portable |
| `sode-share-host` | web/sharehost.js:6 | Address other devices use for links | Site network |
| `sode-salvos`, `sode-salvo-pins`, `sode-lock-token`, `sode-lock-never` | presetstore.js:26-27, signin.js:19-20 | Legacy, migrated or deleted on load | n/a |

### Profiles

A profile is `User.Profile` inside `config.yaml` (config/users.go:27; config/profile.go:17-24). It is never in the hello and is pushed only to connections signed in as that user (docs/agents/profiles.md). `profileSave` is allowed for your own name, and for anyone's with Trusted or PIN (policy.go:140). Size cap 48 KiB (docs/agents/profiles.md). A user rename carries the profile over by `Was`; a removed user loses it (users.go:443-447).

### Cross-references between kinds

Each row is a reference that can break when one side changes or moves.

| From | To | Matched by | Followed on change? |
| --- | --- | --- | --- |
| `users[].rights.presets`, `apiClients[].rights.presets`, `guestRights.presets` | Everyone presets | Name only, no scope (config/api.go:54, :64) | No. `SavePresets` replaces the whole list (internal/app/presets.go:11-16); nothing rewrites Rights, so a renamed preset leaves a dead name **(inferred from the absence of any rename path)** |
| Rights `sources`, `dsts`, `levels` | Router numbers, UI levels | Number | No. Validate only checks levels are within `levels` (config/api.go:180-182) |
| Presets, filter sets, `hiddenDsts`, profiles | Router numbers and names | Number or expression | No |
| Panel cells `n`, `settings.fixedDests`, cell `levels` | Router numbers, UI levels | Number (docs/persisted-names.md, Panel names) | No |
| Panel `preset` cell | Everyone preset or the caller's Me preset | `scope` and `name` | No. `me` resolves to the caller's own profile (internal/app/apipresets.go:33-34) |
| Default artwork (source, destination) | Router numbers | `n`, with the router name kept to warn on a mismatch (artdefaults.go:46-48) | Warns in the editor only |
| Default artwork (preset) | Everyone preset | `name`; `me` refused (artdefaults.go:82-84) | No |
| Panel or Default artwork `art: lib:<id>` | Icon library | Icon id | No. A deleted icon leaves cells drawing nothing (docs/agents/panels.md, Storage) |
| Panel `owner`, `share.users` | Users | Name | Yes on rename and removal from Manage users (users.go:526-557, panelstore.go:395-434). A removed owner stays as the owner **(inferred: `followUsers` rewrites owner only on rename)** |
| Template `owner`, Checkpoint set `owner` and `me-<hex>.yaml` | Users | Name | Yes on rename (users.go:546-547) |
| `panelwatch.yaml` | Users and panel ids | Lowercased name, panel id, device name | Yes on rename and removal (panelmon.go:350; users.go:548) |
| Checkpoint set entries | Panels | Panel id | No **(inferred)** |
| Panel `client` and `acts` | `apiClients[]` entry with `panel` | Client name `Panel <id>` (panelclient.go:20-26) | Orphan clients dropped only at startup (internal/app/app.go:172, panelclient.go:118-128) |
| `keys.json` user keys | Users | Owner name plus `pinFp`, a stamp of the PIN hash (apikeys.go:104-122) | Ended on PIN change, rename or removal (`pruneKeys`, keyserve.go:232; `reconcile` at startup, apikeys.go:422-426) |
| `keys.json` client keys | `apiClients` | Client name | Ended on removal or rename (docs/agents/users-locks.md, API keys) |
| `keys.json` session keys | `tokens.json` | `sess`, sha256 of the token (apikeys.go:113-115) | End with the session |
| `tokens.json` | Users | Name and PIN hash (tokens.go:35-42) | Pruned against live config on every update |
| History entries | Router numbers and names, users, panels | Number with name snapshot, user name, panel name (history.go:18-43) | Never rewritten |

### What the export surfaces cover today

There are three surfaces.

1. **Export & import modal**: a profile, Everyone or This browser scope (web/transfer.js, web/exportfmt.js). It holds `prefs`, `filterSets` and `presets`, plus `site` and `user` labels (exportfmt.js:46-56). Preset items carry `srcName` and `dstName` beside the numbers, so an import matches by name on another router (exportfmt.js:1-4, :148-190). Of the three, this is the only one that carries names with its numbers.
2. **Share links**: one preset or filter set as `#share=` (exportfmt.js:274-286).
3. **Server backup**: `config.yaml` only (internal/app/configfile.go:82-104). With "Include Secrets" off it blanks the PIN and user PIN hashes (configfile.go:90-98). Import replaces the whole config with no KeepOwned. It keeps live PIN hashes by user name (`KeepPins`, config/users.go:93-110; configfile.go:141-142) and refuses a new user with no PIN (config/users.go:108).

Not covered by any of them:

- Panels and their artwork, Push checkpoints, Checkpoint sets, templates, Default artwork, the Icon library and `panelwatch.yaml`. No `/api/v1` route exports or imports a panel to a file. The full route list is in the `registerAPI` calls; `POST /panels/{id}/copy` copies within one install.
- `keys.json`, `tokens.json`, History, the Communication Log and uptime.

The Server backup hint says "Everything saved on this server (settings, users, locks, filter sets, presets, profiles) as one config.yaml" (web/index.html:482). That list is accurate, but "Everything saved on this server" is wrong once panels exist.

Server backup and panels interact in two places:

- A backup carries the panel clients (`apiClients` with `panel`) but not the panels. On an install without those panels the clients stay until the next restart, when `dropOrphanPanelClients` removes them (app.go:172). `importConfig` has no panel handling of its own (configfile.go:115-160) **(inferred: no other caller)**.
- On an install that has the panels, importing a backup that lacks a panel's client removes the client. `pruneKeys` then ends that client's keys, and the panel keeps `acts: api` with no client, so its links stop working **(inferred from `clientOfPanel`, panelclient.go:20-26, and docs/agents/users-locks.md on client removal)**.

## Part 2. Every "you need X first" dead end

The table lists the refusal, the exact text the user sees, and what they must do elsewhere first. "Late" means the UI lets the user fill in a form and refuses only at Save.

### The setup chain for a panel on a fresh install

This is the travel-show path from #375, in the order the code forces it:

1. **Set the Advanced Settings PIN.** Settings > Access > Change Advanced PIN, then the main Save at the bottom of the form.
2. **Add a user.** Settings > Users > Manage users, then Save in that dialog. Tick Trusted if the panel will use a link with a key.
3. **Turn the API on.** Settings > API > API enabled, then Save API settings. This Save is separate from the main Save.
4. **Sign in as that user** in this browser: Settings > This browser > Sign in (web/index.html:195), or through a page lock.
5. **Make the panel** on /panels or /panels/editor.
6. **For a link with a key** (`api`, `both`, `public`), the signed-in user must be Trusted.

Steps 1 to 3 each have their own Save. The API section is `data-apart` (web/apisettings.js:53), so the unsaved-changes check skips it (web/settings.js:52-59). Ticking API enabled and pressing the main Save saves nothing for the API, and closing Settings gives no warning. That is how the travel show lost its API change.

### Settings and access

| What refuses | Exact message | Do first | Where |
| --- | --- | --- | --- |
| Save in Manage users with no PIN set (late) | "Set an Advanced Settings PIN first: users and locks are administered with it" | Set the PIN in Settings > Access and press the main Save | internal/app/users.go:23, :411-412. The dialog asks for a PIN only when one is set (web/users.js:381) |
| Save API settings with no PIN set (late) | Same text. It names users and locks, not the API | Same | internal/app/apisettings.go:22-23; web/apisettings.js:96 |
| Main Save with a page lock ticked and no user | "add a user before turning a lock on" | Add a user in Manage users | internal/config/users.go:177-178 |
| Main Save with a page lock ticked and no PIN | "set an Advanced Settings PIN before turning a lock on: users and locks are administered with it" | Set the PIN. Setting it in the same Save works, but a user is also needed and that needs the PIN saved first | config/users.go:180-181 |
| Main Save that removes the PIN while a lock is on | The same "before turning a lock on" text, which reads wrongly here **(inferred: Validate runs on the combined config)** | Untick the locks first | config/users.go:180-181 |
| Removing the last user while a lock is on | "a screen lock is on: turn it off in Settings > Access (and save) before removing the last user" | Untick the locks and press the main Save | users.go:428 |
| Switching Sign-in mode back to PIN only | Refusal naming users who lack a new or a distinct PIN (`backToPinRefusal`) | Type a new, distinct PIN for every user in the same save | docs/agents/users-locks.md, Sign-in mode |
| Username mode, a user with no username | "user %q needs a username" | Fill in usernames (Fill empty usernames helps) | config/users.go:160-164 |
| Require HTTPS with no trusted proxy | "Require HTTPS needs at least one trusted proxy: only a trusted proxy can say a request was secure" | Add a trusted proxy (Settings > Proxy) | config/api.go:206-207 |
| Require HTTPS saved over plain HTTP | "turning on Require HTTPS needs a request that arrived over HTTPS through a trusted proxy, so you can't lock yourself out; open Sode through your HTTPS address and save again" | Open Sode through the HTTPS proxy | internal/app/settings.go:70 |
| API client named like a user | "API client %q has the name of a user; History would not tell them apart" | Rename one | config/api.go:236-237 |
| A Trusted user changing a `router` or `pin` field, or the PIN | Refused as locked; the page asks for the PIN | Enter the PIN | settings.go:210-216; web/settings.js:489-494 |
| Server backup import of a user with no hash and no live user of that name | "user %q has no PIN in the file and no PIN here to keep: export with PINs included, or add the user in Settings first" | Export with Include Secrets, or add the user | config/users.go:108 |
| Server backup import of a file without `router` | "not a Sode configuration: no router setting (found: ...)" | Use a server backup, not a profile export | configfile.go:133 |
| Guest API on while the API is off | No refusal at Save. Every call gets `api_off` | Turn the API on | web/apisettings.js:25; apiauth.go:55-57 |

#### Where the policy table and the handlers disagree

With no PIN set, `settingsUnlockedMu` returns true (internal/app/pin.go:165), so every `needAdmin` message gets through `admit` (policy.go:213). That covers `configExport`, `configImport`, `clearHistory`, `keyListAll`, `keyCreateFor`, `keyRevokeAny`, `keyRevokeAll` and `userSignOutAll` (policy.go:114-169). On a fresh install, then, anyone may export or replace the whole config and clear History. `SaveUsers` and `SaveAPISettings` refuse anyway, inside the handler (users.go:411; apisettings.go:22).

The REST rule is the opposite of the socket rule. `mayEveryone` refuses when no PIN is set, even though its own message offers "or the Advanced Settings PIN" (paneltemplates.go:296-310). On a fresh install with a non-Trusted user, the only way through is to mark the user Trusted, and that needs the PIN set first.

### API and keys

| What refuses | Exact message | Do first | Where |
| --- | --- | --- | --- |
| Any `/api/v1` call while the API is off (503 `api_off`) | "the API is off; an administrator turns it on in Settings" | Settings > API, tick API enabled, Save API settings (PIN needed, step 3 above) | apiauth.go:55-57; apiv1.go:159-161 |
| A call with no key while Guest is off (401 `key_required`) | "send your key as Authorization: Bearer <key>" | Get a key, or turn Guest API on | apiauth.go:64-67 |
| A key without the route's scope (403 `scope`) | "this key lacks the %q scope" | Widen the client's scopes in Manage users > API clients (PIN) | apiv1.go:170-174 |
| `keyCreate`, `sessionKey` while not signed in | "unlock with your PIN first" | Sign in as a user | keyserve.go:14, :58-67; sessionkey.go:19-29 |
| `sessionKey` after a PIN change | "your PIN changed; unlock again" | Sign in again | sessionkey.go:44 |
| A Me preset over the API with a client's key | "no preset scope %q; use "everyone", or "me" with a user's key" | Use an Everyone preset on a panel that acts as an API client | internal/app/apitake.go:119-124 |
| An Idempotency-Key with no key | "Idempotency-Key needs an API key; a call without one has no caller of its own to keep the answer for" | Send a key | apitake.go:228 |

### Panels

| What refuses | Exact message | Do first | Where |
| --- | --- | --- | --- |
| /panels with no users | "Panels need a user, and Sode has none." | Add a user (PIN first) | web/panelslist.js:339; web/sessionkeyrules.js:9 |
| /panels while signed out | "Sign in with your PIN to see your panels." | Sign in | panelslist.js:338 |
| /panels with the API off | "Panels could not be loaded. the API is off; an administrator turns it on in Settings" (the server text is appended as is, lower-case start) | Turn the API on | panelslist.js:228; web/sessionkeyrules.js:61-76 |
| Panel editor with the API off | "the API is off; an administrator turns it on in Settings" in the editor's status **(inferred: `edCall` passes the server message through `jsonCall`, web/paneleditor.js:66, :606)** | Turn the API on | paneleditor.js:514-523 |
| A Session-mode panel page with no users | "This panel needs a user, and Sode has none." | Add a user | web/panelrules.js:46-47 |
| A Session-mode panel page while signed out | "Sign in with your PIN to open this panel." | Sign in | panelrules.js:44-45 |
| Any panel page (`/panels/<id>`) with the API off | "This panel could not be opened." No reason given, because `openFailText` covers only 404 and 401 | Turn the API on. Nothing on the page says so | panelrules.js:34-38; web/panelrun.js:177 |
| Creating or copying a panel with a client's key (403 `user_required`) | "a panel is owned by a user: use a user's key to create or copy one" | Use a user's key or a signed-in Sode page | apipanels.go:233 |
| Panel set to `api`, `both` or `public` by a user who is not Trusted (403 `trusted_required`) | "only a Trusted user may make a panel act as an API client or change its rights" | An administrator ticks Trusted in Manage users (PIN). No PIN alternative exists on this route | panelclient.go:75-77 |
| Panel client Rights wider than the user's own (403 `outside_rights`) | "the panel's rights may not be wider than your own" | Widen the user's Rights first (PIN) | panelclient.go:82-84 |
| Saving a template to Everyone (403 `trusted_required`) | "only a Trusted user, or the Advanced Settings PIN, may save a template to Everyone" | Be Trusted, or send the PIN. With no PIN set, only Trusted works | paneltemplates.go:334 |
| Everyone Checkpoint set | "only a Trusted user, or the Advanced Settings PIN, may save or change a checkpoint set for Everyone" | Same | apipush.go:200, :208, :297 |
| Default artwork set or clear | "only a Trusted user, or the Advanced Settings PIN, may change Default artwork" | Same | artdefaultsapi.go:124 |
| Default artwork for a Me preset (400) | "a Preset default needs scope "everyone" and the preset's name; Me presets have no Default artwork" | Use an Everyone preset | artdefaults.go:82-84 |
| Icon library add or change | "only a Trusted user, or the Advanced Settings PIN, may add to the Icon library" / "... may change the Icon library" | Same | iconlibapi.go:46, :145 |
| Icon library, Default artwork, templates, Assign, rename and watch with a client's key (403 `user_required`) | "the Icon library is changed with a user's key", "Default artwork is changed with a user's key", "a template is owned by a user: use a user's key to save one", "an Assign is made by a user: use a user's key", "a rename is made by a user: use a user's key", "watching belongs to a user: use a user's key" | Use a user's key | iconlibapi.go:33, :132; artdefaultsapi.go:109; paneltemplates.go:315; panelassign.go:111; panelrename.go:81; panelmon.go:903 |
| Assign to another panel | "only a Trusted user, or the Advanced Settings PIN, may assign an instance to another panel" | Be Trusted, or send the PIN | panelassign.go:24, :135 |
| Renaming another owner's instance | "only the panel's owner, a Trusted user or the Advanced Settings PIN may rename an instance" | Same | panelrename.go:20, :107-111 |
| Editing a panel you do not own (403 `not_owner`) | "this panel is use only for this key: only its owner or a Trusted user may change it" | Ask the owner, or a Trusted user | apipanels.go:113-114, :137 |
| A key link, Assign or setup link past 20 instances (409) | "this panel already has %d instances; revoke one first" / "%s already has %d instances, the most a panel holds; revoke one first" | Revoke an instance in the Share modal | panelkey.go:228; panelassign.go:106 |
| Artwork past the cap | "this panel's artwork would pass the 64 MB total; delete some artwork first" (also Default artwork and Icon library variants) | Delete artwork | panelstore.go:60; artdefaults.go:42; iconlib.go:36 |

## Findings for the other tickets on the map

- #381 (export format): three kinds carry router numbers with no names (Rights, panel cells and `fixedDests`, Default artwork apart from its one name snapshot), and the profile-scope export is the only one that already matches by name. Preset references are by name only, with no scope in Rights. Panels, artwork and the Icon library hold binary files up to 64 MB each, so a single YAML file through `/api/yaml` (1 MiB) or the socket (64 KiB) cannot hold them.
- #383 (one workflow): today's Server backup leaves out every file outside `config.yaml`, and its hint calls itself "Everything saved on this server". Importing a hash ends that user's keys through `pinFp` and their tokens through the stored hash. Panel clients travel in the config while their panels do not.
- #378 and #380 (actors, access positions): with no PIN set, every socket `needAdmin` action passes, but the users and API saves refuse, and REST `mayEveryone` refuses. Making a panel act as an API client needs Trusted, with no PIN route. Any new position has to settle these three rules together.
- #379 (one save model): the API section is `data-apart` and is skipped by the unsaved check, so its edits drop with no warning. The PIN and the locks use the main Save, while users and the API each have their own Save. The code forces the order: the PIN first, then users, then the API or a lock.
- #384 (wizard): the minimum path is PIN, then a user (Trusted for key links), then the API on, then sign in, then a panel. A panel page with the API off says only "This panel could not be opened."
- #385 (structure): the refusal text "users and locks are administered with it" also answers the API save, and the PIN-removal refusal reads "before turning a lock on".
- Glossary sweep (#375, Not yet specified): three server messages quoted above still use retired words. "unlock with your PIN first" (keyserve.go:14, users.go:21) and "your PIN changed; unlock again" (sessionkey.go:44) use "unlock", and "a screen lock is on" (users.go:428) uses a retired lock name. `TestDocsUseGlossaryWords` checks docs, so it flags these quotes here but not the Go strings themselves.
