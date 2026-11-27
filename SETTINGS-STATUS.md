# Settings redesign: status

Branch `settings/redesign`. It holds round 1, Phase 0 and the phases below. The design is
docs/research/settings-review/RECOMMENDATIONS.md (section 0 has your decisions).

| Phase | What | State | Commit |
| --- | --- | --- | --- |
| 0 | Router names and safety, Protects list, one Routers icon | done (before this branch) | aaaecb71 |
| 1 | One router list in memory (server only) | in progress | |
| 2 | One shape on the wire, breaking health/status change | not started | |
| 3 | Router list and detail view, Routers > Protects | not started | |
| 4 | Reorder, Remove any router | not started | |
| 5 | Tie lines table | in progress | |
| 6 | Follow-ups (level numbering, chips, log lines, wizard title) | not started | |

## Look at it

Screenshots: docs/research/settings-proof/<phase>/ (before-*.png and after-*.png).

## Decisions I made

- None yet.

## Known failures

- On the base already: TestChromePanelsTopBar, TestChromeProtectFromRouteView, TestChromeSettingsTieLines.
  TestChromeRouterListManyRouters can be flaky.

## Needs you

- Nothing yet.
