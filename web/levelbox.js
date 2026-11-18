// The Levels box: typed levels plus a picker of toggles and All/None. It takes its levels from a source function,
// so a page with no History, plans or routes can use it. Imports only core.js and levels.js.

import { el } from './core.js';
import { levelStyle, levelInputOf, parseLevelsIn, relabelLevelsFor } from './levels.js';

// ui: levels from the hello have {ui}, levels from /api/v1 have {level}.
const uiOf = (l) => l.ui ?? l.level;

export function levelColor(node, l) {
  const st = levelStyle(l);
  node.style.setProperty('--sode-c', st.color);
  if (st.ink) node.style.setProperty('--sode-ink', st.ink);
  else node.style.removeProperty('--sode-ink');
  return node;
}

export function levelPicks(levels, onToggle) {
  return levels.map((l) => {
    const b = el('button', 'sode__pick', l.label);
    b.type = 'button';
    b.tabIndex = -1;
    b.dataset.ui = uiOf(l);
    levelColor(b, l);
    b.setAttribute('aria-label', `Level ${l.label}`);
    b.addEventListener('click', () => onToggle(uiOf(l)));
    return b;
  });
}

// source() gives { levels, numbering, defaultLevels, prev, offered? } or null before the page has them. prev is
// { mode, max } of the numbering before the last change. offered, when set, is the subset of levels the picker
// shows and All selects; typed text still parses against all levels. hold: the event whose default is stopped so the
// picker and All do not get focus. It is 'mousedown' on desktop and 'pointerdown' on a phone, to keep the keyboard up.
export function levelBox(input, picker, allBtn, source, { hold = 'mousedown', onChange = () => {} } = {}) {
  let none = false; // none: picker cleared to no levels (a blank box would mean default)
  const parse = (src, text) => parseLevelsIn(text, src, none);
  const levels = () => {
    const src = source();
    return src ? parse(src, input.value) : null;
  };
  const shown = (src) => src.offered || src.levels;
  const allUi = (src) => shown(src).map(uiOf);

  function placeholder(src) {
    const def = parse(src, '') || [];
    const word = def.length === src.levels.length ? 'All' : 'default';
    return `${levelInputOf(def, src.levels, src.numbering)} (${word})`;
  }

  function render() {
    const src = source();
    if (!src) return;
    const list = parse(src, input.value);
    const on = new Set(list || []);
    input.classList.toggle('pb-input--invalid', !list);
    input.placeholder = none ? 'none' : placeholder(src);
    for (const b of picker.children) b.setAttribute('aria-pressed', String(on.has(+b.dataset.ui)));
    const full = shown(src).every((l) => on.has(uiOf(l)));
    allBtn.replaceChildren(el('span', null, 'Select'), el('span', null, full ? 'None' : 'All'));
    const def = (!none && parseLevelsIn('', src)) || [];
    const isDefault = !none && !!list && list.length === def.length && list.every((u, i) => u === def[i]);
    input.closest('.sode__keyed')?.classList.toggle('sode__keyed--default', isDefault);
  }

  function set(list) {
    const src = source();
    if (!src) return;
    none = list.length === 0;
    input.value = levelInputOf(list, src.levels, src.numbering);
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
  allBtn.addEventListener('click', () => {
    const src = source();
    if (!src) return;
    const on = new Set(levels() || []);
    set(allUi(src).every((u) => on.has(u)) ? [] : allUi(src));
  });

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
      const src = source();
      if (!src) return;
      input.value = relabelLevelsFor(input.value, src.levels, src.numbering, src.prev);
      const picks = shown(src);
      picker.replaceChildren(...levelPicks(picks, toggle));
      const odd = picks.length % 2;
      picker.classList.toggle('sode__levels--odd', !!odd);
      picker.style.setProperty('--sode-bar-cols', Math.floor(picks.length / 2) + odd);
      render();
    },
  };
}
