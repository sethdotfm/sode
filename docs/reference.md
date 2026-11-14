# Sode reference

Details that the README leaves out. The README has the quickstart.

## Deployment

The page is themed with [patchbay](https://github.com/sethdotfm/patchbay), which is vendored and embedded, so the page fetches nothing from outside.

### Behind a proxy

Set `trustedProxies` (`SODE_TRUSTED_PROXIES`) to the proxy's address so history shows real client IPs. The server logs a warning when it sees `X-Forwarded-For` and the setting is empty. The proxy must pass WebSocket connections (`/ws`) and keep them open while idle for at least 15 seconds. The server sends a small heartbeat every 5 seconds. The page treats a connection that has been silent for 15 seconds as dead and reconnects. If the connection has been quiet for more than about 7 seconds, the page refuses a patch ("NOT sent") instead of losing it.

Require HTTPS (Settings > Proxy, `requireHttps`, an advanced setting) refuses every request except `/healthz` and `/api/health` unless it comes from an address in `trustedProxies` with `X-Forwarded-Proto: https`; the header from anyone else is ignored. Refusals are 403 with a body that says why. While it is on, responses carry `Strict-Transport-Security: max-age=3600`. It will not save with an empty `trustedProxies`, and a save that turns it on is refused unless that request itself arrived securely. Turning it on or off writes an `api` row to the Communication Log. If a bad proxy setup locks you out, run `docker compose exec sode /sode -reset-https && docker compose restart sode`.

### Install as an app

Sode is a web app. Use *Install* in Chrome or Edge, or *Add to Home Screen* in Safari on an iPhone or iPad, and it opens in its own window. Chrome and Edge only offer Install over HTTPS or on localhost, so put your TLS proxy in front first. The app name and icon follow **Logo text** and **Border colour** in Settings, so several instances stay apart on a home screen. Android and desktop Chrome pick up a changed name or icon on their own. On iOS, remove the app and add it again. The app still needs the server to run, so there is no offline mode.

## Using it

Type a source, Tab, a destination, Tab, levels, Enter, then press the churning confirm key. For the shortcut list, press `?` in keyboard mode, which you enter with Esc while every box is empty.

### Levels

Levels accept `1`, `1-3`, `1,3,5-8` (or `+`), `!1` (all but 1), `1-8!3`, and words: `thru`/`t` for `-`, `not`/`n` for `!`, `and` for `,`. On a phone's number pad `.` is `,` and `..` is `-`. Blank means the default levels. With V + A numbering, `v,a1-a8` also works.

### Several destinations

Press `+` or Ctrl+Enter after each destination instead of Tab. The routes land in History as one Group with **Undo all**.

### Presets

A preset is a saved plan of sources to destinations, fired in one confirm. **Edit**, or **Presets…** when there are none, opens the editor. Each entry in a preset is a source, destinations and levels. Levels can be numbers or names, and blank levels mean the default levels. **Add from Patch Boxes…** and **Add from History…** add entries for you. Save for **This browser** or **Everyone**. Everyone needs the Advanced Settings PIN, as filter sets do. Presets are stored as numbers, so a rename on the router changes nothing. A number the router no longer has is left out and listed in the confirm. A preset fires as one take, so it is one Group in History, marked "Preset: name", and Undo all undoes it. Quick search finds it by that name too. Only pinned presets show as buttons above History. Ctrl+g (⌘g on a Mac, the **Presets** button on a phone) opens a popup that lists all of them with a search box. Arrows or Ctrl+j/k move, Enter fires through the usual confirm, and Esc closes. Each row has **Pin/Unpin** for this browser. Pinning for everyone and sharing are in the editor, under the Pinned checkbox and Share. In keyboard mode, `p` jumps to the pinned presets. Because a preset is one confirm of several routes, Sode sends it the way the **Multi destination behavior** setting says, under Settings > Takes. See `SODE_FIRE_MODE` in the settings table below.

### Last take and next

Ctrl+s, Ctrl+d and Ctrl+f fill Source, Destination or Levels from your last take, the last one this browser made. A browser that has made no take has no last take, so the keys do nothing and the boxes show no Last chip. A take in History from another browser never fills a box. Each box shows that value as a small chip you can click. The Next chip and keys also need a take from this browser. Ctrl+Shift+s and Ctrl+Shift+d do the same with the next source and destinations, with every destination shifted, for patching a run in order. For example, Cam 1 to Rec 1 and Rec 5, then Cam 2 to Rec 2 and Rec 6. Each box also has a small "Next" chip that opens to show the value when you hover it or hold Shift. Press the keys again to keep going (Cam 3, then Cam 4, with more destinations each time) while the boxes still hold the last value. Settings > This browser > "Next match behavior" picks the next number or the next match of the name's text. When no name matches any more, it falls back to the next number.

### History

History lists every take, shared by all browsers and kept in a log. Undo restores what the router actually had before. In a row's Before panel, and in the steps of a stacked row, each source is a button that fills Source with it and moves on to Destination. Externals, such as ones from router panels or Companion, appear too, and you can undo them the same way. Every row has **Redo**, or **Retry** if its take failed, and a Group has **Redo all**. Redo takes that line's route again, and the confirm says when the destination already has it. In keyboard mode the key is `r`. Changes the router made without announcing them are marked **found**. Destinations that follow another, such as a multiviewer PiP following a DSM, show as an External follow of the take. Undo leaves them out unless you tick the box in the confirm. Sode doesn't log routes changed while it was disconnected.

### Search and filters

The Source and Destination filters above History and the **Filters** button (`/` in keyboard mode) limit what History shows, per browser. Filters can be named sets, shared with everyone from Settings or shared as a link. Click several set buttons in History's header to stack them. Click one again to take it off. With none left, History shows All. Under Settings > This browser, **Stacked filters behavior** says whether an entry must match all of them, which is the default, or any one. All filters use one small expression language:

| Typed | Matches |
|---|---|
| `65` | number 65 |
| `1-50`, `1 thru 50`, `1 to 50` | numbers 1 to 50 |
| `PIP` | names containing PIP (any case) |
| `'NOT USED'` | names containing that text, taken literally (`''` for a quote inside) |
| `"CAM 1"` | the name exactly (any case), so not CAM 10 |
| `/^DSM\d/` | names matching the regex (any case unless flags are given) |
| `!65`, `!PIP`, `not 'PIP'` | excludes |

Commas separate terms. A name matches if it fits any plain term and no `!` term. A filter of only `!` terms matches everything but those.

**Globally hidden destinations** (Settings > History) uses the same language to hide noisy destinations, such as a shader's monitor, from everyone's History. Sode still records them.

## Settings

Open Settings with the **Settings** button at the bottom of the page. Sode stores them in `/data/config.yaml`. Saving applies at once. Changes to the router, matrix, levels or name options reconnect and re-read names and routes. When there's no config file yet, Sode seeds it from these variables, and after that the file wins:

| Seed var | YAML key | Default | Meaning |
|---|---|---|---|
| `SODE_ROUTER` | `router` | | Router `host:port`. When empty, Sode stays idle until you set it in Settings |
| `SODE_MATRIX` | `matrix` | `0` | SW-P-08 matrix (0-15 with standard name requests) |
| `SODE_LEVEL_COUNT` | `levels` | `17` | Number of levels. Level 1 gets the video swatch, the rest audio |
| `SODE_LEVEL_WIRE_OFFSET` | `levelWireOffset` | `-1` | The router's level number = level + offset. `-1` puts level 1 on the router's level 0 |
| `SODE_DEFAULT_LEVELS` | `defaultLevels` | `1-<levels>` | Used when the levels box is blank. Always 1-based in the file |
| `SODE_LEVEL_NUMBERING` | `levelNumbering` | `1` | How levels are shown and typed: `1` (1-17), `0` (0-16) or `va` (V, 1-16) |
| `SODE_NAME_LEN` | `nameLength` | `12` | Name length to request: 4, 8, 12 or 16 |
| `SODE_NAMES` | `nameRequests` | `std` | `std` (cmd 100/102) or `ext` (cmd 228/230) |
| `SODE_FIRE_MODE` | `fireMode` | `sequential` | How Sode sends a confirm of several takes: `sequential` (one connect after another, about 16 ms per crosspoint on the Ultrix) or `native` (the router's own salvos: a SW-P-08 group send, cmd 120/248 and 121, which needs a router that supports them; verified on the Ross Ultrix). Measured on the Ultrix on 2026-11-02, a group Go announces its crosspoints about 4 ms apart (51 in about 210 ms), against about 16 ms each for ordinary connects (51 in about 840 ms). Whether the switch itself is simultaneous is unproven. Level 17 and numbers past 1023 are staged with the extended stage command (248) in the same group send. If the router doesn't acknowledge it, Sode clears the router's own salvo group and sends the takes one after another as before |
| `SODE_SALVO_GROUP` | `salvoGroup` | `127` | The router's salvo group (0-127) that native salvos use. Every controller shares the router's staged list, so pick a group nothing else uses |
| `SODE_NUM_OFFSET` | `numberOffset` | `1` | Added to router numbers for display (Ultrix shows 1-based) |
| `SODE_CONFIRM_LOCKOUT_MS` | `confirmLockoutMs` | `500` | How long the confirm key is ignored after the dialog opens |
| `SODE_RESULT_DISMISS_S` | `resultDismissSeconds` | `8` | A success banner closes itself after this many seconds (0 keeps it open). Failures always stay until closed |
| `SODE_CONFIRM_ENTER` | `confirmEnter` | `false` | Also allow Enter to confirm |
| `SODE_NAME_REFRESH_S` | `nameRefreshSeconds` | `60` | Re-read names this often (0 turns it off) |
| `SODE_TALLY_RESYNC_S` | `tallyResyncSeconds` | `300` | Re-read all routes and which destinations are protected this often (0 turns it off) |
| `SODE_TRUSTED_PROXIES` | `trustedProxies` | | IPs/CIDRs whose `X-Forwarded-For` Sode believes (your proxy) |
| `SODE_PIN_MAX_TRIES` | `pinMaxTries` | `10` | Wrong Advanced Settings PINs in a row before settings lock until reset or restart. `0` means never |
| `SODE_KILL_ENABLED` | `killEnabled` | `false` | Shows the Kill button in Settings > Access (see [Kill](#kill)). Turn it on only when the container's restart policy is `no` or `on-failure`; Sode cannot read the policy. Only the Advanced Settings PIN can change it |
| `SODE_MAX_PIN_LEN` | `maxPinLen` | `8` | Longest user PIN (4-12 digits). The lock pad stops there and submits on its own only at the longest length in use. Shorter PINs are sent with the check key or Enter. Also set in Settings > Users |

Settings-only keys (no seed var): `brandText` (replaces "Sode" in the header and page title, up to 24 characters), `brandBorder` (`blue`, `teal`, `green`, `amber`, `violet`, `pink`; a border round the window, off by default), `routerName`, `routerShow` (`address`, `name`, `none`), `levelColors`, `filterSets`, `hiddenDsts`, `deviceDefaults` and `settingsPin`. `deviceDefaults` holds what a browser uses until it chooses its own under Settings > This browser: theme, layout, clock, flip, scrollTop, searchFilters, setStack (`and` or `or`) and nextMode (`number` or `fuzzy`).

Process settings stay in env: `SODE_LISTEN` (`:8080`), `SODE_CONFIG` (`/data/config.yaml` in the image), `SODE_HISTORY` (`/data/history.jsonl`), `SODE_SENTLOG` (`/data/sent.jsonl`), `SODE_SENTLOG_MB` (`64`), `SODE_UPTIME` (`/data/uptime.jsonl`; without it the router uptime is kept in memory only) and `SODE_DEBUG=true` to hex-dump all SW-P-08 traffic.

### Advanced Settings PIN

Settings > Access can set a PIN of 4-64 characters that Settings asks for every time it opens. Patching stays open. Wrong tries count for everyone. From the 5th wrong try in a row there's a wait of 30 seconds, doubling up to an hour. After `pinMaxTries`, settings stay locked until the PIN is reset or Sode restarts. Sode stores only a salted PBKDF2 hash.

Forgot it?

```sh
docker compose exec sode /sode -reset-pin && docker compose restart sode
```

or delete the `settingsPin:` line from `/data/config.yaml` and restart.

Every save keeps the file it replaces as `/data/config.yaml.prev`, and the first start of each release keeps the file as `/data/config.yaml.before-<version>` (for a rollback). Both hold PIN hashes, so they are mode 0600 like `config.yaml`; copy them with the same care. A failed backup is logged and never blocks a save or startup. When the file has keys this release doesn't know (a rollback from a newer release), startup and `-reset-pin` or `-reset-https` first keep it as `/data/config.yaml.unknown-<UTC time>`, mode 0600, unless the newest such copy is the same. Sode never removes these copies.

### Kill

An emergency stop for an engineer who suspects Sode is misbehaving. With `killEnabled` on and an Advanced Settings PIN set, Settings > Access shows a **Kill Sode** button once the PIN is entered on that browser. The dialog asks for the PIN again every time; a wrong one counts toward `pinMaxTries`. A Kill writes a `kill` row to the Communication Log, then the process exits at once with code 0, with no graceful shutdown:

- The router link drops, and takes not yet sent are lost.
- Takes already sent may still land on the router, and routes the router already accepted stay.
- Every browser shows the Lost link banner.
- Docker starts Sode again unless the container's restart policy is `no` or `on-failure`. Exit code 0 is not a failure, so `on-failure` keeps it down. With either policy, Docker also does not start Sode after a host reboot, so start it by hand (`docker compose up -d`). The shipped `compose.yaml` uses `unless-stopped`, under which a Kill only restarts Sode. Source: [Docker, Start containers automatically](https://docs.docker.com/engine/containers/start-containers-automatically/).

## Users and locks (optional)

Users and locks are off by default. With no users and no locks, Sode behaves as before. Settings > Users has a Manage users button that opens its own dialog with its own Save. There you add people, each with a name and a 4-12 digit PIN. In the default Sign-in mode, PIN only, PINs must differ, since the PIN alone says who is signing in. In Username and PIN mode (`signInMode: username`, chosen in the Manage users dialog's Sign-in mode box and saved with the users, so it needs the Advanced Settings PIN) each user also has a username, entered in a Username column that only shows in that mode, separate from the name History shows and unique ignoring case. A Fill empty usernames button there sets each empty username to the user's Name without spaces (a number is added when two collapse to the same one, and typed usernames are never changed), so an install with no usernames switches in one save. Two users may then share a PIN, and the sign-in screen asks for the username and the PIN together (the PIN pad then waits for a username before it sends). The Lock of a signed-in user still asks for the PIN only. In PIN only mode the screens are unchanged. Switching to Username and PIN keeps users, PINs and Profiles. Switching back to PIN only needs a new PIN for every user in the same save, all different, because Sode can't tell whether saved PINs match. The dialog marks every user as needing one and blocks the save until each has one and no two match, and the server refuses the save naming the users otherwise. The Rights tab has a No auto-lock box for a user who runs a wall browser that the idle auto-lock must skip. Only an administrator sets it, with the Advanced Settings PIN, and the Lock button still works for that user. Manage users also has an API clients tab and a Rights tab. Rights limit which sources, destinations, levels and presets a user, an API client or Guest may take, for the browser and the API alike. A row's Edit user rights button opens that user in the Rights tab, and one Save sends it all. A user's own API keys are under Settings > My user, in **My API keys**. Settings > Access turns on locks per page type, for the main screen, the Communication Log and the panel pages, and sets the idle auto-lock. Those need the Advanced Settings PIN like the rest of Access. A locked page opens behind a lock screen with a number pad. It opens the moment the typed PIN matches, with no Enter. The server enforces locks per WebSocket connection. Until a connection signs in as a user, the server refuses its takes, name re-reads and history clearing, and on the log page, reading the Communication Log and uptime graph. Saving settings, filter sets, presets and users needs the Advanced Settings PIN, not just any signed-in user, so a locked wall panel can still be set up with the PIN. A trusted user, described below, can also save settings, filter sets and presets. `/healthz` and `/api/health` stay open. The user's name replaces the IP in history and in the log.

- **Lock again.** Press Ctrl+L or the Lock button in the corner, or use Lock in Settings > This browser. With a user signed in, that raises the Lock and keeps the session. The server holds it, so a reload or a reconnect shows the PIN pad again, and Sode sends nothing to the router from that session until the same user's PIN resumes it. Another user's PIN doesn't resume it and counts as a wrong try. Other tabs of the same session lock and resume with it. **Sign out** on the Lock ends the session and revokes its tokens. The idle minutes set in Settings (0 means never) raise the same Lock for every signed-in user, Never sign out users included, except a user marked No auto-lock. It never signs anyone out, and their PIN resumes the session with nothing lost. The page runs that timer, and the server holds the Lock it raises. A dropped connection of a user who isn't signed in for good also locks, since that sign-in belongs to the connection.
- **Trusted users.** You can mark each user Trusted in Settings > Users. While signed in as a trusted user, saving the Settings form, filter sets and presets needs no Advanced Settings PIN. Some actions still need the Advanced Settings PIN: clearing history, adding or removing users and locks, setting or removing the Advanced Settings PIN, and changing anything that reconnects or weakens a guard. That covers the router address, matrix, levels or level offset, how names are read, the fire mode and the router's salvo group, trusted proxies and the PIN attempt limit. Unticking Trusted takes effect at once.
- **Never sign out.** A user marked Never sign out gets a random session token when they sign in. The browser keeps it in localStorage. The server keeps only its hash in `tokens.json` next to the config file, up to 8 per user, dropping the oldest. A token not used for 90 days expires, and a page that stays connected counts as using it. A reload or a dropped connection does not log them out, and the idle timer locks them without signing them out. Ticking the box while they are already signed in, or changing their own PIN, gives their open page a token at once, so the next reload keeps them in. Sign out revokes the token. Lock and Ctrl+L keep it and mark the session locked, so the next reload still shows the PIN pad. Changing that user's PIN, removing the user or unticking the box ends every one of their sessions. A wall panel signs in once as a trusted user who never signs out.
- **Lock with nobody signed in.** Where no PIN lock is on and nobody is signed in, **Ctrl+L** (or Settings > This browser > Lock the screen) covers the page until you press the letter it shows, like a take's confirm, after the confirm lockout. On a touch screen, tap the button. It guards against keyboard mashing and is not a PIN. It covers only this browser page, and a reload clears it. While someone is signed in, the Lock is the server-held one described under Lock again: a PIN pad with no Cancel, and the letter does nothing. The auto-lock minutes also put the letter Lock up when nobody is signed in and the page is idle.
- **Wrong PINs** are counted in two separate classes, each for everyone. The Advanced Settings PIN is one class, with the Advanced Settings PIN rules above: waits from the 5th try and a permanent lock after `pinMaxTries`. User PINs are the other class. Their waits start at the 5th try and double up to 5 minutes, with no permanent lock, so strangers' guesses can't shut the users out. A right user PIN does not reset the count, so holding one PIN doesn't give free guesses at the others. Sode forgets the count after an hour without a wrong PIN. A success or lockout in one class never changes the other. A PIN of a length no user has is just a wrong try, but the page only sends lengths some user has. With users of different lengths, the page sends a PIN half a second after the last digit.
- **Advanced Settings PIN required.** You administer users and locks with the Advanced Settings PIN, so you must set it first, and a lock can't be on without one. That is why `sode -reset-pin` turns the page locks off too. If everyone is locked out otherwise, stop Sode and set `locks:` to all `false` in `/data/config.yaml`. Users are stored as `users: [{name, pin (hash), len, trusted, persistent}]`. Set PINs in Settings, not by hand.
- **Limits.** A lock guards actions, not data. The log lock also guards reading the log. Names, routes, recent history, user names and PIN lengths keep streaming to a locked browser, by design. Idle auto-lock is a client-side convenience in the page, and the server does not enforce it.

## Profiles, sharing and export (optional)

With users set up, each user has a **profile**. It holds their own This browser settings, filter sets and presets, and Sode keeps it so it goes with them to any browser. Wherever you can save a setting, filter set or preset, you get a third choice, **Me**, beside **Everyone** and **This browser**. A browser's own choice wins, then the user's, then everyone's. So setting Light on one tablet doesn't change Ana's other devices unless she presses **Set as my default** (Settings > This browser). Someone who isn't signed in is a guest. A guest gets this browser's and everyone's settings and sees no Me options. With no users configured, none of this appears. To sign in with no locks on, use **Sign in** under Settings > This browser. That sign-in lasts until the tab closes, unless the user has Never sign out. A profile belongs to its user, but admins and trusted users can see and edit every profile. Each row of Manage users has an **Edit Profile** button, and trusted users get the same buttons in Settings > Users. Everyone signed in has **My profile** and **Import & Export** under Settings > This browser. On the Lock, **Sign in as…** switches user.

**Share and export.** A preset's or filter set's **Share** gives a link you can paste in chat. Opening the link shows what it holds next to what you have now, and you choose where to save it. It never applies itself unreviewed. Settings > **Export & import** writes any mix of settings, filter sets and presets to a YAML file and reads one back through the same review. The file is named `YYYY-MM-DD-Sode-Export-Site-User.yaml` unless you name it. Exports carry names beside numbers, so a freelancer's presets can land on another studio's router. Sode matches a source or destination by number when the name agrees, else by its name. It flags anything ambiguous or missing and never applies it silently.

**Server configuration backup.** Settings > Server backup has **Export** and **Import** for the whole `config.yaml`, and both need the Advanced Settings PIN. PINs are left out unless you tick *Include Secrets (private backup)*, which makes a secret file, so keep it private. Importing replaces everything. If the file has no PINs, each user keeps their current PIN by name, and Sode refuses a new user in the file until it carries a PIN.

## Icon library files

The Icon library is the `icons/` directory and its index, `icons.yaml`, both beside `config.yaml` (in `/data` in the image). Neither the server configuration backup nor any export carries it. To back it up or move it to another install, copy `icons.yaml` and the `icons/` directory together with `config.yaml`, then start Sode. A cell names an icon by its id (`lib:<id>`), and Push, Copy and Templates keep that reference, since they stay on one install. A cell whose icon is not in the library draws no Icon A.

To add many icons at once, put SVG and PNG files in `icons/drop-in/`. Sode makes that directory on start. It reads the files at start and when a Trusted user presses **Rescan** in the Uploaded section of Choose icon (the API's `POST /icon-library/rescan`, which also takes the Advanced Settings PIN). Each file is checked as an upload is: PNG or SVG, at most 2 MB, a PNG at most 1024 by 1024 pixels, and 64 MB for the whole library. The file name without its extension becomes the icon name, cut to 40 characters, and a name another icon has becomes "Name 2". Sode removes each file it adds from `icons/drop-in/`. A file it skips stays there, and Sode reports it with the reason in the Rescan's status line and in an `iconRescan` row of the Communication Log. Sode never reads through a link: a linked file is skipped, and a `drop-in` that is itself a link is not read. Hidden files such as `.DS_Store` are ignored.

## API

`/api/v1` lets other apps read names, tally, History and status, fire takes and presets, undo, redo and retry History entries, and follow live changes on a WebSocket stream. It is off until an administrator turns on *API enabled* in Settings (advanced); *Guest API* lets a call with no key in with Guest's scopes, and *Log API reads* writes one `api` row per read. Keys, scopes, rights, visibility and limits are in [api.md](api.md), and the contract is `/api/v1/openapi.yaml`, open even while the API is off.

## Monitoring (Uptime Kuma)

`GET /api/health` reports the server and the router separately. It needs no PIN and shows no settings:

```json
{"server":"up","router":"up","targetAddress":"10.0.0.1:8910","version":"Beta 1.3.1","uptimeSeconds":86400,"browsers":2}
```

`router` is `up`, `connecting` or `down`. `err` appears when the router link has a problem. The status code is 200 only while the router is up and 503 otherwise, so a plain HTTP(s) monitor alerts when the router is lost. A dead server shows as no answer. Uptime Kuma counts a 503 as down even for a JSON Query monitor, so to watch the server alone, add `503` to that monitor's accepted status codes. For a JSON Query monitor, use `router` as the key and `up` as the expected value. `/healthz` is unchanged.

## Communication log

`/log` lists every message Sode wrote to the router, newest first and live, with what it was for and whether the router ACKed it. Open it with **View Logs** in Settings > History, which shows even while Settings are locked. The page is read-only. Settings has a search box above its section list that finds sections and single settings by name or by common words ("password", "dark", "proxy"). Sode keeps the sign in `SODE_SENTLOG` up to `SODE_SENTLOG_MB`. Without that file, the last 20,000 messages stay in memory.

Besides messages to the router, the log has event rows. They show in the default view and have their own filter chips:

- **Browsers.** A browser's WebSocket connecting or dropping, with its address.
- **Commands.** What people fire that changes something: takes, undos and retries, re-reading names, clearing history, saving settings, filter sets or presets, and entering and relocking the Advanced Settings PIN. For the PIN, the row says whether it was right, never the PIN itself.
- **Router link.** The router link going down once per outage, coming up and being changed in Settings, and Sode starting and stopping.

ACKed messages also show how long the router took.

### Router uptime graph

The **Router uptime** graph at the top of the log is one strip with toggles for 1 hour, 24 hours (the default view), 7 days and 30 days. Each toggle shows its bar length, for example "24 hours (15 min)". A toggle also sets the log's From time to that long ago, so the table shows the same period, and Clear times undoes it. Setting From or To by hand shows that range instead. Click a bar to see only that slot. The log's times become the bar's, the table is sorted newest first, and a banner says so. **Back** restores the times and sort you had. Each bar is a slot: a minute for an hour, 15 minutes for a day and an hour for a week. Hover a bar for the details. A bar is green when the router link was up the whole slot. It is yellow when the link was down for part of the slot, or messages failed, needed a retry or took over 300 ms to ACK. It is red when the link was down or Sode wasn't running, and grey with nothing recorded. Sode keeps the record for 400 days in `SODE_UPTIME`. It finds the times it wasn't running from a stamp it rewrites every minute, so a crash shows as down from the last minute it was alive.

## Protocol notes

Crosspoints use extended commands: 0x82 connect, 0x84 connected and 0x95/0x97 tally dump. A take succeeds once the router sends Connected for every requested level, with a 3s timeout. Routers send nothing for a connect that changes nothing, so Sode interrogates and counts those as "already". Sode reads which destinations are protected with the extended protect dump (0x93, answered by 0x94) and refuses takes to protected destinations before sending anything. Sode discards messages queued while disconnected. Names are polled, since the router doesn't announce renames.

## Simulator

The simulator (`go run ./cmd/swp08sim`, on :8910) takes `SIM_*` variables to fake misbehaving routers: `SIM_IGNORE_DSTS`, `SIM_SILENT_DSTS`, `SIM_SLOW_DSTS`, `SIM_FOLLOWS`, `SIM_PANEL_SECONDS`, `SIM_PANEL_SILENT`, `SIM_NO_SALVO=1` for a router that lacks the group send commands, and `SIM_SALVO_ANNOUNCE=1` to announce a group Go's routes with Connected, as the Ultrix sometimes does. See `cmd/swp08sim`.

## Releases and image tags

You cut releases by tag and promote them without a rebuild. [RELEASING.md](../RELEASING.md) has the steps and the rules. In short, bump `Version` in `internal/app/version.go`, the `**Version:**` line in README.md and the `## Beta <version>` heading in [CHANGELOG.md](../CHANGELOG.md), which `TestVersionNoted` checks. Then merge to `main`, push a `vX.Y.Z-rc.N` tag to build a signed candidate, and run the *promote* workflow to make it `X.Y.Z` and `latest`.

| Tag | Follow it if you want |
| --- | --- |
| `latest`, or `1.5` for one minor line | tested releases only (use this with automatic updates) |
| `rc` | the next release before it ships |
| `edge` | whatever is merged on `main`, unsigned and possibly broken |
| `1.5.0`, or `@sha256:...` | to stay put |

Releases touch only the first row, so automatic updaters on `latest` never receive a candidate or an `edge` build.
