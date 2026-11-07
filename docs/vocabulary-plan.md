DISCLOSURE: This was written by an LLM.

# Vocabulary plan

Brings the repo in line with `GLOSSARY.md`. Two tracks. Track 1 renames words and changes no behavior. Track 2 changes behavior, one issue each, and stays out of Track 1's commits.

## Track 1: vocabulary sweep

One branch, one commit per step, so each step can be reverted alone. Run `go test ./...` and `node --test 'web/test/*.test.mjs'` after each step.

File counts below are case-insensitive greps (vendor, CHANGELOG and GLOSSARY excluded) and overcount. "salvo" includes the router's own salvo and the native fire mode, which stay.

| Step | Scope | Notes |
| --- | --- | --- |
| 1. Docs | `AGENTS.md`, `docs/reference.md`, `docs/agents/*.md`, `README.md` | Point `AGENTS.md` at `GLOSSARY.md` for terms. Keep AGENTS.md's rule that it holds only the non-obvious. |
| 2. UI strings | `web/*.html`, `web/*.js` display text | Recent to History, panel to External, This device to This browser, Settings PIN to Advanced Settings PIN, log in/out to Sign in/out, lock names per glossary, sent log to Communication Log. |
| 3. Tests and index | `web/test/*`, `settingsindex.js` DICT | Add the new This browser slug and search terms, or `settingsindex.test.mjs` fails. |
| 4. Code identifiers | Go and JS names | `takeResult`, `takeLevel`, per-destination `take()` become route names. `batch` becomes take. Keep wire message types stable (see step 5). |
| 5. Persisted and wire names | `config.yaml` keys, `history.jsonl` fields, `sode-*` localStorage keys, WebSocket types | Default is to keep them. Rename one only if old files still load, as `levelTypedBase` and `grids:` did. Write the exceptions into AGENTS.md. |
| 6. Release | `internal/app/version.go`, README Version line, CHANGELOG section | Bump per the version rule. `TestVersionNoted` checks it. |

Old-to-new checklist, to tick off per file:

| Old | New | Files (approx.) |
| --- | --- | --- |
| Recent | History | 42 |
| batch | take (Group for its History folder) | 19 |
| panel (changes) | External change | 23 |
| salvo (for presets) | Preset | 37 |
| Settings PIN | Advanced Settings PIN | 44 |
| hand lock, soft lock | Lock | 10 |
| This device | This browser | 13 |
| sent log | Communication Log | 18 |
| auto-lock (as sign-out) | Auto-lock (raises the Lock only) | 20 |
| log in, log out | Sign in, Sign out | 23 |
| follows | External follow | grep at sweep time |
| take (a preset entry or one destination) | plan entry, route | grep at sweep time |

## Track 2: behavior changes

Each becomes a GitHub issue on `sethdotfm/sode` after the list is approved. None is in Track 1.

1. **Server-held Lock for signed-in users.** Reverses #198, where Ctrl+L signs the user out. The server records the locked state per session, so reload and reconnect still show the PIN pad, and only that user's PIN resumes it. Needs an ADR.
2. **Auto-lock for persistent users.** They skip the idle timer today. After this they get the Lock like everyone else, and Auto-lock never signs anyone out.
3. **Sign-in mode.** PIN only stays the default. Username and PIN is optional, with PINs no longer unique across users. Username is separate from the name History shows.
4. **Last chip hides until this browser has made a take.** Remove the fallback to the newest Sode take in History.
5. **Rename keeps persistent sessions.** Today a rename kills them. Decide whether it should.

## ADR

One only, for item 1 above: `docs/adr/0001-server-held-lock.md`. Hard to reverse, surprising without context, and a real security trade-off. The earlier candidates (single router, stdlib-first, no build step, no TLS, config file) were not chosen: single router is a current scope, and the rest are already recorded in `AGENTS.md`.

## Order

1. Track 1, steps 1 to 6, in one branch and one PR.
2. File the Track 2 issues.
3. Write the ADR with item 1's issue.
4. Work Track 2 issues one at a time, each with its own tests and CHANGELOG entry.

## Open

- Track 1 step 5: which persisted names, if any, are worth a migration. Decide with the file counts in hand.
- Whether a signed-in Lock needs a server round trip on every keypress for the PIN pad, or only on resume. Decide in the ADR.
