# Sode

Sode is a keyboard-first web page for patching an SW-P-08 router, built and tested on a Ross Ultrix. It exists to make patching about 1000 times faster and simpler for the people doing it. Today it drives one router. Multiple routers and tie lines are plausible later, after the core product is excellent.

## Language

### Routing

**Source**:
A router input, shown by its number and the name the router gives it.
_Avoid_: input

**Destination**:
A router output, shown by its number and the name the router gives it.
_Avoid_: output

**Level**:
One of the parallel signal layers (such as video or audio) that a source can be connected to a destination on. Levels are numbered 1..N in Sode whatever the router uses on the wire.
_Avoid_: wire level, layer

**Router name**:
The name the router gives a source or destination. Sode cannot change it, and re-reads names from the router on a timer. Keep it apart from a user's name, a preset's name and a filter set's name.
_Avoid_: label, refresh (for the re-read)

**Protected**:
Describes a destination that the router will not change because something has protected it. Sode refuses a plan that includes a protected destination before sending anything.
_Avoid_: protect (as a noun), locked

### Actions

**Command**:
Anything a person fires that Sode acts on, such as a take, an undo, a retry, a router name re-read, clearing History or saving settings. Every command is logged.

**Confirm**:
The deliberate step that turns a plan into a take, done with the churning letter key and its short lockout. No take happens without one.

**Take**:
A kind of command that executes a change on the router hardware. A take sends one source to one destination with levels, or sends several such routes at once. One confirm is one take. "Take" is a reserved word, so nothing that has not been sent to the router is a take.
_Avoid_: batch, route request, using "take" for any other command, a plan or saved entry, or as an ordinary verb meaning "accept" or "interpret literally"

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
The shared, chronological record of takes and external changes that every browser sees.
_Avoid_: Recent

**Group**:
The entry in History for a take that reached several destinations. Undo all and Redo all act on a Group, undoing or repeating every destination in that take.
_Avoid_: batch, folder

**Entry**:
One row in History: a take, or an external change.

**Filter**:
One rule on Source or Destination that limits which entries History shows.

**Filter set**:
A named, savable group of filters, scoped to Me, This browser or Everyone. Several sets can be stacked, and Stacked filters behavior says whether an entry must match all of them or any one.

**Communication Log**:
The page that shows every command and the raw messages Sode sent to the router, plus a router uptime graph.
_Avoid_: sent log

### Changes Sode did not make

**External change**:
A change on the router that Sode did not make, such as one from a hardware panel, Companion or the router itself. Sode learns of it from the router and records it in History next to its own takes.
_Avoid_: panel change, outside change

**Found**:
A kind of external change the router made without announcing it, noticed by Sode only later.

**External follow**:
An external change on a destination that probably followed another destination's change, such as a multiviewer PiP following a DSM. Sode cannot tell a follow from other automation, so this is always a suspicion.
_Avoid_: follows (as a certain fact), automation

### People and access

**User**:
A named person who can sign in to Sode. Their name is what History shows in place of an IP address. Changing a name is an advanced setting. History entries keep the name from when they were made, and the user's Profile follows the rename.
_Avoid_: operator, account

**Sign in** / **Sign out**:
Sign in is how a user identifies themself. Sign out ends that session. Whether a page requires sign-in is set per page type.
_Avoid_: log in, log out, unlock (for a user's session)

**Sign-in mode**:
How a user signs in. PIN only is the default, where the PIN alone says who is signing in, so every PIN is unique. Username and PIN is an optional mode for deployments that want it, where the username is separate from the name History shows.

**Shared settings**:
The settings and saved items that every browser sees, such as the layout and clock defaults for Everyone, Everyone filter sets and Everyone presets. Saving them needs the Advanced Settings PIN or a trusted user. Personal settings, saved to This browser or Me, need neither.

**Advanced settings**:
The shared settings that change the router connection, weaken a guard, or manage users, the PIN and History. Changing them, including adding, removing and renaming users, needs the **Advanced Settings PIN**, even for a trusted user.
_Avoid_: Settings PIN, admin PIN

**Trusted user**:
A user with a flag that lets them save shared settings without the Advanced Settings PIN. It never covers advanced settings.

**Lock**:
A cover over the page that stops every key from reaching it until it is dismissed. With nobody signed in, pressing the shown letter dismisses it, which guards against mashing the keyboard and nothing more. With a user signed in, the Lock is held by the server for that session, so a reload or reconnect still shows it, and only that user's PIN resumes it. They can also sign out from it.
_Avoid_: hand lock, soft lock, screen lock

**Auto-lock**:
Raises the Lock after a set number of idle minutes. It never signs anyone out.
_Avoid_: idle timeout

**Profile**:
A user's saved personal settings, filter sets and presets, kept on the server so they follow the user to any browser. A profile is not private: trusted users and the Advanced Settings PIN can see and edit every profile.

**Me** / **This browser** / **Everyone**:
The three scopes a setting, filter set or preset can be saved to. This browser wins over Me, and Me wins over Everyone. The Settings section for personal choices uses the same name.
_Avoid_: This device

**Guest**:
Someone who is not signed in. A guest sees This browser and Everyone values and has no Me scope.

### Interface

**Patch boxes**:
The Source box, the Destination box and the Level box, where a plan is typed. What they hold is a plan until Confirm.

**Last**:
The chip in a patch box that fills it with that box's value from this browser's most recent take. It is per browser.

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
