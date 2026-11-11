// Find in the panel editor, pure: what can be searched, which of it the signed-in user's Rights allow,
// how results rank, and how a chosen result fills the selection. The page side is editorfind.js.
import { allowed } from './rightsrules.js';
import { fillPlan, applyFill, setCell, rectOf, select, isUnassigned, cellAt } from './gridmodel.js';

export const CATEGORIES = ['source', 'destination', 'preset', 'take', 'clear', 'current', 'panelName', 'instanceName', 'page', 'pageName', 'info', 'goToPage', 'nextPage', 'previousPage'];
// One entry each, placed with nothing chosen: a Go to Page cell gets its Page in the Cell tab.
const NUMBERLESS = ['take', 'clear', 'current', 'panelName', 'instanceName', 'pageName', 'info', 'goToPage', 'nextPage', 'previousPage'];
// The kinds with a list behind them get a chip; the others are one entry each and show under All.
export const CHIP_CATEGORIES = ['source', 'destination', 'preset', 'page'];
// What each chip narrows to: Pages also holds every kind that is about Pages, one entry each.
export const CHIP_KINDS = { page: ['page', 'pageName', 'goToPage', 'nextPage', 'previousPage'] };
export const chipKinds = (chip) => (chip ? CHIP_KINDS[chip] || [chip] : []);
export const CATEGORY_LABEL = {
  source: 'Sources',
  destination: 'Destinations',
  preset: 'Presets',
  take: 'Take',
  clear: 'Clear',
  current: 'Current Source',
  panelName: 'Panel Name',
  instanceName: 'Instance Name',
  pageName: 'Page Name',
  info: 'Info',
  goToPage: 'Go to Page',
  nextPage: 'Next Page',
  previousPage: 'Previous Page',
  page: 'Pages',
};
// Not log-in and log-out: the glossary UI test refuses those words.
export const CATEGORY_ICON = {
  source: 'arrow-right-from-line',
  destination: 'arrow-right-to-line',
  preset: 'bookmark',
  take: 'zap',
  clear: 'eraser',
  current: 'eye',
  panelName: 'panels-top-left',
  instanceName: 'tablet-smartphone',
  pageName: 'file-type',
  info: 'info',
  goToPage: 'file-symlink',
  nextPage: 'chevron-right',
  previousPage: 'chevron-left',
};
// Not in CATEGORY_ICON: that list is the cell kinds, which the artwork picker offers, and a Page is not a cell kind.
export const findIcon = (category) => (category === 'page' ? 'file' : CATEGORY_ICON[category]); // keep in step with editorview.js KIND_ICON
const RIGHTS_KIND = { source: 'sources', destination: 'dsts', preset: 'presets' };

// Everything a user can pick, as { category, key, label, n?, num?, scope?, off, why? }. Numbers in Rights are
// the router's own (n); num is what a person sees. Hidden removes what the Rights exclude, Read-only
// (the default) keeps it with off set. Names and presets are the /api/v1 replies. With fixed (the panel's
// Fixed destination is on) every Destination is off too, whatever the Rights, since its cell would not be drawn.
export function findEntries(namesReply, presetsReply, rights, { fixed = false } = {}) {
  const off = namesReply?.numberOffset || 0;
  const hide = rights?.visibility === 'hidden';
  const out = [];
  const add = (e, value) => {
    e.off = !allowed(rights, RIGHTS_KIND[e.category], value);
    if (e.off && hide) return;
    if (e.off) e.why = 'rights';
    if (fixed && e.category === 'destination') Object.assign(e, { off: true, why: 'fixed' });
    out.push(e);
  };
  const numbered = (category, list) => {
    for (const x of [...(list || [])].sort((a, b) => a.n - b.n)) {
      add(
        { category, key: `${category}:${x.n}`, label: x.name || `${category === 'source' ? 'Source' : 'Destination'} ${x.n + off}`, n: x.n, num: x.n + off },
        x.n,
      );
    }
  };
  numbered('source', namesReply?.sources);
  numbered('destination', namesReply?.destinations);
  // A preset has no number, so "onward" follows the name.
  const byName = [...(presetsReply?.presets || [])].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));
  for (const p of byName) add({ category: 'preset', key: `preset:${p.scope}:${p.name}`, label: p.name, scope: p.scope }, p.name);
  // The numberless kinds are always placeable: Rights limit Sources, Destinations and Presets only.
  for (const category of NUMBERLESS) out.push({ category, key: category, label: CATEGORY_LABEL[category], off: false });
  return out;
}

// The panel's Pages as entries. Choosing one places Go to Page cells, so Rights do not apply.
// alias lets "page 3" match a named Page.
export const pageEntries = (pages) =>
  (pages || []).map((p, i) => ({ category: 'page', key: `page:${p.id}`, id: p.id, n: i, label: p.name?.trim() || `Page ${i + 1}`, num: i + 1, alias: `page ${i + 1}`, off: false }));

const OFF_TAG = { rights: 'Outside rights', fixed: 'Fixed destination is on' };
// Why an entry cannot be placed, '' when it can.
export const offTag = (e) => (e?.off ? OFF_TAG[e.why] || OFF_TAG.rights : '');

// The Panel tab's picker for Fixed destination: Destinations only, those already chosen left out.
export const fixedPick = (entries, query, chosen) => rankFind(entries, query, ['destination']).filter((e) => !chosen.includes(e.n));

// The chip Find opens on: the kind every selected cell was given while none has its Source, Destination or Preset
// chosen yet; else '' (All).
export function findScope(cells) {
  const kind = cells[0]?.kind;
  if (!CHIP_CATEGORIES.includes(kind)) return '';
  return cells.every((c) => c.kind === kind && isUnassigned({ r: 1, c: 1, rs: 1, cs: 1, ...c })) ? kind : '';
}

// The entries a choice places, in order. With picks (a set chosen in the list) those, of the first pick's
// category; otherwise start and every later one of its category in the order findEntries gave. Entries the
// Rights grey out are never placed: a cell for one could not be used.
export function fillRun(entries, start, picks) {
  if (!start || start.off) return [];
  const category = start.category;
  if (picks?.length) return picks.filter((e) => e.category === category && !e.off);
  const from = entries.findIndex((e) => e.key === start.key);
  return from < 0 ? [] : entries.slice(from).filter((e) => e.category === category && !e.off);
}

// What a fill would do, for the preview and for applyFind. Slots are fillPlan's, each with the entry it
// takes; order is one of gridmodel's FILL_ORDERS.
export function findPlan(g, run, order = 'across') {
  if (!run.length) return null;
  const plan = fillPlan(g, { kind: run[0].category, items: run.map((_, i) => i), order });
  if (!plan) return null;
  return { ...plan, slots: plan.slots.map((s) => ({ r: s.r, c: s.c, replace: s.replace, entry: run[s.n] })) };
}

// Two units in the selection, a unit being a cell (merged ones included) or an empty square: side by side they take
// Previous Page then Next Page, left to right; one above the other, Previous on top. Chosen with either kind. null for
// any other selection.
export function pairPlan(g, start, all) {
  if (!start || start.off || !['nextPage', 'previousPage'].includes(start.category)) return null;
  const q = rectOf(g);
  if (!q) return null;
  const units = [];
  for (let r = q.r1; r <= q.r2; r++)
    for (let c = q.c1; c <= q.c2; c++) {
      const hit = cellAt(g, r, c);
      const u = hit ? { r: hit.r, c: hit.c, cs: hit.cs || 1, replace: true } : { r, c, cs: 1, replace: false };
      if (!units.some((x) => x.r === u.r && x.c === u.c)) units.push(u);
    }
  if (units.length !== 2) return null;
  const [a, b] = units;
  const stacked = a.c < b.c + b.cs && b.c < a.c + a.cs;
  units.sort((x, y) => (stacked ? x.r - y.r : x.c - y.c));
  const prev = all.find((e) => e.category === 'previousPage');
  const next = all.find((e) => e.category === 'nextPage');
  if (!prev || !next) return null;
  return { kind: 'pair', pair: true, dropped: 0, slots: units.map((u, i) => ({ r: u.r, c: u.c, replace: u.replace, entry: i ? next : prev })) };
}

// fillPlan stores a router number per slot; only Sources and Destinations have one, so n is removed afterwards, and a Preset gets scope and name.
export function applyFind(g, plan) {
  if (!plan || !plan.slots.length) return g;
  if (plan.pair) {
    let out = g;
    for (const s of plan.slots) {
      out = applyFill(out, { kind: s.entry.category, slots: [{ ...s, n: 0 }] });
      out = setCell(out, s, { n: undefined });
    }
    return out;
  }
  const page = plan.kind === 'page';
  let out = applyFill(g, { ...plan, kind: page ? 'goToPage' : plan.kind, slots: plan.slots.map((s) => ({ ...s, n: s.entry.n ?? 0 })) });
  if (page) {
    for (const s of plan.slots) out = setCell(out, s, { n: undefined, page: s.entry.id });
    return out;
  }
  if (plan.kind !== 'source' && plan.kind !== 'destination') {
    const preset = plan.kind === 'preset';
    for (const s of plan.slots) out = setCell(out, s, { n: undefined, scope: preset ? s.entry.scope : undefined, name: preset ? s.entry.label : undefined });
  }
  return out;
}

// After a click on one selected square: the selection moves to the next square across, else stays.
export function afterFill(g) {
  const q = rectOf(g);
  if (!q) return g;
  if (q.c2 < g.cols) return select(g, q.r1, q.c2 + 1);
  if (q.r2 < g.rows) return select(g, q.r2 + 1, 1);
  return g;
}

const subsequence = (text, word) => {
  let i = 0;
  for (const ch of text) if (ch === word[i] && ++i === word.length) return true;
  return false;
};

// How well one word matches an entry: 0 is no match. A typed number matches the number a person sees.
export function wordScore(e, word) {
  const label = e.label.toLowerCase();
  let best = 0;
  if (label === word) best = 100;
  else if (label.startsWith(word)) best = 80;
  else if (label.split(/[\s\-_.]+/).some((w) => w.startsWith(word))) best = 60;
  else if (label.includes(word)) best = 40;
  else if (word.length > 1 && subsequence(label, word)) best = 15;
  if (e.alias?.includes(word)) best = Math.max(best, 40);
  if (/^\d+$/.test(word) && e.num !== undefined) {
    const num = String(e.num);
    if (num === word) best = Math.max(best, 70);
    else if (num.startsWith(word)) best = Math.max(best, 50);
  }
  return best;
}

// Matching entries best first. Every word must match. Ties go to what the Rights allow, then to the
// category order, then to number (preset: name). categories narrows to those; empty means all.
export function rankFind(entries, query, categories = []) {
  const words = String(query || '')
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean);
  const scored = [];
  for (const e of entries) {
    if (categories.length && !categories.includes(e.category)) continue;
    let score = 0;
    for (const w of words) {
      const s = wordScore(e, w);
      if (!s) {
        score = -1;
        break;
      }
      score += s;
    }
    if (score >= 0) scored.push({ e, score });
  }
  const order = (e) => CATEGORIES.indexOf(e.category);
  scored.sort(
    (a, b) =>
      b.score - a.score ||
      (words.length ? Number(a.e.off) - Number(b.e.off) : 0) ||
      order(a.e) - order(b.e) ||
      (a.e.n ?? 0) - (b.e.n ?? 0) ||
      a.e.label.localeCompare(b.e.label),
  );
  return scored.map((x) => x.e);
}
