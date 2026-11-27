// The Server and Router status with the router's address. The main page's top right (index.html #conn, desktop.js),
// the page bar (panelsbar.js) and the Grid panel's header (gridpanelheadview.js) draw this one block. The phone header
// (mobile.js) uses paintRouters.
import { el, connState, setDot, routerTitle, routerIcon, linkList } from './core.js';
import { statusInfo } from './statusinfo.js';
import { barMark } from './routerlinkrules.js';

// n is { root, srvDot, srvText, routers, addr }. Returns connState(), so a caller can act on a lost link.
export function paintConn(n) {
  const c = connState();
  const list = linkList();
  setDot(n.srvDot, n.srvText, [c.ws[0], 'Server'], { base: 'server', label: c.ws[1] });
  const open = paintRouters(n.routers, barMark(list));
  // Status label (statusLabel of the first router, one setting for the bar) picks the text beside the icon. It names
  // one router, so it shows only while there is one router.
  const [label, isName] = list.length === 1 ? routerTitle() : ['', false];
  n.addr.textContent = label;
  n.addr.classList.toggle('sode__conn-addr--name', isName);
  n.root.classList.toggle('sode__conn--ok', c.ok);
  // Degraded: Compact view shows only the Routers icon and its count (desktop.css).
  n.root.classList.toggle('sode__conn--degraded', c.degraded.length > 0);
  statusInfo(n.root, open);
  return c;
}

// The one Routers icon in box, as a button that opens the router list (statusinfo.js). Kept in place between paints.
// mark is barMark(linkList()). Returns the button.
export function paintRouters(box, mark) {
  let open = box.querySelector(':scope > .sode__conn-open');
  if (!open) {
    const s = status('Router');
    open = el('button', 'sode__conn-open', s.box);
    open.type = 'button';
    box.replaceChildren(open);
  }
  const s = open.firstElementChild;
  setDot(s.firstElementChild, s.lastElementChild, [mark.state, mark.word], { base: routerIcon(), label: mark.aria });
  open.classList.toggle('sode__conn-open--down', mark.down > 0);
  open.setAttribute('aria-label', `${mark.aria}. Router list`);
  return open;
}

function status(word) {
  const dot = el('span', 'sode__state');
  const text = el('span', null, word);
  return { box: el('span', 'pb-status', dot, text), dot, text };
}

// The block made in code, with index.html's markup and classes, for a page whose HTML has none.
export function connBox() {
  const srv = status('Server');
  const routers = el('span', 'sode__conn-routers', status('Router').box);
  const addr = el('span', 'sode__conn-addr');
  const root = el('div', 'sode__conn', srv.box, routers, addr);
  return { root, srvDot: srv.dot, srvText: srv.text, routers, addr };
}
