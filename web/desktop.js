// Desktop layout: three boxes, Tab between them, churning-key confirm.

import { $, el, S, on, connState, routerTitle, needsLink, renderDown, setDot, search, dstLabel, dstItem, srcItem, keepSearch } from './core.js';
import { levelBar, allLevels } from './levelview.js';
import { fedBy, bySource, currentlyNodes, pickEnd } from './routes.js';
import { scrollTopPref, flipped, setFlip } from './device.js';
import { resultNode, showResult, patchPlan, planBlocked, rightsOff, sendTakes, lastTake, nextTake, boxPatch, lastChip, VERBS } from './plans.js';
import { layout } from './layout.js';
import { presetHost } from './presets.js';
import { lastFill } from './historyrows.js';
import { boxPicking, addPatch } from './historypick.js';
import { keyConfirm } from './keyconfirm.js';
import { showAll } from './showall.js';
import { shortcutList } from './shortcuts.js';
import { levelBox, combobox } from './widgets.js';
import { desktopHistory } from './desktophistory.js';
import { desktopKeys } from './desktopkeys.js';

const sel = { src: null, dst: null };
let dsts = [];
const chosen = (d) => dsts.some((x) => x.n === d.n);
const MAX_MATCHES = 100;
const boxes = {
  src: { input: $('src'), list: () => S.srcs },
  dst: { input: $('dst'), list: () => (dsts.length ? S.dsts.filter((d) => !chosen(d)) : S.dsts) },
};
for (const k of ['src', 'dst']) {
  const b = boxes[k];
  b.cb = combobox(b.input, $(`${k}-menu`), {
    source: (text) => search(b.list(), text, MAX_MATCHES),
    onPick: (it) => select(k, it),
    add: k === 'dst' ? (it) => addDst(it) : undefined,
    foot: $(`${k}-count`),
    onRender: () => b.all?.sync(),
    outside: (it) => rightsOff(k === 'src' ? 'sources' : 'dsts', it.n),
  });
}
const lvlInput = $('lvl');
const lvl = levelBox(lvlInput, $('lvl-pick'), $('lvl-all'), { onChange: () => renderTally() });

const active = () => layout() === 'desktop' && document.documentElement.dataset.view !== 'settings';

// ---- connection

function renderConn() {
  const c = connState();
  setDot($('conn-srv-dot'), $('conn-srv-text'), [c.ws[0], 'Server']);
  setDot($('conn-rt-dot'), $('conn-rt-text'), [c.rt[0], 'Router']);
  const [label, isName] = routerTitle();
  $('conn-addr').textContent = label;
  $('conn-addr').classList.toggle('sode__conn-addr--name', isName);
  $('conn').title = [c.ws[1], `${c.rt[1]} ${S.routerAddr}`, S.router !== 'up' && S.routerLastErr].filter(Boolean).join('\n');
  document.documentElement.dataset.conn = c.down ? 'down' : 'up';
  renderDown($('conn-down'), c);
}
needsLink($('patch'));

// ---- the boxes

function select(k, it) {
  sel[k] = it;
  const b = boxes[k];
  b.input.value = it ? it.label : '';
  b.input.classList.remove('pb-input--invalid');
  b.cb.close();
  if (k === 'dst') renderChips();
  renderTally();
}

// ---- destination chips

const allDsts = () => (sel.dst && !chosen(sel.dst) ? [...dsts, sel.dst] : [...dsts]);

// The typed search stays after +, so the next match is one + away; while the box holds exactly
// it, Enter/Tab/Patch go on with the chips.
const kept = keepSearch(() => boxes.dst.input.value);
const keptText = kept.is;

function addDst(pick) {
  const b = boxes.dst;
  b.input.focus();
  // A row moved to in the browse list counts though the box holds no text.
  if (!pick && b.cb.moved) pick = b.cb.current();
  let typed = b.input.value;
  if (pick) sel.dst = pick;
  else if (!sel.dst) {
    if (!typed.trim()) return;
    if (!b.cb.open) b.cb.refresh();
    if (!b.cb.accept()) {
      b.input.classList.add('pb-input--invalid');
      return;
    }
  } else typed = '';
  const added = sel.dst.n;
  if (!chosen(sel.dst)) dsts.push(sel.dst);
  sel.dst = null;
  b.input.value = typed;
  kept.set(true, typed);
  b.input.classList.remove('pb-input--invalid');
  renderChips();
  renderTally();
  showList('dst', b.list().find((d) => d.n > added)?.n);
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
        renderChips();
        renderTally();
      });
      const chip = el('span', 'pb-chip pb-chip--static', d.label, x);
      chip.setAttribute('role', 'listitem');
      return chip;
    }),
  );
  const n = allDsts().length;
  $('dst-label').textContent = n > 1 ? `Destinations (${n})` : 'Destination';
}

// ---- the last take's source, destinations or levels (Ctrl+S/D/F)

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
        el('kbd', 'sode__key-note', `⌃${key}`),
      );
      b.title = `Last take's ${what} (Ctrl+${key})`;
      b.addEventListener('click', () => useLast(k));
      chips.push(b);
    }
    if (k !== 'lvl' && next[k]) {
      const b = el(
        'button',
        'pb-chip sode__last sode__next',
        el('span', 'sode__tag', 'Next'),
        el('kbd', 'sode__key-note sode__tag-key', `⌃⇧${key}`),
        el('span', 'sode__next-open', el('span', 'sode__next-in', el('span', 'sode__last-text', ...next[k]), el('kbd', 'sode__key-note', `⌃⇧${key}`))),
      );
      b.title = `Last take's ${what}, the next one (Ctrl+Shift+${key.toUpperCase()})`;
      b.addEventListener('click', () => useLast(k, true));
      // The closed chip's room stays in the slot so nothing in the row moves.
      chips.push(el('span', 'sode__next-slot', el('span', 'pb-chip sode__last sode__next-size', 'Next', el('kbd', 'sode__key-note', `⌃⇧${key}`)), b));
    }
    for (const b of chips.flatMap((c) => (c.matches('button') ? [c] : [...c.querySelectorAll('button')]))) {
      b.type = 'button';
      b.tabIndex = -1;
      b.addEventListener('mousedown', (e) => e.preventDefault());
    }
    slot.replaceChildren(...chips);
  }
}
on('history', renderLast);
on('last', renderLast);

// Fills one box from the last take, then focuses the opposite box only if empty, else Levels.
// next: use nextTake() and remember it so a repeat goes on.
function useLast(k, next = false) {
  const fill = lastFill(k, next, boxState());
  if (fill?.none) return showResult('info', 'No take yet', "There's no earlier take to reuse.");
  if (!fill) return;
  const t = fill.t;
  if (k === 'src') {
    select('src', t.src);
    const dstEmpty = !sel.dst && !dsts.length && !boxes.dst.input.value.trim();
    focusBox(dstEmpty ? boxes.dst.input : lvlInput);
  } else if (k === 'dst') {
    const have = allDsts();
    const add = t.dsts.filter((d) => !have.some((x) => x.n === d.n));
    if (add.length) {
      dsts = [...have, ...add.slice(0, -1)];
      select('dst', add[add.length - 1]);
    }
    const srcEmpty = !sel.src && !boxes.src.input.value.trim();
    focusBox(srcEmpty ? boxes.src.input : lvlInput);
  } else {
    lvl.set(t.levels);
    focusBox(lvlInput);
  }
}

// ---- tally display

// Both lists show at once, on their own row (the flip swaps them).
function underHead(text, up) {
  return el('div', 'sode__under-head', el('span', up ? 'sode__down-arrow sode__down-arrow--up' : 'sode__down-arrow'), el('span', 'sode__label', text));
}

function sendingRow(src, d, levels) {
  const t = S.tally.get(d);
  const pick = pickEnd('dst', dstLabel(d), 'Add as a destination', () => {
    const have = allDsts();
    if (!have.some((x) => x.n === d)) {
      dsts = have;
      select('dst', dstItem(d));
    }
    lvlInput.focus();
  });
  return el('div', 'sode__route', levelBar(levels.filter((u) => t.get(u) === src)), pick);
}

function pickSrc(n) {
  select('src', srcItem(n));
  lvlInput.focus();
}

function renderTally() {
  renderLast();
  const srcUnder = $('src-under'),
    dstUnder = $('dst-under');
  if (!S.cfg) {
    srcUnder.replaceChildren();
    return dstUnder.replaceChildren();
  }
  const scrolled = [srcUnder, dstUnder].map((u) => u.querySelector('.sode__scroll')?.scrollTop || 0);
  if (sel.src) {
    const levels = lvl.levels() || allLevels();
    const fed = fedBy(sel.src.n, levels);
    srcUnder.replaceChildren(
      underHead(fed.length ? `Sending to ${fed.length}` : 'Sending'),
      fed.length
        ? el('div', 'sode__scroll', ...fed.map((d) => sendingRow(sel.src.n, d, levels)))
        : el('div', 'sode__hint', 'Not sending to any destination on these levels'),
    );
  } else srcUnder.replaceChildren();
  const ds = allDsts();
  if (ds.length) {
    const srcs = new Set(ds.flatMap((d) => bySource(d.n, allLevels()).map((r) => r.src)).filter((s) => s != null && s >= 0));
    dstUnder.replaceChildren(
      underHead(srcs.size ? `Receiving from ${srcs.size}` : 'Receiving', true),
      el('div', 'sode__scroll', ...ds.flatMap((d) => currentlyNodes(d, pickSrc))),
    );
  } else dstUnder.replaceChildren();
  [srcUnder, dstUnder].forEach((u, i) => {
    const box = u.querySelector('.sode__scroll');
    if (box) box.scrollTop = scrolled[i];
  });
}

// ---- history

const historyView = desktopHistory({ select, focusBox, openConfirm, focusSearch, openKeys: () => keys.open(), boxes, active });
const { renderHistory, enterKeys, keyMode, histList, histArea, pager } = historyView;

// ---- flip order (per browser; index.html applies it before first paint)

function showFlip() {
  const on = flipped();
  $('flip').setAttribute('aria-pressed', String(on));
  $('flip').dataset.pbTip = on ? 'Show Source first' : 'Show Destination first';
}
$('flip').addEventListener('mousedown', (e) => e.preventDefault());
$('flip').addEventListener('click', () => setFlip(!flipped()));
on('flip', showFlip);
showFlip();

// ---- shortcut list (?)

const keys = shortcutList({ keyMode, fromHistory: histArea, enterKeys, focusFirst: () => focusBox(boxes.src.input) });

function focusSearch(k) {
  const f = $('history-search').querySelector(`[data-search-${k}]`);
  if ($('history-search').hidden) return;
  f.focus();
  f.select();
}

// ---- confirm

let confirmBack = null;
let confirmKeys = false;
let confirmFromHist = false; // decided at open: rows get redrawn

const confirm = keyConfirm({
  spaceHint: true,
  prompt: (key) => ['Press ', key, S.cfg.confirmEnter ? ' or Enter' : '', ' to confirm, or escape to cancel.'],
  onTake: (p, viaKeys) => {
    if (sendTakes(p) && VERBS[p.verb].resets) clearAll();
    else focusBack(viaKeys);
  },
  onClose: (p, viaKeys) => {
    if (p?.verb === 'patch') lvlInput.focus();
    else focusBack(viaKeys);
  },
});

function wantsMatch(k) {
  const b = boxes[k];
  return b.cb.moved || (!sel[k] && !!b.input.value.trim() && !(k === 'dst' && keptText()));
}

function patchNow() {
  for (const k of ['src', 'dst']) {
    const b = boxes[k];
    if (wantsMatch(k)) {
      if (!b.cb.open) b.cb.refresh();
      b.cb.accept();
    }
  }
  if (boxPicking()) return addPatch();
  tryConfirm();
}

function tryConfirm() {
  if (!S.cfg) return;
  for (const k of ['src', 'dst']) {
    // Chips alone are fine, but text that matched nothing still stops the patch.
    const b = boxes[k];
    if (!sel[k] && (k === 'src' || !dsts.length || (b.input.value.trim() && !(k === 'dst' && keptText())))) {
      b.input.classList.add('pb-input--invalid');
      b.input.focus();
      return;
    }
  }
  const levels = lvl.levels();
  if (!levels) {
    lvlInput.classList.add('pb-input--invalid');
    lvlInput.focus();
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

// Focus returns to where the confirm opened. Keyboard mode only if it was on and answered from
// the keyboard; from Recent otherwise it goes to Source, boxes untouched (the row is redrawn).
function focusBack(viaKeys) {
  if (confirmKeys && viaKeys) enterKeys();
  else if (!confirmFromHist && confirmBack?.isConnected) confirmBack.focus();
  else focusBox(boxes.src.input);
}

// ---- keyboard & input wiring

function clearAll() {
  dsts = [];
  for (const k of ['src', 'dst']) select(k, null);
  lvl.clear();
  renderTally();
  boxes.src.input.focus();
}

for (const k of ['src', 'dst']) {
  const b = boxes[k];
  b.input.addEventListener('input', () => {
    sel[k] = null;
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
// Focusing a box scrolls to the top (Settings → This device).
const calm = matchMedia('(prefers-reduced-motion: reduce)');
// From down the page, skip the browser's own jump so the scroll animates all the way.
function focusBox(input) {
  input.focus({ preventScroll: scrollTopPref() });
}
for (const input of [boxes.src.input, boxes.dst.input, lvlInput]) {
  input.addEventListener('focus', () => {
    input.select();
    // Not while the lock screen goes up or away: the page keeps its scroll.
    if (active() && scrollTopPref() && window.scrollY > 0 && !document.documentElement.dataset.sodeLock) {
      window.scrollTo({ top: 0, behavior: calm.matches ? 'auto' : 'smooth' });
    }
  });
}

desktopKeys({ boxes, dsts: () => dsts, lvl, confirm, keys, historyView, active, select, addDst, useLast, wantsMatch, patchNow, clearAll, renderTally });

$('dst-add').addEventListener('mousedown', (e) => e.preventDefault());
$('dst-add').addEventListener('click', () => addDst());
$('patch').addEventListener('mousedown', (e) => e.preventDefault()); // keep focus where it was
$('patch').addEventListener('click', patchNow);

// ---- subscriptions

on('hello', () => {
  pager.reset();
  lvl.reset();
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
// Done (or Cancel) in a pick bar: no match list may stay open over the page.
on('pick', () => {
  if (!boxPicking()) for (const k of ['src', 'dst']) boxes[k].cb.close();
});
on('conn', renderConn);
on('history', renderHistory);
on('result', (r) => $('result').replaceChildren(resultNode(r)));

presetHost({
  active,
  confirm: openConfirm,
  patch: () => boxPatch(sel.src, allDsts(), lvl),
  clear: clearAll,
  focus: () => boxes.src.input.focus(),
  fill: (p) => {
    dsts = p.dsts.slice(0, -1);
    select('src', p.src);
    select('dst', p.dsts[p.dsts.length - 1]);
    if (p.levels.length) lvl.set(p.levels);
    else lvl.clear();
    focusBox(lvlInput);
  },
});
