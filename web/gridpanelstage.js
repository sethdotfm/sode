// Staging on a Grid panel. Pure: no DOM, no clock, no network. Every function returns a new state.
// A staged entry is { dst, level, src }. Staging is keyed by destination and level, so a breakaway goes in one take
// (video from one source, audio from another). A route (GLOSSARY.md) is one destination and one source with its levels.
// Instant mode holds nothing staged: a press, a Diagonal or a Drawn line sends at once, with no confirm.

// The server takes 200 routes in one take; the Grid stops well under it.
export const STAGE_MAX = 128;

// undoId is set while the stage is this instance's Undo (gridpanelundo.js). The stored mode takeOnly is Take mode
// whose Take sends with no confirm box; ask is false only then. Any other value is today's Take mode, with the box.
export const createStage = ({ mode } = {}) => ({
  mode: mode === 'instant' ? 'instant' : 'take',
  ask: mode !== 'takeOnly',
  staged: [],
  busy: false,
  error: null,
  undoId: null,
});

// The staged entries as routes for POST /takes, one per destination and source, in the order first staged.
export function routesOf(st) {
  const by = new Map();
  for (const x of st.staged) {
    const k = `${x.dst}:${x.src}`;
    if (!by.has(k)) by.set(k, { src: x.src, dst: x.dst, levels: [] });
    by.get(k).levels.push(x.level);
  }
  return [...by.values()].map((r) => ({ ...r, levels: r.levels.sort((a, b) => a - b) }));
}

export const stagedCount = (st) => routesOf(st).length;

const fail = (st, code, message = '') => ({ ...st, error: { code, message } });

// A press on a crosspoint with the levels to stage. send is the routes to send at once (Instant mode), else null.
export const pressCrosspoint = (st, xp, levels) => stageRoutes(st, [xp], levels);

// One press: pairs is [{ src, dst }], a single crosspoint or a Diagonal (gridpanelpick.js), no destination twice.
// Returns { st, send, confirm }. confirm asks for the take confirm at once. In Take mode, a press that adds nothing to
// the stage is the second press. Examples are the same crosspoint, Diagonal or line again, on the same or fewer levels.
// It keeps the stage and asks for the take confirm. Instant mode sends every press at once, however many routes.
export function stageRoutes(st, pairs, levels) {
  const none = { send: null, confirm: false };
  if (!levels.length) return { st: fail(st, 'no-levels'), ...none };
  const base = st.mode === 'instant' ? [] : st.staged;
  const dsts = new Set(pairs.map((x) => x.dst));
  const kept = base.filter((x) => !(dsts.has(x.dst) && levels.includes(x.level)));
  const staged = [...kept, ...pairs.flatMap(({ src, dst }) => levels.map((level) => ({ dst, level, src })))];
  if (base.length && sameStage(base, staged)) return { st: { ...st, error: null }, send: null, confirm: true };
  const next = { ...st, staged, error: null };
  if (stagedCount(next) > STAGE_MAX) return { st: { ...st, error: { code: 'cap', add: pairs.length, have: stagedCount({ staged: base }) } }, ...none };
  if (st.mode === 'take') return { st: next, ...none };
  return { st: { ...st, staged: [], error: null }, send: routesOf(next), confirm: false };
}

const keyOf = (x) => `${x.dst}:${x.level}:${x.src}`;
// Staging is keyed by destination and level, so neither list holds a key twice.
const sameStage = (a, b) => a.length === b.length && new Set([...a, ...b].map(keyOf)).size === a.length;

// Alt-click on a staged crosspoint: that route goes on every level, the rest stay. The same state when it is not staged.
export function unstageRoute(st, { src, dst }) {
  const staged = st.staged.filter((x) => !(x.src === src && x.dst === dst));
  return staged.length === st.staged.length ? st : { ...st, staged, error: null };
}

// What the take confirm sends, or null when nothing is staged. In Instant mode only an Undo waiting on its confirm is.
export const confirmRoutes = (st) => (st.staged.length ? routesOf(st) : null);

// The confirm closed without a take. Instant mode holds nothing staged.
export const cancelConfirm = (st) => (st.mode === 'instant' ? dropStaged(st) : st);

export const dropStaged = (st) => ({ ...st, staged: [], error: null, undoId: null });

// A newer saved copy of the panel: staged routes belong to the old one and go (panelupdate.js), as on a Buttons panel.
// A take in flight holds the update back, but busy is carried so its answer still lands.
export const restage = (st, panel) => ({ ...createStage({ mode: panel.settings?.mode }), busy: st.busy });

export function stageText(err) {
  switch (err?.code) {
    case 'cap':
      return `At most ${STAGE_MAX} routes can be staged${err.add > 1 ? `. This press adds ${err.add} routes to ${err.have}.` : ''}`;
    case 'no-levels':
      return "No levels to stage: this panel's levels are not on the router or outside its Rights";
    case 'take':
      return err.message;
    default:
      return '';
  }
}

export const startSend = (st) => ({ ...st, busy: true, error: null });

// failed is null for a take Sode answered, else the reason. A failed take keeps the stage so it can be sent again,
// except in Instant mode, which holds nothing staged.
export function afterTake(st, failed) {
  if (!failed) return { ...st, staged: [], busy: false, error: null };
  return { ...fail(st, 'take', failed), staged: st.mode === 'instant' ? [] : st.staged, busy: false };
}
