# AGENTS.md

Notes for AI agents working in this repo. Only the non-obvious is here; code and git history hold the rest.

Sode: keyboard-first web patching for one SW-P-08 router (Ross Ultrix). Settings table and protocol summary are in docs/reference.md.

Terms: use the words in GLOSSARY.md and avoid its _Avoid_ words in code comments, docs and UI text; TestDocsUseGlossaryWords and TestUIStringsUseGlossaryWords enforce it. Wire message types, config keys and localStorage keys keep their old names, so name them in backticks.

## Commands

go test ./...   (about 70s; Dockerfile build runs it too; run it and docker builds with run_in_background)
go vet ./...
go test ./internal/app -run TestName -v
node --test 'web/test/*.test.mjs'
go run ./cmd/swp08sim   (simulator on :8910, SIM_* env in cmd/swp08sim and docs/reference.md)
SODE_ROUTER=127.0.0.1:8910 SODE_DEBUG=true go run ./cmd/sode   (:8080, SODE_DEBUG hex-dumps SW-P-08)
docker compose --profile sim up --build

Format and lint run as a PostToolUse hook (.claude/settings.json, .claude/hooks/lint.sh): gofmt + go vet on .go; Prettier 3.3.3 via npx (config .prettierrc.json, no package.json) + node --check on web/*.js, web/test/*.mjs, web/*.css. Exit 2 reports back. Don't hand-format; web modules are capped at 1000 lines (imports.test.mjs); split before crossing it.

Without SODE_CONFIG, config comes from SODE_* env and is not persisted. The web UI is go:embed (web/web.go): restart after frontend edits.

## Key decisions

- Go, stdlib-first. Deps only coder/websocket and yaml.v3; no new deps or frameworks without asking. Distroless image, HTTP :8080 (TLS is the user's proxy).
- SW-P-08 over TCP to one router, single matrix, always extended commands (16-bit src/dst). Protected destinations are refused before sending. Auto-reconnect and keepalive. Names come from the router, re-polled (default 60s).
- Levels: wire = level + offset. Internally levels are UI 1..N everywhere. levelNumbering (1, 0, va) only changes display (Level.Label) and typing (parseLevels in levelview.js). levelTypedBase is dropped but kept so old files load. No free-text level names (user asked for removal).
- Config lives in /data/config.yaml, edited in Settings. SODE_* env only seeds the file on first start. Save stays open; leaving with unsaved server settings warns once, a second try discards.
- Frontend: vanilla ES modules, no build. Follow patchbay rules exactly (https://github.com/sethdotfm/patchbay llms.txt): pb- classes only, tokens only, no Bootstrap utilities, status always with text. Vendored in web/vendor/patchbay (VERSION file, v2.2.0); copy missing icons from upstream, never fake them. UI scales with html font-size 125% (phone 100%), never zoom; sizes in rem; numbers use --pb-font-num, --pb-font-mono is for code/hex; no per-theme CSS; tooltips are data-pb-tip (+ pb-tip--end); green buttons pb-btn--ok; icon-only ghost buttons need pb-btn--icon; layout.js sets data-pb-layout="phone".
- Comments: context only (why, invariants, quirks, keep-in-step links); never restate the code, no issue numbers (git history has them).
- Panel pages (`/panels`, `/panels/editor`, `/panels/<id>`) show status as an icon with optional text, an exception to patchbay's "status always with text" (docs/panels-design.md). Nothing else in Sode takes it. They share the `panel` page lock (`locks.panel`) and sit at different URL depths, so their HTML uses `<base href="../">`.
- Users never ride the hello (it reaches locked pages): hello() blanks cfg.users. Names come in lockState.names (only once the connection has access, S.userNames), PIN lengths in lockState.lens, the Users editor asks usersList. Keep new user fields out of anything sent before the PIN is entered.
- Confirm dialog: churning random letter plus lockout (default 500ms, Enter optional).
- Version (#95): Version and Stage in internal/app/version.go, sent in hello as BuildLabel(), shown in Settings, noted in README's Version line. Every release bumps both and adds a CHANGELOG.md section (#147). TestVersionNoted fails on mismatch, or with SODE_RELEASE_TAG if the tag differs or the heading says unreleased. See RELEASING.md. Only promote.yml may push the latest image tag.

- Vocabulary: GLOSSARY.md fixes the terms. TestRetiredWordsCoverGlossary and the scans in internal/app/vocabulary_test.go fail on any _Avoid_ term used in docs or UI strings unless its row names the pending ticket. A new _Avoid_ term needs a row; a finished sweep clears its ticket. TestIdentifiersUseGlossaryWords applies the same rows to web identifiers and file names (strings, so wire types and `sode-*` keys, are out of scope); a kept name goes in identAllow with its reason. A route is the per-destination unit, so names for one say route, not take.
- Kept old names: nothing persisted or sent was renamed for the glossary, since old files, deploy env and open pages hold them. Full table in docs/persisted-names.md; a rename needs the issue owner's approval, a load-old-files path and a fixture test. TestAllowlistsMatchSomething fails on a vocabAllow or identAllow entry that exempts nothing, so drop an entry when its name goes.
  - Config and env: `settingsPin` (the Advanced Settings PIN hash), `nameRefreshSeconds` / `SODE_NAME_REFRESH_S`, `deviceDefaults`, `SODE_SENTLOG*` (Communication Log file), `SIM_PANEL_*` and `SIM_FOLLOWS` (simulator). `salvos` and history's `salvo` are read-only legacy; `sode-salvos` and `sode-salvo-pins` are migrated once.
  - `localStorage`: `sode-device-defaults` (web/prepaint.js reads it before first paint).
  - Wire: `batch`, `recent` (also the `hello` field), `refreshNames`, `sent`/`sentAdd`/`sentPage`, `unlock`/`unlockResult`, `userUnlock`/`userUnlockResult`, `protect` (also the `hello` field), `locked`. The server logs `refreshNames`, `unlock` and `userUnlock` as `why` values in Communication Log rows, so those also stay.
  - `salvoGroup`, `fireMode` `native` and the `salvo*` request kinds are the router's own salvo and are correct.

## More notes (read the file when your task touches it)

- docs/agents/settings-saves.md: Settings and saves. Read before touching the Advanced Settings PIN, device defaults, Settings search, saveShared, owned fields, data-apart or the Settings form.
- docs/agents/server.md: Server architecture. Read before touching the SW-P-08 client, takes, check, undo, failures, late answers, HTTP routes, the WebSocket, config load or the simulator.
- docs/agents/web-layout.md: Web layout. Read before touching web modules and their import rules, the two layouts, keyboard mode and key plumbing, Last and Next chips, socket freshness or source ends.
- docs/agents/history.md: History, ext and External follow. Read before touching History, undo, ext/found entries, External follow entries, filters and filter sets, stacked sets, Redo, quick search.
- docs/agents/users-locks.md: Users and locks (#151). Read before touching users, PINs, page locks, the policy table, sessions and tokens, lock screen, tab sign-in, the Lock.
- docs/agents/profiles.md: Profiles (#161), guests (#187), sharing (#169). Read before touching profiles, guests, the profile editor, export/import/share links, server config export/import.
- docs/agents/comm-log.md: Communication Log (#72, /log). Read before touching /log page, Communication Log rows, event rows, uptime graph.
- docs/agents/router-verified.md: Verified against the real Ultrix (2026-10-22, 172.17.60.101:8910). Read before touching what was measured on the real Ultrix and the protocol quirks Sode depends on.
- docs/agents/browser-checks.md: Browser checks (headless Chrome against swp08sim, `go test ./internal/app -run TestChrome`). Read before adding or changing one.
- docs/agents/presets.md: Presets. Read before touching presets, pinning and pin order, the editor and popup, preset keys.
- docs/agents/salvos.md: Native fire mode (#154). Read before touching fireMode native, the router's own salvos and the simulator's native-mode options.
- docs/panels-design.md: Sode Panels (4x8 grid pages on `/panels`, driven over `/api/v1`). Read before touching panels, the panel editor, panel storage or panel sign-in. The prototype is only on branch `prototype/panels`, never main: `git switch prototype/panels && node prototype/panels/serve.mjs` serves variants A, B and C on :8101 to :8103 with stub data and no router. Panel status is an icon with optional text, an exception to patchbay's "status always with text". The editor prototype is `git switch prototype/panels && node prototype/panels/serve-editor.mjs` (variant F on :8116 is the chosen one). The panels page prototype is `node prototype/panels/serve-overview.mjs` (variant C on :8123). ADR 0006 covers panel API clients.

## Agent skills

### Issue tracker

Issues live in GitHub Issues for sethdotfm/sode, through the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default five-label vocabulary (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
