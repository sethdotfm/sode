# Infra guardrails Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make rollback safe and harden CI, release and lint before parallel feature branches start.

**Architecture:** Every config struct gets an inline `Extra` map, so keys a release doesn't know load, survive saves, and show as a problem instead of stopping startup. Changelog entries move to one file per PR so parallel branches can't conflict. CI gains a Go version drift check, SHA pins, timeouts, govulncheck and a race job.

**Tech Stack:** Go 1.27 stdlib, yaml.v3, GitHub Actions, ESLint 10.14.0 via npx.

**Spec:** the audit plan in `.scratch/audit-2026-11/audit-plan.md` (git-ignored) and the advisor report in this session. Decisions copied here are the spec of record.

## Global Constraints

- Dependencies stay stdlib + coder/websocket + yaml.v3. No `tool` directive; tools run as `go run <pkg>@<version>`.
- Comments give context only, one line where possible, no issue numbers (AGENTS.md, Wording).
- Use GLOSSARY.md terms in comments, docs and UI text (`TestRetiredWordsCoverGlossary`).
- `go test -short ./...` passes before every commit; the PostToolUse hook formats and lints.
- PR bodies and issue comments start with `DISCLOSURE: This was written by an LLM.`
- Work on branch `infra/guardrails` off main, one commit per task, pushed as you go. Open one draft PR at the start (with the user's OK): ci.yml runs only on PRs and on main, so Tasks 7, 8 and 10 can't be checked by a branch push alone.
- Check workflow edits locally with `go run github.com/rhysd/actionlint/cmd/actionlint@<latest tag> ` before pushing.
- No GitHub setting changes without the user's approval for that specific setting.
- Feature PRs never touch `internal/app/version.go`, the README Version line or a CHANGELOG.md heading; release PRs do.

## Review Focus

1. A hand-edited config with a misspelled key: the server starts, keeps the key, logs it, and shows it in the Communication Log with Problems only on (Task 4).
2. An unknown key inside a list item (`users[1].futureField`) survives load and save (Task 2).
3. A Settings form save keeps unknown keys at top level and inside `locks` (Task 3).
4. Importing a config file from a newer release keeps its unknown keys (Task 3).
5. A backup that can't be written (read-only dir entry, disk full) logs and lets the save go ahead (Task 5).
6. Importing a YAML file that isn't a Sode config (a panel file, a compose file) is refused and leaves settings unchanged, now that Parse accepts unknown keys (Task 3).

---

### Task 1: Changelog fragments and the version rule

**Files:**
- Create: `changelog.d/README.md`
- Modify: `internal/app/version_test.go:17-46`
- Modify: `RELEASING.md` (release steps), `AGENTS.md` (Version bullet), `CHANGELOG.md:3` (intro line)

**Interfaces:**
- Produces: the rule "each PR adds `changelog.d/<branch-or-slug>.md` holding one or more `- ` bullets in CHANGELOG style". Tasks 2 to 10 each add one.

- [ ] **Step 1: Write the failing test** `TestChangelogFragments` in `version_test.go`: every `changelog.d/*.md` other than README.md is non-empty and every non-blank line starts with `- `; with `SODE_RELEASE_TAG` set, any fragment fails with "fold changelog.d into CHANGELOG.md before tagging".
- [ ] **Step 2: Run** `go test ./internal/app -run 'TestChangelogFragments|TestVersionNoted' -v`. with a scratch fragment `changelog.d/x.md` and `SODE_RELEASE_TAG=v1.8.3` set. Expected: FAIL only once the test exists; without it nothing checks fragments. Delete the scratch fragment after Step 4.
- [ ] **Step 3: Implement.** `changelog.d/README.md` states the format in three lines. RELEASING.md gains the fold step: append fragment bullets under the new `## <Stage> <Version>` heading, `git rm changelog.d/*.md` except README. AGENTS.md Version bullet gains: feature PRs add a fragment and never bump. CHANGELOG.md intro names `changelog.d/`.
- [ ] **Step 4: Run** with the scratch fragment and the tag: FAIL with the fold message. Without the tag: PASS. Remove the scratch fragment: PASS either way.
- [ ] **Step 5: Commit** `changelog: one fragment file per PR`.

### Task 2: Config keeps keys it doesn't know

**Files:**
- Modify: `internal/config/config.go` (Config, DeviceDefaults, FilterSet, Parse), `internal/config/users.go` (User, Locks), `internal/config/api.go` (Rights, APIClient, APISettings), `internal/config/presets.go` (Preset, PresetItem), `internal/config/profile.go` (Profile and every struct type it holds)
- Test: `internal/config/extra_test.go`

**Interfaces:**
- Produces: field `Extra map[string]any \`yaml:",inline" json:"-"\`` on every struct type reachable from Config; on Config it also carries `owned:"true"`. `func (c Config) UnknownKeys() []string` returns sorted dotted paths, e.g. `futureKey`, `users[1].futureField`, `locks.futureLock`.
- Invariant: an `Extra` map is never mutated in place, so `Clone` may share it.

- [ ] **Step 1: Write the failing tests.**
  - `TestEveryConfigStructKeepsExtra`: reflect-walk Config through fields, slices, maps and pointers; every struct type in package config must have `Extra map[string]any` with yaml tag `,inline` and json tag `-`.
  - `TestUnknownKeysRoundTrip`: parse YAML with `futureKey: 1`, `users: [{name: a, futureField: x}]`, `locks: {futureLock: true}`; `Parse` returns no error; `UnknownKeys()` equals `[]string{"futureKey", "locks.futureLock", "users[0].futureField"}`; `Save` then `Load` returns the same three keys with the same values.
  - `TestLegacyKeysAreNotUnknown`: every legacy key Config keeps a field for (the `Legacy*` fields and `levelTypedBase`) loads into that field and `UnknownKeys()` is empty.
- [ ] **Step 2: Run** `go test ./internal/config -run 'Extra|UnknownKeys|LegacyKeys' -v`. Expected: FAIL, Parse errors "field futureKey not found".
- [ ] **Step 3: Implement.** Add the `Extra` fields. Keep `dec.KnownFields(true)`: yaml.v3 sends leftover keys to an inline map before its known-fields check, so a struct with `Extra` accepts them. If Step 4 shows otherwise, drop KnownFields and stop; report back before going further. Update the Parse comment (unknown keys are kept, see `UnknownKeys`). `UnknownKeys` walks with reflect, using yaml tag names for path segments.
- [ ] **Step 4: Run** the Step 2 command, then `go test ./internal/config`. Expected: PASS, including `TestEveryFieldChooses`.
- [ ] **Step 5: Commit** with a fragment: "Settings keep keys this release doesn't know, so rolling back no longer stops startup or loses newer settings."

### Task 3: Settings saves and imports keep unknown keys; imports refuse non-config files

**Files:**
- Modify: `internal/app/configfile.go:105-125` (`ImportConfig`)
- Modify: `docs/agents/settings-saves.md` (Owned fields bullet)
- Test: `internal/app/settings_extra_test.go`

**Interfaces:**
- Consumes: `Extra` and `UnknownKeys()` from Task 2.
- `saveSettings` decodes browser JSON over a clone of live (settings.go:31-33) and `Extra` is `json:"-"`, so form saves should keep every `Extra` with no new code. The app test proves it; add code only if it fails.
- Known limit, write it in settings-saves.md: a list item an older release rewrites from browser JSON (an edited preset or filter set) loses the keys only the newer release knows.

- [ ] **Step 1: Write the failing tests.**
  - `TestSettingsSaveKeepsUnknownKeys`: start an App from a config file containing `futureKey: 1` and `locks: {futureLock: true}`; do a Settings form save that changes `routerName`; reload the file; `UnknownKeys()` still lists both.
  - `TestImportKeepsUnknownKeys`: `ImportConfig` with a valid config that also has `futureKey: 2`; the saved file keeps it.
  - `TestImportRefusesNonConfig`: `ImportConfig` with a panel file's YAML (`id: p1`, `cells: []`) returns an error naming the unknown keys, and the saved config is byte-identical to before.
- [ ] **Step 2: Run** `go test ./internal/app -run 'TestSettingsSaveKeepsUnknownKeys|TestImport(Keeps|Refuses)' -v`. Expected: `TestImportRefusesNonConfig` FAILS; the other two are expected to pass already.
- [ ] **Step 3: Implement** in `ImportConfig`: refuse a file whose top level has no key Config knows, with `not a Sode configuration: no settings this release knows (found: <keys>)`. When some keys are known and some aren't, import it and name the unknown ones in the reply. Write the settings-saves.md lines.
- [ ] **Step 4: Run** the Step 2 command, then `go test -short ./...`. Expected: PASS.
- [ ] **Step 5: Commit**.

### Task 4: Show unknown keys as a problem

**Files:**
- Modify: `internal/app/app.go:236-240` (`Run`, after the server start event)
- Modify: `cmd/sode/main.go:31` (log after Load)
- Test: `internal/app/unknownkeys_test.go`

**Interfaces:**
- Consumes: `UnknownKeys()`.
- Produces: one Communication Log row, `event(kindServer, "", "", "start")` style, with `Err` = `config.yaml has keys this release doesn't know, kept as is: <comma list>`. `Err` puts it under Problems only (web/log.js problem filter). main logs the same text once.

- [ ] **Step 1: Write the failing test** `TestUnknownKeysLogged`: an App whose config has `futureKey`; after Run starts, the Communication Log holds a server row whose `Err` contains `futureKey`. A config with no unknown keys adds no such row.
- [ ] **Step 2: Run** `go test ./internal/app -run TestUnknownKeysLogged -v`. Expected: FAIL.
- [ ] **Step 3: Implement** in `Run` from `a.r().cfg.UnknownKeys()`, and the main.go log line.
- [ ] **Step 4: Run** the Step 2 command. Expected: PASS.
- [ ] **Step 5: Commit**.

### Task 5: Config backups

**Files:**
- Modify: `internal/config/config.go` (`Save`)
- Modify: `cmd/sode/main.go` (startup, and the reset-pin path)
- Test: `internal/config/backup_test.go`

**Interfaces:**
- Produces: `Save` copies the current file to `<path>.prev` before the write. `func BackupOnce(path, label string) error` in config writes `<path>.before-<label>` only if absent; main calls it with `app.Version` right after each successful `config.Load` (main.go:31 and the reset-pin path at main.go:67), before any `config.Save` (main.go:55, main.go:78).
- Backups hold PIN hashes: write them 0600 (AtomicWrite's temp file already is; assert it).
- A backup error is logged and never blocks the save or startup.

- [ ] **Step 1: Write the failing tests.**
  - `TestSaveKeepsPrev`: Save A, Save B; `<path>.prev` holds A, `<path>` holds B.
  - `TestBackupOnce`: the first call writes `.before-1.8.4`; a second call after changing the file leaves the backup unchanged.
  - `TestBackupsArePrivate`: both backup files have mode 0600.
  - `TestSaveWhenPrevFails`: make `<path>.prev` a directory; Save B still succeeds and writes B.
- [ ] **Step 2: Run** `go test ./internal/config -run 'Prev|BackupOnce' -v`. Expected: FAIL.
- [ ] **Step 3: Implement.** Use `AtomicWrite` for both backups. Add `config.yaml.prev` and `config.yaml.before-*` to docs/reference.md where /data files are listed, with one line that they hold PIN hashes.
- [ ] **Step 4: Run** the Step 2 command, then `go test -short ./...`. Expected: PASS.
- [ ] **Step 5: Commit** with a fragment naming the backup files.

### Task 6: Go 1.27 and a drift check

**Files:**
- Modify: `go.mod` (`go 1.27`)
- Test: `internal/app/toolchain_test.go`

- [ ] **Step 1: Write the failing test** `TestDockerfileGoMatchesGoMod`: read `../../go.mod` (the `go` directive's major.minor, accepting `1.27` and `1.27.0`) and `../../Dockerfile` (the `golang:X.Y` tag); fail with both values if they differ.
- [ ] **Step 2: Run** `go test ./internal/app -run TestDockerfileGoMatchesGoMod -v`. Expected: FAIL, 1.24 vs 1.27.
- [ ] **Step 3: Implement.** `go mod edit -go=1.27`, `go mod tidy`, `go vet ./...`.
- [ ] **Step 4: Run** `go test ./...` in the background (several minutes). Expected: PASS. Fix new vet findings in place.
- [ ] **Step 5: Commit**.

### Task 7: Workflow hardening

**Files:**
- Modify: `.github/workflows/claude.yml`, `.github/workflows/claude-code-review.yml`, `ci.yml`, `rc.yml`, `promote.yml`

- [ ] **Step 1: Pin** `actions/checkout@v7` and `anthropics/claude-code-action@v1` in both claude workflows to commit SHAs with a `# vX.Y.Z` comment, matching ci.yml's style. Get each SHA with `gh api repos/<owner>/<repo>/commits/<tag> --jq .sha`. Keep `id-token: write`; the action needs it for OAuth.
- [ ] **Step 2: Add `timeout-minutes`** to every job: changes 5, lint 10, go 25 (its test timeout is 20m), browser 20, leak 15, ci-ok 5, edge 20, rc 30, promote 20, claude jobs 15. Keep ci.yml's main-branch cancel (its comment explains why `edge` needs it).
- [ ] **Step 3: Add concurrency.** claude-code-review.yml: workflow level, group per PR, `cancel-in-progress: true`. claude.yml: job level (a job skipped by its `if:` then doesn't join the group), group per PR or issue number, `cancel-in-progress: false`, so an unrelated comment never cancels a running job. rc.yml and promote.yml already have `cancel-in-progress: false`; leave them.
- [ ] **Step 4: Verify** with actionlint, then push to the draft PR; ci passes and `ci-ok main` reports.
- [ ] **Step 5: Commit**.

### Task 8: govulncheck

**Files:**
- Create: `.github/workflows/vuln.yml`

- [ ] **Step 1:** Find the latest version with `go list -m -versions golang.org/x/vuln | tr ' ' '\n' | tail -1`, then run `go run golang.org/x/vuln/cmd/govulncheck@<that version> ./...` locally. Record the result. Fix or file anything it reports before the workflow goes in.
- [ ] **Step 2:** Write vuln.yml: on `pull_request`, `push` to main and `schedule` weekly (Monday 06:00 UTC); `permissions: contents: read`; SHA-pinned checkout and setup-go matching ci.yml; `go-version-file: go.mod`; timeout 10; the same `go run ...@<version>` command. Not part of `ci-ok`, so a new stdlib advisory doesn't block unrelated PRs; the weekly run reports it.
- [ ] **Step 3: Verify** with actionlint and a push to the draft PR; the vuln job is green.
- [ ] **Step 4: Commit**.

### Task 9: ESLint rules

**Files:**
- Modify: `eslint.config.mjs`

**Blocked by:** #364 for `no-unused-expressions` (users.js:38 fails it). If #364 is still open, land the other rules and leave a one-line note in the commit.

- [ ] **Step 1:** Add `eqeqeq`, `no-fallthrough` and `no-unused-expressions` as errors. Run `npx --yes eslint@10.14.0 web`. Expected: clean apart from users.js:38 while #364 is open.
- [ ] **Step 2:** Add a `web/*rules.js` block with `no-restricted-globals` for `document`, `window`, `localStorage`, `sessionStorage`, `navigator`, `location`.
- [ ] **Step 3:** Add per-file `no-restricted-imports`: `web/keys.js` imports nothing; `web/core.js` imports only `./tooltip.js` and `./sprite.js` (check docs/agents/web-layout.md for the exact rules first and copy them).
- [ ] **Step 4: Run** `npx --yes eslint@10.14.0 web` and `node --test 'web/test/*.test.mjs'`. Expected: clean, PASS. Then break one rule on purpose to confirm it fires, and revert.
- [ ] **Step 5: Commit**, and add one line to docs/agents/web-layout.md saying ESLint enforces these rules.

### Task 10: Race job

**Blocked by:** #365.

**Files:**
- Modify: `.github/workflows/ci.yml` (new `race` job, added to `ci-ok`'s needs)

- [ ] **Step 1:** Run `go test -short -race -count=1 ./...` locally in the background. Expected: PASS once #365 is fixed. Record the wall time.
- [ ] **Step 2:** Add job `race`: same setup as `go`, `go test -short -race -count=1 -timeout 15m ./...`, timeout 20. Add it to `ci-ok`.
- [ ] **Step 3: Verify** with a push to the draft PR: race is green and `ci-ok` waits for it.
- [ ] **Step 4: Commit**, and comment on #233 that the job is back.

### Task 11: GitHub settings and the review bot (the user's)

Each line needs the user's yes before it runs, or the user does it in the GitHub UI. Commands for reference, one per approval:

- [ ] Branch protection on main: `required_status_checks.strict = true`, `enforce_admins = true`. First confirm no workflow pushes commits to main (`grep -n "git push" .github/workflows/*.yml`).
- [ ] Repo: `allow_auto_merge = true`, `delete_branch_on_merge = true`.
- [ ] Tag ruleset for `v*`: block deletion and force-push.
- [ ] Release environment: the user as required reviewer.
- [ ] Dependabot alerts and security updates on.
- [ ] Review bot: open a draft PR with a planted bug (an off-by-one in a test-only helper) and see whether claude-code-review comments. If not, fix its prompt or delete the workflow.

### Release handoff

Tasks 2 to 5 protect `config.yaml` only, and only on rollback to a release that contains them. Cut a release with this branch before any feature adds a config key (RELEASING.md; the release PR folds `changelog.d/` per Task 1). Tasks 6 to 10 add no config keys and can ship in the same release. New profile prefs, API scopes, enum values and edited list items (presets, filter sets, profile lists) are not covered, so a feature PR that adds one must say in its description what a rollback does.

The other persisted files (keys.json, tokens.json, templates, art defaults, icons, History) still drop fields an older release doesn't know when it rewrites them; panel files already keep `Extra`. Until a follow-up gives them the same rule, a feature that adds a field to one of them must say in its PR what a rollback loses.
