// The one page header of every page but the main page and a panel page: /panels, the panel editor (its Live Editor
// too) and /log. The Lock bar floats at its left (lock.css). Then come the site name, a group of icon links, a gap,
// Inbox and the status. base.css lays it out. Panel pages keep their own.
import { el, icon, on } from './core.js';
import { connBox, paintConn } from './connview.js';
import { mac } from './keys.js';

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

// Home, with the Home key that presses it from any focus (homekey.js).
function homeLink() {
  const n = iconLink('sode-bar__home', 'house', 'Sode', './');
  n.dataset.pbTip = `Sode (${mac ? '⌥⇧h' : 'Alt+Shift+h'})`;
  n.setAttribute('aria-keyshortcuts', 'Alt+Shift+H');
  return n;
}

// Settings, then Panels (with panels), then Home. leave is the page's unsaved-edits rule (paneleditor.js).
export function barLinks({ panels = false, leave = null } = {}) {
  const links = [
    // The main page's Settings button (index.html #settings-open), as a link: settings.js opens the view at #settings.
    iconLink('sode-bar__settings', 'settings', 'Settings', './#settings'),
    panels ? panelsLink() : null,
    homeLink(),
  ].filter(Boolean);
  if (leave) for (const a of links) guard(a, leave);
  return links;
}

// Sets the site name in a bar's title. The name can be cut with an ellipsis, so the tip gives it in full.
export function barTitle(title, text) {
  const name = title.querySelector('.sode-bar__name');
  if (!name) {
    title.textContent = text;
    return;
  }
  name.textContent = text;
  title.dataset.pbTip = text;
}

// Makes head (a header with its .sode-panels__title) the bar: the links go after the title and the status goes last.
export function panelsBar(head, opts = {}) {
  const title = head.querySelector('.sode-panels__title');
  // The ellipsis goes on an inner span, since the title's own overflow would cut its tip.
  const text = title.textContent;
  title.replaceChildren(el('span', 'sode-bar__name'));
  barTitle(title, text);
  const tools = el('div', 'sode-bar__tools', ...barLinks(opts));
  title.after(tools);
  const conn = connBox();
  conns.add(conn);
  paintConn(conn);
  head.classList.add('sode-bar');
  head.append(conn.root);
  return tools;
}
