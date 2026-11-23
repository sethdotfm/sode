// Settings' warning under the Palette character (This browser and Everyone's defaults): the router names that start
// with it. Settings saves the character all the same; such a name is found with the character typed twice.
import { $, el, on, routerData } from './core.js';
import { pref } from './device.js';
import { paletteCharOf, namesStartingWith, startsNamesText } from './paletterules.js';

const notes = new Map();
function note(sel) {
  let n = notes.get(sel);
  if (n) return n;
  n = el('p', { class: 'sode__hint sode-palette__warn', role: 'status', hidden: true });
  sel.closest('.sode__field')?.append(n);
  notes.set(sel, n);
  return n;
}

function paint() {
  for (const [id, own] of [
    ['cfg-palette-char', true],
    ['gcfg-palette-char', false],
  ]) {
    const sel = $(id);
    if (!sel) continue;
    // An empty This browser choice follows the default, so the warning names the character in force.
    const ch = paletteCharOf(own && !sel.value ? pref('paletteChar') : sel.value);
    const text = startsNamesText(ch, namesStartingWith(ch, routerData('').srcs, routerData('').dsts));
    const n = note(sel);
    n.textContent = text;
    n.hidden = !text;
  }
}

for (const id of ['cfg-palette-char', 'gcfg-palette-char']) $(id)?.addEventListener('change', paint);
on('prefs', paint);
on('names', paint);
on('helloData', paint);
