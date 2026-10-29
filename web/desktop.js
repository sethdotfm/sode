// Desktop layout: three boxes, Tab between them, churning-key confirm.
// + in the destination box adds another destination as a chip; the search
// and its match list stay for the next one.

import {
  $, el, S, on, connState, routerLabel, needsLink, renderDown, setDot, search, dstLabel, dstItem, srcItem, tallyText, labelNode, toggle, keepSearch,
} from './core.js';
import { levelBar, allLevels } from './levelview.js';
import { fedBy, bySource, currentlyNodes, pickEnd, srcRow } from './routes.js';
import { timeNodes, scrollTopPref, flipped, setFlip } from './device.js';
import {
  historyPager, beforeGroups, attachFollows, entryNote, statusNode, folderTitle, folderPartial, folderLevels, folderIds,
} from './history.js';
import {
  undoPlan, retryPlan, planVerb, folderUndoPlan, folderRetryPlan, rowUndoPlan, planEmpty, resultNode, showResult, patchPlan,
  planBlocked, planSummary, sendTakes, lastTake, lastChip,
} from './plans.js';
import { readStr, writeStr } from './store.js';
import { layout } from './layout.js';
import { openFilters, emptyNote } from './filters.js';
import { levelBox, combobox, disclosure, undoButton, retryButton, arrivals, lockout, stackSteps, followBoxes, failureRows, clickToToggle, pullToLoad } from './widgets.js';

const sel = { src: null, dst: null };
let dsts = []; // destinations added with +, before the one in the box
const chosen = (d) => dsts.some((x) => x.n === d.n);
// The dropdown scrolls: up to MAX_MATCHES results.
const MAX_MATCHES = 100;
const boxes = {
  src: { input: $('src'), list: () => S.srcs },
  // One entry per destination: chips already added don't come up again.
  dst: { input: $('dst'), list: () => (dsts.length ? S.dsts.filter((d) => !chosen(d)) : S.dsts) },
};
for (const k of ['src', 'dst']) {
  const b = boxes[k];
  b.cb = combobox(b.input, $(`${k}-menu`), {
    source: (text) => search(b.list(), text, MAX_MATCHES),
    onPick: (it) => select(k, it),
    // Destinations: a + on each row adds it as a chip and keeps the search.
    add: k === 'dst' ? (it) => addDst(it) : undefined,
    foot: true,
  });
}
const lvlInput = $('lvl');
const lvl = levelBox(lvlInput, $('lvl-pick'), $('lvl-all'), { onChange: () => renderTally() });

const active = () => layout() === 'desktop' && document.documentElement.dataset.view !== 'settings';
const mac = /Mac|iPhone|iPad/.test(navigator.platform);
const expanded = new Set(); // history IDs showing their before state
const openFolders = new Set(); // history groups showing their takes
const openFollowers = new Set(); // history groups showing their Followers
const arrived = arrivals();

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

// Every destination the patch will go to: the chips, then the box.
const allDsts = () => (sel.dst && !chosen(sel.dst) ? [...dsts, sel.dst] : [...dsts]);

// +: the box's match becomes a chip. The typed search stays and its list
// stays open without the chips, so the next match is one + away; typing
// edits the search.
// The search left in the box by the last + (null when there is none).
// While the box still holds exactly that, it isn't a destination waiting
// to be picked: Enter, Tab and Patch go on with the chips.
const kept = keepSearch(() => boxes.dst.input.value);
const keptText = kept.is;

function addDst(pick) {
  const b = boxes.dst;
  b.input.focus();
  // Picking a match writes its label into the box; the search is put back.
  let typed = b.input.value;
  if (pick) sel.dst = pick;
  else if (!sel.dst) {
    if (!typed.trim()) return;
    if (!b.cb.open) b.cb.refresh();
    if (!b.cb.accept()) { b.input.classList.add('pb-input--invalid'); return; }
  } else typed = '';
  if (!chosen(sel.dst)) dsts.push(sel.dst);
  sel.dst = null;
  b.input.value = typed;
  kept.set(true, typed);
  b.input.classList.remove('pb-input--invalid');
  renderChips();
  renderTally();
  b.cb.refresh();
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

// ---- the last take's source, destinations or levels (Ctrl+S/D/F)

// The last take's value for each box, as a chip by its label that does
// what Ctrl+S/D/F does. Hidden when the box already has it, and for
// levels when they're the default.
function renderLast() {
  if (!S.cfg) return;
  const now = lastChip(lastTake(), { src: sel.src, dsts: allDsts(), levels: lvl.levels() || [] });
  for (const k of ['src', 'dst', 'lvl']) {
    const slot = document.querySelector(`[data-last="${k}"]`);
    const v = now[k];
    if (!v) { slot.replaceChildren(); continue; }
    const key = { src: 's', dst: 'd', lvl: 'f' }[k];
    const b = el('button', 'pb-chip sode__last', el('span', 'sode__last-text', 'Last: ', ...v), el('kbd', 'sode__key-note', `⌃${key}`));
    b.type = 'button';
    b.tabIndex = -1;
    b.title = `Last take's ${{ src: 'source', dst: 'destination', lvl: 'levels' }[k]} (Ctrl+${key})`;
    b.addEventListener('mousedown', (e) => e.preventDefault());
    b.addEventListener('click', () => useLast(k));
    slot.replaceChildren(b);
  }
}
on('history', renderLast);

// Fills one box from the last take and moves on: source to destination,
// destination back to source, but only if that one is empty (no pick, chips
// or typing); otherwise on to levels. Levels stays.
function useLast(k) {
  const t = lastTake();
  if (!t) return showResult('info', 'No take yet', 'There\'s no earlier take to reuse.');
  if (k === 'src') {
    select('src', t.src);
    const dstEmpty = !sel.dst && !dsts.length && !boxes.dst.input.value.trim();
    focusBox(dstEmpty ? boxes.dst.input : lvlInput);
  } else if (k === 'dst') {
    // Adds to the destinations already there; the last one added lands
    // in the box.
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

// Under each box: where the source is sending, and what the destinations
// are receiving, as level bars like the history's. Both show at once, on a
// row of their own under their boxes (the flip swaps them). Long lists scroll.
// The arrow points down for what the source sends, up for what the
// destinations receive.
function underHead(text, up) {
  return el('div', 'sode__under-head', el('span', up ? 'sode__down-arrow sode__down-arrow--up' : 'sode__down-arrow'), el('span', 'sode__label', text));
}

// One row per destination src is on, with the levels it's on there; the
// name adds it to the destinations (the newest lands in the box).
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

// A source in Receiving becomes the source.
function pickSrc(n) {
  select('src', srcItem(n));
  lvlInput.focus();
}

function renderTally() {
  renderLast();
  const srcUnder = $('src-under'), dstUnder = $('dst-under');
  if (!S.cfg) { srcUnder.replaceChildren(); return dstUnder.replaceChildren(); }
  // Redrawing the lists keeps where they were scrolled to.
  const scrolled = [srcUnder, dstUnder].map((u) => u.querySelector('.sode__scroll')?.scrollTop || 0);
  if (sel.src) {
    const levels = lvl.levels() || allLevels();
    const fed = fedBy(sel.src.n, levels);
    srcUnder.replaceChildren(underHead(fed.length ? `Sending to ${fed.length}` : 'Sending'), fed.length
      ? el('div', 'sode__scroll', ...fed.map((d) => sendingRow(sel.src.n, d, levels)))
      : el('div', 'sode__hint', 'Not sending to any destination on these levels'));
  } else srcUnder.replaceChildren();
  const ds = allDsts();
  if (ds.length) {
    const srcs = new Set(ds.flatMap((d) => bySource(d.n, allLevels()).map((r) => r.src)).filter((s) => s != null && s >= 0));
    dstUnder.replaceChildren(underHead(srcs.size ? `Receiving from ${srcs.size}` : 'Receiving', true),
      el('div', 'sode__scroll', ...ds.flatMap((d) => currentlyNodes(d, pickSrc))));
  } else dstUnder.replaceChildren();
  [srcUnder, dstUnder].forEach((u, i) => {
    const box = u.querySelector('.sode__scroll');
    if (box) box.scrollTop = scrolled[i];
  });
}

// ---- history

function histRow(e) {
  const src = srcItem(e.src, e.srcName), dst = dstItem(e.dst, e.dstName);
  const known = e.levels.filter((u) => S.levelBy.has(u));

  const btn = e.chain
    ? undoButton(undoPlan(e), `Undo ${e.chain.length} changes on ${dst.label}`, openConfirm, { text: `Undo ${e.chain.length}` })
    : undoButton(undoPlan(e), `Undo ${src.label} to ${dst.label}`, openConfirm);
  const retry = retryButton(!e.ok, `Retry ${src.label} to ${dst.label}`, () => openConfirm(retryPlan(e)), { placeholder: true });

  // Before: what the destination had on these levels just before the take.
  const open = expanded.has(e.id);
  const detailId = `hist-before-${e.id}`;
  const disc = disclosure(open, 'Before', detailId, () => {
    toggle(expanded, e.id);
    renderHistory();
  });

  const row = el('li', 'pb-input sode__hist-row' + (arrived.entry(e) ? ' sode--enter' : '') + curClass(`e${e.id}`),
    el('span', 'sode__hist-id', `#${e.id}`),
    histTime(e.time),
    levelBar(known),
    el('span', 'sode__hist-route sode__hist-route--boxes', ...routeBoxes(src, dst), entryNote(e)),
    statusNode({ e }),
    disc,
    el('span', 'sode__hist-actions', retry, btn));
  row.dataset.key = `e${e.id}`;
  clickToToggle(row, disc, '.sode__hist-before');
  if (open) {
    const detail = el('div', 'sode__hist-before',
      ...(e.chain
        ? [el('div', 'sode__label', `Changes on ${dst.label}`), ...stackSteps(e, openConfirm, pickSrc)]
        : [el('div', 'sode__label', `Before #${e.id}, on ${dst.label}`), ...beforeGroups(e).map((g) => srcRow(g.levels, tallyText(g.src), g.src != null && g.src >= 0 ? () => pickSrc(g.src) : null))]),
      ...failureRows(e),
      ...(e.follows && !e.chain ? [el('div', 'sode__route', followBoxes(e.follows, e.levels))] : []));
    // Below 1000px the row's level bar moves in here (see history.css).
    detail.prepend(el('div', 'sode__hist-levels', el('div', 'sode__label', 'Levels'), levelBar(known)));
    detail.id = detailId;
    row.append(detail);
  }
  return row;
}

// A row's source → destination as boxes, as in the Before panel. src and
// dst are items, or nodes for a folder of several. Clicking an item's box
// fills that patch box and moves on, as Ctrl+S/D do.
const routeBoxes = (src, dst) => [routeBox('src', src), el('span', 'sode__arrow', '→'), routeBox('dst', dst)];
function routeBox(k, it) {
  const cls = `sode__end sode__end--${k}`;
  if (it instanceof Node) return el('span', cls, it);
  const b = el('button', `${cls} sode__end--pick`, labelNode(it));
  b.type = 'button';
  b.tabIndex = -1;
  b.title = `Use as ${k === 'src' ? 'source' : 'destination'}`;
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', () => {
    select(k, it);
    focusBox(k === 'src' ? boxes.dst.input : boxes.src.input);
  });
  return b;
}

// A folder: one confirm's takes under a summary row with the master status,
// Undo all and (if any failed) Retry for the failed ones.
function folderRow(f) {
  const open = openFolders.has(f.group);
  const kidsId = `hist-folder-${f.group}`;
  const title = folderTitle(f);
  const failed = f.entries.filter((e) => !e.ok).length;
  // The folder's one source (or destination) as an item, if it has one.
  const one = (k, item) => (new Set(f.entries.map((e) => e[k])).size === 1 ? item(f.entries[0][k], f.entries[0][`${k}Name`]) : null);

  const undo = undoButton(folderUndoPlan(f), `Undo all ${f.entries.length} takes of ${folderIds(f)}`, openConfirm,
    { text: folderPartial(f) ? `Undo all ${f.entries.length}` : 'Undo all' });
  const retry = retryButton(failed, `Retry the ${failed} failed take${failed === 1 ? '' : 's'} of ${folderIds(f)}`,
    () => openConfirm(folderRetryPlan(f)), { placeholder: true });

  const disc = disclosure(open, `${f.shown.length}`, kidsId, () => {
    toggle(openFolders, f.group);
    autoOpen.delete(f.group); // opened by hand: stays open
    renderHistory();
  });
  disc.setAttribute('aria-label', `${open ? 'Hide' : 'Show'} the ${f.shown.length} takes`);

  const row = el('li', 'pb-input sode__hist-row' + (arrived.folder(f, open) ? ' sode--enter' : '') + curClass(`f${f.group}`),
    el('span', 'sode__hist-id', folderIds(f)),
    histTime(f.entries[0].time),
    levelBar(folderLevels(f)),
    el('span', 'sode__hist-route sode__hist-route--boxes', ...routeBoxes(one('src', srcItem) || title[0].firstChild, one('dst', dstItem) || title[2].firstChild), ...title.slice(3)),
    statusNode({ f }),
    disc,
    el('span', 'sode__hist-actions', retry, undo));
  row.dataset.key = `f${f.group}`;
  clickToToggle(row, disc, '.sode__hist-folder');
  if (open) {
    const kids = el('ul', 'sode__hist-folder', ...f.shown.map(histRow), ...followerRows(f));
    kids.id = kidsId;
    row.append(kids);
  }
  return row;
}

// The "Followers" row at the bottom of a folder, and (opened) the
// followers inside it: changes made elsewhere that seem to follow the
// confirm, not any one take. Same columns as a take's row, and a stop for
// the keyboard cursor like a folder (key w<group>).
function followerRows(f) {
  if (!f.followers?.length) return [];
  const open = openFollowers.has(f.group);
  const kidsId = `hist-followers-${f.group}`;
  const key = `w${f.group}`;
  const disc = disclosure(open, `${f.followers.length}`, kidsId, () => {
    toggle(openFollowers, f.group);
    renderHistory();
  });
  disc.setAttribute('aria-label', `${open ? 'Hide' : 'Show'} the ${f.followers.length} followers`);
  const head = el('li', 'pb-input sode__hist-row' + curClass(key),
    el('span', 'sode__hist-id'), el('span', 'sode__hist-time'), el('span'),
    el('span', 'sode__hist-route', 'Followers'),
    el('span'), disc, el('span', 'sode__hist-actions'));
  head.dataset.key = key;
  clickToToggle(head, disc, '.sode__hist-folder');
  if (open) {
    const kids = el('ul', 'sode__hist-folder', ...f.followers.map(histRow));
    kids.id = kidsId;
    head.append(kids);
  }
  return [head];
}

// A row's date and time; below 1000px the date hides and shows on hover.
function histTime(iso) {
  const t = el('span', 'sode__hist-time', ...timeNodes(iso));
  t.dataset.hint = t.textContent;
  return t;
}

// Marks the keyboard cursor's row.
const curClass = (key) => (key === cursor ? ' sode__hist-row--cur' : '');

const pager = historyPager(10, () => layout() === 'desktop');
pullToLoad(window, $('history-more'), pager, () => active() && !document.querySelector('dialog[open]'));

// scroll: bring the cursor row into view (the cursor just moved). Other
// redraws (someone else's take, a result) leave the page where it is.
function renderHistory(scroll = false) {
  $('history-bar').hidden = !S.hist.length;
  $('history-search').hidden = !S.hist.length;
  const list = pager.list();
  if (cursor && !stops(list).some((x) => x.key === cursor)) {
    // Its row left the page (newer patches pushed it off): drop the
    // cursor and close the folders it had opened.
    cursor = null;
    for (const g of autoOpen) openFolders.delete(g);
    autoOpen.clear();
  }
  histList.replaceChildren(...list.map((r) => (r.f ? folderRow(r.f) : histRow(r.e))));
  $('history-more').replaceChildren(...[emptyNote(list, pager.fetching()), pager.button()].filter(Boolean));
  if (scroll === true && keyMode()) histList.querySelector('.sode__hist-row--cur')?.scrollIntoView({ block: 'nearest' });
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
    if (!openFolders.has(r.f.group)) return;
    for (const e of r.f.shown) out.push({ key: `e${e.id}`, e, top, in: r.f });
    // The Followers row, and its rows when open.
    if (!r.f.followers?.length) return;
    out.push({ key: `w${r.f.group}`, head: r.f, top, in: r.f });
    if (openFollowers.has(r.f.group)) for (const e of r.f.followers) out.push({ key: `e${e.id}`, e, top, in: r.f, follower: true });
  });
  return out;
}
const curStop = (list = stops()) => list.find((x) => x.key === cursor);

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
  if (redraw || !to) return renderHistory(true);
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
    const kids = stops().filter((x) => x.in?.group === next.f.group);
    moveTo(kids[kids.length - 1]);
  }
}

// K: back to the latest take (or the folder holding it), at the top.
function latest() {
  const first = stops()[0];
  if (first) moveTo(first);
}

// J: the next folder down, skipping single takes. Past the last loaded
// one, load more; the next press moves on to it.
function nextGroup() {
  const list = stops();
  const s = curStop(list);
  const next = list.find((x) => x.f && (!s || x.top > s.top));
  if (next) moveTo(next);
  else loadMore();
}

// l: open a take's Before or a folder. h: close it, or go up to the folder.
function open(on) {
  const s = curStop();
  if (!s) return;
  if (s.head) {
    // l opens the Followers row, h closes it, or goes up to the folder.
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
  if (s.f && s.f.entries.some((e) => !e.ok)) openConfirm(folderRetryPlan(s.f));
  else if (s.e && !s.e.ok) openConfirm(retryPlan(s.e));
  else showResult('info', 'Nothing to retry', 'Only failed takes can be retried.');
}

// key: put the cursor on that row (Esc from one of its buttons).
function enterKeys(key) {
  // Works on an empty list too: s/d/f, `,`, `/`, `?` and Esc still apply.
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
  if (k === '/') { openFilters(); return true; }
  if (k === 'S' || k === 'D') { focusSearch(k === 'S' ? 'src' : 'dst'); return true; }
  if (k === '?') { openKeys(); return true; }
  if (k === 'Escape') { focusBox(boxes.src.input); return true; }
  const act = {
    j: () => step(1), ArrowDown: () => (e.shiftKey ? nextGroup() : step(1)),
    k: () => step(-1), ArrowUp: () => (e.shiftKey ? latest() : step(-1)),
    J: nextGroup, K: latest,
    l: () => open(true), ArrowRight: () => open(true), Enter: () => open(!e.shiftKey),
    h: () => open(false), ArrowLeft: () => open(false),
    u: () => { const s = curStop(); if (s && (s.f || s.e)) undoRow(s.f ? { f: s.f } : { e: s.e }); },
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

// ---- Source/Destination order: the arrow flips how they're shown (per
// browser; index.html applies it before first paint)

function showFlip() {
  const on = flipped();
  $('flip').setAttribute('aria-pressed', String(on));
  $('flip').dataset.tip = on ? 'Show Source first' : 'Show Destination first';
}
$('flip').addEventListener('mousedown', (e) => e.preventDefault());
$('flip').addEventListener('click', () => setFlip(!flipped()));
on('flip', showFlip);
showFlip();

// ---- shortcut tip: a note in the corner until dismissed (per browser)

const TIP_KEY = 'sode-hint-keys';
$('keys-tip-text').textContent = `${mac ? '⌘z' : 'Ctrl+z'} undoes the last Sode take · ${mac ? '⌃?' : 'Ctrl+?'} for shortcuts`;
$('keys-tip').hidden = readStr(TIP_KEY) === 'hidden';
$('keys-tip-close').addEventListener('click', () => {
  $('keys-tip').hidden = true;
  writeStr(TIP_KEY, 'hidden');
});

// ---- shortcut list (?)

// Closing from the keyboard returns to keyboard mode if it was on when the
// list opened; closing with the mouse never does (a click leaves it).
let keysBack = null;
let keysWere = false;
let keysFromHist = false; // opened from Recent, whose rows may be redrawn meanwhile
function openKeys() {
  keysBack = document.activeElement;
  keysWere = keyMode();
  keysFromHist = histArea(keysBack);
  $('keys').hidden = false;
  $('keys-dialog').focus();
}
function closeKeys(viaKeys = false) {
  $('keys').hidden = true;
  if (keysWere && viaKeys) enterKeys();
  else if (keysBack?.isConnected && !keysFromHist) keysBack.focus();
  else focusBox(boxes.src.input);
}
// Mac labels; a key that only a Mac has (⌃Z beside ⌘Z) is hidden elsewhere.
if (mac) for (const k of $('keys').querySelectorAll('[data-mac]')) { k.textContent = k.dataset.mac; k.hidden = false; }

// The Filter by Source / Destination boxes above Recent.
function focusSearch(k) {
  const f = $('history-search').querySelector(`[data-search-${k}]`);
  if ($('history-search').hidden) return;
  f.focus();
  f.select();
}
$('keys-close').addEventListener('click', () => closeKeys());

// ---- confirm

const KEYS = 'abcdefghjkmnpqrstuvwxyz';
let lastKey = '';
let pending = null; // plan + key + lock (a shallow copy: planBlocked's prot is keyed by its items)
let confirmBack = null; // focus to return to when the confirm closes
let confirmKeys = false; // keyboard mode was on when it opened
let confirmFromHist = false; // opened from Recent (decided then: rows get redrawn)

// Patch button and Enter: take any half-typed box's best match, then confirm.
function patchNow() {
  for (const k of ['src', 'dst']) {
    const b = boxes[k];
    if (!sel[k] && b.input.value.trim() && !(k === 'dst' && keptText())) {
      if (!b.cb.open) b.cb.refresh();
      b.cb.accept();
    }
  }
  tryConfirm();
}

// Undo the newest take made in Sode (a whole folder), through the usual
// confirm. Changes from other panels are skipped, whatever the filters
// show: they're someone else's.
function undoLast() {
  const last = S.hist.find((x) => !x.ext);
  if (!last) return showResult('info', 'Nothing to undo', 'No takes made in Sode yet.');
  const e = attachFollows(last);
  const takes = e.group && e.groupN > 1 && S.hist.filter((x) => x.group === e.group).map(attachFollows);
  undoRow(takes ? { f: { group: e.group, n: e.groupN, entries: takes, shown: takes } } : { e });
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
    if (!sel[k] && (k === 'src' || !dsts.length || (b.input.value.trim() && !(k === 'dst' && keptText())))) {
      b.input.classList.add('pb-input--invalid');
      b.input.focus();
      return;
    }
  }
  const levels = lvl.levels();
  if (!levels) { lvlInput.classList.add('pb-input--invalid'); lvlInput.focus(); return; }
  openConfirm(patchPlan(sel.src, allDsts(), levels));
}

function openConfirm(plan) {
  const { block, prot } = planBlocked(plan);
  if (block) return showResult(block.kind, block.title, block.text);
  pending?.lock.cancel(); // one replaced: its lockout mustn't unlock this one
  let key;
  do key = KEYS[Math.floor(Math.random() * KEYS.length)]; while (key === lastKey);
  lastKey = key;
  pending = { ...plan, key };
  confirmBack = document.activeElement;
  confirmKeys = keyMode();
  confirmFromHist = confirmBack === histList || histArea(confirmBack);

  const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
  const verb = planVerb(plan);
  const Verb = verb[0].toUpperCase() + verb.slice(1);
  $('confirm-title').textContent = `Confirm ${verb}`;
  $('confirm-body').replaceChildren(
    ...planSummary(plan, { prot, spaceHint: true }),
    el('div', 'sode__prompt', 'Press ', keyEl, S.cfg.confirmEnter ? ' or Enter' : '', ' to confirm, or escape to cancel.'),
  );
  $('confirm-ok').textContent = `${Verb} (${key.toUpperCase()})`;
  $('confirm-ok').disabled = true;
  $('confirm').hidden = false;
  $('confirm-dialog').focus();

  pending.lock = lockout(() => {
    keyEl.classList.remove('sode__key--locked');
    $('confirm-ok').disabled = false;
  });
}

// Focus goes back where the confirm was opened from. Keyboard mode only if
// it was on and the confirm was answered from the keyboard (a click leaves
// it). From Recent otherwise (a row's button, or the list on browsers
// that don't focus clicked buttons) it goes to Source, leaving the boxes
// as they are: the row is redrawn with the result.
function focusBack(viaKeys) {
  if (confirmKeys && viaKeys) enterKeys();
  else if (!confirmFromHist && confirmBack?.isConnected) confirmBack.focus();
  else focusBox(boxes.src.input);
}
function histArea(n) {
  return !!n && (histList.contains(n) || $('history-more').contains(n));
}

function closeConfirm(viaKeys = false) {
  const patch = pending?.verb === 'patch';
  pending?.lock.cancel();
  pending = null;
  $('confirm').hidden = true;
  if (patch) lvlInput.focus();
  else focusBack(viaKeys);
}

function doTake(viaKeys = false) {
  const p = pending;
  p.lock.cancel();
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
  // Space ticks the box that adds an undo's follows.
  if (e.key === ' ' && pending.extra) {
    $('confirm-body').querySelector('.sode__plan-extra input')?.click();
    return;
  }
  if (!pending.lock.ready()) return;
  if (e.key.toLowerCase() === pending.key || (S.cfg.confirmEnter && e.key === 'Enter')) doTake(true);
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
  // Coming back to unconfirmed text reopens its matches.
  b.input.addEventListener('focus', () => { if (!sel[k]) b.cb.refresh(); });
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

// Modifiers, on every OS: Control is for Sode's commands and moving through
// match lists (Mac too, where it is free of browser shortcuts), Option/Alt
// only toggles the theme, and ⌘ is left to the OS and the browser, but for
// ⌘Z and ⌘J/K, which work as Ctrl+Z and Ctrl+J/K do. Each check excludes the other two
// modifiers. Key labels are lowercase, with ⇧ where Shift is needed.
document.addEventListener('keydown', (e) => {
  if (pending) return onConfirmKey(e);
  if (layout() !== 'desktop') return;
  // Ctrl+? opens or closes the shortcut list from anywhere on desktop,
  // Settings included, even mid-typing. By character, so any keyboard
  // layout's ?; by the / key too, for browsers that report e.key without
  // Shift under Ctrl.
  if (e.ctrlKey && !e.metaKey && !e.altKey && (e.key === '?' || (e.shiftKey && e.code === 'Slash'))) {
    e.preventDefault();
    e.stopImmediatePropagation(); // not Settings' own keys
    if ($('keys').hidden) openKeys();
    else closeKeys(true);
    return;
  }
  if (!$('keys').hidden) {
    e.stopImmediatePropagation(); // Esc here closes the list, not Settings
    if (e.key === 'Escape' || e.key === '?') { e.preventDefault(); closeKeys(true); }
    // Keep focus in the dialog: Close is its only control.
    if (e.key === 'Tab') { e.preventDefault(); $('keys-close').focus(); }
    return;
  }
  if (!active()) return;
  // Ctrl+, opens Settings from anywhere (⌘, is the browser's own).
  if (e.ctrlKey && !e.metaKey && !e.altKey && e.key === ',') {
    e.preventDefault();
    $('settings-open').click();
    return;
  }
  // Ctrl+S/D/F: the last take's source, destinations or levels, from
  // anywhere.
  const last = e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey && { KeyS: 'src', KeyD: 'dst', KeyF: 'lvl' }[e.code];
  if (last) {
    e.preventDefault();
    useLast(last);
    return;
  }
  const id = e.target.id;
  const k = id === 'src' || id === 'dst' ? id : null;
  const b = k && boxes[k];
  const inBox = k || id === 'lvl';

  // Ctrl+Z undoes the last Sode take, and so does ⌘Z on a Mac. Whichever is
  // the OS text undo (⌘Z on a Mac, Ctrl+Z elsewhere) leaves a box with text
  // to undo its typing.
  const cmdZ = mac && e.metaKey && !e.ctrlKey;
  if ((cmdZ || (e.ctrlKey && !e.metaKey)) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'z') {
    if (cmdZ === mac && e.target instanceof HTMLInputElement && e.target.value) return;
    e.preventDefault();
    undoLast();
    return;
  }

  // The match list's keys: ↑/↓, PgUp/PgDn, Ctrl+J/K (and ⌘J/K on a Mac).
  if (b && b.cb.nav(e)) return;
  // Ctrl+Enter and ⌘Enter add another destination too.
  if (k === 'dst' && e.key === 'Enter' && (e.ctrlKey !== e.metaKey) && !e.shiftKey && !e.altKey) {
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
  // Backspace in an empty destination box takes the last chip back to edit.
  if (k === 'dst' && e.key === 'Backspace' && !b.input.value && dsts.length) {
    e.preventDefault();
    select('dst', dsts.pop());
    return;
  }
  if (e.key === 'Tab') {
    // Leaving a box either way takes its best match, like Enter does.
    if (b && !sel[k] && b.input.value.trim() && !(k === 'dst' && keptText()) && !b.cb.accept()) {
      e.preventDefault();
      b.input.classList.add('pb-input--invalid');
      return;
    }
    if (inBox) {
      // One ring, wrapping both ways: Source, Destination, Levels (Destination
      // first when flipped).
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
    if (b && b.cb.open) { b.cb.close(); return; }
    if (e.target.value) {
      if (k) select(k, null);
      else lvl.clear();
      renderTally();
    } else if (!dsts.length && !boxes.src.input.value && !boxes.dst.input.value && lvl.empty()) {
      enterKeys(); // Esc with everything empty: keyboard mode
    } else {
      clearAll();
    }
  }
});

$('dst-add').addEventListener('mousedown', (e) => e.preventDefault());
$('dst-add').addEventListener('click', () => addDst());
$('patch').addEventListener('mousedown', (e) => e.preventDefault()); // keep focus where it was
$('patch').addEventListener('click', patchNow);
$('confirm-cancel').addEventListener('click', () => closeConfirm());
$('confirm-ok').addEventListener('click', () => {
  if (pending && pending.lock.ready()) doTake();
});

// ---- subscriptions

on('hello', () => {
  pager.reset();
  lvl.reset();
  renderTally();
  renderHistory();
});
on('names', () => {
  for (const k of ['src', 'dst']) if (document.activeElement === boxes[k].input && !sel[k]) boxes[k].cb.refresh();
  renderTally();
});
on('tally', renderTally);
on('conn', renderConn);
on('history', renderHistory);
on('result', (r) => $('result').replaceChildren(resultNode(r)));
