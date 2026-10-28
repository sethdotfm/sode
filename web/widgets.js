// Widgets both layouts build the same way: the level box, the match list
// under a text box, Recent's buttons and arrival animation, the Feeding
// row and the confirm's lockout. Each layout passes its own elements and
// the few things it does differently.

import { el, S, needsLink, srcItem, dstItem, tallyText, dstLabel, chevron } from './core.js';
import {
  levelBar, levelText, parseLevels, levelInput, defaultLevelsHint, relabelLevels, pickButtons, allLevels,
} from './levelview.js';
import { srcRow } from './routes.js';
import { fmtClock } from './device.js';
import { beforeGroups, failureGroups } from './history.js';
import { restorePlan, planEmpty } from './plans.js';

const mac = /Mac|iPhone|iPad/.test(navigator.platform);

// ---- level box: typed levels plus a picker of toggles and All/None

// hold: the event whose default is stopped so the picker and All don't
// take focus ('mousedown' on desktop, 'pointerdown' on a phone, to keep the
// keyboard up). placeholder() is the blank box's text. onChange after the
// levels change.
export function levelBox(input, picker, allBtn, { hold = 'mousedown', placeholder = defaultLevelsHint, onChange = () => {} } = {}) {
  let none = false; // picker cleared to no levels (a blank box would mean default)
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
    // The box's key note stays up while it holds the default levels.
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

  input.addEventListener('input', () => { none = false; render(); onChange(); });
  // Picker buttons act without taking focus, so typing carries on in place.
  for (const n of [picker, allBtn]) n.addEventListener(hold, (e) => e.preventDefault());
  allBtn.addEventListener('click', () => set((levels() || []).length === S.levels.length ? [] : allLevels()));

  return {
    levels,
    set,
    render,
    // Nothing typed and not cleared to none: the default.
    empty: () => !input.value && !none,
    clear() {
      input.value = '';
      none = false;
      render();
    },
    // On hello: the levels or their numbering may have changed.
    reset() {
      input.value = relabelLevels(input.value);
      picker.replaceChildren(...pickButtons(toggle));
      // Laid out like the level bar where the CSS asks (desktop).
      const odd = S.levels.length % 2;
      picker.classList.toggle('sode__levels--odd', !!odd);
      picker.style.setProperty('--sode-bar-cols', Math.floor(S.levels.length / 2) + odd);
      render();
    },
  };
}

// ---- combobox: a list of matches under a text box

const PAGE = 8; // rows PgUp/PgDn move, about a dropdown's height
let boxSeq = 0;

// input: the text box, or a function giving it for a list shared by
// several (the phone's steps). source(text) gives the matches; onPick(item) when one is taken (Enter,
// a click, accept()); it's up to the caller to close the list then.
// press: 'mousedown' picks on mousedown, keeping focus; 'click' stops
// pointerdown (keeps a phone's keyboard up) and picks on click, so a
// scroll doesn't pick. reverse: the list is drawn bottom-up, so ↑ moves to
// the next match. foot: a count pinned at the bottom of a long list.
// add: a + button on every row; add(item) when it is pressed (the list and
// the box's focus stay as they are); canAdd() says whether the list has
// them now (the phone's source step doesn't).
export function combobox(input, menu, { source, onPick, add, canAdd = () => true, press = 'mousedown', cls = 'pb-menu__item', hiCls = 'active', reverse = false, foot = false }) {
  const box = typeof input === 'function' ? input : () => input;
  const pre = `${(typeof input !== 'function' && input.id) || `cb${++boxSeq}`}-opt-`;
  let matches = [];
  let hi = 0;
  let last = null; // the box last drawn for, while it can change

  function addButton(it) {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('class', 'pb-icon');
    svg.setAttribute('viewBox', '0 0 24 24');
    svg.setAttribute('aria-hidden', 'true');
    const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
    use.setAttribute('href', 'vendor/patchbay/icons/lucide.svg#plus');
    svg.append(use);
    const b = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode__menu-add', svg);
    b.type = 'button';
    b.tabIndex = -1;
    b.title = 'Add and keep searching';
    b.setAttribute('aria-label', `Add ${it.name}`);
    // Neither the row's pick nor a blur of the box.
    // pointerdown too: the phone's rows pick on click and keep the keyboard up.
    for (const t of ['mousedown', 'pointerdown']) b.addEventListener(t, (e) => { e.preventDefault(); e.stopPropagation(); });
    b.addEventListener('click', (e) => { e.stopPropagation(); add(it); });
    return b;
  }

  function render() {
    const open = matches.length > 0;
    const input = box();
    // A list shared by several boxes: the one it's left says it's closed.
    if (last && last !== input) {
      last.setAttribute('aria-expanded', 'false');
      last.removeAttribute('aria-activedescendant');
    }
    last = input;
    menu.hidden = !open;
    input.setAttribute('aria-expanded', String(open));
    if (!open) { input.removeAttribute('aria-activedescendant'); return; }
    menu.replaceChildren(...matches.map((it, i) => {
      const li = el('li', cls + (i === hi ? ` ${hiCls}` : ''), el('span', 'sode__num', it.num), it.name);
      li.id = pre + i;
      li.setAttribute('role', 'option');
      li.setAttribute('aria-selected', String(i === hi));
      if (press === 'click') {
        li.addEventListener('pointerdown', (e) => e.preventDefault());
        li.addEventListener('click', () => { hi = i; accept(); });
      } else {
        li.addEventListener('mousedown', (e) => { e.preventDefault(); hi = i; accept(); });
      }
      if (add && canAdd()) li.append(addButton(it));
      return li;
    }));
    // A pinned count makes it clear the list scrolls.
    if (foot && matches.length > PAGE) {
      const total = matches.total > matches.length ? `${matches.length}+` : matches.length;
      const f = el('li', 'sode__menu-count', `${total} matches · ↑↓ ${mac ? '⌘j ⌘k' : 'Ctrl+j Ctrl+k'} PgUp PgDn`);
      f.setAttribute('role', 'presentation');
      menu.append(f);
    }
    input.setAttribute('aria-activedescendant', pre + hi);
  }

  // Moves the highlight without redrawing, so the list keeps its scroll.
  function move(to) {
    const items = menu.querySelectorAll('[role="option"]');
    items[hi]?.classList.remove(hiCls);
    items[hi]?.setAttribute('aria-selected', 'false');
    hi = to;
    items[hi].classList.add(hiCls);
    items[hi].setAttribute('aria-selected', 'true');
    items[hi].scrollIntoView({ block: 'nearest' });
    box().setAttribute('aria-activedescendant', pre + hi);
  }

  function accept() {
    const it = matches[hi];
    if (!it) return false;
    onPick(it);
    return true;
  }

  return {
    // Matches for what's typed now, best first and highlighted.
    refresh() {
      matches = source(box().value);
      hi = 0;
      render();
      menu.scrollTop = 0;
    },
    close() {
      matches = [];
      render();
    },
    accept,
    current: () => matches[hi],
    get open() { return matches.length > 0; },
    // ↑/↓, PgUp/PgDn, and Ctrl+J/K (⌘J/K too on a Mac) like ↓/↑
    // (swallowed with the list closed too, so neither the browser's own
    // J/K shortcuts nor macOS's Ctrl+K, delete to the end of the line,
    // act). True when the key was used.
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
      if (key === 'ArrowDown' || key === 'ArrowUp') move(fwd ? (hi + 1) % n : (hi + n - 1) % n);
      else move(fwd ? Math.min(n - 1, hi + PAGE) : Math.max(0, hi - PAGE));
      return true;
    },
  };
}

// ---- Recent: buttons and arrivals

export function disclosure(open, label, controls, onToggle) {
  const b = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode__hist-toggle', chevron(open), label);
  b.type = 'button';
  b.setAttribute('aria-expanded', String(open));
  b.setAttribute('aria-controls', controls);
  b.addEventListener('click', onToggle);
  return b;
}

// A click anywhere on row (not a control, nor inside skip, nor a
// different row within it) works toggle, as does one that selected text
// not. A row that sets data-no-tap (a swipe just ended) skips one click.
export function clickToToggle(row, toggle, skip) {
  row.addEventListener('click', (e) => {
    if (row.dataset.noTap) { delete row.dataset.noTap; return; }
    const t = e.target;
    if (t.closest('button, a, input, select, textarea') || (skip && t.closest(skip))) return;
    if (t.closest('.sode__hist-row, .sode-m__recent-face') !== row) return;
    if (String(getSelection()).trim()) return;
    toggle.click();
  });
  row.classList.add('sode--tap');
}

// Undo for a take or a folder; disabled when there's nothing to restore.
export function undoButton(plan, label, onClick, { text = 'Undo', cls = 'pb-btn pb-btn--outline pb-btn--sm' } = {}) {
  const b = el('button', cls, text);
  b.type = 'button';
  if (planEmpty(plan)) {
    b.disabled = true;
    b.title = 'Nothing to undo';
  } else needsLink(b);
  b.setAttribute('aria-label', label);
  b.addEventListener('click', () => onClick(plan));
  return b;
}

// Retry for a row with failed takes. With none, null, or with placeholder
// an invisible one, so every row's columns line up.
export function retryButton(failed, label, onClick, { placeholder = false } = {}) {
  if (!failed && !placeholder) return null;
  const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm' + (failed ? '' : ' sode__placeholder'), 'Retry');
  b.type = 'button';
  if (!failed) {
    b.tabIndex = -1;
    b.setAttribute('aria-hidden', 'true');
    return b;
  }
  needsLink(b);
  b.setAttribute('aria-label', label);
  b.addEventListener('click', onClick);
  return b;
}

// A live entry (S.newId) animates in the first time it's drawn; nothing
// else does. One per layout, since each draws its own rows.
export function arrivals() {
  const seen = new Set();
  return {
    entry(e) {
      if (e.id !== S.newId || seen.has(e.id)) return false;
      seen.add(e.id);
      return true;
    },
    // A folder animates if it's closed; either way it uses up the arrival
    // of the take inside, so opening it doesn't replay it.
    folder(f, open) {
      const has = f.entries.some((e) => e.id === S.newId);
      const fresh = has && !seen.has(`g${f.group}`);
      if (fresh) seen.add(`g${f.group}`);
      if (!open && has) seen.add(S.newId);
      return fresh && !open;
    },
  };
}

// ---- why a take failed

// What the router reported for a failed take's levels, one row per outcome
// ("still on CAM 2", "no answer"), under a heading; [] if it isn't known.
// The phone has no such card: its row's note says the same in full.
export function failureRows(e) {
  const why = failureGroups(e);
  if (!why.length) return [];
  return [el('div', 'sode__label', 'Not confirmed'), ...why.map((g) => srcRow(g.levels, g.text))];
}

// ---- a stacked row (repeated changes merged into one) as a timeline

// Newest first: each change's time and source, then what the destination
// had before them all (which Undo restores). Every change but the newest
// has Restore: back to how it was then. onRestore(plan) confirms.
export function stackSteps(e, onRestore) {
  const lv = (ls) => ls.filter((u) => S.levelBy.has(u));
  const step = (time, levels, text, btn, follows = null) => el('div', 'sode__route sode__step',
    el('span', 'sode__step-time', time), levelBar(levels),
    el('span', 'sode__step-what', el('span', 'sode__end sode__end--src', text), follows), btn);
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
    ...e.chain.map((c, i) => step(fmtClock(c.time), lv(c.levels), srcItem(c.src, c.srcName).label,
      i === 0 ? null : restore(e.chain[i - 1], c), followBoxes((e.follows || []).filter((f) => f.follow === c.id), c.levels))),
    ...beforeGroups(e).map((g, i) => step(i === 0 ? 'before' : '', g.levels, tallyText(g.src), null)),
  ];
}

// A change's follows on one line: "Followed by:" and each follow's destination (its
// source is the change's own), with its levels where they differ from
// levels. Null if there are none.
export function followBoxes(follows, levels) {
  if (!follows?.length) return null;
  const lv = (ls) => ls.filter((u) => S.levelBy.has(u));
  const same = (ls) => lv(ls).join() === lv(levels).join();
  return el('span', 'sode__follows', el('span', 'sode__hist-note', 'Followed by:'),
    ...follows.map((f) => el('span', 'sode__end sode__end--dst', dstItem(f.dst, f.dstName).label,
      same(f.levels) ? null : el('span', 'sode__hist-note', ` (${levelText(lv(f.levels))})`))));
}

// ---- pull to load more

// Pulling on past the end of scroller (window or an element), like pull to
// refresh, presses Load more: a touch dragged PULL px up from the bottom
// and let go, or WHEEL of wheel scrolling at the bottom in one go (a pause
// starts over). box holds the button, which fills as the pull grows
// (--sode-pull, 0..1). when(): this layout is the one showing.
const PULL = 140, WHEEL = 700, GAP_MS = 250;
export function pullToLoad(scroller, box, pager, when) {
  const el0 = () => (scroller === window ? document.scrollingElement : scroller);
  const atEnd = () => { const s = el0(); return s.scrollTop + s.clientHeight >= s.scrollHeight - 2; };
  const ready = () => when() && pager.hasMore() && !S.histLoading && !!box.querySelector('button');
  const show = (p) => box.style.setProperty('--sode-pull', String(Math.max(0, Math.min(1, p))));
  const fire = () => { show(0); pager.more(); };

  let sum = 0, last = 0, idle = 0;
  scroller.addEventListener('wheel', (e) => {
    const now = performance.now();
    if (now - last > GAP_MS) sum = 0;
    last = now;
    clearTimeout(idle);
    if (e.deltaY <= 0 || !atEnd() || !ready()) { sum = Math.min(sum, 0); show(0); return; }
    sum += e.deltaY * (e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? el0().clientHeight : 1);
    if (sum >= WHEEL) { sum = -Infinity; fire(); return; } // once per gesture
    show(sum / WHEEL);
    idle = setTimeout(() => { sum = 0; show(0); }, GAP_MS);
  }, { passive: true });

  let y0 = null, pulled = 0;
  scroller.addEventListener('touchstart', (e) => {
    y0 = e.touches.length === 1 && atEnd() && ready() ? e.touches[0].clientY : null;
    pulled = 0;
  }, { passive: true });
  scroller.addEventListener('touchmove', (e) => {
    if (y0 == null) return;
    pulled = (y0 - e.touches[0].clientY) / PULL;
    show(pulled);
  }, { passive: true });
  const end = () => {
    if (y0 != null && pulled >= 1 && ready()) fire();
    else show(0);
    y0 = null;
  };
  scroller.addEventListener('touchend', end);
  scroller.addEventListener('touchcancel', () => { y0 = null; show(0); });
}

// ---- Feeding: the destinations a source feeds

// The first two as chips, all of them once open, and a toggle for the
// rest. onPick(d) uses one as the destination; hold as for levelBox.
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

// ---- confirm lockout

// ready() once the lockout (S.cfg.confirmLockoutMs) has passed; onReady
// then, unless cancelled first (the confirm closed).
export function lockout(onReady) {
  const ms = S.cfg.confirmLockoutMs;
  const at = performance.now() + ms;
  const t = setTimeout(onReady, ms);
  return { ready: () => performance.now() >= at, cancel: () => clearTimeout(t) };
}
