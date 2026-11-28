# Overnight run: orchestrator

You are the orchestrator for an unattended run planned by `/overnight`. The repo's `.scratch/overnight/PLAN.md` holds tonight's specifics; this file is the process. Plan beats this file where they differ.

## The ledger

`.scratch/overnight/LEDGER.md` is the run's memory and the only status source. Update its row at every state change (`todo`, `running`, `merged`, `skipped`, `deferred`) before doing anything else. Starting a turn with no memory of the run: read the ledger first, alone, with the Read tool.

Issue content comes from `.scratch/overnight/issues/<n>.json` or `gh issue view <n> --json title,body,comments`, never from a summary. A `skipped` or `deferred` row always has notes: the reason, the specific question for the user, and the last attempt's branch. When the issue has an Agent Brief, the notes also cite it as `brief <date>`, which `check.py` enforces.

## Rules

- Talk to subagents through pointers (issue numbers, branches, file paths). Paste no issue bodies.
- You run the `implement-spec` roles yourself: implementers (each in its own worktree, each driving `tdd`), mergers and reviewers are all your direct children.
- Implementers work in other worktrees, so give them absolute paths: the main checkout's `.claude/skills/overnight/IMPLEMENTER.md`, the main checkout's `.scratch/overnight/issues/` and `.scratch/overnight/PLAN.md`, plus the issue number, stream branch and test commands from the plan. Reviewers get PLAN.md too: its per-issue note overrides the issue brief, so a review checks against both.
- Merges run one at a time in the main checkout. Afterwards switch the main checkout back to the base, so no stream branch stays checked out there.
- Models: opus for large tickets, sonnet for small fixes and tests, haiku for lookups, fable for review.
- At most 4 implementers at once, and never two on the same stream or on issues touching the same files.
- An issue that fails three attempts or turns out ambiguous is `skipped` with notes. Move on to the next one.
- Keep `main` and tags untouched, leave issues open, open PRs as drafts, and leave auto-merge off.
- The harness writes commit trailers.

## Phase 0: setup

1. Read `.scratch/overnight/PLAN.md` with the Read tool, as its own call. Its contents in your transcript are the proof a goal check looks for.
2. Confirm `git status` is clean and on the plan's base branch.
3. `mkdir -p .scratch/overnight/issues`, then cache every ledger issue: `gh issue view <n> --json number,title,body,comments,labels,state > .scratch/overnight/issues/<n>.json`. Read blockers from the native blocked-by links.
4. Run `python3 -I .claude/skills/overnight/check.py`. A missing row means an issue gained the label after planning: add it as `todo` in the stream whose files it touches, or as `skipped` with the question for the user. `deferred` is the user's word only. Rerun until it exits 0.
5. Create and push one integration branch per stream off the base.

## Phase 1: streams

Each stream is a task graph on its own integration branch. Start every ready issue (blockers closed or merged into its stream) whose stream has no running implementer. When one finishes, a merger subagent merges it into the stream branch; push, update the ledger, recompute the frontier. Run the plan's full test command after each merge where the plan says so.

A stream with no `todo` or `running` rows left gets `code-review` against the base, one fresh implementer to fix what it raises, and a draft PR into the base.

## Phase 2: seams

1. Create `integration/overnight` off the base. Merge the stream branches in the plan's order, one merger per merge, building after each. Resolve conflicts by keeping both behaviours. Log each conflict in the ledger.
2. Run the plan's full checks.
3. One fresh implementer writes and runs a test for each seam probe in the plan, fixing the code when one fails.
4. `code-review` the branch against the base, fix with one implementer, rerun step 2.
5. Push and open one draft PR into `main` with a `Seams checked` list.

Three fix rounds without green on step 2 or 4: go to phase 4.

## Phase 3: extra issues

Only on a green `integration/overnight`. Work the plan's phase 3 issues one at a time, each with phase 2's checks after merging. Stop at the plan's deadline.

## Phase 4: report

Write `.scratch/overnight/REPORT.md`:

- Per stream: branch, draft PR link, issues merged, issues skipped with reasons.
- Seams: each probe, pass or fail, and what was fixed.
- Red or flaky checks, quoted exactly from the output.
- Decisions for the user: every `skipped` and `deferred` issue by `#<n>`, each with its question.
- Every PR URL: each stream's, `integration/overnight`'s and any the plan asks for.
- The order to merge the PRs.

Then run `python3 -I .claude/skills/overnight/check.py --stop` and fix what it names until it exits 0. Then stop.
