// Grid panels (#150): the pure parts. No imports, so Node tests load it.
// A cell is { x, y, w, h, kind, ref, label }: zero-based, spans w x h, kind
// 'src' | 'dst' | 'salvo' | 'confirm' (no ref) | '' (a spacer: an empty slot
// that is saved like any other cell). Cells are the only truth; a square no
// cell covers is empty too. A rect is { x, y, w, h }.

// The rect between two squares, whichever corner each is.
export function rectBetween(a, b) {
  const x = Math.min(a.x, b.x);
  const y = Math.min(a.y, b.y);
  return { x, y, w: Math.max(a.x, b.x) - x + 1, h: Math.max(a.y, b.y) - y + 1 };
}

export const meets = (a, b) => a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;
const inside = (c, r) => c.x >= r.x && c.y >= r.y && c.x + c.w <= r.x + r.w && c.y + c.h <= r.y + r.h;

function union(a, b) {
  const x = Math.min(a.x, b.x);
  const y = Math.min(a.y, b.y);
  return { x, y, w: Math.max(a.x + a.w, b.x + b.w) - x, h: Math.max(a.y + a.h, b.y + b.h) - y };
}

// r grown until it holds every cell it touches whole, so a selection that
// clips a joined cell takes all of it.
export function expandRect(cells, r) {
  let out = { ...r };
  for (let again = true; again;) {
    again = false;
    for (const c of cells) {
      if (meets(out, c) && !inside(c, out)) {
        out = union(out, c);
        again = true;
      }
    }
  }
  return out;
}

export const cellAt = (cells, x, y) => cells.find((c) => meets(c, { x, y, w: 1, h: 1 })) || null;
export const cellsIn = (cells, r) => cells.filter((c) => meets(c, r));
const assigned = (c) => c.kind !== '';

// Joins everything in (the expansion of) r into one cell. It keeps the first
// assigned cell's kind, ref and label (top-left first), else is a spacer.
export function join(cells, r) {
  const rect = expandRect(cells, r);
  const inRect = cells.filter((c) => inside(c, rect)).sort((a, b) => a.y - b.y || a.x - b.x);
  if (rect.w * rect.h < 2) return { cells, rect };
  const from = inRect.find(assigned);
  const merged = { ...rect, kind: from?.kind || '', ref: from?.ref || '', label: from?.label || '' };
  return { cells: [...cells.filter((c) => !inside(c, rect)), merged], rect };
}

// Assigned cells a join of r would lose: all but the one it keeps.
export function lostByJoin(cells, r) {
  const rect = expandRect(cells, r);
  const n = cells.filter((c) => inside(c, rect) && assigned(c)).length;
  return rect.w * rect.h < 2 ? 0 : Math.max(0, n - 1);
}

// Assigned cells that resize to cols x rows would drop (those starting
// past the new edge; ones across it are only shortened).
export const lostByResize = (cells, cols, rows) => cells.filter((c) => (c.x >= cols || c.y >= rows) && assigned(c)).length;

// Splits every joined cell touching r back into single squares. The first
// square keeps the cell; the rest are left empty (no cell).
export function split(cells, r) {
  const out = [];
  for (const c of cells) {
    if (!meets(c, r) || (c.w === 1 && c.h === 1)) {
      out.push(c);
      continue;
    }
    out.push({ ...c, w: 1, h: 1 });
  }
  return out;
}

// cells[i] set to patch, or a new 1x1 cell at (x, y) if none is there.
export function assign(cells, x, y, patch) {
  const at = cellAt(cells, x, y);
  if (!at) return [...cells, { x, y, w: 1, h: 1, kind: '', ref: '', label: '', ...patch }];
  return cells.map((c) => (c === at ? { ...c, ...patch } : c));
}

export function remove(cells, r) {
  return cells.filter((c) => !meets(c, r));
}

// Cells that overlap one another, as pairs of indexes (the editor can't
// make them; a hand-edited file might).
export function overlaps(cells) {
  const out = [];
  for (let i = 0; i < cells.length; i++) {
    for (let j = i + 1; j < cells.length; j++) if (meets(cells[i], cells[j])) out.push([i, j]);
  }
  return out;
}

// The cells after the grid became cols x rows: those past the new edge are
// dropped, those across it shortened.
export function resize(cells, cols, rows) {
  const out = [];
  for (const c of cells) {
    if (c.x >= cols || c.y >= rows) continue;
    out.push({ ...c, w: Math.min(c.w, cols - c.x), h: Math.min(c.h, rows - c.y) });
  }
  return out;
}

// Every square of the grid as the editor draws it: the cells, and an empty
// 1x1 for each square no cell covers.
export function squares(cells, cols, rows) {
  const taken = new Set();
  for (const c of cells) for (let y = c.y; y < c.y + c.h; y++) for (let x = c.x; x < c.x + c.w; x++) taken.add(y * cols + x);
  const out = cells.map((c) => ({ ...c, cell: c }));
  for (let y = 0; y < rows; y++) {
    for (let x = 0; x < cols; x++) if (!taken.has(y * cols + x)) out.push({ x, y, w: 1, h: 1, cell: null });
  }
  return out.sort((a, b) => a.y - b.y || a.x - b.x);
}

// Whether a panel asks to confirm: its own setting, else the global default.
export function confirmFor(grid, globalConfirm) {
  if (grid.confirm === 'on') return true;
  if (grid.confirm === 'off') return false;
  return globalConfirm !== false;
}

// A link made from a name: lowercase letters, digits and single dashes.
export function slugify(name) {
  return String(name).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40).replace(/-+$/, '');
}

// Only for the live outline; the server has the last word on a link.
export const validLink = (s) => /^[a-z0-9]+(-[a-z0-9]+)*$/.test(s) && s.length <= 40;

// A link not used by another grid: base, base-2, base-3, …
export function freeLink(base, taken) {
  const root = slugify(base) || 'grid';
  let link = root;
  for (let n = 2; taken.includes(link) || !validLink(link); n++) link = `${root}-${n}`;
  return link;
}
