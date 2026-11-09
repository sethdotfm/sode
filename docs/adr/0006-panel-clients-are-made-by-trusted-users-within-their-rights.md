# A Trusted user may create a panel's API client, capped by their own Rights

1.8.0 made API clients an advanced setting, so only the Advanced Settings PIN could make one. A panel that must act on its own, such as a wall tablet that outlives anyone's PIN (ADR 0003), needs a client, and sending every such panel to an administrator would make panels a ticket queue. So a Trusted user may create a panel-owned API client, whose Rights can be no wider than the creator's own. The client exists only through its panel, shows in the administrator's client list, and goes with the panel, keys included. Only an administrator can widen its Rights.

Considered: Trusted users with no cap, which lets a Trusted user mint access beyond their own and so weakens the advanced-setting line. Also admin-only creation, which keeps 1.8.0's rule and makes the panel editor useless to the people who run the walls.

Consequences: a creator's later loss of Rights does not shrink an existing client, so a Rights change for a user should list the panel clients they created. A panel in Session mode needs none of this and uses the user's own Rights.

A panel client holds several keys, one per named instance (at most 20 on a panel), so a link can be made again for one kiosk and one screen can be ended without signing out the others. Each link is made for a required name, which is also the instance's device name. A key's secret is still shown once and stored only as a hash. The Rights cap is the client's, so every instance has the same Rights.

The limit of 12 API clients bounds the administrator's own clients only. Panel clients are bounded by the 500 panels, because a wall of tablets would otherwise fill the 12 and leave the administrator none. The cap is checked when a save sets the client's Rights to something new, not on a save that leaves them as stored, so a panel an administrator widened keeps saving.
