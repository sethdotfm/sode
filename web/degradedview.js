// The Degraded note on the main page (desktop #conn-degraded, phone #m-degraded). While one or more routers are down
// and one or more are up, the Lost link banner does not show. The red icon of the down router and this note show
// which destinations a person cannot patch now (docs/tielines-design.md, Link state on the page). The words are in
// routerlinkrules.js.
import { el, icon, routerData } from './core.js';
import { degradedLine } from './routerlinkrules.js';

// c is connState(). Redraw only when the words change, so a screen reader does not read the note again on each paint.
export function renderDegraded(box, c) {
  const lines = c.degraded.map((r) => degradedLine(r, routerData(r.uid).dsts || []));
  const key = lines.join('\n');
  box.hidden = !lines.length;
  if (box.dataset.key === key) return;
  box.dataset.key = key;
  box.replaceChildren(...(lines.length ? [icon('triangle-alert', false), el('div', 'sode__degraded-lines', ...lines.map((t) => el('div', null, t)))] : []));
}
