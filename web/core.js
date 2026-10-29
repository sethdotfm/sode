// Shared state, the connection to Sode, and helpers used by both layouts.
// Views subscribe with on(event, fn) and render from S.

import './tooltip.js';

export const $ = (id) => document.getElementById(id);

// el('div', 'cls', child, 'text', …) — builds DOM without innerHTML so
// router-supplied names can never inject markup.
export function el(tag, cls, ...kids) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  for (const k of kids) if (k != null) e.append(k);
  return e;
}

// ---- state & events

export const S = {
  cfg: null,
  configPath: '',
  version: '',
  srcs: [], dsts: [],
  srcBy: new Map(), dstBy: new Map(),
  tally: new Map(), // dst → Map(ui level → src)
  prot: new Map(),  // dst → Map(ui level → protecting device)
  levels: [], levelDefaults: [],
  levelBy: new Map(),
  namesAt: null,
  wsState: 'connecting',
  wsLost: false,    // the socket dropped and hasn't come back yet
  router: null, routerAddr: '', routerErr: '',
  routerLastErr: '', // newest error, kept through reconnect attempts until the router is up
  ws: null,
  hist: [],         // newest first
  histMore: false,
  histLoading: false,
  result: null,     // last {kind, title, text} banner
  newId: null,      // history entry that just arrived live
  prevNumbering: null, // { mode, max } of the levels before the last hello
};

// Adds k to the set, or removes it if it's there.
export function toggle(set, k) {
  if (set.has(k)) set.delete(k);
  else set.add(k);
}

// The search left in a box by the last + (a destination added with the
// search kept): `is()` is true while the box still holds exactly that, so
// it isn't a destination waiting to be picked. `read` gives the box's text.
export function keepSearch(read) {
  let kept = null;
  return {
    is: () => kept !== null && read() === kept,
    set: (keep, text) => { kept = keep && text.trim() ? text : null; },
  };
}

const listeners = {};
export const on = (ev, fn) => { (listeners[ev] ||= []).push(fn); };
// Like on, but runs before the listeners added so far and later: for a
// module that must refresh its own memo before any view reads it.
export const onFirst = (ev, fn) => { (listeners[ev] ||= []).unshift(fn); };
export const emit = (ev, ...args) => {
  for (const fn of listeners[ev] || []) fn(...args);
};

// Messages from the server that other modules handle: onMsg('recent', fn).
const handlers = {};
export const onMsg = (t, fn) => {
  if (handlers[t]) throw new Error(`onMsg('${t}') is already handled`);
  handlers[t] = fn;
};

// The number shown beside a source or destination is its index plus this.
export const off = () => (S.cfg ? S.cfg.numberOffset : 0);

export function item(n, name) {
  const num = String(n + off());
  return { n, name, num, lname: name.toLowerCase(), label: name ? `${num} ${name}` : num };
}
export const srcItem = (n, name = '') => S.srcBy.get(n) || item(n, name);
export const dstItem = (n, name = '') => S.dstBy.get(n) || item(n, name);
export const srcLabel = (n) => srcItem(n).label;
// A source or destination as rows and confirms show it: the number muted
// and fixed-width, so it can't run into a name that starts with a digit.
export const labelNode = (it) => el('span', 'sode__lbl', el('span', 'sode__lbl-n', it.num), ...(it.name ? [' ', it.name] : []));
export const dstLabel = (n) => dstItem(n).label;
// Tally source text: null = never reported, -1 = router says nothing routed.
export const tallyText = (s) => (s == null ? 'unknown' : s < 0 ? 'nothing' : srcLabel(s));

// ---- websocket

let retry = 1000;

export function connect() {
  const base = location.pathname.replace(/[^/]*$/, '');
  const ws = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + base + 'ws');
  S.ws = ws;
  S.wsState = 'connecting';
  emit('conn');
  ws.onopen = () => { retry = 1000; S.wsState = 'up'; S.wsLost = false; emit('conn'); };
  ws.onclose = () => {
    S.ws = null;
    S.wsState = 'down';
    S.wsLost = true;
    S.histLoading = false;
    emit('wsclose'); // plans.js: takes in flight lose their replies
    emit('conn');
    setTimeout(connect, retry);
    retry = Math.min(retry * 2, 10000);
  };
  ws.onmessage = (e) => handle(JSON.parse(e.data));
}

export function wsSend(m) {
  if (!S.ws || S.ws.readyState !== WebSocket.OPEN) return false;
  S.ws.send(JSON.stringify(m));
  return true;
}

function handle(m) {
  switch (m.t) {
    case 'hello':
      // How levels were typed until now, so views can rewrite their boxes.
      S.prevNumbering = S.cfg && { mode: S.cfg.levelNumbering, max: S.levels.length };
      S.cfg = m.cfg;
      emit('helloCfg', m.cfg); // device.js: the global device defaults
      S.pinSet = !!m.pinSet;
      S.configPath = m.configPath;
      S.version = m.version || '';
      S.levels = m.levels;
      S.levelBy = new Map(m.levels.map((l) => [l.ui, l]));
      S.levelDefaults = m.levelDefaults || [];
      setNames(m.srcs, m.dsts);
      S.namesAt = m.at;
      S.tally = new Map();
      applyTally(m.tally || []);
      S.prot = new Map();
      applyProtect(m.protect || []);
      setStatus(m.status);
      S.hist = m.recent || [];
      S.histMore = m.more;
      emit('helloData'); // filterstore.js: follow a global filter set that changed
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
      emit('tally');
      break;
    case 'protect':
      applyProtect(m.u);
      emit('tally');
      break;
    case 'configResult':
      emit('configResult', m);
      break;
    case 'unlockResult':
      emit('unlockResult', m);
      break;
    case 'filterSetsResult':
      emit('filterSetsResult', m);
      break;
    case 'locked':
      emit('locked');
      break;
    case 'status':
      setStatus(m);
      emit('conn');
      break;
    case 'sentPage': // the /log page (log.js)
    case 'sentAdd':
      emit(m.t, m);
      break;
    default: // takeResult (plans.js); recent, historyCleared, history (history.js); filterSets (filterstore.js)
      handlers[m.t]?.(m);
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

function applyProtect(list) {
  for (const [d, l, dev] of list) {
    let t = S.prot.get(d);
    if (dev < 0) {
      if (t) t.delete(l);
      continue;
    }
    if (!t) S.prot.set(d, (t = new Map()));
    t.set(l, dev);
  }
}

function setStatus(st) {
  if (st.addr !== S.routerAddr) S.routerLastErr = '';
  S.router = st.router;
  S.routerAddr = st.addr;
  S.routerErr = st.err || '';
  if (S.routerErr) S.routerLastErr = S.routerErr;
  else if (S.router === 'up') S.routerLastErr = '';
}

// Dot state and text for the WebSocket and router links, plus down:
// { title, text, err, links } once either link is lost. Not while first
// connecting, or reconnecting after a restart or new settings: the router
// only counts as lost when it's down, or still connecting after a failure.
export function connState() {
  const ws = { up: ['ok', 'Server connected'], connecting: ['warn', 'Server connecting'], down: ['error', 'Server disconnected'] }[S.wsState];
  const map = { up: ['ok', 'connected'], connecting: ['warn', 'connecting'], down: ['error', 'down'] };
  const [dot, word] = S.wsState === 'up' && S.router ? map[S.router] || ['idle', S.router] : ['idle', 'unknown'];
  // down.links: browser↔server and server↔router, true / false / null (unknown).
  let down = null;
  if (S.wsLost) {
    down = {
      title: 'Connection to Sode server interrupted',
      text: 'Reconnecting…',
      links: [false, null],
    };
  } else if (S.wsState === 'up' && S.router === 'down' && !S.routerAddr) {
    down = { title: 'No router set', text: 'Set the router address in Settings.' };
  } else if (S.wsState === 'up' && (S.router === 'down' || (S.router === 'connecting' && S.routerLastErr))) {
    down = {
      title: 'Connected to Sode server, router offline',
      text: `Reconnecting to ${S.routerAddr}…`,
      err: S.routerLastErr,
      links: [true, false],
    };
  }
  return { ws, rt: [dot, `Router ${word}`], down };
}

// What the status shows for the router (Settings → Router → Status
// shows): [text, is a name]. Name falls back to the address when unset.
export function routerLabel() {
  const show = S.cfg?.routerShow || 'address';
  if (show === 'none') return ['', false];
  if (show === 'name' && S.cfg.routerName) return [S.cfg.routerName, true];
  return [S.routerAddr || '', false];
}

// Fills (or hides) a disconnected banner: what's lost, a browser → server
// → router map with each link marked, and the router's error if any.
// Redrawn only when its content changes: status flaps while reconnecting,
// and each redraw would re-announce the alert.
let downObs;
function publishDownHeight(box) {
  document.documentElement.style.setProperty('--sode-down-h', `${box.hidden ? 0 : box.offsetHeight}px`);
}
export function renderDown(box, c) {
  box.hidden = !c.down;
  if (!downObs && typeof ResizeObserver !== 'undefined' && typeof document !== 'undefined') {
    // Publish the banner's real height so the Settings contents list clears it.
    downObs = new ResizeObserver(() => publishDownHeight(box));
    downObs.observe(box);
  }
  if (downObs) publishDownHeight(box);
  const key = c.down ? JSON.stringify([c.down, S.routerAddr, S.cfg?.routerName]) : '';
  if (box.dataset.key === key) return;
  box.dataset.key = key;
  if (!c.down) return box.replaceChildren();
  const { title, text, err, links } = c.down;
  const link = (ok) => el('span', 'sode__down-link' + (ok === false ? ' sode__down-link--lost' : ''),
    ok === true ? 'ok' : ok === false ? '✕ lost' : '?');
  box.replaceChildren(...[
    el('div', 'sode__down-title', title),
    links && el('div', 'sode__down-map',
      el('span', 'sode__down-node', 'This browser'), link(links[0]),
      el('span', 'sode__down-node', 'Sode server'), link(links[1]),
      el('span', 'sode__down-node', 'Router ', S.cfg?.routerName ? `${S.cfg.routerName} ` : '', el('code', null, S.routerAddr || '?'))),
    el('div', null, text, err ? el('div', null, el('code', null, err)) : null),
  ].filter(Boolean));
}

// Server errors are Go-style (lowercase, no full stop); shown as sentences.
export const sentence = (t) => {
  const s = String(t || '').trim();
  return s && s[0].toUpperCase() + s.slice(1) + (/[.!?…]$/.test(s) ? '' : '.');
};

// Patches can go out: Sode is reachable and connected to the router.
export const linked = () => !!S.ws && S.wsState === 'up' && S.router === 'up';

// Buttons that send to the router are marked with needsLink: shown as
// disabled while patches can't go out, and kept in step on each 'conn'
// without redrawing anything. The class and aria-disabled, not the disabled
// attribute, so a focused button keeps focus; a press still goes through
// planBlocked, which says it's not connected.
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

export function setDot(dotEl, textEl, [state, text]) {
  dotEl.className = `pb-dot pb-dot--${state}`;
  textEl.textContent = text;
}

// Badge text for protect devices: 'Protected by 9' or 'Protected by 9, 12'.
export const protectedLabel = (devices) => `Protected by ${devices.join(', ')}`;

// Protected levels of dst among levels, with the devices holding them.
export function protectedOn(dst, levels) {
  const t = S.prot.get(dst);
  const hit = t ? levels.filter((u) => t.has(u)) : [];
  return { levels: hit, devices: [...new Set(hit.map((u) => t.get(u)))].sort((a, b) => a - b) };
}
// ---- fuzzy finder

function score(it, q) {
  if (it.num === q) return 1000;
  if (/^\d+$/.test(q) && it.num.startsWith(q)) return 800 - it.num.length;
  if (it.lname === q) return 900;
  if (it.lname.startsWith(q)) return 700 - it.lname.length;
  const idx = it.lname.indexOf(q);
  if (idx >= 0) return 600 - idx;
  // Subsequence over "num name", rewarding runs and word starts.
  const t = it.num + ' ' + it.lname;
  const qq = q.replace(/\s+/g, '');
  let qi = 0, pts = 0, prev = -2;
  for (let i = 0; i < t.length && qi < qq.length; i++) {
    if (t[i] !== qq[qi]) continue;
    pts += (i === prev + 1 ? 3 : 1) + (i === 0 || t[i - 1] === ' ' ? 2 : 0);
    prev = i;
    qi++;
  }
  return qi < qq.length ? -1 : 100 + pts - t.length / 100;
}

export function search(list, q, n = 8) {
  q = q.trim().toLowerCase();
  if (!q) return [];
  const out = [];
  for (const it of list) {
    const s = score(it, q);
    if (s >= 0) out.push([s, it]);
  }
  out.sort((a, b) => b[0] - a[0] || a[1].n - b[1].n);
  const best = out.slice(0, n).map((x) => x[1]);
  best.total = out.length; // how many matched before the cut
  return best;
}
// A chevron for disclosure toggles: down when open, right when closed.
export function chevron(open) {
  const icon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  icon.setAttribute('class', 'pb-icon pb-icon--sm');
  icon.setAttribute('viewBox', '0 0 24 24');
  icon.setAttribute('aria-hidden', 'true');
  const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
  use.setAttribute('href', `vendor/patchbay/icons/lucide.svg#${open ? 'chevron-down' : 'chevron-right'}`);
  icon.append(use);
  return icon;
}
