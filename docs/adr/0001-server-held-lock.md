---
status: accepted
---

# A signed-in user's Lock is held by the server

When a user is signed in, the Lock is state of their session on the server. A reload or reconnect shows the Lock again, the server accepts nothing for the router from that session until the same user's PIN resumes it, and the user can Sign out from it. With nobody signed in the Lock stays a page cover that the shown letter dismisses, and the server knows nothing about it.

## What the code does today

Ctrl+L, the chip button, Settings and the idle timer all end the signed-in user's session. `lockNow()` in `web/lock.js` sends `userLock`, `App.UserLock` in `internal/app/users.go` revokes the session token and clears the connection's user, and the PIN pad that comes next is a Sign in. A reload shows the page with nobody signed in. The docs call this the signed-in lock (#198).

This contradicts the glossary, where the Lock never signs anyone out and Sign out is a separate act. Auto-lock is also client-side and skips persistent users (`idleLock` in `web/lockrules.js`), so a station that stays signed in never covers itself. This ADR replaces that behavior.

## Considered options

- **Page-only cover for everyone.** A reload clears it, and a reconnect with a persistent token signs the user straight back in. It stops a mashed keyboard and nothing else, so a signed-in station could be opened by closing the tab. Rejected for signed-in users, kept for guests, where the glossary says it guards only against mashing.
- **Sign out on Lock (today).** The PIN is enforced, but the user loses the session, and every other tab on that session, each time they step away. It also makes Auto-lock impossible for persistent users without signing them out, which the glossary forbids. Rejected.
- **Server-held Lock (chosen).** The locked flag lives on the session beside the user and the token, so it survives reload and reconnect. Nothing about the session is lost, and every other connection sharing the session is locked with it.

## Trade-offs

The server stores one more bit per session. The policy table must refuse every `needTake`, `needMain` and `needLog` command from a locked session, as it does for a page lock. Locks still cover actions and log reads, not data, so hello, names, tally and History keep streaming to a locked browser.

A locked session that nobody resumes stays signed in until its token expires or an admin uses `userSignOutAll`. Signing out from the Lock ends it on purpose.

## Open questions, answered

**Does a rename end a user's persistent sessions? Yes.** `tokenStore.user` looks the user up by name on every use, so a rename already kills the token. A rename is rare, deliberate and needs the Advanced Settings PIN. Keying tokens by a stable id would add a field and a migration to save the user one Sign in per browser. The Profile, the PIN and the names already in History survive. The rename dialog should say that the user will be signed out everywhere. This does not change the Lock ticket.

**Is the signed-in Lock's PIN pad checked on every keypress? No, only on resume.** The pad keeps its current rule. It submits when the PIN reaches the longest length in use, or on the check key or Enter for a shorter one. Checking each keypress would count every prefix as a wrong try in the user PIN pool and would reveal PIN lengths. The server knows whose session it is, so it compares the PIN with that user's hash only. A PIN that belongs to another user is a wrong try. It does not resume the Lock or switch users. That already matches the Lock ticket's criteria, so its scope does not change.

## Consequences for later work

Auto-lock raises this server-held Lock for every signed-in user, persistent ones included, and signs no one out.
