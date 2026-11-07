// Shared state, the socket, and helpers for both layouts. Imports only tooltip.js: nothing may
// cycle.

import './tooltip.js';

export const $ = (id) => document.getElementById(id);

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
  brandColors: [],
  srcs: [],
  dsts: [],
  srcBy: new Map(),
  dstBy: new Map(),
  tally: new Map(),
  prot: new Map(),
  levels: [],
  levelDefaults: [],
  levelBy: new Map(),
  namesAt: null,
  wsState: 'connecting',
  wsLost: false,
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
};

export function toggle(set, k) {
  if (set.has(k)) set.delete(k);
  else set.add(k);
}

// is() is true while the box still holds the search left by the last + (not a destination
// waiting to be picked).
export function keepSearch(read) {
  let kept = null;
  return {
    is: () => kept !== null && read() === kept,
    set: (keep, text) => {
      kept = keep && text.trim() ? text : null;
    },
  };
}

const listeners = {};
export const on = (ev, fn) => {
  (listeners[ev] ||= []).push(fn);
};
// Runs before earlier listeners: for a module that must refresh its memo before views read it.
export const onFirst = (ev, fn) => {
  (listeners[ev] ||= []).unshift(fn);
};
export const emit = (ev, ...args) => {
  for (const fn of listeners[ev] || []) fn(...args);
};

const handlers = {};
export const onMsg = (t, fn) => {
  if (handlers[t]) throw new Error(`onMsg('${t}') is already handled`);
  handlers[t] = fn;
};

export const off = () => (S.cfg ? S.cfg.numberOffset : 0);

export function item(n, name) {
  const num = String(n + off());
  return { n, name, num, lname: name.toLowerCase(), label: name ? `${num} ${name}` : num };
}
export const srcItem = (n, name = '') => S.srcBy.get(n) || item(n, name);
export const dstItem = (n, name = '') => S.dstBy.get(n) || item(n, name);
export const srcLabel = (n) => srcItem(n).label;
// Number is muted and fixed-width so it can't run into a name starting with a digit.
export const labelNode = (it) => el('span', 'sode__lbl', el('span', 'sode__lbl-n', it.num), ...(it.name ? [' ', it.name] : []));
export const dstLabel = (n) => dstItem(n).label;
export const tallyText = (s) => (s == null ? 'unknown' : s < 0 ? 'nothing' : srcLabel(s));

// ---- websocket

let retry = 1000;

// Browsers can't see ping frames; the server sends {"t":"ping"} every ~5s. A socket quiet for
// STALE_MS is dead even if OPEN; nothing is sent into one quiet for FRESH_MS.
// ws.lastRx is when anything last arrived.
const STALE_MS = 15000;
const FRESH_MS = 7000;
const quiet = (ws) => Date.now() - ws.lastRx;

// A dead link's close event can be long in coming, so report the loss now, once.
function drop(ws) {
  const lost = ws.onclose;
  ws.onclose = ws.onmessage = null;
  try {
    ws.close();
  } catch {
    /* already closed */
  }
  lost?.();
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
  ws.onclose = () => {
    clearInterval(watch);
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

function handle(m) {
  switch (m.t) {
    case 'hello':
      S.prevNumbering = S.cfg && { mode: S.cfg.levelNumbering, max: S.levels.length };
      S.cfg = m.cfg;
      emit('helloCfg', m.cfg);
      S.pinSet = !!m.pinSet;
      S.configPath = m.configPath;
      S.version = m.version || '';
      S.brandColors = m.brandColors || [];
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
      // A page asked for before this snapshot is for a replaced list and is ignored (takePage):
      // don't wait for it.
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
    case 'locked':
      emit('locked');
      break;
    case 'status':
      setStatus(m);
      emit('conn');
      break;
    case 'ping':
      break;
    case 'sentPage':
    case 'uptimeResult':
    case 'sentAdd':
      emit(m.t, m);
      break;
    default:
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

export function connState() {
  const ws = { up: ['ok', 'Server connected'], connecting: ['warn', 'Server connecting'], down: ['error', 'Server disconnected'] }[S.wsState];
  const map = { up: ['ok', 'connected'], connecting: ['warn', 'connecting'], down: ['error', 'down'] };
  const [dot, word] = S.wsState === 'up' && S.router ? map[S.router] || ['idle', S.router] : ['idle', 'unknown'];
  let down = null;
  if (S.wsLost) {
    down = {
      title: 'Lost link: Server link down',
      text: 'Reconnecting…',
      links: [false, null],
    };
  } else if (S.wsState === 'up' && S.router === 'down' && !S.routerAddr) {
    down = { title: 'No router set', text: 'Set the router address in Settings.' };
  } else if (S.wsState === 'up' && (S.router === 'down' || (S.router === 'connecting' && S.routerLastErr))) {
    down = {
      title: 'Lost link: Router link down',
      text: `Reconnecting to ${S.routerAddr}…`,
      err: S.routerLastErr,
      links: [true, false],
    };
  }
  return { ws, rt: [dot, `Router ${word}`], down };
}

export function routerLabel() {
  const show = S.cfg?.routerShow || 'address';
  if (show === 'none') return ['', false];
  if (show === 'name' && S.cfg.routerName) return [S.cfg.routerName, true];
  return [S.routerAddr || '', false];
}

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
    banners.add(box);
    downObs.observe(box);
  }
  if (downObs) publishDownHeight();
  const key = c.down ? JSON.stringify([c.down, S.routerAddr, S.cfg?.routerName]) : '';
  if (box.dataset.key === key) return;
  box.dataset.key = key;
  if (!c.down) return box.replaceChildren();
  const { title, text, err, links } = c.down;
  const link = (ok) => el('span', 'sode__down-link' + (ok === false ? ' sode__down-link--lost' : ''), ok === true ? 'ok' : ok === false ? '✕ lost' : '?');
  box.replaceChildren(
    ...[
      el('div', 'sode__down-title', title),
      links &&
        el(
          'div',
          'sode__down-map',
          el('span', 'sode__down-node', 'This browser'),
          link(links[0]),
          el('span', 'sode__down-node', 'Sode server'),
          link(links[1]),
          el('span', 'sode__down-node', 'Router ', S.cfg?.routerName ? `${S.cfg.routerName} ` : '', el('code', null, S.routerAddr || '?')),
        ),
      el('div', null, text, err ? el('div', null, el('code', null, err)) : null),
    ].filter(Boolean),
  );
}

export const sentence = (t) => {
  const s = String(t || '').trim();
  return s && s[0].toUpperCase() + s.slice(1) + (/[.!?…]$/.test(s) ? '' : '.');
};

// names: names already taken (any case); old: the name being edited, may change case; reserved:
// a name nothing may take.
export function nameProblem(name, names, { noun = 'item', old = '', max = 32, reserved = '' } = {}) {
  const n = name.trim();
  if (!n) return 'Name required.';
  if ([...n].length > max) return `Names are at most ${max} characters.`;
  if (reserved && n.toLowerCase() === reserved.toLowerCase()) return `“${n}” is taken.`;
  if (names.some((x) => x.toLowerCase() === n.toLowerCase() && x.toLowerCase() !== old.toLowerCase())) {
    return `There is already a ${noun} named “${n}”.`;
  }
  return '';
}

export const linked = () => !!S.ws && S.wsState === 'up' && S.router === 'up';

// needsLink buttons look disabled via class and aria-disabled, not the disabled attribute, so a
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

export function setDot(dotEl, textEl, [state, text]) {
  dotEl.className = `pb-dot pb-dot--${state}`;
  textEl.textContent = text;
}

export const protectedLabel = (devices) => `Protected by ${devices.join(', ')}`;

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
  let qi = 0,
    pts = 0,
    prev = -2;
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
export function icon(name, sm = true) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('class', sm ? 'pb-icon pb-icon--sm' : 'pb-icon');
  svg.setAttribute('viewBox', '0 0 24 24');
  svg.setAttribute('aria-hidden', 'true');
  const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
  use.setAttribute('href', `vendor/patchbay/icons/lucide.svg#${name}`);
  svg.append(use);
  return svg;
}

export const chevron = (open) => icon(open ? 'chevron-down' : 'chevron-right');

// Enter never submits the page. (An inline onsubmit would break the CSP, see http.go.)
if (typeof document !== 'undefined') {
  document.addEventListener('submit', (e) => {
    if (e.target.matches?.('.sode__boxes, .sode-log__filters')) e.preventDefault();
  });
}
