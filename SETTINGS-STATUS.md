# Settings redesign: status

One branch, `settings/redesign`, holds round 1 alpha fixes, Phase 0 and Phases 1 to 6. The design is
docs/research/settings-review/RECOMMENDATIONS.md (section 0 has your decisions). No other branch is needed.

| Phase | What | State | Commit |
| --- | --- | --- | --- |
| 0 | Router names and safety, Protects list, one Routers icon | done | aaaecb71 |
| 1 | One router list in memory (server only) | done | 82d3205a |
| 2 | Every router setting is `router/<uid>/<attribute>`; health/status lose router A's flat fields | done | 7cf4d0b3 |
| 3 | Router list and detail view; Routers > Protects is its own section | done | 0517bb8a |
| 4 | Reorder (`routers/order`); Remove works for any router | done | f17b27ff |
| 5 | Tie lines table | done | a1126a4c |
| 6 | Log lines name the router, blank Default levels, levels in each router's numbering, wizard title | done | 6336c4c5, 3710fe22 |

## Look at it

- Screenshots, before and after: docs/research/settings-proof/<phase>/ (phase3, phase4, phase5, phase6).
- Run it: start 2 to 4 simulators (`SIM_PORT=9311 go run ./cmd/swp08sim`, then 9312, ...), then
  `SODE_CONFIG=/tmp/s/config.yaml go run ./cmd/sode` and add the routers in Settings > Routers.

## Decisions I made (you did not)

- `/healthz` answers only `{"server":"up"}` (it gave router A's address with no key).
- Status label is one field at the top of Routers > Routers. It is in the settings export as `statusLabel`.
- Sode's device number is in each router's detail view, so a Trusted user sees it read-only.
- "Re-read now" reads every router: the server has no read of one router.
- Reorder is not Critical and reconnects nothing. The review says when the default router changes.
- The uptime file stays with the router it records. A new first router's uptime is in memory until a restart.
- Blank Default levels stays blank and means every level. Log lines name the router ("B Studio B").
- Old bookmarks to removed Settings sections find the field in the new detail view.
- The Communication Log link in a router's view is not filtered to that router (/log has no router filter).
- Tie lines: groups cannot close; the State column says what each tie line carries now.

## Known failures

- None in the checks run here: `go test -short`, Node tests, vet, staticcheck, lint and about 60 browser checks.
  The full browser suite runs only in CI.
- TestChromePanelsTopBar, TestChromeProtectFromRouteView and TestChromeSettingsTieLines failed on the base
  before; all three passed here. TestChromeRouterListManyRouters can be flaky.
- staticcheck needs `GOTOOLCHAIN=go1.27.0` in the cloud container (the tool's own build picks Go 1.26).

## Needs you

- Breaking change for monitors: `/api/health` and `/api/v1/status` lost the top-level router fields. Read
  `state` and `routers[]` (changelog.d/settings-router-keys.md).
- An older Sode reading a new config.yaml shows the status label as "address".
- Review the draft PR, then merge it into `alpha` yourself.
