// The Server and Router status with the router's address. The main page's top right (index.html #conn, desktop.js)
// and the Grid panel's header (gridpanelheadview.js) draw this one block.
import { el, S, connState, setDot, routerTitle } from './core.js';

// n is { root, srvDot, srvText, rtDot, rtText, addr }. Returns connState(), so a caller can act on a lost link.
export function paintConn(n) {
  const c = connState();
  setDot(n.srvDot, n.srvText, [c.ws[0], 'Server']);
  setDot(n.rtDot, n.rtText, [c.rt[0], 'Router']);
  const [label, isName] = routerTitle();
  n.addr.textContent = label;
  n.addr.classList.toggle('sode__conn-addr--name', isName);
  n.root.classList.toggle('sode__conn--ok', c.ok);
  n.root.title = [c.ws[1], `${c.rt[1]} ${S.routerAddr}`, S.router !== 'up' && S.routerLastErr].filter(Boolean).join('\n');
  return c;
}

// The block made in code, with index.html's markup and classes, for a page whose HTML has none.
export function connBox() {
  const status = (word) => {
    const dot = el('span', 'pb-dot pb-dot--idle');
    dot.setAttribute('aria-hidden', 'true');
    const text = el('span', null, word);
    return { box: el('span', 'pb-status', dot, text), dot, text };
  };
  const srv = status('Server');
  const rt = status('Router');
  const addr = el('span', 'sode__conn-addr');
  const root = el('div', 'sode__conn', srv.box, rt.box, addr);
  return { root, srvDot: srv.dot, srvText: srv.text, rtDot: rt.dot, rtText: rt.text, addr };
}
