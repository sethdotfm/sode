# Settings redesign: status

Branch `settings/redesign`. It holds round 1, Phase 0 and the phases below. The design is
docs/research/settings-review/RECOMMENDATIONS.md (section 0 has your decisions).

| Phase | What | State | Commit |
| --- | --- | --- | --- |
| 0 | Router names and safety, Protects list, one Routers icon | done (before this branch) | aaaecb71 |
| 1 | One router list in memory (server only) | done | 82d3205a |
| 2 | One shape on the wire, breaking health/status change | not started | |
| 3 | Router list and detail view, Routers > Protects | not started | |
| 4 | Reorder, Remove any router | not started | |
| 5 | Tie lines table | done (merged) | 2b629148, merge a1126a4c |
| 6 | Follow-ups (level numbering, chips, log lines, wizard title) | not started | |

## Look at it

Screenshots: docs/research/settings-proof/<phase>/ (before-*.png and after-*.png).

## Decisions I made

- Phase 1: `Config.Routers` is one list. `DefaultRouter()` is the first router. Golden files pin the file, hello,
  health, status and one Save shapes (`TestPinRouterShapes`).
- Phase 5: the State column says what the tie line carries now. Groups cannot close. "Add tie line" is an outline
  button. The server's duplicate tie line text is unchanged; the page checks first and names the other tie line.

## Known failures

- On the base already: TestChromePanelsTopBar, TestChromeProtectFromRouteView, TestChromeSettingsTieLines
  (this one passes here now).
  TestChromeRouterListManyRouters can be flaky.

## Needs you

- Nothing yet.
