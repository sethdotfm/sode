# Panels

Sode Panels: saved 4x8 grid pages on `/panels`, `/panels/editor` and `/panels/<id>`, driven over `/api/v1` only. The design and what was built are in docs/panels-design.md; this note holds what bites. Names that are stored or sent are in docs/persisted-names.md, routes in docs/api.md and `internal/app/openapi.yaml`.

## Storage

All beside the config, under `/data` in the image.

- `panels/<id>.yaml`: one file per panel. `<id>.art/<art id>` holds its artwork and `<id>.checkpoints` its Push checkpoints (at most 10). `panelStore` (`panelstore.go`) owns all three; every write goes through `writeLocked`, which moves `Rev`.
- `panelsets/`: Checkpoint sets, `everyone.yaml` and `me-<hex of user>.yaml`.
- `templates/me/` and `templates/everyone/`: templates, apart from the panel files.
- `panelwatch.yaml`: who watches what, as watch keys: a panel id, or `<id>/<device lowercased>` for one screen (`watchKey` in `panelmon.go`; keep per-instance watches keyed by the device name, since #293 makes named instances first class). Never part of a panel file, and the API only sends the caller's own `watching` and `watchingInstances`.
- `yamlDir` (`yamldir.go`) is the mutex, directory, id retry, atomic write and name sort that the panel, template and Checkpoint set stores share; file names and skip rules stay in each store. Sets keep several records per file, so they use only part of it.
- A file whose id, name or directory disagrees with its content is skipped, not repaired.

## Keys

- A panel that acts as an API client has an `apiClients` entry with `panel` set to the panel id. It is made and removed with its panel and does not count toward the 12-client limit. A Trusted user may make one, with Rights no wider than their own (`Rights.Within`, ADR 0006). The cap is checked only when a save sets Rights to something new, so a panel an administrator widened keeps saving.
- Its key is a key-store entry named `Link`. Network limits are the client's `networks`, a CIDR list of at most 16, checked in the auth path so every route and the setup exchange are covered.
- A setup link is a `keys.json` record with `"setup": true`. It lasts one hour and works once, and the page trades it at `/api/v1/panels/{id}/setup`. Making, rotating or revoking a key forgets that panel's devices.
- A panel has an optional `slug` (unique, `validPanelSlug`). `panelStore` checks uniqueness under its lock. `GET /panels/{id}` and the setup route turn a slug into the id (`panelIDOf`); every other route takes the id only. Build links with `linkFor` (`web/panellink.js`), never by hand. Panel keys and setup tokens are 128 bits and the fragment drops their `sode_` prefix (`fullKey` and `fullSetup` put it back).
- The panel page reads `#key=` (API mode), a stored `sode-panel-key-<id>` (secure mode) or starts a session key (Session mode). A key never rides a query string.

## Browser session key

A Session-mode page trades its signed-in session for a user key over the socket: message `sessionKey`, answer `sessionKeyResult` (`internal/app/sessionkey.go`, `web/sessionkey.js`, rules in `sessionkeyrules.js`). The key is memory only, ends with the session, and `sessionFetch` adds the Bearer header and asks once more after `invalid_key` or `key_expired`. Use `sessionFetch` for every `/api/v1` call from a Sode page; never build the header by hand. For a JSON answer wrap it in `jsonCall(fetcher, { failed, down })` (`sessionkey.js`), which throws an Error with `status`, `code` and the server's message; a page with a URL key passes its own fetcher. Details are in docs/agents/users-locks.md.

## Seams

- REST: `panelsApp`, `userKey`, `makePanel`, `apiSend` and `apiSendRaw` in `panelsapi_test.go` run the real handlers against a fake router. Use them for any new route test.
- Web rules modules are pure and tested without a DOM: `panelmodel.js` (view, stage, Take), `panelrules.js`, `panelupdate.js`, `panelwatchrules.js`, `panelkeyrules.js`, `panelsrules.js`, `panelpushrules.js`, `gridmodel.js`, `editorrules.js` and `findrules.js`. Put new logic there and keep the DOM modules thin.
- Browser harness: docs/agents/browser-checks.md. Chrome checks use the real Handler and a built simulator, never `panelsApp`.

## Push, Rev and live update

`panelStore.applyLocked` writes the checkpoint, then the panel. A checkpoint that cannot be saved stops the Push. Open pages poll `GET /panels/{id}` every 5 seconds and compare `rev` (`panelupdate.js`, `panellive.js`), so a Push, an editor save and a restore all reach them through the same store write. Push and import never copy `acts`, `client` or `clientRights`, nor id, name, owner or `share`. A new route that stores a panel must not widen that list. The group for Push is worked out in the page (`pushGroups`); the server only sees ids.

## Monitoring

`panelmon.go` keeps devices in memory, so a restart alerts nothing. A device is dropped after 24 hours. A page holds `GET /panels/{id}/instances/stream` (a `fetch` with the Authorization header, in `panelkeepalive.js`), so a closed page is seen at once and lost 8 seconds later (`monGrace`); a device that never held a stream, or beat again with none open, keeps the 45 second keepalive miss (`monMiss`). A panel is online while any device counts. Notices go only to connections signed in as a watcher and not under the Lock, and are never in the hello. The Sode page shows a banner for the offline panels (`panelwatch.js`, the badge on pages without one). Its Clear menu sends `panelAlarm`: `clear` hides the outage for that user until the panel's next notice (memory only), `stop` removes the watch. The tests use `a.mon.now`, and `monHold` in `panelmonstream_test.go` for a real held connection. A watch is the whole panel or one device (`?instance=`); `watchersOf` decides who hears an event and an instance-only watcher gets the notice with `offline` false. `offlineFor` rows carry `instance` for those, and `panelAlarm` echoes it. A dead peer with no FIN is caught by `KeepAliveListener` (TCP keepalive, about 5 s) and then the 8 s grace; `TestChromeClosedPanelTabIsLostInsideFifteenSeconds` runs a closed tab on the real clock. The panels page polls every 2 s (`MON_POLL_MS`) instead of having a push channel.

## Gotchas

- Routes for sets and templates are `/panel-sets` and `/panel-templates`, not under `/panels/`. Go's mux reports a conflict between `DELETE /panels/templates/{id}` and `/panels/{id}/key`.
- The glossary scan rejects every word on a `_Avoid_` line, in docs and UI strings alike, and only a few are exempt in backticks. Check GLOSSARY.md before choosing a word. It fails only in the full `go test ./...`, because `TestDocsUseGlossaryWords` and `TestUIStringsUseGlossaryWords` scan the whole tree; a `-run` on one panel test will not show it.
- `panelRows` in `web/panelpage.js` is the rows of a grid. Its name collides with a retired phrase, so it has an `identAllow` entry; keep it when moving the code.
- Web modules are capped at 1000 lines (`imports.test.mjs`). The editor and panels page are close; split by concern before crossing.
- In a worktree the lint hook's `go vet` prints noise about the package path. It is not a real failure; run `go vet ./...` from the worktree root.
- Panel pages sit at different URL depths and use `<base href="../">`. They share the `panel` page lock.
- Status on panel pages is an icon first, with optional text. This exception is in AGENTS.md and nowhere else in Sode.
- The editor's level picker (`editorlevels.js`) is the main page's: `pickButtons` from `levelview.js` over `S.levels`, which the page's socket fills from the hello (not `/api/v1/tally`), with its CSS in `base.css`. The sidebar redraws it on the `hello` event.
- The Lock button bar is `position: fixed` top left on every page. `panels.css` pads `.sode-panels` under it (`data-sode-chip`) and hides it for a kiosk (`data-sode-kiosk`, set by `applyLook` in `panelrun.js`); keep the padding in step with the bar's size in `lock.css`.
