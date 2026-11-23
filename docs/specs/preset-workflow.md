DISCLOSURE: This was written by an LLM.

# Preset workflow

Status: proposal for triage. Nothing here is built. The step counts for today's flow come from reading `web/presets.js`, `web/bookmarks.js` and `docs/agents/presets.md`. Check them in the running page before you plan the work.

## Problem

A person who has routed something and wants to keep it needs about seven actions across four views. These are the Bookmarks strip, the All Presets popup, the Edit Presets dialog and the pick bar. Another Bookmark step puts the preset on the strip. Each step is easy to find alone. Together they are slow, and a person under pressure gives up.

## User goals

1. Make a preset from what is routed now.
2. Edit a preset.
3. Put a preset on the strip, so one click or Alt+digit fires it.

The common case is goal 1 followed by goal 3. A person usually wants a preset because they want it on the strip.

## Today's flow

Start: the patch boxes hold the plan (or a take was sent before).

1. Select All presets on the strip, or press Ctrl+G. The All Presets popup opens.
2. Select Preset Editor in the popup header. The popup closes and Edit Presets opens with no preset loaded.
3. Type a name. Pick Save for (This browser, Me or Everyone).
4. Select Add from Patch Boxes…. The editor hides and the pick bar shows.
5. Press Take or Enter in the boxes. Sode adds the plan as a row and clears the boxes. The boxes are empty again, so the plan is not in them any more.
6. Select Done on the pick bar. The editor comes back.
7. Select Save.
8. For the strip: select Edit on the strip, choose a list, select Add from the Palette…, find the preset, then Save. Alt+B on the preset's Palette row is shorter. It adds the bookmark to Me, or to This browser for a Guest, with no choice of list.

Steps 1 to 7 are seven actions. Step 8 is three or four more. The person must also know that a preset and a bookmark are two things.

Problems in the flow:

- The plan the person already built is cleared in step 5, then typed again in the editor if it needs a change.
- A take that Sode already sent has only one path to a preset: Add from History… in the editor. That path has different steps.
- Preset and Bookmark are two saves with two dialogs.
- A person who has no strip chip does not find a new preset, unless they open All Presets.

## Proposed flow

### Common case, three actions or fewer

1. Select Save as preset (or press its key) next to the patch boxes. A small dialog opens with the plan from the boxes. The name box holds a suggested name, such as `CAM 3 to MON 1`, selected for typing.
2. Type a name, or keep the suggestion. Save for and Put on the strip are already set. Save for starts as the last scope used, else Me. Put on the strip starts on.
3. Press Enter (or select Save).

Result: Sode saves the preset, adds a bookmark to the list that the strip shows, and keeps the boxes as they were. A one-line note says "Saved as CAM 3 to MON 1 and put on the strip." The new chip shows at the end of the strip.

This flow is two actions if the person keeps the suggested name.

### Same flow from History

A History group or entry gets Save as preset, which opens the same dialog filled with the routes of that take. This covers "make a preset from what is routed now", because the take that put the signals on air is in History. A group of several takes becomes one preset with one item for each route.

### What stays

Edit Presets stays for goal 2: change rows, rename, move to another scope, delete. All Presets stays as the search list. The dialog above does not replace them. It is a short path for the common case. A link in the dialog, Edit rows…, opens Edit Presets on the new preset for the person who wants more.

### Rules for the dialog

- The dialog uses the same checks as Save in Edit Presets: names unique in the scope, caps, the PIN row for Everyone, Rights. It must call the same save path (`addToPreset`, `itemsToAdd`, `saveList`), not a second one.
- It is not enabled when the boxes hold no complete route. The button says why in its tip.
- Save for Everyone needs a Trusted user or an Administrator, as it does today. A person without the role sees only This browser and Me.
- Put on the strip adds the bookmark to the list that the same Save for names. If that list is not the one the strip shows, the note says which list holds it. The strip shows This browser's list, else Me, else Everyone.
- A Guest gets This browser only.

## Option: New preset on the strip

A New preset button on the strip opens Edit Presets on a blank preset and skips the All Presets popup. It removes steps 1 and 2 of today's flow. The code is `emit('editPresetAt', { scope: '', name: '' })`, which calls `openPresets('', '')` and loads a blank preset. No change to `presets.js` is needed. The strip change is in `web/bookmarks.js` and its CSS.

| | Detail |
| --- | --- |
| Gain | Five actions instead of seven, with a small change. It helps a person who builds a preset from rows or History and not from the boxes. |
| Cost | The strip must stay one row (`docs/agents/presets.md`). A third ghost button beside Edit and All presets takes width that the chips need. On a narrow window the run scrolls sideways, and the button must stay outside the run. |
| Overlap | If Save as preset ships, New preset is used less. It still serves a person who starts from nothing. |
| Alternative | Put New preset in the All Presets popup header next to Preset Editor, and as a Palette action "New preset". No strip space is used. |

Likely user critique: "Why a button that opens an empty editor when I already typed the route?" The answer is that this option is the cheap one. Save as preset is the real fix. The owner can ship only one, or both.

## Option: the strip title (Bookmarks or Presets)

The strip heading says Bookmarks. People think of the items on it as presets, and the owner asked whether two rows (Presets and Bookmarks) would be clearer. `docs/agents/presets.md` keeps the strip to one row, because the strip sits above History and every row costs space on a small screen.

| Option | What changes | Gain | Cost |
| --- | --- | --- | --- |
| A. Keep Bookmarks, one row | Nothing. | Matches the glossary and the code. The strip holds presets, filter sets, pages, panels and web addresses, so the word is correct. | A person who made a preset looks for "Presets" and does not see it. Save as preset reduces this, because the dialog says it put the preset on the strip. |
| B. Rename the heading to Presets | One label. | Fits the main use. | Wrong when the strip holds other items. Breaks the glossary term Bookmark and the Edit Bookmarks dialog name. A rename needs the owner's decision and a glossary row. |
| C. Two rows: Presets, then Bookmarks | A second strip row and a second Edit button. | Presets get their own place. The heading is exact. | Costs one row of height on every window. Breaks the one-row rule. A preset that is bookmarked shows twice or the lists must be split by kind. The phone layout has no room. |
| D. One row, kind groups | Preset chips first, then a divider, then other bookmarks. The heading stays Bookmarks. | A person finds presets first with no new row. | Needs a kind order in `shownBookmarks`, and a divider that scrolls with the run. Alt+1 to 9 numbering must stay in strip order. |
| E. One row, heading shows the list | "Bookmarks: Me". | Says which list the strip shows, which the note in Save as preset needs. | Does not answer the owner's question. Can combine with A or D. |

Proposed: A with E's list name, and D if the owner still wants presets first. Do not do C unless the owner accepts the lost row. Keep B out unless the owner also wants the glossary changed.

Likely user critique: "I made a preset, not a bookmark." Save as preset answers it in the flow: the word "bookmark" does not show up in the common path, only "put on the strip". The label in the dialog is Put on the strip, not Bookmark.

## Proposals, with why and critique

### 1. Save as preset next to the patch boxes

Why the user cares: a preset takes two actions, and the plan stays in the boxes. A person can try a patch, like it and keep it without leaving the page.

Likely critique: "Now there are two ways to make a preset." Reply: Edit Presets is for editing. Save as preset is for the moment after a patch. The button's tip says "Save this plan as a preset". Also: "The button crowds the boxes." Place it with the Take and Clear controls. If the phone has no room, show it in the sheet after the take.

### 2. Save as preset from History

Why: the person often decides to keep a patch after they sent it. History already holds it.

Likely critique: "History rows are busy." Reply: one button on the group row and on the entry detail only. An ext row (a change made outside Sode) can be saved too, since it has routes. Check Rights, because a preset can hold only routes the person is allowed to fire.

### 3. Put on the strip in the same dialog

Why: the second Bookmark step is the part that people miss. After it, a saved preset sits in a popup and nobody finds it.

Likely critique: "I do not want every preset on the strip." Reply: the box is on by default and remembers the last choice. The strip holds 30 bookmarks at most. If the list is full, the box is off and says why.

### 4. Keep the plan in the boxes on save

Why: the person can still want to take the plan. Today Add from Patch Boxes clears the boxes.

Likely critique: "Clearing was how I knew the add worked." Reply: the note and the new chip say it worked. The boxes in the Add from Patch Boxes session keep their current behaviour.

### 5. New preset on the strip (option above)

Why: removes two actions for editor-first work.

Likely critique: see the option.

### 6. Suggested name

Why: typing a name is the slowest step. `SRC to DST` matches the chip tip (`presetTip`). A plan of several destinations uses `SRC to N Destinations`. Sode adds a number if the name is taken.

Likely critique: "The suggested names are bad." Reply: the name is selected, so any key replaces it. Names stay editable in Edit Presets, and the id keeps every bookmark.

## Open questions

1. Where does Save as preset go on desktop: beside Take, or in the strip? On phone, in the sheet after Confirm, or after the take only?
2. Does the dialog also work for an empty boxes state with a take in History selected? That is proposal 2, so it should.
3. Which scope is the default for a signed-in user: Me, or the last scope used? Everyone needs the PIN row.
4. Does a Guest get the dialog? Proposed: yes, This browser only.
5. Is the dialog a modal, or a popover under the button? A modal matches Filters and Edit Presets. A popover is quicker.
6. Should Alt+B on a preset row in the Palette ask for a list? Today it chooses Me.
7. The strip title: A, B, C, D or E above.
8. Should "Save as preset" also exist as a Palette action row? It costs nothing and helps keyboard users.
9. Naming in the UI: "Save as preset" or "Make preset"? The glossary uses Preset and has no verb for it. Add a row when the owner picks.

## Tests and docs to change

Tests, if the owner approves Save as preset:

- A new browser check, `TestChromeSavePresetFromBoxes`: fill the boxes, press Save as preset, accept the name, press Enter. Assert the preset exists, the bookmark chip shows, and the boxes are unchanged.
- `TestChromeSavePresetFromHistory`: the same from a History group.
- A node test for the name suggestion and the scope-to-list rule (pure rules in a new module under `web/`, tested in `web/test/`).
- `TestChromeBookmarks` (`internal/app/bookmarks_browser_test.go`) if the strip gets a New preset button or kind groups.
- A refusal case in `web/test/preset_cases.json` only if the validation changes. It must not, since the dialog reuses the save path.
- `TestPresetsMessageFits` and the caps tests stay as they are.

Docs and text:

- `docs/agents/presets.md`: add the dialog, its link to the save path, and the strip decision.
- `GLOSSARY.md`: a row for the new action if the owner picks a name. `docs/ste.md` and `internal/app/vocabulary_test.go` scan the new text.
- The shortcuts dialog in `web/index.html`, `web/settingsindex.js` and the Palette catalogue (`web/palettecatalogue.js`), for the key and the action row.
- A `changelog.d/<slug>.md` fragment on the implementation PR. This spec PR has none.

## Out of scope

- Changing the preset store, the caps or the wire messages.
- The Bookmarks editor layout (a separate change).
- Panel cells that fire a preset.
