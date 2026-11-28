# Sode reference

Details that the README leaves out. The README has the quickstart.

## Deployment

The page is themed with [patchbay](https://github.com/sethdotfm/patchbay). The patchbay files and the IBM Plex fonts (`web/vendor/fonts`, SIL Open Font License) are vendored and embedded. So the page fetches nothing from outside and draws with no internet connection.

Each web file has an `ETag` and `Cache-Control: no-cache`. A browser keeps its copy and asks again on each load: an unchanged file costs one `304`, and a new build shows at once. Text files (HTML, JS, CSS, SVG) go gzip-compressed to a browser that accepts gzip. Sode compresses each file once, at its first request.

### Behind a proxy

Set `trustedProxies` (`SODE_TRUSTED_PROXIES`) to the proxy's address so history shows real client IPs. The server logs a warning when it sees `X-Forwarded-For` and the setting is empty. The proxy must pass WebSocket connections (`/ws`) and keep them open while idle for at least 15 seconds. The server sends a small heartbeat every 5 seconds. The page treats a connection that has been silent for 15 seconds as dead and reconnects. If the connection has been quiet for more than about 7 seconds, the page refuses a patch ("NOT sent") instead of losing it.

Require HTTPS (Settings > Proxy, `requireHttps`, an advanced setting) refuses every request except `/healthz` and `/api/health` unless it comes from an address in `trustedProxies` with `X-Forwarded-Proto: https`. Sode ignores the header from anyone else. Refusals are 403 with a body that says why. While it is on, responses carry `Strict-Transport-Security: max-age=3600`. It will not save with an empty `trustedProxies`. Sode refuses a save that turns it on unless that request itself arrived securely. Turning it on or off writes an `api` row to the Communication Log. If a bad proxy setup locks you out, run `docker compose exec sode /sode -reset-https && docker compose restart sode`.

### Install as an app

Sode is a web app. Use *Install* in Chrome or Edge, or *Add to Home Screen* in Safari on an iPhone or iPad, and it opens in its own window. Chrome and Edge only offer Install over HTTPS or on localhost, so put your TLS proxy in front first. The app name and icon follow **Logo text** and **Theme color** in Settings, so several instances stay apart on a home screen. Android and desktop Chrome pick up a changed name or icon on their own. On iOS, remove the app and add it again. The app still needs the server to work. On a secure origin, a page that the device opened once also opens while Sode is down (README). It shows its Offline state and connects when Sode comes back.

## Using it

Type a source, Tab, a destination, Tab, levels, Enter, then press the churning confirm key. For the shortcut list, press `?` in keyboard mode. Esc with empty boxes starts it, and Alt+Shift+k (⌥⇧k on a Mac) starts and ends it.

### Levels

Levels accept `1`, `1-3`, `1,3,5-8` (or `+`), `!1` (all but 1), `1-8!3`, and words: `thru`/`t` for `-`, `not`/`n` for `!`, `and` for `,`. On a phone's number pad `.` is `,` and `..` is `-`. Blank means the default levels. With V + A numbering, `v,a1-a8` also works.

### Several destinations

Press `+` or Ctrl+Enter after each destination instead of Tab. The Destination box then empties, so the next digits start a new search. Directly after an add, `+` or Ctrl+Enter on the empty box adds the next destination. The box shows which one: "Ctrl+Enter adds:" and a 12 Rec 2 chip. It uses the **Next match behavior** setting, as the Next chip does. That is the next router number, or the next destination with the same name before its number. The phone does the same: after `+` its Destination box empties and shows "+ adds 12 Rec 2". So Cam 1, Tab, Rec 1 and Ctrl+Enter four times, then Enter, takes Cam 1 to Rec 1 to Rec 4. The routes land in History as one Group with **Undo all**.

A list or a range in the Destination box, such as `11-13`, `11,12,13`, `Rec 1-4` or `Rec 1-Rec 4`, shows all its destinations selected in the match list. A range of names can have more name after it. For example, `MV 1-4 PIP 1` gives MV 1 PIP 1 to MV 4 PIP 1. The name after the range can have a second range. `MV 1-4 PIP 1-2` gives MV 1 PIP 1, MV 1 PIP 2, MV 2 PIP 1 and so on to MV 4 PIP 2. A list can have two ranges in one name, not more. A range of numbers has no name after it, so `1-4 PIP 1` is a normal search. A space between a word and its number is optional, so `MV1 PIP1` and `MV 1 PIP 1` are the same. Enter, Tab, `+` or Ctrl+Enter adds them all. The list tells you which numbers or names the router does not have. A destination with a name that starts with the text makes it a normal search. The Source box takes one source: for many routes, use the Grid (Ctrl+e).

Ctrl-click a destination in History, in a Followed by list or in the Destination match list to add it the same way. On a Mac, Cmd-click also adds it. A click without Ctrl fills the box as before.

### Presets

A preset is a saved plan of sources to destinations, fired in one confirm. **Edit**, or **Presets…** when there are none, opens the editor. Each entry in a preset is a source, destinations and levels. Levels can be numbers or names, and blank levels mean the default levels. **Add from Patch Boxes…** and **Add from History…** add entries for you. Save for **This browser** or **Everyone**. Everyone needs a Trusted user or an Administrator, as filter sets do.

Sode stores presets as numbers, so a rename on the router changes nothing. A number the router no longer has is left out and listed in the confirm. A preset fires as one take, so it is one Group in History, marked "Preset: name", and Undo all undoes it. Quick search finds it by that name too.

The Bookmarks strip above History holds your bookmarks: presets, pages, filter sets, settings or web addresses. Alt+1 to 9 open the first nine. Ctrl+g (⌘g on a Mac, **Show all** on the strip) opens a popup that lists all presets with a search box. Arrows or Ctrl+j/k move, Enter fires through the usual confirm, and Esc closes. In keyboard mode, `p` jumps to the bookmarks.

A preset is one confirm of several routes. So Sode sends it the way the **Multi destination** setting of the router says (Settings > Routers, the Takes group). See `SODE_FIRE_MODE` in the settings table below.

### Last take and next

Ctrl+s, Ctrl+d and Ctrl+f fill Source, Destination or Levels from your last take, the last one this browser made. The browser keeps it across reloads and shares it with every tab, and anyone who signs in on that browser sees it. It never expires. A patch sets it, and so does a redo whose routes all share one source and one set of levels. A Group redo with several sources, a Retry and a Preset leave it alone. A browser that has made no take has no last take, so the keys do nothing and the boxes show no Last chip. A kept take whose source or a destination the router no longer has counts as none. A take in History from another browser never fills a box. Each box shows that value as a small chip you can click.

The Next chip and keys also need a take from this browser. Ctrl+Shift+s and Ctrl+Shift+d do the same with the next source and destinations, with every destination shifted, to patch a run in order. For example, Cam 1 to Rec 1 and Rec 5, then Cam 2 to Rec 2 and Rec 6. Each box also has a small "Next" chip that opens to show the value when you hover it or hold Shift. Press the keys again to keep going (Cam 3, then Cam 4, with more destinations each time) while the boxes still hold the last value. A reload ends the run, and the next press steps from the last take. Settings > This browser > "Next match behavior" picks the next number or the next match of the name's text. When no name matches any more, it falls back to the next number.

### History

History lists every take, shared by all browsers and kept in a log. Undo restores what the router actually had before. In a row's Before panel, and in the steps of a stacked row, each source is a button. The button fills Source with that source and moves on to Destination. Externals, such as ones from router panels or Companion, appear too, and you can undo them the same way. Every row has **Redo**, or **Retry** if its take failed, and a Group has **Redo all**. Redo takes that line's route again, and the confirm says when the destination already has it. In keyboard mode the key is `r`. Sode marks changes that the router made without an announcement as **found**. Destinations that follow another, such as a multiviewer PiP following a DSM, show as an External follow of the take. Undo leaves them out unless you tick the box in the confirm. Sode does not log routes changed while it was disconnected.

When one destination gets several changes on the same levels, each within 30 seconds of the one before, History shows them as one stacked row. The same time folds quick Protect or Free Protect actions by one person into one row. Settings > This browser > **Stack quick changes in History** sets that time. It is 30 seconds by default, or 10 seconds, 1 minute, 2 minutes or 5 minutes. Off shows every change as its own row. The changes can come from any browser, panel, API client or router panel. The row shows the source the destination had before the first change, then "to" and the source after the last change. Open the row to see each change with its time and a **Restore** button. **Undo** on the row goes back to before the first change. A failed take, an undo, a Group and an External follow always get their own row.

### Home key

Alt+Shift+h (⌥⇧h on a Mac) is the Home key. It works on every page and from inside a box or a dialog. On another page it opens Home in the same tab. On Home it closes the Palette, the Grid popup, the open dialog and Settings, then puts the focus in the Source box. It does not clear the patch boxes or the staged routes: Esc does that. Unsaved changes in Settings, the panel editor, the preset editor or the Bookmarks editor get the usual warning on the first press. The second press discards them. Under the Lock, on a Kiosk panel and during setup it does nothing.

### Compact view

Alt+Shift+c (⌥⇧c on a Mac) turns Compact view on or off on the desktop main page. It hides History and centres the title, the patch boxes and a smaller Presets row in the window. Results show under Presets and do not move the boxes. The Settings and Sode Panels buttons (bottom left), the Lock bar (top left) and the Server and Router status (top right) fade out. They come back while pointed at or focused, and keep their place. The status fades only while both links are up: any other state, and the Lost link banner, always show. The key works from inside the patch boxes too, so on a Mac Option+Shift+c toggles Compact view there and does not type Ç. It sets This browser; Settings can also set it for your profile or for Everyone. With History hidden there is no keyboard mode, and Esc only clears the boxes. While you use the preset editor's **Add from History…** to pick takes, History shows again. The phone layout ignores Compact view.

### Search and filters

The Source and Destination filters above History and the **Filters** button (`/` in keyboard mode) limit what History shows, per browser. Filters can be named sets, shared with everyone from Settings or shared as a link. Click several set buttons in History's header to stack them. Click one again to remove it from the stack. With none left, History shows All. Under Settings > This browser, **Stacked filters behavior** says whether an entry must match all of them, which is the default, or any one. All filters use one small expression language:

| Typed | Matches |
|---|---|
| `65` | number 65 |
| `1-50`, `1 thru 50`, `1 to 50` | numbers 1 to 50 |
| `PIP` | names containing PIP (any case) |
| `'NOT USED'` | names containing that text, read literally (`''` for a quote inside) |
| `"CAM 1"` | the name exactly (any case), so not CAM 10 |
| `/^DSM\d/` | names matching the regex (any case unless flags are given) |
| `!65`, `!PIP`, `not 'PIP'` | excludes |

Commas separate terms. A name matches if it fits any plain term and no `!` term. A filter of only `!` terms matches everything but those.

**Globally hidden destinations** (Settings > History) uses the same language to hide noisy destinations, such as a shader's monitor, from everyone's History. Sode still records them.

## Settings

Open Settings with the **Settings** button at the bottom of the page. Sode stores them in `/data/config.yaml`. A save applies immediately. Changes to the router, matrix, levels or name options reconnect and re-read names and routes. When there is no config file yet, Sode opens the setup wizard, and these variables fill its fields. The wizard's Finish writes the file, and after that the file wins.

The router's keys sit under `routers`, a list that holds one router. Each router also has a `uid`, which Sode makes when it adds the router and never changes, not even with the address. It has a `prefix`, one or two letters A to Z (`A` for the first router), hidden while there is one router. Rows marked "router" below are keys of that router. The others are top-level keys. The first key is `format: 2`. The top-level `installUid` names this install. Sode makes it on first start and never changes it, not even with a config import; Item links carry it. A file without `format` is from v1, and on start Sode moves the v1 files into `/data/v1/` and starts as a fresh install:

```yaml
format: 2
installUid: 3b8e05c1d29a7f44
routers:
  - uid: 7f3c9a1e2b4d5f60
    prefix: A
    title: Studio Ultrix
    address: 10.0.0.50:8910
    matrix: 0
    levels: 17
statusLabel: address
```


| Seed var | YAML key | Default | Meaning |
|---|---|---|---|
| `SODE_ROUTER` | `address` (router) | | Router `host:port`. When empty, Sode stays idle until you set it in Settings |
| `SODE_MATRIX` | `matrix` (router) | `0` | SW-P-08 matrix (0-15 with standard name requests) |
| `SODE_LEVEL_COUNT` | `levels` (router) | `17` | Number of levels. Level 1 gets the video swatch, the rest audio |
| `SODE_LEVEL_WIRE_OFFSET` | `levelOffset` (router) | `-1` | The router's level number = level + offset. `-1` puts level 1 on the router's level 0 |
| `SODE_DEFAULT_LEVELS` | `defaultLevels` (router) | blank (every level) | Used when the levels box is blank. Always 1-based in the file. Blank stays blank, so it stays every level when the level count changes |
| `SODE_LEVEL_NUMBERING` | `levelNumbering` (router) | `1` | How levels are shown and typed: `1` (1-17), `0` (0-16) or `va` (V, 1-16) |
| `SODE_NAME_LEN` | `nameLength` (router) | `12` | Name length to request: 4, 8, 12 or 16 |
| `SODE_NAMES` | `nameRequests` (router) | `std` | `std` (cmd 100/102) or `ext` (cmd 228/230) |
| `SODE_FIRE_MODE` | `fireMode` (router) | `sequential` | How Sode sends a confirm of several takes: `sequential` (one connect after another, about 16 ms per crosspoint on the Ultrix) or `native` (the router's own salvos: a SW-P-08 group send, cmd 120/248 and 121, which needs a router that supports them; verified on the Ross Ultrix). Measured on the Ultrix on 2026-11-02, a group Go announces its crosspoints about 4 ms apart (51 in about 210 ms), against about 16 ms each for ordinary connects (51 in about 840 ms). Whether the switch itself is simultaneous is unproven. Level 17 and numbers past 1023 are staged with the extended stage command (248) in the same group send. If the router does not acknowledge it, Sode clears the router's own salvo group and sends the takes one after another as before |
| `SODE_SALVO_GROUP` | `salvoGroup` (router) | `127` | The router's salvo group (0-127) that native salvos use. Every controller shares the router's staged list, so pick a group nothing else uses |
| (none) | `protectDevice` (router) | `0` | Sode's device number for Protects (0 turns them off, at most 4095). The Protects group of each router in Settings > Routers. Panel IDs and device numbers are one number space, and the router does not check who sends a number. So pick a number no panel uses (ADR 0012) |
| `SODE_NUM_OFFSET` | `numberOffset` (router) | `1` | Added to router numbers for display (Ultrix shows 1-based) |
| `SODE_CONFIRM_LOCKOUT_MS` | `confirmLockoutMs` | `500` | How long the confirm key is ignored after the dialog opens |
| `SODE_RESULT_DISMISS_S` | `resultDismissSeconds` | `8` | A success banner closes itself after this many seconds (0 keeps it open). Failures always stay until closed |
| `SODE_TIE_HOP_WEIGHT` | `tieHopWeight` | `1` | The cost of each hop that changes a tie line, added to the number of Affected destinations (0-100). Settings > Routers > Tie lines |
| `SODE_CONFIRM_ENTER` | `confirmEnter` | `false` | Also allow Enter to confirm |
| `SODE_NAME_REREAD_S` | `nameRereadSeconds` (router) | `60` | Re-read names this often (0 turns it off) |
| `SODE_TALLY_RESYNC_S` | `routeRereadSeconds` (router) | `300` | Re-read all routes and which destinations are protected this often (0 turns it off) |
| `SODE_TRUSTED_PROXIES` | `trustedProxies` | | IPs/CIDRs whose `X-Forwarded-For` Sode believes (your proxy) |
| `SODE_MAX_PIN_LEN` | `maxPinLen` | `8` | Longest user PIN (4-12 digits). The lock pad stops there and submits on its own only at the longest length in use. Shorter PINs are sent with the check key or Enter. Also set in Settings > Users |
| `SODE_ACCESS_PRESET` | | | An Access preset: `open` (Open access), `standard` (Standard access) or `secured` (Secured access); the full names also work. On a first start the setup wizard preselects it, and env-only mode applies it with the page locks off. An unknown name stops the first start |

Settings-only keys (no seed var):

- `brandText`: replaces "Sode" in the header and page title, up to 24 characters.
- `brandBorder` (`blue`, `teal`, `violet`, `lilac`, or a custom `#rrggbb`), the Theme color: a border round the window, off by default. Sode makes the accent from it (docs/color-roles.md). `green`, `amber`, `pink`, `sky` and `slate` are retired: Settings does not offer them. A config that has one still loads, and green, amber and pink show a caution note in Settings.
- `title` (router): the router title, up to 32 characters.
- `statusLabel` (`address`, `name` or `none`): the text next to the Routers icon in the status bar, one setting for every router. It is Status label at the top of Settings > Routers > Routers. It exports in the `routers` group of settings.yaml. Older releases kept it on each router. A file with no top-level `statusLabel` takes the first router's, and Sode does not write the router's key again.
- `routers`: every router, in the order of the list in Settings > Routers. The first router is the default router (GLOSSARY.md). The one Save key `routers/order` (a list of router UIDs) changes the order.
- `levelColors`
- `filterSets`
- `tieLines`: the tie lines of Settings > Routers > Tie lines (Administrator). Each has a hidden `id` and a `name`, the Tie line name (unique, up to 32 characters). `from` is a destination, `{router, n}`, and `to` is a source on another router. Each end also keeps its router `name` from the save. Sode fills a blank `name` from the source name. The section saves each change at once, apart from the Save button, and needs two or more routers.
- `hiddenDsts`
- `browserDefaults`: what a browser uses until it chooses its own under Settings > This browser. It holds theme, layout, clock, flip, scrollTop, searchFilters, setStack (`and` or `or`), nextMode (`number` or `fuzzy`), stackGap and compact. The stackGap field is a number of seconds written as text: `0` (off), `10`, `30` (the default), `60`, `120` or `300`. The compact field is Compact view, off by default (see Compact view above). The paletteChar field is the Palette character: one of `>~!/;:=#@$%^&*|?<_()[]{}§°¬£€¤`, or `off` for no character. The default is `>`.

Process settings stay in env. They are `SODE_LISTEN` (`:8080`), `SODE_CONFIG` (`/data/config.yaml` in the image), `SODE_HISTORY` (`/data/history.jsonl`), `SODE_COMMLOG` (`/data/sent.jsonl`), `SODE_COMMLOG_MB` (`64`), `SODE_UPTIME` (`/data/uptime.jsonl`) and `SODE_DEBUG=true`, which hex-dumps all SW-P-08 traffic. `SODE_SETTINGS_SAVES` (`/data/settings-saves.jsonl`) holds the Settings saves record. Without `SODE_UPTIME`, the router uptime is kept in memory only.

### Not saved

Without `SODE_CONFIG`, Sode reads its settings from the `SODE_*` variables and keeps every change in memory only. The main page shows a yellow Not saved banner, on a phone too. There is no setup wizard. Until you add a user, every browser is an Administrator and no lock or PIN recheck applies. `SODE_ACCESS_PRESET` writes its preset, but the page locks stay off. The first user you add must be an Administrator with a PIN. A restart starts empty again, and Item links and paired panels from before the restart stop working.

To fix it, set `SODE_CONFIG` to a file on a volume that Sode keeps, for example `/data/config.yaml` (the image sets this), and restart Sode. The setup wizard then runs, and the `SODE_*` variables fill its fields.

### No Administrator

Sode always keeps one Administrator with a PIN. Use this command if no Administrator knows their PIN, or if a hand-edited config file has users but no Administrator. Run it on the machine that runs Sode, in the directory with its `docker-compose.yml`:

```sh
docker compose exec sode /sode -make-administrator "NAME" && docker compose restart sode
```

It asks for a new PIN on the terminal, and the PIN does not show as you type it (Linux and macOS). The user named NAME (any case) becomes an Administrator with that PIN. A name that no user has adds a new Administrator, if Sode has fewer than 12 users. At 12 users, give the name of a user that exists. The command refuses a PIN that breaks the PIN rules. To pipe the PIN in, use `echo 1234 | docker compose exec -T sode /sode -make-administrator "NAME"`. Restart Sode at once: until the restart, the running server keeps its old user list, and a save from it writes the old list back. That user's API keys end at the restart, as for any PIN change.

A config with users and no Administrator still starts. Sode refuses every Administrator action and names the command in the refusal. The main page shows a banner with the command.

Every save keeps the file it replaces as `/data/config.yaml.prev`, and the first start of each release keeps the file as `/data/config.yaml.before-<version>` (for a rollback). Both hold PIN hashes, so they are mode 0600 like `config.yaml`; copy them with the same care. A failed backup is logged and never blocks a save or startup. If the file has keys that this release does not know (a rollback from a newer release), Sode first keeps a copy of it. The copy is `/data/config.yaml.unknown-<UTC time>`, mode 0600. It does this at startup and on `-make-administrator` or `-reset-https`, unless the newest such copy is the same. Sode never removes these copies.

## Users and locks (optional)

Users and locks are off by default. With no users and no locks, Sode behaves as before. Settings > Users has a Manage users button that opens its own dialog. Its changes save with the Save button of Settings. There you add people, each with a name and a 4-12 digit PIN. Two users can have the same PIN.

Every sign-in asks for the name, then the PIN. The name box finds a user as you type and lists every user. Pick a name with the arrow keys and Enter, or click it. The browser remembers the last name. The Lock of a signed-in user asks for the PIN only.

The Rights tab has a No auto-lock box for a user who runs a wall browser that the idle auto-lock must skip. Only an Administrator sets it, and the Lock button still works for that user. Manage users also has an API clients tab and a Rights tab. Rights limit which sources, destinations, levels and presets a user, an API client or Guest can command, for the browser and the API alike. A row's Edit user rights button opens that user in the Rights tab, and the Save button of Settings sends it all. A user's own API keys are under Settings > My user, in **My API keys**.

Settings > Access > Locks turns on locks per page type, for the main screen, the Communication Log and the panel pages, and sets the idle auto-lock. Only an Administrator changes them, like the rest of Locks. A locked page opens behind a lock screen with a number pad. It opens the moment the typed PIN matches, with no Enter. The server enforces locks per WebSocket connection.

Until a connection signs in as a user, the server refuses its takes, name re-reads and history clearing. On the log page, it also refuses reads of the Communication Log and uptime graph. A save of shared settings, Everyone filter sets and Everyone presets needs a Trusted user or an Administrator. A save of users needs an Administrator. `/healthz` and `/api/health` stay open. The user's name replaces the IP in history and in the log.

- **Lock again.** Press Ctrl+L, or open the Lock drop-down in the corner and pick Lock, or use Lock in Settings > This browser. With a user signed in, that raises the Lock and keeps the session. The server holds it, so a reload or a reconnect shows the PIN pad again. Sode sends nothing to the router from that session until the same user's PIN resumes it. Another user's PIN does not resume it and counts as a wrong try. Other tabs of the same session lock and resume with it. **Sign out** on the Lock ends the session and revokes its tokens. The idle minutes set in Settings (0 means never) raise the same Lock for every signed-in user, Never sign out users included. The one exception is a user marked No auto-lock. It never signs anyone out, and their PIN resumes the session with nothing lost. The page runs that timer, and the server holds the Lock it raises. A dropped connection of a user who is not signed in for good also locks, since that sign-in belongs to the connection.
- **Roles.** Each user has a Role in Manage users: Standard user, Trusted user or Administrator. A Trusted user can save the shared settings of Look and feel that a Trusted user edits, Everyone filter sets and Everyone presets. Only an Administrator can clear history, change users, API clients and locks, or change anything that reconnects or weakens a guard. That covers the router address, matrix, levels or level offset, and how names are read. It also covers the fire mode and the router's salvo group, trusted proxies and the Wait after wrong PINs switch. A Role change applies immediately. A user without a PIN acts as a Standard user, or as the Guest role when that is higher. Their Rights are no wider than Guest Rights.
- **Never sign out.** A user marked Never sign out gets a random session token when they sign in. The browser keeps it in localStorage. The server keeps only its hash in `tokens.json` next to the config file, up to 8 per user. Past that it drops the oldest. A token not used for 90 days expires, and a page that stays connected counts as using it. A reload or a dropped connection does not sign them out, and the idle timer locks them without signing them out. If you tick the box while they are already signed in, or they change their own PIN, their open page gets a token immediately. As a result, the next reload keeps them in. Sign out revokes the token. Lock and Ctrl+L keep it and mark the session locked, so the next reload still shows the PIN pad. Changing that user's PIN, removing the user or unticking the box ends every one of their sessions. A wall panel signs in one time as a trusted user who never signs out.
- **Lock with nobody signed in.** Where no PIN lock is on and nobody is signed in, **Ctrl+L** (or Settings > This browser > Lock the screen) covers the page. It stays until you press the letter it shows, like a take's confirm, after the confirm lockout. On a touch screen, tap the button. It guards against keyboard mashing and is not a PIN. It covers only this browser page, and a reload clears it. While someone is signed in, the Lock is the server-held one described under Lock again. It is a PIN pad with no Cancel, and the letter does nothing. The auto-lock minutes also put the letter Lock up when nobody is signed in and the page is idle.
- **Wrong PINs** for users are counted per user name, so one person's wrong PINs do not slow down anyone else. Waits start at the 5th wrong PIN for that name and double from 30 seconds up to 5 minutes. A right PIN for that name stops its wait. Nothing locks for good. Sode forgets the count after an hour without a wrong PIN. Settings > Access > Locks > Wait after wrong PINs (on by default) turns the waits off, except for the names of Administrators, which always wait. Names that match no user always wait too. A PIN of a length no user has is only a wrong try, but the page only sends lengths some user has. With users of different lengths, the page sends a PIN half a second after the last digit.
- **Administrator required.** You change users and locks as an Administrator, and a lock cannot be on without a user. If no Administrator knows their PIN, use the command under No Administrator. Sode stores each user under `users:` with a PIN hash, never the PIN. Set PINs in Settings, not by hand.
- **Limits.** A lock guards actions, not data. The log lock also guards reading the log. Names, routes, recent history, user names and PIN lengths keep streaming to a locked browser, by design. Idle auto-lock is a client-side convenience in the page, and the server does not enforce it.

## Profiles, sharing and export (optional)

With users set up, each user has a **profile**. It holds their own This browser settings, filter sets and presets, and Sode keeps it so it goes with them to any browser. Wherever you can save a setting, filter set or preset, you get a third choice, **Me**, beside **Everyone** and **This browser**. A browser's own choice wins, then the user's, then everyone's. For example, if Ana sets Light on one tablet, her other devices do not change unless she presses **Set as my default** (Settings > This browser).

Someone who is not signed in is a guest. A guest gets this browser's and everyone's settings and sees no Me options. With no users configured, none of this appears. To sign in with no locks on, use **Sign in** under Settings > This browser. That sign-in lasts until the tab closes, unless the user has Never sign out.

A profile belongs to its user, but admins and trusted users can see and edit every profile. Each row of Manage users has an **Edit profile** button, and trusted users get the same buttons in Settings > Users. Everyone signed in has **My profile** and **Import & Export** under Settings > This browser. On the Lock, **Sign in as…** switches user.

**Share and export.** A preset's or filter set's **Share** gives a link you can paste in chat. Opening the link shows what it holds next to what you have now, and you choose where to save it. It never applies itself unreviewed.

**Import & Export** under Settings > This browser writes your settings, filter sets and presets to a YAML file. It reads one back through the same review. The file is named `YYYY-MM-DD-Sode-Export-Site-User.yaml` unless you name it. Exports carry names beside numbers, so a freelancer's presets can land on another studio's router. Sode matches a source or destination by number when the name agrees, else by its name. It flags anything ambiguous or missing and never applies it silently.

**Export & Import.** Settings > Server > **Export & Import** (Administrators only, on a computer) opens a dialog with Export and Import tabs. Export lists nine categories: Settings (by section), Routers, Users, Presets, Filter sets, Panels, Artwork, API clients and This browser. Tick whole categories or single items; each item and category shows its size, and Export shows the size of the zip. A router always goes with an item that uses it, and the Icon library icons that a panel or Default artwork uses go with it. **Include PINs** (on by default) puts the users' PIN hashes in the file, so keep such a file private. **Back up everything** exports every category with PINs in one click. The Import tab comes in a later release.

## Icon library files

The Icon library is the `icons/` directory and its index, `icons.yaml`, both beside `config.yaml` (in `/data` in the image). Neither the server configuration backup nor any export carries it. To back it up or move it to another install, copy `icons.yaml` and the `icons/` directory together with `config.yaml`, then start Sode. A cell names an icon by its id (`lib:<id>`), and Push, Copy and Templates keep that reference, since they stay on one install. A cell whose icon is not in the library draws no Icon A.

To add many icons in one step, put SVG and PNG files in `icons/drop-in/`. Sode makes that directory on start. It reads the files at start and when a Trusted user presses **Rescan** in the Icon library dialog (Choose icon, Uploaded icons, **Icon library**). The API's `POST /icon-library/rescan` does the same.

Sode checks each file as it checks an upload. It must be PNG or SVG, at most 2 MB, and a PNG at most 1024 by 1024 pixels. The whole library has a limit of 64 MB. The file name without its extension becomes the icon name, cut to 40 characters. A name that another icon has becomes "Name 2". Sode removes each file it adds from `icons/drop-in/`. A file it skips stays there. Sode reports it, with the reason, in the Rescan's status line and in an `iconRescan` row of the Communication Log. Sode never reads through a link: a linked file is skipped, and a `drop-in` that is itself a link is not read. Hidden files such as `.DS_Store` are ignored.

## API

`/api/v1` lets other apps read names, tally, History and status, and fire takes and presets. Apps can also undo, redo and retry History entries, and follow live changes on a WebSocket stream. The API is always on. *Keyless API* lets a call with no key in, with the *Keyless scopes* inside Guest Rights. *Log API reads* writes one `api` row per read. Keys, scopes, rights, visibility and limits are in [api.md](api.md), and the contract is `/api/v1/openapi.yaml`, open with no key.

## Monitoring (Uptime Kuma)

`GET /api/health` reports the server, the state of Sode as a whole and each router. It needs no PIN and shows no settings:

```json
{"server":"up","state":"degraded","routers":[{"uid":"3f9c...","prefix":"A","title":"Main","router":"up","targetAddress":"10.0.0.1:8910"},{"uid":"7d21...","prefix":"B","title":"Stage 2","router":"down","targetAddress":"10.0.0.2:8910","err":"connection refused"}],"version":"Beta 1.3.1","uptimeSeconds":86400,"browsers":2}
```

`state` is `healthy` when every router is up and `down` when every router is down. It is `degraded` when one or more routers are down and one or more are up. A router that is connecting counts as down. `routers` has one element for each router, in the order of Settings. Its `router` is `up`, `connecting` or `down`, and `err` appears when that router link has a problem. There are no flat router fields: read `state` and `routers`. The status code is 200 only while Sode is Healthy and 503 otherwise, so a plain HTTP(s) monitor alerts when any router is lost. A dead server shows as no answer. Uptime Kuma counts a 503 as down even for a JSON Query monitor. To watch the server alone, add `503` to that monitor's accepted status codes. For a JSON Query monitor, use `state` as the key and `healthy` as the expected value. `/healthz` only says that Sode answers: `{"server":"up"}`.

## Communication log

`/log` lists every message Sode wrote to the router, newest first and live, with what it was for and whether the router ACKed it. Open it with **Communication Log** at the foot of Settings. Settings search finds it in History. The page is read-only. Settings has a search box above its section list that finds sections and single settings by name or by common words ("password", "dark", "proxy"). Sode writes the log to `SODE_COMMLOG` up to `SODE_COMMLOG_MB`. Without that file, the last 20,000 messages stay in memory.

Besides messages to the router, the log has event rows. They show in the default view and have their own filter chips:

- **Browsers.** A browser's WebSocket that connects or drops, with its address.
- **Commands.** What people fire that changes something. These are takes, undos and retries, re-reading names, clearing history, and saving settings, filter sets or presets. A sign-in row says whether the PIN was right, never the PIN itself.
- **Router link.** The router link when it goes down (one row per outage), comes up or is changed in Settings, and Sode when it starts and stops.
- **Settings saves.** One row for each save of shared settings, in Settings or through the API. Open the row to see each field with its old and new value. A list shows what was added and removed, and a PIN shows only "changed". Values of Access and Server fields show only to an Administrator. Changes in My user and This browser get no row. Sode keeps these rows for 365 days, and at most 5 MB, in `SODE_SETTINGS_SAVES`. They are in no export.

ACKed messages also show how long the router needed.

### Router uptime graph

The **Router uptime** graph at the top of the log is one strip with toggles for 1 hour, 24 hours, 7 days and 30 days. The default view is 24 hours. Each toggle shows its bar length, for example "24 hours (15 min)". A toggle also sets the log's From time to that long ago, so the table shows the same period, and Clear times undoes it. Setting From or To by hand shows that range instead. Click a bar to see only that slot. The log's times become the bar's, the table is sorted newest first, and a banner says so. **Back** restores the times and sort you had.

Each bar is a slot: a minute for an hour, 15 minutes for a day and an hour for a week. Hover a bar for the details. A bar is grey when the router link was up the whole slot. A healthy state shows no color (docs/color-roles.md). It is yellow when the link was down for part of the slot. It is also yellow when messages failed, needed a retry or needed more than 300 ms to ACK. It is red when the link was down or Sode was not running, and the color of an empty control with nothing recorded. Sode keeps the record for 365 days in `SODE_UPTIME`, and at most 5 MB of it. Sode finds the times when it was not running from a stamp that it rewrites every minute. As a result, a crash shows as down from the last minute that Sode was alive.

## Protocol notes

Crosspoints use extended commands: 0x82 connect, 0x84 connected and 0x95/0x97 tally dump. A take succeeds when the router sends Connected for every requested level, with a 3s timeout. Routers send nothing for a connect that changes nothing, so Sode interrogates and counts those as "already". Sode reads which destinations are protected with the extended protect dump (0x93, answered by 0x94). It also applies the 0x8D and 0x8F the router sends, one for each level, when a client changes which levels of a destination are protected. Sode refuses takes to protected destinations before it sends anything, also to a destination that Sode itself Protected. Sode sets a Protect with 0x8C and clears it with 0x8E, on every level of the destination. Each message covers one level, and 0x8D or 0x8F confirms it. Sode discards messages queued while it is disconnected. Names are polled, since the router does not announce renames.

## Simulator

The simulator (`go run ./cmd/swp08sim`, on :8910) has 200 sources, 120 destinations and 17 levels. `SIM_SOURCES`, `SIM_DESTS` and `SIM_LEVELS` change these sizes, for example `SIM_SOURCES=2500 SIM_DESTS=750` for a large router. It also reads `SIM_*` variables to fake misbehaving routers: `SIM_IGNORE_DSTS`, `SIM_SILENT_DSTS`, `SIM_SLOW_DSTS`, `SIM_EXTERNAL_FOLLOW`, `SIM_EXTERNAL_SECONDS` and `SIM_EXTERNAL_SILENT`. `SIM_NO_SALVO=1` fakes a router that lacks the group send commands. `SIM_SALVO_ANNOUNCE=1` announces a group Go's routes with Connected, as the Ultrix sometimes does. `SIM_PROTECTS=12:2:9,20:1:112` (0-based dst:kind:device) starts those destinations Protected on every level. Kind 1 is a Protect and kind 2 a Router lock. Dst 12 starts with a Router lock by device 9, and `12:0:0` frees it. A connect to a Protected level gets an ACK and no other reply. `SIM_NAMES_FILE` names a file that the simulator reads again on every name request, so a test can rename while it runs. Lines are such as `src 12 CAM C` or `dst 0 PGM` (0-based); no name makes that number blank. See `cmd/swp08sim`.

## Releases and image tags

You cut releases by tag and promote them without a rebuild. [RELEASING.md](../RELEASING.md) has the steps and the rules. In short, bump `Version` in `internal/app/version.go`, the `**Version:**` line in README.md and the `## Beta <version>` heading in [CHANGELOG.md](../CHANGELOG.md), which `TestVersionNoted` checks. Then merge to `main`, push a `vX.Y.Z-rc.N` tag to build a signed candidate, and run the *promote* workflow to make it `X.Y.Z` and `latest`.

| Tag | Follow it if you want |
| --- | --- |
| `latest`, or `1.5` for one minor line | tested releases only (use this with automatic updates) |
| `rc` | the next release before it ships |
| `edge` | whatever is merged on `main`, unsigned and possibly broken |
| `1.5.0`, or `@sha256:...` | to stay put |

Releases touch only the first row, so automatic updaters on `latest` never receive a candidate or an `edge` build.
