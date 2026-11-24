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

Two areas: the Tie line picker (AB-01, AB-02) and the main page patch row (AB-03 to AB-06).

## Tie line picker

### AB-01: One tie line choice for every destination that shares a hop

- **Status:** grilling
- **Area:** Tie line picker (web/tieflow.js, web/tiepicker.js, web/tiepickrules.js, web/tietakerules.js; server internal/app/tietake.go)
- **Reported:** "When taking a single source from router A to multiple destinations on router B, it should default to using the same tie line choice for all of them. If they're all going to the same router and it only needs one or two ties, it could just pick A goes to B via tie 5, Enter, and not ask me for every single one. This also applies to going to multiple routers: if source A1 is going to B1, B2 and C1, C2, it would ask about the A→B tie line and then a B→C tie line, but use commonality wherever possible."
- **Now:** `tieTake` (tieflow.js) opens the picker once for each route across routers, in plan order: "Route 1 of 2 across routers", with Next on each one and Take on the last. The server already lets two destinations with one source share a tie line (docs/agents/web-layout.md, Tie line picker). So the sharing works on the wire, but the page asks again for each destination.
- **Expected:** The picker asks once for each hop that the routes share. One source to many destinations on router B asks one A→B question, and every B destination uses that choice. With destinations on B and C, it asks A→B once and B→C once. A route that needs a hop the others do not have still gets its own question.
- **Decisions:**
  - Group by hop (from-router → to-router, same source and levels), not by destination.
  - Uses the planner's existing paths. The page still never computes paths (web-layout.md rule).
- **Open questions:** see the grill list at the end of the round.
- **Proof:**
- **Notes:**

### AB-02: Tie line state readable at a glance

- **Status:** grilling
- **Area:** Tie line picker (web/tiepicker.js, web/tiepicker.css)
- **Reported:** "Make the status of the tie lines more obvious, maybe with an outline or some sort of color bar. Right now I have to do a lot of reading to figure out what's currently on it, how many things are going to change, what's the most optimal, is there something that's already this source. All of those questions could be answered at a glance with better design."
- **Now:** Each choice is text: the tie line name, what it carries, Affected destinations, held or Reserved. The only color is `--pb-tieline` on the wire of the chosen choice.
- **Expected:** Four questions answered without reading: what the tie line carries now, how many destinations change, which choice is best, and whether one already carries this source.
- **Decisions:**
  - Must follow docs/color-roles.md and ADR 0013: a healthy state shows no color, red is failure only, amber is caution and every Take, green is routed only. Read them before adding a color. Status always comes with text (patchbay).
- **Open questions:** see the grill list.
- **Proof:**
- **Notes:**

## Main page patch row

### AB-03: Next chip too wide from its key hint

- **Status:** grilling
- **Area:** Main page, Last and Next chips (web/desktop.js `renderLast`, web/base.css `.sode__next*`)
- **Reported:** "The Next chip is extended very wide because of the full-length Control plus Shift plus D helper text. I think we can hide the helper text on those. A user who's keyboard focused is going to read the docs, so it doesn't need to be shown there."
- **Now:** web/desktop.js:282 writes the hint as `keyLabel('ctrl+shift+d')`, which spells out "Ctrl+Shift+d". On `main` it was the short glyph `⌃⇧d`. The hint is in the chip three times: the tag, the open part, and the hidden size chip that holds the slot.
- **Expected:** No key hint text on the chips. The key stays in the tooltip (`data-pb-tip`) and in the keys list (web/index.html:847).
- **Decisions:**
- **Open questions:** see the grill list.
- **Proof:**
- **Notes:**

### AB-04: Next chip animation is broken

- **Status:** grilling
- **Area:** Main page, Next chip open on hover, focus or Shift (web/base.css `.sode__next`, `.sode__next-open`, `html[data-shift]`)
- **Reported:** "The animation behavior is totally scuffed up. It's just kind of a mess."
- **Now:** The Next chip opens through CSS to show the value on hover, focus or Shift held, and hides Last meanwhile (web-layout.md). The slot keeps the closed chip's room so nothing in the row moves.
- **Expected:** A clean open and close, as on `main`, or better. Likely tied to AB-03, because the wide hint changes the widths that the animation runs between.
- **Decisions:** Fix AB-03 first, then judge the animation again.
- **Open questions:** see the grill list.
- **Proof:** a short screen recording or frame strip of the open and close, main vs alpha vs fixed.
- **Notes:**

### AB-05: Chips above the Destination box work differently than before

- **Status:** grilling
- **Area:** Main page, chips above the Destination box
- **Reported:** "The chips on the top are different in functionality than they were previously."
- **Now:** Alpha changed this area in several commits: 140a287e (the armed next add and the chosen destinations show as destination chips), 3fc40cbf (the next-add hint shows only while the Destination box has focus), c56eda36 (compact chips), and the Last and Next chips now use `keyLabel`.
- **Expected:** (to grill)
- **Decisions:**
- **Open questions:** see the grill list.
- **Proof:**
- **Notes:**

### AB-06: + button, Levels, Take and Clear sit higher than the boxes

- **Status:** grilling
- **Area:** Main page patch row layout (web/desktop.css `.sode__boxes`, `.sode__group--dst`, `.sode__group--levels`)
- **Reported:** "To the right of the destination box, the plus icon, the levels, Take and Clear all seem to be vertically aligned differently. They are now higher than they were before."
- **Now:** Alpha made the patch row three groups of one width with a subgrid (1bcf487a, cd0e549e, 3a7a0857). The + button, Levels box, Take and Clear now sit higher than on `main`.
- **Expected:** They line up with the Source and Destination boxes as they did on `main`.
- **Decisions:** Keep the three-groups layout; fix only the vertical alignment. TestChromeMainBoxesLayout gets a check on the vertical centres of the boxes and these controls.
- **Open questions:** see the grill list.
- **Proof:** screenshots main vs alpha vs fixed, with and without a Last chip showing, and with 4 and 17 levels.
- **Notes:**

## Round 1 grill list

(Answers go into each entry's Decisions; this list empties as they land.)
