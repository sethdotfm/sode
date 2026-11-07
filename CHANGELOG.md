# Changelog

Newest first. Each release also has notes on [GitHub](https://github.com/sethdotfm/sode/releases). Give it a section for each release when you bump `Version` (see README, Releasing); `TestVersionNoted` checks the heading.

## Beta 1.6.0

- **Cleanup before rc.3**, no new features: middle dots are gone from the UI. The log's Why cell, the log's one-slot note, *not pinned* in Presets and the Filters button count are spaced apart instead, and page titles, option labels and summaries use plain wording (*Text (my default)*, *100% up, 0 drops*, *ACK in 12 ms*). Behind that, code comments are cut to context only, `web/` is formatted with Prettier, `filters.js` and `desktop.js` are split into smaller modules (`filtereditor.js`, `searchbox.js`, `desktoprecent.js`, `desktopkeys.js`), presets and filter sets share one scoped save and delete, both layouts share the Recent row wording (`recentrows.js`), and the server has one config commit and one WebSocket dispatch table (the policy table). A format and lint hook (`.claude/hooks/lint.sh`) runs on edits.
- **Review fixes before rc.2**: one prompt for a settings PIN needed mid-session (the popup asks and then retries Manage users, the config file, clearing history and refused saves; a trusted user's Settings no longer collapses to the PIN prompt after a refusal, and clearing history reports only when the server did it); result banners no longer pile up, close in both layouts together, and a failure that resolves now times out; the lock screen keeps the foot in the page flow (no overlap on a short screen), one Log out button for both locks and a long site name refits when the layout changes; the pick bar is placed by the phone layout (and follows a layout switch); Settings search finds View Logs again; browse lists scroll to the chosen row; the composer's bottom padding follows the real keyboard; code moved out of `plans.js`, `settings.js`, `desktop.js` and `index.html` into `banner.js`, `settingstoc.js`, `settingspin.js`, `about.js`, `shortcuts.js`.
- **Phone fixes**: *Your account* is its own section after This device (which has tighter fields on a phone); the lock screen scales a long site name to fit, sits in the visible height (dvh, plus the safe area) so its foot buttons aren't behind the address bar, and keeps *Log out* (now also for a logged-in user's lock) at the bottom beside *Log in as…*; no double-tap zoom on the PIN pad or anywhere else on the phone; no empty band above the keyboard in the composer; the "Logged in as" chip no longer sits over the page on a phone (it shows under a lock screen only; the name is in Settings); Redo (and Retry for failed takes) is a swipe: drag a Recent row or folder right to show it, left still shows Undo; the buttons in the rows are gone; Settings on a phone is a list of sections, each a page of its own (Back returns to the list, then leaves; search and failing fields open their section); every text field, select and text area is at least 16px on the phone, so iOS no longer zooms in on focus and stays zoomed; opening or leaving Settings (or entering its PIN) no longer puts the cursor in a box on a phone, which raised the keyboard; choosing takes for a preset: the bar is a row of the phone view instead of covering the patch summary, a picked row keeps a solid background (the swipe-undo button showed through it), and the composer waits while choosing from Recent.
- *Manage users* without the settings PIN asks for it in a popup (pinprompt.js) instead of opening an empty editor, and a refusal inside the dialog asks again the same way; Settings no longer flips to its locked layout for it. The fire mode reads *Sequential*.
- **Locking while logged in needs the PIN** (#198): with a user logged in and no PIN lock on, the lock button, Ctrl+L, *Lock the screen* and the idle lock end the session and show the PIN pad (no Cancel, but a *Log out* button); the letter hand lock is only for pages nobody is logged in on. Also: the *Report a bug* tooltip opens upward, *My profile* has no ellipsis, the example hex has its swatch inside the code block and the name-refresh *s* suffix is gone.
- **Settings polish, round 4**: the Profile dialog is wider with tiles (*User defaults* beside Filter sets and Presets), bordered rows, no intro line and *Add preset* / *Add filter set*; long section names wrap in the section list; subtitles sit under their headings; *Refreshed …* is its own box beside *Refresh now*; a tooltip on *Report a bug*; space under the account name; the *my default* buttons lost their tooltips.
- **Settings polish, round 3**: sections renamed (*Global defaults*, *Router Source & Destination Names*, *Sode Networking*) and *Branding* moved below Matrix; the server backup is its own *Server Backup* section (*Include Secrets*, Export / Import without ellipses); the Advanced PIN fields sit behind a *Change Advanced PIN* button; Page locks and the idle time share a row; View Logs moved to the bottom row beside the centred version; the global filter-sets list is gone from History (Recent → Filters edits them); most hints removed or reworded, a Seconds suffix on the name refresh, a swatch beside the example hex, side-by-side *Save as my default* / *Clear my default*, one-line config path, *Edit Profile* and *Manage users* buttons.
- **Settings polish, round 2**: This device options read the same logged in or out (*Everyone's default (…)*, *My default (…)*, tagged *· everyone's default*), *Browser language* shows what it gives (12- or 24-hour), the account name is large with the user's name in the accent, *Lock the screen* sits at the row's right end, *Import & Export*, no rule over This device, and the locked prompt is a PIN box with the hint as its placeholder, an *Open* button and a same-width *Forgot PIN?*; View Logs moved to the Exit row and the version is centred there.
- **Settings polish** (annotations on #197): Ctrl+J/K (⌘J/K on a Mac) step through the sections; On/Off preferences read Yes/No; *Searching history respects filters*, *Stacked filters behavior*, *Next match behavior*; the theme, account and lock-screen hints are gone; Lock sits beside Log in under a rule, with a lock icon; the locked prompt is *Advanced Settings* with Unlock on the PIN's row and a small *Forgot PIN?* under it; *View Logs* and the GitHub link carry icons.
- **Result banners dismiss themselves** (annotations on #197): a success closes after *Success banner closes after (s)* (Settings → Takes, `resultDismissSeconds`, default 8, 0 keeps it) with a bar running down; every banner has a close button, and failures stay until closed. Focusing an empty (or confirmed) Source or Destination box no longer opens the whole list: a small *Show all sources* / *Show all destinations* button sits under it instead (also when a search finds nothing), and ↓ or ↑ opens the list too. That list shows no cursor until ↓ is pressed, since Enter does not take its first row.
- **User profiles** (#161): a third scope, *Me*, beside Everyone and This browser, for This device settings, filter sets and presets, kept on the server so they follow a user to any browser. Order: this browser, then Me, then Everyone. *Set as my default* per setting. The user edits their own (*My profile…*); trusted users and the settings PIN can see and edit any (a *Profile…* button per user). A profile is capped at 48 KiB. New messages `profile`, `profileSave`, `profileGet`.
- **Locks and auto-lock** moved from the Users dialog into **Settings → Access**, saved by the main Save and changed only with the settings PIN. A Settings save that leaves them out keeps them.
- **Log in without a lock** (#187): with users but no locks, Settings → This device has *Log in*, kept for the tab. A guest gets this browser's and everyone's settings; a previous user's cached settings are no longer applied to a guest.
- **Share links, export and import** (#169): *Share* on a preset or filter set makes a `#share=` link (a URL fragment, so it never reaches the proxy; old `?f=` links still open); *Settings → Export & import* writes and reads a YAML file. Everything opens a review sheet (Importing → Current, Replace / Keep both / Skip). Exports carry names beside numbers so they can move between studios. New stateless `POST /api/yaml` converter.
- **Server configuration export / import** (Settings → Access, settings PIN only): download `config.yaml` (PINs left out unless *Include PINs (private backup)* is ticked) and restore it from a file. Without PINs, users keep their current PIN by name; a new user in the file needs a PIN and is refused otherwise. New messages `configExport`, `configImportBegin`, `configImport`.
- **My profile is a full editor**: all eight preferences, plus Add / Edit / Remove for filter sets and presets, using the normal editors aimed at that profile.
- Dialogs close when you click outside them and a dialog opened from another opens in front. Manage users has Name and PIN headers; Settings search is focused after the PIN unlocks; the hand lock screen has *Log in as…* and *Log out* at the bottom; fixed a wrong "must be a whole number" error on save and old errors now clear.
- Review fixes: one backdrop close in Filters (a profile draft no longer leaks into your own filter), the config import waits for an acknowledgement before sending the file, config export re-checks the settings PIN, quick preference changes in My profile accumulate, Manage users rows align.
- Design tweaks: the lock chip (even padding, square lock button, open/closed lock icon, slides in at the border's rate), Settings fixed bottom-left on desktop, the Presets bar laid out like Recent with a *Show all* chip, Edit and Filters styled like Settings, a divider above Recent, and the Recent bar and search always shown (*No history yet* when empty).
- Choose-takes bar is an accent bar with a large title, clears the lock chip and lights up rows under the pointer; the Settings save bar sits in front of fields scrolling under it; the lock chip's button no longer takes hover while it slides in and out.
- Small labels above every text box and select in the Filters dialog and the preset editor (name, Save for, PIN, the share link, and each take's Source, Destinations and Levels).
- The Filters dialog is laid out like the preset editor (picker, then name, Save for, Save, Save as…, Share, Delete, with the share link under the note) and a saved filter set can move scope with Save for too.
- Filters rename like presets: the Filters dialog has an editable name box and Save writes a new name with the rest (the Rename button is gone). The choose-takes bar says "Include follows" and "select", and the lock chip no longer shows as a grey box over it.
- Lock chip: the lock button and the closed-lock icon cross-fade with the lock screen instead of snapping, and the chip has no background while a lock screen is up.
- Lock chip: on unlock its background no longer fades in as a grey box under the lock button; it returns when the lock screen is gone.
- Presets: a saved preset can change scope (*Save for* is no longer fixed; Save copies it to the new scope, then removes it from the old one, with the settings PIN for Everyone unless trusted). *Fill from Recent…* puts Recent into a pick mode: tick takes or folders (optionally with their follows, which are likely router automation), then Add. The Show all chip has a rule after it, the popup's Close sits on the right, and *Copy link* turns green and says *Copied* in place.

- **Preset keys and order**: Option/Alt+1–9 fire the first nine pinned presets (hold Option/Alt to see ⌥1–9 on the chips), Alt+Shift+1–9 fill the patch boxes from one without taking (like Shift+click), and in the Presets popup Alt+P pins or unpins and Alt+↑/↓ or drag reorder the pins. The order is kept in the user's profile (`pinOrder`), else per browser. The popup lists pinned first, has no Share or Pin-for-everyone (both are in the editor) and links to the editor ("Preset Editor"; the editor links back with "Preset Organizer").
- **Add from Patch Boxes…** in the preset editor: build each take in the normal boxes and press Add (Enter) until Done, beside *Add from Recent…*; the editor's rows show their labels once and blank rows are ignored.
- **Source and Destination lists** open on focus with every item, the chosen one highlighted (Enter/Tab take a row only once you've picked or moved); the count and keys line is under the list.
- **Keyboard mode**: Ctrl+J/K (⌘J/K on a Mac) move like j/k; the shortcuts dialog is in sections.
- **Recent and Presets bars**: the titles have their own row; the filter boxes and the filter sets share one row, always side by side; sets and presets stay on one row with the first chip (All, Show all) pinned and the rest scrolling; preset chips are taller than filter chips; Sode tooltips on the chips and on Undo, Redo and Retry; the Filters button is accent-washed when filters are on and Clear stays, greyed out; the bottom-left shortcut tip is gone.
- **Windows and Linux**: letting go of a bare Alt no longer focuses the browser's menu bar while presets show, and Alt+1–9 no longer reaches the browser (unverified on a real Windows browser).
- **Phone**: the lock chip has no lock button and is hidden when it has no name; the lock screen's site name wraps.
- **Next** keeps going (#195): press Ctrl+Shift+S/D again and the source moves on again (Cam 2, Cam 3, …) and the destinations add the following group, for as long as the boxes still hold the last Next. The Next chips show the step the key would take.
- Fuzzy **Next** falls back to the next number when no higher name matches (#196).
- Internal: `keys.js` (`chord`, held Shift/Alt flags) and `presetkeys.js` (page-level preset keys), pin order in one place, a descriptor per pick mode, the match list's footer as its own element. Two flaky tests fixed.
- Review fixes: Filters *Save* now closes once a set for Everyone or Me has really been saved, and Cancel no longer rolls back a write that landed; Alt+N in the Presets popup fires once; Alt+1–9 waits while a pick bar is open; moving through a source or destination list with the arrows and pressing Enter, Tab or + takes the row; the open list is kept when names update; the full list is drawn a window at a time; Shift-fill leaves out what no longer exists (and never falls back to default levels); a failed pin reorder is put back and reported; the server keeps pin-order keys as the browser wrote them.

## Beta 1.5.0

- **Presets** (#152, renamed from "salvos" in #162): saved multi-source/multi-destination takes as buttons above Recent, per browser or for everyone (Settings PIN). Ctrl+G / ⌘G opens a searchable popup; pinning, an editor, and one history folder per fire.
- **Native fire mode** (#154): *Settings → Takes → Fire confirms of several takes* can send a multi-take confirm as one SW-P-08 group salvo (commands 120/121), about 4 ms apart on the Ultrix instead of ~16 ms. Falls back to one after another if the router doesn't acknowledge. New `fireMode` and `salvoGroup` settings.
- **Users and the lock screen** (#151, #158, #159): trusted users and persistent sessions, per-user PINs, auto-lock, a lock chip, Ctrl+L, and a lock screen that works without a PIN (churning-letter unlock). Sessions can be revoked and signed out everywhere.
- **Redo** (#153): Redo (or Retry for a failed take) beside Undo on every row and folder; `r` in keyboard mode.
- **Settings search**, and the **Grid** feature removed (#150).
- Fuzzy **Next** steps through letters as well as digits ("Prmpt A" → "Prmpt B") (#149).
- Small UI tweaks (#157), lock chip as plain text (#164), number stepper down arrow visible (#166), Presets popup keeps focus (#155, #156).
- **Native fire** now stages level 17 and numbers past 1023 too, with the extended stage command (248/250), so every take of a confirm can ride one group salvo (#154). Not yet seen live through Sode.
- **Users** moved out of the Settings page into their own dialog (Settings keeps a summary and a *Manage users…* button) and the layout is simpler (#165, #168).
- The lock button's tooltip no longer runs off the screen (#167).
- Security and sessions (found in review): a right user PIN no longer resets the shared wrong-PIN count (it is forgotten after an hour without a wrong one); a persistent user's session is renewed each time it is used or its page is still open, instead of expiring 90 days after login; the lock pad follows a PIN-length change at once; another browser's settings save can no longer wipe a valid session token.
- The user directory (names, PIN lengths, trusted and never-log-out flags) is no longer sent to every browser in the hello. A locked page learns only the PIN lengths in use; an unlocked page gets the names (for "by Ana" in history); the Users editor asks for its list with the settings PIN.
- **Trusted users** now also need the settings PIN to change the matrix, levels, level offset, name settings, fire mode and salvo group (and still the router, trusted proxies and PIN attempt limit).
- Docs: this changelog (#147) and an AI disclosure in the README (#121).
- **Releases**: versions are now `MAJOR.MINOR.PATCH` (this one is 1.5.0). Pushing a `-rc.N` tag builds a signed release candidate (`rc` image tag); a manual *promote* run turns it into the release without rebuilding. `latest` and `X.Y` only ever move forward and only by a promotion; every push to `main` publishes an unsigned `edge` image. Settings shows the build's short commit. See RELEASING.md.

**Upgrading:** an existing `config.yaml` loads as is: a `salvos:` list is read as `presets:` and written under the new name on the next save, and Grid settings are still ignored. Old history lines, and presets a browser saved under the old `sode-salvos` keys, load as before. Pull and restart with the same `/data` volume.

## Beta 1.4.1

Upgrades the vendored Patchbay design system from v1 to v2.1.0 (#146).

- UI scaling uses the root font size (125%, phone 100%) instead of `zoom`; sizes are rem.
- Built-in Patchbay tooltips, green `ok` buttons, code/kbd styling and phone layout replace local copies.
- Theme switching crossfades with a View Transition.
- Numbers stay in Plex Sans; hex and code use Plex Mono.

## Beta 1.4

Next chips for patching a run in order.

- A **Next** chip beside **Last** in each patch box; Ctrl+Shift+S / Ctrl+Shift+D fill Source and Destination with the last take's numbers moved on by one, every destination shifted together. Levels aren't carried.
- Settings → This device → **Next means**: the next number (default) or the next match of the name's text in router order.

Existing `config.yaml` loads as is (`nextMode` defaults to `number`).

## Beta 1.3.1 (includes 1.3)

Monitoring, a fuller /log, and stackable filter sets. Beta 1.3 was never published as a release of its own: 1.3.1 is 1.3 with a fixed flaky test and a faster image build, no code changes in Sode.

- `GET /api/health` for uptime monitors: 200 only while the router is up, else 503 (#140).
- /log: event rows, ACK times and a Router uptime graph kept 400 days (#139); Load older keeps reading until it finds shown rows (#141).
- WebSocket heartbeat every 5 s; a silent connection is dropped and a patch into a stale one is refused with "Not sent" (#142). Proxies must keep `/ws` open while idle for 15 s+.
- Filter-set chips in Recent toggle and stack; Settings → This device sets all-must-match or any (#131).
- Sources in a row's Before panel are buttons that use the source (#138).

The image now sets `SODE_UPTIME=/data/uptime.jsonl`; without the image's env or a `/data` mount, set it yourself or the uptime graph stays in memory.

## Beta 1.2

- Esc enters keyboard mode even with no history (#120); Tab follows Destination-first mode (#125).
- Protected destinations read "Protected by N" (#130); the lost-link banner is fixed across the top (#119).
- Levels box keeps its width for any level count (#135); /log message kinds are chips (#132).
- Logo text and an optional colour border per instance (#126), favicon (#127), GitHub and bug-report links in Settings (#123).
- Installable as an app (web app manifest and icons).

## Beta 1.1

First release published from CI: the ghcr image is multi-arch (linux/amd64 and linux/arm64) (#122).
