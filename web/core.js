// Imports only tooltip.js, sprite.js, fuzzy.js, buildrules.js, routerprefix.js and routerlinkrules.js, so no import
// cycle occurs.

import './tooltip.js';
import { iconHref, adoptIcons } from './sprite.js';
import { fuzzyScore } from './fuzzy.js';
import { searchText, prefixOf, typedPrefix, labelPrefix } from './routerprefix.js';
import { buildChanged } from './buildrules.js';
import { routerLinks, wholeState, lost } from './routerlinkrules.js';

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
  installUid: '', // from the hello; Item links name it
  noAdministrator: false, // users and no Administrator: the page shows the recovery banner
  notSaved: false,
  setup: null, // setupwizard.js
  // The first router's Protects. Each router after the first keeps its own in its routerData (protectsOf).
  prot: new Map(), // dst → Map(level → {dev, kind}), from the router
  protBy: new Map(), // dst → {by, device, levels}: the Protects in Sode's own record
  levelDefaults: [],
  wsState: 'connecting',
  wsLost: false,
  wsErr: '',
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
  // Every router, the first one first: [{uid, prefix, title, address, matrix}]. Read one router's names, tally, levels
  // and link with routerData.
  routers: [],
};

// The state of each router after the first, by the UID of the hello's routers list.
const more = new Map();
// otherRouter gives the state of a router after the first, or null for the first router. A UID that the hello did not
// list is the first router, as a blank one is.
const otherRouter = (uid) => (uid && more.get(uid)) || null;

// The first router's state. A hello fills it in place, so a reference to it stays good. Its number offset is the
// settings' (off), so tests that set S.cfg still number its items.
const FIRST = Object.defineProperty(newRouter({}, true), 'off', { get: () => off(), enumerable: true });

// routerData is the one accessor for the names, tally, Via rows, levels (with numbering and default levels) and link
// of one router. Blank is the first router. Tests set the fields of routerData('') to give the first router names.
export const routerData = (uid) => otherRouter(uid) || FIRST;

// The UID that a take sends for an item's router: blank for the first router.
export const routerOfItem = (it) => (otherRouter(it?.router) ? it.router : '');

// Each router's Via rows, the first router first, as `[[uid, via]]` (viarules.js fedThrough). uid is the router's own
// UID.
export const viaByRouter = () => [FIRST, ...more.values()].map((r) => [r.uid, r.via]);

// sameEnd: the same number on the same router.
export const sameEnd = (a, b) => !!a && !!b && a.n === b.n && routerOfItem(a) === routerOfItem(b);

// The prefixes of every router, the first one first.
const prefixes = () => (S.routers.length ? S.routers.map((r) => r.prefix) : [FIRST.prefix]);
const many = () => S.routers.length > 1;

// routerNamed is the router whose prefix the text starts with (A.23, A23, BA.23), as a routerData UID. It is blank for
// the first router, and null when the text names no router (typedPrefix).
export function routerNamed(q) {
  const p = typedPrefix(q, prefixes());
  if (!p) return null;
  if (p === FIRST.prefix) return '';
  return [...more.values()].find((r) => r.prefix === p)?.uid ?? null;
}
// The router whose prefix the text starts with, as a routerData UID: blank for none or the first router.
export const routerOfText = (q) => routerNamed(q) || '';

function newRouter(r, first = false) {
  return {
    first,
    uid: r.uid || '',
    prefix: r.prefix || '',
    title: r.title || '',
    off: r.numberOffset ?? 0,
    levels: r.levels || [],
    levelBy: new Map((r.levels || []).map((l) => [l.ui, l])),
    numbering: r.levelNumbering || '1',
    defaultLevels: r.defaultLevels || '',
    srcs: [],
    dsts: [],
    srcBy: new Map(),
    dstBy: new Map(),
    tally: new Map(),
    via: new Map(),
    namesAt: null,
    namesSavedAt: null, // the hello's savedAt: names from the saved names table, until a read.
    // A router after the first keeps its Protects here (protectsOf).
    prot: new Map(),
    protBy: new Map(), // Sode's record of them.
    protectDevice: r.protectDevice || 0, // Sode's device number on the router.
    protectAt: r.protectAt || null, // while the link is not up: when it went down, the age of the Protect rows.
    link: null,
    addr: '',
    err: '',
  };
}

// The hello's routers list: the first router goes to FIRST, each other one to more. S.routers comes first: with two
// or more routers, the items of the first router show its prefix too.
function setRouters(list) {
  S.routers = list.map((r) => ({ uid: r.uid, prefix: r.prefix || '', title: r.title || '', address: r.address || '', matrix: r.matrix || 0 }));
  more.clear();
  const { link, addr, err } = FIRST;
  const fresh = newRouter(list[0] || {}, true);
  delete fresh.off; // the offset of the first router comes from the settings (off)
  // The link fields stay. So the hello's status keeps the last error of an unchanged address, as a status message does.
  Object.assign(FIRST, fresh, { link, addr, err });
  fillRouter(FIRST, list[0] || {});
  for (const r of list.slice(1)) {
    const st = newRouter(r);
    more.set(st.uid, st);
    fillRouter(st, r);
  }
}

function fillRouter(st, r) {
  setNames(r.srcs || [], r.dsts || [], st);
  st.namesAt = r.at ?? null;
  st.namesSavedAt = r.savedAt || null;
  applyTally(r.tally || [], st.tally);
  st.via = viaMap(r.via);
  setRouterStatus(st, r.status || {});
  if (!st.first) {
    applyProtect(r.protected || [], st.prot);
    st.protBy = byDst(r.protectBy);
  }
}

// protectsOf gives the Protects of one router (a routerData UID; blank is the first). prot is dst → Map(level →
// {dev, kind}). by is Sode's record, dst → {by, device, levels}. device is Sode's device number on the router.
export function protectsOf(uid = '') {
  const rt = otherRouter(uid);
  if (!rt) return { prot: S.prot, by: S.protBy, device: S.cfg?.protectDevice || 0 };
  return { prot: rt.prot, by: rt.protBy, device: rt.protectDevice };
}

// err is the last error, kept while the link connects again. A new address or an up link clears it.
function setRouterStatus(st, m) {
  if ((m.addr || '') !== st.addr) st.err = '';
  st.link = m.router || null;
  st.addr = m.addr || '';
  if (m.err) st.err = m.err;
  else if (st.link === 'up') st.err = '';
}

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

// An item of a router after the first has router (its UID) and prefix, and its label shows the prefix (B.23). num stays
// the number alone, so a search of that router's list matches it.
// With two or more routers, an item of the first router has prefix too (A.23), and no router.
export function item(n, name, uid = '') {
  const rt = otherRouter(uid);
  const num = String(n + (rt ? rt.off : off()));
  const prefix = rt ? rt.prefix : many() ? S.routers[0].prefix : '';
  const shown = prefix ? `${prefix}.${num}` : num;
  const it = { n, name, num, lname: name.toLowerCase(), label: name ? `${shown} ${name}` : shown };
  if (rt) it.router = rt.uid;
  if (prefix) it.prefix = prefix;
  return it;
}
export const srcItem = (n, name = '', uid = '') => routerData(uid).srcBy.get(n) || item(n, name, uid);
export const dstItem = (n, name = '', uid = '') => routerData(uid).dstBy.get(n) || item(n, name, uid);
export const srcLabel = (n, uid = '') => srcItem(n, '', uid).label;
// The number as shown: with its router prefix when there are two or more routers (B.23).
export const numText = (it) => (it.prefix ? `${it.prefix}.${it.num}` : it.num);
// Number is muted and fixed-width so it cannot run into a name starting with a digit.
export const labelNode = (it) => el('span', 'sode__lbl', el('span', 'sode__lbl-n', numText(it)), ...(it.name ? [' ', it.name] : []));
export const dstLabel = (n, uid = '') => dstItem(n, '', uid).label;
// endSrcLabel: the label of a source end { router, n } of any router, as a Via row's origin names it.
export const endSrcLabel = (e) => srcLabel(e.n, routerOfItem(e));
export const tallyText = (s, uid = '') => (s == null ? 'unknown' : s < 0 ? 'nothing' : srcLabel(s, uid));

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
      if (FIRST.prefix && m.routers?.[0]?.prefix && FIRST.prefix !== m.routers[0].prefix) {
        globalThis.location?.reload();
        return;
      }
      S.prevNumbering = S.cfg && { mode: FIRST.numbering, max: FIRST.levels.length };
      S.cfg = m.cfg;
      S.versions = m.versions || {};
      // Before helloCfg: brand.js makes the accent from a named color's hex.
      S.brandColors = m.brandColors || [];
      emit('helloCfg', m.cfg);
      S.configPath = m.configPath;
      S.version = m.version || '';
      S.installUid = m.installUid || '';
      S.users = m.users || [];
      S.userNames = S.users.map((u) => u.name);
      S.noAdministrator = !!m.noAdministrator;
      S.notSaved = !!m.notSaved;
      S.setup = m.setup || null;
      S.levelDefaults = m.levelDefaults || [];
      setRouters(m.routers || []);
      S.prot = new Map();
      applyProtect(m.protected || []);
      applyProtectBy(m.protectBy || []);
      S.hist = m.history || [];
      S.histMore = m.more;
      // A page asked for before this snapshot is for a replaced list and is ignored (takePage):
      // do not wait for it.
      S.histLoading = false;
      emit('helloData');
      emit('hello');
      emit('conn');
      break;
    // tally, names and status name their router; one with no routerUid is from a server with one router.
    case 'names': {
      const rt = routerData(m.routerUid);
      setNames(m.srcs, m.dsts, rt);
      rt.namesAt = m.at;
      emit('names');
      break;
    }
    case 'namesChecked': {
      routerData(m.routerUid).namesAt = m.at;
      emit('namesChecked');
      break;
    }
    case 'tally': {
      const rt = otherRouter(m.routerUid);
      applyTally(m.u, (rt || FIRST).tally);
      // tallyRows is for the first router: panel pages show only it.
      if (!rt) emit('tallyRows', m.u);
      emit('tally');
      break;
    }
    case 'viaRows': {
      // The whole set of one router (internal/app/pagevia.go).
      routerData(m.routerUid).via = viaMap(m.rows);
      emit('viaRows');
      emit('tally');
      break;
    }
    case 'protected': {
      // A router after the first keeps its own. An open Tie line picker plans again (protectedOther).
      const rt = otherRouter(m.routerUid);
      if (rt) {
        applyProtect(m.u, rt.prot);
        if (m.by) rt.protBy = byDst(m.by);
        emit('protectedOther');
        emit('protectedRows');
        emit('tally');
        break;
      }
      applyProtect(m.u);
      if (m.by) applyProtectBy(m.by);
      emit('protectedRows');
      emit('tally');
      break;
    }
    case 'status': {
      setRouterStatus(routerData(m.routerUid), m);
      emit('conn');
      break;
    }
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

// to: the state of one router (routerData).
function setNames(srcs, dsts, to) {
  const uid = to.first ? '' : to.uid;
  to.srcs = srcs.map(([n, name]) => item(n, name, uid));
  to.dsts = dsts.map(([n, name]) => item(n, name, uid));
  to.srcBy = new Map(to.srcs.map((i) => [i.n, i]));
  to.dstBy = new Map(to.dsts.map((i) => [i.n, i]));
}

// The Via rows of one router by dst and level.
function viaMap(rows = []) {
  const out = new Map();
  for (const r of rows || []) {
    if (!out.has(r.dst)) out.set(r.dst, new Map());
    out.get(r.dst).set(r.level, r);
  }
  return out;
}

function applyTally(list, tally) {
  for (const [d, l, s] of list) {
    let t = tally.get(d);
    if (!t) tally.set(d, (t = new Map()));
    t.set(l, s);
  }
}

// [dst, level, device, kind]; device -1 clears. A row with no kind is a Protect. prot is the first router's when none.
function applyProtect(list, prot = S.prot) {
  for (const [d, l, dev, kind = 1] of list) {
    let t = prot.get(d);
    if (dev < 0) {
      if (t) t.delete(l);
      if (t && !t.size) prot.delete(d);
      continue;
    }
    if (!t) prot.set(d, (t = new Map()));
    t.set(l, { dev, kind });
  }
}

const byDst = (list) => new Map((list || []).map((r) => [r.dst, r]));

function applyProtectBy(list) {
  S.protBy = byDst(list);
}

export function connState() {
  const ws = { up: ['ok', 'Server connected'], connecting: ['warn', 'Server connecting'], down: ['error', 'Server disconnected'] }[S.wsState];
  const list = linkList();
  // rt: the first router's link.
  const { state: dot, word } = list[0];
  const whole = S.wsState === 'up' ? wholeState(list) : '';
  let down = null;
  if (S.wsLost) {
    down = { title: 'Lost link: Server link down', err: S.wsErr, links: [false, null] };
  } else if (list.length === 1 && S.wsState === 'up' && FIRST.link === 'down' && !FIRST.addr) {
    down = { title: 'No router set', text: 'Set the router address in Settings.' };
  } else if (list.length === 1 && whole === 'down') {
    down = { title: 'Lost link: Router link down', err: FIRST.err, links: [true, false] };
  } else if (whole === 'down') {
    // Sode is Down: one hop for each router.
    const err = list.map((r) => (r.err ? `Router ${r.prefix}: ${r.err}` : '')).filter(Boolean);
    down = { title: 'Lost link: every Router link down', err: err.join('\n'), links: [true, false], routers: list };
  }
  // ok: every link up, the only state that Compact view is allowed to hide the status for. degraded: the lost routers
  // while Sode is Degraded, with no banner.
  return {
    ws,
    rt: [dot, `Router ${word}`],
    down,
    degraded: !down && whole === 'degraded' ? list.filter(lost) : [],
    ok: ws[0] === 'ok' && !down && list.every((r) => r.state === 'ok'),
  };
}

// The Router link of every router, in config order (routerlinkrules.js).
export const linkList = () => routerLinks(S, routerData);

export function routerTitle() {
  const show = S.cfg?.routerShow || 'address';
  if (show === 'none') return ['', false];
  if (show === 'name' && S.cfg.routerName) return [S.cfg.routerName, true];
  return [FIRST.addr, false];
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
  const key = c.down ? JSON.stringify([c.down, FIRST.addr, S.cfg?.routerName]) : '';
  if (box.dataset.key === key) return;
  box.dataset.key = key;
  if (!c.down) return box.replaceChildren();
  const { title, text, err, links, routers } = c.down;
  // links[k] is the wire into node k + 1: true ok, false lost, null unknown (past a lost link).
  const wire = (ok) => {
    const [tone, word] = ok === true ? ['ok', 'ok'] : ok === false ? ['error', 'lost'] : ['idle', 'unknown'];
    return el('span', `sode__down-link sode__down-link--${tone}`, el('span', 'sode__down-word', icon(STATE_GLYPH[tone], false), word));
  };
  const node = (glyph, ok, ...label) =>
    el('span', 'sode__down-node' + (ok === false ? ' sode__down-node--down' : ok === null ? ' sode__down-node--unknown' : ''), icon(glyph, false), ...label);
  // With more than one router, the last wire leads to a node for each router.
  const routerNodes = routers
    ? routers.map((r) => node(routerIcon(), links[1], `Router ${r.prefix} `, r.title ? `${r.title} ` : '', el('code', null, r.addr || '?')))
    : [node(routerIcon(), links?.[1], 'Router ', S.cfg?.routerName ? `${S.cfg.routerName} ` : '', el('code', null, FIRST.addr || '?'))];
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
          el('span', 'sode__down-hop', wire(links[1]), ...routerNodes),
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

// linked: the Server link and one or more Router links are up. While Sode is Degraded a take can still go to a router
// that is up; planBlocked refuses a route to a router with no link. firstLinked is for the Protects, which the page
// keeps on the first router only.
export const linked = () => !!S.ws && S.wsState === 'up' && linkList().some((r) => r.link === 'up');
export const firstLinked = () => !!S.ws && S.wsState === 'up' && FIRST.link === 'up';

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

// [score, item, rank] for each item of list that matches q (trimmed, lower case).
function scored(list, q, rank = 0, bonus = 0) {
  const out = [];
  for (const it of list) {
    const s = fuzzyScore(it, q);
    if (s >= 0) out.push([s + bonus, it, rank]);
  }
  return out;
}
function best(out, n) {
  out.sort((a, b) => b[0] - a[0] || a[2] - b[2] || a[1].n - b[1].n);
  const top = out.slice(0, n).map((x) => x[1]);
  top.total = out.length; // how many matched before the cut
  return top;
}

// A.23 searches 23.
export function search(list, q, n = 8) {
  q = searchText(q, FIRST.prefix, [], [FIRST.prefix]);
  q = q?.trim().toLowerCase();
  if (!q) return [];
  return best(scored(list, q), n);
}

// boxSearch gives the matches of a patch box (kind 'src' or 'dst') across the routers. A router prefix with digits
// (B.42, B42) searches that router's numbers, and those matches always come first. Without the dot, the name matches
// of the whole text come after them, so "B42 MON" still shows. A bare number searches def, the default router of the
// box (a routerData UID). Other text searches the names of every router; on a tie, def comes first. keep drops items.
// An edited label of a pick ("A.23 cam", from "A.23 Cam 1") searches its router for the number and the rest ("23 cam").
export function boxSearch(kind, text, { def = '', n = 8, keep = () => true } = {}) {
  const q = text.trim().toLowerCase();
  if (!q) return [];
  const listOf = (rt) => (kind === 'src' ? rt.srcs : rt.dsts).filter(keep);
  const lp = labelPrefix(text, prefixes());
  const p = lp ? { ...lp, text: lp.text.toLowerCase() } : prefixOf(text);
  if (lp || typedPrefix(text, prefixes())) {
    const uid = routerNamed(lp ? `${lp.prefix}.${lp.num}` : text);
    if (uid == null) return Object.assign([], { total: 0 });
    const first = scored(listOf(routerData(uid)), p.text, 0, 10000);
    if (p.dot) return best(first, n);
    const seen = new Set(first.map((x) => x[1]));
    return best([...first, ...everyRouter(listOf, q, def).filter((x) => !seen.has(x[1]))], n);
  }
  if (/^\d+$/.test(q) || !many()) return search(listOf(routerData(def)), text, n);
  return best(everyRouter(listOf, q, def), n);
}
// The matches of q on every router; def ranks first on a tie.
function everyRouter(listOf, q, def) {
  const home = routerData(def);
  return [FIRST, ...more.values()].flatMap((rt) => scored(listOf(rt), q, rt === home ? 0 : 1));
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
