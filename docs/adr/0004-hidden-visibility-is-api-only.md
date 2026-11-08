# Hidden visibility applies to API callers only

A rights entry with hidden visibility removes out-of-rights sources and destinations from what the actor sees, but only over the API (`/api/v1`, keys and the stream). Browsers always see everything. The browser connection broadcasts names, tally and `recent` entries to every connection, locked or guest, and Prev, Now, Already and Failed all carry source numbers, so per-actor filtering there would mean filtering every broadcast, History page and Group. A hidden user who signs in on a browser that already holds everything would gain nothing anyway.

Consequences: a rights entry can be hidden for a panel key and read-only for the same person's browser. The browser still refuses commands outside the rights. Making hidden cover browsers is a later project and needs a per-connection view of server state first.
