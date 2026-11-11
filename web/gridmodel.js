// The panel editor's grid: cells, selection, merge, fill and numbering. No DOM, no clock, no network, so the
// rules test directly (test/gridmodel.test.mjs). Every function returns a new grid; one it cannot apply
// returns the grid it was given.
//
// A grid is { rows, cols, cells, sel }. cells hold the stored shape (panelrules.js: { r, c, rs, cs, kind, n?,
// caption?, levels? } with r and c from 1) and always have whole spans inside the grid. A source or destination
// cell with no n is unassigned: stored, shown dashed, and left out of a live panel by cellsIn. sel is
// { anchor, head } squares, or null. Rectangles are { r1, c1, r2, c2 }.
import { KINDS, cellsIn } from './panelrules.js';

export const ROWS_MAX = 24; // keep in step with panelRowsMax and panelColsMax in internal/app/panelstore.go
export const COLS_MAX = 24;
export const DEFAULT_ROWS = 4; // maps one to one onto a hardware key deck
export const DEFAULT_COLS = 8;

export const FILL_ORDERS = [
  { id: 'across', name: 'Across, then down' },
  { id: 'across-rev', name: 'Right to left, then down' },
  { id: 'snake-across', name: 'Snake across' },
  { id: 'down', name: 'Down, then across' },
  { id: 'down-rev', name: 'Bottom to top, then across' },
  { id: 'snake-down', name: 'Snake down' },
];

const clamp = (v, lo, hi, fallback) => (Number.isFinite(Number(v)) && v !== '' && v !== null ? Math.max(lo, Math.min(hi, Math.trunc(Number(v)))) : fallback);
const span = (c) => ({ r2: c.r + c.rs - 1, c2: c.c + c.cs - 1 });
const touches = (c, q) => {
  const { r2, c2 } = span(c);
  return !(r2 < q.r1 || c.r > q.r2 || c2 < q.c1 || c.c > q.c2);
};
const reading = (a, b) => a.r - b.r || a.c - b.c;
const withCells = (g, cells) => ({ ...g, cells });
const range = (a, z) => Array.from({ length: z - a + 1 }, (_, i) => a + i);

// A cell the panel page would leave out only because it has no number is still valid here.
function normalCells(rows, cols, cells) {
  const probe = (cells || []).map((c, i) => ({
    ...c,
    rs: c?.rs || 1,
    cs: c?.cs || 1,
    n: Number.isInteger(c?.n) ? c.n : 0,
    ...(c?.kind === 'preset' && !c.name ? { scope: 'me', name: '-' } : {}),
    ix: i,
  }));
  return cellsIn({ rows, cols, cells: probe }).map((p) => {
    const orig = cells[p.ix];
    return { ...orig, rs: p.rs, cs: p.cs };
  });
}

export function createGrid({ rows = DEFAULT_ROWS, cols = DEFAULT_COLS, cells = [] } = {}) {
  const r = clamp(rows, 1, ROWS_MAX, DEFAULT_ROWS);
  const c = clamp(cols, 1, COLS_MAX, DEFAULT_COLS);
  return { rows: r, cols: c, cells: normalCells(r, c, cells), sel: null };
}

export const cellAt = (g, r, c) => g.cells.find((x) => r >= x.r && r <= span(x).r2 && c >= x.c && c <= span(x).c2);

// Cells that start outside the new size. A span that only reaches outside is clipped, not lost.
export function cellsLost(g, rows, cols) {
  const r = clamp(rows, 1, ROWS_MAX, g.rows);
  const c = clamp(cols, 1, COLS_MAX, g.cols);
  return g.cells.filter((x) => x.r > r || x.c > c).length;
}

export function resize(g, rows, cols) {
  const r = clamp(rows, 1, ROWS_MAX, g.rows);
  const c = clamp(cols, 1, COLS_MAX, g.cols);
  if (r === g.rows && c === g.cols) return g;
  const cells = g.cells.filter((x) => x.r <= r && x.c <= c).map((x) => ({ ...x, rs: Math.min(x.rs, r - x.r + 1), cs: Math.min(x.cs, c - x.c + 1) }));
  const out = (p) => p.r > r || p.c > c;
  const sel = g.sel && (out(g.sel.anchor) || out(g.sel.head)) ? null : g.sel;
  return { ...g, rows: r, cols: c, cells, sel };
}

const inGrid = (g, r, c) => Number.isInteger(r) && Number.isInteger(c) && r >= 1 && c >= 1 && r <= g.rows && c <= g.cols;

// A plain click selects one square. With extend the head moves and the anchor stays (drag, Shift-click).
export function select(g, r, c, extend = false) {
  if (!inGrid(g, r, c)) return g;
  const sel = extend && g.sel ? { anchor: g.sel.anchor, head: { r, c } } : { anchor: { r, c }, head: { r, c } };
  return { ...g, sel };
}

export const deselect = (g) => (g.sel ? { ...g, sel: null } : g);

export const selectAll = (g) => selectRect(g, { r1: 1, c1: 1, r2: g.rows, c2: g.cols });

// The selection as a rectangle grown until it holds every merged cell it touches whole.
export function rectOf(g) {
  if (!g.sel) return null;
  const { anchor: a, head: h } = g.sel;
  const q = { r1: Math.min(a.r, h.r), c1: Math.min(a.c, h.c), r2: Math.max(a.r, h.r), c2: Math.max(a.c, h.c) };
  for (let grew = true; grew; ) {
    grew = false;
    for (const x of g.cells) {
      if (!touches(x, q)) continue;
      const { r2, c2 } = span(x);
      if (x.r < q.r1 || x.c < q.c1 || r2 > q.r2 || c2 > q.c2) {
        q.r1 = Math.min(q.r1, x.r);
        q.c1 = Math.min(q.c1, x.c);
        q.r2 = Math.max(q.r2, r2);
        q.c2 = Math.max(q.c2, c2);
        grew = true;
      }
    }
  }
  return q;
}

export const selCells = (g) => {
  const q = rectOf(g);
  return q ? g.cells.filter((x) => touches(x, q)).sort(reading) : [];
};
export const selSquares = (g) => {
  const q = rectOf(g);
  return q ? (q.r2 - q.r1 + 1) * (q.c2 - q.c1 + 1) : 0;
};

// The one cell the selection is exactly, or null. Its settings belong in the sidebar's Cell tab.
export function single(g) {
  const cells = selCells(g);
  return cells.length === 1 && selSquares(g) === cells[0].rs * cells[0].cs ? cells[0] : null;
}

const selectRect = (g, q) => ({ ...g, sel: { anchor: { r: q.r1, c: q.c1 }, head: { r: q.r2, c: q.c2 } } });

// Fields that describe the router item and so do not survive a change of kind.
const ITEM_FIELDS = ['n', 'scope', 'name', 'page', 'show'];
const dropItem = (c) => {
  const out = { ...c };
  for (const f of ITEM_FIELDS) delete out[f];
  return out;
};

// kind '' is Empty: the cells in the selection go. Other kinds fill empty squares with a 1x1 cell and
// change the cells already there, which keep their caption, levels and span.
export function setKind(g, kind) {
  const q = rectOf(g);
  if (!q) return g;
  const hit = selCells(g);
  if (kind === '')
    return hit.length
      ? withCells(
          g,
          g.cells.filter((x) => !hit.includes(x)),
        )
      : g;
  if (!KINDS.includes(kind)) return g;
  const cells = g.cells.map((x) => (hit.includes(x) && x.kind !== kind ? { ...dropItem(x), kind } : x));
  for (let r = q.r1; r <= q.r2; r++) for (let c = q.c1; c <= q.c2; c++) if (!cellAt(g, r, c)) cells.push({ r, c, rs: 1, cs: 1, kind });
  return withCells(g, cells);
}

export const clearSelection = (g) => setKind(g, '');

// Change one cell, found by its top left square. A field set to undefined is removed.
export function setCell(g, at, patch) {
  return withCells(
    g,
    g.cells.map((x) => {
      if (x.r !== at.r || x.c !== at.c) return x;
      const out = { ...x, ...patch };
      for (const k of Object.keys(patch)) if (out[k] === undefined) delete out[k];
      return out;
    }),
  );
}

export const canMerge = (g) => selSquares(g) >= 2;

// One cell over the selection. The first cell in reading order keeps its settings; a selection with no
// cell becomes a Take cell, the large button merging is mostly for.
export function merge(g) {
  if (!canMerge(g)) return g;
  const q = rectOf(g);
  const hit = selCells(g);
  const keep = hit[0] || { kind: 'take' };
  const whole = { ...keep, r: q.r1, c: q.c1, rs: q.r2 - q.r1 + 1, cs: q.c2 - q.c1 + 1 };
  return selectRect(withCells(g, [...g.cells.filter((x) => !hit.includes(x)), whole]), q);
}

export const canUnmerge = (g) => selCells(g).some((x) => x.rs * x.cs > 1);

// The top left square keeps the cell and the others become empty. The selection stays on all of them.
export function unmerge(g) {
  if (!canUnmerge(g)) return g;
  const q = rectOf(g);
  return selectRect(
    withCells(
      g,
      g.cells.map((x) => (x.rs * x.cs > 1 && touches(x, q) ? { ...x, rs: 1, cs: 1 } : x)),
    ),
    q,
  );
}

// The squares of a rectangle in the order a fill walks them.
export function squareOrder(q, order) {
  const rows = range(q.r1, q.r2);
  const cols = range(q.c1, q.c2);
  const rev = (a) => [...a].reverse();
  const out = [];
  switch (order) {
    case 'across-rev':
      rows.forEach((r) => rev(cols).forEach((c) => out.push([r, c])));
      break;
    case 'snake-across':
      rows.forEach((r, i) => (i % 2 ? rev(cols) : cols).forEach((c) => out.push([r, c])));
      break;
    case 'down':
      cols.forEach((c) => rows.forEach((r) => out.push([r, c])));
      break;
    case 'down-rev':
      cols.forEach((c) => rev(rows).forEach((r) => out.push([r, c])));
      break;
    case 'snake-down':
      cols.forEach((c, i) => (i % 2 ? rev(rows) : rows).forEach((r) => out.push([r, c])));
      break;
    default:
      rows.forEach((r) => cols.forEach((c) => out.push([r, c])));
  }
  return out;
}

// What a fill would do, for a preview and for applyFill. items are router numbers placed in order, one
// per square. A square inside a merged cell (not its top left) is skipped; keep also skips squares with a cell.
export function fillPlan(g, { kind, items, order = 'across', keep = false }) {
  const q = rectOf(g);
  if (!q) return null;
  const queue = [...items];
  const slots = [];
  for (const [r, c] of squareOrder(q, order)) {
    const hit = cellAt(g, r, c);
    if (hit && (hit.r !== r || hit.c !== c || keep)) continue;
    if (!queue.length) break;
    slots.push({ r, c, n: queue.shift(), replace: !!hit });
  }
  return { kind, slots, dropped: queue.length };
}

function place(g, kind, slots) {
  const cells = g.cells.map((x) => {
    const s = slots.find((y) => y.r === x.r && y.c === x.c);
    return s ? { ...dropItem(x), kind, n: s.n } : x;
  });
  for (const s of slots) if (!cellAt(g, s.r, s.c)) cells.push({ r: s.r, c: s.c, rs: 1, cs: 1, kind, n: s.n });
  return withCells(g, cells);
}

export const applyFill = (g, plan) => (plan && plan.slots.length ? place(g, plan.kind, plan.slots) : g);

// Dragging a selection's corner grip: toRect grows the selection on one side and each line of cells running
// that way continues by its own step (the gap between its last two numbers, else 1), in its last cell's kind.
export function extendPlan(g, toRect) {
  const q = rectOf(g);
  if (!q) return null;
  const to = { r1: Math.max(1, toRect.r1), c1: Math.max(1, toRect.c1), r2: Math.min(g.rows, toRect.r2), c2: Math.min(g.cols, toRect.c2) };
  const grows = [to.r1 < q.r1, to.r2 > q.r2, to.c1 < q.c1, to.c2 > q.c2];
  const same = [to.r1 === q.r1, to.r2 === q.r2, to.c1 === q.c1, to.c2 === q.c2];
  const side = grows.findIndex(Boolean);
  if (side < 0 || grows.filter(Boolean).length !== 1 || same.filter(Boolean).length !== 3) return null;
  // side 0 up, 1 down, 2 left, 3 right. dr and dc point away from the selection.
  const [dr, dc] = [
    [-1, 0],
    [1, 0],
    [0, -1],
    [0, 1],
  ][side];
  const horizontal = dr === 0;
  const lines = range(horizontal ? q.r1 : q.c1, horizontal ? q.r2 : q.c2);
  const slots = [];
  for (const line of lines) {
    const sq = (k) => (horizontal ? [line, k] : [k, line]);
    const edge = horizontal ? (dc > 0 ? q.c2 : q.c1) : dr > 0 ? q.r2 : q.r1;
    const inner = edge - (horizontal ? dc : dr);
    const e = cellAt(g, ...sq(edge));
    if (!e || !Number.isInteger(e.n)) continue;
    const i = edge === inner ? null : cellAt(g, ...sq(inner));
    const step = i && i !== e && i.kind === e.kind && Number.isInteger(i.n) && i.n !== e.n ? e.n - i.n : 1;
    const count = horizontal ? Math.abs((dc > 0 ? to.c2 : to.c1) - edge) : Math.abs((dr > 0 ? to.r2 : to.r1) - edge);
    for (let k = 1; k <= count; k++) {
      const n = e.n + step * k;
      if (n < 0) break;
      const [r, c] = horizontal ? sq(edge + dc * k) : sq(edge + dr * k);
      if (cellAt(g, r, c)) continue;
      slots.push({ r, c, n, kind: e.kind });
    }
  }
  return { slots, to, first: slots[0]?.n ?? -1, last: slots.at(-1)?.n ?? -1, count: slots.length };
}

// The rectangle a grip drag asks extendPlan for: the pointer's square decides which one side grows, the side it
// is furthest past (a tie goes across). Null while the pointer is inside the selection.
export function dragRect(g, r, c) {
  const q = rectOf(g);
  if (!q) return null;
  const down = r > q.r2 ? r - q.r2 : 0;
  const up = r < q.r1 ? q.r1 - r : 0;
  const right = c > q.c2 ? c - q.c2 : 0;
  const left = c < q.c1 ? q.c1 - c : 0;
  const v = Math.max(down, up);
  const h = Math.max(right, left);
  if (!v && !h) return null;
  if (h >= v) return right >= left ? { ...q, c2: c } : { ...q, c1: c };
  return down >= up ? { ...q, r2: r } : { ...q, r1: r };
}

// The extension is placed per line, and the selection grows with it.
export function applyExtend(g, plan) {
  if (!plan || !plan.slots.length) return g;
  let out = g;
  for (const kind of new Set(plan.slots.map((s) => s.kind)))
    out = place(
      out,
      kind,
      plan.slots.filter((s) => s.kind === kind),
    );
  return selectRect(out, plan.to);
}

// A cell the live panel leaves out only for want of a Source or Destination.
export const isUnassigned = (c) => !cellsIn({ rows: c.r + c.rs, cols: c.c + c.cs, cells: [c] }).length;
// What a live panel shows of this grid: every cell it can use, so unassigned ones are left out.
export const liveCells = (g) => cellsIn(g);

// The grid as a panel stores it: reading order, no selection. Unassigned cells are kept.
// Fixed destination keeps Destination cells out of the editor's grid but in the stored panel, so no edit can touch a
// square nobody sees. putBack drops a held cell whose squares were filled meanwhile or that no longer fits;
// the editor drops them at once and says how many (paneleditor.js holdOrPutBack).
export function setAside(g, kind) {
  const held = g.cells.filter((c) => c.kind === kind);
  return held.length
    ? {
        grid: withCells(
          g,
          g.cells.filter((c) => c.kind !== kind),
        ),
        held,
      }
    : { grid: g, held };
}
// The held cells putBack would keep: the first held cell wins a square two of them share.
export function heldThatFit(g, held) {
  const kept = [];
  for (const h of held) {
    const { r2, c2 } = span(h);
    const area = { r1: h.r, c1: h.c, r2, c2 };
    if (r2 <= g.rows && c2 <= g.cols && ![...g.cells, ...kept].some((x) => touches(x, area))) kept.push(h);
  }
  return kept;
}
export function putBack(g, held) {
  const kept = heldThatFit(g, held);
  return kept.length ? withCells(g, [...g.cells, ...kept]) : g;
}

export const saved = (g) => ({ rows: g.rows, cols: g.cols, cells: g.cells.map((c) => ({ ...c })).sort(reading) });
