// The change goes into the filter like a hand edit, and Save stores it. Rules: brokenrules.js.
import { el, off } from './core.js';
import { replaceRows, replaceTerm } from './brokenrules.js';

const SIDE_TEXT = { src: 'Sources', dst: 'Destinations' };

// io: { set() the active set entry or null, read() the filter shown, put(f), reload() }.
export function replaceBox(io) {
  const box = el('div', 'pb-alert pb-alert--warn sode__filters-replace');
  box.setAttribute('role', 'group');
  box.setAttribute('aria-label', 'Broken terms');
  box.hidden = true;
  const paint = () => {
    const s = io.set();
    const rows = s ? replaceRows({ f: io.read(), broken: s.broken }, off()) : [];
    box.hidden = !rows.length;
    box.replaceChildren(
      ...(rows.length ? [el('div', 'pb-alert__title', 'Broken terms')] : []),
      ...rows.map((r) => {
        const input = el('input', 'pb-input');
        input.value = r.now;
        input.placeholder = 'New name or number';
        input.setAttribute('aria-label', `New term for ${r.term}`);
        const go = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Replace term');
        go.type = 'button';
        go.addEventListener('click', () => {
          const f = io.read();
          const next = replaceTerm(f[r.side], r.mark, off(), input.value);
          if (next === f[r.side]) return input.focus();
          io.put({ ...f, [r.side]: next });
          io.reload();
          paint();
        });
        return el('div', 'sode__filters-replace-row', el('span', null, `${SIDE_TEXT[r.side]}: `, el('code', null, r.term)), input, go);
      }),
    );
  };
  return { box, paint };
}
