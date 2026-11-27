// The Degraded note on the main page (desktop #conn-degraded, phone #m-degraded). While one or more routers are down
// and one or more are up, the Lost link banner does not show. The amber Routers icon and this note show which
// destinations a person cannot patch now (docs/tielines-design.md, Link state on the page). With three or more lost
// routers the note is one sentence with a Show routers button, which opens the router list (statusinfo.js). The words
// are in routerlinkrules.js.
import { el, emit, icon, routerData } from './core.js';
import { degradedNote } from './routerlinkrules.js';

// c is connState(). Redraw only when the words change, so a screen reader does not read the note again on each paint.
export function renderDegraded(box, c) {
  const { lines, more } = degradedNote(c.degraded, (r) => routerData(r.uid).dsts || []);
  const key = lines.join('\n');
  box.hidden = !lines.length;
  if (box.dataset.key === key) return;
  box.dataset.key = key;
  if (!lines.length) return box.replaceChildren();
  const kids = [icon('triangle-alert', false), el('div', 'sode__degraded-lines', ...lines.map((t) => el('div', null, t)))];
  if (more) {
    const show = el('button', { class: 'pb-btn pb-btn--outline pb-btn--sm sode__degraded-show', type: 'button' }, 'Show routers');
    show.addEventListener('click', () => emit('routerListOpen'));
    kids.push(show);
  }
  box.replaceChildren(...kids);
}
