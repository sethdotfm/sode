// Desktop Recent list: history rows, folders, followers, the pager and keyboard mode (single keys on the list).

import { $, el, S, on, emit, tallyText, labelNode, toggle, srcItem, dstItem } from './core.js';
import { mac } from './keys.js';
import { levelBar } from './levelview.js';
import { srcRow } from './routes.js';
import { timeNodes } from './device.js';
import { historyPager, beforeGroups, attachFollows, entryNote, statusNode, folderTitle, folderLevels, folderIds } from './history.js';
import { rowUndoPlan, planEmpty, showResult, againPlan } from './plans.js';
import { layout } from './layout.js';
import { openFilters } from './filters.js';
import { emptyNote } from './searchbox.js';
import { rowActions, disclosureAria, followersHead } from './recentrows.js';
import { isPicked, picking, pickBox, togglePick, bindPick } from './recentpick.js';
import { byLabel, disclosure, undoButton, againButton, stackSteps, followBoxes, failureRows, clickToToggle, pullToLoad, arrivals } from './widgets.js';

// What it takes from desktop.js: select(k, item), focusBox(input), openConfirm(plan), focusSearch(k),
// openKeys() (the shortcut list, built after this), boxes (for the two inputs) and active().
// The module owns the Recent state: expanded rows, open folders and followers, the arrival marks,
// the keyboard cursor.
export function desktopRecent({ select, focusBox, openConfirm, focusSearch, openKeys, boxes, active }) {
  const expanded = new Set();
  const openFolders = new Set();
  const openFollowers = new Set();
  const arrived = arrivals();

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
          : [
              el('div', 'sode__label', `Before #${e.id}, on ${dst.label}`),
              ...beforeGroups(e).map((g) => srcRow(g.levels, tallyText(g.src), g.src, useRowSrc)),
            ]),
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
      openKeys();
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

  function histArea(n) {
    return !!n && (histList.contains(n) || $('history-more').contains(n));
  }

  return { renderHistory, enterKeys, keyMode, histList, histArea, onListKey, undoLast, pager };
}
