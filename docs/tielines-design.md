DISCLOSURE: This was written by an LLM.

# Several routers and tie lines (design, version 1)

This file holds the decisions for several routers on one Sode server and for tie lines between them. It is a draft that the owner and agents edit together. The spec issues come from it later. GLOSSARY.md has the terms: Router, Tie line, One-off tie line, Hop, Affected destination, Via, Reserved tie line and Tie line picker.

Target: Sode v2 (`integration/v2`, parent spec #543). v2 is not an alpha yet, so this work can change stored files and the API without a migration path from v1.

## The problem

A facility has two stages. Stage 1 has a large router (17 levels) and several small 4x8 routers. Stage 2 has a router with one level. Bridge apps make each of these routers speak SW-P-08. Today a user opens one Sode server for each router. To send a camera from stage 1 to stage 2, the user patches the camera to a tie line on one server. Then the user patches that tie line to the destination on the other server.

With this design, the user types `A.1` to `B.42` on one Sode page and does one take. Sode finds the tie line and sends both hops.

## Routers

1. A Router is one SW-P-08 matrix at one address. Two matrices at one address are two routers. A backup address for the same matrix is a setting of one router.
2. Every router speaks SW-P-08. Bridge apps convert other protocols, and Sode does not.
3. The owner reports that the bridge apps act as an Ultrix does: extended commands, announced changes, Protect and names all work. One fact is not known: does a bridge send a Connected message for a connect that changes nothing? Sode depends on the Ultrix not sending it (docs/agents/router-verified.md). Add a swp08sim mode for each answer, and test both before the spec is final.
4. Each router has its own state, link, timers and External tracking. A change to the settings of one router cuts takes in flight only on that router. An External follow never crosses routers.
5. `/api/health` answers 200 while one or more routers are up. The body gives the state of each router. A query parameter asks about one router.

### Router prefix and typing

1. A Router prefix has one or two letters: A, B, BA, AA. With one router, Sode hides the prefix.
2. The dot is optional when a person types. `A11` and `A.11` mean the same thing. Sode always shows the dot.
3. A dot-less form counts as a prefix only when no name matches better. The match list shows both choices.
4. Each patch box has a default router, set for each browser and shown in the box. A bare number means that router.
5. docs/api.md says that a prefix is one letter, A to Z. Change it to one or two letters.

### Levels

1. Levels belong to a router. Each router has its own level count, level names and default levels.
2. Rights `levels`, the level subset of a panel and level names say which router they mean.
3. On a route, the typed levels are levels of the router of the destination.
4. A router with `levels: 1` cannot break away. On that router, any typed levels mean its one level, which is the whole signal. Sode does not refuse "1-17" to such a destination. History says "all levels".
5. A router with more than one level keeps the rule of today: Sode refuses a level past the count.

### Link state on the page

1. The top bar shows the link and the uptime of every router, not of one router.
2. When one router is down and others are up, Sode is degraded. The full red banner does not show. The icon of that router turns red, and the page shows which destinations cannot be patched now.
3. The banner shows only when the Server link is down or every router is down.
4. In Focus mode, only the icon of a down router shows. The full status shows again only when the banner would show.
5. GLOSSARY.md needs a term for the degraded state, and the Reconnecting banner row must name more than one Router link.

### Rights matching

`Rights.Allows*` and `refN` compare numbers only. With two routers, a right for source 5 would also allow source 5 on the other router. Fix this before the Add router button exists, not with tie lines. docs/agents/server.md lists the other open items under "Room for a second router".

## Tie lines

1. A Tie line is a one-way cable from a destination on one router to a source on another router. A two-way tie is two tie lines.
2. A tie line carries all levels, since it is SDI or SMPTE 2110. Sode does not map levels across a tie line.
3. A tie line has a hidden id, as presets and filter sets have. Presets and panels store the id.
4. An Administrator adds, edits and removes tie lines in Settings > Routers > Tie lines.
5. Sode suggests a tie line when a destination on one router has the same name as a source on another router. Sode trims the names and ignores case. An Administrator accepts each suggestion. Sode does not use a suggestion that nobody accepted.
6. An Administrator can also pair any destination with any source by hand. Sode cannot see the signal, so it trusts each pair.
7. Sode stores a tie line by router reference and number. A later name change marks it Name moved, and it continues to work.
8. The ends of a tie line stay ordinary sources and destinations. The UI and the API can patch them directly.

### One-off tie lines

1. In the Tie line picker, a user can state that a destination feeds a source for this take only. An example is a fibre that is normally used for something else.
2. Any user with the "Use tie lines" right can do this. The user needs no Rights on the upstream destination that they state (owner decision).
3. An Administrator can press "Save as tie line" to keep it.

## Routes through tie lines

### Hops and levels

1. A route through tie lines has one Hop for each router on the path. A.1 to B.42 through one tie line has two hops.
2. The last hop switches the typed levels. Each hop before it switches all levels of its router.
3. A path never visits a router two times.
4. There is no limit on the number of hops. The routers can make any network, not only a line.

### Affected destinations and cost

1. An Affected destination is a destination, other than the one asked for, whose signal changes on any level.
2. Sode traces the change downstream through every tie line, on every router. It counts each destination one time.
3. If the downstream chain comes back to a tie line that it already passed, Sode stops there and shows a loop mark (∞).
4. If a router downstream is down, or Sode has no state for it, that part shows as "unknown (router C down)". Any tie line with an unknown part ranks below every tie line with known state.
5. A count older than `checkFresh` shows as stale. Sode reads the routes downstream of each candidate tie line before the picker shows counts.
6. The cost of a path is the number of Affected destinations plus one for each hop. The weight of a hop is a setting.
7. Sode orders the candidates as follows:
   1. A tie line that already carries the wanted source, on all levels. Its cost is 0, and Sode sends nothing upstream.
   2. A tie line with no downstream users.
   3. The lowest cost.
   4. The oldest last change. A tie line that changed in the last 5 minutes ranks below every older one.
8. The picker opens even when a tie line already carries the wanted source. A user can want a second tie line with the same source, because the first one will change later in the show.

### Last change of a tie line

1. Sode stores the time of the last change of each tie line on disk, next to the saved names table, so it survives a restart.
2. If Sode was down after that time, the picker shows "before HH:MM". Sode never claims more than it knows.
3. A found entry gives the time when Sode found the change, also as "before HH:MM".

### Via

1. Everywhere that Sode shows the source of a destination, it traces the tie lines back to the first source: "A.1 CAM 1 via TIE 3". This applies to the patch boxes, History, panels, Grid crosspoints and the API.
2. Sode uses the word "via" for nothing else.
3. Via resolves for each level. If the levels upstream have different sources, Sode shows "CAM 1 via TIE 3 (mixed)" with the level pie. A mixed tie line never counts as one that already carries the source.
4. If the upstream router is down, the destination shows "unknown via TIE 3".
5. The Source box shows everything that the source feeds now, through tie lines too.
6. The Destination box shows the full Via chain of what it receives.
7. The Grid view uses the same rules as the patch boxes.

### API tally and stream

1. `src` stays the real crosspoint on the router of the destination, so old clients stay correct.
2. Each destination gains `via` (the tie lines in order) and `origin` (the router and source at the start of the chain).
3. When a tie line changes upstream, Sode sends a tally frame for every destination downstream of it.
4. For a caller with hidden visibility, Sode blanks an origin that the caller cannot see and keeps the tie line name.

## The Tie line picker

1. The picker opens before a take that has a route across routers. It fills a large part of the screen.
2. It shows the source on the left, a box for each router and the wires between them. Each hop is a step. The chosen tie line lights its wire.
3. For each choice, the picker shows:
   - what the tie line carries now and what it will carry
   - a timer that counts up from the last change
   - each Affected destination, with "now shows X, will show Y"
4. Keys: J and K or the up and down arrows choose a tie line. H and L or Tab and Shift+Tab move between hops. P protects. Enter sends the take. Esc cancels.
5. The mouse and touch do everything that the keys do. A tap on a wire chooses it. A shield button protects.
6. A change on one hop makes Sode calculate the later hops again.
7. The picker updates while it is open. If the cost, the Protect state or a link of the chosen path changes, the 500 ms lockout starts again. The picker marks the changed row. The picker never moves the selection by itself.
8. An open picker holds nothing. Other users and panels can use every tie line while it is open.
9. For a take with only tie routes, the picker replaces the churning-letter confirm. It keeps the 500 ms lockout.

### Takes with plain routes and tie routes

1. The picker shows the tie routes first. Each tie route has a "Clear this route" button.
2. Then the churning-letter confirm lists every route of the take. It marks the tie routes "via TIE n".
3. In one take, Sode does not offer a tie line that an earlier route of the same take uses for a different source.
4. Two destinations that want the same source share one tie line by default.
5. Hops do not count against the limit of 8 routes in one wire `take`. The send queue of each router counts them.

## Sending a tie take

### The stale check

1. The take carries, for each tie line, the source that the picker showed on it.
2. The server holds a lock on each tie line for the whole take, and for no longer.
3. After the pre-take check and before the first connect, the server compares each tie line with what the picker showed.
4. If a tie line moved, the server refuses the take with "TIE 3 changed" and the picker opens again. The server never sends the old choice.

### Order and failures

1. Sode sends the hops from upstream to downstream. If a hop fails, Sode does not send the hops after it.
2. Before Sode sends anything, it runs the Protect check on every hop, Router locks too. If one hop fails the check, Sode refuses the whole route.
3. If a hop that is not the last fails, the History row says "TIE 3 changed, B.42 not". The row offers "Put TIE 3 back to CAM 9". That works only while the tie line still carries what this take set.
4. A Late answer on a hop that is not the last never starts the later hops. The row says "TIE 3 changed late" and offers the same put-back action.
5. Retry sends only the failed hops, if the earlier hops still hold. Otherwise Retry opens the picker, and the late tie line has cost 0.
6. If the link of one router drops during a take, the take fails at the hop on that router, with the same put-back action.

### Protect in a tie take

1. A Trusted user can press P or the shield button on a hop. Sode then protects the upstream destination of that tie line.
2. Sode refuses its own take to a destination that it Protected. So Sode sets the Protect after every hop confirmed, and only then.
3. The Protect is its own entry in the same History Group.
4. A Protected tie line shows who set it and when. It is grey and a user cannot choose it.
5. A Protected tie line that already carries the wanted source stays available for the downstream hop, since Sode sends nothing upstream.
6. Each router has a "supports Protect" flag. The picker hides P where the router cannot set a Protect.
7. A user frees the Protect with Free Protect, as for any destination.

## Rights

1. Rights apply to the typed source and the typed destination only. Hops through saved tie lines need no Rights.
2. A new right, "Use tie lines", is on by default. Without it, a user cannot send a route across routers.
3. Hops ignore Rights `levels`.
4. For a user with hidden visibility, the picker shows "N others" in place of names that the user cannot see.

## History, Undo, Redo, Retry

1. A tie take is one History Group.
2. History shows one row for each destination asked for: "B.42 CAM 1 via TIE 3". The row opens to show each hop and what each tie line carried before.
3. Each entry keeps the router of the destination, the router of the source and the list of hops. Each hop has its own Result and Prev.
4. Filters match the typed source and destination, never the hops.
5. A tie row never stacks with a row that used a different tie line.
6. History records only real changes on a router. If someone else changes TIE 3 on router A, that is an External on A.42. The entry notes "also changes B.42, B.50 via TIE 3", worked out at that time.

### Undo

1. Undo puts back only the destination asked for. It never switches a tie line back. A tie line can have new users since the take, and Undo must not change their signal.
2. The entry stores the full Via chain from before the take.
3. If the chain now differs, Undo sends nothing. It says "TIE 5 now carries CAM 2, not CAM 9" and offers to open the picker for CAM 9.
4. Ctrl+Z on such a row opens that confirm. It never sends without it.
5. The panel Undo cell uses the same rules.

### Redo and Retry

1. Redo opens the picker. It chooses the old tie line first only if that tie line costs no more now than the cheapest. Otherwise it chooses the cheapest and marks the old one "used last time".
2. For Retry, see "Order and failures" above.

## Panels

1. A panel has a tie line setting: Off or Reserved.
2. Off refuses every route across routers.
3. Reserved names one tie line for each one-way link between two routers that the panel crosses. The panel always uses it, even if another path costs less.
4. A path through more than one link needs a Reserved tie line on each link. If one is missing, the panel refuses the press.
5. One tie line can be Reserved by one panel only. Sode checks this when the panel is saved.
6. A Reserved tie line is not Protected. Everywhere else, it shows "Reserved by panel X". Any other user who chooses it confirms one more time.
7. If someone else Protected the Reserved tie line, the panel refuses the route and says who holds it.
8. If one press needs two sources through one Reserved tie line, the panel refuses the whole press and says why. Fixed destination refusals work the same way.
9. A panel in Take mode shows the Affected destinations in its confirm.
10. A choice of tie line for each button press is out of scope.

## Presets

1. A preset entry across routers stores the ids of the tie lines that the user chose when the entry was added. A change to the entry opens the picker.
2. A preset with a tie route always opens a tie confirm page. No confirm setting skips it.
3. For each stored tie line, the page shows what it carries now, what it will carry, the last change and every Affected destination.
4. If a stored tie line is now Protected, or Reserved by a panel, the page marks it. Enter stays off until each such route is confirmed or cleared.
5. A deleted tie line makes the preset Broken, with a new `tie` kind. `RefWalk` and `TestWalkRefsSeesEverySlot` gain the slot.
6. A panel Preset cell sends the stored tie lines with the take, under the Reserved rule if the panel has one.

## API

1. `POST /takes` accepts an optional `ties` list, one tie line for each hop, with the source that the caller expects on each.
2. Without `ties`, Sode refuses a route across routers with 409 `tie_line_needed`. The reply lists the cheapest path.
3. A preset fire with tie lines answers 409 `tie_confirm_needed` with the state of each tie line. A request that names each tie line and its expected source goes through.
4. `GET /tielines` lists each tie line with what it carries, the last change, and any Protect or Reserve.

## Other stores

1. A checkpoint stores the first source of the chain for a destination fed through a tie line. If the chain differs on restore, Sode opens the tie confirm page.
2. Swap rewrites the ends of a tie line through `RefWalk`.
3. Tie lines are their own export category. On import, Sode drops a tie line with a note when one of its two routers is not mapped.
4. Before the Critical confirm, a router delete lists every tie line, panel and preset that it will break.

## Open items

1. Does each bridge send a Connected message for a connect that changes nothing? (Routers, item 3.)
2. A GLOSSARY.md term for the degraded state.
3. How to split the work into issues. The owner expects one large spec with many issues.
