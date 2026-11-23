// The preset editor's Add from Patch Boxes session (docs/agents/presets.md). While it is active, the patch boxes' Take
// and the Grid popup add routes to the preset and send nothing. Every change emits 'presetAdd'. A leaf module, so the
// layouts, the popup, the pick bar and the editor can all read it.
import { emit, routerData } from './core.js';

// { label, rows(), add(items), onDone(), note, covered }. add is presets.js addToPreset: { ok, rows, text }.
let s = null;
const told = () => emit('presetAdd');
const NO_PATCH = 'Pick a Source and a Destination first.';

export const presetAdd = {
  active: () => !!s,
  label: () => (s?.label ? `“${s.label}”` : 'the preset'),
  // The preset's rows as preset items, [] with no session.
  rows: () => s?.rows() || [],
  // The pick bar's line: the last row count, or why the boxes' patch added nothing.
  note: () => s?.note || '',
  // True while the Grid popup covers the pick bar, so Esc closes the popup and not the session.
  covered: () => !!s?.covered,
  start(opts) {
    if (s) return;
    s = { ...opts, note: '', covered: false };
    told();
  },
  // A failed add keeps the bar's count: the popup says why itself.
  add(items) {
    if (!s) return { ok: false, rows: 0, text: '' };
    const r = s.add(items);
    if (r.ok) {
      s.note = r.text;
      told();
    }
    return r;
  },
  // The bar's line after an add that came later: the tie lines of a route across routers were chosen first.
  say(text) {
    if (!s) return;
    s.note = text;
    told();
  },
  // The boxes' patch (a host's patch(), or null) as rows: one for each router of its destinations. Here the bar says
  // why nothing was added.
  addPatch(p) {
    const r = p ? this.add(patchRows(p)) : { ok: false, rows: 0, text: NO_PATCH };
    if (s && !r.ok) {
      s.note = r.text;
      told();
    }
    return r;
  },
  cover(on) {
    if (s) s.covered = on;
  },
  // Done: the editor comes back (onDone).
  done() {
    const was = s;
    s = null;
    told();
    was?.onDone?.();
  },
  // A lock or a closed editor: no onDone.
  end() {
    if (!s) return;
    s = null;
    told();
  },
};

// patchRows: the boxes' patch as preset items with full router UIDs. A row whose destinations are on another router
// than its source is a route across routers (presetref.js): the editor asks for its tie lines.
function patchRows(p) {
  const router = routerData(p.src.router).uid;
  const by = new Map();
  for (const d of p.dsts) {
    const u = routerData(d.router).uid;
    if (!by.has(u)) by.set(u, []);
    by.get(u).push(d.n);
  }
  return [...by].map(([u, dsts]) => ({ src: p.src.n, dsts, levels: p.levels, router, ...(u !== router ? { dstRouter: u } : {}) }));
}
