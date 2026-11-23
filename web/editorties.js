// The Panel tab's tie line setting (docs/tielines-design.md, Panels): Off or Reserved, and for Reserved one tie line
// for each one-way link between two routers. Rules in paneltierules.js. The server checks the setting at the save:
// a tie line that another panel Reserves cannot be chosen here, and the save refuses it too.
import { el, routerData } from './core.js';
import { segmented } from './formbits.js';
import { sessionFetch } from './sessionkey.js';
import { linksOf, reservedOn, setReserve, heldBy, tiesOn } from './paneltierules.js';

const routerWord = (uid) => `router ${routerData(uid).prefix || uid}`;

export function createTieField(ed) {
  let list = null; // GET /tielines, or null until it comes
  const set = (tieLines) => ed.setFields({ tieLines });
  const mode = segmented(
    'ed-set-ties',
    'Tie lines',
    [
      ['off', 'Off'],
      ['reserved', 'Reserved'],
    ],
    (v) => set(v === 'reserved' ? { mode: 'reserved', reserved: ed.fields().tieLines?.reserved || [] } : { mode: 'off' }),
  );
  const links = el('div', 'sode-side__ties');
  const hint = el('span', 'sode__hint');
  hint.id = 'ed-ties-note';
  const node = el('div', 'sode__field', el('span', 'sode__field-label', 'Tie lines'), mode.node, hint, links);

  async function read() {
    const r = await sessionFetch('api/v1/tielines').catch(() => null);
    list = r?.ok ? (await r.json()).tieLines || [] : [];
    draw();
  }

  function draw() {
    const setting = ed.fields().tieLines;
    const on = tiesOn({ tieLines: setting });
    const ro = ed.readOnly();
    const id = ed.panel()?.id || '';
    mode.set(on ? 'reserved' : 'off');
    mode.disable(ro);
    hint.textContent = on
      ? 'A route across routers always uses the Reserved tie line of its link. A link with no Reserved tie line is refused.'
      : 'Off: the panel refuses a route across routers.';
    links.replaceChildren();
    links.hidden = !on;
    if (!on || !list) return;
    const all = linksOf(list);
    if (!all.length) links.append(el('span', 'sode__hint', 'Sode has no tie lines.'));
    for (const link of all) {
      const s = el('select', 'pb-select');
      s.id = `ed-ties-${link.from}-${link.to}`;
      s.disabled = ro;
      s.append(Object.assign(el('option', null, 'None'), { value: '' }));
      for (const t of link.ties) {
        const by = heldBy(t, id);
        const o = Object.assign(el('option', null, by ? `${t.name}, Reserved by panel ${by}` : t.name), { value: t.id, disabled: !!by });
        s.append(o);
      }
      s.value = reservedOn(setting, link);
      s.addEventListener('change', () => set(setReserve(ed.fields().tieLines, link, s.value)));
      const label = Object.assign(el('label', null, `${routerWord(link.from)} to ${routerWord(link.to)}`), { htmlFor: s.id });
      links.append(el('div', 'sode__field', label, el('span', 'pb-select-wrap', s)));
    }
  }

  return {
    node,
    // A panel load or save reads the list again, so a Reserve of another panel shows.
    read,
    update: draw,
  };
}
