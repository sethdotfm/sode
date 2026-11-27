# Settings redesign: status

Branch `settings/redesign`. It holds round 1, Phase 0 and the phases below. The design is
docs/research/settings-review/RECOMMENDATIONS.md (section 0 has your decisions).

| Phase | What | State | Commit |
| --- | --- | --- | --- |
| 0 | Router names and safety, Protects list, one Routers icon | done (before this branch) | aaaecb71 |
| 1 | One router list in memory (server only) | done | 82d3205a |
| 2 | One shape on the wire, breaking health/status change | done | 7cf4d0b3 |
| 3 | Router list and detail view, Routers > Protects | done | 0517bb8a |
| 4 | Reorder, Remove any router | in progress | |
| 5 | Tie lines table | done (merged) | 2b629148, merge a1126a4c |
| 6 | Follow-ups (level numbering, chips, log lines, wizard title) | server part done (47471675); page part not started | |

## Look at it

Screenshots: docs/research/settings-proof/<phase>/ (before-*.png and after-*.png).

## Decisions I made

- Phase 1: `Config.Routers` is one list. `DefaultRouter()` is the first router. Golden files pin the file, hello,
  health, status and one Save shapes (`TestPinRouterShapes`).
- Phase 5: the State column says what the tie line carries now. Groups cannot close. "Add tie line" is an outline
  button. The server's duplicate tie line text is unchanged; the page checks first and names the other tie line.
- Phase 2: `/healthz` now answers only `{"server":"up"}` (it carried router A's address with no key). Panel pages
  read `routers[]` from `/status` for Offline.
- Phase 3: Status label is one field at the top of Routers > Routers. Protects is only the list; Sode's device
  number is in each router's detail view, so a Trusted user sees it read-only. "Re-read now" reads every router
  (the server has no read of one router). Add router sends every setting. Old bookmarks to removed sections
  find the new field. The Communication Log link is not filtered to the router (/log has no router filter).
- Phase 6: blank Default levels stays blank and means every level. Log lines name the router ("B Studio B").

## Known failures

- On the base already: TestChromePanelsTopBar, TestChromeProtectFromRouteView, TestChromeSettingsTieLines.
  All three passed in this session's runs.
  TestChromeRouterListManyRouters can be flaky.

## Needs you

- Nothing yet.
