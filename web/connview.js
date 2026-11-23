// The Server and Router status with the router's address. The main page's top right (index.html #conn, desktop.js)
// and the Grid panel's header (gridpanelheadview.js) draw this one block.
import { el, S, connState, setDot, routerTitle, routerIcon } from './core.js';
import { statusInfo } from './statusinfo.js';
import { routerLinks, routerMarks, allUp } from './routerlinkrules.js';

// n is { root, srvDot, srvText, routers, addr }. Returns connState() with ok for every router, so a caller can act on a
// lost link.
export function paintConn(n) {
  const c = connState();
  const list = routerLinks(S);
  setDot(n.srvDot, n.srvText, [c.ws[0], 'Server'], { base: 'server', label: c.ws[1] });
  paintRouters(n.routers, routerMarks(list));
  // With more than one router each mark names its prefix and the details give the addresses.
  const [label, isName] = list.length === 1 ? routerTitle() : ['', false];
  n.addr.textContent = label;
  n.addr.classList.toggle('sode__conn-addr--name', isName);
  const ok = c.ok && allUp(list);
  n.root.classList.toggle('sode__conn--ok', ok);
  // The phone shows these words as the native title; elsewhere the details show them (statusinfo.js).
  statusInfo(n.root).paint(connTitle(c, list));
  return { ...c, ok };
}

const connTitle = (c, list) =>
  [c.ws[1], ...routerMarks(list, Infinity).map((m, i) => `${m.aria} ${list[i].addr}`.trim()), ...list.map((r) => r.err)].filter(Boolean).join('\n');

// One .pb-status per mark in box, kept in place between paints. titled: the phone's native title on each.
export function paintRouters(box, marks, titled = false) {
  while (box.children.length > marks.length) box.lastElementChild.remove();
  while (box.children.length < marks.length) box.append(status('Router').box);
  marks.forEach((m, i) => {
    const s = box.children[i];
    setDot(s.firstElementChild, s.lastElementChild, [m.state, m.label], { base: routerIcon(), label: m.aria, count: m.count });
    if (titled) s.title = m.aria;
  });
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
