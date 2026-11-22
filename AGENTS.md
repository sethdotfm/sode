# AGENTS.md

Notes for AI agents working in this repo. Only the non-obvious is here; code and git history hold the rest.

Sode: keyboard-first web patching for one SW-P-08 router (Ross Ultrix). Settings table and protocol summary are in docs/reference.md.

## Commands

```
go test -short ./...   (fast group, about 25s: run before every commit)
go test ./internal/app -run TestName -v   (while editing)
go test ./internal/app -run '^TestChromeName$' -v   (only the browser checks you added or changed; CI runs the rest)
go test ./...   (everything, several minutes: only when touching the browser harness, swp08sim or timing code; use run_in_background)
```

Develop fast: do not run area-wide or full browser suites locally, push and let CI run them (owner's rule). Pass -timeout (10m for the full run) so a hang fails early. Rerun a failing browser check alone before you call it a regression.

```
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...   (2026.2.2; CI runs it in lint, so it must be clean)
node --test 'web/test/*.test.mjs'
go run ./cmd/swp08sim   (simulator on :8910, SIM_* env in cmd/swp08sim and docs/reference.md)
SODE_ROUTER=127.0.0.1:8910 SODE_DEBUG=true go run ./cmd/sode   (:8080, SODE_DEBUG hex-dumps SW-P-08)
docker compose --profile sim up --build
```

Format and lint run as a PostToolUse hook (.claude/settings.json, .claude/hooks/lint.sh). On .go: gofmt + go vet. On web/*.js, web/test/*.mjs, web/*.css: Prettier 3.3.3 through npx (config .prettierrc.json, no package.json) + node --check. On web/*.js and web/test/*.mjs: ESLint 10.14.0 through npx (eslint.config.mjs, `no-undef` and `no-unused-vars` only, browser globals listed by hand). CI's lint job fails on drift: `gofmt -l`, `prettier --check` on the same globs, and `npx --yes eslint@10.14.0 web`. Never `eslint .`, since .claude/worktrees holds a repo copy. The first npx run per machine downloads ESLint and can look hung. Exit 2 reports back. Do not hand-format. Web modules are capped at 1000 lines (imports.test.mjs); split a module before it crosses the cap.

Test groups: fast is what `-short` runs. Browser checks are named `TestChrome*`, since CI selects them by name; browserTest fails any other name that starts Chrome. Leak checks are `TestChrome*HoldsNoMemory`. Slow non-browser tests call slowTest(t); add it to any test over about a second. internal/app's TestMain calls config.UseTestCosts (1000 PBKDF2 rounds, no fsync), so a test that needs a slow PIN check stores a full-cost hash itself (lock_held_test.go). CI (ci.yml): `lint`, `go` (all non-browser), `browser` (3 shards by `-list`, failures rerun once and reported as flaky), `leak`. PRs into integration/* and docs-only PRs run lint plus `-short`. `ci-ok main` is the one required check. A flaky warning in CI is a bug to fix, not to ignore.

Without SODE_CONFIG, config comes from SODE_* env and is not persisted. The web UI is go:embed (web/web.go): restart after frontend edits.

## Key decisions

- Go, stdlib-first. Deps only coder/websocket and yaml.v3; no new deps or frameworks without asking. Distroless image, HTTP :8080 (TLS is the user's proxy).
- SW-P-08 over TCP to one router, single matrix, always extended commands (16-bit src/dst). Protected destinations are refused before sending. Auto-reconnect and keepalive. Names come from the router, re-polled (default 60s).
- Levels: wire = level + offset. Internally levels are UI 1..N everywhere. levelNumbering (1, 0, va) only changes display (Level.Label) and typing (parseLevels in levelview.js). No free-text level names (user asked for removal).
- Config lives in /data/config.yaml, edited in Settings. A config path with no users runs the setup wizard (docs/agents/server.md), whose Finish writes the file; SODE_* env only fills the wizard. Save stays open; leaving with unsaved server settings warns once, a second try discards.
- Frontend: vanilla ES modules, no build. Follow patchbay rules exactly (https://github.com/sethdotfm/patchbay llms.txt): pb- classes only, tokens only, no Bootstrap utilities, status always with text. Vendored in web/vendor/patchbay (VERSION file, v2.2.1); copy missing icons from upstream, never fake them. The one exception: Sode-owned icons (`sode-` names, owner-approved) live in web/sodeicons.svg, outside the vendor folder; today that is `sode-grid`. UI scales with html font-size 100% (the Grid panel page and its Live Editor 90%; phone 100% everywhere), never zoom. Above 2400 px the desktop root grows by 1px per 360 px, to 125% at 3840 px (Grid panel page 112.5%, docs/agents/web-layout.md). Sizes in rem; numbers use --pb-font-num, --pb-font-mono is for code/hex; no per-theme CSS. Tooltips are data-pb-tip (+ pb-tip--end); green buttons pb-btn--ok; icon-only ghost buttons need pb-btn--icon; layout.js sets data-pb-layout="phone".
- Big scroll bars: where a user must scroll, give a big scroll bar that always shows, drawn by web/bigscroll.js. Users work with trackpads, touch screens, trackballs or a pointer with no wheel, often under pressure. Native bars are not enough, since Firefox cannot size them and macOS hides them. Big is the default size; use little only for small secondary lists (a match list in a small box, the filter sets row). Add a new scroller to `SCROLLERS` (docs/agents/web-layout.md). The phone layout keeps native bars.
- Comments: context only (why, invariants, quirks, keep-in-step links); never restate the code, no issue numbers (git history has them).
- Panel pages (`/panels`, `/panels/editor`, `/panels/<id>`) show status as an icon with optional text, an exception to patchbay's "status always with text" (docs/panels-design.md). Nothing else in Sode gets it. They share the `panel` page lock (`locks.panel`) and sit at different URL depths, so their HTML uses `<base href="../">`.
- The hello reaches locked pages, so it carries each user's name and PIN length (`users`, 0 for no PIN) and nothing else: hello() blanks cfg.users. The Users editor asks usersList. Keep new user fields out of anything sent before the PIN is entered (TestUsernamesStayOutOfPublicMessages).
- Confirm dialog: churning random letter plus lockout (default 500ms, Enter optional).
- Version (#95): Version and Stage in internal/app/version.go, sent in hello as BuildLabel(), shown in Settings, noted in README's Version line. Every release bumps both and adds a CHANGELOG.md section (#147). A PR with a user-visible change adds a `changelog.d/<slug>.md` fragment and never bumps. TestVersionNoted fails on mismatch, or with SODE_RELEASE_TAG if the tag differs or the heading says unreleased. See RELEASING.md. Only promote.yml is allowed to push the latest image tag.

- Writing: all docs, UI text and comments follow docs/ste.md (based on ASD-STE100). GLOSSARY.md has priority over it. internal/app/ste_test.go checks it; `STE_REPORT=<file>` writes the hit list.
- Vocabulary: use GLOSSARY.md's terms in code comments, docs and UI text. TestRetiredWordsCoverGlossary and the scans in internal/app/vocabulary_test.go fail on any _Avoid_ term used in docs or UI strings unless its row names the pending ticket. docs/research/ is skipped, since it quotes outside sources. A new _Avoid_ term needs a row; a finished sweep clears its ticket. TestIdentifiersUseGlossaryWords applies the same rows to web identifiers and file names (strings, so wire types and `sode-*` keys, are out of scope). A kept name goes in identAllow with its reason. A route is the per-destination unit, so names for one say route, not take.
- v2 reads no v1 files (docs/agents/server.md, config load). config.yaml carries `format: 2`, and a v1 install moves to `/data/v1/` on start. The first v2 page load clears v1 `sode-*` keys. A page reloads when the hello's build label changes. There are no legacy read paths.
- Kept old names: persisted and sent names follow GLOSSARY.md. v2 renamed the v1 wire, config, env, simulator and `localStorage` names; docs/persisted-names.md lists old and new. Name kept names in backticks. A new rename of a persisted or sent name needs the issue owner's approval. TestAllowlistsMatchSomething fails on a vocabAllow or identAllow entry that exempts nothing, so drop an entry when its name goes.
  - `salvoGroup`, `fireMode` `native` and the `salvo*` request kinds are the router's own salvo and are correct.

## More notes (read the file when your task touches it)

- docs/agents/settings-saves.md: Settings and saves. Read before touching device defaults, Settings search, the one Save and its sources (Manage users, API), data-apart or the Settings form.
- docs/agents/server.md: Server architecture. Read before touching the SW-P-08 client, takes, check, undo, failures, late answers, HTTP routes, the WebSocket, config load or the simulator.
- docs/agents/web-layout.md: Web layout. Read before touching web modules and import rules, the two layouts, keyboard mode and key plumbing, Last and Next chips, socket freshness or source ends.
- docs/agents/history.md: History, ext and External follow. Read before touching History, undo, ext/found entries, External follow entries, filters and filter sets, stacked sets, Redo, quick search.
- docs/agents/users-locks.md: Users and locks (#151). Read before touching users, PINs, page locks, the policy table, sessions and tokens, lock screen, tab sign-in, the Lock.
- docs/agents/profiles.md: Profiles (#161), guests (#187), sharing (#169). Read before touching profiles, guests, the profile editor, export/import, Item links, server config export/import.
- docs/agents/comm-log.md: Communication Log (#72, /log). Read before touching /log page, Communication Log rows, event rows, uptime graph.
- docs/agents/router-verified.md: Verified against the real Ultrix (2026-10-22, 172.17.60.101:8910). Read before touching what was measured on the real Ultrix and the protocol quirks Sode depends on.
- docs/agents/browser-checks.md: Browser checks (headless Chrome against swp08sim, `go test ./internal/app -run TestChrome`). Read before adding or changing one.
- docs/agents/presets.md: Presets. Read before touching presets, Bookmarks and the Bookmarks strip, the editor and popup, preset keys.
- docs/agents/salvos.md: Native fire mode (#154). Read before touching fireMode native, the router's own salvos and the simulator's native-mode options.
- docs/agents/panels.md: Panels storage, keys, the browser session key, seams, Push and live update, monitoring and gotchas. Read before touching panel files, panel API clients and keys, `web/sessionkey.js`, Push or the panel pages' tests. Read it also for a Grid panel (`web/gridpanel*.js`, `panelgrid.go`, the editor's Sources and Destinations tabs).
- docs/panels-design.md: Sode Panels (4x8 grid pages on `/panels`, driven over `/api/v1`). Read before touching panels (Buttons or Grid), the panel editor, panel storage or panel sign-in. Prototypes live only on branch `prototype/panels`, never main; panels-design.md names each one's server and port. ADR 0006 covers panel API clients.

## Agent skills

### Issue tracker

Issues live in GitHub Issues for sethdotfm/sode, through the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default five-label vocabulary (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
