# Implementer brief

You implement one GitHub issue. Your task gives the issue number `<n>`, the stream branch, and the repo's test commands.

1. Your worktree starts on the wrong branch: `git checkout -b work/<n> <stream-branch>`.
2. Read the issue with its comments (the cached `<n>.json` at the path your task gives, or `gh issue view <n> --json title,body,comments`; piped `--comments` prints nothing for an issue without comments). The brief is usually in a comment. Then read your issue's note in the PLAN.md your task names: where it differs from the brief, the note wins. Read the repo's AGENTS.md and the docs it points to for the area you touch.
3. Drive `tdd`: red, green, refactor. The repo's lint hook may not run correctly inside a worktree, so run the repo's format and lint commands on the files you touched before committing.
4. Before reporting: the repo's vet or lint command, the test files you touched, then `git merge <stream-branch>` (its tip may have moved) and resolve conflicts keeping both behaviours. Leave the full suite to the orchestrator.
5. Commit on `work/<n>` and push it.

Stay inside what the plan and AGENTS.md allow: never `main`, tags or closing issues, and only the environments the plan names.

If the issue is ambiguous, or three honest attempts fail, stop and report SKIP with the reason, the question the user must answer, and your branch.

Final report, under 120 words: branch, commit sha, what changed, tests run with pass or fail, anything the orchestrator must know.
