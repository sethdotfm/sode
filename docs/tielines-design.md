DISCLOSURE: This was written by an LLM.

# Several routers and tie lines (design, version 2)

This file holds the decisions for several routers on one Sode server and for tie lines between them. It is a draft that the owner and agents edit together. The spec issues come from it later. GLOSSARY.md has the terms: Router, Tie line, Tie line name, One-off tie line, Hop, Affected destination, Via, Reserved tie line and Tie line picker.

Target: Sode v2 (`integration/v2`, parent spec #543). v2 is not an alpha yet, so this work can change stored files and the API without a migration path from v1.

## The problem

A facility has two stages. Stage 1 has a large router (17 levels) and several small 4x8 routers. Stage 2 has a router with one level. Bridge apps make each of these routers speak SW-P-08. Today a user opens one Sode server for each router. To send a camera from stage 1 to stage 2, the user patches the camera to a tie line on one server. Then the user patches that tie line to the destination on the other server.

With this design, the user types `A.1` to `B.42` on one Sode page and does one take. Sode finds the tie line and sends both hops.

## Routers

1. A Router is one SW-P-08 matrix at one address. Two matrices at one address are two routers. A backup address for the same matrix is a setting of one router.
2. Every router speaks SW-P-08. Bridge apps convert other protocols, and Sode does not.
3. The owner reports that the bridge apps act as an Ultrix does: extended commands, announced changes, Protect and names all work. Sode depends on a router that sends no Connected message for a connect that changes nothing (docs/agents/router-verified.md). On 2026-11-22 the Videohub bridge sent only an ACK for such a connect, as the Ultrix does. Nobody has tested the openGear bridge yet.
4. Each router has its own state, link, timers and External tracking. A change to the settings of one router cuts takes in flight only on that router. An External follow never crosses routers.

### Health and status

1. Sode as a whole is Healthy, Degraded or Down. Healthy means that every router is up. Degraded means that one or more routers are down and one or more are up. Down means that every router is down.
2. `/api/health` answers 200 only when Sode is Healthy. It answers 503 when Sode is Degraded or Down.
3. The body of `/api/health` gives the state of Sode as a whole and has one element for each router, with its own state.
4. The status topic of the API stream and `GET /status` use the same shape: the state as a whole and one element for each router.

### Router prefix and typing

1. A Router prefix has one or two letters: A, B, BA, AA. With one router, Sode hides the prefix.
2. The dot is optional when a person types. `A11` and `A.11` mean the same thing. Sode always shows the dot.
3. Letters that are a router prefix, followed by digits, always mean that router first. Name matches come after the prefix match. Thus `B42` stays B.42 when someone names a source "B42 MON".
4. With two or more routers, each match in every name search shows its router prefix.
5. Each patch box has a default router, set for each browser and shown in the box. A bare number means that router. A new browser gets the first router.
6. With two or more routers, History and presets always show the prefix.
7. docs/api.md says that a prefix is one letter, A to Z. Change it to one or two letters.

### Levels

1. Levels belong to a router. Each router has its own level count, level names and default levels.
2. Rights `levels`, the level subset of a panel and level names say which router they mean. Rights `levels` and a panel's `settings.levels` are lists of levels under the router UID. A router that they do not name is not limited, so a limit on router A does not limit router B. A cell's levels are levels of the router of the cell.
3. On a route, the typed levels are levels of the router of the destination.
4. A router with `levels: 1` cannot break away. On that router, any typed levels mean its one level, which is the whole signal. Sode does not refuse "1-17" to such a destination. History says "all levels".
5. A router with more than one level keeps the rule of today: Sode refuses a level past the count.

### Link state on the page

1. The top bar shows the link and the uptime of every router, not of one router.
2. When Sode is Degraded, the full red banner does not show. The icon of the down router turns red, and the page shows which destinations cannot be patched now.
3. The banner shows only when the Server link is down or Sode is Down.
4. In Focus mode, only the icon of a down router shows. The full status shows again only when the banner would show.

### Rights matching

Rights checks compare the router and the number of each reference. A right for source 5 on router A does not allow source 5 on router B. Hidden Rights use the same match. docs/agents/server.md lists the other open items under "Room for a second router".

## Wire and API shape

1. Each end of a route is a router reference, `{router, n}`. A route can have its source on one router and its destination on another.
2. The `router` query parameter stays for reads about one router. With two or more routers, a read that needs a router and has none gets 400 `router_needed`.
3. The server works out paths, cost, Affected destinations and Via. The page does not. The page, the API reply and hidden visibility then all use one result.
4. The hello carries every router: its UID, prefix, title, levels, names and tally.

## Tie lines

1. A Tie line is a one-way cable from a destination on one router to a source on another router. A two-way tie is two tie lines.
2. A tie line carries all levels, since it is SDI or SMPTE 2110. Sode does not map levels across a tie line.
3. A tie line has a hidden id, as presets and filter sets have. Presets and panels store the id. `config.yaml` keeps the list under `tieLines`: `id`, `name`, `from` (the destination) and `to` (the source), each end a router reference. `GET /tielines` uses the same names.
4. A tie line has a Tie line name, unique on the server. When an Administrator accepts or adds a tie line, Sode fills the name from the name of the downstream source. The Administrator can change it.
5. An Administrator adds, edits and removes tie lines in Settings > Routers > Tie lines.
6. Sode suggests a tie line when a destination on one router has the same name as a source on another router. Sode trims the names and ignores case. An Administrator accepts each suggestion. Sode does not use a suggestion that nobody accepted.
7. An Administrator can also pair any destination with any source by hand. Sode cannot see the signal, so it trusts each pair.
8. Sode stores a tie line by router reference and number. If the name of either end changes, Sode marks the tie line Name moved, and it continues to work. The Tie line name does not change. The mark goes when the old name comes back. It also goes when an Administrator saves that tie line again in its edit form, which stores the names now. A save of a different tie line does not change the mark (agent decision).
9. The ends of a tie line stay ordinary sources and destinations. The UI and the API can patch them directly.
10. A change of a tie line is any crosspoint change on its upstream destination, on any level. Sode or an External can make it.

### One-off tie lines

1. In the Tie line picker, a user can state that a destination feeds a source for this take only. An example is a fibre that is normally used for something else.
2. A user with the "Use tie lines" right can do this. The user needs no Rights on the upstream destination that they state (owner decision).
3. The Protect check covers the stated upstream destination.
4. Sode does not know what the stated upstream destination feeds. The picker shows its Affected destinations as unknown, not 0.
5. The History entry stores the two stated ends and who stated them.
6. Sode keeps the one-off tie line in memory while its downstream source is routed to a destination. While it does, Via, Undo and Redo work as for a saved tie line.
7. An Administrator can press "Save as tie line" to keep it.
8. The API takes `{from, to}` in place of a tie line id.
9. The picker offers a One-off tie line between the router of the source and the router of the destination, as one step. Its name is "One-off" and its upstream destination, such as "One-off A.12". Without `expect`, Sode does not compare it, since Sode can have no state for that destination. Ends of a saved tie line are that tie line (agent decisions).
10. An Administrator presses "Save as tie line" in Settings > Routers > Tie lines, which lists the One-off tie lines that Sode keeps now. Sode fills the name from the name of the downstream source (agent decision).

## Routes through tie lines

### Hops and levels

1. A route through tie lines has one Hop for each router on the path. A.1 to B.42 through one tie line has two hops.
2. The last hop switches the typed levels. Each hop before it switches all levels of its router.
3. A path never visits a router two times.
4. There is no limit on the number of hops. The routers can make any network, not only a line.
5. Sode leaves out each path through a router that is down.

### Affected destinations and cost

1. An Affected destination is a destination, other than the one asked for, whose signal changes on any level.
2. Sode traces the change downstream through every tie line, on every router. It counts each destination one time.
3. If the downstream chain comes back to a tie line that it already passed, Sode stops there and shows a loop mark (∞).
4. If a router downstream is down, or Sode has no state for it, that part shows as "unknown (router C down)".
5. The picker opens at once and uses the tally that Sode has. The bridges announce changes, so the tally is current. A count is marked stale only when its router connected again after the last full read.
6. The cost of a path is the number of Affected destinations plus one for each hop. The weight of a hop is a setting (`tieHopWeight`, Settings > Routers > Tie lines, default 1). It counts each hop that sends upstream. A path that sends nothing upstream, because its tie lines already carry the wanted source, has cost 0.
7. Sode sorts whole paths, not single tie lines, by these keys in order:
   1. A path with no unknown part comes before a path with an unknown part.
   2. A path where no tie line is Protected or Reserved by a panel comes before one where a tie line is.
   3. The lower cost comes first.
   4. A path where no tie line changed in the last 5 minutes comes before one where a tie line did.
   5. The path whose newest tie line change is older comes first. Thus a stale tie line comes before one that changed recently (owner decision).
8. The picker opens even when a path has cost 0. A user can want a second tie line with the same source, because the first one will change later in the show.

### Last change of a tie line

1. Sode stores the time of the last change of each tie line on disk, next to the saved names table, so it survives a restart. The file is `names/tielines.changes.yaml`. Any crosspoint change on the upstream destination, on any level, by Sode or an External, stamps it. The first read of a level is a baseline and stamps nothing. A change to the ends of a tie line removes its stamp, since the stamp is about the old cable.
2. If Sode was down after that time, the picker shows "before HH:MM". Sode never claims more than it knows.
3. A found entry gives the time when Sode found the change, also as "before HH:MM".

### Via

1. Everywhere that Sode shows the source of a destination, it traces the tie lines back to the first source: "A.1 CAM 1 via TIE 3". This applies to the patch boxes, History, panels, Grid crosspoints and the API.
2. A chain of two tie lines shows as "CAM 1 via TIE 3, TIE 9". A Grid crosspoint has no space for this. It shows the first source and a Via mark, with the full chain in the tip.
3. Sode uses the word "via" for nothing else.
4. Via resolves for each level. If the levels upstream have different sources, Sode shows "CAM 1 via TIE 3 (mixed)" with the level pie. A mixed tie line never counts as one that already carries the source.
5. A tie line whose upstream router has one level always carries one source on all levels.
6. If the upstream router is down, the destination shows "unknown via TIE 3".
7. If the upstream destination of a tie line has no source, the destination shows "nothing via TIE 3".
8. If the chain upstream loops, the destination shows the loop mark.
9. The Source box shows everything that the source feeds now, through tie lines too.
10. The Destination box shows the full Via chain of what it receives.
11. The Grid view uses the same rules as the patch boxes.

### API tally and stream

1. `src` stays the real crosspoint on the router of the destination, so old clients stay correct.
2. Each destination gains `via` (the tie lines in order) and `origin` (the router and source at the start of the chain).
3. When a tie line changes upstream, Sode sends a tally frame for every destination downstream of it.
4. For a caller with hidden visibility, Sode blanks an origin that the caller cannot see and keeps the Tie line name.

## The Tie line picker

1. The picker opens before a take that has a route across routers. It fills a large part of the screen.
2. It shows the source on the left, a box for each router and the wires between them. Each hop is a step. The chosen tie line lights its wire.
3. For each choice, the picker shows:
   - what the tie line carries now and what it will carry
   - a timer that counts up from the last change
   - each Affected destination, with "now shows X, will show Y"
   - "Reserved by panel X" or the Protect, if there is one
   - its state at a glance. "Has it" with a routed left bar: it carries the source already. A caution left bar and a count badge ("3 change"): the take changes Affected destinations. "Free" with no color: it changes nothing else. A Protected or Reserved tie line is grey. The best choice has an outline and "Best".
4. Keys: J and K or the up and down arrows choose a tie line. H and L or Tab and Shift+Tab move between hops. P protects. Enter sends the take. Esc cancels.
5. The mouse and touch do everything that the keys do. A tap on a wire chooses it. A shield button protects.
6. A change on one hop makes Sode calculate the later hops again.
7. The picker updates while it is open. If the cost, the Protect state or a link of the chosen path changes, the 500 ms lockout starts again. The picker marks the changed row. It never moves the selection by itself.
8. An open picker holds nothing. Other users and panels can use every tie line while it is open.
9. For a take with only tie routes, the picker replaces the churning-letter confirm. It keeps the 500 ms lockout.
10. If no path is usable, the picker names each reason: no tie line between these routers, a router down, every tie line Protected. While the routers are up, it still offers a One-off tie line.

### Takes with plain routes and tie routes

1. The picker shows the tie routes first. It asks one question for each hop that the tie routes share. A hop is shared when the routes have the same source, levels and choices before it. The hop must also go from and to the same routers.
2. One source to many destinations on router B is one question: "A → B, for B.1 B.2 B.3". Each destination uses that choice. Enter on the best choice sends.
3. Destinations on B and C get one question for the hop A → B, and one for the hop B → C. A route that needs a hop that the others do not have gets its own question.
4. The picker asks each question every time, also when a tie line carries the source already. The best choice is chosen first, so the question needs one Enter.
5. "Choose per destination" (D) gives each destination of a hop its own question. It is off by default.
6. Each question has a "Clear this route" button, which drops the routes of that question.
7. Then the churning-letter confirm lists every route of the take. It marks the tie routes "via TIE n".
8. In one take, Sode does not offer a tie line that an earlier route of the same take uses for a different source.
9. Two destinations that want the same source share one tie line by default.
10. Hops do not count against the limit of 8 routes in one wire `take`. The send queue of each router counts them.
11. The page still never calculates paths. A question that comes after a hop asks the plan with `through`. The server then gives only the paths through the tie lines that the person chose before.
12. A change that the live update finds on the path of any destination of a question marks that destination and starts the lockout again.

## Sending a tie take

### The stale check

1. The take carries, for each tie line, the source that the picker showed on it. It also carries the Affected destinations that the user saw.
2. The server takes a lock on every tie line of the whole take before it sends anything. It takes the locks in the order of the tie line ids, so two takes cannot wait on each other. It frees them when the take ends.
3. A plain Sode take to the upstream destination of a tie line takes the same lock.
4. After the pre-take check and before the first connect, the server compares each tie line with what the picker showed.
5. A tie line can move, or gain Affected destinations that the user did not see. Then the server refuses the whole take before it sends anything. The page shows "TIE 3 changed", and the picker opens again. The server never sends the old choice.
6. The compare uses these rules:
   - Sode compares every level of a tie line. A move on any level is a change. A mixed tie line never matches one expected source (Via, rule 4).
   - The expected source is one source for all levels, or nothing. For a mixed tie line, it is a list with the source of each level, or nothing. The design names no form for the mixed state, so the API adds this list (agent decision).
   - A null expected source and an expected source that is not there both mean nothing routed.
   - An API caller that sends no Affected destinations gets no compare of them.
   - Sode compares only the Affected destinations that the caller can see. The caller cannot name the others.

### Order and failures

1. A take sends each different upstream hop one time, before every route that needs it. Each route that shares a hop stores the same hop record.
2. Sode sends the hops from upstream to downstream. If a hop fails, Sode does not send the hops after it.
3. Tie routes always use ordinary connects. In `fireMode` native, Sode still sends tie routes this way.
4. Before Sode sends anything, it runs the Protect check on every hop, Router locks too. If one hop fails the check, Sode refuses that route. The other routes of the take go, as today.
5. The Result of a tie route comes from all its hops. If the last hop is Already and an upstream hop changed, the route is a normal success.
6. If a hop that is not the last fails, the History row says "TIE 3 changed, B.42 not". The row offers "Put TIE 3 back to CAM 9". That works only while the tie line still carries what this take set. Put back is the Undo of the entry of the hop that changed TIE 3, on the levels that the hop changed. The API has `POST /history/{id}/putback` for it (agent decision).
7. A Late answer on a hop that is not the last never starts the later hops. The row says "TIE 3 changed late" and offers the same put-back action.
8. Retry sends only the failed hops, if the earlier hops still hold. Otherwise Retry opens the picker, and the late tie line has cost 0. The API answers 409 `tie_line_changed` for the picker. A hop that failed is not compared, since Sode sends it again (agent decision).
9. If the link of one router drops during a take, the take fails at the hop on that router, with the same put-back action.

### Protect in a tie take

1. A user with the Protect right can press P or the shield button on a hop. Sode then protects the upstream destination of that tie line.
2. Sode refuses its own take to a destination that it Protected. So Sode sets the Protect after every hop confirmed, and only then.
3. The Protect is its own entry in the same History Group. If the Protect fails, the take stays a success, and the Protect entry shows the failure with its `Still` levels.
4. A Protected tie line shows who set it and when. It is grey and a user cannot choose it.
5. A Protected tie line that already carries the wanted source stays available for the downstream hop, since Sode sends nothing upstream.
6. A user frees the Protect with Free Protect, as for any destination.
7. If a route that asked for the Protect fails, Sode does not send the Protect. Its entry says so, so the Group keeps its size (agent decision).
8. P and the shield act on the chosen tie line of their step. A second press stops the Protect. A tie line that has a Protect cannot get a second one (agent decision).

## Rights

1. Rights apply to the typed source and the typed destination only. Hops through saved tie lines need no Rights.
2. A new right, "Use tie lines", is on by default for users. Without it, a user cannot send a route across routers.
3. For a Guest, the Access preset writes the right: on under Open access, off under Standard access and Secured access.
4. Hops ignore Rights `levels`.
5. For a user with hidden visibility, the picker shows "N others" in place of names that the user cannot see.

## History, Undo, Redo, Retry

1. A tie take is one History Group. A tie take of one destination asked for is not a Group for stacking, so its row can stack.
2. History shows one row for each destination asked for: "B.42 CAM 1 via TIE 3". The row opens to show each hop and what each tie line carried before.
3. Each entry keeps the router of the destination, the router of the source and the list of hops. Each hop has its own Result and Prev.
4. Filters match the typed source and destination, never the hops.
5. A tie row never stacks with a row that used a different tie line.
6. History records only real changes on a router. If someone else changes TIE 3 on router A, that is an External on A.42. The entry notes "also changes B.42, B.50 via TIE 3", worked out at that time. For a caller with hidden visibility, `view().Entry` removes each destination in that note that the caller cannot see. `alsoOthers` counts them, as `others` in a 409 body.

### Undo

1. Undo puts back the destination asked for. It never switches a tie line back by itself, since a tie line can have new users after the take.
2. The entry stores the full Via chain from before the take.
3. The chain can differ only because this take switched a tie line that still carries what the take set. Then the Undo confirm offers "Put TIE 3 back to CAM 9" with its Affected destinations.
4. If the chain differs for any other reason, Undo sends nothing. It says "TIE 5 now carries CAM 2, not CAM 9" and offers to open the picker for CAM 9.
5. Ctrl+Z on a tie row opens the Undo confirm. It never sends without it.
6. The panel Undo cell uses the same rules. It has no confirm that can offer a put-back or the picker, so it refuses such an Undo and says why. `POST /history/{id}/undo` does the same, with 409 `tie_line_changed` (agent decision).
7. The put-back that the Undo confirm offers is off until the person ticks it. An undone tie row says "tie lines left as they are" (agent decision).

### Redo and Retry

1. Redo opens the picker. It chooses the old tie lines while they are usable, also when they cost more now than the cheapest. It marks them "used last time". If an old tie line is gone or not usable, the picker keeps the choice of the planner for that step and says why. Ctrl+Shift+Z on a tie row does the same. The lockout and the confirm stay (agent decision, from the plan of #808; it replaces "the cheapest first").
2. For Retry, see "Order and failures" above.

## Panels

1. A panel has a tie line setting: Off or Reserved.
2. The server applies the setting when the Actor is a panel, also for a panel that acts as an API client. Panel settings are otherwise page conveniences, so this is a new server rule.
3. Off refuses every route across routers.
4. Reserved names one tie line for each one-way link between two routers that the panel crosses. The panel always uses it, even if another path costs less.
5. A path through more than one link needs a Reserved tie line on each link. If one is missing, the panel refuses the press.
6. One tie line can be Reserved by one panel only. Sode checks this when the panel is saved.
7. Push, templates, Save a copy, checkpoints and Checkpoint sets leave the tie line setting out, as they do fixed destinations.
8. A Reserved tie line is not Protected. Everywhere else, it shows "Reserved by panel X". In the picker it sorts with the Protected tie lines. Any other user who chooses it confirms one more time.
9. If someone else Protected the Reserved tie line, the panel refuses the route and says who holds it.
10. If one press needs two sources through one Reserved tie line, the panel refuses the whole press and says why. Fixed destination refusals work the same way.
11. A panel in Take mode shows the Affected destinations in its confirm. A panel in Instant mode sends on its Reserved tie line with no confirm.
12. A choice of tie line for each button press is out of scope.
13. The setting is `tieLines` on the panel (`mode` `off` or `reserved`, and the `reserved` ids), beside the layout and not in `settings`. Thus rule 7 holds by itself. Save a copy and an Inbox card drop it. An import drops each tie line that another panel Reserves (agent decision).
14. A panel asks the plan of its Reserved tie lines with `POST /takes/plan`, which answers 409 `tie_line_needed`. It takes nothing and writes no Communication Log row. It then sends the ties of that plan with what it showed. A Buttons panel has no confirm box, so in Take mode the first Take shows the Affected destinations and a second Take sends (agent decision).

## Presets

1. A preset entry across routers stores the ids of the tie lines that the user chose when the entry was added. A change to the entry opens the picker.
2. A preset with a tie route always opens a tie confirm page. No confirm setting skips it.
3. For each stored tie line, the page shows what it carries now, what it will carry, the last change and every Affected destination.
4. If a stored tie line is now Protected, or Reserved by a panel, the page marks it. Enter stays off until each such route is confirmed or cleared.
5. A deleted tie line makes the preset Broken, with a new `tie` kind. `RefWalk` and `TestWalkRefsSeesEverySlot` gain the slot.
6. A panel Preset cell sends the stored tie lines with the take, under the Reserved rule if the panel has one.
7. A preset item stores the tie line ids in `ties`, upstream first. The destinations of one item share one router. An item across routers needs `ties`, and an item on one router has none. A Broken mark of kind `tie` names the id in `tie`. Sode sets and clears these marks from the tie lines each time it saves the settings (agent decision).
8. The tie confirm page is the Tie line picker, opened on the stored path. The person can choose other tie lines for that fire only. The preset does not change. Take stays off on a Protected stored tie line, and a Reserved one asks one more time (Panels, rule 8). The preset editor opens the picker from the Tie lines… button of a row. It also opens it when an add from the patch boxes or the Grid popup crosses routers. There it chooses only saved tie lines (agent decision).
9. Under the Reserved rule, the stored tie lines of a Preset cell must be the path of the Reserved tie lines of the panel. If they are not, Sode refuses the press and sends nothing. The panel never sends on other tie lines (agent decision).

## API

1. Each route of `POST /takes` accepts an optional `ties` list: one tie line for each hop before the last, upstream first. Each has the source that the caller expects on it (`{id, expect}`). A One-off tie line is `{from, to}`.
2. Without `ties`, Sode refuses a route across routers with 409 `tie_line_needed`. The reply lists the ranked paths, cheapest first. A route with `through` (tie line ids, upstream first) gets only the paths whose first tie lines are these.
3. A preset fire with tie lines answers 409 `tie_confirm_needed` with the state of each tie line. A request that names each tie line and its expected source goes through.
4. If a tie line moved, Sode answers 409 `tie_line_changed` with the state of each tie line now.
5. A panel at Off, or a caller without the "Use tie lines" right, gets 403 `tie_lines_off`.
6. If no path is usable, Sode answers 409 `no_path` with each reason.
7. Every such reply blanks the names that a caller with hidden visibility cannot see. A hop end that the caller cannot see is null, with no change time, as in `GET /tielines`. The origin of what a tie line carries stays, since the caller sends it back as `expect`.
8. An Idempotency-Key replay returns the first answer, as for any take.
9. `GET /tielines` lists each tie line with its name, what it carries, the last change, and any Protect or Reserve.
10. openapi.yaml gains every new route and error code in the same issue.

## Other stores

1. Swap does not move a tie line end by itself, since a tie line is a cable. The Swap review row lists each tie line that uses a swapped number, and the Administrator chooses. Each tie line starts not ticked. A ticked tie line moves its end to the new number. A user's own Swap never lists a tie line (agent decision).
2. Tie lines are their own export category. On import, Sode drops a tie line with a note when one of its two routers is not mapped.
3. Before the Critical confirm, a router delete lists every tie line, panel and preset that it will break. The delete also deletes each tie line with an end on that router. Validate refuses a tie line with a router that Sode does not have.
4. A router delete marks each preset, filter set, panel cell and Grid member that names the router Broken. Rights and a panel's level subset lose that router, since a Rights entry cannot name a router that Sode does not have. The first router cannot be deleted. An Administrator changes its address in its own block.

## Open items

1. Test the openGear bridge for a Connected message on a connect that changes nothing. (Routers, item 3.)
2. How to split the work into issues. The owner expects one large spec with many issues.
