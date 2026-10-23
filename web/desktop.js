// Desktop layout: three boxes, Tab between them, churning-key confirm.
// + in the destination box adds another destination as a chip.

import {
  $, el, S, on, connState, needsLink, renderDown, setDot, search, parseLevels, levelText, levelInput, defaultLevelsText, relabelLevels, levelBar, pickButtons,
  allLevels, fedBy, dstItem, dstLabel, srcItem, currentlyNodes, fmtTime, undoPlan, historyPager,
  beforeGroups, srcRow, tallyText, retryPlan, planVerb, entryNote, chevron,
  histRows, statusNode, folderTitle, folderLevels, folderIds, folderUndoPlan, folderRetryPlan, rowUndoPlan, planEmpty,
  resultNode, showResult, patchPlan, planBlocked, planSummary, sendTakes, scrollTopPref,
} from './core.js';
import { layout } from './layout.js';

const sel = { src: null, dst: null };
let dsts = []; // destinations added with +, before the one in the box
const chosen = (d) => dsts.some((x) => x.n === d.n);
const boxes = {
  src: { input: $('src'), menu: $('src-menu'), list: () => S.srcs, matches: [], hi: 0 },
  // One entry per destination: chips already added don't come up again.
  dst: { input: $('dst'), menu: $('dst-menu'), list: () => (dsts.length ? S.dsts.filter((d) => !chosen(d)) : S.dsts), matches: [], hi: 0 },
};
const lvlInput = $('lvl');
let lvlNone = false;  // picker cleared to no levels (blank box would mean default)
let feedOpen = false; // "and N others" expanded

const active = () => layout() === 'desktop' && document.documentElement.dataset.view !== 'settings';
const mac = /Mac|iPhone|iPad/.test(navigator.platform);
const expanded = new Set(); // history IDs showing their before state
const openFolders = new Set(); // history groups showing their takes
const entered = new Set();  // live entries already animated in

// ---- connection

// A small status in the corner while all is well; once a link is lost, a
// banner saying which, a red outline round the page, and no patching.
function renderConn() {
  const c = connState();
  setDot($('conn-srv-dot'), $('conn-srv-text'), [c.ws[0], 'Server']);
  setDot($('conn-rt-dot'), $('conn-rt-text'), [c.rt[0], 'Router']);
  const [label, isName] = routerLabel();
  $('conn-addr').textContent = label;
  $('conn-addr').classList.toggle('sode__conn-addr--name', isName);
  $('conn').title = [c.ws[1], `${c.rt[1]} ${S.routerAddr}`, S.router !== 'up' && S.routerLastErr].filter(Boolean).join('\n');
  document.documentElement.dataset.conn = c.down ? 'down' : 'up';
  renderDown($('conn-down'), c);
}
needsLink($('patch'));

// What the status shows for the router (Settings → Router → Status
// shows): [text, is a name]. Name falls back to the address when unset.
function routerLabel() {
  const show = S.cfg?.routerShow || 'address';
  if (show === 'none') return ['', false];
  if (show === 'name' && S.cfg.routerName) return [S.cfg.routerName, true];
  return [S.routerAddr || '', false];
}

// ---- fuzzy finder

// The dropdown scrolls: up to MAX_MATCHES results, about PAGE rows tall.
const MAX_MATCHES = 100;
const PAGE = 8;

function refresh(k) {
  const b = boxes[k];
  b.matches = search(b.list(), b.input.value, MAX_MATCHES);
  b.hi = 0;
  renderMenu(k);
  b.menu.scrollTop = 0;
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
  // A pinned count makes it clear the list scrolls.
  if (b.matches.length > PAGE) {
    const total = b.matches.total > b.matches.length ? `${b.matches.length}+` : b.matches.length;
    const foot = el('li', 'sode__menu-count', `${total} matches · ↑↓ ${mac ? '⌥J ⌥K' : 'Alt+J Alt+K'} PgUp PgDn`);
    foot.setAttribute('role', 'presentation');
    b.menu.append(foot);
  }
  b.input.setAttribute('aria-activedescendant', `${k}-opt-${b.hi}`);
}

// Moves the highlight without redrawing, so the list keeps its scroll.
function moveHi(k, to) {
  const b = boxes[k];
  const items = b.menu.querySelectorAll('[role="option"]');
  items[b.hi]?.classList.remove('active');
  items[b.hi]?.setAttribute('aria-selected', 'false');
  b.hi = to;
  items[b.hi].classList.add('active');
  items[b.hi].setAttribute('aria-selected', 'true');
  items[b.hi].scrollIntoView({ block: 'nearest' });
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
  if (k === 'dst') renderChips();
  renderTally();
}

function accept(k) {
  const it = boxes[k].matches[boxes[k].hi];
  if (!it) return false;
  select(k, it);
  return true;
}

// ---- destination chips

// Every destination the patch will go to: the chips, then the box.
const allDsts = () => (sel.dst && !chosen(sel.dst) ? [...dsts, sel.dst] : [...dsts]);

// +: the box's match becomes a chip and the box starts over for the next.
function addDst() {
  const b = boxes.dst;
  b.input.focus();
  if (!sel.dst) {
    if (!b.input.value.trim()) return;
    if (!b.matches.length) refresh('dst');
    if (!accept('dst')) { b.input.classList.add('pb-input--invalid'); return; }
  }
  if (!chosen(sel.dst)) dsts.push(sel.dst);
  select('dst', null);
}

function renderChips() {
  const box = $('dst-chips');
  box.hidden = !dsts.length;
  box.replaceChildren(...dsts.map((d, i) => {
    const x = el('button', 'pb-chip__remove', '×');
    x.type = 'button';
    x.tabIndex = -1;
    x.setAttribute('aria-label', `Remove ${d.label}`);
    x.addEventListener('mousedown', (e) => e.preventDefault());
    x.addEventListener('click', () => { dsts.splice(i, 1); renderChips(); renderTally(); });
    const chip = el('span', 'pb-chip pb-chip--static', d.label, x);
    chip.setAttribute('role', 'listitem');
    return chip;
  }));
  const n = allDsts().length;
  $('dst-label').textContent = n > 1 ? `Destinations (${n})` : 'Destination';
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
  lvlInput.placeholder = lvlNone ? 'none' : defaultLevelsText();
  for (const b of $('lvl-pick').children) b.setAttribute('aria-pressed', String(on.has(+b.dataset.ui)));
  const full = on.size === S.levels.length;
  const all = $('lvl-all');
  all.textContent = full ? 'None' : 'All';
  all.dataset.tip = full ? 'No levels' : 'All levels';
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
  const ds = allDsts();
  if (ds.length) return box.replaceChildren(...ds.flatMap(currentlyNodes));
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

  const plan = undoPlan(e);
  const btn = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Undo');
  btn.type = 'button';
  btn.setAttribute('aria-label', `Undo ${src.label} to ${dst.label}`);
  if (planEmpty(plan)) {
    btn.disabled = true;
    btn.title = 'Nothing to undo';
  } else needsLink(btn);
  btn.addEventListener('click', () => openConfirm(plan));

  // Before: what the destination had on these levels just before the take.
  const open = expanded.has(e.id);
  const detailId = `hist-before-${e.id}`;
  const toggle = disclosure(open, 'Before', detailId, () => {
    if (expanded.has(e.id)) expanded.delete(e.id);
    else expanded.add(e.id);
    renderHistory();
  });

  const row = el('li', 'pb-input sode__hist-row' + enterClass(e) + curClass(`e${e.id}`),
    el('span', 'sode__hist-id', `#${e.id}`),
    el('span', 'sode__hist-time', fmtTime(e.time)),
    levelBar(known),
    el('span', 'sode__hist-route', el('strong', null, src.label), ' → ', el('strong', null, dst.label),
      entryNote(e)),
    statusNode({ e }),
    toggle,
    el('span', 'sode__hist-actions', retryButton(e, src, dst), btn));
  row.dataset.key = `e${e.id}`;
  if (open) {
    const detail = el('div', 'sode__hist-before',
      el('div', 'sode__label', `Before #${e.id}, on ${dst.label}`),
      ...beforeGroups(e).map((g) => srcRow(g.levels, tallyText(g.src))));
    detail.id = detailId;
    row.append(detail);
  }
  return row;
}

function disclosure(open, label, controls, onToggle) {
  const b = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode__hist-toggle', chevron(open), label);
  b.type = 'button';
  b.setAttribute('aria-expanded', String(open));
  b.setAttribute('aria-controls', controls);
  b.addEventListener('click', onToggle);
  return b;
}

// A folder: one confirm's takes under a summary row with the master status,
// Undo all and (if any failed) Retry for the failed ones.
function folderRow(f) {
  const open = openFolders.has(f.group);
  const kidsId = `hist-folder-${f.group}`;
  const title = folderTitle(f);
  const failed = f.entries.filter((e) => !e.ok).length;

  const undo = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Undo all');
  undo.type = 'button';
  const plan = folderUndoPlan(f);
  if (planEmpty(plan)) {
    undo.disabled = true;
    undo.title = 'Nothing to undo';
  } else needsLink(undo);
  undo.setAttribute('aria-label', `Undo all ${f.entries.length} takes of ${folderIds(f)}`);
  undo.addEventListener('click', () => openConfirm(plan));

  const retry = el('button', 'pb-btn pb-btn--outline pb-btn--sm' + (failed ? '' : ' sode__placeholder'), 'Retry');
  retry.type = 'button';
  if (failed) {
    needsLink(retry);
    retry.setAttribute('aria-label', `Retry the ${failed} failed take${failed === 1 ? '' : 's'} of ${folderIds(f)}`);
    retry.addEventListener('click', () => openConfirm(folderRetryPlan(f)));
  } else {
    retry.tabIndex = -1;
    retry.setAttribute('aria-hidden', 'true');
  }

  const toggle = disclosure(open, `${f.entries.length}`, kidsId, () => {
    if (openFolders.has(f.group)) openFolders.delete(f.group);
    else openFolders.add(f.group);
    autoOpen.delete(f.group); // opened by hand: stays open
    renderHistory();
  });
  toggle.setAttribute('aria-label', `${open ? 'Hide' : 'Show'} the ${f.entries.length} takes`);

  const fresh = f.entries.some((e) => e.id === S.newId) && !entered.has(f.group);
  if (fresh) entered.add(f.group);
  // A closed folder has used up the arrival; don't replay it on opening.
  if (!open && f.entries.some((e) => e.id === S.newId)) entered.add(S.newId);
  const row = el('li', 'pb-input sode__hist-row' + (fresh && !open ? ' sode--enter' : '') + curClass(`f${f.group}`),
    el('span', 'sode__hist-id', folderIds(f)),
    el('span', 'sode__hist-time', fmtTime(f.entries[0].time)),
    levelBar(folderLevels(f)),
    el('span', 'sode__hist-route', ...title),
    statusNode({ f }),
    toggle,
    el('span', 'sode__hist-actions', retry, undo));
  row.dataset.key = `f${f.group}`;
  if (open) {
    const kids = el('ul', 'sode__hist-folder', ...f.entries.map(histRow));
    kids.id = kidsId;
    row.append(kids);
  }
  return row;
}

// Marks the keyboard cursor's row.
const curClass = (key) => (key === cursor ? ' sode__hist-row--cur' : '');

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
  needsLink(b);
  b.setAttribute('aria-label', `Retry ${src.label} to ${dst.label}`);
  b.addEventListener('click', () => openConfirm(retryPlan(e)));
  return b;
}

const pager = historyPager(10);

function renderHistory() {
  $('history-head').hidden = !S.hist.length;
  const list = pager.list();
  if (cursor && !stops(list).some((x) => x.key === cursor)) {
    // Its row left the page (newer patches pushed it off): drop the
    // cursor and close the folders it had opened.
    cursor = null;
    for (const g of autoOpen) openFolders.delete(g);
    autoOpen.clear();
  }
  histList.replaceChildren(...list.map((r) => (r.f ? folderRow(r.f) : histRow(r.e))));
  $('history-more').replaceChildren(...[pager.button()].filter(Boolean));
  if (keyMode()) histList.querySelector('.sode__hist-row--cur')?.scrollIntoView({ block: 'nearest' });
}

// ---- keyboard mode: focus on the history list, where single keys act.
// The cursor is 'e<id>' on a take or 'f<group>' on a folder. Folders open
// as the cursor enters them and close as it leaves, unless opened by hand.
// Entered from the keyboard only (Esc, or a letter from the page); any
// click leaves it, so clicking in Recent doesn't arm single-key actions.

let cursor = null;
const autoOpen = new Set();
const histList = $('history');
let keysOn = false;
const keyMode = () => keysOn && document.activeElement === histList;
document.addEventListener('pointerdown', (e) => {
  keysOn = false;
  histList.classList.remove('sode__hist--keys');
  // Clicking a row moves the (hidden) cursor there, so a key from here
  // starts on the row clicked, not one left from earlier.
  const row = histList.contains(e.target) && e.target.closest('[data-key]');
  if (row) cursor = row.dataset.key;
}, true);

// Every row the cursor can stop on, top to bottom. top is the index of
// the row (or the folder holding it) in the pager's list.
function stops(list = pager.list()) {
  const out = [];
  list.forEach((r, top) => {
    if (r.e) return out.push({ key: `e${r.e.id}`, e: r.e, top });
    out.push({ key: `f${r.f.group}`, f: r.f, top });
    if (openFolders.has(r.f.group)) for (const e of r.f.entries) out.push({ key: `e${e.id}`, e, top, in: r.f });
  });
  return out;
}
const curStop = () => stops().find((x) => x.key === cursor);

// Moves the cursor. Opening or closing a folder redraws the list; a plain
// move just shifts the highlight between two rows.
function moveTo(s) {
  const group = s.f?.group ?? s.in?.group;
  let redraw = false;
  for (const g of autoOpen) {
    if (g !== group) { openFolders.delete(g); autoOpen.delete(g); redraw = true; }
  }
  if (s.f && !openFolders.has(s.f.group)) { openFolders.add(s.f.group); autoOpen.add(s.f.group); redraw = true; }
  const row = (key) => key && histList.querySelector(`[data-key="${CSS.escape(key)}"]`);
  const from = row(cursor), to = row(s.key);
  cursor = s.key;
  if (redraw || !to) return renderHistory();
  from?.classList.remove('sode__hist-row--cur');
  to.classList.add('sode__hist-row--cur');
  if (keyMode()) to.scrollIntoView({ block: 'nearest' });
}

// Past the last loaded row, load more; the next press moves on to it.
function loadMore() {
  if (pager.hasMore() && !S.histLoading) pager.more();
}

// j/k: the next or previous row, walking into open folders.
function step(d) {
  const list = stops();
  const i = list.findIndex((x) => x.key === cursor);
  if (i < 0) return list.length && moveTo(list[d > 0 ? 0 : list.length - 1]);
  const next = list[i + d];
  if (!next) return d > 0 && loadMore();
  moveTo(next);
  // Coming up into a closed folder lands on its last take, mirroring
  // the way down.
  if (d < 0 && next.f && list[i].in?.group !== next.f.group && autoOpen.has(next.f.group)) {
    const last = next.f.entries[next.f.entries.length - 1];
    moveTo(stops().find((x) => x.key === `e${last.id}`));
  }
}

// J/K: the next or previous top-level row (a take or a whole folder).
function jump(d) {
  const tops = stops().filter((x) => !x.in);
  const s = curStop();
  const next = s ? tops.find((x) => x.top === s.top + d) : tops[0];
  if (next) moveTo(next);
  else if (d > 0) loadMore();
}

// l: open a take's Before or a folder. h: close it, or go up to the folder.
function open(on) {
  const s = curStop();
  if (!s) return;
  if (s.e) {
    if (on) expanded.add(s.e.id);
    else if (expanded.has(s.e.id)) expanded.delete(s.e.id);
    else if (s.in) return moveTo(stops().find((x) => x.key === `f${s.in.group}`));
  } else if (on) {
    openFolders.add(s.f.group);
    autoOpen.delete(s.f.group);
  } else {
    openFolders.delete(s.f.group);
    autoOpen.delete(s.f.group);
  }
  renderHistory();
}

function retryCur() {
  const s = curStop();
  if (!s) return;
  if (s.f && s.f.entries.some((e) => !e.ok)) openConfirm(folderRetryPlan(s.f));
  else if (s.e && !s.e.ok) openConfirm(retryPlan(s.e));
  else showResult('info', 'Nothing to retry', 'Only failed takes can be retried.');
}

// key: put the cursor on that row (Esc from one of its buttons).
function enterKeys(key) {
  keysOn = true;
  histList.classList.add('sode__hist--keys');
  histList.focus();
  const list = stops();
  // moveTo also closes folders opened for a cursor that's since moved.
  const to = (key && list.find((x) => x.key === key)) || list.find((x) => x.key === cursor) || list[0];
  if (to) moveTo(to);
  else renderHistory();
}

const BOXES = { s: 'src', d: 'dst', f: 'lvl' };

// Keys while the history list (or nothing) has focus. True when handled.
function onListKey(e) {
  if (e.ctrlKey || e.metaKey || e.altKey) return false;
  const k = e.key;
  if (BOXES[k]) { focusBox($(BOXES[k])); return true; }
  if (k === ',') { $('settings-open').click(); return true; }
  if (k === '?') { openKeys(); return true; }
  if (k === 'Escape') { focusBox(boxes.src.input); return true; }
  const act = {
    j: () => step(1), ArrowDown: () => (e.shiftKey ? jump(1) : step(1)),
    k: () => step(-1), ArrowUp: () => (e.shiftKey ? jump(-1) : step(-1)),
    J: () => jump(1), K: () => jump(-1),
    l: () => open(true), ArrowRight: () => open(true), Enter: () => open(!e.shiftKey),
    h: () => open(false), ArrowLeft: () => open(false),
    u: () => { const s = curStop(); if (s) undoRow(s.f ? { f: s.f } : { e: s.e }); },
    r: retryCur,
  }[k];
  if (!act) return false;
  // From the page itself, arrows keep scrolling it; only letters start
  // keyboard mode there.
  if (!keyMode() && k.startsWith('Arrow')) return false;
  if (!keyMode()) enterKeys();
  else act();
  return true;
}

// ---- shortcut tip: a note in the corner until dismissed (per browser)

const TIP_KEY = 'sode-hint-keys';
$('keys-tip-text').textContent = `${mac ? '⌘Z' : 'Ctrl+Z'} undoes the last · ${mac ? '⌃?' : 'Ctrl+?'} for shortcuts`;
try { $('keys-tip').hidden = localStorage.getItem(TIP_KEY) === 'hidden'; } catch (e) { $('keys-tip').hidden = false; }
$('keys-tip-close').addEventListener('click', () => {
  $('keys-tip').hidden = true;
  try { localStorage.setItem(TIP_KEY, 'hidden'); } catch (e) {}
});

// ---- shortcut list (?)

// Closing from the keyboard returns to keyboard mode if it was on when the
// list opened; closing with the mouse never does (a click leaves it).
let keysBack = null;
let keysWere = false;
function openKeys() {
  keysBack = document.activeElement;
  keysWere = keyMode();
  $('keys').hidden = false;
  $('keys-dialog').focus();
}
function closeKeys(viaKeys = false) {
  $('keys').hidden = true;
  if (keysWere && viaKeys) enterKeys();
  else keysBack?.focus();
}
if (mac) for (const k of $('keys').querySelectorAll('[data-mac]')) k.textContent = k.dataset.mac;
$('keys-close').addEventListener('click', () => closeKeys());

// ---- confirm

const KEYS = 'abcdefghjkmnpqrstuvwxyz';
let lastKey = '';
let pending = null; // plan + key
let readyAt = 0;
let confirmBack = null; // focus to return to when the confirm closes
let confirmKeys = false; // keyboard mode was on when it opened

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

// Undo the newest history row (a whole folder), through the usual confirm.
function undoLast() {
  const [row] = histRows(S.hist);
  if (!row) return showResult('info', 'Nothing to undo', 'No patches yet.');
  undoRow(row);
}

// Undo a history row: { e } for a take, { f } for a whole folder.
function undoRow(row) {
  const plan = rowUndoPlan(row);
  if (planEmpty(plan)) {
    const what = row.f ? folderIds(row.f) : `#${row.e.id}`;
    return showResult('warn', 'Nothing to undo', `${what} had nothing routed before it, so there is nothing to restore.`);
  }
  openConfirm(plan);
}

function tryConfirm() {
  if (!S.cfg) return;
  for (const k of ['src', 'dst']) {
    // The destination box may be empty when chips are there, but text
    // that matched nothing still stops the patch.
    const b = boxes[k];
    if (!sel[k] && (k === 'src' || !dsts.length || b.input.value.trim())) {
      b.input.classList.add('pb-input--invalid');
      b.input.focus();
      return;
    }
  }
  const levels = parseLevels(lvlInput.value, lvlNone);
  if (!levels) { lvlInput.classList.add('pb-input--invalid'); lvlInput.focus(); return; }
  openConfirm(patchPlan(sel.src, allDsts(), levels));
}

function openConfirm(plan) {
  const blocked = planBlocked(plan);
  if (blocked) return showResult(blocked.kind, blocked.title, blocked.text);
  let key;
  do key = KEYS[Math.floor(Math.random() * KEYS.length)]; while (key === lastKey);
  lastKey = key;
  pending = { ...plan, key };
  confirmBack = document.activeElement;
  confirmKeys = keyMode();

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

// Focus goes back where the confirm was opened from. Keyboard mode only if
// it was on and the confirm was answered from the keyboard (a click leaves
// it). From Recent otherwise (a row's button, or the list on browsers
// that don't focus clicked buttons) it goes to Source, leaving the boxes
// as they are: the row is redrawn with the result.
function focusBack(viaKeys) {
  if (confirmKeys && viaKeys) enterKeys();
  else if (confirmBack === histList || histArea(confirmBack)) focusBox(boxes.src.input);
  else if (confirmBack?.isConnected) confirmBack.focus();
}
const histArea = (n) => !!n && (histList.contains(n) || $('history-more').contains(n));

function closeConfirm(viaKeys = false) {
  const patch = pending?.verb === 'patch';
  pending = null;
  $('confirm').hidden = true;
  if (patch) lvlInput.focus();
  else focusBack(viaKeys);
}

function doTake(viaKeys = false) {
  const p = pending;
  pending = null;
  $('confirm').hidden = true;
  // Only a patch came from the boxes; an undo or retry leaves them be.
  if (sendTakes(p) && p.verb === 'patch') clearAll();
  else focusBack(viaKeys);
}

function onConfirmKey(e) {
  e.preventDefault();
  e.stopPropagation();
  if (e.key === 'Escape') return closeConfirm(true);
  if (e.repeat || e.ctrlKey || e.metaKey || e.altKey) return;
  if (performance.now() < readyAt) return;
  if (e.key.toLowerCase() === pending.key || (S.cfg.confirmEnter && e.key === 'Enter')) doTake(true);
}

// ---- keyboard & input wiring

function clearAll() {
  dsts = [];
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
// Focusing a box brings the page back to the top (Settings → This device),
// so it isn't left off-screen after scrolling down Recent.
const calm = matchMedia('(prefers-reduced-motion: reduce)');
// From down the page: skip the browser's own jump to the box so the scroll
// to the top animates the whole way.
function focusBox(input) {
  input.focus({ preventScroll: scrollTopPref() });
}
for (const input of [boxes.src.input, boxes.dst.input, lvlInput]) {
  input.addEventListener('focus', () => {
    input.select();
    if (active() && scrollTopPref() && window.scrollY > 0) {
      window.scrollTo({ top: 0, behavior: calm.matches ? 'auto' : 'smooth' });
    }
  });
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
  // Ctrl+? (Ctrl+Shift+/) opens or closes the shortcut list from anywhere,
  // even mid-typing. ⌘? works too where the OS lets it through (macOS keeps
  // it for the Help menu, hence ⌃? on the tip). Matched by e.code: Shift
  // changes e.key.
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && !e.altKey && e.code === 'Slash') {
    e.preventDefault();
    if ($('keys').hidden) openKeys();
    else closeKeys(true);
    return;
  }
  if (!$('keys').hidden) {
    if (e.key === 'Escape' || e.key === '?') { e.preventDefault(); closeKeys(true); }
    // Keep focus in the dialog: Close is its only control.
    if (e.key === 'Tab') { e.preventDefault(); $('keys-close').focus(); }
    return;
  }
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

  // Option/Alt+J/K move like ↓/↑. Matched by e.code because Option
  // changes e.key on macOS.
  const altJK = e.altKey && !e.ctrlKey && !e.metaKey && !e.shiftKey && { KeyJ: 'ArrowDown', KeyK: 'ArrowUp' }[e.code];
  const nav = altJK || e.key;
  if (b && altJK && b.menu.hidden) { e.preventDefault(); return; } // not ∆ or ˚ in the box
  if (b && !b.menu.hidden && ['ArrowDown', 'ArrowUp', 'PageDown', 'PageUp'].includes(nav)) {
    e.preventDefault();
    const n = b.matches.length;
    if (nav === 'ArrowDown') moveHi(k, (b.hi + 1) % n);
    else if (nav === 'ArrowUp') moveHi(k, (b.hi + n - 1) % n);
    else if (e.key === 'PageDown') moveHi(k, Math.min(n - 1, b.hi + PAGE));
    else moveHi(k, Math.max(0, b.hi - PAGE));
    return;
  }
  // + adds another destination; Ctrl/Cmd + is left for zoom.
  if (k === 'dst' && e.key === '+' && !e.ctrlKey && !e.metaKey) {
    e.preventDefault();
    addDst();
    return;
  }
  // Backspace in an empty destination box takes the last chip back to edit.
  if (k === 'dst' && e.key === 'Backspace' && !b.input.value && dsts.length) {
    e.preventDefault();
    select('dst', dsts.pop());
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
  if (e.target === histList || e.target === document.body) {
    if (onListKey(e)) e.preventDefault();
    return;
  }
  // Esc from a button in Recent: keyboard mode, on that button's row.
  if (e.key === 'Escape' && e.target !== histList && histArea(e.target)) {
    e.preventDefault();
    enterKeys(e.target.closest('[data-key]')?.dataset.key);
    return;
  }
  if (!inBox) return; // leave buttons to the browser
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
    } else if (!dsts.length && !boxes.src.input.value && !boxes.dst.input.value && !lvlInput.value && !lvlNone) {
      enterKeys(); // Esc with everything empty: keyboard mode
    } else {
      clearAll();
    }
  }
});

$('dst-add').addEventListener('mousedown', (e) => e.preventDefault());
$('dst-add').addEventListener('click', addDst);
$('patch').addEventListener('mousedown', (e) => e.preventDefault()); // keep focus where it was
$('patch').addEventListener('click', patchNow);
$('confirm-cancel').addEventListener('click', () => closeConfirm());
$('confirm-ok').addEventListener('click', () => {
  if (pending && performance.now() >= readyAt) doTake();
});

// ---- subscriptions

on('hello', () => {
  pager.reset();
  lvlInput.value = relabelLevels(lvlInput.value);
  lvlInput.placeholder = defaultLevelsText();
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
