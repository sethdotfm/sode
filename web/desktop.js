import { $, el, S, on, needsLink, renderDown, search, dstLabel, dstItem, srcItem, labelNode, routerData, routerOfItem, routerOfText, sameEnd } from './core.js';
import { keyLabel } from './keys.js';
import { searchText, prefixOf } from './routerprefix.js';
import { parseList, namedAs, listMatches, missingNote, nextAdd, LIST_MAX } from './boxlist.js';
import { paintConn } from './connview.js';
import { levelBar, allLevels, levelSource, levelsShown } from './levelview.js';
import { levelBox } from './levelbox.js';
import { levelRouterOf } from './levels.js';
import { fedBy, bySource, currentlyNodes, pickEnd } from './routes.js';
import { scrollTopPref, flipped, setFlip } from './device.js';
import {
  resultNode,
  showResult,
  patchPlan,
  planBlocked,
  rightsOff,
  sendTakes,
  lastTake,
  nextTake,
  nextFind,
  boxPatch,
  lastChip,
  VERBS,
  planVerb,
  planSummary,
} from './plans.js';
import { layout } from './layout.js';
import { presetHost } from './presets.js';
import { lastFill } from './historyrows.js';
import { presetAdd } from './presetadd.js';
import { keyConfirm } from './keyconfirm.js';
import { showAll } from './showall.js';
import { shortcutList } from './shortcuts.js';
import { combobox } from './widgets.js';
import { desktopHistory } from './desktophistory.js';
import { desktopKeys } from './desktopkeys.js';
import { compactCentre } from './compactcentre.js';
import { gridShown, toggleGrid, stagedCount, takeStaged, dropStaged, takeRoutes, paintTake } from './gridpopup.js';
import { takeWord } from './gridpanelphone.js';
import { protectKey } from './protects.js';

const sel = { src: null, dst: null };
let dsts = [];
const chosen = (d) => dsts.some((x) => sameEnd(x, d));
const MAX_MATCHES = 100;
// list(rt) is the box's list on router rt (routerData); with none, the router that the box's text names.
const boxes = {
  src: { input: $('src'), list: (rt = textRouter('src')) => rt.srcs },
  dst: { input: $('dst'), list: (rt = textRouter('dst')) => (dsts.length ? rt.dsts.filter((d) => !chosen(d)) : rt.dsts) },
};
// A box searches the router whose prefix its text starts with (B.12), else the first router.
const textRouter = (k) => routerData(routerOfText(boxes[k].input.value));
for (const k of ['src', 'dst']) {
  const b = boxes[k];
  b.cb = combobox(b.input, $(`${k}-menu`), {
    source: (text) => matchesOf(k, text),
    onPick: (it) => select(k, it),
    add: k === 'dst' ? (it) => addDst(it) : undefined,
    addTip: 'Add as a destination',
    onPickAll: k === 'dst' ? (items) => addAll(items) : undefined,
    foot: $(`${k}-count`),
    onRender: () => b.all?.sync(),
    outside: (it) => rightsOff(k === 'src' ? 'sources' : 'dsts', it),
  });
}
// A typed list ("11-13", "Rec 1-4", boxlist.js) in the Destination box shows its destinations all selected, so Enter,
// Tab, + and Ctrl+Enter add exactly what shows. A text that starts a name is a search. The Source box takes one source:
// a list there gets a note that points to the Grid.
function matchesOf(k, text) {
  const rt = routerData(routerOfText(text));
  const all = k === 'src' ? rt.srcs : rt.dsts;
  // A router after the first: its own list, searched with the text after the prefix.
  const q = rt.first ? searchText(text, S.prefix, [], [S.prefix]) : prefixOf(text).text;
  const list = q && parseList(q);
  if (!list || namedAs(q, all)) return search(boxes[k].list(rt), rt.first ? text : q, MAX_MATCHES);
  if (k === 'src') return Object.assign([], { note: `One source only. For many routes, use the Grid (${keyLabel('ctrl+e')}).` });
  if (list.tooMany) return Object.assign([], { note: `A list can have ${LIST_MAX} destinations or fewer.` });
  const { found, missing } = listMatches(list, rt.dsts);
  const rows = found.filter((d) => !chosen(d));
  const note = missingNote(missing) || (found.length && !rows.length ? 'These destinations are added.' : '');
  return Object.assign(rows, { all: true, note });
}

// Typed levels are levels of the router of the route's destination. That is the router of a chosen destination, else
// the router that the Destination box's text names, else the router of the source. The picker follows it (followLevels).
const lvlRouter = () => levelRouterOf({ dsts: sel.dst ? [sel.dst, ...dsts] : dsts, dstText: boxes.dst.input.value, src: sel.src }, routerOfItem, routerOfText);
let lvlAt = '';
const lvlInput = $('lvl');
const lvl = levelBox(lvlInput, $('lvl-pick'), $('lvl-all'), () => levelSource(lvlRouter()), { onChange: () => renderTally() });
function followLevels() {
  if (lvlRouter() === lvlAt) return;
  lvlAt = lvlRouter();
  lvl.repick();
}
// With the Levels box hidden, a move to Levels goes to the last box before Take, where Enter takes.
function toLevels() {
  focusBox(levelsShown() ? lvlInput : flipped() ? boxes.src.input : boxes.dst.input);
}

const active = () =>
  layout() === 'desktop' && document.documentElement.dataset.view !== 'settings' && !document.documentElement.hasAttribute('data-sode-setup');

function renderConn() {
  const c = paintConn({
    root: $('conn'),
    srvDot: $('conn-srv-dot'),
    srvText: $('conn-srv-text'),
    routers: $('conn-routers'),
    addr: $('conn-addr'),
  });
  document.documentElement.dataset.conn = c.down ? 'down' : 'up';
  renderDown($('conn-down'), c);
}
needsLink($('patch'));

function select(k, it) {
  sel[k] = it;
  if (k === 'dst') lastAdd = null;
  const b = boxes[k];
  b.input.value = it ? it.label : '';
  b.input.classList.remove('pb-input--invalid');
  b.cb.close();
  if (k === 'dst') renderChips();
  renderTally();
}

const allDsts = () => (sel.dst && !chosen(sel.dst) ? [...dsts, sel.dst] : [...dsts]);

// After an add the box is empty, so the next digits start a new search. The last add arms the next add (of a list, the
// one with the highest router number). On the empty box, +, Ctrl+Enter and the + button add nextAdd's destination. The
// box shows it (paintArmed). It uses the Next match behavior setting, as the Next chip does. Typing, any pick, a take and
// Clear end it. With no add directly before, an add on the empty box does nothing.
let lastAdd = null;
const armed = () => {
  if (lastAdd === null || sel.dst || boxes.dst.input.value) return null;
  const all = routerData(lastAdd.router).dsts;
  return nextAdd(lastAdd, all, chosen, nextFind(all));
};

function addDst(pick) {
  const b = boxes.dst;
  b.input.focus();
  // A row moved to in the browse list counts though the box holds no text.
  if (!pick && b.cb.moved) pick = b.cb.current();
  if (!pick && !sel.dst && !b.input.value.trim()) {
    pick = armed();
    if (!pick) return;
  }
  if (pick) return addAll([pick]);
  if (sel.dst) return addAll([sel.dst]);
  if (!b.cb.open) b.cb.refresh();
  // A typed list adds all its destinations through onPickAll; a match comes back as sel.dst.
  if (!b.cb.accept()) b.input.classList.add('pb-input--invalid');
  else if (sel.dst) addAll([sel.dst]);
}

function addAll(items) {
  const b = boxes.dst;
  for (const d of items) if (!chosen(d)) dsts.push(d);
  sel.dst = null;
  b.input.value = '';
  b.input.classList.remove('pb-input--invalid');
  b.cb.close();
  lastAdd = items.reduce((a, d) => (!a || d.n > a.n ? d : a), null);
  renderChips();
  renderTally();
}

// The armed next add shows in the empty box as a destination chip, laid over the input (CSS hides it once there is text).
// A placeholder cannot hold a chip, so it stays one space, which keeps :placeholder-shown; aria-description says the add.
let armedAt = null;
function paintArmed() {
  const d = armed();
  if (d === armedAt) return;
  armedAt = d;
  const slot = $('dst-armed');
  slot.hidden = !d;
  const key = keyLabel('mod+enter');
  slot.replaceChildren(...(d ? [el('span', null, `${key} adds:`), el('span', 'sode__end sode__end--dst', labelNode(d))] : []));
  if (d) boxes.dst.input.setAttribute('aria-description', `${key} adds ${d.label}`);
  else boxes.dst.input.removeAttribute('aria-description');
}

// Ctrl-click on a destination elsewhere on the page: it joins the chips, with the box's own pick.
function addEnd(it) {
  lastAdd = null;
  const have = allDsts();
  dsts = have.some((x) => sameEnd(x, it)) ? have : [...have, it];
  if (sel.dst) {
    sel.dst = null;
    boxes.dst.input.value = '';
  }
  boxes.dst.cb.close();
  renderChips();
  renderTally();
}

function renderChips() {
  const box = $('dst-chips');
  box.hidden = !dsts.length;
  box.replaceChildren(
    ...dsts.map((d, i) => {
      const x = el('button', 'pb-chip__remove', '×');
      x.type = 'button';
      x.tabIndex = -1;
      x.setAttribute('aria-label', `Remove ${d.label}`);
      x.addEventListener('mousedown', (e) => e.preventDefault());
      x.addEventListener('click', () => {
        dsts.splice(i, 1);
        lastAdd = null;
        renderChips();
        renderTally();
      });
      const chip = el('span', 'sode__end sode__end--dst sode__dst-chip', labelNode(d), x);
      chip.setAttribute('role', 'listitem');
      return chip;
    }),
  );
  const n = allDsts().length;
  $('dst-label').textContent = n > 1 ? `Destinations (${n})` : 'Destination';
}

// Last chip does what Ctrl+S/D/F do; the Next chip (Ctrl+Shift+S/D) opens over it on hover,
// focus or Shift (data-shift). Hidden when the box has it, and for default levels.
const boxState = () => ({ src: sel.src, dsts: allDsts(), levels: lvl.levels() || [] });

function renderLast() {
  if (!S.cfg) return;
  const state = boxState();
  const last = lastChip(lastTake(), state);
  const next = lastChip(nextTake(state), state);
  for (const k of ['src', 'dst', 'lvl']) {
    const slot = document.querySelector(`[data-last="${k}"]`);
    const key = { src: 's', dst: 'd', lvl: 'f' }[k];
    const what = { src: 'source', dst: 'destination', lvl: 'levels' }[k];
    const chips = [];
    if (last[k]) {
      const b = el(
        'button',
        'pb-chip sode__last',
        el('span', 'sode__tag', 'Last'),
        el('span', 'sode__last-text', ...last[k]),
        el('kbd', 'sode__key-note', keyLabel(`ctrl+${key}`)),
      );
      b.dataset.pbTip = `Last ${what} (${keyLabel(`ctrl+${key}`)})`;
      b.addEventListener('click', () => useLast(k));
      chips.push(b);
    }
    if (k !== 'lvl' && next[k]) {
      const b = el(
        'button',
        'pb-chip sode__last sode__next',
        el('span', 'sode__tag', 'Next'),
        el('kbd', 'sode__key-note sode__tag-key', keyLabel(`ctrl+shift+${key}`)),
        el(
          'span',
          'sode__next-open',
          el('span', 'sode__next-in', el('span', 'sode__last-text', ...next[k]), el('kbd', 'sode__key-note', keyLabel(`ctrl+shift+${key}`))),
        ),
      );
      b.dataset.pbTip = `Next ${what} (${keyLabel(`ctrl+shift+${key}`)})`;
      b.addEventListener('click', () => useLast(k, true));
      // The closed chip's room stays in the slot so nothing in the row moves.
      chips.push(
        el('span', 'sode__next-slot', el('span', 'pb-chip sode__last sode__next-size', 'Next', el('kbd', 'sode__key-note', keyLabel(`ctrl+shift+${key}`))), b),
      );
    }
    for (const b of chips.flatMap((c) => (c.matches('button') ? [c] : [...c.querySelectorAll('button')]))) {
      b.type = 'button';
      b.tabIndex = -1;
      b.addEventListener('mousedown', (e) => e.preventDefault());
    }
    slot.replaceChildren(...chips);
  }
}
// A Next match behavior change comes as history (device.js), so the next add's hint changes with it.
on('history', () => {
  renderLast();
  paintArmed();
});
on('last', renderLast);

// Fills one box from the last take, then focuses the opposite box only if empty, else Levels.
// next: use nextTake() and remember it so a repeat goes on.
function useLast(k, next = false) {
  const fill = lastFill(k, next, boxState());
  if (fill?.none) return showResult('info', 'No earlier take', '');
  if (!fill) return;
  const t = fill.t;
  if (k === 'src') {
    select('src', t.src);
    const dstEmpty = !sel.dst && !dsts.length && !boxes.dst.input.value.trim();
    if (dstEmpty) focusBox(boxes.dst.input);
    else toLevels();
  } else if (k === 'dst') {
    const have = allDsts();
    const add = t.dsts.filter((d) => !have.some((x) => sameEnd(x, d)));
    if (add.length) {
      dsts = [...have, ...add.slice(0, -1)];
      select('dst', add[add.length - 1]);
    }
    const srcEmpty = !sel.src && !boxes.src.input.value.trim();
    if (srcEmpty) focusBox(boxes.src.input);
    else toLevels();
  } else if (levelsShown()) {
    lvl.set(t.levels);
    focusBox(lvlInput);
  }
}

// Both lists show at the same time, on their own row (the flip swaps them).
function underHead(text, up) {
  return el('div', 'sode__under-head', el('span', up ? 'sode__down-arrow sode__down-arrow--up' : 'sode__down-arrow'), el('span', 'sode__label', text));
}

function sendingRow(src, d, levels, uid) {
  const t = routerData(uid).tally.get(d);
  const pick = pickEnd('dst', dstLabel(d, uid), 'Add as a destination', () => {
    const have = allDsts();
    const it = dstItem(d, '', uid);
    if (!have.some((x) => sameEnd(x, it))) {
      dsts = have;
      select('dst', it);
    }
    toLevels();
  });
  return el(
    'div',
    'sode__route',
    levelBar(
      levels.filter((u) => t.get(u) === src),
      uid,
    ),
    pick,
  );
}

// The route view of one destination: no source, one destination. It offers Protect and Free Protect.
const routeView = () => !sel.src && !boxes.src.input.value.trim() && allDsts().length === 1;
const protectFromKeys = (set) => routeView() && protectKey(allDsts()[0], set, openConfirm);

function pickSrc(n, uid = '') {
  select('src', srcItem(n, '', uid));
  toLevels();
}

function renderTally() {
  followLevels();
  renderLast();
  paintTakes();
  paintArmed();
  const srcUnder = $('src-under'),
    dstUnder = $('dst-under');
  if (!S.cfg) {
    srcUnder.replaceChildren();
    return dstUnder.replaceChildren();
  }
  const scrolled = [srcUnder, dstUnder].map((u) => u.querySelector('.sode__scroll')?.scrollTop || 0);
  if (sel.src) {
    const uid = sel.src.router || '';
    const levels = lvl.levels() || allLevels(uid);
    const fed = fedBy(sel.src.n, levels, uid);
    srcUnder.replaceChildren(
      underHead(fed.length ? `Sending to ${fed.length}` : 'Sending'),
      fed.length
        ? el('div', 'sode__scroll', ...fed.map((d) => sendingRow(sel.src.n, d, levels, uid)))
        : el('div', 'sode__hint', 'Not sending to any destination on these levels'),
    );
  } else srcUnder.replaceChildren();
  const ds = allDsts();
  if (ds.length) {
    // One source number on two routers is two sources.
    const srcs = new Set(
      ds.flatMap((d) =>
        bySource(d.n, allLevels(), d.router)
          .filter((r) => r.src != null && r.src >= 0)
          .map((r) => `${d.router || ''}.${r.src}`),
      ),
    );
    dstUnder.replaceChildren(
      underHead(srcs.size ? `Receiving from ${srcs.size}` : 'Receiving', true),
      el('div', 'sode__scroll', ...ds.flatMap((d) => currentlyNodes(d, pickSrc, routeView() ? openConfirm : null))),
    );
  } else dstUnder.replaceChildren();
  [srcUnder, dstUnder].forEach((u, i) => {
    const box = u.querySelector('.sode__scroll');
    if (box) box.scrollTop = scrolled[i];
  });
}

const historyView = desktopHistory({
  select,
  addEnd,
  focusBox,
  openConfirm,
  focusSearch,
  openKeys: () => keys.open(),
  boxes,
  active,
  protect: (set) => protectFromKeys(set),
});
const { renderHistory, enterKeys, keyMode, histList, histArea, pager, useEnd } = historyView;

// Per browser; index.html applies it before first paint.

function showFlip() {
  const on = flipped();
  $('flip').setAttribute('aria-pressed', String(on));
  $('flip').dataset.pbTip = on ? 'Show Source first' : 'Show Destination first';
}
$('flip').addEventListener('mousedown', (e) => e.preventDefault());
$('flip').addEventListener('click', () => setFlip(!flipped()));
on('flip', showFlip);
showFlip();

const keys = shortcutList({ keyMode, fromHistory: histArea, enterKeys, focusFirst: () => focusBox(boxes.src.input) });

function focusSearch(k) {
  const f = $('history-search').querySelector(`[data-search-${k}]`);
  if ($('history-search').hidden) return;
  f.focus();
  f.select();
}

let confirmBack = null;
let confirmKeys = false;
let confirmFromHist = false; // decided at open: rows get redrawn

const confirm = keyConfirm({
  verb: planVerb,
  summary: planSummary,
  enter: () => !!S.cfg?.confirmEnter,
  spaceHint: true,
  prompt: (key) => ['Press ', key, S.cfg.confirmEnter ? ' or Enter' : '', ' to confirm, Esc to cancel.'],
  onTake: (p, viaKeys) => {
    // A One Take of the staged routes with the boxes' route (gridpopup.js) clears the boxes as a patch does.
    if (sendTakes(p) && (VERBS[p.verb].resets || p.resets)) clearAll();
    else focusBack(viaKeys);
  },
  onClose: (p, viaKeys) => {
    if (p?.verb === 'patch') toLevels();
    else focusBack(viaKeys);
  },
});

function wantsMatch(k) {
  const b = boxes[k];
  return b.cb.moved || (!sel[k] && !!b.input.value.trim());
}

const boxesEmpty = () => !sel.src && !sel.dst && !dsts.length && !boxes.src.input.value.trim() && !boxes.dst.input.value.trim();

// The route the boxes hold, as tryConfirm would send it, or null. It marks nothing and moves no focus.
function boxRoute() {
  const ds = allDsts();
  const dstText = !sel.dst && boxes.dst.input.value.trim();
  const levels = lvl.levels();
  return sel.src && ds.length && !dstText && levels ? { src: sel.src, dsts: ds, levels } : null;
}

// One Take: with staged routes, Take sends them and the boxes' route in one Confirm (gridpopup.js). Boxes with only
// part of a route stay out of the take, because one box can find an item on the grid (Ctrl+e). An Enter that picks a
// match only picks it, as with nothing staged. It goes on to the empty box and opens no Confirm. While a preset adds
// patches (presetadd.js) it adds the boxes' patch, sends nothing and gives { ok, text }. focus false keeps focus where
// it is (the Grid popup's Take).
function patchNow(focus = true) {
  const picked = pickTyped();
  if (presetAdd.active()) {
    const r = presetAdd.addPatch(boxPatch(sel.src, allDsts(), lvl));
    if (r.ok) clearAll(focus);
    return r;
  }
  if (stagedCount() && (!picked || boxRoute()) && takeStaged()) return null;
  tryConfirm();
  return null;
}

// Typed text not yet picked takes its best match, as Enter picks it: before a Take, and before Ctrl+e finds the box on
// the grid. True when it picked one. With no match the box keeps its text.
function pickTyped() {
  let picked = false;
  for (const k of ['src', 'dst']) {
    const b = boxes[k];
    if (wantsMatch(k)) {
      if (!b.cb.open) b.cb.refresh();
      picked = !!b.cb.accept() || picked;
    }
  }
  return picked;
}

// The one Clear (#clear-all) and Esc: the boxes and the staged routes.
const nothingToClear = () => boxesEmpty() && lvl.empty() && !stagedCount();
function clearEverything(focus = true) {
  dropStaged();
  clearAll(focus);
}
// Take counts what it sends when that is more than one route (One Take). A plain patch says Take. While a preset adds
// patches it is Add in the add colour. disabled is for the Grid popup's Take: the page's own stays live, so a press with
// empty boxes marks the box to fill.
function takeState() {
  const adding = presetAdd.active();
  const box = boxRoute();
  return { word: adding ? 'Add' : takeWord(takeRoutes(box)), ok: adding, disabled: !adding && !box && !stagedCount() };
}
function paintTakes() {
  $('clear-all').disabled = nothingToClear();
  const t = takeState();
  if ($('patch').textContent !== t.word) $('patch').textContent = t.word;
  $('patch').classList.toggle('pb-btn--commit', !t.ok);
  $('patch').classList.toggle('sode-btn--primary', t.ok);
  paintTake();
}

function tryConfirm() {
  if (!S.cfg) return;
  for (const k of ['src', 'dst']) {
    // Chips alone are fine, but text that matched nothing still stops the patch.
    const b = boxes[k];
    if (!sel[k] && (k === 'src' || !dsts.length || b.input.value.trim())) {
      b.input.classList.add('pb-input--invalid');
      b.input.focus();
      return;
    }
  }
  const levels = lvl.levels();
  if (!levels) {
    lvlInput.classList.add('pb-input--invalid');
    toLevels();
    return;
  }
  openConfirm(patchPlan(sel.src, allDsts(), levels));
}

function openConfirm(plan) {
  const { block, prot } = planBlocked(plan);
  if (block) return showResult(block.kind, block.title, block.text);
  confirmBack = document.activeElement;
  confirmKeys = keyMode();
  confirmFromHist = confirmBack === histList || histArea(confirmBack);
  confirm.open(plan, prot);
}

// Focus returns to where the confirm opened. Keyboard mode returns only if it was on and the answer came from the
// keyboard. From History, focus goes to Source and the boxes stay (the row is redrawn).
function focusBack(viaKeys) {
  if (confirmKeys && viaKeys) enterKeys();
  else if (!confirmFromHist && confirmBack?.isConnected) confirmBack.focus();
  else focusBox(boxes.src.input);
}

function clearAll(focus = true) {
  dsts = [];
  for (const k of ['src', 'dst']) select(k, null);
  lvl.clear();
  renderTally();
  if (focus) boxes.src.input.focus();
}

for (const k of ['src', 'dst']) {
  const b = boxes[k];
  b.input.addEventListener('input', () => {
    sel[k] = null;
    if (k === 'dst') lastAdd = null;
    b.input.classList.remove('pb-input--invalid');
    b.cb.refresh();
    renderTally();
  });
  b.input.addEventListener('blur', () => b.cb.close());
  // Focus lists nothing in an empty or confirmed box: its Show all button is there (↓/↑ opens
  // the list too).
  b.input.addEventListener('focus', () => {
    if (!sel[k] && b.input.value.trim()) b.cb.refresh();
    b.all.sync();
  });
  b.all = showAll(b, $(`${k}-all`), () => sel[k]);
}
function showList(k, at, keepMoved = false) {
  const b = boxes[k];
  if (sel[k] || !b.input.value.trim()) b.cb.browse(b.list(), at, { keepMoved });
  else b.cb.refresh();
}
// A focus on a box scrolls to the top (Settings, This browser).
const calm = matchMedia('(prefers-reduced-motion: reduce)');
// From down the page, skip the browser's own jump so the scroll animates all the way.
function focusBox(input) {
  input.focus({ preventScroll: scrollTopPref() });
}
for (const input of [boxes.src.input, boxes.dst.input, lvlInput]) {
  input.addEventListener('focus', () => {
    input.select();
    // Not while the Lock goes up or away: the page keeps its scroll.
    if (active() && scrollTopPref() && window.scrollY > 0 && !document.documentElement.dataset.sodeLock) {
      window.scrollTo({ top: 0, behavior: calm.matches ? 'auto' : 'smooth' });
    }
  });
}

desktopKeys({
  boxes,
  dsts: () => dsts,
  confirm,
  keys,
  historyView,
  active,
  select,
  addDst,
  useLast,
  wantsMatch,
  patchNow,
  clearEverything,
  nothingToClear,
  grid: { shown: gridShown, toggle: toggleGrid },
});

$('dst-add').addEventListener('mousedown', (e) => e.preventDefault());
$('dst-add').addEventListener('click', () => addDst());
$('dst-add').dataset.pbTip = `Add destination (+, ${keyLabel('mod+enter')})`;
$('patch').addEventListener('mousedown', (e) => e.preventDefault()); // keep focus where it was
$('patch').addEventListener('click', () => patchNow());
$('clear-all').addEventListener('mousedown', (e) => e.preventDefault());
$('clear-all').addEventListener('click', () => clearEverything());
on('staged', paintTakes);
on('presetAdd', paintTakes);

on('hello', () => {
  pager.reset();
  lvlAt = lvlRouter();
  lvl.reset();
  // A Settings save sends a new hello, so an open page follows a change of levels.
  const box = lvlInput.closest('.sode__box');
  box.hidden = !levelsShown();
  if (box.hidden) {
    lvl.clear();
    if (document.activeElement === lvlInput) toLevels();
  }
  renderTally();
  renderHistory();
});
on('names', () => {
  for (const k of ['src', 'dst']) {
    const b = boxes[k];
    if (document.activeElement !== b.input || (!b.cb.open && (sel[k] || !b.input.value.trim()))) continue;
    showList(k, b.cb.open ? b.cb.current()?.n : sel[k]?.n, true);
  }
  renderTally();
});
on('tally', renderTally);
on('myRights', renderTally); // the route view's Protect follows the Protect right
// Done (or Cancel) in a pick bar: a match list must not stay open over the page.
on('pick', () => {
  if (!presetAdd.active()) for (const k of ['src', 'dst']) boxes[k].cb.close();
});
on('conn', renderConn);
// One redraw per frame: each take in a burst sends an entry, and a redraw rebuilds every row.
let histFrame = 0;
const redrawHistory = () => {
  histFrame ||= requestAnimationFrame(() => {
    histFrame = 0;
    renderHistory();
  });
};
on('history', redrawHistory);
// A Protect row's Undo follows the router's state, the sign-in and the Rights (historyrows.js protectActions).
on('protectedRows', redrawHistory);
on('myRights', redrawHistory);
on('conn', redrawHistory);
on('result', (r) => $('result').replaceChildren(resultNode(r)));

presetHost({
  active,
  confirm: openConfirm,
  patch: () => boxPatch(sel.src, allDsts(), lvl),
  picks: () => ({ src: sel.src, dsts: allDsts(), levels: lvl.empty() ? [] : lvl.levels() || [] }),
  clear: clearAll,
  // The Grid popup's Take and Clear (gridpopup.js), with no focus move: the boxes are under the popup.
  take: () => patchNow(false),
  clearAll: () => clearEverything(false),
  takeState,
  focus: () => boxes.src.input.focus(),
  // Back from the Grid popup: the first empty box, else Levels, so the next Enter takes (owner's rule).
  entry: () => (!sel.src ? focusBox(boxes.src.input) : !allDsts().length ? focusBox(boxes.dst.input) : toLevels()),
  route: boxRoute,
  // showGrid picks typed text first, so the popup opens on what the boxes show, not on the last place.
  pickTyped,
  // A staged route's chips (gridpopup.js), as a History row's.
  useEnd,
  addEnd,
  fill: (p) => {
    dsts = p.dsts.slice(0, -1);
    select('src', p.src);
    select('dst', p.dsts[p.dsts.length - 1]);
    if (p.levels.length) lvl.set(p.levels);
    else lvl.clear();
    toLevels();
  },
});

compactCentre();
