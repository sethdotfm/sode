// The /grid/<link> page (#150): a panel of source, destination and salvo
// buttons. Tap destinations to light them, then tap a source to take it to
// all of them on the panel's levels, with the usual confirm unless the
// panel is set to send at once.
import { $, el, S, on, onMsg, connect, connState, setDot, labelNode, srcItem, dstItem, tallyText, protectedOn, protectedLabel } from './core.js';
import { levelText, parseLevels } from './levelview.js';
import { patchPlan, planBlocked, planSummary, planVerb, sendTakes, showResult, resultNode } from './plans.js';
import { lockout } from './widgets.js';
import { confirmFor } from './gridgeom.js';
import './device.js';
import './brand.js';
import './lock.js'; // locks (#151): the page type comes from the path

// The link is the segment after /grid/ (the page may sit behind a proxy's prefix).
const link = (() => {
  const p = location.pathname;
  const i = p.lastIndexOf('/grid/');
  return decodeURIComponent((i < 0 ? '' : p.slice(i + 6)).split('/')[0]);
})();

const board = $('grid-board');
const lit = new Set(); // destination numbers
let grid = null;
let cells = []; // [{ cell, node, now }] for the current grid

const currentGrid = () => (S.cfg?.grids || []).find((g) => g.link === link) || null;

// The panel's levels: its own that this router has, else the default levels.
const levelsOf = (g) => {
  const own = (g.levels || []).filter((u) => S.levelBy.has(u));
  return own.length ? own : (parseLevels('') || []);
};

let brand = 'Sode';
on('brand', ({ text }) => { brand = text; title(); });
function title() {
  const name = grid?.name || 'Grid';
  $('grid-title').textContent = name;
  document.title = `${name} · ${brand}`;
}

// ---- drawing

// The structure: one button per cell, placed by its span. Dynamic bits (lit,
// current source, protection) are updated in place by refresh, so a tally
// message never replaces a button under a finger.
function build() {
  grid = currentGrid();
  title();
  board.replaceChildren();
  cells = [];
  const missing = $('grid-missing');
  missing.hidden = !!grid || !S.cfg;
  $('grid-bar').hidden = !grid;
  if (!grid) {
    missing.replaceChildren(el('strong', null, 'No such grid. '), link ? `There is no grid with the link "${link}".` : 'Add a link after /grid/ to open a grid.',
      ' ', el('a', null, 'Grids are set up in Settings.'));
    missing.lastChild.href = '../settings/grids';
    return;
  }
  board.style.setProperty('--sode-grid-cols', grid.cols);
  board.style.gridTemplateRows = `repeat(${grid.rows}, minmax(5.5rem, 1fr))`;
  if (!grid.cells.length) {
    board.append(el('p', 'sode__hint', 'This grid has no buttons yet.'));
  }
  for (const cell of grid.cells) {
    const b = el('button', `sode-grid__cell sode-grid__cell--${cell.kind}`);
    b.type = 'button';
    b.style.gridColumn = `${cell.x + 1} / span ${cell.w}`;
    b.style.gridRow = `${cell.y + 1} / span ${cell.h}`;
    const entry = { cell, node: b, now: null, tags: null, nameEl: null, numEl: null };
    if (cell.kind === 'salvo') {
      b.disabled = true;
      b.append(el('span', 'sode-grid__name', cell.label || cell.ref), el('span', 'sode-grid__now', 'needs Salvos'));
    } else {
      const n = Number(cell.ref);
      entry.nameEl = el('span', 'sode-grid__name');
      entry.numEl = el('span', 'sode-grid__num');
      b.append(entry.nameEl, entry.numEl);
      relabelEntry(entry);
      if (cell.kind === 'dst') {
        entry.now = el('span', 'sode-grid__now');
        entry.tags = el('span', 'sode-grid__tags');
        b.setAttribute('aria-pressed', 'false');
        b.append(entry.now, entry.tags);
      }
      b.addEventListener('click', () => (cell.kind === 'src' ? takeSource(n) : toggleDst(n)));
    }
    board.append(b);
    cells.push(entry);
  }
  if (busy) setBusy(true);
  refresh();
}

// The cell's texts from the names now known.
function relabelEntry(e) {
  const { cell } = e;
  const it = cell.kind === 'src' ? srcItem(Number(cell.ref)) : dstItem(Number(cell.ref));
  e.nameEl.textContent = cell.label || it.name || it.num;
  e.numEl.textContent = `${cell.kind === 'src' ? 'Source' : 'Destination'} ${it.num}`;
}

// New names change the texts in place: the buttons stay, so a tap is not lost.
function relabel() {
  for (const e of cells) if (e.nameEl) relabelEntry(e);
}

// Where a destination is routed now on the panel's levels, in words.
function nowText(dst, levels) {
  const t = S.tally.get(dst);
  const seen = levels.map((u) => t?.get(u));
  if (!seen.length || seen.every((s) => s == null)) return 'Now: unknown';
  if (seen.every((s) => s === seen[0])) return `Now: ${tallyText(seen[0])}`;
  return `Now: mixed (${[...new Set(seen.map((s) => tallyText(s)))].join(', ')})`;
}

function refresh() {
  if (!grid) return;
  const levels = levelsOf(grid);
  for (const e of cells) {
    if (e.cell.kind !== 'dst') continue;
    const n = Number(e.cell.ref);
    const on = lit.has(n);
    e.node.setAttribute('aria-pressed', String(on));
    e.now.textContent = on ? nowText(n, levels) : '';
    const tags = [];
    if (on) tags.push(el('span', 'pb-badge pb-badge--info', 'Lit'));
    const p = protectedOn(n, levels);
    if (p.levels.length) tags.push(el('span', 'pb-badge pb-badge--warn', protectedLabel(p.devices)));
    e.tags.replaceChildren(...tags);
  }
  $('grid-clear').disabled = !lit.size;
  $('grid-hint').textContent = lit.size
    ? `${lit.size} destination${lit.size === 1 ? '' : 's'} lit. Tap a source to take it to ${lit.size === 1 ? 'it' : 'them'} on level${levelsOf(grid).length === 1 ? '' : 's'} ${levelText(levels)}.`
    : 'Tap one or more destinations to light them, then tap a source.';
}

function toggleDst(n) {
  if (lit.has(n)) lit.delete(n);
  else lit.add(n);
  refresh();
}

$('grid-clear').addEventListener('click', () => { lit.clear(); refresh(); });

// ---- taking

// A take is in flight: source buttons wait, so a double tap sends one batch.
let busy = null;
function setBusy(on) {
  clearTimeout(busy);
  busy = on ? setTimeout(() => setBusy(false), 5000) : null;
  for (const e of cells) {
    if (e.cell.kind !== 'src') continue;
    e.node.setAttribute('aria-disabled', String(on));
  }
}

function takeSource(n) {
  if (busy) return;
  $('grid-prot').hidden = true;
  if (!lit.size) return showResult('warn', 'No destination lit', 'Tap one or more destinations to light them first, then tap a source.');
  const levels = levelsOf(grid);
  if (!levels.length) return showResult('error', 'No levels', 'This grid has no levels that exist on the router.');
  const dsts = [...lit].sort((a, b) => a - b).map((d) => dstItem(d));
  const plan = patchPlan(srcItem(n), dsts, levels);
  const { block, prot } = planBlocked(plan);
  if (block) return showResult(block.kind, block.title, block.text);
  if (confirmFor(grid, S.cfg.gridConfirm)) return openConfirm(plan, prot);
  // Sent at once. Protected destinations are refused by the router, and said so.
  if (sendTakes(plan)) setBusy(true);
  noteProtected(prot);
}

// Names the destinations that are protected, left on the page after the
// result banner moves on.
function noteProtected(prot) {
  const box = $('grid-prot');
  if (!prot.size) { box.hidden = true; return; }
  box.replaceChildren(el('strong', null, 'Protected, not changed on the protected levels: '),
    ...[...prot].flatMap(([item, p], i) => [i ? '; ' : '', labelNode(item.dst), ` (levels ${levelText(p.levels)}, ${protectedLabel(p.devices).toLowerCase()})`]));
  box.hidden = false;
}

// ---- confirm: the same churning letter and lockout as the main page

const KEYS = 'abcdefghjkmnpqrstuvwxyz';
let lastKey = '';
let pending = null;

function openConfirm(plan, prot) {
  pending?.lock.cancel();
  let key;
  do key = KEYS[Math.floor(Math.random() * KEYS.length)]; while (key === lastKey);
  lastKey = key;
  pending = { plan, prot, key };
  const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
  const verb = planVerb(plan);
  $('confirm-title').textContent = `Confirm ${verb}`;
  $('confirm-body').replaceChildren(
    ...planSummary(plan, { prot }),
    el('div', 'sode__prompt', 'Press ', keyEl, S.cfg.confirmEnter ? ' or Enter' : '', ' to confirm, or tap Take once it is ready.'),
  );
  $('confirm-ok').textContent = `Take (${key.toUpperCase()})`;
  $('confirm-ok').disabled = true;
  $('confirm').hidden = false;
  $('confirm-dialog').focus();
  pending.lock = lockout(() => {
    keyEl.classList.remove('sode__key--locked');
    $('confirm-ok').disabled = false;
  });
}

function closeConfirm() {
  pending?.lock.cancel();
  pending = null;
  $('confirm').hidden = true;
}

function doTake() {
  const p = pending;
  closeConfirm();
  if (sendTakes(p.plan)) setBusy(true);
  noteProtected(p.prot);
}

$('confirm-cancel').addEventListener('click', closeConfirm);
$('confirm-ok').addEventListener('click', () => { if (pending?.lock.ready()) doTake(); });
$('confirm-dialog').addEventListener('keydown', (e) => {
  e.preventDefault();
  e.stopPropagation();
  if (e.key === 'Escape') return closeConfirm();
  if (e.repeat || e.ctrlKey || e.metaKey || e.altKey || !pending?.lock.ready()) return;
  if (e.key.toLowerCase() === pending.key || (S.cfg.confirmEnter && e.key === 'Enter')) doTake();
});

// ---- wiring

on('result', (r) => {
  $('grid-result').replaceChildren(resultNode(r));
  if (r.kind !== 'info') setBusy(false); // the take has answered
});
on('wsclose', () => setBusy(false));
on('hello', () => {
  // A dst that no longer exists can't stay lit.
  for (const n of [...lit]) if (!S.dstBy.has(n)) lit.delete(n);
  closeConfirm();
  build();
});
on('names', relabel);
on('tally', refresh);
onMsg('grids', (m) => {
  if (!S.cfg) return;
  S.cfg.grids = m.grids;
  S.cfg.gridConfirm = m.gridConfirm;
  build();
});
on('conn', () => {
  const c = connState();
  setDot($('grid-ws-dot'), $('grid-ws-text'), [c.ws[0], 'Server']);
  setDot($('grid-rt-dot'), $('grid-rt-text'), [c.rt[0], 'Router']);
  const down = $('grid-down');
  down.hidden = !c.down;
  if (c.down) down.replaceChildren(el('strong', null, `${c.down.title}. `), c.down.text);
});

connect();
