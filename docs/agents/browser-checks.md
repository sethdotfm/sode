# Browser checks

Scripted headless Chrome against the real Handler and a built `swp08sim`. Never a live Ultrix: the only router a check may reach is the simulator it starts itself.

Run: `go test ./internal/app -run TestChrome -v` (about 3 s). `go test ./...` runs them too and they skip with a message when no Chrome or Chromium is found (looked up in `SODE_CHROME`, the macOS app paths, then PATH). `SODE_BROWSER_SHOTS=/tmp/shots` saves a PNG at each `b.shot(...)` and on a timeout.

Files, all in `internal/app`:
- `browser_test.go`: the harness. `startBrowser(t)` skips or returns a tab; `open`, `eval` (JS, promises awaited), `waitFor(what, expr)` (fails the test on a miss, naming the check), `shot`, `check`. It collects console errors, uncaught exceptions and failed loads, and `b.check()` fails the test on any. Google Fonts load failures are ignored.
- `panelbrowser_test.go`: `startSim(t)` (builds `cmd/swp08sim`, free port), `simApp(t, addr)` (real SW-P-08 client, user Ana with PIN 1234, API on) and the first check, `TestChromePanelSessionTake`: launch in Session mode, the PIN pad, stage and Take, then the tally read over REST.
- `panelrefusals_browser_test.go`: `TestChromePanelKioskAndRefusal`, a kiosk panel with the header hidden and a route outside the user's Rights refused on the page.
- `panelboth_browser_test.go`: `TestChromePanelBothMode`, a Both panel opened by its keyed link with no sign-in, then by its plain link behind the PIN pad.
- `paneleditorchrome_browser_test.go`: toolbar and menu chrome, `TestChromeEditorMenuLeavesSidebar` (a sidebar click puts the menu away and keeps the selection) and `TestChromeEditorFindFitsViewport` (Find's flyout from a low selection stays inside the window).
- `paneleditormem_browser_test.go`: `TestChromeEditorHoldsNoMemory`, the leak harness. `leakProbe` (installed with `Page.addScriptToEvaluateOnNewDocument` through `editorTabPrep`) counts live intervals, document and window listeners, object URLs and fetches; `b.measure()` forces a GC and reads them with `Runtime.getHeapUsage` and `Memory.getDOMCounters`. Six rounds of use must end where the second began, and the editor must run no CSS animation. Run it with `-v` for a sample after every step.

Add a check:
1. Name the test `TestChrome...` so one `-run` pattern finds it. Call `startBrowser(t)` first, then `startSim(t)`.
2. Build the app with `simApp`, run it (`a.Run(ctx)`), serve `a.Handler(web.FS)` from `httptest`, and make fixtures through the REST helpers (`userKey`, `makePanel`, `apiSend`). Don't use `apiTestApp` or `panelsApp` here: they replace the router with a fake.
3. Wait on the server for names and tally before opening the page (see the loop in the first check), since a page reads names only every 30 s.
4. Act with `b.eval(...)` and wait with `b.waitFor(...)` on DOM state, never `time.Sleep`. Assert the router's side through REST, not only what the page shows. End with `b.check()`.
   Bootstrap's reboot sets `scroll-behavior: smooth`, so `scrollTo` and `scrollIntoView` animate: after a scroll, wait for the element's rect to be in view before pressing at its coordinates (`TestChromeEditorFindFitsViewport`).
5. Keep checks few: each costs a Chrome start.

The simulator starts with destination d routed from source d on every level (`SIM_*` options: docs/reference.md).
