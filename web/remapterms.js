// The remap step's filter term rows (importremap.js): one per number or range term whose matches differ between
// the file and the target. Rules are in remaptermrules.js.

import { el } from './core.js';

const SIDE = { src: 'Sources', dst: 'Destinations' };
export const TERM_CHOICE = { keep: 'Keep numbers', names: 'Convert to names', skip: 'Skip term' };

const setsText = (router, keys) => {
  const names = keys.map((k) => router.sets.find((s) => s.key === k)).map((s) => (s.user ? `${s.name} (${s.user})` : s.name));
  return `${names.length === 1 ? 'filter set' : 'filter sets'} ${names.join(', ')}`;
};

// choices: row key → keep | names | skip. onChange after a choice.
export function termView(router, rows, choices, onChange) {
  if (!rows.length) return [];
  const box = el('div', 'sode__xi-names sode__xi-terms');
  for (const row of rows) {
    const sel = el('select', 'pb-select');
    for (const [v, t] of Object.entries(TERM_CHOICE)) {
      if (v === 'names' && !row.canConvert) continue;
      const o = el('option', null, t);
      o.value = v;
      sel.append(o);
    }
    sel.value = choices[row.key];
    sel.dataset.key = `term:${row.key}`;
    sel.setAttribute('aria-label', `${SIDE[row.side]} ${row.label}`);
    sel.addEventListener('change', () => {
      choices[row.key] = sel.value;
      onChange();
    });
    const hint = row.canConvert ? `In the file: ${row.names.join(', ')}` : 'No names at these numbers in the file';
    const r = el(
      'div',
      'sode__xi-name',
      el('span', null, el('strong', null, row.label), ' ', el('span', 'sode__hint', `${SIDE[row.side]}, ${setsText(router, row.sets)}`)),
      el('span', 'sode__xi-pick', el('span', 'pb-select-wrap', sel)),
      el('span', 'sode__hint sode__xi-sub', hint),
    );
    r.dataset.term = row.key;
    box.append(r);
  }
  return [el('p', 'sode__label sode__xi-sub', 'Filter terms whose names differ here'), box];
}
