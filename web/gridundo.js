// The panel editor's undo stack for cell edits, one for the whole panel. Pure, so the rules test directly
// (test/gridundo.test.mjs, test/editorpagerules.test.mjs); paneleditor.js records in its edits and restores outside
// them. A step keeps a pageSnap from before an edit (Destination cells set aside by Fixed destination included) and a
// short name for the status line.
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

// Every Page's cells by Page id, so an undo never brings back a Page name, order, enable or deletion (those are not
// undo steps). list: a template apply, which replaced the Page list itself, so its undo puts the whole list back.
// extra rides along for the editor (focus and selection).
export const pageSnap = (pages, rows, cols, { list = false, ...extra } = {}) => ({
  ...extra,
  rows,
  cols,
  cells: Object.fromEntries(pages.map((p) => [p.id, p.cells || []])),
  ...(list ? { list: pages } : {}),
});

export function samePages(a, b) {
  if (a.rows !== b.rows || a.cols !== b.cols) return false;
  const ka = Object.keys(a.cells);
  if (ka.length !== Object.keys(b.cells).length) return false;
  return ka.every((id) => id in b.cells && (a.cells[id] === b.cells[id] || cellsKey({ cells: a.cells[id] }) === cellsKey({ cells: b.cells[id] })));
}

// pages with a step's cells put back by id. A Page added since keeps its cells and one deleted since stays deleted.
export const restorePages = (pages, snap) => snap.list || pages.map((p) => (p.id in snap.cells ? { ...p, cells: snap.cells[p.id] } : p));

// An unsaved Page 1 has id '' until the first save gives it one.
export function rekeyUndo(h, from, to) {
  const fix = (s) => {
    if (!(from in s.grid.cells)) return s;
    const { [from]: cells, ...rest } = s.grid.cells;
    const list = s.grid.list?.map((p) => (p.id === from ? { ...p, id: to } : p));
    return { ...s, grid: { ...s.grid, cells: { ...rest, [to]: cells }, ...(list ? { list } : {}), ...(s.grid.focus === from ? { focus: to } : {}) } };
  };
  return { past: h.past.map(fix), future: h.future.map(fix) };
}

// before and after are the Page ids in list order around a save; any Page that had id '' follows its new one.
export const rekeySaved = (h, before, after) => before.reduce((acc, id, i) => (id === '' && after[i] && after[i] !== id ? rekeyUndo(acc, id, after[i]) : acc), h);
