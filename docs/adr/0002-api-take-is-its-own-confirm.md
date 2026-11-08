# An API take request is its own Confirm

A browser Confirm is a churning letter key and a short lockout, which an API client cannot press. We loosened the glossary so that over the API the take request is the Confirm, and the API client is responsible for guarding it (the panels app can add a hold or a double press). The alternative, an API that only prepares plans for a person to confirm in a browser, would defeat a button panel.

Consequences: nothing on the server can tell a deliberate API take from a script bug, so the rights check, per-key rate limit and `api` rows in the Communication Log carry the safety that the lockout carries in a browser. Undo, Redo and Retry over the API are plans that the same request confirms.
