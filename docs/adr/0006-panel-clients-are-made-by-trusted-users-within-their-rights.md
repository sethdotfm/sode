# A Trusted user may create a panel's API client, capped by their own Rights

1.8.0 made API clients an advanced setting, so only the Advanced Settings PIN could make one. A panel that must act on its own, such as a wall tablet that outlives anyone's PIN (ADR 0003), needs a client, and sending every such panel to an administrator would make panels a ticket queue. So a Trusted user may create a panel-owned API client, whose Rights can be no wider than the creator's own. The client exists only through its panel, shows in the administrator's client list, and goes with the panel, keys included. Only an administrator can widen its Rights.

Considered: Trusted users with no cap, which lets a Trusted user mint access beyond their own and so weakens the advanced-setting line. Also admin-only creation, which keeps 1.8.0's rule and makes the panel editor useless to the people who run the walls.

Consequences: a creator's later loss of Rights does not shrink an existing client, so a Rights change for a user should list the panel clients they created. A panel in Session mode needs none of this and uses the user's own Rights.
