# Big, Bloaty, and Broken

Alpha bugs for the multi-router build, on branch `alpha` (cut from `integration/overnight` at 7f281f87). Notes are the owner's, taken while poking at the software, then grilled here.

Test setup: two swp08sim routers and Sode. Locally, `.dev/run-alpha.sh` (not in git) runs router A on :8910, router B on :8911 and Sode on :8080. Elsewhere, run `SIM_PORT=8910 go run ./cmd/swp08sim` and `SIM_PORT=8911 go run ./cmd/swp08sim`, start Sode against the first, and add router B in Settings > Routers. For a third router (AB-01 chains), add `SIM_PORT=8912` as router C, with tie lines A→B and B→C in Settings > Routers > Tie lines.

## Workflow

1. **Capture.** Record each note as reported. Do not fix anything yet.
2. **Grill.** Go through the entries with the owner. Fill in Expected, Decisions and Open questions until none is open.
3. **Hand off.** A cloud agent (with subagents) works the entries marked `ready`. It follows AGENTS.md, makes one commit per entry, and fills in Proof and Notes.

Status values: `captured` → `grilling` → `ready` → `in progress` → `done` (or `wontfix`, `blocked`).

## Proof rules (for the agents)

The owner wants each fix proven, not claimed. For every entry:

- **Before and after.** Reproduce the bug first on the two-router sim, and say how. Then show it fixed.
- **UI entries:** headless Chrome screenshots at 1920×1080 desktop, before and after, saved under `docs/research/alpha-proof/<entry id>/` (not checked by the STE and vocabulary tests). For a layout or animation entry, also show `main` for comparison where the entry says "as before".
- **A test that fails before the fix and passes after.** A browser check (`TestChrome*`, docs/agents/browser-checks.md) for UI behavior, a node test (`web/test/*.test.mjs`) for pure rules, a Go test for server behavior.
- **Checks run.** `go test -short ./...`, `go vet ./...`, `node --test 'web/test/*.test.mjs'`, and only the browser checks added or changed (AGENTS.md: CI runs the rest). Paste the pass lines in Notes.
- **Notes.** What changed, file:line of the main change, the commit SHA, and anything left over or found on the way.

## Template

### AB-00: short title

- **Status:** captured
- **Area:**
- **Reported:** the owner's words
- **Now:** what the code does today (file:line)
- **Expected:**
- **Decisions:**
- **Open questions:**
- **Proof:**
- **Notes:**

---

# Round 1

Two areas: the Tie line picker (AB-01, AB-02) and the main page patch row (AB-03, AB-04, AB-06). AB-05 was folded into AB-04.

Handoff: branch `alpha-round1` off `alpha-notes`, one commit per entry in the order AB-03, AB-04, AB-06, AB-01, AB-02 (small and visual first), one draft PR into `alpha`. Update each entry's Status, Proof and Notes in this file in the same commit as the fix.

## Tie line picker

### AB-01: One tie line choice for every destination that shares a hop

- **Status:** ready
- **Area:** Tie line picker (web/tieflow.js, web/tiepicker.js, web/tiepickrules.js, web/tietakerules.js; server internal/app/tietake.go only if the plan needs it)
- **Reported:** "When taking a single source from router A to multiple destinations on router B, it should default to using the same tie line choice for all of them. If they're all going to the same router and it only needs one or two ties, it could just pick A goes to B via tie 5, Enter, and not ask me for every single one. This also applies to going to multiple routers: if source A1 is going to B1, B2 and C1, C2, it would ask about the A→B tie line and then a B→C tie line, but use commonality wherever possible."
- **Now:** `tieTake` (tieflow.js) opens the picker once for each route across routers, in plan order: "Route 1 of 2 across routers", with Next on each one and Take on the last. The server already lets two destinations with one source share a tie line (docs/agents/web-layout.md, Tie line picker). So the sharing works on the wire, but the page asks again for each destination.
- **Expected:**
  - The picker asks one question for each hop that the routes share. A hop is (from router → to router) for the same source and levels.
  - One source to many destinations on router B: one A→B question. Every B destination uses that choice. Enter on the preselected best choice sends.
  - Destinations on B and C: one A→B question and one B→C question. A route that needs a hop that the others do not have gets its own question.
  - The picker opens every time a take crosses routers (owner's choice), even when a tie line already carries the source. The best choice is preselected, so it is one Enter.
  - Optional split: a hop question has a way to open it per destination (for example a "Choose per destination" control on that step, with a key). Split is off by default. When split, each destination on that hop gets its own choice, as today.
  - The heading names the hop and its destinations ("A → B, for B.1 B.2 B.3"), not "Route 1 of 3".
- **Decisions:**
  - Owner: ask once per hop, always; no auto-skip. (2026-11-24)
  - Owner: per-destination split is allowed, off by default. (2026-11-24)
  - The page still never computes paths. It groups the planner's paths by hop. If the plan does not give enough to group, extend `tiePlan`'s answer, not the page.
  - Live update, the lockout, Protect (shield and P), Reserve asks, One-off tie lines, the `tie_line_changed` reopen and Redo (`lastState`) keep working for a grouped hop. A change that hits any destination of the hop marks the hop.
  - Update docs/agents/web-layout.md (Tie line picker) and docs/tielines-design.md for the new flow, in STE (docs/ste.md).
- **Open questions:** none.
- **Proof:**
  - Browser check with two simulators: one source, three destinations on B: one picker step, one Enter, three routes made through one tie line. Fails on alpha (three pickers), passes after.
  - Browser check with three simulators (A, B, C, tie lines A→B and B→C): one source to B.1, B.2, C.1, C.2 asks exactly two questions.
  - Browser check: split a hop and send one destination through another tie line.
  - Node tests for the grouping rule in tiepickrules.js or tietakerules.js.
  - Existing TestChromeTiePicker* checks still pass (run them alone).
  - Screenshots of the grouped picker.
- **Notes:**

### AB-02: Tie line state readable at a glance

- **Status:** ready
- **Area:** Tie line picker (web/tiepicker.js, web/tiepicker.css, rules in web/tiepickrules.js)
- **Reported:** "Make the status of the tie lines more obvious, maybe with an outline or some sort of color bar. Right now I have to do a lot of reading to figure out what's currently on it, how many things are going to change, what's the most optimal, is there something that's already this source. All of those questions could be answered at a glance with better design."
- **Now:** Each choice is text: the tie line name, what it carries, Affected destinations, held or Reserved. The only color is `--pb-tieline` on the wire of the chosen choice.
- **Expected:** Owner chose "left bar + count badge":

  ```
    ┌──────────────────────────────────────┐
  ▌ │ TIE 5   carries A.12 CAM 1   ✓ Has it │  <- bar: routed
    └──────────────────────────────────────┘
  ╔══════════════════════════════════════╗
  ║ TIE 6   free                 Best    ║  <- outline = best
  ╚══════════════════════════════════════╝
  ▌ TIE 7   carries A.40 VT 2   [3 change]   <- bar: caution
    TIE 8   Protected by Ana 10:42  (grey)
  ```

  - Already carries this source: left bar in the routed color, and text "Has it".
  - Free, changes nothing: no bar, no color (dark cockpit).
  - In use, N destinations change: left bar in the caution color, and a badge with the count ("3 change").
  - Best pick: an outline on the row, and text "Best". It can combine with the other states.
  - Protected or Reserved: grey as today, with who and when.
- **Decisions:**
  - Owner: all four states at a glance; left bar + count badge + outline. (2026-11-24)
  - Follow docs/color-roles.md and ADR 0013 (read them first). Use patchbay tokens only (green = routed only, amber = caution). Each state also has text (patchbay: status always with text). If a color role does not fit, stop and note it in this entry; do not invent a color.
  - Bars use `--pb-accent-bar-w`, as the source and destination chips do.
  - The same states show on the grouped hop of AB-01. Do AB-01 first, then AB-02.
- **Open questions:** none.
- **Proof:** screenshots of a picker with one row in each state, light and dark theme. Node test for the state rule (which state a choice gets). A browser check asserts the state classes on the rows.
- **Notes:**

## Main page patch row

### AB-03: Next chip too wide from its key hint

- **Status:** ready
- **Area:** Main page, Last and Next chips (web/desktop.js `renderLast`, web/base.css `.sode__next*`, `.sode__key-note`)
- **Reported:** "The Next chip is extended very wide because of the full-length Control plus Shift plus D helper text. I think we can hide the helper text on those. A user who's keyboard focused is going to read the docs, so it doesn't need to be shown there."
- **Now:** web/desktop.js:282 writes the hint as `keyLabel('ctrl+shift+d')`, which spells out "Ctrl+Shift+d". On `main` it was the short glyph `⌃⇧d`. The hint is in the chip three times: the tag, the open part, and the hidden size chip that holds the slot.
- **Expected:** No key hint text on any Last or Next chip (source, destination, levels). The key stays in the tooltip (`data-pb-tip`) and in the keys list (web/index.html:847).
- **Decisions:**
  - Owner: hide on all chips. (2026-11-24)
  - Remove the `kbd` elements from the chips, not only hide them with CSS, and delete the CSS that only served them.
  - Check the phone layout (mobile.js) too; same rule if it shows hints on these chips.
- **Open questions:** none.
- **Proof:** screenshots main vs alpha vs fixed with Last and Next showing. A browser check asserts that the chips hold no `kbd` and that the open Next chip is no wider than its tag plus its value.
- **Notes:**

### AB-04: Next chip animation strobes and no longer slides

- **Status:** ready
- **Area:** Main page, Next chip open on hover, focus or Shift (web/base.css `.sode__next`, `.sode__next-open`, `.sode__next-slot`, `html[data-shift]`; web/desktop.js `renderLast`)
- **Reported:** "The animation behavior is totally scuffed up. It's just kind of a mess." Grilled (was AB-05, "the chips on the top are different in functionality"): "Just visual glitchiness as far as I can tell. If near the edge of the hitbox it opens and closes repeatedly, looking like a strobe. The animation used to be a smooth transition and now it disappears into nothing."
- **Now:** The Next chip opens through CSS to show the value on hover, focus or Shift held, and hides Last meanwhile (web-layout.md). The slot keeps the closed chip's room.
- **Expected:**
  - No strobe. With the pointer anywhere, including still on the edge, the chip is either open or closed, and stays so.
  - A smooth open and close, as on `main`. Nothing disappears with no transition.
- **Decisions:**
  - Owner: no change in function; this is visual only. (2026-11-24)
  - Likely cause to check first: the hover is on the element that changes size, so opening moves the edge away from the pointer, which closes it again. The hover target must be a box that does not change size while it opens (the slot), or the open part must not change the hover area.
  - Find the commit on alpha that broke the transition (compare with `main`'s base.css and desktop.js), and name it in Notes.
  - Do AB-03 first; the hint removal changes the widths.
- **Open questions:** none.
- **Proof:** frame strips (screenshots every 50 ms through the open and close) for main, alpha and fixed. A browser check that holds the pointer on the edge of the chip for 1 s and asserts the open state does not toggle. Respect `prefers-reduced-motion`.
- **Notes:**

### AB-05: folded into AB-04

- **Status:** wontfix (duplicate of AB-04)
- **Notes:** The owner confirmed that the chips work as before; only the glitch of AB-04 is wrong.

### AB-06: + button, Levels, Take and Clear sit higher than the boxes

- **Status:** ready
- **Area:** Main page patch row layout (web/desktop.css `.sode__boxes`, `.sode__group--dst`, `.sode__group--levels`)
- **Reported:** "To the right of the destination box, the plus icon, the levels, Take and Clear all seem to be vertically aligned differently. They are now higher than they were before."
- **Now:** Alpha made the patch row three groups of one width with a subgrid (1bcf487a, cd0e549e, 3a7a0857). The + button, Levels box, Take and Clear now sit higher than on `main`.
- **Expected:** They line up with the Source and Destination boxes as they did on `main`.
- **Decisions:** Keep the three-groups layout; fix only the vertical alignment. Measure on `main` what "lined up" was (centres or tops) and match it.
- **Open questions:** none.
- **Proof:** screenshots main vs alpha vs fixed, with and without a Last chip showing, and with 4 and 17 levels. TestChromeMainBoxesLayout gets an assert on the vertical position of these controls against the Destination box (fails on alpha, passes after).
- **Notes:**
