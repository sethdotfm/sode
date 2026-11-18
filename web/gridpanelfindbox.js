// The Source bar and Destination bar search boxes of a Grid panel. The rules are in gridpanelfind.js and
// the keys in gridpanelkeys.js (findBoxAction); the run (gridpanelrun.js) joins them.
import { el, icon } from './core.js';
import { segmented } from './formbits.js';

const NOUN = { srcs: 'sources', dsts: 'destinations' };
const KEY = { srcs: 's', dsts: 'd' };
const MODE = { find: 'Find', filter: 'Filter' };

// Each bar's step buttons point along it. Keep the key names in step with BOX in gridpanelkeys.js.
const STEPS = {
  srcs: [
    [-1, 'chevron-left', 'Previous match. Shift+Enter or Ctrl+P'],
    [1, 'chevron-right', 'Next match. Enter or Ctrl+N'],
  ],
  dsts: [
    [-1, 'chevron-up', 'Previous match. Shift+Enter or Ctrl+P'],
    [1, 'chevron-down', 'Next match. Enter or Ctrl+N'],
  ],
};

// host is the deck's search slot, or g.head in the editor's live test. One mode serves both boxes. dir is 1 for the
// next match. The step buttons show only while the box has text, which is when the placeholder is gone.
export function findBoxes(host, { onText, onMode, onStep }) {
  const make = (kind) => {
    const input = el('input', 'pb-input sode-gridpanel__find-input');
    input.type = 'text';
    input.autocomplete = 'off';
    input.spellcheck = false;
    input.placeholder = `${MODE.find} ${NOUN[kind]}`;
    input.setAttribute('aria-label', input.placeholder);
    input.setAttribute('aria-keyshortcuts', KEY[kind]);
    const count = el('span', 'sode-gridpanel__find-count', document.createTextNode(''));
    count.setAttribute('aria-live', 'polite');
    const key = el('kbd', 'sode-gridpanel__find-key', KEY[kind]);
    key.setAttribute('aria-hidden', 'true');
    // Tab goes between the boxes, so the buttons are out of the Tab order, as the mode button is.
    const steps = STEPS[kind].map(([dir, name, tip]) => {
      const b = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode-gridpanel__find-step', icon(name));
      b.type = 'button';
      b.tabIndex = -1;
      b.hidden = true;
      b.dataset.dir = String(dir);
      b.dataset.pbTip = tip;
      b.setAttribute('aria-label', tip.split('.')[0]);
      b.addEventListener('click', () => {
        onStep(kind, dir);
        input.focus();
      });
      return b;
    });
    const wrap = el('div', 'sode-gridpanel__find', el('span', 'sode-gridpanel__find-field', input, count, key), ...steps);
    wrap.dataset.kind = kind;
    wrap.dataset.mode = 'find';
    input.addEventListener('input', () => onText(kind, input.value));
    return { wrap, input, count, steps };
  };
  const boxes = { srcs: make('srcs'), dsts: make('dsts') };
  let last = 'srcs';
  let now = 'find';
  // One switch is the mode of both boxes: on is Filter, off is Find. A switch, not a button, so it reads as a mode
  // and not as an action. A click goes back to the box, so Tab still goes between the boxes.
  const sw = el('input', 'pb-switch__input');
  sw.type = 'checkbox';
  sw.setAttribute('role', 'switch');
  sw.tabIndex = -1;
  sw.addEventListener('change', () => (sw.checked ? 'filter' : 'find') !== now && onMode());
  // The word names the mode now on (paint), and the tip says what each does. The name stays "Filter", so a screen
  // reader reads "Filter, on" or "Filter, off".
  const word = el('span', 'sode-gridpanel__find-word', document.createTextNode(MODE.find));
  const words = el('span', 'pb-check__label', word);
  word.setAttribute('aria-hidden', 'true');
  sw.setAttribute('aria-label', MODE.filter);
  const mode = el('label', 'pb-check sode-gridpanel__find-mode', sw, words);
  mode.dataset.pbTip = 'On: the bars show only the matches. Off: Find marks the matches. Ctrl+/';
  // On a phone a focused box opens the keyboard, so a mode change leaves focus where it is.
  mode.addEventListener('click', () => document.documentElement.dataset.layout !== 'phone' && boxes[last].input.focus());
  // The phone shows the mode as two named choices in place of the switch (gridpanelphone.css).
  const choice = segmented(
    'panel-find-mode',
    'Search mode',
    [
      ['find', MODE.find],
      ['filter', MODE.filter],
    ],
    (v) => v !== now && onMode(),
  );
  choice.set(now);
  const choiceRow = el('div', 'sode-gridpanel__find-choice', choice.node);
  host.prepend(choiceRow, boxes.srcs.wrap, boxes.dsts.wrap, mode);
  return {
    input: (kind) => boxes[kind].input,
    // 'srcs' or 'dsts' when node is in that box, else null.
    kindOf(node) {
      for (const k of ['srcs', 'dsts']) if (node instanceof Node && boxes[k].wrap.contains(node)) return k;
      return null;
    },
    // find is gridpanelfind.js state; counts[kind] is { n: matches, at: the current match's place from 1 }.
    paint(find, counts) {
      last = find.last;
      now = find.mode;
      if (sw.checked !== (now === 'filter')) sw.checked = now === 'filter';
      if (word.firstChild.data !== MODE[now]) word.firstChild.data = MODE[now];
      choice.set(now);
      for (const k of ['srcs', 'dsts']) {
        const b = boxes[k];
        const box = find[k];
        if (b.input.value !== box.text) b.input.value = box.text;
        if (b.wrap.dataset.mode !== find.mode) b.wrap.dataset.mode = find.mode;
        const word = `${MODE[find.mode]} ${NOUN[k]}`;
        if (b.input.placeholder !== word) b.input.placeholder = word;
        if (b.input.getAttribute('aria-label') !== word) b.input.setAttribute('aria-label', word);
        const c = counts[k];
        const text = !box.text.trim() ? '' : c.n ? `${c.at} of ${c.n}` : 'No match';
        if (b.count.firstChild.data !== text) b.count.firstChild.data = text;
        b.wrap.toggleAttribute('data-counted', !!text);
        for (const s of b.steps) {
          if (s.hidden !== !text) s.hidden = !text;
          if (s.disabled !== !c.n) s.disabled = !c.n;
        }
      }
    },
  };
}
