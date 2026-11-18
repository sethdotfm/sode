// The chip and Clear button beside a Grid panel's Take button. The words come from pickNote
// (gridpanelphone.js); Clear does what Esc does (gridpanelkeys.js clearAction).
import { el } from './core.js';

const setText = (n, t) => {
  if (n.firstChild.data !== t) n.firstChild.data = t;
  if (n.hidden === !!t) n.hidden = !t;
};

// host is the Levels box; before is the node they go in front of (the Take button). onClear runs on a press.
export function clearNote(host, before, onClear) {
  const chip = el('span', 'pb-badge pb-badge--info sode-gridpanel__chip', document.createTextNode(''));
  const note = el('div', 'sode-gridpanel__note', chip);
  note.setAttribute('role', 'status');
  const clear = el('button', 'pb-btn sode-gridpanel__clear', 'Clear');
  clear.type = 'button';
  clear.id = 'panel-clear';
  clear.setAttribute('aria-keyshortcuts', 'Escape');
  clear.classList.add('pb-tip--end');
  // Clear drops every staged route; the tip gives the way to drop only one.
  clear.dataset.pbTip = 'Esc. To unstage one route, right-click it, Alt-click it or press Delete on it';
  clear.addEventListener('click', onClear);
  // A press keeps focus where it was, so the grid keeps its keys.
  clear.addEventListener('mousedown', (e) => e.preventDefault());
  host.insertBefore(note, before);
  host.insertBefore(clear, before);
  return {
    // n is pickNote's words; can is true when Clear has something to drop.
    paint(n, can) {
      setText(chip, n);
      if (clear.disabled === can) clear.disabled = !can;
    },
    remove() {
      note.remove();
      clear.remove();
    },
  };
}
