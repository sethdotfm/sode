// Rows and Columns steppers of the Panel tab. ed.setSize clamps and says what no longer fits (paneleditor.js).
import { el, icon } from './core.js';
import { btn } from './formbits.js';
import { ROWS_MAX, COLS_MAX } from './gridmodel.js';

function stepper(ed, which, text, max) {
  const input = el('input', 'pb-input sode-ed__num');
  input.id = `ed-${which}`;
  input.type = 'number';
  input.min = '1';
  input.max = String(max);
  input.inputMode = 'numeric';
  const step = (d, label, name) => {
    const b = btn('pb-btn--ghost pb-btn--icon pb-btn--sm', '', () => ed.setSize(which, ed.grid()[which] + d));
    b.id = `ed-${which}-${d < 0 ? 'dec' : 'inc'}`;
    b.setAttribute('aria-label', label);
    b.append(icon(name));
    return b;
  };
  const dec = step(-1, `Fewer ${text}`, 'minus');
  const inc = step(1, `More ${text}`, 'plus');
  input.addEventListener('change', () => ed.setSize(which, input.value));
  const label = Object.assign(el('label', null, text[0].toUpperCase() + text.slice(1)), { htmlFor: input.id });
  return {
    node: el('div', 'sode-side__step', label, dec, input, inc),
    update() {
      const ro = ed.readOnly();
      if (document.activeElement !== input) input.value = String(ed.grid()[which]);
      for (const n of [input, dec, inc]) n.disabled = ro;
    },
  };
}

export function createSizeFields(ed) {
  const rows = stepper(ed, 'rows', 'rows', ROWS_MAX);
  const cols = stepper(ed, 'cols', 'columns', COLS_MAX);
  return { node: el('div', 'sode-side__steps', rows.node, cols.node), update: () => (rows.update(), cols.update()) };
}
