// The editor's clipboard block: what Cmd or Ctrl+C writes and Cmd or Ctrl+V reads. Pure (test/gridclip.test.mjs);
// editorclip.js moves the text in and out of the system clipboard.
//
// A block is { sode: CLIP_MARK, version, panel, rows, cols, cells }: the copied rectangle's size and its cells as
// stored (gridmodel.js), with r and c counted from the rectangle's top left. panel is the panel copied from ('' when
// unsaved), since an uploaded image belongs to that panel's artwork store.
import { rectOf, selCells, ROWS_MAX, COLS_MAX } from './gridmodel.js';
import { KINDS, PRESET_SCOPES } from './panelrules.js';
import { artOf, ART_B_CHOICES } from './panellook.js';
import { SHOW_PARTS } from './pagecells.js';

export const CLIP_MARK = 'sode-panel-cells';
export const CLIP_VERSION = 1;
// Far past any real block (24 by 24 cells), so a stray paste of a huge text is refused before parsing.
const TEXT_MAX = 1_000_000;

const posInt = (v) => Number.isInteger(v) && v >= 1;
export const overlap = (a, b) => !(a.r + a.rs - 1 < b.r || b.r + b.rs - 1 < a.r || a.c + a.cs - 1 < b.c || b.c + b.cs - 1 < a.c);
export const plural = (n, one, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

// A clipboard is text anyone can write, so each field is checked as the Cell tab and the server would make it.
// Keep in step with CAPTION_MAX (editorcell.js), FONT_MIN and FONT_MAX (sidebarrules.js), pageIDRe
// (internal/app/panelpagelist.go) and dstNumberMax (internal/app/panelfixed.go).
const CAPTION_MAX = 40;
const NUMBER_MAX = 0xffff;
const LEVEL_MAX = 256;
const NAME_MAX = 200;
const PAGE_ID = /^[A-Za-z0-9_-]{1,16}$/;
const str = (max) => (v) => typeof v === 'string' && v.length <= max;
const CELL_FIELDS = {
  r: posInt,
  c: posInt,
  rs: posInt,
  cs: posInt,
  kind: (v) => KINDS.includes(v),
  n: (v) => Number.isInteger(v) && v >= 0 && v <= NUMBER_MAX,
  levels: (v) => Array.isArray(v) && v.length <= LEVEL_MAX && v.every((l) => Number.isInteger(l) && l >= 1 && l <= LEVEL_MAX),
  caption: str(CAPTION_MAX),
  scope: (v) => PRESET_SCOPES.includes(v),
  name: str(NAME_MAX),
  page: (v) => v === '' || (typeof v === 'string' && PAGE_ID.test(v)),
  show: (v, kind) => Array.isArray(v) && v.every((x, i) => SHOW_PARTS[kind]?.includes(x) && v.indexOf(x) === i),
  art: (v) => !!artOf({ art: v }),
  artB: (v) => ART_B_CHOICES.includes(v),
  hideText: (v) => typeof v === 'boolean',
  fontSize: (v) => Number.isInteger(v) && v >= 50 && v <= 200,
};

// A copy of the cell with only known fields, or null when any known field has the wrong shape.
function cleanCell(c) {
  if (!c || typeof c !== 'object' || Array.isArray(c)) return null;
  const out = {};
  for (const [k, ok] of Object.entries(CELL_FIELDS)) {
    if (!(k in c) || c[k] === undefined) continue;
    if (!ok(c[k], c.kind)) return null;
    out[k] = c[k];
  }
  return out;
}

// null when nothing is selected or the selection holds no cell.
export function copyBlock(g, panel) {
  const q = rectOf(g);
  const cells = selCells(g);
  if (!q || !cells.length) return null;
  return {
    sode: CLIP_MARK,
    version: CLIP_VERSION,
    panel: panel || '',
    rows: q.r2 - q.r1 + 1,
    cols: q.c2 - q.c1 + 1,
    cells: cells.map((c) => ({ ...structuredClone(c), r: c.r - q.r1 + 1, c: c.c - q.c1 + 1 })),
  };
}

export const blockText = (block) => JSON.stringify(block);

// The block in text, or null for anything else. One bad cell refuses the whole block, so a paste is all or nothing.
export function readBlock(text) {
  if (typeof text !== 'string' || !text || text.length > TEXT_MAX) return null;
  let b;
  try {
    b = JSON.parse(text);
  } catch {
    return null;
  }
  if (!b || typeof b !== 'object' || Array.isArray(b) || b.sode !== CLIP_MARK || b.version !== CLIP_VERSION) return null;
  if (typeof b.panel !== 'string' || !posInt(b.rows) || !posInt(b.cols) || b.rows > ROWS_MAX || b.cols > COLS_MAX) return null;
  if (!Array.isArray(b.cells) || !b.cells.length || b.cells.length > b.rows * b.cols) return null;
  const seen = [];
  for (const raw of b.cells) {
    const c = cleanCell(raw);
    if (!c || !c.kind || ![c.r, c.c, c.rs, c.cs].every(posInt) || c.r + c.rs - 1 > b.rows || c.c + c.cs - 1 > b.cols) return null;
    if (seen.some((x) => overlap(x, c))) return null;
    seen.push(c);
  }
  return { sode: b.sode, version: b.version, panel: b.panel, rows: b.rows, cols: b.cols, cells: seen };
}

// Where a paste would land and what it would place, before anything changes. at is the selection's top left.
// Uploaded images (artOf gives an id that is not a library icon) stay with the panel they were uploaded to, so they
// are dropped across panels; an unsaved panel has none of its own. A Go to Page target is a Page id of the panel copied
// from (ADR 0007), so across panels it is dropped and the cell arrives unassigned; an unsaved panel's Pages already
// have their ids, so it counts as the same panel. Destination cells are skipped while Fixed
// destination is on (fixed), so they never reach the editor's held cells. fits false: some placed cell runs past
// the grid's edge (outside counts them) and the grid must not change.
export function pastePlan(g, block, { panel = '', fixed = false } = {}) {
  const q = rectOf(g);
  if (!q) return null;
  const at = { r: q.r1, c: q.c1 };
  const samePanel = block.panel === panel;
  let lostArt = 0;
  let lostTargets = 0;
  let skippedDests = 0;
  const placed = [];
  for (const cell of block.cells) {
    if (fixed && cell.kind === 'destination') {
      skippedDests++;
      continue;
    }
    const c = { ...structuredClone(cell), r: cell.r + at.r - 1, c: cell.c + at.c - 1 };
    const a = artOf(c);
    if ((!samePanel || !panel) && a?.id && !a.lib) {
      delete c.art;
      lostArt++;
    }
    if (!samePanel && c.kind === 'goToPage' && c.page) {
      delete c.page;
      lostTargets++;
    }
    placed.push(c);
  }
  const outside = placed.filter((c) => c.r + c.rs - 1 > g.rows || c.c + c.cs - 1 > g.cols).length;
  return { at, rows: block.rows, cols: block.cols, placed, fits: outside === 0, outside, lostArt, lostTargets, skippedDests };
}

// One paste as one new grid, so an undo step can wrap it. placed holds absolute cells inside the grid; a cell
// already there that shares a square with one of them goes whole, and the rest stay, so a blank square in the
// block leaves the grid's square as it was. The selection becomes the area the placed cells cover.
export function applyPaste(g, placed) {
  if (!placed.length) return g;
  const cells = [...g.cells.filter((x) => !placed.some((p) => overlap(x, p))), ...placed];
  const r1 = Math.min(...placed.map((p) => p.r));
  const c1 = Math.min(...placed.map((p) => p.c));
  const r2 = Math.max(...placed.map((p) => p.r + p.rs - 1));
  const c2 = Math.max(...placed.map((p) => p.c + p.cs - 1));
  return { ...g, cells, sel: { anchor: { r: r1, c: c1 }, head: { r: r2, c: c2 } } };
}

// The status line for a plan, applied or not.
export function pasteNote(plan) {
  if (!plan.fits) return `Nothing pasted: ${plural(plan.outside, 'cell')} would run past the edge of the grid. Pick a square further up or left.`;
  const parts = [plan.placed.length ? `Pasted ${plural(plan.placed.length, 'cell')}.` : 'Nothing pasted.'];
  if (plan.lostArt)
    parts.push(
      `${plural(plan.lostArt, 'cell')} lost ${plan.lostArt === 1 ? 'its' : 'their'} uploaded image, since images stay with the panel they were added to.`,
    );
  if (plan.lostTargets) parts.push(`${plural(plan.lostTargets, 'Go to Page cell')} came in unassigned, since a Page belongs to the panel it is on.`);
  if (plan.skippedDests) parts.push(`Skipped ${plural(plan.skippedDests, 'Destination cell')}, since Fixed destination is on.`);
  return parts.join(' ');
}
