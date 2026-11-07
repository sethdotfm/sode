// Widgets both layouts build the same way; each layout passes its own elements.

import { actorUser, groupActor } from './lockrules.js';
import { el, S, needsLink, srcItem, dstItem, tallyText, dstLabel, chevron, icon } from './core.js';
import { mac } from './keys.js';
import { levelBar, levelText, parseLevels, levelInput, defaultLevelsHint, relabelLevels, pickButtons, allLevels } from './levelview.js';
import { srcRow, srcEnd } from './routes.js';
import { fmtClock } from './device.js';
import { windowAround, growEdge } from './listwindow.js';
import { beforeGroups, failureGroups } from './history.js';
import { restorePlan, planEmpty } from './plans.js';

// ---- level box: typed levels plus a picker of toggles and All/None

// hold: the event whose default is stopped so the picker and All don't take focus ('mousedown'
// on desktop, 'pointerdown' on a phone, to keep the keyboard up).
export function levelBox(input, picker, allBtn, { hold = 'mousedown', placeholder = defaultLevelsHint, onChange = () => {} } = {}) {
  let none = false; // none: picker cleared to no levels (a blank box would mean default)
  const levels = () => parseLevels(input.value, none);

  function render() {
    if (!S.cfg) return;
    const list = levels();
    const on = new Set(list || []);
    input.classList.toggle('pb-input--invalid', !list);
    input.placeholder = none ? 'none' : placeholder();
    for (const b of picker.children) b.setAttribute('aria-pressed', String(on.has(+b.dataset.ui)));
    const full = on.size === S.levels.length;
    allBtn.replaceChildren(el('span', null, 'Select'), el('span', null, full ? 'None' : 'All'));
    const def = parseLevels('') || [];
    const isDefault = !none && !!list && list.length === def.length && list.every((u, i) => u === def[i]);
    input.closest('.sode__keyed')?.classList.toggle('sode__keyed--default', isDefault);
  }

  function set(list) {
    none = list.length === 0;
    input.value = levelInput(list);
    render();
    onChange();
  }

  function toggle(ui) {
    const on = new Set(levels() || []);
    if (on.has(ui)) on.delete(ui);
    else on.add(ui);
    set([...on].sort((x, y) => x - y));
  }

  input.addEventListener('input', () => {
    none = false;
    render();
    onChange();
  });
  for (const n of [picker, allBtn]) n.addEventListener(hold, (e) => e.preventDefault());
  allBtn.addEventListener('click', () => set((levels() || []).length === S.levels.length ? [] : allLevels()));

  return {
    levels,
    set,
    render,
    empty: () => !input.value && !none,
    clear() {
      input.value = '';
      none = false;
      render();
    },
    reset() {
      input.value = relabelLevels(input.value);
      picker.replaceChildren(...pickButtons(toggle));
      const odd = S.levels.length % 2;
      picker.classList.toggle('sode__levels--odd', !!odd);
      picker.style.setProperty('--sode-bar-cols', Math.floor(S.levels.length / 2) + odd);
      render();
    },
  };
}

// ---- combobox: a list of matches under a text box

const PAGE = 8;
let boxSeq = 0;

// input: the text box, or a function giving it for a list shared by several. press: 'mousedown'
// picks keeping focus; 'click' stops pointerdown (keeps a phone's keyboard up) and picks on
// click so a scroll doesn't pick. reverse: drawn bottom-up, ↑ moves to the next match. foot:
// hidden element for the count line. add: + button on every row; canAdd() says whether the list
// has them now.
export function combobox(
  input,
  menu,
  { source, onPick, add, canAdd = () => true, press = 'mousedown', cls = 'pb-menu__item', hiCls = 'active', reverse = false, foot = null, onRender = null },
) {
  const box = typeof input === 'function' ? input : () => input;
  const pre = `${(typeof input !== 'function' && input.id) || `cb${++boxSeq}`}-opt-`;
  let matches = [];
  let hi = 0;
  let last = null;
  // A list opened by focus (browse): Enter and Tab must not take its first row. Cleared by
  // typing, a row click, or moving the highlight.
  let browsing = false;
  // Opened by browse() at all: such a list is long, so only a window of it is drawn
  // (listwindow.js).
  let browsed = false;
  let win = { lo: 0, hi: 0 };
  let rows = [];

  function addButton(it) {
    const b = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode__menu-add', icon('plus', false));
    b.type = 'button';
    b.tabIndex = -1;
    b.title = 'Add and keep searching';
    b.setAttribute('aria-label', `Add ${it.name}`);
    // pointerdown too: the phone's rows pick on click and keep the keyboard up.
    for (const t of ['mousedown', 'pointerdown'])
      b.addEventListener(t, (e) => {
        e.preventDefault();
        e.stopPropagation();
      });
    b.addEventListener('click', (e) => {
      e.stopPropagation();
      add(it);
    });
    return b;
  }

  function placeFoot() {
    if (!foot) return;
    const long = !menu.hidden && matches.length > PAGE;
    foot.hidden = !long;
    if (!long) return;
    const total = matches.total > matches.length ? `${matches.length}+` : matches.length;
    foot.replaceChildren(el('span', null, `${total} matches`), el('span', 'sode__menu-keys', `↑ ↓ or ${mac ? '⌘j/k' : 'Ctrl+j/k'}`));
  }

  function render() {
    const open = matches.length > 0;
    const input = box();
    if (last && last !== input) {
      last.setAttribute('aria-expanded', 'false');
      last.removeAttribute('aria-activedescendant');
    }
    last = input;
    menu.hidden = !open;
    onRender?.(open);
    input.setAttribute('aria-expanded', String(open));
    if (!open) {
      rows = [];
      input.removeAttribute('aria-activedescendant');
      placeFoot();
      return;
    }
    if (!browsed) win = { lo: 0, hi: matches.length };
    rows = rowsFor(win.lo, win.hi);
    menu.replaceChildren(...rows);
    placeFoot();
    if (browsing) input.removeAttribute('aria-activedescendant');
    else input.setAttribute('aria-activedescendant', pre + hi);
  }

  // Rows carry no listeners: one pair on the menu reads data-i, so a window of rows costs little.
  function rowsFor(from, to) {
    const out = [];
    for (let i = from; i < to; i++) {
      const it = matches[i];
      const on = i === hi && !browsing;
      const li = el('li', cls + (on ? ` ${hiCls}` : ''), el('span', 'sode__num', it.num), it.name);
      li.id = pre + i;
      li.dataset.i = String(i);
      li.setAttribute('role', 'option');
      li.setAttribute('aria-selected', String(on));
      if (add && canAdd()) li.append(addButton(it));
      out.push(li);
    }
    return out;
  }

  const rowOf = (e) => {
    const li = e.target?.closest?.('[role="option"]');
    return li && menu.contains(li) ? +li.dataset.i : -1;
  };
  if (press === 'click') {
    menu.addEventListener('pointerdown', (e) => {
      if (rowOf(e) >= 0) e.preventDefault();
    });
    menu.addEventListener('click', (e) => {
      const i = rowOf(e);
      if (i >= 0) choose(i);
    });
  } else {
    menu.addEventListener('mousedown', (e) => {
      const i = rowOf(e);
      if (i >= 0) {
        e.preventDefault();
        choose(i);
      }
    });
  }
  menu.addEventListener('scroll', () => {
    if (!browsed || !matches.length) return;
    const below = menu.scrollTop + menu.clientHeight >= menu.scrollHeight - 40;
    if (!below && menu.scrollTop > 40) return;
    const next = growEdge(win, matches.length, below);
    if (next === win) return;
    const was = menu.scrollHeight;
    const added = below ? rowsFor(win.hi, next.hi) : rowsFor(next.lo, win.lo);
    if (below) {
      menu.append(...added);
      rows.push(...added);
    } else {
      menu.prepend(...added);
      rows.unshift(...added);
      menu.scrollTop += menu.scrollHeight - was;
    }
    win = next;
  });

  // Rows are not redrawn (the list keeps its scroll) unless a windowed list has to draw the row
  // first.
  function move(to) {
    browsing = false;
    if (browsed && (to < win.lo || to >= win.hi)) {
      hi = to;
      win = windowAround(matches.length, to);
      render();
      rows[hi - win.lo].scrollIntoView({ block: 'nearest' });
      return;
    }
    rows[hi - win.lo]?.classList.remove(hiCls);
    rows[hi - win.lo]?.setAttribute('aria-selected', 'false');
    hi = to;
    const row = rows[hi - win.lo];
    row.classList.add(hiCls);
    row.setAttribute('aria-selected', 'true');
    row.scrollIntoView({ block: 'nearest' });
    box().setAttribute('aria-activedescendant', pre + hi);
  }

  function accept() {
    const it = matches[hi];
    if (!it || browsing) return false;
    onPick(it);
    return true;
  }
  function choose(i) {
    hi = i;
    onPick(matches[i]);
  }

  // keepMoved: a browse list replaced by a fresh one (names changed) that the user already
  // moved through stays one Enter may take from.
  function show(items, at, isBrowse, keepMoved = false) {
    matches = items;
    browsed = isBrowse && items.length > 0;
    browsing = browsed && !keepMoved;
    hi = at;
    if (browsed) win = windowAround(items.length, at);
    render();
  }

  return {
    refresh() {
      show(source(box().value), 0, false);
      menu.scrollTop = 0;
    },
    browse(items, n, { keepMoved = false } = {}) {
      show(
        items,
        Math.max(
          0,
          items.findIndex((it) => it.n === n),
        ),
        true,
        keepMoved && browsed && !browsing,
      );
      rows[hi - win.lo]?.scrollIntoView({ block: 'nearest' });
    },
    close() {
      show([], 0, false);
    },
    accept,
    current: () => matches[hi],
    get open() {
      return matches.length > 0;
    },
    get moved() {
      return browsed && !browsing;
    },
    // Also swallows the keys with the list closed, so neither the browser's J/K shortcuts nor
    // macOS's Ctrl+K act.
    nav(e) {
      const mod = (e.ctrlKey && !e.metaKey) || (mac && e.metaKey && !e.ctrlKey);
      const ctrlJK = mod && !e.altKey && !e.shiftKey && { KeyJ: 'ArrowDown', KeyK: 'ArrowUp' }[e.code];
      const key = ctrlJK || e.key;
      if (!matches.length || !['ArrowDown', 'ArrowUp', 'PageDown', 'PageUp'].includes(key)) {
        if (ctrlJK) e.preventDefault();
        return !!ctrlJK;
      }
      e.preventDefault();
      const n = matches.length;
      const fwd = (key === 'ArrowDown' || key === 'PageDown') !== reverse;
      const step = key === 'ArrowDown' || key === 'ArrowUp';
      if (browsing && step && fwd) move(hi);
      else if (step) move(fwd ? (hi + 1) % n : (hi + n - 1) % n);
      else move(fwd ? Math.min(n - 1, hi + PAGE) : Math.max(0, hi - PAGE));
      return true;
    },
  };
}

export function disclosure(open, label, controls, onToggle) {
  const b = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode__hist-toggle', chevron(open), label);
  b.type = 'button';
  b.setAttribute('aria-expanded', String(open));
  b.setAttribute('aria-controls', controls);
  b.addEventListener('click', onToggle);
  return b;
}

// The label wraps the control, so clicking the words focuses it.
export const labeled = (text, control, cls = '') => el('label', `sode__fld ${cls}`.trim(), el('span', 'sode__label', text), control);

// A click anywhere on row (not a control, nor inside skip, nor a different row) works toggle,
// as does one that selected text not. data-no-tap (a swipe just ended) skips one click.
export function clickToToggle(row, toggle, skip) {
  row.addEventListener('click', (e) => {
    if (row.dataset.noTap) {
      delete row.dataset.noTap;
      return;
    }
    const t = e.target;
    if (t.closest('button, a, input, select, textarea') || (skip && t.closest(skip))) return;
    if (t.closest('.sode__hist-row, .sode-m__recent-face') !== row) return;
    if (String(getSelection()).trim()) return;
    toggle.click();
  });
  row.classList.add('sode--tap');
}

const UNDO_TIP = 'Set destination to its source before this change.';
export const UNDO_TIP_ALL = 'Set destinations to their sources before these changes.';
export function undoButton(plan, label, onClick, { text = 'Undo', cls = 'pb-btn pb-btn--outline pb-btn--sm', tip = UNDO_TIP } = {}) {
  const b = el('button', `${cls} pb-tip--end`, icon('undo-2'), text);
  b.type = 'button';
  if (planEmpty(plan)) {
    b.disabled = true;
    b.dataset.pbTip = 'Nothing to undo';
  } else {
    b.dataset.pbTip = tip;
    needsLink(b);
  }
  b.setAttribute('aria-label', label);
  b.addEventListener('click', () => onClick(plan));
  return b;
}

// Retry (failed) or Redo (other) by plan.verb: one button, one slot. With nothing to offer an
// invisible placeholder keeps columns lined up (null with placeholder false).
export function againButton(plan, label, onClick, { placeholder = true, cls = 'pb-btn pb-btn--outline pb-btn--sm' } = {}) {
  const none = !plan.items.length;
  if (none && !placeholder) return null;
  const redo = plan.verb !== 'retry';
  const b = el('button', `${cls}${none ? ' sode__placeholder' : ''}`, redo ? icon('redo-2') : null, redo ? 'Redo' : 'Retry');
  b.type = 'button';
  if (none) {
    b.tabIndex = -1;
    b.setAttribute('aria-hidden', 'true');
    return b;
  }
  b.classList.add('pb-tip--end');
  const many = plan.items.length > 1;
  b.dataset.pbTip = redo ? `Perform ${many ? 'these changes' : 'this change'} again.` : `Try the ${many ? 'takes' : 'take'} that failed again.`;
  needsLink(b);
  b.setAttribute('aria-label', label);
  b.addEventListener('click', () => onClick(plan));
  return b;
}

// A live entry (S.newId) animates in once; one per layout, since each draws its own rows.
export function arrivals() {
  const seen = new Set();
  return {
    entry(e) {
      if (e.id !== S.newId || seen.has(e.id)) return false;
      seen.add(e.id);
      return true;
    },
    // A folder animates if closed; either way it uses up the arrival of the take inside, so
    // opening it doesn't replay it.
    group(f, open) {
      const has = f.entries.some((e) => e.id === S.newId);
      const fresh = has && !seen.has(`g${f.group}`);
      if (fresh) seen.add(`g${f.group}`);
      if (!open && has) seen.add(S.newId);
      return fresh && !open;
    },
  };
}

// One row per outcome ("still on CAM 2", "no answer"); [] if unknown. The phone has no such
// card: its row's note says it in full.
export function failureRows(e) {
  const why = failureGroups(e);
  if (!why.length) return [];
  return [el('div', 'sode__label', 'Not confirmed'), ...why.map((g) => srcRow(g.levels, g.text))];
}

// Stacked row timeline, newest first. Every change but the newest has Restore. restore(plan)
// confirms; pickSrc(n) / pickDst(item), if given, make each source / followed destination a
// button.

export function stackSteps(e, { restore: onRestore, pickSrc: onPickSrc, pickDst: onPickDst }) {
  const lv = (ls) => ls.filter((u) => S.levelBy.has(u));
  const step = (time, levels, text, btn, follows = null, src = null) =>
    el(
      'div',
      'sode__route sode__step',
      el('span', 'sode__step-time', time),
      levelBar(levels),
      el('span', 'sode__step-what', srcEnd(src, text, onPickSrc), follows),
      btn,
    );
  const restore = (then, at) => {
    const label = srcItem(at.src, at.srcName).label;
    const plan = restorePlan(then, at);
    const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Restore');
    b.type = 'button';
    if (planEmpty(plan)) b.disabled = true;
    else needsLink(b);
    b.setAttribute('aria-label', `Restore ${label}, as at ${fmtClock(at.time)}`);
    b.addEventListener('click', () => onRestore(plan));
    return b;
  };
  return [
    ...e.chain.map((c, i) =>
      step(
        fmtClock(c.time),
        lv(c.levels),
        srcItem(c.src, c.srcName).label,
        i === 0 ? null : restore(e.chain[i - 1], c),
        followBoxes(
          (e.follows || []).filter((f) => f.follow === c.id),
          c.levels,
          onPickDst,
        ),
        c.src,
      ),
    ),
    ...beforeGroups(e).map((g, i) => step(i === 0 ? 'before' : '', g.levels, tallyText(g.src), null, null, g.src)),
  ];
}

// "Followed by:" with each follow's destination, levels shown where they differ. Null if none.
// onPickDst(item) makes each fill the Destination box.
export function followBoxes(follows, levels, onPickDst) {
  if (!follows?.length) return null;
  const lv = (ls) => ls.filter((u) => S.levelBy.has(u));
  const same = (ls) => lv(ls).join() === lv(levels).join();
  return el(
    'span',
    'sode__follows',
    el('span', 'sode__hist-note', 'Followed by:'),
    ...follows.map((f) => {
      const it = dstItem(f.dst, f.dstName);
      const note = same(f.levels) ? null : el('span', 'sode__hist-note', ` (${levelText(lv(f.levels))})`);
      if (!onPickDst) return el('span', 'sode__end sode__end--dst sode__end--follow', it.label, note);
      const b = el('button', 'sode__end sode__end--dst sode__end--follow sode__end--pick', it.label, note);
      b.type = 'button';
      b.tabIndex = -1;
      b.dataset.pbTip = 'Use as destination';
      b.addEventListener('mousedown', (e) => e.preventDefault());
      b.addEventListener('click', () => onPickDst(it));
      return b;
    }),
  );
}

// Pulling past the end of scroller presses Load more: a touch dragged PULL px up and let go, or
// WHEEL of wheel scrolling at the bottom in one go (a pause starts over). box's --sode-pull
// runs 0..1. when(): this layout is the one showing.
const PULL = 140,
  WHEEL = 700,
  GAP_MS = 250;
export function pullToLoad(scroller, box, pager, when) {
  const el0 = () => (scroller === window ? document.scrollingElement : scroller);
  const atEnd = () => {
    const s = el0();
    return s.scrollTop + s.clientHeight >= s.scrollHeight - 2;
  };
  const ready = () => when() && pager.hasMore() && !S.histLoading && !!box.querySelector('button');
  const show = (p) => box.style.setProperty('--sode-pull', String(Math.max(0, Math.min(1, p))));
  const fire = () => {
    show(0);
    pager.more();
  };

  let sum = 0,
    last = 0,
    idle = 0;
  scroller.addEventListener(
    'wheel',
    (e) => {
      const now = performance.now();
      if (now - last > GAP_MS) sum = 0;
      last = now;
      clearTimeout(idle);
      if (e.deltaY <= 0 || !atEnd() || !ready()) {
        sum = Math.min(sum, 0);
        show(0);
        return;
      }
      sum += e.deltaY * (e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? el0().clientHeight : 1);
      if (sum >= WHEEL) {
        sum = -Infinity;
        fire();
        return;
      }
      show(sum / WHEEL);
      idle = setTimeout(() => {
        sum = 0;
        show(0);
      }, GAP_MS);
    },
    { passive: true },
  );

  let y0 = null,
    pulled = 0;
  scroller.addEventListener(
    'touchstart',
    (e) => {
      y0 = e.touches.length === 1 && atEnd() && ready() ? e.touches[0].clientY : null;
      pulled = 0;
    },
    { passive: true },
  );
  scroller.addEventListener(
    'touchmove',
    (e) => {
      if (y0 == null) return;
      pulled = (y0 - e.touches[0].clientY) / PULL;
      show(pulled);
    },
    { passive: true },
  );
  const end = () => {
    if (y0 != null && pulled >= 1 && ready()) fire();
    else show(0);
    y0 = null;
  };
  scroller.addEventListener('touchend', end);
  scroller.addEventListener('touchcancel', () => {
    y0 = null;
    show(0);
  });
}

// First two as chips, all once open, toggle for the rest. onPick(d) uses one as the destination.
export function feeding(fed, { open, onToggle, onPick, hold = 'mousedown', empty = 'Not feeding any destination' }) {
  if (!fed.length) return el('div', 'sode__hint', empty);
  const chip = (d) => {
    const c = el('button', 'pb-chip', dstLabel(d));
    c.type = 'button';
    c.title = 'Use as destination';
    c.addEventListener(hold, (e) => e.preventDefault());
    c.addEventListener('click', () => onPick(d));
    return c;
  };
  const shown = open ? fed : fed.slice(0, 2);
  const out = el('div', 'sode__feed', el('span', 'sode__label', `Feeding ${fed.length}`), el('div', 'sode__chips', ...shown.map(chip)));
  if (fed.length > 2) {
    const more = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', open ? 'Show less' : `and ${fed.length - 2} others`);
    more.type = 'button';
    more.setAttribute('aria-expanded', String(open));
    more.addEventListener(hold, (e) => e.preventDefault());
    more.addEventListener('click', onToggle);
    out.append(more);
  }
  return out;
}

export { lockout } from './lockout.js';

export { trapDialog } from './dialogkeys.js';

// "by Ana": a Sode take by a configured user (never an address; ext and found entries have
// none). A folder shows it when all its takes share one.
export function byLabel(x) {
  const users = S.userNames;
  const name = x.entries ? groupActor(x.entries, users) : actorUser(x.client, users);
  return name ? el('span', 'sode__hist-by', `by ${name}`) : null;
}
