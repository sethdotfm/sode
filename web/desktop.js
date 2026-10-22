// Desktop layout: three boxes, Tab between them, churning-key confirm.

import {
  $, el, S, on, connState, setDot, search, parseLevels, levelText, levelInput, levelBar, pickButtons,
  allLevels, fedBy, dstItem, dstLabel, srcItem, currentlyNodes, fmtTime, undoPlan, requestHistory,
  beforeGroups, srcRow, tallyText, retryPlan, planVerb, entryNote,
  resultNode, showResult, patchPlan, planBlocked, planSummary, sendTakes,
} from './core.js';
import { layout } from './layout.js';

const sel = { src: null, dst: null };
const boxes = {
  src: { input: $('src'), menu: $('src-menu'), list: () => S.srcs, matches: [], hi: 0 },
  dst: { input: $('dst'), menu: $('dst-menu'), list: () => S.dsts, matches: [], hi: 0 },
};
const lvlInput = $('lvl');
let lvlNone = false;  // picker cleared to no levels (blank box would mean default)
let feedOpen = false; // "and N others" expanded

const active = () => layout() === 'desktop' && document.documentElement.dataset.view !== 'settings';
const mac = /Mac|iPhone|iPad/.test(navigator.platform);
const expanded = new Set(); // history IDs showing their before state
const entered = new Set();  // live entries already animated in

// ---- connection

function renderConn() {
  const c = connState();
  setDot($('ws-dot'), $('ws-text'), c.ws);
  setDot($('rt-dot'), $('rt-text'), c.rt);
  $('rt-name').textContent = `Router ${S.routerAddr}`;
  $('rt-text').parentElement.title = S.routerErr;
}

// ---- fuzzy finder

function refresh(k) {
  const b = boxes[k];
  b.matches = search(b.list(), b.input.value);
  b.hi = 0;
  renderMenu(k);
}

function renderMenu(k) {
  const b = boxes[k];
  const open = b.matches.length > 0 && !sel[k];
  b.menu.hidden = !open;
  b.input.setAttribute('aria-expanded', String(open));
  if (!open) { b.input.removeAttribute('aria-activedescendant'); return; }
  b.menu.replaceChildren(...b.matches.map((it, i) => {
    const li = el('li', 'pb-menu__item' + (i === b.hi ? ' active' : ''), el('span', 'sode__num', it.num), it.name);
    li.id = `${k}-opt-${i}`;
    li.setAttribute('role', 'option');
    li.setAttribute('aria-selected', String(i === b.hi));
    li.addEventListener('mousedown', (e) => { e.preventDefault(); b.hi = i; accept(k); });
    return li;
  }));
  b.input.setAttribute('aria-activedescendant', `${k}-opt-${b.hi}`);
}

function select(k, it) {
  sel[k] = it;
  if (k === 'src') feedOpen = false;
  const b = boxes[k];
  b.input.value = it ? it.label : '';
  b.input.classList.remove('pb-input--invalid');
  b.matches = [];
  renderMenu(k);
  renderTally();
}

function accept(k) {
  const it = boxes[k].matches[boxes[k].hi];
  if (!it) return false;
  select(k, it);
  return true;
}

// ---- level picker

function setLevels(list) {
  lvlNone = list.length === 0;
  lvlInput.value = levelInput(list);
  renderLevels();
  renderTally();
}

function renderLevels() {
  if (!S.cfg) return;
  const list = parseLevels(lvlInput.value, lvlNone);
  const on = new Set(list || []);
  lvlInput.classList.toggle('pb-input--invalid', !list);
  lvlInput.placeholder = lvlNone ? 'none' : S.cfg.defaultLevels;
  for (const b of $('lvl-pick').children) b.setAttribute('aria-pressed', String(on.has(+b.dataset.ui)));
  const full = on.size === S.levels.length;
  const all = $('lvl-all');
  all.textContent = full ? 'None' : 'All';
  all.dataset.tip = full ? 'Select no levels' : 'Select all levels';
  all.setAttribute('aria-label', all.dataset.tip);
}

function buildPicker() {
  $('lvl-pick').replaceChildren(...pickButtons((ui) => {
    const on = new Set(parseLevels(lvlInput.value, lvlNone) || []);
    if (on.has(ui)) on.delete(ui);
    else on.add(ui);
    setLevels([...on].sort((x, y) => x - y));
  }));
}

// ---- tally display

function renderTally() {
  const box = $('tally');
  if (!S.cfg) return box.replaceChildren();
  if (sel.dst) return box.replaceChildren(...currentlyNodes(sel.dst));
  if (sel.src) {
    const fed = fedBy(sel.src.n, parseLevels(lvlInput.value, lvlNone) || allLevels());
    return box.replaceChildren(el('div', 'sode__label', 'Currently'), feedingRow(fed));
  }
  box.replaceChildren();
}

function feedingRow(fed) {
  if (!fed.length) return el('div', 'sode__route', 'Not feeding any destination on these levels');
  if (feedOpen || fed.length <= 2) {
    const list = el('div', 'sode__chips', ...fed.map((d) => {
      const chip = el('button', 'pb-chip', dstLabel(d));
      chip.type = 'button';
      chip.title = 'Use as destination';
      chip.addEventListener('click', () => { select('dst', dstItem(d)); lvlInput.focus(); });
      return chip;
    }));
    const row = el('div', 'sode__feed', el('span', null, `Feeding ${fed.length} destination${fed.length === 1 ? '' : 's'}`), list);
    if (fed.length > 2) {
      const less = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Show less');
      less.type = 'button';
      less.setAttribute('aria-expanded', 'true');
      less.addEventListener('click', () => { feedOpen = false; renderTally(); });
      row.append(el('div', null, less));
    }
    return row;
  }
  const more = el('button', 'pb-btn pb-btn--ghost sode__more', `and ${fed.length - 2} others`);
  more.type = 'button';
  more.setAttribute('aria-expanded', 'false');
  more.addEventListener('click', () => { feedOpen = true; renderTally(); });
  return el('div', 'sode__route', `Feeding ${fed.slice(0, 2).map(dstLabel).join(', ')},`, more);
}

// ---- history

function histRow(e) {
  const src = srcItem(e.src, e.srcName), dst = dstItem(e.dst, e.dstName);
  const known = e.levels.filter((u) => S.levelBy.has(u));
  const status = el('span', 'pb-status',
    el('span', `pb-dot pb-dot--${e.ok ? 'ok' : 'error'}`),
    e.ok ? 'ok' : 'failed');
  status.firstChild.setAttribute('aria-hidden', 'true');
  if (!e.ok) status.title = e.err || '';

  const plan = undoPlan(e);
  const btn = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Undo');
  btn.type = 'button';
  btn.setAttribute('aria-label', `Undo ${src.label} to ${dst.label}`);
  if (!plan.groups.length) {
    btn.disabled = true;
    btn.title = 'Nothing to restore';
  }
  btn.addEventListener('click', () => openConfirm(plan));

  // Before: what the destination had on these levels just before the take.
  const open = expanded.has(e.id);
  const detailId = `hist-before-${e.id}`;
  const icon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  icon.setAttribute('class', 'pb-icon pb-icon--sm');
  icon.setAttribute('viewBox', '0 0 24 24');
  icon.setAttribute('aria-hidden', 'true');
  const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
  use.setAttribute('href', `vendor/patchbay/icons/lucide.svg#${open ? 'chevron-down' : 'chevron-right'}`);
  icon.append(use);
  const toggle = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode__hist-toggle', icon, 'Before');
  toggle.type = 'button';
  toggle.setAttribute('aria-expanded', String(open));
  toggle.setAttribute('aria-controls', detailId);
  toggle.addEventListener('click', () => {
    if (expanded.has(e.id)) expanded.delete(e.id);
    else expanded.add(e.id);
    renderHistory();
  });

  const row = el('li', 'pb-input sode__hist-row' + enterClass(e),
    el('span', 'sode__hist-id', `#${e.id}`),
    el('span', 'sode__hist-time', fmtTime(e.time)),
    levelBar(known),
    el('span', 'sode__hist-route', el('strong', null, src.label), ' → ', el('strong', null, dst.label),
      entryNote(e)),
    status,
    toggle,
    el('span', 'sode__hist-actions', retryButton(e, src, dst), btn));
  if (open) {
    const detail = el('div', 'sode__hist-before',
      el('div', 'sode__label', `Before #${e.id}, on ${dst.label}`),
      ...beforeGroups(e).map((g) => srcRow(g.levels, tallyText(g.src))));
    detail.id = detailId;
    row.append(detail);
  }
  return row;
}

// Successful rows keep an invisible Retry so every row's columns line up.
// A live entry animates in the first time it's drawn; nothing else does.
function enterClass(e) {
  if (e.id !== S.newId || entered.has(e.id)) return '';
  entered.add(e.id);
  return ' sode--enter';
}

function retryButton(e, src, dst) {
  const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm' + (e.ok ? ' sode__placeholder' : ''), 'Retry');
  b.type = 'button';
  if (e.ok) {
    b.tabIndex = -1;
    b.setAttribute('aria-hidden', 'true');
    return b;
  }
  b.setAttribute('aria-label', `Retry ${src.label} to ${dst.label}`);
  b.addEventListener('click', () => openConfirm(retryPlan(e)));
  return b;
}

function renderHistory() {
  $('history-head').hidden = !S.hist.length;
  $('history-hint').textContent = `(${mac ? '⌘Z' : 'Ctrl+Z'} undoes the last)`;
  $('history').replaceChildren(...S.hist.map(histRow));
  $('history-more').textContent = S.histLoading ? 'Loading…' : '';
  maybeLoadMore();
}

function maybeLoadMore() {
  if (!active()) return;
  const r = $('history-more').getBoundingClientRect();
  if (r.top > window.innerHeight + 200) return;
  if (requestHistory()) $('history-more').textContent = 'Loading…';
}

new IntersectionObserver(maybeLoadMore, { rootMargin: '200px' }).observe($('history-more'));

// ---- confirm

const KEYS = 'abcdefghjkmnpqrstuvwxyz';
let lastKey = '';
let pending = null; // plan + key
let readyAt = 0;

// Patch button and Enter: take any half-typed box's best match, then confirm.
function patchNow() {
  for (const k of ['src', 'dst']) {
    const b = boxes[k];
    if (!sel[k] && b.input.value.trim()) {
      if (!b.matches.length) refresh(k);
      accept(k);
    }
  }
  tryConfirm();
}

// Undo the newest take, through the usual confirm.
function undoLast() {
  const e = S.hist[0];
  if (!e) return showResult('info', 'Nothing to undo', 'No patches yet.');
  const plan = undoPlan(e);
  if (!plan.groups.length) {
    return showResult('warn', 'Nothing to undo', `#${e.id} had nothing routed before it, so there is nothing to restore.`);
  }
  openConfirm(plan);
}

function tryConfirm() {
  if (!S.cfg) return;
  for (const k of ['src', 'dst']) {
    if (!sel[k]) {
      boxes[k].input.classList.add('pb-input--invalid');
      boxes[k].input.focus();
      return;
    }
  }
  const levels = parseLevels(lvlInput.value, lvlNone);
  if (!levels) { lvlInput.classList.add('pb-input--invalid'); lvlInput.focus(); return; }
  openConfirm(patchPlan(sel.src, sel.dst, levels));
}

function openConfirm(plan) {
  const blocked = planBlocked(plan);
  if (blocked) return showResult(blocked.kind, blocked.title, blocked.text);
  let key;
  do key = KEYS[Math.floor(Math.random() * KEYS.length)]; while (key === lastKey);
  lastKey = key;
  pending = { ...plan, key };

  const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
  const verb = planVerb(plan);
  const Verb = verb[0].toUpperCase() + verb.slice(1);
  $('confirm-title').textContent = `Confirm ${verb}`;
  $('confirm-body').replaceChildren(
    ...planSummary(plan),
    el('div', 'sode__prompt', 'Press ', keyEl, S.cfg.confirmEnter ? ' or Enter' : '', ' to confirm, or escape to cancel.'),
  );
  $('confirm-ok').textContent = `${Verb} (${key.toUpperCase()})`;
  $('confirm-ok').disabled = true;
  $('confirm').hidden = false;
  $('confirm-dialog').focus();

  readyAt = performance.now() + S.cfg.confirmLockoutMs;
  setTimeout(() => {
    if (pending && pending.key === key) {
      keyEl.classList.remove('sode__key--locked');
      $('confirm-ok').disabled = false;
    }
  }, S.cfg.confirmLockoutMs);
}

function closeConfirm() {
  const wasUndo = pending && pending.undo;
  pending = null;
  $('confirm').hidden = true;
  if (!wasUndo) lvlInput.focus();
}

function doTake() {
  const p = pending;
  pending = null;
  $('confirm').hidden = true;
  if (sendTakes(p) && !p.undo) clearAll();
}

function onConfirmKey(e) {
  e.preventDefault();
  e.stopPropagation();
  if (e.key === 'Escape') return closeConfirm();
  if (e.repeat || e.ctrlKey || e.metaKey || e.altKey) return;
  if (performance.now() < readyAt) return;
  if (e.key.toLowerCase() === pending.key || (S.cfg.confirmEnter && e.key === 'Enter')) doTake();
}

// ---- keyboard & input wiring

function clearAll() {
  for (const k of ['src', 'dst']) select(k, null);
  lvlInput.value = '';
  lvlNone = false;
  renderLevels();
  renderTally();
  boxes.src.input.focus();
}

for (const k of ['src', 'dst']) {
  const b = boxes[k];
  b.input.addEventListener('input', () => {
    sel[k] = null;
    if (k === 'src') feedOpen = false;
    b.input.classList.remove('pb-input--invalid');
    refresh(k);
    renderTally();
  });
  b.input.addEventListener('blur', () => { b.matches = []; renderMenu(k); });
  // Coming back to unconfirmed text reopens its matches.
  b.input.addEventListener('focus', () => { if (!sel[k]) refresh(k); });
}
for (const input of [boxes.src.input, boxes.dst.input, lvlInput]) {
  input.addEventListener('focus', () => input.select());
}
lvlInput.addEventListener('input', () => { lvlNone = false; renderLevels(); renderTally(); });

// Picker buttons act without taking focus, so typing carries on in place.
for (const id of ['lvl-pick', 'lvl-all']) $(id).addEventListener('mousedown', (e) => e.preventDefault());
$('lvl-all').addEventListener('click', () => {
  const full = (parseLevels(lvlInput.value, lvlNone) || []).length === S.levels.length;
  setLevels(full ? [] : allLevels());
});

document.addEventListener('keydown', (e) => {
  if (pending) return onConfirmKey(e);
  if (!active()) return;
  const id = e.target.id;
  const k = id === 'src' || id === 'dst' ? id : null;
  const b = k && boxes[k];
  const inBox = k || id === 'lvl';

  // Mod+Z undoes the last patch, unless a box has text to undo instead.
  if ((mac ? e.metaKey : e.ctrlKey) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'z') {
    if (e.target instanceof HTMLInputElement && e.target.value) return;
    e.preventDefault();
    undoLast();
    return;
  }

  if (b && !b.menu.hidden && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
    e.preventDefault();
    const n = b.matches.length;
    b.hi = (b.hi + (e.key === 'ArrowDown' ? 1 : n - 1)) % n;
    renderMenu(k);
    return;
  }
  if (e.key === 'Tab') {
    // Leaving a box either way takes its best match, like Enter does.
    if (b && !sel[k] && b.input.value.trim() && !accept(k)) {
      e.preventDefault();
      b.input.classList.add('pb-input--invalid');
      return;
    }
    if (id === 'lvl' && !e.shiftKey) { e.preventDefault(); boxes.src.input.focus(); }
    return;
  }
  if (!inBox) return; // leave buttons and the history list to the browser
  if (e.key === 'Enter') {
    e.preventDefault();
    patchNow();
    return;
  }
  if (e.key === 'Escape') {
    if (b && !b.menu.hidden) { b.matches = []; renderMenu(k); return; }
    if (e.target.value) {
      if (k) select(k, null);
      else e.target.value = '';
      renderLevels();
      renderTally();
    } else {
      clearAll();
    }
  }
});

$('patch').addEventListener('mousedown', (e) => e.preventDefault()); // keep focus where it was
$('patch').addEventListener('click', patchNow);
$('confirm-cancel').addEventListener('click', closeConfirm);
$('confirm-ok').addEventListener('click', () => {
  if (pending && performance.now() >= readyAt) doTake();
});

// ---- subscriptions

on('hello', () => {
  lvlInput.placeholder = S.cfg.defaultLevels;
  buildPicker();
  renderLevels();
  renderTally();
  renderHistory();
});
on('names', () => {
  for (const k of ['src', 'dst']) if (document.activeElement === boxes[k].input && !sel[k]) refresh(k);
  renderTally();
});
on('tally', renderTally);
on('conn', renderConn);
on('history', renderHistory);
on('result', (r) => $('result').replaceChildren(resultNode(r)));
