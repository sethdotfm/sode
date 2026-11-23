// Desktop History list: rows, Groups, External follows, the pager and keyboard mode (single keys on the list).

import { $, el, S, on, emit, tallyText, labelNode, toggle, srcItem } from './core.js';
import { listStep, pickOrAdd } from './keys.js';
import { levelBar, levelsShown } from './levelview.js';
import { srcRow } from './routes.js';
import { timeNodes, compactOn } from './device.js';
import { historyPager, beforeGroups, attachFollows, entryNote, undoneBadge, statusNode, groupLevels, groupIds, routeNote } from './history.js';
import { planEmpty, showResult } from './plans.js';
import { backStep, forwardStep, ownTakes, redoSteps, keepRedo, entryKey } from './stepback.js';
import { layout } from './layout.js';
import { openFilters } from './filters.js';
import { emptyNote } from './searchbox.js';
import { rowView, rowActions, disclosureAria, followersHead } from './historyrows.js';
import { isPicked, picking, pickBox, togglePick, bindPick } from './historypick.js';
import {
  byLabel,
  disclosure,
  undoButton,
  againButton,
  stackSteps,
  stackFrom,
  followBoxes,
  failureRows,
  clickToToggle,
  pullToLoad,
  arrivals,
} from './widgets.js';

// From desktop.js: select(k, item), focusBox(input), openConfirm(plan), focusSearch(k), openKeys() (built after this),
// boxes, active() and protect(set), the route view's Protect key (false when it offers none). The module owns the list state: expanded rows, open Groups and External follows, the arrival
// marks, the keyboard cursor.
export function desktopHistory({ select, addEnd, focusBox, openConfirm, focusSearch, openKeys, boxes, active, protect }) {
  const expanded = new Set();
  const openGroups = new Set();
  const openFollowers = new Set();
  const arrived = arrivals();

  function histRow(e) {
    const view = rowView({ e });
    const r = view.route(e);
    const dst = r.dst;
    const known = e.levels.filter((u) => S.levelBy.has(u));

    const acts = view.actions(e);
    const btn = undoButton(acts.undo.plan, acts.undo.label, openConfirm, { text: acts.undo.text, tip: acts.undo.tip });
    const retry = againButton(acts.again.plan, acts.again.label, openConfirm);

    const open = expanded.has(e.id);
    const detailId = `hist-before-${e.id}`;
    const groups = beforeGroups(e);
    const disc = disclosure(open, e.chain ? `${e.chain.length}` : view.disc(e, groups), detailId, () => {
      toggle(expanded, e.id);
      renderHistory();
    });
    if (e.chain) disc.setAttribute('aria-label', disclosureAria(open, e.chain.length, 'changes'));

    const row = el(
      'li',
      'pb-input sode__hist-row' + (arrived.entry(e) ? ' sode--enter' : '') + curClass(`e${e.id}`),
      el('span', 'sode__hist-id', e.chain ? `#${e.chain[e.chain.length - 1].id}–#${e.id}` : `#${e.id}`),
      histTime(e.time),
      levelBar(known),
      el(
        'span',
        'sode__hist-route sode__hist-route--boxes',
        ...(e.chain ? stackFrom(e, useRowSrc) : []),
        ...routeNodes(r),
        entryNote(e),
        undoneBadge(e.undone),
        byLabel(e),
      ),
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
          ? [
              el('div', 'sode__label', `Changes on ${dst.label}`),
              ...stackSteps(e, { restore: openConfirm, pickSrc: useRowSrc, pickDst: useRowDst, addDst: addEnd }),
            ]
          : [el('div', 'sode__label', view.detail(e, dst, groups)), ...groups.map((g) => srcRow(g.levels, tallyText(g.src), g.src, useRowSrc))]),
        ...failureRows(e),
        ...(e.follows && !e.chain ? [el('div', 'sode__route', followBoxes(e.follows, e.levels, useRowDst, addEnd))] : []),
      );
      // Below 1000px the row's level bar moves in here (see history.css).
      detail.prepend(el('div', 'sode__hist-levels', el('div', 'sode__label', 'Levels'), levelBar(known)));
      detail.id = detailId;
      row.append(detail);
    }
    return row;
  }

  // r: rowView's route. A lead (a Protect chip) stands where the source box is, with no arrow.
  const words = (x) => (typeof x === 'string' ? document.createTextNode(x) : x);
  const routeNodes = (r) => [
    ...(r.lead ? [r.lead] : [routeBox('src', words(r.src)), el('span', 'sode__arrow', '→')]),
    routeBox('dst', words(r.dst), r.many),
    routeNote(r.notes),
  ];
  // all: the Group's destinations; a chip that names several adds them all, with any click.
  function routeBox(k, it, all) {
    const cls = `sode__end sode__end--${k}`;
    if (it instanceof Node && all?.length) {
      const b = el('button', `${cls} sode__end--pick`, it);
      b.type = 'button';
      b.tabIndex = -1;
      b.dataset.pbTip = 'Add all as destinations';
      b.addEventListener('mousedown', (e) => e.preventDefault());
      pickOrAdd(
        b,
        () => all.forEach(addEnd),
        () => all.forEach(addEnd),
      );
      return b;
    }
    if (it instanceof Node) return el('span', cls, it);
    const b = el('button', `${cls} sode__end--pick`, labelNode(it));
    b.type = 'button';
    b.tabIndex = -1;
    b.dataset.pbTip = `Use as ${k === 'src' ? 'source' : 'destination'}`;
    b.addEventListener('mousedown', (e) => e.preventDefault());
    pickOrAdd(b, () => useEnd(k, it), k === 'dst' && (() => addEnd(it)));
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

  function groupRow(f) {
    const open = openGroups.has(f.group);
    const kidsId = `hist-folder-${f.group}`;
    const view = rowView({ f });
    const acts = view.actions(f);
    const undo = undoButton(acts.undo.plan, acts.undo.label, openConfirm, { text: acts.undo.text, tip: acts.undo.tip });
    const retry = againButton(acts.again.plan, acts.again.label, openConfirm);

    const disc = disclosure(open, `${f.shown.length}`, kidsId, () => {
      toggle(openGroups, f.group);
      autoOpen.delete(f.group); // opened by hand: stays open
      renderHistory();
    });
    disc.setAttribute('aria-label', disclosureAria(open, f.shown.length, view.noun));

    const row = el(
      'li',
      'pb-input sode__hist-row' + (arrived.group(f, open) ? ' sode--enter' : '') + curClass(`f${f.group}`),
      el('span', 'sode__hist-id', groupIds(f)),
      histTime(f.entries[0].time),
      levelBar(groupLevels(f)),
      el('span', 'sode__hist-route sode__hist-route--boxes', ...routeNodes(view.route(f)), undoneBadge(f.undone), byLabel(f)),
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

  // External follows: Externals that seem to follow the confirm. A keyboard cursor stop, as a Group is (key w<group>).
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
      el('span', 'sode__hist-route', 'External follow entries'),
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

  // scroll: only when the cursor moved; other redraws leave the page where it is.
  function renderHistory(scroll = false) {
    const list = pager.list();
    if (cursor && !stops(list).some((x) => x.key === cursor)) {
      // Its row left the page: drop the cursor and close the Groups it opened.
      cursor = null;
      for (const g of autoOpen) openGroups.delete(g);
      autoOpen.clear();
    }
    histList.replaceChildren(...list.map((r) => (r.f ? groupRow(r.f) : histRow(r.e))));
    $('history-more').replaceChildren(...[emptyNote(list, pager.fetching()), pager.button()].filter(Boolean));
    if (scroll === true && keyMode()) histList.querySelector('.sode__hist-row--cur')?.scrollIntoView({ block: 'nearest' });
  }

  // Keyboard mode: focus on the History list, where single keys act. Cursor is 'e<id>' on a take, 'f<group>' on a
  // Group. Keyboard entry only; any click leaves it.

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
  // Keep in step with the Compact view rule in desktop.css.
  const historyShown = () => !compactOn() || document.documentElement.dataset.sodePick === '';
  // Compact view turned on mid keyboard mode: History is gone, so focus goes back to Source.
  on('prefs', () => {
    if (historyShown() || !histArea(document.activeElement)) return;
    setKeys(false);
    focusBox(boxes.src.input);
  });
  document.addEventListener(
    'pointerdown',
    (e) => {
      setKeys(false);
      // A click on a row moves the hidden cursor there, so the next key starts on it.
      const row = histList.contains(e.target) && e.target.closest('[data-key]');
      if (row) cursor = row.dataset.key;
    },
    true,
  );

  // top: the index of the row (or of its Group) in the pager's list.
  function stops(list = pager.list()) {
    const out = [];
    list.forEach((r, top) => {
      if (r.e) return out.push({ key: `e${r.e.id}`, e: r.e, top });
      out.push({ key: `f${r.f.group}`, f: r.f, top });
      if (!openGroups.has(r.f.group)) return;
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
        openGroups.delete(g);
        autoOpen.delete(g);
        redraw = true;
      }
    }
    if (s.f && !openGroups.has(s.f.group)) {
      openGroups.add(s.f.group);
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
      openGroups.add(s.f.group);
      autoOpen.delete(s.f.group);
    } else {
      openGroups.delete(s.f.group);
      autoOpen.delete(s.f.group);
    }
    renderHistory();
  }

  function retryCur() {
    const s = curStop();
    if (!s) return;
    const plan = rowActions(s.f ? { f: s.f } : { e: s.e }).again.plan;
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
    const jk = { 1: 'j', [-1]: 'k' }[listStep(e)] || '';
    if (!jk && (e.ctrlKey || e.metaKey || e.altKey)) return false;
    const k = jk || e.key;
    if (BOXES[k] && (k !== 'f' || levelsShown())) {
      focusBox($(BOXES[k]));
      return true;
    }
    if (k === ',') {
      emit('settingsOpen');
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
      openKeys();
      return true;
    }
    if (k === 'p') {
      emit('presetStrip');
      return true;
    }
    // x Protects the route view's destination, Shift+x frees Sode's Protect; only in Keyboard mode.
    if ((k === 'x' || k === 'X') && keyMode()) return protect(k === 'x');
    if (k === 'Escape') {
      focusBox(boxes.src.input);
      return true;
    }
    // While takes are picked for a preset, Space/Enter ticks the row and nothing else acts.
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
  // Ctrl+Z: undo the newest take this tab sent that nothing has put back yet (a whole Group), one further back each press
  // (backStep in stepback.js). Externals and other browsers' takes are never a step, whatever the filters show.
  // A take whose Undo has nothing to send is passed over, as the panel Undo cell does.
  function undoLast() {
    const step = backStep(S.hist, ownTakes(), redoSteps(), (entries) => planEmpty(planOf(rowOf(entries), 'undo')));
    if (!step) return showResult('info', 'Nothing to undo', 'No take from this browser is left to undo.');
    openConfirm({ ...planOf(rowOf(step.entries), 'undo'), step: 'back', stepIds: step.entries.map(entryKey) });
  }

  // Ctrl+Shift+Z: redo the newest take that Ctrl+Z undid in this tab, as the row's Redo does.
  function redoLast() {
    const { entries, keep } = forwardStep(S.hist, redoSteps());
    keepRedo(keep);
    if (!entries) return showResult('info', 'Nothing to redo', 'No take from this browser is left to redo.');
    const row = rowOf(entries);
    const plan = planOf(row, 'again');
    if (planEmpty(plan)) return showResult('warn', 'Nothing to redo', rowView(row).empty(row.f || row.e));
    openConfirm({ ...plan, step: 'forward' });
  }

  // One entry is a row of its own; more are its Group.
  function rowOf(entries) {
    const takes = entries.map(attachFollows);
    const e = takes[0];
    return takes.length > 1 ? { f: { group: e.group, n: e.groupN, entries: takes, shown: takes } } : { e };
  }

  const planOf = (row, act) => rowActions(row)[act].plan;

  function undoRow(row) {
    const view = rowView(row);
    const x = row.f || row.e;
    const plan = view.actions(x).undo.plan;
    if (planEmpty(plan)) return showResult('warn', 'Nothing to undo', view.empty(x));
    openConfirm(plan);
  }

  // The Palette's Undo and Redo rows, and the Grid popup.
  on('undoLast', undoLast);
  on('redoLast', redoLast);

  function histArea(n) {
    return !!n && (histList.contains(n) || $('history-more').contains(n));
  }

  return { renderHistory, enterKeys, keyMode, historyShown, histList, histArea, onListKey, undoLast, redoLast, pager, useEnd };
}
