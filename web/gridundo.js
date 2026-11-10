// The panel editor's undo stack for grid edits. Pure, so the rules test directly (test/gridundo.test.mjs);
// paneleditor.js records in setGrid and restores outside it. A step keeps the stored grid from before an edit
// (Destination cells set aside by Fixed destination included) and a short name for the status line.
export const UNDO_MAX = 100;

// past and future hold { what, grid, join? }; future's last entry is the next redo.
export const emptyUndo = () => ({ past: [], future: [] });

// join: edits that share it, one straight after another, make one step (typing in the Cell tab).
export function record(h, { what, before, join }) {
  const top = h.past.at(-1);
  if (join && top?.join === join && !h.future.length) return h;
  return { past: [...h.past, { what, grid: before, join }].slice(-UNDO_MAX), future: [] };
}

// Each answers null when there is nothing to step to, else { undo, grid, what } with grid to restore.
export function undo(h, current) {
  const top = h.past.at(-1);
  if (!top) return null;
  return { undo: { past: h.past.slice(0, -1), future: [...h.future, { what: top.what, grid: current }] }, grid: top.grid, what: top.what };
}

export function redo(h, current) {
  const top = h.future.at(-1);
  if (!top) return null;
  return { undo: { past: [...h.past, { what: top.what, grid: current }], future: h.future.slice(0, -1) }, grid: top.grid, what: top.what };
}

// Keys sorted, so two copies of one cell compare equal however they were built.
const canon = (v) => (v && typeof v === 'object' && !Array.isArray(v) ? Object.fromEntries(Object.keys(v).sort().map((k) => [k, canon(v[k])])) : v);
const cellsKey = (g) =>
  JSON.stringify(
    g.cells
      .map(canon)
      .sort((a, b) => a.r - b.r || a.c - b.c)
      .map((c) => JSON.stringify(c)),
  );

// True when an edit changed nothing that is stored: a selection move is not an edit.
export const sameCells = (a, b) => a.rows === b.rows && a.cols === b.cols && (a.cells === b.cells || cellsKey(a) === cellsKey(b));
