// Settings > Routers: the Address field tries the typed address before Save (ALPHA-BUGS AB-23). After the pause of
// the setup wizard (600 ms, setupwizard.js tryRouter) the page sends routerTry. The server reads the router's names on
// a connection of its own (internal/app/routertry.go), so the Router link does not move. The words are the wizard's
// (routerLine in setupwizardrules.js). A failed try shows why and tries again every RETRY_MS while the address stays,
// as "Sode keeps trying" says. The next key cancels the try that runs. The saved address of a router that Sode has is
// not tried: the detail head shows its link state. Only an Administrator can try (the server checks it again).

import { $, el, request } from './core.js';
import { statusText } from './statustext.js';
import { routerLine } from './setupwizardrules.js';
import { tryWhy } from './routerdetailrules.js';

const PAUSE_MS = 600;
const RETRY_MS = 5000;

let timer = 0;
let seq = 0;
let cur = { uid: '', key: '', v: null };

const box = () => $('cfg-rt-address-try');
// keyOf: the values that a try reads. A change of one of them starts a new try.
const keyOf = (v) => JSON.stringify([String(v.address || '').trim(), v.matrix, v.nameRequests, v.nameLength]);

// A failed try shows why in operator words, with the server's error text in the title. An error with no words shows as
// it is.
function show(line, err) {
  const b = box();
  if (!line) return void b.replaceChildren();
  const words = tryWhy(err);
  const why = !err ? [] : words ? [el('span', { class: 'sode__hint', title: err }, words)] : [el('span', 'sode__hint sode__code', err)];
  b.replaceChildren(statusText(line.ok ? 'ok' : 'warn', el('span', null, line.text)), ...why);
}

// tryRouterAddress follows the router in view: uid, its values v, and saved, the address that Sode connects to now (blank
// for a router to add). can: the person is an Administrator.
export function tryRouterAddress(uid, v, saved, can) {
  const addr = String(v.address || '').trim();
  const key = can && addr && addr !== saved ? keyOf(v) : '';
  if (uid === cur.uid && key === cur.key) return;
  clearTimeout(timer);
  const was = cur.key;
  cur = { uid, key, v: { ...v, address: addr } };
  const n = ++seq;
  if (!key) {
    show(null);
    // A blank address cancels the try that runs on the server.
    if (was) request('routerTryResult', { t: 'routerTry', try: { address: '' } });
    return;
  }
  show(routerLine(addr, '', 0, 0), '');
  timer = setTimeout(() => send(n), PAUSE_MS);
}

async function send(n) {
  // Settings closed or the router list redrew without this router: stop, and let the next paint start again.
  if (n !== seq) return;
  if (!box().offsetParent) {
    cur = { uid: '', key: '', v: null };
    return;
  }
  const v = cur.v;
  const m = await request('routerTryResult', {
    t: 'routerTry',
    try: { address: v.address, matrix: v.matrix, nameRequests: v.nameRequests, nameLength: v.nameLength },
  });
  if (n !== seq || m.state === 'cancelled') return;
  if (m.state === 'connected') return void show(routerLine(v.address, 'up', m.sources, m.destinations), '');
  if (!m.state) return void show(null);
  show(routerLine(v.address, '', 0, 0), m.err || '');
  timer = setTimeout(() => send(n), RETRY_MS);
}
