# A panel is an ordered list of Pages, each with a stable ID

A panel was one grid, stored as `cells`. A show with many Sources needs more squares than one grid holds, so a panel now holds up to 16 Pages that share its size and its one plan. A Page's number is its position in the list, so pages can be reordered and renumbered. Each Page also carries a random short ID that never changes, and a Go to Page cell stores that ID, not a number. A reorder or a rename therefore never moves a cell's target. A panel file stores `pages: [{id, name?, enabled, cells}]`. A file from before Pages loads its `cells` as the single first Page, and the next save writes only `pages`.

Considered: sixteen fixed slots whose number is the Page's identity, which needs no IDs but makes reordering a copy and clear, and makes every Go to Page cell wrong after one. Also a `page` field on each cell with `cells` kept flat, which leaves a Page with no home for its name or enabled state.

Consequences: an older Sode build cannot read a panel saved after this change, and Sode does not mirror `cells` to hide that. A Go to Page cell has no meaning outside its panel, so a paste into another panel brings it in unassigned. Push, templates, Checkpoint sets and restores copy every Page with its ID, which keeps Go to Page cells working inside the copied layout.
