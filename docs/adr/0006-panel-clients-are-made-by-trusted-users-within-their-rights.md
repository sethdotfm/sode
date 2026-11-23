# A Trusted user may create a panel's API client, capped by their own Rights

1.8.0 made API clients an advanced setting, so only the shared PIN for advanced settings could make one. A panel that must act on its own, such as a wall tablet that outlives anyone's PIN (ADR 0003), needs a client, and sending every such panel to an administrator would make panels a ticket queue. So a Trusted user may create a panel-owned API client, whose Rights can be no wider than the creator's own. The client exists only through its panel, shows in the administrator's client list, and goes with the panel, keys included. Only an administrator can widen its Rights.

Considered: Trusted users with no cap, which lets a Trusted user mint access beyond their own and so weakens the advanced-setting line. Also admin-only creation, which keeps 1.8.0's rule and makes the panel editor useless to the people who run the walls.

Consequences: a creator's later loss of Rights does not shrink an existing client, so a Rights change for a user should list the panel clients they created. A panel in Session mode needs none of this and uses the user's own Rights.

A panel client holds several keys, one per named instance (at most 20 on a panel), so a link can be made again for one kiosk and one instance can be ended without signing out the others. Each link is made for a required name, which is also the instance's device name. A key's secret is still shown once and stored only as a hash. The Rights cap is the client's, so every instance has the same Rights.

A panel set to Both holds the same client under the same rule, and its page runs in Session mode when it has no key. The choice of mode never widens access: with a key it is the client's Rights, without one the signed-in user's.

The limit of 12 API clients bounds the administrator's own clients only. Panel clients are bounded by the 500 panels, because a wall of tablets would otherwise fill the 12 and leave the administrator none. The cap is checked when a save sets the client's Rights to something new, not on a save that leaves them as stored, so a panel an administrator widened keeps saving.

## v2.0: any panel owner, and API clients from My user

Roles (ADR 0010) widen this decision. Any user who can edit a panel may create its API client, whatever their Role, still capped by their own Rights. Rights wider than the creator's stay an Administrator's change: over the API the answer is 403 `administrator_only`, which says to make the change in Settings, since a key never acts as an Administrator.

A user may also make API clients that belong to no panel, from My user, within their own Rights (owner ruling on the v2.0 spec). The user owns such a client. It saves at once, and it is deleted with its user. Unlike a panel client, its Rights are capped live: each call uses the client's Rights and the owner's current Rights together, so narrowing the user narrows the client at once. Each user may have 12 such clients, apart from the administrator's 12. A user without a PIN holds no lasting keys, so cannot make one. Settings > Access > API stays Administrator only.
