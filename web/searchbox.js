// Recent's quick search boxes (kept in step across layouts) and the note under an empty list.

import { el, S, on, search, item, labelNode } from './core.js';
import { histRows, setSearch, searchActive, searchRespectsFilters } from './history.js';
import { parseFilter } from './filter.js';
import { pickSet, activeStack } from './filterstore.js';
import { combobox } from './widgets.js';

// While searching: still reading back through the log, or nothing found (the filters may not
// even apply).
export function emptyNote(rows, fetching) {
  if (rows.length) return null;
  if (!S.hist.length) return el('div', 'sode__filters-empty', el('span', 'sode__hint', 'No history yet'));
  const searching = searchActive();
  if (searching && fetching) return el('div', 'sode__filters-empty', el('span', 'sode__hint', 'Searching…'));
  const clear = el('button', 'pb-btn pb-btn--outline pb-btn--sm', searching ? 'Clear search' : 'Clear filters');
  clear.type = 'button';
  clear.addEventListener('click', () => (searching ? clearSearch() : pickSet('all', 'All')));
  return el(
    'div',
    'sode__filters-empty',
    el('span', 'sode__hint', !searching && activeStack().length ? 'No matches in the stacked sets.' : 'No matches.'),
    clear,
  );
}

// ---- quick search boxes above Recent, kept in step across layouts. Names logged in history count too, so a renamed source is still found. Text that's an expression applies as typed instead.

const looksLikeExpr = (t) => /[,!/"']/.test(t) || /^\s*\d+\s*(-|\s(thru|through|to)\s)\s*\d+\s*$/i.test(t) || /^\s*not\s+['"/\d]/i.test(t);
const MENU = 50;

// The router's items plus differing names history logged for them (another router, or renamed).
// Built once per names/history change.
const candCache = { src: null, dst: null };
function candidates(k) {
  const live = k === 'src' ? S.srcs : S.dsts;
  const stamp = [live, S.hist, S.hist.length];
  const c = candCache[k];
  if (c && c.stamp.every((x, i) => x === stamp[i])) return c.list;
  const list = buildCandidates(k, live);
  candCache[k] = { stamp, list };
  return list;
}
function buildCandidates(k, live) {
  const by = k === 'src' ? S.srcBy : S.dstBy;
  const seen = new Set(live.map((i) => `${i.n}\u0000${i.name}`));
  const extra = [];
  for (const e of S.hist) {
    for (const x of [e, ...(e.follows || [])]) {
      const n = x[k],
        name = k === 'src' ? x.srcName : x.dstName;
      const key = `${n}\u0000${name || ''}`;
      if (seen.has(key) || (by.has(n) && !name)) continue;
      seen.add(key);
      extra.push(item(n, name || ''));
    }
  }
  return extra.length ? [...live, ...extra] : live;
}

const searchBoxes = [...document.querySelectorAll('[data-search]')];
const sb = { src: { picks: [], text: '' }, dst: { picks: [], text: '' } };
const lists = [];

function menuOf(f) {
  return f.parentElement.querySelector('.sode__menu');
}

function boxQuery(k) {
  const st = sb[k];
  const expr = looksLikeExpr(st.text) ? st.text.trim() : '';
  return [...st.picks.map((it) => it.num), expr].filter(Boolean).join(', ');
}
const applySearch = () => setSearch({ src: boxQuery('src'), dst: boxQuery('dst') });

function clearSearch() {
  for (const k of ['src', 'dst']) Object.assign(sb[k], { picks: [], text: '' });
  for (const f of document.querySelectorAll('[data-search] input')) f.value = '';
  for (const cb of lists) cb.close();
  applySearch();
}

function pick(k, f, cb, it) {
  const st = sb[k];
  if (!st.picks.some((p) => p.n === it.n)) st.picks.push(it);
  st.text = '';
  f.value = '';
  cb.close();
  applySearch();
}

function unpick(k, i) {
  sb[k].picks.splice(i, 1);
  applySearch();
}

function typed(k, f, cb) {
  sb[k].text = f.value;
  cb.refresh();
  applySearch();
}

for (const b of searchBoxes) {
  for (const k of ['src', 'dst']) {
    const f = b.querySelector(`[data-search-${k}]`);
    f.before(el('span', 'sode__search-chips'));
    const cb = combobox(f, menuOf(f), {
      source: (t) => (!t.trim() || looksLikeExpr(t) ? [] : search(candidates(k), t, MENU)),
      onPick: (it) => pick(k, f, cb, it),
    });
    lists.push(cb);
    f.addEventListener('input', () => typed(k, f, cb));
    f.addEventListener('focus', () => {
      if (f.value && !looksLikeExpr(f.value)) typed(k, f, cb);
    });
    f.addEventListener('blur', () => cb.close());
    f.addEventListener('keydown', (e) => {
      const st = sb[k];
      if (cb.nav(e)) return;
      if (cb.open && (e.key === 'Enter' || e.key === '+' || (e.key === 'Tab' && !e.shiftKey))) {
        if (cb.accept() && e.key !== 'Tab') e.preventDefault();
        return;
      }
      if (e.key === 'Backspace' && !f.value && st.picks.length) {
        e.preventDefault();
        unpick(k, st.picks.length - 1);
        return;
      }
      if (e.key !== 'Escape') return;
      // Esc closes the list, then clears the typing, then leaves the box.
      e.preventDefault();
      e.stopPropagation();
      if (cb.open) cb.close();
      else if (f.value) {
        f.value = '';
        typed(k, f, cb);
      } else f.blur();
    });
  }
  // Clear leaves focus alone (no phone keyboard); the button greys out, so let focus go.
  b.querySelector('[data-search-clear]').addEventListener('click', (e) => {
    clearSearch();
    e.currentTarget.blur();
  });
}

function renderSearch() {
  const on = searchActive();
  const n = on ? histRows(S.hist).length : 0;
  const note = !on ? '' : `${n} found${S.histMore ? ' so far' : ''}${searchRespectsFilters() ? '' : ', ignoring filters'}`;
  for (const b of searchBoxes) {
    for (const k of ['src', 'dst']) {
      const st = sb[k];
      const f = b.querySelector(`[data-search-${k}]`);
      if (document.activeElement !== f && f.value !== st.text) f.value = st.text;
      const err = looksLikeExpr(st.text) ? parseFilter(st.text).error : null;
      f.classList.toggle('pb-input--invalid', !!err);
      f.title = err || '';
      f.previousElementSibling.replaceChildren(
        ...st.picks.map((it, i) => {
          const x = el('button', 'pb-chip__remove', '×');
          x.type = 'button';
          x.tabIndex = -1;
          x.setAttribute('aria-label', `Remove ${it.label}`);
          x.addEventListener('mousedown', (e) => e.preventDefault());
          x.addEventListener('click', () => unpick(k, i));
          return el('span', 'pb-chip pb-chip--static sode__search-chip', labelNode(it), x);
        }),
      );
    }
    b.querySelector('[data-search-clear]').disabled = !on && !['src', 'dst'].some((k) => sb[k].text || sb[k].picks.length);
    b.querySelector('[data-search-note]').textContent = note;
  }
}
on('search', renderSearch);
on('history', () => {
  if (searchActive()) renderSearch();
});
