// The Panel tab's Fixed destination field: the chosen Destinations by name, each with a remove button, and a
// picker over Find's search (findrules.js fixedPick) that honours the signed-in user's Rights and Visibility.
// Rules are findrules.js and sidebarrules.js (addFixed, dropFixed); this file is the DOM.
import { el, icon } from './core.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { resultStep, chord } from './keys.js';
import { findEntries, fixedPick, offTag } from './findrules.js';
import { addFixed, dropFixed, FIXED_MAX } from './sidebarrules.js';
import { fixedDestsOf } from './panelrules.js';
import { myRights, onMyRights } from './editorrights.js';

const SHOWN = 50; // rows drawn; typing narrows the rest
const STALE_MS = 5000;

export function createFixedField(ed) {
  let data = { names: null, at: 0, loading: false };
  let results = [];
  let hi = 0;
  let open = false;
  let drawnKey = '';

  const set = (fn) => ed.setFields({ settings: fn(ed.fields().settings) });
  const chosen = () => fixedDestsOf({ settings: ed.fields().settings });

  const help = el('span', 'sode-fixed__help', icon('circle-help'));
  help.tabIndex = 0;
  help.setAttribute('aria-label', 'About fixed destinations');
  help.setAttribute('data-pb-tip', 'A Source or Preset press patches to these, with no Destination cell to press first. Empty is off.');
  const list = el('ul', 'sode-fixed__chosen');
  list.id = 'ed-fixed-list';
  list.setAttribute('aria-label', 'Fixed destinations');
  const input = el('input', 'pb-input');
  input.id = 'ed-fixed-find';
  input.type = 'search';
  input.setAttribute('aria-label', 'Fixed destination');
  input.autocomplete = 'off';
  input.placeholder = 'Add fixed panel destinations';
  input.setAttribute('role', 'combobox');
  input.setAttribute('aria-controls', 'ed-fixed-results');
  input.setAttribute('aria-expanded', 'false');
  const found = el('ul', 'sode-find__list sode-fixed__results');
  found.id = 'ed-fixed-results';
  found.setAttribute('role', 'listbox');
  found.setAttribute('aria-label', 'Destinations to add');
  found.hidden = true;
  const note = el('span', 'sode__hint');
  note.setAttribute('role', 'status');
  const node = el(
    'div',
    'sode__field sode-fixed',
    el(
      'span',
      'sode__field-label',
      'Fixed destinations ',
      help,
    ),
    list,
    input,
    found,
    note,
  );

  async function load() {
    if (data.loading || Date.now() - data.at < STALE_MS) return;
    data.loading = true;
    const names = await jsonCall(sessionFetch)('api/v1/names').catch(() => null);
    data = { names, at: Date.now(), loading: false };
    draw();
  }

  function add(e) {
    if (!e || e.off) return;
    input.value = '';
    hi = 0;
    set((s) => addFixed(s, e.n));
    input.focus({ preventScroll: true });
  }

  function drawResults() {
    const full = chosen().length >= FIXED_MAX;
    const entries = findEntries(data.names, null, myRights());
    results = full ? [] : fixedPick(entries, input.value, chosen());
    hi = Math.max(0, Math.min(hi, results.length - 1));
    found.hidden = !open;
    input.setAttribute('aria-expanded', String(open));
    note.textContent = full ? `At most ${FIXED_MAX} fixed destinations.` : '';
    if (!open) return;
    const rows = results.slice(0, SHOWN).map((e, i) => {
      const li = el(
        'li',
        `sode-find__row sode-fixed__row${e.off ? ' sode__menu-off' : ''}${i === hi ? ' is-hi' : ''}`,
        icon('arrow-right-to-line'),
        el('span', 'sode__num', String(e.num)),
        el('span', 'sode-find__name', e.label),
      );
      if (e.off) li.append(el('span', 'sode__menu-tag', offTag(e)));
      li.setAttribute('role', 'option');
      li.setAttribute('aria-selected', String(i === hi));
      li.setAttribute('aria-disabled', String(!!e.off));
      li.addEventListener('click', () => add(e));
      return li;
    });
    const none = !data.names ? 'Loading the router names.' : full ? '' : results.length ? '' : 'No Destination matches.';
    found.replaceChildren(...rows, ...(none ? [el('li', 'sode-find__none', none)] : []));
    found.querySelector('.is-hi')?.scrollIntoView({ block: 'nearest' });
  }

  function drawChosen() {
    const names = ed.names();
    const ro = ed.readOnly();
    const now = chosen();
    // Rebuilt only when it changed, so a focused remove button is not swapped out under the keyboard.
    const key = JSON.stringify([ro, now.map((n) => [n, names.destination(n)])]);
    if (key === drawnKey) return;
    drawnKey = key;
    list.replaceChildren(
      ...now.map((n) => {
        const name = names.destination(n);
        const remove = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm', icon('x'));
        remove.type = 'button';
        remove.disabled = ro;
        remove.setAttribute('aria-label', `Remove ${name}`);
        remove.addEventListener('click', () => set((s) => dropFixed(s, n)));
        const li = el('li', 'sode-fixed__dst', el('span', 'sode__num', String(n + (names.off || 0))), el('span', 'sode-find__name', name), remove);
        li.dataset.n = String(n);
        return li;
      }),
    );
  }

  function draw() {
    drawChosen();
    drawResults();
  }

  input.addEventListener('focus', () => {
    open = true;
    load();
    draw();
  });
  input.addEventListener('blur', () => {
    open = false;
    draw();
  });
  found.addEventListener('pointerdown', (e) => e.preventDefault()); // keeps focus in the box, so a click adds
  input.addEventListener('input', () => {
    hi = 0;
    open = true;
    load();
    draw();
  });
  input.addEventListener('keydown', (e) => {
    const step = resultStep(e, results.length);
    if (step) {
      e.preventDefault();
      hi = (hi + step + results.length) % results.length;
      draw();
    } else if (chord(e) === 'enter') {
      e.preventDefault();
      add(results[hi]);
    } else if (chord(e) === 'escape' && input.value) {
      e.preventDefault();
      input.value = '';
      draw();
    }
  });
  onMyRights(() => draw());

  return {
    node,
    update() {
      input.disabled = ed.readOnly();
      draw();
    },
  };
}
