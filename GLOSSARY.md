# Sode

Sode is a keyboard-first web page for patching an SW-P-08 router, built and tested on a Ross Ultrix. It exists to make patching about 1000 times faster and simpler for the people doing it. Today it drives one router. Multiple routers and tie lines are plausible later, after the core product is excellent.

## Language

### Routing

**Source**:
A router connection that signal comes from, shown by its number and the name the router gives it.
_Avoid_: input

**Destination**:
A router connection that signal goes to, shown by its number and the name the router gives it.
_Avoid_: output

**Level**:
One of the parallel signal types (such as video or audio) that a source can be connected to a destination on. Levels are numbered 1..N in Sode whatever the router uses on the wire.
_Avoid_: wire level, layer

**Router name**:
The name the router gives a source or destination. Sode cannot change it, and re-reads names from the router on a timer. Keep it apart from a user's name, a preset's name and a filter set's name.
_Avoid_: label, refresh (for the re-read)

**Route**:
One source connected to one destination on a set of levels. A take sends one or more routes.

**Protected**:
Describes a destination that the router will not change because something has protected it. Sode refuses a plan that includes a protected destination before sending anything.
_Avoid_: protect (as a noun), locked

### Actions

**Command**:
Anything a person or an API client fires that Sode acts on, such as a take, an undo, a retry, a router name re-read, clearing History or saving settings. Every command is logged.

**Confirm**:
The deliberate step that turns a plan into a take. In a browser it is the churning letter key and its short lockout. Over the API it is the take request itself, which the API client is responsible for guarding. No take happens without one.

**Take**:
A kind of command that executes a change on the router hardware by sending one or more routes. One confirm is one take. "Take" is a reserved word, so nothing that has not been sent to the router is a take.
_Avoid_: batch, using "take" for any other command, a plan or saved entry, or as an ordinary verb meaning "accept" or "interpret literally"

**Plan**:
What Sode intends to send but has not yet sent. A plan becomes a take when a person confirms it. Undo, Redo and Retry are plans: each one prepares a plan that does nothing until confirmed.
_Avoid_: pending take

**Preset**:
A saved plan that can be fired in one confirm, pinned and shared. It is Sode's own feature.
_Avoid_: salvo

**Salvo**:
The router's own stored group of routes, fired by number on the router. Sode does not use this word for anything of its own, so "fire salvo 12" always means the router's.

### Record

**History**:
The shared, chronological record of takes and External entries that every browser sees.
_Avoid_: Recent

**Group**:
The expandable row in History for a take that sent several routes. Undo all and Redo all act on a Group, undoing or repeating every route in that take.
_Avoid_: batch, folder

**Entry**:
One row in History: a route from a take, or an External.

**Before**:
What the router had on a destination just before a take. Undo restores it.

**Result**:
What the router did with one destination in a take. It connected the source, it already had it (Already), or it did not confirm (Failed). A Result can arrive Late.

**Already**:
A Result where the router already had that source on that destination, so nothing changed.

**Failed**:
A Result where the router did not confirm the change, or refused it. Retry prepares a plan for the Failed destinations.

**Late**:
A Result that the router confirmed after Sode had stopped waiting. Sode amends the Entry when it arrives.

**Filter**:
One rule on Source or Destination that limits which entries History shows.

**Filter set**:
A named, savable group of filters, scoped to Me, This browser or Everyone. Several sets can be stacked, and Stacked filters behavior says whether an entry must match all of them or any one.

**Communication Log**:
The page that shows every command and the raw messages Sode sent to the router, plus a router uptime graph.
_Avoid_: sent log

### Changes Sode did not make

**External**:
A change on the router that Sode did not make, such as one from a hardware panel, Companion or the router itself. Sode learns of it from the router and records it in History next to its own takes.
_Avoid_: external change, panel change, outside change

**Found**:
A kind of External the router made without announcing it, noticed by Sode only later.

**External follow**:
An External on a destination that probably followed another destination's change, such as a multiviewer PiP following a DSM. Sode cannot tell a follow from other automatic behavior, so this is always a suspicion.
_Avoid_: follows (as a certain fact), automation

### People and access

**User**:
A named person who can sign in to Sode. Their name is what History shows in place of an IP address. Changing a name is an advanced setting. History entries keep the name from when they were made, and the user's Profile is renamed with them.
_Avoid_: operator, account

**Sign in** / **Sign out**:
Sign in is how a user identifies themself. Sign out ends that session. Whether a page requires sign-in is set per page type.
_Avoid_: log in, log out, unlock (for a user's session)

**Sign-in mode**:
How a user signs in. PIN only is the default, where the PIN alone says who is signing in, so every PIN is unique. Username and PIN is an optional mode for deployments that want it, where the username is separate from the name History shows.

**API client**:
A named, non-person actor that uses Sode's API with an API key, such as a button panel app. Its name is what History shows in place of a user's name. Managing API clients is an advanced setting.

**API key**:
The secret an API client or a user sends over the API to prove who it is. Sode shows it once when it is created and keeps only a hash. Each key has a key name, such as "Panel 21", and History shows the actor with it, like "Seth (Panel 21)". A user's key dies with their PIN. A session token is a different thing: it belongs to a user's sign-in.

**Rights**:
What a user, API client or Guest may command: allowed sources, destinations, levels and presets, where an unset list means all. A take with any route outside the rights is refused whole before anything is sent. Rights are an advanced setting.

**Visibility**:
Set on each rights entry. Read-only, the default, shows everything and refuses commands outside the rights. Hidden also removes what is outside the rights from what that actor sees over the API. Browsers always see everything.

**Shared settings**:
The settings and saved items that every browser sees, such as the layout and clock defaults for Everyone, Everyone filter sets and Everyone presets. Saving them needs the Advanced Settings PIN or a trusted user. Personal settings, saved to This browser or Me, need neither.

**Advanced settings**:
The shared settings that change the router connection, weaken a guard, or manage users, API clients, the PIN and History. Changing them, including adding, removing and renaming users and API clients, needs the **Advanced Settings PIN**, even for a trusted user.
_Avoid_: Settings PIN, admin PIN

**Trusted user**:
A user with a flag that lets them save shared settings without the Advanced Settings PIN. It never covers advanced settings.

**Lock**:
A cover over the page that stops every key from reaching it until it is dismissed. With nobody signed in, pressing the shown letter dismisses it, which guards against mashing the keyboard and nothing more. With a user signed in, the Lock is held by the server for that session, so a reload or reconnect still shows it, and only that user's PIN resumes it. They can also sign out from it.
_Avoid_: hand lock, soft lock, screen lock

**Auto-lock**:
Raises the Lock after a set number of idle minutes. It never signs anyone out. A user marked No auto-lock is skipped, and the Lock button still works for them.
_Avoid_: idle timeout

**Profile**:
A user's saved personal settings, filter sets and presets, kept on the server so they follow the user to any browser. A profile is not private: trusted users and the Advanced Settings PIN can see and edit every profile.

**Me** / **This browser** / **Everyone**:
The three scopes a setting, filter set or preset can be saved to. This browser wins over Me, and Me wins over Everyone. The Settings section for personal choices is My user.
_Avoid_: This device

**Guest**:
Someone who is not signed in. A guest sees This browser and Everyone values and has no Me scope. Guests have one rights entry, and the same entry covers an API call that sends no key.

### Panels

**Panel**:
A saved grid of tappable cells that patches the router from its own page, made for a wall tablet or a phone. It has an id, a display name and an owner. A panel acts either as a user, with that user's Rights, or as an API client of its own, with Rights no wider than its creator's, or as both: the client when the page has a key, else the user who opens it.

**Hardware panel**:
A physical router panel. Sode does not make these. Changes from one arrive as an External.

**Instance**:
One open page of a panel, named once in its browser (its device name) or by the link's `?instance=`. A panel is online while at least one instance is. A key, a watch and a Lost notice can each be for one instance.
_Avoid_: screen (for an open panel page)

**Cell**:
One square of a panel's grid. It has a cell kind and may be a merged cell.

**Cell kind**:
What a cell does: Source, Destination, Preset, Take, Clear, Current Source, Panel Name or Instance Name. A Source or Destination cell is bound to a router number and shows the Router name.

**Current Source** (cell kind):
A cell that shows, under the caption "Currently", the source now on the viewed destination. It cannot be pressed. It stays useful when that source has no cell in the grid.

**Panel Name** (cell kind):
A cell that shows the panel's name. It cannot be pressed. A caption is a line above the name.

**Assign**:
A trusted user sends one instance to a different panel from the panels page (Assign to). The instance's page switches at once with no prompt on the device. A panel that acts as an API client gives the instance a new key for it; a panel that acts as a user makes the device show the sign-in, where the person at the device types their own PIN. An instance that is online but has no valid key can be assigned too.

**Keyless instance**:
An open panel page with no valid key. It still reports its device name, or else its address, so a trusted user can see it and Assign it.

**Instance Name** (cell kind):
A cell that shows the name of the instance showing the page, its device name. It cannot be pressed. A caption is a line above the name.

**Viewed destination**:
The destination whose current and staged source a panel's Source cells show. Pressing a Destination cell views it.

**Staged**:
Describes a change a panel holds for one destination until Take. Each destination has its own, together they are the panel's plan, and Clear drops them all.

**Merged cell**:
A cell that spans several squares of the grid, such as a large Take cell.

**Page**:
One of up to 16 grids a panel holds, in order, each with an optional name. All pages of a panel are the same size and share one plan. A page's number is its position and changes when pages are reordered. A Page can be disabled, which greys out every cell that leads to it.

**Browser page**:
The web page a person opens, such as a panel's own page or the editor. Not a Page of a panel.

**Go to Page** (cell kind):
A cell that shows another Page of the panel. It is bound to that Page itself, so reordering or renaming the Page does not change its target. It still works when the target Page has no cells.

**Next Page** / **Previous Page** (cell kinds):
Cells that show the Page one after or one before the current one, skipping disabled Pages. Past the last or first Page they go to the other end, unless the panel's wrap setting is off.

**Fill cascade**:
What a fill or paste does when it runs past the last square of a Page: it continues on the next enabled Page, and stops at the last Page without returning to the first.

**Page Name** (cell kind):
A cell that shows the name, the number or both of the Page it sits on. It cannot be pressed.

**Info** (cell kind):
A cell that combines the panel's name, its Page's number, its Page's name and the instance's name, each switched on or off. It cannot be pressed.

**Armed**:
Describes a Take cell once a Source and a Destination are selected. It lights or blinks until pressed, and pressing it is the Confirm.

**Take mode** / **Instant mode**:
The panel setting that decides when a panel's plan is confirmed. In Take mode the armed Take cell must be pressed. In Instant mode pressing the Source is the Confirm. A panel can add a hold or a double press to guard the confirm.

**Clear** (cell kind):
Cancels the plan the panel has pending. It never changes the router.

**Fixed destination**:
A panel setting that holds a list of destinations the panel always patches to, at most 16. A panel with it set has no viewed destination to choose: a Source press stages a route to each one. An empty list turns it off.

**Kiosk**:
A panel setting that hides Sode's branding and settings so the page is only the grid.

**Template**:
A saved layout of a panel: its cell kinds, and optionally its settings and cell contents. By default cells come back unassigned. A template is saved to Me or Everyone.

**Artwork**:
The icon or image on a panel cell. A cell has Icon A (an icon or uploaded image) and Icon B (a number 0 to 16 or a letter A to Z). Artwork a cell sets itself wins over Default artwork.

**Icon library**:
The install-wide set of SVG and PNG icons that trusted users add, which any cell's Icon A can use. An SVG draws in the cell's text colour and a PNG keeps its colours. It is separate from the vendored icons and from a panel's own pictures.

**Default artwork**:
Artwork, with a Hide text choice, that a Source, Destination or Preset supplies to every cell showing it that sets none of its own. One shared set for everyone; setting or clearing one needs a trusted user or the Advanced Settings PIN.

**Push**:
Copies a panel's saved config to other panels. Each target keeps its own name, id, sharing and fixed destinations, and a checkpoint of it is kept first so the Push can be put back.

**Checkpoint set**:
One saved layout per panel in a group, so a single Push gives each panel in the group its own layout.

### Interface

**Patch boxes**:
The Source box, the Destination box and the Level box, where a plan is typed. What they hold is a plan until Confirm.

**Last**:
The chip in a patch box that fills it with that box's value from this browser's most recent take. It is per browser, and the chip is hidden until this browser has made a take.

**Next**:
The chip in a patch box that fills it with the value after Last, either the number plus one or the next router name match, as the browser's Next match behavior says. Pressing it again keeps stepping. It is per browser.

**Keyboard mode**:
The state where keys act on History entries instead of the patch boxes. Esc from empty patch boxes starts it and any click ends it.

**Server link**:
The connection between a browser and the Sode server.

**Router link**:
The connection between the Sode server and the router.

**Lost link**:
The banner a browser shows when the Server link or the Router link is down, naming which one.
