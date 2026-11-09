# PROTOTYPE: Sode Panels UI (throwaway, wipe me)

Three structurally different panel views over one stub router and panel engine. No server, no Sode
connection, no persistence. Run: `node prototype/panels/serve.mjs` then open
8101 (A), 8102 (B), 8103 (C). Each view has a prototype toolbar for mode and multi-destination,
which stands in for the settings page that comes later.

## Editor prototype (settings and grid editing)

Three editor layouts over one editor state (`shared/ed.js`) and shared views (`shared/edview.js`).
Run `node prototype/panels/serve-editor.mjs`, then open 8111 (A inspector), 8112 (B paint), 8113 (C navigator).
"Test it live" swaps the edit grid for the working stub panel with the panel's settings applied.
"Viewer is Trusted" shows how API client panels are gated. Save only clears the Unsaved badge.

Variants D (8114), E (8115) and F (8116, smart menu) is the smart cursor: a floating chooser on the selected square, a grip that extends a run, and a gesture pad
for multi-square selections. Templates (toolbar, A and D) are saved layouts whose cells come back unassigned.
