// The one page header of every page but a panel page: the main page (homebar.js), /panels, the panel editor (its
// Live Editor too) and /log. The Lock bar floats at its left (lock.css). Then come the site name, a rule, the icon
// links, a gap, Inbox, a rule and the status. Off Home the title goes to the bar's middle and the rule stays only
// after the Lock bar. bar.css lays it out. Panel pages keep their own.
import { $, el, icon, on, emit, connState, renderDown } from './core.js';
import { connBox, paintConn } from './connview.js';
import { keyLabel, ariaKeys } from './keys.js';
import { HOME_CHORD } from './homekeyrules.js';

// core.js's on() has no unsubscribe, so one subscription paints every bar's status. A page whose bar makes its own
// status also gets the Lost link banner, #conn-down. Home draws its own banner (desktop.js, mobile.js).
const conns = new Set();
let banner = false;
on('conn', () => {
  for (const c of conns) paintConn(c);
  if (banner) paintDown();
});

function paintDown() {
  const box = $('conn-down');
  if (!box) return;
  const c = connState();
  document.documentElement.dataset.conn = c.down ? 'down' : 'up';
  renderDown(box, c);
}

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
  n.dataset.pbTip = `Sode (${keyLabel(HOME_CHORD)})`;
  n.setAttribute('aria-keyshortcuts', ariaKeys(HOME_CHORD));
  return n;
}

// The Palette, as Ctrl+Shift+P (palette.js).
const PALETTE_CHORD = 'ctrl+shift+p';
function paletteButton() {
  const n = el('button', 'pb-btn pb-btn--ghost pb-btn--icon sode-bar__palette', icon('terminal', false));
  n.type = 'button';
  n.setAttribute('aria-label', 'Palette');
  n.setAttribute('aria-keyshortcuts', ariaKeys(PALETTE_CHORD));
  n.dataset.pbTip = `Palette (${keyLabel(PALETTE_CHORD)})`;
  n.addEventListener('click', () => emit('paletteButton'));
  return n;
}

// The bar's buttons in display order: first, Panels (with panels), Palette, settings. first is Home's Grid button, else
// the Home link. settings is Home's own button, else a link (settings.js opens the view at #settings). leave is the
// page's unsaved-edits rule (paneleditor.js); it guards only the links made here.
export function barLinks({ panels = false, leave = null, first = null, settings = null } = {}) {
  const link = (a) => {
    if (leave) guard(a, leave);
    return a;
  };
  return [
    first ?? link(homeLink()),
    panels && link(panelsLink()),
    paletteButton(),
    settings ?? link(iconLink('sode-bar__settings', 'settings', 'Settings', './#settings')),
  ].filter(Boolean);
}

// Sets a bar's title: the site name, a space, then the page name (Home has none). The name can be cut with an
// ellipsis, so the tip gives it in full.
export function barTitle(title, site, page = '') {
  const name = el('span', 'sode-bar__name', el('span', 'sode-bar__site', site));
  if (page) name.append(' ', el('span', 'sode-bar__page', page));
  title.replaceChildren(name);
  title.dataset.pbTip = page ? `${site} ${page}` : site;
}

// The least room for a centred title, in rem; with less the title goes back to the bar's left.
const CENTRE_MIN = 10;

// A centred title (bar.css) gets the room that the wider side leaves on both sides of the bar's middle. Thus it
// stays in the middle and never runs under a button. Sums of widths, not places, so the title's own place changes nothing.
function centreTitle(head, title) {
  const fit = () => {
    const cs = getComputedStyle(head);
    const gap = parseFloat(cs.columnGap) || 0;
    const chip = document.documentElement.hasAttribute('data-sode-chip');
    let side = parseFloat(cs.paddingLeft);
    let left = 0;
    for (const n of head.children) {
      if (n === title) continue;
      if (n.matches('.sode-panels__spacer, .sode-log__spacer')) {
        left = side;
        side = 0;
        continue;
      }
      // Without the Lock bar the start rule hides while the title is centred (bar.css).
      if (n.matches('.sode-bar__rule--start') && !chip) continue;
      const w = n.getBoundingClientRect().width;
      if (w > 0) side += w + gap;
    }
    const right = side + parseFloat(cs.paddingRight);
    const room = head.getBoundingClientRect().width - 2 * Math.max(left, right);
    const rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
    head.style.setProperty('--sode-title-room', `${Math.max(0, Math.floor(room))}px`);
    head.toggleAttribute('data-sode-centre', room >= CENTRE_MIN * rem);
  };
  const sizes = new ResizeObserver(fit);
  sizes.observe(head);
  for (const n of head.children) sizes.observe(n);
  // The Live Editor and Inbox add parts after the bar is made.
  new MutationObserver((recs) => {
    for (const r of recs) for (const n of r.addedNodes) if (n.nodeType === 1) sizes.observe(n);
    fit();
  }).observe(head, { childList: true });
}

function rule(cls) {
  const n = el('span', `sode-bar__rule ${cls}`);
  n.setAttribute('aria-hidden', 'true');
  return n;
}

// Makes head (a header with its .sode-panels__title) the bar: the links go after the title and the status goes last.
// opts.conn is a status the head already holds (Home's #conn), which its page paints; without it the bar makes one.
export function panelsBar(head, opts = {}) {
  const title = head.querySelector('.sode-panels__title');
  // The ellipsis goes on an inner span, since the title's own overflow would cut its tip. data-page keeps the page
  // name, so each brand event builds the title again from its two parts.
  barTitle(title, 'Sode', title.dataset.page);
  const tools = el('div', 'sode-bar__tools', ...barLinks(opts));
  title.after(rule('sode-bar__rule--start'), tools);
  head.classList.add('sode-bar');
  // Home keeps its title at the left.
  if (!head.classList.contains('sode-home-bar')) centreTitle(head, title);
  let conn = opts.conn;
  if (!conn) {
    const box = connBox();
    conns.add(box);
    paintConn(box);
    head.append(box.root);
    conn = box.root;
    banner = true;
    paintDown();
  }
  conn.before(rule('sode-bar__rule--end'));
  return tools;
}
