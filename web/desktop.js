// Desktop layout: three boxes, Tab between them, churning-key confirm.

import {
  $,
  el,
  S,
  on,
  emit,
  connState,
  routerLabel,
  needsLink,
  renderDown,
  setDot,
  search,
  dstLabel,
  dstItem,
  srcItem,
  tallyText,
  labelNode,
  toggle,
  keepSearch,
} from './core.js';
import { mac, chord } from './keys.js';
import { levelBar, allLevels } from './levelview.js';
import { fedBy, bySource, currentlyNodes, pickEnd, srcRow } from './routes.js';
import { timeNodes, scrollTopPref, flipped, setFlip } from './device.js';
import { historyPager, beforeGroups, attachFollows, entryNote, statusNode, folderTitle, folderLevels, folderIds } from './history.js';
import { rowUndoPlan, planEmpty, resultNode, showResult, patchPlan, planBlocked, sendTakes, lastTake, nextTake, boxPatch, lastChip, VERBS } from './plans.js';
import { layout } from './layout.js';
import { openFilters, emptyNote } from './filters.js';
import { presetHost } from './presets.js';
import { rowActions, disclosureAria, followersHead, lastFill } from './recentrows.js';
import { picking, isPicked, pickBox, togglePick, bindPick, boxPicking, addPatch } from './recentpick.js';
import { keyConfirm } from './keyconfirm.js';
import { showAll } from './showall.js';
import { shortcutList } from './shortcuts.js';
import {
  byLabel,
  levelBox,
  combobox,
  disclosure,
  undoButton,
  againButton,
  arrivals,
  stackSteps,
  followBoxes,
  failureRows,
  clickToToggle,
  pullToLoad,
} from './widgets.js';

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
  });
}
const lvlInput = $('lvl');
const lvl = levelBox(lvlInput, $('lvl-pick'), $('lvl-all'), { onChange: () => renderTally() });

const active = () => layout() === 'desktop' && document.documentElement.dataset.view !== 'settings';
const expanded = new Set();
const openFolders = new Set();
const openFollowers = new Set();
const arrived = arrivals();

// ---- connection

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

function histRow(e) {
  const src = srcItem(e.src, e.srcName),
    dst = dstItem(e.dst, e.dstName);
  const known = e.levels.filter((u) => S.levelBy.has(u));

  const acts = rowActions({ e });
  const btn = undoButton(acts.undo.plan, acts.undo.label, openConfirm, { text: acts.undo.text, tip: acts.undo.tip });
  const retry = againButton(acts.again.plan, acts.again.label, openConfirm);

  const open = expanded.has(e.id);
  const detailId = `hist-before-${e.id}`;
  const disc = disclosure(open, 'Before', detailId, () => {
    toggle(expanded, e.id);
    renderHistory();
  });

  const row = el(
    'li',
    'pb-input sode__hist-row' + (arrived.entry(e) ? ' sode--enter' : '') + curClass(`e${e.id}`),
    el('span', 'sode__hist-id', `#${e.id}`),
    histTime(e.time),
    levelBar(known),
    el('span', 'sode__hist-route sode__hist-route--boxes', ...routeBoxes(src, dst), entryNote(e), byLabel(e)),
    statusNode({ e }),
    disc,
    el('span', 'sode__hist-actions', ...(picking() ? [pickBox(`e${e.id}`)] : [retry, btn])),
  );
  row.dataset.key = `e${e.id}`;
  row.classList.toggle('sode__hist-row--picked', isPicked(`e${e.id}`));
  clickToToggle(row, disc, '.sode__hist-before');
  if (open) {
    const detail = el(
      'div',
      'sode__hist-before',
      ...(e.chain
        ? [el('div', 'sode__label', `Changes on ${dst.label}`), ...stackSteps(e, { restore: openConfirm, pickSrc: useRowSrc, pickDst: useRowDst })]
        : [el('div', 'sode__label', `Before #${e.id}, on ${dst.label}`), ...beforeGroups(e).map((g) => srcRow(g.levels, tallyText(g.src), g.src, useRowSrc))]),
      ...failureRows(e),
      ...(e.follows && !e.chain ? [el('div', 'sode__route', followBoxes(e.follows, e.levels, useRowDst))] : []),
    );
    // Below 1000px the row's level bar moves in here (see history.css).
    detail.prepend(el('div', 'sode__hist-levels', el('div', 'sode__label', 'Levels'), levelBar(known)));
    detail.id = detailId;
    row.append(detail);
  }
  return row;
}

const routeBoxes = (src, dst) => [routeBox('src', src), el('span', 'sode__arrow', '→'), routeBox('dst', dst)];
function routeBox(k, it) {
  const cls = `sode__end sode__end--${k}`;
  if (it instanceof Node) return el('span', cls, it);
  const b = el('button', `${cls} sode__end--pick`, labelNode(it));
  b.type = 'button';
  b.tabIndex = -1;
  b.title = `Use as ${k === 'src' ? 'source' : 'destination'}`;
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', () => useEnd(k, it));
  return b;
}

function useEnd(k, it) {
  select(k, it);
  focusBox(k === 'src' ? boxes.dst.input : boxes.src.input);
}
function useRowSrc(n) {
  useEnd('src', srcItem(n));
}
const useRowDst = (it) => useEnd('dst', it);

function folderRow(f) {
  const open = openFolders.has(f.group);
  const kidsId = `hist-folder-${f.group}`;
  const title = folderTitle(f);
  const one = (k, item) => (new Set(f.entries.map((e) => e[k])).size === 1 ? item(f.entries[0][k], f.entries[0][`${k}Name`]) : null);

  const acts = rowActions({ f });
  const undo = undoButton(acts.undo.plan, acts.undo.label, openConfirm, { text: acts.undo.text, tip: acts.undo.tip });
  const retry = againButton(acts.again.plan, acts.again.label, openConfirm);

  const disc = disclosure(open, `${f.shown.length}`, kidsId, () => {
    toggle(openFolders, f.group);
    autoOpen.delete(f.group); // opened by hand: stays open
    renderHistory();
  });
  disc.setAttribute('aria-label', disclosureAria(open, f.shown.length, 'takes'));

  const row = el(
    'li',
    'pb-input sode__hist-row' + (arrived.folder(f, open) ? ' sode--enter' : '') + curClass(`f${f.group}`),
    el('span', 'sode__hist-id', folderIds(f)),
    histTime(f.entries[0].time),
    levelBar(folderLevels(f)),
    el(
      'span',
      'sode__hist-route sode__hist-route--boxes',
      ...routeBoxes(one('src', srcItem) || title[0].firstChild, one('dst', dstItem) || title[2].firstChild),
      ...title.slice(3),
      byLabel(f),
    ),
    statusNode({ f }),
    disc,
    el('span', 'sode__hist-actions', ...(picking() ? [pickBox(`f${f.group}`)] : [retry, undo])),
  );
  row.dataset.key = `f${f.group}`;
  row.classList.toggle('sode__hist-row--picked', isPicked(`f${f.group}`));
  clickToToggle(row, disc, '.sode__hist-folder');
  if (open) {
    const kids = el('ul', 'sode__hist-folder', ...f.shown.map(histRow), ...followerRows(f));
    kids.id = kidsId;
    row.append(kids);
  }
  return row;
}

// Followers: changes made elsewhere that seem to follow the confirm; a keyboard cursor stop
// like a folder (key w<group>).
function followerRows(f) {
  if (!f.followers?.length) return [];
  const open = openFollowers.has(f.group);
  const { count, aria, kidsId } = followersHead(f, open, 'hist');
  const key = `w${f.group}`;
  const disc = disclosure(open, count, kidsId, () => {
    toggle(openFollowers, f.group);
    renderHistory();
  });
  disc.setAttribute('aria-label', aria);
  const head = el(
    'li',
    'pb-input sode__hist-row' + curClass(key),
    el('span', 'sode__hist-id'),
    el('span', 'sode__hist-time'),
    el('span'),
    el('span', 'sode__hist-route', 'Followers'),
    el('span'),
    disc,
    el('span', 'sode__hist-actions'),
  );
  head.dataset.key = key;
  clickToToggle(head, disc, '.sode__hist-folder');
  if (open) {
    const kids = el('ul', 'sode__hist-folder', ...f.followers.map(histRow));
    kids.id = kidsId;
    head.append(kids);
  }
  return [head];
}

function histTime(iso) {
  const t = el('span', 'sode__hist-time', ...timeNodes(iso));
  t.dataset.hint = t.textContent;
  return t;
}

const curClass = (key) => (key === cursor ? ' sode__hist-row--cur' : '');

const pager = historyPager(10, () => layout() === 'desktop');
pullToLoad(window, $('history-more'), pager, () => active() && !document.querySelector('dialog[open]'));

// scroll: only when the cursor just moved; other redraws leave the page where it is.
function renderHistory(scroll = false) {
  const list = pager.list();
  if (cursor && !stops(list).some((x) => x.key === cursor)) {
    // Its row left the page: drop the cursor and close the folders it opened.
    cursor = null;
    for (const g of autoOpen) openFolders.delete(g);
    autoOpen.clear();
  }
  histList.replaceChildren(...list.map((r) => (r.f ? folderRow(r.f) : histRow(r.e))));
  $('history-more').replaceChildren(...[emptyNote(list, pager.fetching()), pager.button()].filter(Boolean));
  if (scroll === true && keyMode()) histList.querySelector('.sode__hist-row--cur')?.scrollIntoView({ block: 'nearest' });
}

// ---- keyboard mode: focus on the history list, where single keys act.
// Cursor is 'e<id>' on a take, 'f<group>' on a folder. Keyboard entry only; any click leaves it.

let cursor = null;
const autoOpen = new Set();
const histList = $('history');
bindPick(histList);
on('pick', () => renderHistory());
let keysOn = false;
function setKeys(on) {
  keysOn = on;
  histList.classList.toggle('sode__hist--keys', on);
  document.documentElement.toggleAttribute('data-keys-on', on);
}
histList.addEventListener('blur', () => document.documentElement.removeAttribute('data-keys-on'));
const keyMode = () => keysOn && document.activeElement === histList;
document.addEventListener(
  'pointerdown',
  (e) => {
    setKeys(false);
    // Clicking a row moves the hidden cursor there so the next key starts on it.
    const row = histList.contains(e.target) && e.target.closest('[data-key]');
    if (row) cursor = row.dataset.key;
  },
  true,
);

// top: the row's (or its folder's) index in the pager's list.
function stops(list = pager.list()) {
  const out = [];
  list.forEach((r, top) => {
    if (r.e) return out.push({ key: `e${r.e.id}`, e: r.e, top });
    out.push({ key: `f${r.f.group}`, f: r.f, top });
    if (!openFolders.has(r.f.group)) return;
    for (const e of r.f.shown) out.push({ key: `e${e.id}`, e, top, in: r.f });
    if (!r.f.followers?.length) return;
    out.push({ key: `w${r.f.group}`, head: r.f, top, in: r.f });
    if (openFollowers.has(r.f.group)) for (const e of r.f.followers) out.push({ key: `e${e.id}`, e, top, in: r.f, follower: true });
  });
  return out;
}
const curStop = (list = stops()) => list.find((x) => x.key === cursor);

function moveTo(s) {
  const group = s.f?.group ?? s.in?.group;
  let redraw = false;
  for (const g of autoOpen) {
    if (g !== group) {
      openFolders.delete(g);
      autoOpen.delete(g);
      redraw = true;
    }
  }
  if (s.f && !openFolders.has(s.f.group)) {
    openFolders.add(s.f.group);
    autoOpen.add(s.f.group);
    redraw = true;
  }
  const row = (key) => key && histList.querySelector(`[data-key="${CSS.escape(key)}"]`);
  const from = row(cursor),
    to = row(s.key);
  cursor = s.key;
  if (redraw || !to) return renderHistory(true);
  from?.classList.remove('sode__hist-row--cur');
  to.classList.add('sode__hist-row--cur');
  if (keyMode()) to.scrollIntoView({ block: 'nearest' });
}

function loadMore() {
  if (pager.hasMore() && !S.histLoading) pager.more();
}

function step(d) {
  const list = stops();
  const i = list.findIndex((x) => x.key === cursor);
  if (i < 0) return list.length && moveTo(list[d > 0 ? 0 : list.length - 1]);
  const next = list[i + d];
  if (!next) return d > 0 && loadMore();
  moveTo(next);
  if (d < 0 && next.f && list[i].in?.group !== next.f.group && autoOpen.has(next.f.group)) {
    const kids = stops().filter((x) => x.in?.group === next.f.group);
    moveTo(kids[kids.length - 1]);
  }
}

function latest() {
  const first = stops()[0];
  if (first) moveTo(first);
}

function nextGroup() {
  const list = stops();
  const s = curStop(list);
  const next = list.find((x) => x.f && (!s || x.top > s.top));
  if (next) moveTo(next);
  else loadMore();
}

function open(on) {
  const s = curStop();
  if (!s) return;
  if (s.head) {
    if (on) openFollowers.add(s.head.group);
    else if (openFollowers.has(s.head.group)) openFollowers.delete(s.head.group);
    else return moveTo(stops().find((x) => x.key === `f${s.in.group}`));
  } else if (s.e) {
    if (on) expanded.add(s.e.id);
    else if (expanded.has(s.e.id)) expanded.delete(s.e.id);
    else if (s.follower) return moveTo(stops().find((x) => x.key === `w${s.in.group}`));
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
  const plan = againPlan(s.f ? { f: s.f } : { e: s.e });
  if (plan.items.length) openConfirm(plan);
  else showResult('info', 'Nothing to redo', 'This row has no source or level to take again.');
}
function enterKeys(key) {
  setKeys(true);
  histList.focus();
  const list = stops();
  const to = (key && list.find((x) => x.key === key)) || list.find((x) => x.key === cursor) || list[0];
  if (to) moveTo(to);
  else renderHistory();
}

const BOXES = { s: 'src', d: 'dst', f: 'lvl' };

function onListKey(e) {
  const jk = e.ctrlKey !== e.metaKey && (e.ctrlKey || mac) && !e.altKey && !e.shiftKey && /^Key[JK]$/.test(e.code) ? e.code[3].toLowerCase() : '';
  if (!jk && (e.ctrlKey || e.metaKey || e.altKey)) return false;
  const k = jk || e.key;
  if (BOXES[k]) {
    focusBox($(BOXES[k]));
    return true;
  }
  if (k === ',') {
    $('settings-open').click();
    return true;
  }
  if (k === '/') {
    openFilters();
    return true;
  }
  if (k === 'S' || k === 'D') {
    focusSearch(k === 'S' ? 'src' : 'dst');
    return true;
  }
  if (k === '?') {
    keys.open();
    return true;
  }
  if (k === 'p') {
    emit('presetStrip');
    return true;
  }
  if (k === 'Escape') {
    focusBox(boxes.src.input);
    return true;
  }
  // Choosing takes for a preset: Space/Enter ticks the row, nothing else acts.
  if (picking()) {
    if (keyMode() && (k === ' ' || k === 'Enter')) {
      if (cursor) togglePick(cursor);
      return true;
    }
    if (k === 'u' || k === 'r') return true;
  }
  const act = {
    j: () => step(1),
    ArrowDown: () => (e.shiftKey ? nextGroup() : step(1)),
    k: () => step(-1),
    ArrowUp: () => (e.shiftKey ? latest() : step(-1)),
    J: nextGroup,
    K: latest,
    l: () => open(true),
    ArrowRight: () => open(true),
    Enter: () => open(!e.shiftKey),
    h: () => open(false),
    ArrowLeft: () => open(false),
    u: () => {
      const s = curStop();
      if (s && (s.f || s.e)) undoRow(s.f ? { f: s.f } : { e: s.e });
    },
    r: retryCur,
  }[k];
  if (!act) return false;
  // From the page itself, arrows keep scrolling; only letters start keyboard mode.
  if (!keyMode() && k.startsWith('Arrow')) return false;
  if (!keyMode()) enterKeys();
  else act();
  return true;
}

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

// Undo the newest take made in Sode (a whole folder); other panels' changes are skipped
// whatever the filters show.
function undoLast() {
  const last = S.hist.find((x) => !x.ext);
  if (!last) return showResult('info', 'Nothing to undo', 'No takes made in Sode yet.');
  const e = attachFollows(last);
  const takes = e.group && e.groupN > 1 && S.hist.filter((x) => x.group === e.group).map(attachFollows);
  undoRow(takes ? { f: { group: e.group, n: e.groupN, entries: takes, shown: takes } } : { e });
}

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
function histArea(n) {
  return !!n && (histList.contains(n) || $('history-more').contains(n));
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

// Modifier rule: see AGENTS.md. Each check excludes the other two modifiers.
document.addEventListener('keydown', (e) => {
  if (confirm.isOpen()) return confirm.key(e);
  if (layout() !== 'desktop') return;
  // Ctrl+? toggles the shortcut list anywhere, even mid-typing. Matched by character (any
  // layout) and by the / key, for browsers reporting e.key without Shift under Ctrl.
  if (e.ctrlKey && !e.metaKey && !e.altKey && (e.key === '?' || (e.shiftKey && e.code === 'Slash'))) {
    e.preventDefault();
    e.stopImmediatePropagation(); // not Settings' own keys
    if (keys.shown()) keys.close(true);
    else keys.open();
    return;
  }
  if (!$('keys').hidden) {
    e.stopImmediatePropagation(); // Esc here closes the list, not Settings
    if (e.key === 'Escape' || e.key === '?') {
      e.preventDefault();
      keys.close(true);
    }
    // Keep focus in the dialog: Close is its only control.
    if (e.key === 'Tab') {
      e.preventDefault();
      $('keys-close').focus();
    }
    return;
  }
  if (!active()) return;
  if (e.ctrlKey && !e.metaKey && !e.altKey && e.key === ',') {
    e.preventDefault();
    $('settings-open').click();
    return;
  }
  const last = e.ctrlKey && !e.metaKey && !e.altKey && { KeyS: 'src', KeyD: 'dst', KeyF: 'lvl' }[e.code];
  if (last && (!e.shiftKey || last !== 'lvl')) {
    e.preventDefault();
    useLast(last, e.shiftKey);
    return;
  }
  const id = e.target.id;
  const k = id === 'src' || id === 'dst' ? id : null;
  const b = k && boxes[k];
  const inBox = k || id === 'lvl';

  // Ctrl+Z / ⌘Z undoes the last Sode take; the OS text-undo chord leaves a box with text to
  // undo its typing.
  const cmdZ = mac && e.metaKey && !e.ctrlKey;
  if ((cmdZ || (e.ctrlKey && !e.metaKey)) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'z') {
    if (cmdZ === mac && e.target instanceof HTMLInputElement && e.target.value) return;
    e.preventDefault();
    undoLast();
    return;
  }

  if (b && !b.cb.open && (chord(e) === 'down' || chord(e) === 'up')) {
    e.preventDefault();
    b.all.open();
    return;
  }
  if (b && b.cb.nav(e)) return;
  if (k === 'dst' && e.key === 'Enter' && e.ctrlKey !== e.metaKey && !e.shiftKey && !e.altKey) {
    e.preventDefault();
    addDst();
    return;
  }
  // + adds another destination; Ctrl/Cmd + is left for zoom.
  if (k === 'dst' && e.key === '+' && !e.ctrlKey && !e.metaKey) {
    e.preventDefault();
    addDst();
    return;
  }
  if (k === 'dst' && e.key === 'Backspace' && !b.input.value && dsts.length) {
    e.preventDefault();
    select('dst', dsts.pop());
    return;
  }
  if (e.key === 'Tab') {
    if (b && wantsMatch(k) && !b.cb.accept()) {
      e.preventDefault();
      b.input.classList.add('pb-input--invalid');
      return;
    }
    if (inBox) {
      const ring = flipped() ? ['dst', 'src', 'lvl'] : ['src', 'dst', 'lvl'];
      const i = ring.indexOf(id);
      if (i >= 0) {
        e.preventDefault();
        $(ring[(i + (e.shiftKey ? ring.length - 1 : 1)) % ring.length]).focus();
      }
    }
    return;
  }
  if (e.target === histList || e.target === document.body) {
    if (onListKey(e)) e.preventDefault();
    return;
  }
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
    if (b && b.cb.open) {
      b.cb.close();
      return;
    }
    if (e.target.value) {
      if (k) select(k, null);
      else lvl.clear();
      renderTally();
    } else if (!dsts.length && !boxes.src.input.value && !boxes.dst.input.value && lvl.empty()) {
      enterKeys();
    } else {
      clearAll();
    }
  }
});

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
