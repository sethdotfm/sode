// The top bar of /panels, the panel editor and the Live Editor, made after the Grid head row (gridpanelheadview.js).
// It has the title, a group of icon links, and the Server and Router status at the right. Panel pages keep their own.
import { el, icon, on } from './core.js';
import { connBox, paintConn } from './connview.js';

// core.js's on() has no unsubscribe, so one subscription paints every bar's status.
const conns = new Set();
on('conn', () => {
  for (const c of conns) paintConn(c);
});

// A ghost icon link with its tip, as on the Grid head row.
export function iconLink(cls, name, label, href) {
  const n = el('a', `pb-btn pb-btn--ghost pb-btn--icon ${cls}`, icon(name, false));
  n.href = href;
  n.setAttribute('aria-label', label);
  n.dataset.pbTip = label;
  return n;
}

// The Sode Panels logo, a link to /panels.
function panelsLink() {
  const logo = el('span', 'sode-logo sode-panels__logo');
  logo.setAttribute('aria-hidden', 'true');
  const n = el('a', 'pb-btn pb-btn--ghost pb-btn--icon sode-bar__panels', logo);
  n.href = 'panels';
  n.setAttribute('aria-label', 'Panels');
  n.dataset.pbTip = 'Panels';
  return n;
}

// A plain click on a link asks leave(href) first; false keeps the page. A click that opens a new tab or window leaves
// nothing, so it goes as usual.
function guard(a, leave) {
  a.addEventListener('click', (e) => {
    if (e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
    if (!leave(a.getAttribute('href'))) e.preventDefault();
  });
}

// Settings, then Panels (with panels), then Home. leave is the page's unsaved-edits rule (paneleditor.js).
export function barLinks({ panels = false, leave = null } = {}) {
  const links = [
    // The main page's Settings button (index.html #settings-open), as a link: settings.js opens the view at #settings.
    iconLink('sode-bar__settings', 'settings', 'Settings', './#settings'),
    panels ? panelsLink() : null,
    iconLink('sode-bar__home', 'house', 'Sode', './'),
  ].filter(Boolean);
  if (leave) for (const a of links) guard(a, leave);
  return links;
}

// Makes head (a .sode-panels__head with its title) the bar: the links go after the title and the status goes last.
export function panelsBar(head, opts = {}) {
  const tools = el('div', 'sode-bar__tools', ...barLinks(opts));
  head.querySelector('.sode-panels__title').after(tools);
  const conn = connBox();
  conns.add(conn);
  paintConn(conn);
  head.classList.add('sode-bar');
  head.append(conn.root);
  return tools;
}
