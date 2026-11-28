---
name: overnight
description: Plan an unattended overnight implementation run over the repo's ready-for-agent issues, and hand over the orchestrator's start prompt.
disable-model-invocation: true
---

Plan tonight's run with the user, then hand them a start prompt for a fresh orchestrator session. The orchestrator follows [RUN.md](RUN.md); implementers follow [IMPLEMENTER.md](IMPLEMENTER.md). Run files live in `.scratch/overnight/` in the repo, which must be git-ignored (check with `git check-ignore`).

**Scope is the label, not a list.** Every open issue with the ready-for-agent label (name from `docs/agents/triage-labels.md`, default `ready-for-agent`) gets a ledger row. The user narrows scope by deferring issues with a reason, never by leaving them off. [check.py](check.py) enforces this, so an issue only drops out of a run when someone wrote down why.

**Live state only.** Issue status comes from `gh` at the moment you read it. Handoff notes, earlier session summaries and old plans go stale (a brief posted after the note was written is invisible to it), so use them for issue numbers and nothing else.

## Steps

1. **Archive the last run.** If `.scratch/overnight/` exists, rename it to `.scratch/overnight-<date of its LEDGER.md>`. Done when `.scratch/overnight/` is empty or absent.

2. **Pull the candidates.** `gh issue list --state open --label ready-for-agent --limit 500 --json number,title,labels,updatedAt`. Done when you have the full list.

3. **Read every candidate with its comments.** `gh issue view <n> --comments`. Briefs usually live in a comment, so a body-only read misses them. For more than about ten issues, fan out read-only subagents (haiku or sonnet) that each return per issue: where the brief is, files and areas touched, blocked-by links, and any doubt about readiness. Done when every candidate has that line.

4. **Settle doubts with the user now.** A doubt about a ready-for-agent issue is a question for the user while they are awake. If the answer is that it is not ready, ask to relabel it `needs-info` with a comment naming what is missing; it then leaves the candidate set. Done when no candidate carries an unasked doubt.

5. **Agree the shape with the user.** The base branch, the streams (groups of issues that touch disjoint files, in blocked-by order), the seams where streams meet, and which issues to defer tonight and why. Done when the user has approved it.

6. **Write the plan and the ledger.**
   - `.scratch/overnight/PLAN.md`: base branch, guardrails taken from the repo's AGENTS.md and memory (test commands, environments that are off limits, commit and push rules), the streams table, per-issue notes that point at where the brief lives, the seam probes, any phase 3 issues, and the deadline. It points at `.claude/skills/overnight/RUN.md` for the process rather than restating it.
   - `.scratch/overnight/LEDGER.md` with the table below, one row per candidate, status `todo` or `deferred`. A deferred row's notes give the user's reason, and cite the issue's Agent Brief as `brief <date>` when it has one.

   ```markdown
   | # | stream | status | branch | notes |
   |---|---|---|---|---|
   | #189 | 2 lock | todo | | brief in comment 1 |
   ```

7. **Run the coverage check.** `python3 -I .claude/skills/overnight/check.py`. Done when it exits 0. A failure names missing rows and deferrals that skip over a brief; fix them and rerun. Show the user its `not built:` lines verbatim and get their OK on each.

8. **Hand over.** Give the user the start prompt in a `text` block, to paste into a fresh session at the repo root in auto mode, plus the keep-awake reminder (sleep stops the run):

   ```text
   /goal `python3 -I .claude/skills/overnight/check.py --stop` exits 0
   Read .claude/skills/overnight/RUN.md, then run tonight's overnight batch from .scratch/overnight/PLAN.md.
   ```

   Check the `/goal` syntax in `/help` first if it has changed.
