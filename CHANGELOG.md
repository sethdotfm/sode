# Changelog

Newest first. Each release also has notes on [GitHub](https://github.com/sethdotfm/sode/releases). Give it a section for each release when you bump `Version` (see README, Releasing); `TestVersionNoted` checks the heading.

## Beta 1.5 (unreleased)

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
