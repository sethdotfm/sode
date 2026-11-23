// The Protect picker: the Palette's Protect row (palettehome.js) opens it on Home. It lists the destinations in the
// Rights, with chips for All destinations, Currently protected and Currently free. A row can be picked when the route
// view would offer Protect or Free Protect on it (protectOfferOf); the other rows show who holds them. A find box
// filters the list. The usual Confirm (the host's, with its churning letter) then sends the picks. A mix of free and
// protected picks protects first, so the safe action is the default; Free stays one button away. The server checks
// each one again.
import { el, S, on as onBus, icon, labelNode, routerData } from './core.js';
import { keyLabel, listStep } from './keys.js';
import { question } from './modal.js';
import { fuzzyScore } from './fuzzy.js';
import { boxHost } from './presets.js';
import { showResult } from './banner.js';
import { allowed } from './rightsrules.js';
import { PICK_FILTERS, pickRows, pickSplit, pickRange, filterRows } from './protectrules.js';
import { protectOfferOf, protectManyPlan, protectState, holdBadge } from './protects.js';

let open = null; // the shown picker's { close }

const EMPTY = {
  all: 'No destination in your Rights.',
  protected: 'No destination is protected now.',
  free: 'No destination is free now.',
};

export function openProtectPick() {
  if (open) return;
  const host = boxHost();
  if (!host) return showResult('warn', 'Not open', 'Go to Home, then try again.');
  let filter = 'all';
  const rows = () =>
    pickRows(S.dsts, filter, {
      inRights: (n) => allowed(S.myRights, 'dsts', n, routerData('').uid), // the first router's Protects
      held: (n) => protectState(n).levels.length > 0,
      offerOf: protectOfferOf,
    });
  const picked = new Set(); // destination numbers
  let shown = [];
  let cur = 0;
  let anchor = -1; // the row a Shift+click picks from
  // After a move with the keys, Space picks; after typing, Space types, since names have spaces. Ctrl+Space always
  // picks (not Cmd+Space, which the Mac keeps for itself).
  let browsing = false;

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
  const chips = PICK_FILTERS.map((f) => {
    const b = el('button', { class: 'pb-chip', type: 'button', 'data-filter': f.id }, f.label);
    b.addEventListener('mousedown', (e) => e.preventDefault()); // the find box keeps focus
    b.addEventListener('click', () => {
      filter = f.id;
      cur = 0;
      anchor = -1;
      paint();
    });
    return b;
  });
  const filters = el('div', { class: 'sode-protpick__filters', role: 'group', 'aria-label': 'Show' }, ...chips);
  const list = el('div', { class: 'sode-protpick__list', id: 'protpick-list', role: 'listbox', 'aria-multiselectable': 'true', 'aria-label': 'Destinations' });
  const count = el('span', { class: 'pb-badge sode-protpick__count', id: 'protpick-count', 'aria-live': 'polite' });
  const alt = el('button', { class: 'pb-btn pb-btn--outline', type: 'button', id: 'protpick-alt' });
  const go = el('button', { class: 'pb-btn', type: 'button', id: 'protpick-go' });
  const cancel = el('button', { class: 'pb-btn pb-btn--ghost', type: 'button' }, 'Cancel');
  const hint = el(
    'div',
    'sode-protpick__hint',
    el('kbd', null, keyLabel('ctrl+space')),
    ' picks. ',
    el('kbd', null, keyLabel('shift+click')),
    ' picks a range. ',
    el('kbd', null, keyLabel('enter')),
    ' goes on.',
  );

  const q = question({
    titleId: 'protpick-title',
    title: 'Protect',
    cls: 'sode-protpick',
    rootId: 'protpick',
    head: [icon('shield')],
    body: [filters, find, list, hint],
    foot: [count, cancel, alt, go],
    back: document.activeElement,
  });
  open = q;
  q.answer.then(() => {
    open = null;
    offs.forEach((f) => f());
  });

  function paint() {
    // A pick that can no longer be acted on (freed or taken elsewhere) drops out.
    for (const n of picked) if (!protectOfferOf(n)) picked.delete(n);
    for (const c of chips) c.setAttribute('aria-pressed', String(c.dataset.filter === filter));
    const live = rows();
    shown = filterRows(live, find.value, fuzzyScore);
    cur = Math.min(cur, Math.max(0, shown.length - 1));
    list.replaceChildren(
      ...(shown.length ? shown.map((d, i) => row(d, i)) : [el('div', 'sode__hist-note', live.length ? 'No destination matches.' : EMPTY[filter])]),
    );
    if (shown.length) find.setAttribute('aria-activedescendant', `protpick-opt-${cur}`);
    else find.removeAttribute('aria-activedescendant');
    list.querySelector('[data-cue]')?.scrollIntoView({ block: 'nearest' });
    count.textContent = `${picked.size} picked`;
    count.hidden = !picked.size;
    const s = pickSplit([...picked], protectOfferOf);
    const main = s.first === 'clear' ? 'clear' : 'set';
    go.className = 'pb-btn sode-btn--primary';
    go.textContent = main === 'set' ? (s.protect.length ? `Protect ${s.protect.length}` : 'Protect') : `Free ${s.clear.length}`;
    go.disabled = !s.first;
    // With a mix, Free is the second button: Enter and the main button protect.
    alt.hidden = !(s.protect.length && s.clear.length);
    alt.textContent = `Free ${s.clear.length}`;
  }

  function row(d, i) {
    const can = !!d.offer;
    const box = el('input', { class: 'pb-check__input', type: 'checkbox', tabIndex: -1, disabled: !can });
    box.checked = picked.has(d.n);
    const p = protectState(d.n);
    const r = el(
      'div',
      { class: 'sode-protpick__row', id: `protpick-opt-${i}`, role: 'option', 'aria-selected': String(box.checked), 'data-dst': String(d.n) },
      box,
      labelNode(d),
      ...p.holds.map(holdBadge),
    );
    if (!can) {
      r.setAttribute('aria-disabled', 'true');
      r.classList.add('sode-protpick__row--off');
    }
    if (i === cur) r.dataset.cue = '';
    r.addEventListener('mousedown', (e) => e.preventDefault()); // the find box keeps focus
    r.addEventListener('click', (e) => {
      if (e.shiftKey && anchor >= 0) {
        for (const n of pickRange(shown, anchor, i)) picked.add(n);
        cur = i;
        paint();
        return;
      }
      cur = i;
      anchor = i;
      flip(d);
    });
    return r;
  }

  function flip(d) {
    if (!d?.offer) return paint();
    if (picked.has(d.n)) picked.delete(d.n);
    else picked.add(d.n);
    paint();
  }

  // set: the action to send. With nothing picked, Enter goes on with the outlined row.
  function send(set) {
    if (!picked.size && shown[cur]?.offer) picked.add(shown[cur].n);
    const s = pickSplit([...picked], protectOfferOf);
    const want = set ?? s.first === 'set';
    const ns = want ? s.protect : s.clear;
    if (!ns.length) return paint();
    const byN = new Map(S.dsts.map((d) => [d.n, d]));
    q.close(true);
    host.confirm(
      protectManyPlan(
        ns.map((n) => byN.get(n)),
        want,
      ),
    );
  }

  find.addEventListener('input', () => {
    cur = 0;
    anchor = -1;
    browsing = false;
    paint();
  });
  find.addEventListener('keydown', (e) => {
    const plain = !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey;
    const step = listStep(e) || (plain && { ArrowDown: 1, ArrowUp: -1 }[e.key]) || 0;
    const ctrlSpace = e.key === ' ' && e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey;
    if (step) {
      e.preventDefault();
      if (!shown.length) return;
      cur = (cur + step + shown.length) % shown.length;
      browsing = true;
      paint();
    } else if (ctrlSpace || (e.key === ' ' && plain && (browsing || !find.value))) {
      e.preventDefault();
      anchor = cur;
      flip(shown[cur]);
    } else if (e.key === 'Enter' && plain) {
      e.preventDefault();
      send(null);
    }
  });
  go.addEventListener('click', () => send(null));
  alt.addEventListener('click', () => send(false));
  cancel.addEventListener('click', () => q.close(null));
  // A change on the router (a Protect set or freed elsewhere) changes what the list offers.
  const offs = ['protectedRows', 'names', 'hello', 'conn'].map((ev) => onBus(ev, paint));

  paint();
  find.focus();
}
