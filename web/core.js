// Imports only tooltip.js, sprite.js, fuzzy.js, buildrules.js and routerprefix.js, so no import cycle occurs.

import './tooltip.js';
import { iconHref, adoptIcons } from './sprite.js';
import { fuzzyScore } from './fuzzy.js';
import { searchText } from './routerprefix.js';
import { buildChanged } from './buildrules.js';

// A send or a pending answer with no Server link.
export const LINK_DOWN = 'Server link down.';

export const $ = (id) => document.getElementById(id);

// A line that hides when empty; an unchanged text is not written again (a redraw each second must not restart a
// screen reader).
export function showNote(n, text) {
  if (n.textContent !== text) n.textContent = text;
  n.hidden = !text;
}

// props is a class string or an object; a key the element has as a property is set as one, any other as an attribute.
export function el(tag, props, ...kids) {
  const e = document.createElement(tag);
  if (typeof props === 'string') e.className = props;
  else if (props)
    for (const [k, v] of Object.entries(props)) {
      if (v == null) continue;
      if (k === 'data-pb-tip' && !String(v).trim()) continue; // a blank tip still draws an empty bubble
      if (k === 'class') e.className = v;
      else if (k in e) e[k] = v;
      else if (v !== false) e.setAttribute(k, v === true ? '' : v);
    }
  for (const k of kids) if (k != null) e.append(k);
  return e;
}

export const S = {
  cfg: null,
  configPath: '',
  version: '',
  brandColors: [],
  routerUid: '',
  installUid: '', // from the hello; Item links name it
  prefix: '', // the router prefix, from the hello
  noAdministrator: false, // users and no Administrator: the page shows the recovery banner
  notSaved: false,
  setup: null, // setupwizard.js
  srcs: [],
  dsts: [],
  srcBy: new Map(),
  dstBy: new Map(),
  tally: new Map(),
  prot: new Map(), // dst → Map(level → {dev, kind}), from the router
  protBy: new Map(), // dst → {by, device, levels}: the Protects in Sode's own record
  levels: [],
  levelDefaults: [],
  levelBy: new Map(),
  namesAt: null,
  namesSavedAt: null, // the hello's savedAt: names from the saved names table, until a read
  wsState: 'connecting',
  wsLost: false,
  wsErr: '',
  router: null,
  routerAddr: '',
  routerErr: '',
  routerLastErr: '',
  ws: null,
  hist: [],
  histMore: false,
  histLoading: false,
  result: null,
  newId: null,
  prevNumbering: null,
  myRights: null,
  users: [], // the hello's sign-in list, [{name, len}]; len 0 is no PIN
  userNames: [],
};

export function toggle(set, k) {
  if (set.has(k)) set.delete(k);
  else set.add(k);
}

const listeners = {};
// Returns a function that removes fn, for a module that can stop (a panel run).
export const on = (ev, fn) => {
  (listeners[ev] ||= []).push(fn);
  return () => {
    const l = listeners[ev];
    const i = l.indexOf(fn);
    if (i >= 0) l.splice(i, 1);
  };
};
// The removal waits a microtask: one inside emit would skip the next listener.
export const once = (ev, fn) => {
  const off = on(ev, (...a) => {
    queueMicrotask(off);
    fn(...a);
  });
};
// Runs before earlier listeners: for a module that must refresh its memo before views read it.
export const onFirst = (ev, fn) => {
  (listeners[ev] ||= []).unshift(fn);
};
export const emit = (ev, ...args) => {
  for (const fn of listeners[ev] || []) fn(...args);
};

// A Map, so a message type such as "constructor" never reaches an Object.prototype method.
const handlers = new Map();
export const onMsg = (t, fn) => {
  if (handlers.has(t)) throw new Error(`onMsg('${t}') is already handled`);
  handlers.set(t, fn);
};

export const off = () => (S.cfg ? S.cfg.numberOffset : 0);

export function item(n, name) {
  const num = String(n + off());
  return { n, name, num, lname: name.toLowerCase(), label: name ? `${num} ${name}` : num };
}
export const srcItem = (n, name = '') => S.srcBy.get(n) || item(n, name);
export const dstItem = (n, name = '') => S.dstBy.get(n) || item(n, name);
export const srcLabel = (n) => srcItem(n).label;
// Number is muted and fixed-width so it cannot run into a name starting with a digit.
export const labelNode = (it) => el('span', 'sode__lbl', el('span', 'sode__lbl-n', it.num), ...(it.name ? [' ', it.name] : []));
export const dstLabel = (n) => dstItem(n).label;
export const tallyText = (s) => (s == null ? 'unknown' : s < 0 ? 'nothing' : srcLabel(s));

let retry = 1000;

// Browsers cannot see ping frames; the server sends {"t":"ping"} about every 5s. A socket quiet for
// STALE_MS is dead even if OPEN; nothing is sent into one quiet for FRESH_MS.
// ws.lastRx is when anything last arrived.
const STALE_MS = 15000;
const FRESH_MS = 7000;
const quiet = (ws) => Date.now() - ws.lastRx;

// A dead link's close event can be long in coming, so report the loss now, one time.
function drop(ws) {
  const lost = ws.onclose;
  ws.onclose = ws.onmessage = null;
  try {
    ws.close();
  } catch {
    /* already closed */
  }
  lost?.({ stale: true });
}

export function connect() {
  const url = new URL('ws', document.baseURI);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(url);
  S.ws = ws;
  S.wsState = 'connecting';
  emit('conn');
  const watch = setInterval(() => {
    if (ws.readyState === WebSocket.OPEN && quiet(ws) > STALE_MS) drop(ws);
  }, 1000);
  ws.onopen = () => {
    ws.lastRx = Date.now();
    retry = 1000;
    S.wsState = 'up';
    S.wsLost = false;
    emit('conn');
  };
  ws.onclose = (e) => {
    clearInterval(watch);
    // The why for the Lost link banner. A browser gives no more than the close code: 1006 is a drop with no close frame.
    S.wsErr = `${location.host}: ` + (e?.stale ? `no data for ${STALE_MS / 1000} s` : `connection closed (code ${e?.code}${e?.reason ? `, ${e.reason}` : ''})`);
    S.ws = null;
    S.wsState = 'down';
    S.wsLost = true;
    S.histLoading = false;
    emit('wsclose');
    emit('conn');
    setTimeout(connect, retry);
    retry = Math.min(retry * 2, 10000);
  };
  ws.onmessage = (e) => {
    ws.lastRx = Date.now();
    handle(JSON.parse(e.data));
  };
}

export function wsSend(m) {
  const ws = S.ws;
  if (!ws || ws.readyState !== WebSocket.OPEN) return false;
  if (quiet(ws) > FRESH_MS) {
    drop(ws);
    return false;
  }
  ws.send(JSON.stringify(m));
  return true;
}

// One message and its answer of type resultT, matched by id. msg keeps its literal t at the call site: the policy
// test reads the types from the source. With no link, or when the socket closes first, it answers { ok: false, err }.
const waiting = new Map();
const asked = new Set();
let askSeq = 0;
export function request(resultT, msg) {
  if (!asked.has(resultT)) {
    asked.add(resultT);
    onMsg(resultT, (m) => {
      const done = waiting.get(m.id);
      waiting.delete(m.id);
      done?.(m);
    });
  }
  const id = ++askSeq;
  return new Promise((resolve) => {
    if (!wsSend({ ...msg, id })) return resolve({ ok: false, err: LINK_DOWN });
    waiting.set(id, resolve);
  });
}
on('wsclose', () => {
  for (const done of waiting.values()) done({ ok: false, err: LINK_DOWN });
  waiting.clear();
});

function handle(m) {
  switch (m.t) {
    case 'hello':
      // This page's code is from the old build, and the new server can send messages it does not know.
      if (buildChanged(S.version, m.version)) {
        globalThis.location?.reload();
        return;
      }
      // The prefix changed while this socket was down, so the reload message never came.
      if (S.prefix && m.prefix && S.prefix !== m.prefix) {
        globalThis.location?.reload();
        return;
      }
      S.prevNumbering = S.cfg && { mode: S.cfg.levelNumbering, max: S.levels.length };
      S.cfg = m.cfg;
      S.versions = m.versions || {};
      emit('helloCfg', m.cfg);
      S.configPath = m.configPath;
      S.version = m.version || '';
      S.brandColors = m.brandColors || [];
      S.routerUid = m.routerUid || '';
      S.installUid = m.installUid || '';
      S.prefix = m.prefix || '';
      S.users = m.users || [];
      S.userNames = S.users.map((u) => u.name);
      S.noAdministrator = !!m.noAdministrator;
      S.notSaved = !!m.notSaved;
      S.setup = m.setup || null;
      S.levels = m.levels;
      S.levelBy = new Map(m.levels.map((l) => [l.ui, l]));
      S.levelDefaults = m.levelDefaults || [];
      setNames(m.srcs, m.dsts);
      S.namesAt = m.at;
      S.namesSavedAt = m.savedAt || null;
      S.tally = new Map();
      applyTally(m.tally || []);
      S.prot = new Map();
      applyProtect(m.protected || []);
      applyProtectBy(m.protectBy || []);
      setStatus(m.status);
      S.hist = m.history || [];
      S.histMore = m.more;
      // A page asked for before this snapshot is for a replaced list and is ignored (takePage):
      // do not wait for it.
      S.histLoading = false;
      emit('helloData');
      emit('hello');
      emit('conn');
      break;
    case 'names':
      setNames(m.srcs, m.dsts);
      S.namesAt = m.at;
      emit('names');
      break;
    case 'namesChecked':
      S.namesAt = m.at;
      emit('namesChecked');
      break;
    case 'tally':
      applyTally(m.u);
      emit('tallyRows', m.u);
      emit('tally');
      break;
    case 'protected':
      applyProtect(m.u);
      if (m.by) applyProtectBy(m.by);
      emit('protectedRows');
      emit('tally');
      break;
    case 'status':
      setStatus(m);
      emit('conn');
      break;
    case 'ping':
      break;
    case 'reload':
      globalThis.location?.reload();
      break;
    case 'logPage':
    case 'uptimeResult':
    case 'logAdd':
      emit(m.t, m);
      break;
    default: {
      const fn = handlers.get(m.t);
      if (typeof fn === 'function') fn(m);
    }
  }
}

function setNames(srcs, dsts) {
  S.srcs = srcs.map(([n, name]) => item(n, name));
  S.dsts = dsts.map(([n, name]) => item(n, name));
  S.srcBy = new Map(S.srcs.map((i) => [i.n, i]));
  S.dstBy = new Map(S.dsts.map((i) => [i.n, i]));
}

function applyTally(list) {
  for (const [d, l, s] of list) {
    let t = S.tally.get(d);
    if (!t) S.tally.set(d, (t = new Map()));
    t.set(l, s);
  }
}

// [dst, level, device, kind]; device -1 clears. A row with no kind is a Protect.
function applyProtect(list) {
  for (const [d, l, dev, kind = 1] of list) {
    let t = S.prot.get(d);
    if (dev < 0) {
      if (t) t.delete(l);
      if (t && !t.size) S.prot.delete(d);
      continue;
    }
    if (!t) S.prot.set(d, (t = new Map()));
    t.set(l, { dev, kind });
  }
}

function applyProtectBy(list) {
  S.protBy = new Map(list.map((r) => [r.dst, r]));
}

function setStatus(st) {
  if (st.addr !== S.routerAddr) S.routerLastErr = '';
  S.router = st.router;
  S.routerAddr = st.addr;
  S.routerErr = st.err || '';
  if (S.routerErr) S.routerLastErr = S.routerErr;
  else if (S.router === 'up') S.routerLastErr = '';
}

export function connState() {
  const ws = { up: ['ok', 'Server connected'], connecting: ['warn', 'Server connecting'], down: ['error', 'Server disconnected'] }[S.wsState];
  const map = { up: ['ok', 'connected'], connecting: ['warn', 'connecting'], down: ['error', 'down'] };
  const [dot, word] = S.wsState === 'up' && S.router ? map[S.router] || ['idle', S.router] : ['idle', 'unknown'];
  let down = null;
  if (S.wsLost) {
    down = { title: 'Lost link: Server link down', err: S.wsErr, links: [false, null] };
  } else if (S.wsState === 'up' && S.router === 'down' && !S.routerAddr) {
    down = { title: 'No router set', text: 'Set the router address in Settings.' };
  } else if (S.wsState === 'up' && (S.router === 'down' || (S.router === 'connecting' && S.routerLastErr))) {
    down = { title: 'Lost link: Router link down', err: S.routerLastErr, links: [true, false] };
  }
  // ok: both links up, the only state that Compact view is allowed to hide the status for.
  return { ws, rt: [dot, `Router ${word}`], down, ok: ws[0] === 'ok' && dot === 'ok' && !down };
}

export function routerTitle() {
  const show = S.cfg?.routerShow || 'address';
  if (show === 'none') return ['', false];
  if (show === 'name' && S.cfg.routerName) return [S.cfg.routerName, true];
  return [S.routerAddr || '', false];
}

// The video router's icon in the status and the Lost link banner. Lucide's `router` is a Wi-Fi router, which the
// SW-P-08 matrix is not. `server-nas` is copied from lucide-static 1.56.0 into the vendored sprite (owner's pick).
export const routerIcon = () => 'server-nas';

// The page starts below a fixed banner (the desktop one; the phone's has its own column):
// publish the fixed, showing banner's height as --sode-down-h.
const banners = new Set();
let downObs;
function publishDownHeight() {
  let h = 0;
  for (const b of banners) {
    if (!b.hidden && getComputedStyle(b).position === 'fixed') h = Math.max(h, b.offsetHeight);
  }
  document.documentElement.style.setProperty('--sode-down-h', `${h}px`);
}
// Redraw only when content changes: each redraw would re-announce the alert.
export function renderDown(box, c) {
  box.hidden = !c.down;
  if (typeof ResizeObserver !== 'undefined' && typeof document !== 'undefined' && !banners.has(box)) {
    downObs ??= new ResizeObserver(publishDownHeight);
    // The phone banner is fixed only while the Lock is up (mobile.css): re-measure when that flips.
    if (!banners.size) new MutationObserver(publishDownHeight).observe(document.documentElement, { attributes: true, attributeFilter: ['data-sode-lock'] });
    banners.add(box);
    downObs.observe(box);
  }
  if (downObs) publishDownHeight();
  const key = c.down ? JSON.stringify([c.down, S.routerAddr, S.cfg?.routerName]) : '';
  if (box.dataset.key === key) return;
  box.dataset.key = key;
  if (!c.down) return box.replaceChildren();
  const { title, text, err, links } = c.down;
  // links[k] is the wire into node k + 1: true ok, false lost, null unknown (past a lost link).
  const wire = (ok) => {
    const [tone, word] = ok === true ? ['ok', 'ok'] : ok === false ? ['error', 'lost'] : ['idle', 'unknown'];
    return el('span', `sode__down-link sode__down-link--${tone}`, el('span', 'sode__down-word', icon(STATE_GLYPH[tone], false), word));
  };
  const node = (glyph, ok, ...label) =>
    el('span', 'sode__down-node' + (ok === false ? ' sode__down-node--down' : ok === null ? ' sode__down-node--unknown' : ''), icon(glyph, false), ...label);
  box.replaceChildren(
    ...[
      el('div', 'sode__down-title', icon('unplug', false), title),
      links &&
        el(
          'div',
          'sode__down-map',
          node('monitor', true, 'This browser'),
          // A wire and the node it leads to wrap as one, so a line never starts with a loose wire.
          el('span', 'sode__down-hop', wire(links[0]), node('server', links[0], 'Sode server')),
          el(
            'span',
            'sode__down-hop',
            wire(links[1]),
            node(routerIcon(), links[1], 'Router ', S.cfg?.routerName ? `${S.cfg.routerName} ` : '', el('code', null, S.routerAddr || '?')),
          ),
        ),
      text && el('div', null, text),
      err && el('div', 'sode__down-why', el('span', null, 'Reason'), el('code', null, err)),
    ].filter(Boolean),
  );
}

export const sentence = (t) => {
  const s = String(t || '').trim();
  return s && s[0].toUpperCase() + s.slice(1) + (/[.!?…]$/.test(s) ? '' : '.');
};

// names: names already in use (any case). old: the name being edited, which is allowed to change case.
// reserved: a name that no item is allowed to have.
export function nameProblem(name, names, { noun = 'item', old = '', max = 32, reserved = '' } = {}) {
  const n = name.trim();
  if (!n) return 'Name required.';
  if ([...n].length > max) return `Names are at most ${max} characters.`;
  if (reserved && n.toLowerCase() === reserved.toLowerCase()) return `“${n}” is already in use.`;
  if (names.some((x) => x.toLowerCase() === n.toLowerCase() && x.toLowerCase() !== old.toLowerCase())) {
    return `There is already a ${noun} named “${n}”.`;
  }
  return '';
}

export const linked = () => !!S.ws && S.wsState === 'up' && S.router === 'up';

// needsLink buttons look disabled through a class and aria-disabled, not the disabled attribute, so a
// focused button keeps focus; a press still goes through planBlocked.
export function setLinkDisabled(b, off) {
  b.classList.toggle('disabled', off);
  b.setAttribute('aria-disabled', String(off));
}
export function needsLink(b) {
  b.dataset.needsLink = '';
  setLinkDisabled(b, !linked());
  return b;
}
on('conn', () => {
  for (const b of document.querySelectorAll('[data-needs-link]')) setLinkDisabled(b, !linked());
});

// The Server and Router status (.sode__state, bar.css), not patchbay's square. The link's icon (base) has the state's
// colour. A corner badge tells the state apart by its shape, also in greyscale. data-state is one of STATE_GLYPH's keys;
// the HTML leaves it out until the first paint. label is the spoken name with the state; count shows inside the icon,
// so it stays where the words hide. The icons are made only on a change: panel pages repaint often.
const STATE_GLYPH = { ok: 'circle-check', warn: 'loader-circle', error: 'circle-x', idle: 'circle-minus', info: 'circle-dot' };
export function setDot(dotEl, textEl, [state, text], { base, label = text, count = '' } = {}) {
  if (dotEl.dataset.base !== base) {
    dotEl.dataset.base = base;
    delete dotEl.dataset.state;
    dotEl.replaceChildren(icon(base, false), el('span', 'sode__state-badge'), el('span', 'sode__state-count'));
    dotEl.removeAttribute('aria-hidden');
    dotEl.setAttribute('role', 'img');
  }
  if (dotEl.dataset.state !== state) {
    dotEl.dataset.state = state;
    dotEl.children[1].replaceChildren(icon(STATE_GLYPH[state]));
  }
  if (dotEl.children[2].textContent !== count) dotEl.children[2].textContent = count;
  if (dotEl.getAttribute('aria-label') !== label) dotEl.setAttribute('aria-label', label);
  textEl.textContent = text;
}

// A.23 searches 23.
export function search(list, q, n = 8) {
  q = searchText(q, S.prefix, [], [S.prefix]);
  q = q?.trim().toLowerCase();
  if (!q) return [];
  const out = [];
  for (const it of list) {
    const s = fuzzyScore(it, q);
    if (s >= 0) out.push([s, it]);
  }
  out.sort((a, b) => b[0] - a[0] || a[1].n - b[1].n);
  const best = out.slice(0, n).map((x) => x[1]);
  best.total = out.length; // how many matched before the cut
  return best;
}
export function icon(name, sm = true) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('class', sm ? 'pb-icon pb-icon--sm' : 'pb-icon');
  svg.setAttribute('viewBox', '0 0 24 24');
  svg.setAttribute('aria-hidden', 'true');
  const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
  use.setAttribute('href', iconHref(name));
  svg.append(use);
  return svg;
}

export const chevron = (open) => icon(open ? 'chevron-down' : 'chevron-right');

// Enter never submits the page. (An inline onsubmit would break the CSP, see http.go.)
if (typeof document !== 'undefined') {
  adoptIcons();
  document.addEventListener('submit', (e) => {
    if (e.target.matches?.('.sode__boxes, .sode-log__filters')) e.preventDefault();
  });
}
