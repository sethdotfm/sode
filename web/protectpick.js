// The Protect picker: the Palette's Protect and Clear Protect rows (palettehome.js) open it on Home. It lists the
// destinations where the route view would offer the same (protectOfferOf) and that are in the Rights. A find box
// filters the list; the user picks one or more, then the usual Confirm (the host's, with its churning letter) sends
// them. The server checks each one again.
import { el, S, on as onBus, icon, labelNode } from './core.js';
import { keyLabel } from './keys.js';
import { question } from './modal.js';
import { fuzzyScore } from './fuzzy.js';
import { boxHost } from './presets.js';
import { showResult } from './banner.js';
import { allowed } from './rightsrules.js';
import { holdLabel, pickRows, filterRows } from './protectrules.js';
import { protectOfferOf, protectManyPlan, protectState } from './protects.js';

let open = null; // the shown picker's { close }

// set: true lists destinations to Protect, false the ones with Sode's Protect to clear.
export function openProtectPick(set) {
  if (open) return;
  const host = boxHost();
  if (!host) return showResult('warn', 'Not open', 'Go to Home, then try again.');
  const want = set ? 'set' : 'clear';
  const rows = () => pickRows(S.dsts, want, protectOfferOf, (n) => allowed(S.myRights, 'dsts', n));
  const picked = new Set(); // destination numbers
  let shown = [];
  let cur = 0;

  const find = el('input', {
    class: 'pb-input sode-protpick__find',
    id: 'protpick-find',
    type: 'text',
    placeholder: 'Find a destination',
    autocomplete: 'off',
    spellcheck: false,
    role: 'combobox',
    'aria-expanded': 'true',
    'aria-controls': 'protpick-list',
    'aria-label': 'Find a destination',
  });
  const list = el('div', { class: 'sode-protpick__list', id: 'protpick-list', role: 'listbox', 'aria-multiselectable': 'true', 'aria-label': 'Destinations' });
  const count = el('span', { class: 'sode-protpick__count', 'aria-live': 'polite' });
  const all = el('button', { class: 'pb-btn pb-btn--ghost pb-btn--sm', type: 'button', id: 'protpick-all' }, 'Pick all shown');
  const go = el('button', { class: `pb-btn ${set ? 'pb-btn--primary' : 'pb-btn--danger'}`, type: 'button', id: 'protpick-go' });
  const cancel = el('button', { class: 'pb-btn pb-btn--ghost', type: 'button' }, 'Cancel');
  const hint = el(
    'div',
    'sode-protpick__hint',
    el('kbd', null, keyLabel('enter')),
    ' picks the outlined row. ',
    el('kbd', null, keyLabel('ctrl+enter')),
    ' goes to the Confirm.',
  );

  const q = question({
    titleId: 'protpick-title',
    title: set ? 'Protect' : 'Clear Protect',
    cls: 'sode-protpick',
    rootId: 'protpick',
    head: [icon(set ? 'shield' : 'shield-off')],
    body: [
      el(
        'div',
        'sode__hist-note',
        set ? 'Pick the destinations to Protect on every level.' : "Pick the destinations to clear. The list shows Sode's own Protects only.",
      ),
      find,
      list,
      hint,
    ],
    foot: [all, count, cancel, go],
    back: document.activeElement,
  });
  open = q;
  q.answer.then(() => {
    open = null;
    offs.forEach((f) => f());
  });

  function paint() {
    const live = rows();
    const liveN = new Set(live.map((d) => d.n));
    for (const n of picked) if (!liveN.has(n)) picked.delete(n);
    shown = filterRows(live, find.value, fuzzyScore);
    cur = Math.min(cur, Math.max(0, shown.length - 1));
    list.replaceChildren(
      ...(shown.length
        ? shown.map((d, i) => row(d, i))
        : [
            el(
              'div',
              'sode__hist-note',
              live.length ? 'No destination matches.' : set ? 'No destination can take a Protect now.' : 'Sode has no Protect to clear here.',
            ),
          ]),
    );
    if (shown.length) find.setAttribute('aria-activedescendant', `protpick-opt-${cur}`);
    else find.removeAttribute('aria-activedescendant');
    list.querySelector('[data-cue]')?.scrollIntoView({ block: 'nearest' });
    count.textContent = `${picked.size} picked`;
    go.textContent = picked.size ? `${set ? 'Protect' : 'Clear'} ${picked.size}` : set ? 'Protect' : 'Clear';
    go.disabled = !picked.size;
    all.disabled = !shown.length || shown.every((d) => picked.has(d.n));
  }

  function row(d, i) {
    const box = el('input', { class: 'pb-check__input', type: 'checkbox', tabIndex: -1 });
    box.checked = picked.has(d.n);
    const p = protectState(d.n);
    const hold = !set && p.sode ? [el('span', 'pb-badge pb-badge--warn', holdLabel(p.sode))] : [];
    const r = el(
      'div',
      { class: 'sode-protpick__row', id: `protpick-opt-${i}`, role: 'option', 'aria-selected': String(box.checked), 'data-dst': String(d.n) },
      box,
      labelNode(d),
      ...hold,
    );
    if (i === cur) r.dataset.cue = '';
    r.addEventListener('mousedown', (e) => e.preventDefault()); // the find box keeps focus
    r.addEventListener('click', () => {
      cur = i;
      flip(d.n);
    });
    return r;
  }

  function flip(n) {
    if (picked.has(n)) picked.delete(n);
    else picked.add(n);
    paint();
  }

  function confirm() {
    const live = rows().filter((d) => picked.has(d.n));
    if (!live.length) return;
    q.close(true);
    host.confirm(protectManyPlan(live, set));
  }

  find.addEventListener('input', () => {
    cur = 0;
    paint();
  });
  find.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (!shown.length) return;
      cur = (cur + (e.key === 'ArrowDown' ? 1 : -1) + shown.length) % shown.length;
      paint();
    } else if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      confirm();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (shown[cur]) flip(shown[cur].n);
    }
  });
  all.addEventListener('click', () => {
    for (const d of shown) picked.add(d.n);
    paint();
  });
  go.addEventListener('click', confirm);
  cancel.addEventListener('click', () => q.close(null));
  // A change on the router (a Protect set or cleared elsewhere) changes what the list offers.
  const offs = ['protectedRows', 'names', 'hello', 'conn'].map((ev) => onBus(ev, paint));

  paint();
  find.focus();
}
