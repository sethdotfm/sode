DISCLOSURE: This was written by an LLM.

# Settings review: recommendations for routers, levels and Protects

Branch `alpha-round1` at ae0d7e15. This document joins three reviews and proposes one design. No app code changed.

- [routers/findings.md](routers/findings.md): R-01 to R-29.
- [levels-protects/findings.md](levels-protects/findings.md): LP-01 to LP-25.
- [statusbar/findings.md](statusbar/findings.md): SB-01 to SB-15.
- [prototypes/index.html](prototypes/index.html): three static prototypes of the design below.

Scope: the routers, their levels and the Protects in Settings. The status bar is in scope where it meets the router settings. The design leaves room for the other Settings topics, and it does not change them.

How the claims were checked: I read the three reports, looked at the screenshots, and read the code for each root cause. "Confirmed" below means that the code or a screenshot shows it.

## 1. Verdict

Seven problems matter most. Each one lists the findings that support it.

### 1.1 The first router is a different kind of thing

Findings: R-01, R-02, R-12, R-21, LP-05, LP-13, SB-08.

Router A has a one-line row, and its settings are in a block at the end of the topic. With 12 routers that block starts about 5900 px below the list. Every other router is an inline form in the list, plus a second form in Router levels.

Confirmed, with one correction to the routers review and to the brief. `config.yaml` is uniform already: format 2 writes one `routers:` list, and each element has the same keys. The split is in four other places:

- In memory: `Config` embeds router A as `Router` and holds the others in `MoreRouters` (`internal/config/config.go`, `router.go`).
- On the wire: the hello and the `config` message carry router A as flat keys. `protected` and `protectBy` at the top of the hello are router A's.
- In the one Save: router A uses flat keys. The others use `router/<uid>/<attribute>`, and only eight attributes exist (`onesavefields.go`, `routeradd.go`).
- In the page: fixed ids for router A, and `firstLinked()` for the Protects.

So the file needs no migration. The work is in the server model, the messages and the page.

### 1.2 A router after the first cannot set most of its settings

Findings: R-03, R-04, LP-04, LP-23, SB-07.

Routers B and later have eight fields in the UI. They have no Sode's device number, so a person cannot turn Protect on for them in Settings. They have no Name requests, so matrix 16 and above is a dead end (R-04).

One claim needs a limit. R-03 says that Sode cannot Protect on routers B to L. That is true in the UI only. The server reads `protectDevice` for every router, and the levels and Protects review set 201 and 202 by hand in the file. The default is 0 for every router, router A too. The defect is the missing field.

### 1.3 Sode names the wrong router, or no router

Findings: R-05, R-09, R-10, R-11, R-23, LP-15, LP-16, SB-06, SB-14.

- A change to the address of router B asks "Save and reconnect to 127.0.0.1:9310?", which is the address of router A. Confirmed in the screenshot and in `web/settingssave.js` (`address: now.router`). This is the most dangerous defect in the three reports.
- Review rows and errors have a prefix for router B and none for router A.
- Settings search gives eight equal lines for "address".
- One router has five display forms, and its title field has three labels (Title, Name, router name).

### 1.4 The Protects list does not scale, and Free loses the place and the result

Findings: LP-01, LP-02, LP-03, LP-06, LP-07, LP-08, LP-09, LP-10, LP-11, R-22.

158 rows are one flat list, 5282 px tall, with no find box, filter, group or count. Free on a row moves the page 5300 px to a Confirm at the end of the list. The result goes to a banner on the main page, which Settings hides. Free Sode's Protects gives no count and no choice of router. All confirmed in the screenshots. This is the owner's "Protects run right off the screen".

### 1.5 The Routers topic grows without limit and shows no link state

Findings: R-13, R-14, R-24, LP-24, LP-25, SB-05.

The topic is 6880 px with 12 routers. The content column stays 640 px wide. A router that is down looks the same as one that is up, and a stale name count has no mark.

### 1.6 With four or more routers the status bar hides which router is lost

Findings: SB-01, SB-03, SB-04, SB-09, SB-10, SB-11, SB-12.

The bar itself never overflows on the desktop, which is good. But from four routers it shows one red icon and `19/24`. The hover details are 4255 px tall with 24 routers and close when the pointer leaves. The status has no click action. On the phone, three routers push Settings off the screen (confirmed in the screenshot).

### 1.7 Order, the default router and Swap depend on which router is first

Findings: R-06, R-07, SB-08.

The file order decides the default router of a new browser, the order of the bar and which router cannot be removed. No UI changes the order. Swap always acts on router A (`web/swap.js` sends `routerData('').uid`).

### 1.8 Tie lines: second in priority

Findings: R-15 to R-20.

Remove deletes a tie line with one click and no Confirm, and presets that use it become Broken (R-18). That one item is urgent. The rest (a list that does not scale, the far edit form, entry by number only) is real and comes after the router work.

### Where I disagree, and what did not hold up

- **The file shape (R-12 and the brief).** See 1.1. No `config.yaml` migration is necessary for a uniform list.
- **Status label.** R-22 says hide or drop it. SB-07 says hide it or make it one server setting. I keep it for each router and give it a job at every count: it picks the text that the bar shows when the bar names that router. This needs no rename. See question 4.
- **Where the Protects list goes.** R-22 says "a server-wide place". LP-05 says a section of the Routers topic. I agree with LP-05: Routers > Protects.
- **Level colors for each router (LP-17).** I do not recommend a palette for each router. One palette by position keeps "the first level is this color" true on every router. The labels of the slots are the defect, not the shared palette.
- **The Users pattern (R-29).** I agree with a list and a detail view. I do not agree with a dialog. The router view must stay in the page, so the one Save, the link state and Settings search keep working.
- **Two save models for tie lines (R-19).** The instant save is a decision in docs/agents/settings-saves.md, not an accident. The defect is Remove with no Confirm. The model is a question for the owner (question 5).
- **`19/24` reads as a fault count (SB-12).** I agree in part. The design shows "19 of 24 up" and names the lost routers, which removes the doubt.
- **Not verified here.** R-28 (stale rows after the list changes) has no screenshot. R-11 quotes log lines with no screenshot, but `internal/app/health.go` confirms the flat health fields. The second half of R-20 was not tested by its reviewer.
- **Out of scope for now, and still real:**
  - the Lost link banner with many routers (SB-02)
  - amber uptime slots after a restart (SB-13)
  - the main page and History with 32 levels (LP-18 to LP-21)
  - the Protect picker for router A only (LP-12)
  - the setup wizard (R-27) and the PIN recheck words (R-26)
- **One thing the reviews did not report.** In Settings a Protect of another device is an amber badge. docs/color-roles.md gives Protected the color magenta and keeps amber for caution. The prototype uses plain text with a magenta shield. See question 6.

## 2. The proposed model

The rule: **a router is one item in one list, and every router has the same fields in the same place.** No field, action or message depends on the position of a router. Two facts that position decides today become explicit: the order (a person sets it) and the default router (a setting).

The Routers topic gets three sections: **Routers**, **Tie lines** and **Protects**. Router levels, Router & matrix, Router names, Takes and the block of router A all go into the detail view of each router.

### 2.1 One name for a router

Use one display form on every surface: the Router prefix, then the Router title. Example: "B Studio B". With one router Sode hides the prefix, as GLOSSARY.md says, and shows the title. A router with no title shows "Router B".

- The field label is "Router title" for every router (R-09). Add router and the setup wizard give the same default title, "Router A" and so on.
- Review rows, errors, the Confirm, log lines and search results all use this form (R-05, R-10, R-11, R-23, LP-16).

### 2.2 The router list

Prototype: [prototypes/routers.html](prototypes/routers.html). Switch between 1, 2, 12 and 24 routers.

The section is two panes: a list on the left and the detail view of the selected router on the right. Each list row has two lines and a fixed height:

- Line 1: the prefix, the Router title, and the marks Default, New, Edited or "Remove on Save".
- Line 2: the address, and the link state as an icon with words: Connected, Connecting, Down or Never connected.

A healthy row shows no color. Down and Never connected are red, and Connecting is amber, as docs/color-roles.md requires. Green is not used.

How it scales:

| Routers | List |
| --- | --- |
| 1 | The list is hidden. The detail view fills the section, with no Prefix field and no Default router choice. Add router stays. Tie lines says in one line that it needs two routers (R-24). |
| 2 to 7 | Each router is a row. No find box. |
| 8 to 23 | A find box (prefix, title or address) and "Only routers with a problem" show. The list has a fixed height and its own big scroll bar (`bigscroll.js`). |
| 24 and more | The same. The summary line above the list gives the counts: "24 routers. 19 connected, 5 down." |

A line above the list always gives the counts, so a person sees a lost router with no scroll. On a narrow window the list goes above the detail view.

### 2.3 The detail view of one router

Every router has every field. The groups, in order:

| Group | Fields and facts |
| --- | --- |
| Head | The name, the link state with its time and error, and the actions Move up, Move down, Swap…, Communication Log and Remove…. |
| Router | Prefix, Router title, Status label, Default router. |
| Connection | Address, Matrix, Name requests, Name character limit. A note says that a change here reconnects this router only. |
| Levels | Levels, Level offset, Level numbering, Default levels, and a row of this router's level chips. |
| Router names | Source and destination numbers, Re-read names every, the name counts with the time of the last read, Re-read now. |
| Takes | Multi destination (`fireMode`), Router salvo group, Re-read routes every. |
| Protects | Sode's device number, and "Protect is off for this router" when it is 0. The counts of Protects on this router, with a link to Routers > Protects. |
| Tie lines | The count of tie lines from and to this router, with a link to Tie lines filtered to it. |

Rules for the view:

- An action that is not available says why next to it. Example: "Sode cannot read the names while the Router link is down." (R-14)
- Sode checks a field when the person leaves it, and shows the error under the field (LP-15). One message form names the value and the valid range.
- If a field has an error, Save lists every error with the router's name. Each line goes to its field.
- The page checks Matrix against Name requests, so the dead end of R-04 cannot occur.
- Swap… is an action of the router that the view shows, so it acts on that router (R-06).
- The Broken and Name moved count is about every router. It moves out of the row of router A, to the head of the section (R-22).

### 2.4 Add, remove, reorder and the default router

- **Add router** makes a row with the mark New and opens its detail view. The person sets every field before Save. The review lists the address, matrix, levels, numbering and device number (R-25).
- **Remove…** is available for every router while Sode has two or more. It marks the row "Remove on Save" and the view offers "Keep this router". The review says what becomes Broken. Sode refuses to remove the last router.
- **Reorder:** drag a row, use Move up and Move down, or press Alt+Up and Alt+Down on a row. The order is the order of the list, the status bar, the Protects groups and every router select. It has no other meaning.
- **Default router** is a setting. One router has it. A new browser starts its patch boxes on that router. If a person removes the default router, Sode gives the setting to the first router that stays, and the review says so.
- All four go through the one Save, with one review and one PIN recheck.

The review comes before the PIN recheck and nothing asks again after the PIN. If a change reconnects routers, the review names each one with its new address, and the main action becomes "Save and reconnect" in amber (R-05).

### 2.5 Levels for each router

- **One place:** the Levels group of the detail view (R-08, LP-13). The Router levels section goes.
- **One control for Level numbering:** three buttons that show the real range of this router, for example "1–17", "0–16" and "V, 1–16". The same control is in the setup wizard. The words "Start at 1" then belong to Source and destination numbers only.
- **A hint for Level offset** on every router: "Level 1 = level 0 on the router."
- **Default levels:** a person types them in the numbering of that router. Blank means every level and stays blank in the file, so it stays true when the level count changes (LP-22). The review shows the values in the numbering of the router (LP-14).
- **Colors across routers:** Level colors stays one palette by position, in Look and feel. Label its slots as positions, not as level numbers. Each detail view shows the chips of its router with that router's labels, so a person sees which color each label gets (LP-17).
- Where levels of two routers show in one list, the destination's Router prefix says which router the labels belong to. Write "V, 1" and not "V-1" (LP-21).

### 2.6 Tie lines

The section stays in the Routers topic. The changes, in order of value:

1. Remove asks for a Confirm that says what uses the tie line. The result shows in the row (R-18).
2. The list is a table: Tie line name, from (destination), to (source) and state. Sode groups the rows by router pair, such as "A to B, 4", and each row shows the Router titles (R-15).
3. A filter by router and a find box are above the table. The link in a router's detail view opens the section with that filter.
4. Add tie line is at the top. Each end has a find box that accepts a name or a number, as the patch boxes do (R-17).
5. Edit opens in the row (R-16).
6. An existing cable gives the error "A.5 already feeds B.7 as CG3 Fill" (R-20).

There is no prototype for tie lines. The table follows the same pattern as the Protects prototype: groups, a find box, a filter and actions in the row.

### 2.7 Protects at scale

Prototype: [prototypes/protects.html](prototypes/protects.html), with 158 rows on 4 routers.

Routers > Protects is one list for every router. Sode's device number is not here: it is a setting of each router (2.3).

- **Bounded height.** The list has a fixed height and its own big scroll bar. The page does not grow (LP-01).
- **A group for each router.** The heading stays in view while its rows scroll. It shows the name, the link state, the counts and "Free Sode's on B…". A group can close (LP-06).
- **Find and filter.** A find box takes a name or a number. The filters are All, Sode's, Mine, Other devices and Router locks, each with its count. Router buttons limit the list to one router. With many routers the router filter becomes a select with a find box.
- **A count.** "158 on 4 routers. Sode set 91." With a filter: "Shows 12 of 158."
- **Columns.** Number, destination, levels, set by and the action. The Levels column is empty when the Protect is on every level (LP-07).
- **Free in the row.** Free turns the row into the Confirm, with the churning letter and the lockout. The list does not move. The row then shows "Protect freed" for about two seconds, and a status line under the list keeps the last result. A refusal shows in red in the row with "Try again" (LP-02, LP-03).
- **Free Sode's Protects, by router.** The dialog lists each router with its count and a checkbox. A router that is down or has Protect off is listed with the reason and cannot be ticked. The button says "Free 86 Protects on 2 routers". The result shows for each router in its heading (LP-10, LP-11).
- **A router that is down.** Its rows are grey with the note "These are the last known rows, read at 14:02:10." Free is disabled with the reason (LP-09).
- **Protect off.** The group heading says so and links to that router's detail view (LP-04).
- **Rows that Sode cannot free.** One line under the list says where to free them (LP-08). A label for a device number ("device 300 is the MCR panel") is a later item.

## 3. What the status bar needs

Prototype: [prototypes/statusbar.html](prototypes/statusbar.html). Switch the router count, the state and the layout.

The rule: **healthy routers collapse to a count, and each lost router keeps its name.**

| Routers | Bar on the desktop |
| --- | --- |
| 1 | The router icon, and the text that Status label picks: the title, the address or nothing. |
| 2 or 3 | One icon for each router, with its prefix and the text that its Status label picks. |
| 4 and more, all up | One Routers icon and "24 Routers". No color. |
| 4 and more, some lost | The icon in the worst color, "19 of 24 up", then each lost router by prefix: red for Down, amber for Connecting. The bar shows five, then "+4". |

- **Naming.** The bar and every list under it use the form of 2.1.
- **Click-through.** A click on a prefix or on the Routers icon opens a list that stays open. The hover details go (SB-04). The list has a fixed maximum height and its own scroll bar. Lost routers come first, with one line for each router. A find box shows from eight routers.
- A row opens to show the address, the error and the last 24 hours. It has two links: "Router settings" opens Settings > Routers with that router selected, and "Communication Log" opens /log (SB-05). A user who is not an Administrator sees the list with no settings link.
- **Never connected** is a state of its own, with the note "Check the address." (SB-15). It is red, as Down is.
- **Degraded note.** With one or two lost routers it stays as today. With three or more it is one sentence: "5 routers are not connected: B, E, L, R, W. You cannot patch their destinations now." A "Show routers" button opens the list (SB-09).
- **Compact view.** The lost prefixes show while Sode is Degraded. The rule in GLOSSARY.md, "only the icon of each Router that is down shows", then holds at any count (SB-10).
- **Phone.** The count form starts at two routers, so the header always fits (SB-03). The header shows "5 of 24 lost", and the note under it lists the prefixes. A press opens the same list (SB-11).
- **Both directions agree.** The list in the bar and the list in Settings > Routers show the same state words, in the same order, with the same names.

The Lost link banner with many routers (SB-02) needs the same grouping. It is not designed here.

## 4. Data and API implications

AGENTS.md says that a rename of a persisted or sent name needs the owner's approval. Each table below marks what is new and what changes.

### 4.1 `config.yaml`

| Name | Change |
| --- | --- |
| `routers:` | No change. It is a uniform list today. Its order becomes the order that a person sets. |
| `defaultRouter` | **New key**, at the top level. The value is a Router UID. A file with no key means the first router of the list, so old files load with no migration and no new `format`. |
| `statusLabel` | No rename. Its values `address`, `name` and `none` stay. The UI word for `name` becomes "Title". |
| `defaultLevels` | No rename. A blank value stays blank on save (LP-22). Today `normalize` fills it. |

### 4.2 In memory

`Config.Router` and `Config.MoreRouters` become one `Routers []Router`. This is not a persisted name. `RouterByUID("")`, a blank `hiddenDstsRouter` and `firstLinked()` lose their "first router" meaning. Each caller names a router, or asks for the default router.

### 4.3 The one Save keys

| Today | Proposed | Note |
| --- | --- | --- |
| Flat keys for router A: `router`, `routerName`, `routerShow`, `matrix`, `levels`, `levelWireOffset`, `levelNumbering`, `numberOffset`, `defaultLevels`, `nameLength`, `nameRequests`, `nameRereadSeconds`, `tallyResyncSeconds`, `fireMode`, `salvoGroup`, `protectDevice` | `router/<uid>/<attribute>` for every router | **Sent names change.** The flat keys leave the one Save. |
| `router/<uid>/{prefix,title,address,matrix,levels,levelWireOffset,levelNumbering,defaultLevels}` | The same, for every router | No rename. |
| None | `router/<uid>/{routerShow,numberOffset,nameLength,nameRequests,nameRereadSeconds,tallyResyncSeconds,fireMode,salvoGroup,protectDevice}` | **New keys.** I propose the JSON names that Sode sends today, so no new word appears. The file names (`statusLabel`, `routeRereadSeconds`) are the other choice. |
| `router/<uid>/remove` refuses router A | Allowed for every router while two or more stay | The behavior changes. The key stays. |
| None | `routers/order` (a list of Router UIDs) and `defaultRouter` | **New keys.** |
| Sections `router-levels`, `router-matrix`, `router-names`, `takes`, `protects` | Each field moves to the section `routers` | **Section names go.** The saves record, Settings search and a bookmark to a setting use them. A bookmark to an old setting must keep working or show Missing. |

### 4.4 The hello and the `config` message

| Today | Proposed |
| --- | --- |
| The settings of router A are flat keys of the hello. Each `routers[]` element has only some fields. | Each `routers[]` element carries every router setting. The flat keys of router A go. **Sent names change.** |
| `protected` and `protectBy` at the top are router A's. Each later router has its own in `routers[]`. | Every router has `protected`, `protectBy` and `protectDevice` in `routers[]`. The two top-level fields go. **Sent names change.** docs/persisted-names.md records the present split. |
| None | `defaultRouter` (a Router UID) at the top level. **New.** |

A page reloads when the build label changes, so the browser needs no old read path.

### 4.5 `/api/health` and `/api/v1/status`

| Field | Today | Proposed |
| --- | --- | --- |
| `state`, `routers[]` | The state of Sode as a whole and one element for each router | No change. These are the truth for a monitor. |
| `/api/health`: top-level `router`, `targetAddress`, `err` | Router A's | Keep the names. They become the default router's. Mark them as old in docs/api.md. |
| `/api/v1/status`: flat `router`, `routerUp`, `error`, `address`, `err` | Router A's | The same. |
| `routers[]` element | `uid`, `prefix`, `title`, and the link fields | Add `default: true` on one element. **New field.** |

A monitor that reads only the top-level `router` gets "up" while Sode is Degraded (R-11). The HTTP status is 503 then, so a monitor that reads the status code is correct. I do not recommend a silent change of meaning to "worst of all". See question 3.

`GET /api/v1/routers` returns the routers in the order of Settings, as today.

### 4.6 Text that changes, with no name change

- Validation errors always name the router by prefix and title when Sode has two or more (R-10, LP-15). The address error says "address".
- Log lines name the router: `names: B Studio B: 200 sources, 120 destinations` (R-11).
- docs/tielines-design.md: "Router prefix and typing" rule 5 says "the default router", and "Other stores" rule 4 loses "The first router cannot be deleted."
- GLOSSARY.md gets a **Default router** term.

## 5. Phased plan

Each step is small, can merge by itself, and leaves Sode in a state that can ship.

**Phase 0: fixes with no rename.** These three tracks touch different files, so they can run in parallel.

- 0a. Names and safety (`settingssave.js`, `savereview.js`, `swap.js`, `config.validateRouters`, `settingstielines.js`): the reconnect Confirm names the right router and address (R-05). Review rows and errors always have the prefix (R-10). Swap has a router choice (R-06). The label is "Router title" (R-09). Tie line Remove asks for a Confirm (R-18). Each router row shows its link state in words (R-14).
- 0b. Protects list (`settingsprotects.js`): the bounded list, groups, find, filters, counts, Free in the row with its result, and Free Sode's Protects by router (1.4). The list stays where it is until phase 3 moves it.
- 0c. Status bar (`connview.js`, `routerlinkrules.js`, `statusinfo.js`, `degradedview.js`, `mobile.js`): lost prefixes in the bar, the list on click with a fixed height, the one-sentence Degraded note, and the phone header (section 3). The "Router settings" link goes to the present row until phase 3.

**Phase 1: one list in memory.** `Config.Routers []Router`, with no change to a file or a message. A shim keeps the flat JSON. This is one serial step and it blocks phases 2 to 4. Server only.

**Phase 2: one shape on the wire.** This phase needs the owner's approval of the names in section 4. Every router setting gets a `router/<uid>/<attribute>` key, and each `routers[]` element of the hello carries every setting and the Protect fields. The page keeps the old layout and reads the new shape. After this step every router can set every field, router A too, through the same code.

**Phase 3: the list and the detail view.**

- The new Routers section (2.2, 2.3) with the Levels group (2.5).
- Routers > Protects as its own section.
- The sidebar tree and Settings search by router.
- The block of router A and the Router levels section go.

**Phase 4: order and the default router.** `defaultRouter`, `routers/order`, Remove for every router, and the doc changes in 4.6. Then the flat health and status fields follow the default router (4.5).

**Phase 5: tie lines.** The table, the filter, Add at the top, Edit in the row and find by name (2.6). It can run in parallel with phases 3 and 4, after phase 2.

**Phase 6: follow-up items.** These are independent and can run at any time after phase 1.

- Level review values in the numbering of the router, and blank Default levels (LP-14, LP-22).
- Level colors labels (LP-17).
- Log lines (R-11).
- The wizard title (R-27).

Order and parallel work: 0a, 0b and 0c in parallel. Then 1, then 2. Then 3 and 5 in parallel. Then 4. Phase 6 items go where they fit.

## 6. Open questions for the owner

1. **Default router.** Is one explicit setting (`defaultRouter`, a Router UID) correct, with the list order free to change? The alternative is "the first router of the list is the default", with reorder only.
2. **Names.** Do you approve the changes in 4.3 and 4.4: the flat save keys and hello fields of router A go, and each new `router/<uid>/<attribute>` key takes the JSON name that Sode sends today?
3. **Health.** Do the top-level `router` and `targetAddress` of `/api/health` stay, as the default router's? Or do they go at the next breaking release, so a monitor must read `state`?
4. **Status label.** Is it a setting of each router, as the prototype shows? Or is it one setting for the bar?
5. **Tie lines and the one Save.** Do tie line changes stay instant, with a Confirm on Remove? Or do they join the one Save, with a review and a PIN recheck?
6. **Color.** Three points follow from "green is for panels and a successful Take". "Connected" stays plain text with an icon, with no green. "Protect freed" is a neutral result. A Protect of another device becomes plain text with a magenta shield, not an amber badge. Do you agree with all three?
7. **Level colors.** Is one palette by position for every router correct, or do you want a palette for each router?

## 7. Prototypes

All three are static files with fake data. Each one makes no outside request and has a Theme switch for light and dark. They copy the color tokens from `web/vendor/patchbay/css/patchbay.tokens.css`, the sizes from `web/base.css` and the icons from the vendored Lucide sprite. The font stack is Sode's, and the files do not embed the font.

| File | What it shows |
| --- | --- |
| [prototypes/index.html](prototypes/index.html) | Links to the three pages. |
| [prototypes/routers.html](prototypes/routers.html) | The list and the detail view with 1, 2, 12 and 24 routers and mixed link states. Select, find, add, remove and reorder routers (drag, buttons or Alt+Up and Alt+Down). Set the default router, edit fields with their checks, and read the Save review. |
| [prototypes/protects.html](prototypes/protects.html) | 158 Protects and Router locks on 4 routers. One router is down and one has Protect off. Find, filter, Free in the row (B.9 shows a refusal), and Free Sode's Protects by router. |
| [prototypes/statusbar.html](prototypes/statusbar.html) | The bar with 1, 2, 3, 4, 12 and 24 routers, in three states and two layouts. A lost prefix opens the list, and "Router settings" opens routers.html on that router with the same states. |

The prototypes were run in headless Chromium at 1440, 1280 and 390 px, in both themes. A script checked each flow in the table.

The prototypes do not show the tie line table, the PIN recheck or the Lost link banner. They use a wide native scroll bar in place of the real big scroll bar.
