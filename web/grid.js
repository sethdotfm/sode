// The /grid/<link> page (#150): a panel of source, destination, salvo and
// Confirm buttons. Without a Confirm button, tap destinations to light them,
// then tap a source to take it to all of them on the panel's levels (with
// the churning-letter confirm unless the panel sends at once). With one, a
// source tap only selects it and Confirm sends the route, at once.
import { $, el, S, on, onMsg, connect, connState, setDot, labelNode, srcItem, dstItem, tallyText, protectedOn, protectedLabel } from './core.js';
import { levelText, parseLevels } from './levelview.js';
import { patchPlan, salvoPlan, planBlocked, sendTakes, showResult, resultNode } from './plans.js';
import { keyConfirm } from './keyconfirm.js';
import { salvoNamed } from './salvostore.js';
import { confirmFor } from './gridgeom.js';
import * as G from './gridstate.js';
import './device.js';
import './brand.js';
import './lock.js'; // locks (#151): the page type comes from the path

// The link is the last segment of the path (/grid/<link>).
const link = decodeURIComponent(location.pathname.split('/').pop());

const board = $('grid-board');
let st = G.newState();
let grid = null;
let cells = []; // [{ cell, node, ... }] for the current grid

const currentGrid = () => (S.cfg?.grids || []).find((g) => g.link === link) || null;
const text = (t) => el('span', 'sode-grid__now', t);

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
// selected, protection) are updated in place by refresh, so a tally message
// never replaces a button under a finger.
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
    missing.lastChild.href = 'settings/grids';
    return;
  }
  if (!G.hasConfirm(grid)) st = { ...st, src: null };
  board.style.setProperty('--sode-grid-cols', grid.cols);
  board.style.gridTemplateRows = `repeat(${grid.rows}, minmax(5.5rem, 1fr))`;
  if (!grid.cells.length) board.append(el('p', 'sode__hint', 'This grid has no buttons yet.'));
  for (const cell of grid.cells) {
    const place = (n) => {
      n.style.gridColumn = `${cell.x + 1} / span ${cell.w}`;
      n.style.gridRow = `${cell.y + 1} / span ${cell.h}`;
      return n;
    };
    if (!cell.kind) { // a spacer: an empty slot
      const gap = place(el('div', 'sode-grid__spacer-cell'));
      gap.setAttribute('aria-hidden', 'true');
      board.append(gap);
      continue;
    }
    const b = place(el('button', `sode-grid__cell sode-grid__cell--${cell.kind}`));
    b.type = 'button';
    const entry = { cell, node: b, now: null, tags: el('span', 'sode-grid__tags'), nameEl: el('span', 'sode-grid__name'), numEl: null };
    b.append(entry.nameEl);
    if (cell.kind === 'confirm') {
      entry.now = text('');
      b.append(entry.now);
      b.addEventListener('click', confirmPressed);
    } else if (cell.kind === 'salvo') {
      entry.now = text('');
      b.append(entry.now);
      b.addEventListener('click', () => fireSalvo(cell));
    } else {
      const n = Number(cell.ref);
      entry.numEl = el('span', 'sode-grid__num');
      b.append(entry.numEl);
      if (cell.kind === 'dst') {
        entry.now = text('');
        b.append(entry.now);
        b.setAttribute('aria-pressed', 'false');
      }
      b.append(entry.tags);
      b.addEventListener('click', () => (cell.kind === 'src' ? tapSource(n) : tapDestination(n)));
    }
    board.append(b);
    cells.push(entry);
  }
  relabel();
  applyBusy();
  refresh();
}

// The cells' texts from the names now known.
function relabel() {
  for (const e of cells) {
    const { cell } = e;
    if (cell.kind === 'confirm') e.nameEl.textContent = cell.label || 'Confirm';
    else if (cell.kind === 'salvo') e.nameEl.textContent = cell.label || cell.ref;
    else {
      const it = cell.kind === 'src' ? srcItem(Number(cell.ref)) : dstItem(Number(cell.ref));
      e.nameEl.textContent = cell.label || it.name || it.num;
      e.numEl.textContent = `${cell.kind === 'src' ? 'Source' : 'Destination'} ${it.num}`;
    }
  }
}

// Where a destination is routed now on the panel's levels, in words.
function nowText(dst, levels) {
  const t = S.tally.get(dst);
  const seen = levels.map((u) => t?.get(u));
  if (!seen.length || seen.every((s) => s == null)) return 'Now: unknown';
  if (seen.every((s) => s === seen[0])) return `Now: ${tallyText(seen[0])}`;
  return `Now: mixed (${[...new Set(seen.map((s) => tallyText(s)))].join(', ')})`;
}

const badge = (kind, t) => el('span', `pb-badge pb-badge--${kind}`, t);

function refresh() {
  if (!grid) return;
  const levels = levelsOf(grid);
  const withConfirm = G.hasConfirm(grid);
  const status = G.confirmStatus(st);
  for (const e of cells) {
    const { cell } = e;
    if (cell.kind === 'dst') {
      const n = Number(cell.ref);
      const on = st.lit.has(n);
      e.node.setAttribute('aria-pressed', String(on));
      e.now.textContent = on ? nowText(n, levels) : '';
      const tags = [];
      if (on) tags.push(badge('info', 'Lit'));
      const p = protectedOn(n, levels);
      if (p.levels.length) tags.push(badge('warn', protectedLabel(p.devices)));
      e.tags.replaceChildren(...tags);
    } else if (cell.kind === 'src') {
      const on = withConfirm && st.src === Number(cell.ref);
      if (withConfirm) e.node.setAttribute('aria-pressed', String(on));
      else e.node.removeAttribute('aria-pressed');
      e.tags.replaceChildren(...(on ? [badge('info', 'Selected')] : []));
    } else if (cell.kind === 'confirm') {
      e.node.disabled = !status.ready;
      e.now.textContent = status.ready ? `${srcItem(st.src).label} to ${st.lit.size} destination${st.lit.size === 1 ? '' : 's'}` : status.text;
    } else if (cell.kind === 'salvo') {
      const found = !!salvoNamed(cell.ref);
      e.node.disabled = !found;
      e.now.textContent = found ? '' : 'Salvo not found';
    }
  }
  $('grid-clear').disabled = !st.lit.size && st.src == null;
  const lv = `level${levels.length === 1 ? '' : 's'} ${levelText(levels)}`;
  $('grid-hint').textContent = withConfirm
    ? `Tap a source to select it and destinations to light them, then press Confirm to take on ${lv}.`
    : st.lit.size
      ? `${st.lit.size} destination${st.lit.size === 1 ? '' : 's'} lit. Tap a source to take it to ${st.lit.size === 1 ? 'it' : 'them'} on ${lv}.`
      : 'Tap one or more destinations to light them, then tap a source.';
}

function tapDestination(n) {
  st = G.tapDst(st, n);
  refresh();
}

$('grid-clear').addEventListener('click', () => { st = G.clearAll(st); refresh(); });

// ---- taking

// A take is in flight: buttons that send wait, so a double tap sends one batch.
let busy = false;
let busyTimer = 0;
function applyBusy() {
  for (const e of cells) if (e.cell.kind !== 'dst') e.node.setAttribute('aria-disabled', String(busy));
}
function setBusy(on) {
  clearTimeout(busyTimer);
  busy = on;
  if (on) busyTimer = setTimeout(() => setBusy(false), 5000);
  applyBusy();
}

// A plan is sent at once (Confirm, or a panel that sends at once) or after
// the churning-letter confirm. Protected destinations are refused by the
// router and said so; an all-protected plan is blocked with a message.
// False if it was not sent or opened.
function dispatch(plan, viaDialog) {
  $('grid-prot').hidden = true;
  const { block, prot } = planBlocked(plan);
  if (block) { showResult(block.kind, block.title, block.text); return false; }
  if (viaDialog) { confirm.open(plan, prot); return true; }
  return send(plan, prot);
}

function send(plan, prot) {
  const ok = sendTakes(plan);
  if (ok) setBusy(true);
  notes(prot, plan.warnings || []);
  return ok;
}

// Without a Confirm button: a source tap takes it to the lit destinations.
function tapSource(n) {
  if (busy) return;
  if (G.hasConfirm(grid)) {
    st = G.tapSrc(st, n);
    return refresh();
  }
  if (!st.lit.size) return showResult('warn', 'No destination lit', 'Tap one or more destinations to light them first, then tap a source.');
  const plan = routePlan(n);
  if (plan) dispatch(plan, confirmFor(grid, S.cfg.gridConfirm));
}

function routePlan(n) {
  const levels = levelsOf(grid);
  if (!levels.length) { showResult('error', 'No levels', 'This grid has no levels that exist on the router.'); return null; }
  return patchPlan(srcItem(n), [...st.lit].sort((a, b) => a - b).map((d) => dstItem(d)), levels);
}

// The Confirm button is the confirmation: it sends at once.
function confirmPressed() {
  if (busy || !G.confirmStatus(st).ready) return;
  const plan = routePlan(st.src);
  if (plan && dispatch(plan, false)) { st = G.afterConfirm(st); refresh(); }
}

function fireSalvo(cell) {
  if (busy) return;
  const s = salvoNamed(cell.ref);
  if (!s) return;
  const plan = salvoPlan(s, levelsOf(grid));
  if (!plan.items.length) {
    return showResult('error', `Salvo ${s.name} not sent`, plan.warnings.length ? `${plan.warnings.join('; ')}.` : 'It has no takes.');
  }
  dispatch(plan, confirmFor(grid, S.cfg.gridConfirm));
}

// Names the destinations that are protected and what a plan left out, kept
// on the page after the result banner moves on.
function notes(prot, warnings) {
  const box = $('grid-prot');
  const kids = [];
  if (prot.size) {
    kids.push(el('div', null, el('strong', null, 'Protected, not changed on the protected levels: '),
      ...[...prot].flatMap(([item, p], i) => [i ? '; ' : '', labelNode(item.dst), ` (levels ${levelText(p.levels)}, ${protectedLabel(p.devices).toLowerCase()})`])));
  }
  if (warnings.length) kids.push(el('div', null, el('strong', null, 'Left out: '), warnings.join('; ')));
  box.replaceChildren(...kids);
  box.hidden = !kids.length;
}

// ---- confirm: the same churning letter and lockout as the main page

const confirm = keyConfirm({
  prompt: (key) => ['Press ', key, S.cfg.confirmEnter ? ' or Enter' : '', ' to confirm, or tap Take once it is ready.'],
  onTake: (plan, _viaKeys, prot) => { send(plan, prot); },
  onClose: () => {},
});
$('confirm-dialog').addEventListener('keydown', confirm.key);

// ---- wiring

on('result', (r) => {
  $('grid-result').replaceChildren(resultNode(r));
  if (r.kind !== 'info') setBusy(false); // the take has answered
});
on('wsclose', () => setBusy(false));
on('hello', () => {
  st = G.prune(st, (n) => S.srcBy.has(n), (n) => S.dstBy.has(n));
  confirm.close();
  build();
});
on('names', () => { relabel(); refresh(); });
on('tally', refresh);
onMsg('grids', (m) => {
  if (!S.cfg) return;
  S.cfg.grids = m.grids;
  S.cfg.gridConfirm = m.gridConfirm;
  build();
});
on('salvos', refresh);
on('conn', () => {
  const c = connState();
  setDot($('grid-ws-dot'), $('grid-ws-text'), [c.ws[0], 'Server']);
  setDot($('grid-rt-dot'), $('grid-rt-text'), [c.rt[0], 'Router']);
  const down = $('grid-down');
  down.hidden = !c.down;
  if (c.down) down.replaceChildren(el('strong', null, `${c.down.title}. `), c.down.text);
});

connect();
