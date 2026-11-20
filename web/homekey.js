// The Home key, Alt+Shift+h, from any focus on every page (homekeyrules.js decides). Another page opens Home in this
// tab, as the Palette's Home row does. A page header's Home link (panelsbar.js) asks the page's unsaved-edits rule
// first, so the key presses that link where there is one. On Home the bus event 'homeKey' goes to homeview.js.
import { emit } from './core.js';
import { chord } from './keys.js';
import { pageOf } from './palettecatalogue.js';
import { pageRefusal } from './palette.js';
import { HOME_CHORD, homeKeyAct } from './homekeyrules.js';

const appRoot = () => new URL('./', document.baseURI);

function leave() {
  const link = document.querySelector('.sode-panels__head .sode-bar__home');
  if (link) link.click();
  else location.assign(appRoot());
}

// Capture, so it works from any focus, also inside a box or a dialog. preventDefault keeps macOS Option+Shift+H from
// typing Ó, also where the key does nothing.
document.addEventListener(
  'keydown',
  (e) => {
    if (chord(e) !== HOME_CHORD) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    if (e.repeat) return;
    const page = pageOf(location.pathname.slice(appRoot().pathname.length));
    const act = homeKeyAct({ page, refusal: pageRefusal() });
    if (act === 'here') emit('homeKey');
    else if (act === 'go') leave();
  },
  true,
);
