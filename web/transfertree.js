// The tree in Export, Import and the Inbox review: category heads, item rows and the clash choice. data-key names a
// control across redraws, so keepFocus can find it again.
import { el, icon } from './core.js';
import { btn, segmented } from './formbits.js';
import { sizeText } from './exportrules.js';

export function tick(key, text, { checked = false, mixed = false, disabled = false, onChange }) {
  const box = el('input', { class: 'pb-check__input', type: 'checkbox', checked, indeterminate: mixed, disabled, 'data-key': key });
  box.addEventListener('change', () => onChange(box.checked));
  return el('label', 'pb-check', box, el('span', 'pb-check__label', text));
}

export const sizeEl = (n) => el('span', 'sode__xi-size', n == null ? '' : sizeText(n));

export function caret(expanded, cat, text, redraw) {
  const open = expanded.has(cat);
  const b = btn('pb-btn--ghost pb-btn--icon pb-btn--sm sode__xi-caret', icon(open ? 'chevron-down' : 'chevron-right'), () => {
    if (open) expanded.delete(cat);
    else expanded.add(cat);
    redraw();
  });
  b.dataset.key = `caret:${cat}`;
  b.setAttribute('aria-expanded', String(open));
  b.setAttribute('aria-label', `Show the ${text.toLowerCase()} items`);
  return b;
}

// lead: the caret, or null for a blank slot that keeps the columns.
export function catHead(cat, lead, box, hint = '', size = null, hintCls = '') {
  const head = el('div', 'sode__xi-head', lead || el('span'), box, el('span', `sode__hint ${hintCls}`.trim(), hint), sizeEl(size));
  head.dataset.cat = cat;
  return head;
}

// A category that only an Administrator moves, shown so the list of the file is complete.
export const adminRow = (cat, text) => catHead(`admin:${cat}`, null, tick(`admin:${cat}`, text, { disabled: true, onChange() {} }), 'Administrators only');

// choice: { id, options, value, onPick, lead } for an item that clashes with one here.
export function planRow({ cat, it, ticked, disabled, onTick, broken, hint, size, choice, missing }) {
  const row = el(
    'div',
    { class: 'sode__xi-item', 'data-item': it.key },
    tick(`item:${cat}:${it.key}`, it.name, { checked: ticked, disabled, onChange: onTick }),
    el('span', 'sode__hint', ...(broken ? [el('span', 'pb-badge pb-badge--error', 'Broken'), ' '] : []), hint),
    ...(size == null ? [] : [sizeEl(size)]),
  );
  if (choice) {
    const seg = segmented(choice.id, `${it.name} clash`, choice.options, choice.onPick);
    seg.set(choice.value);
    for (const r of seg.node.querySelectorAll('input')) r.dataset.key = `choice:${cat}:${it.key}:${r.value}`;
    seg.node.classList.add('sode__xi-choice');
    row.append(el('div', 'sode__xi-sub', ...(choice.lead ? [el('span', 'sode__hint', choice.lead)] : []), seg.node));
  }
  if (missing) row.append(el('span', 'sode__hint sode__xi-sub sode__error-text', `Missing ${missing}`));
  return row;
}

export function keepFocus(box, draw) {
  const focused = box.contains(document.activeElement) ? document.activeElement.dataset.key : '';
  draw();
  if (focused) [...box.querySelectorAll('[data-key]')].find((x) => x.dataset.key === focused)?.focus();
}
