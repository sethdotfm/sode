// Shared state, the connection to Sode, and helpers used by both layouts.
// Views subscribe with on(event, fn) and render from S.

export const $ = (id) => document.getElementById(id);

// el('div', 'cls', child, 'text', …) — builds DOM without innerHTML so
// router-supplied names can never inject markup.
export function el(tag, cls, ...kids) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  for (const k of kids) if (k != null) e.append(k);
  return e;
}

// Text cut off with an ellipsis shows in full in a tooltip on hover, as
// soon as the pointer arrives. Elements with a title of their own keep it.
const cutTip = el('div', 'sode__cut-tip');
cutTip.hidden = true;
cutTip.setAttribute('aria-hidden', 'true'); // the full text is already in the DOM
document.body.append(cutTip);
let cutFor = null;
const hideCutTip = () => { cutTip.hidden = true; cutFor = null; };

function cutOff(n) {
  for (; n instanceof HTMLElement; n = n.parentElement) {
    const cs = getComputedStyle(n);
    if (cs.textOverflow !== 'ellipsis' || cs.overflowX === 'visible') continue;
    if (n.title || n.scrollWidth <= n.clientWidth) return null;
    const text = (n instanceof HTMLInputElement ? n.value : n.textContent).replace(/\s+/g, ' ').trim();
    return text ? { n, text } : null;
  }
  return null;
}

// Under the element (above it if there's no room), kept on screen. The tip
// sits in the zoomed page, so viewport positions are scaled back by the
// zoom it actually renders at.
function placeCutTip(n) {
  cutTip.style.left = cutTip.style.top = '0px';
  const o = cutTip.getBoundingClientRect(), r = n.getBoundingClientRect();
  const k = o.width / cutTip.offsetWidth || 1, gap = 4 * k;
  const x = Math.max(gap, Math.min(r.left, innerWidth - o.width - gap));
  const y = r.bottom + gap + o.height <= innerHeight ? r.bottom + gap : r.top - gap - o.height;
  cutTip.style.left = `${(x - o.left) / k}px`;
  cutTip.style.top = `${(y - o.top) / k}px`;
}

document.addEventListener('mouseover', (e) => {
  const c = cutOff(e.target);
  if (!c) return hideCutTip();
  if (c.n === cutFor) return;
  cutFor = c.n;
  cutTip.textContent = c.text;
  cutTip.hidden = false;
  placeCutTip(c.n);
});
document.addEventListener('mouseout', (e) => { if (!e.relatedTarget) hideCutTip(); });
for (const t of ['scroll', 'pointerdown', 'keydown']) document.addEventListener(t, hideCutTip, true);

// ---- state & events

export const S = {
  cfg: null,
  configPath: '',
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
};

const listeners = {};
export const on = (ev, fn) => { (listeners[ev] ||= []).push(fn); };
const emit = (ev, ...args) => { for (const fn of listeners[ev] || []) fn(...args); };

const off = () => (S.cfg ? S.cfg.numberOffset : 0);

function item(n, name) {
  const num = String(n + off());
  return { n, name, num, lname: name.toLowerCase(), label: name ? `${num} ${name}` : num };
}
export const srcItem = (n, name = '') => S.srcBy.get(n) || item(n, name);
export const dstItem = (n, name = '') => S.dstBy.get(n) || item(n, name);
export const srcLabel = (n) => srcItem(n).label;
export const dstLabel = (n) => dstItem(n).label;
// Tally source text: null = never reported, -1 = router says nothing routed.
export const tallyText = (s) => (s == null ? 'unknown' : s < 0 ? 'nothing' : srcLabel(s));

// ---- websocket

let retry = 1000;
let prevNumbering = null; // { mode, max } before the last hello
let takeSeq = 0;
const takes = new Map(); // take id → batch

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
    if (takes.size) {
      // Replies to a dead socket are lost; the outcome is in history.
      takes.clear();
      showResult('warn', 'Connection lost', 'The take was sent but its result is unknown. Check Recent once reconnected.');
    }
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
      prevNumbering = S.cfg && { mode: S.cfg.levelNumbering, max: S.levels.length };
      S.cfg = m.cfg;
      S.pinSet = !!m.pinSet;
      S.configPath = m.configPath;
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
    case 'takeResult':
      takeResult(m);
      break;
    case 'recent':
      if (!S.hist.some((e) => e.id === m.e.id)) S.hist.unshift(m.e);
      S.newId = m.e.id; // views animate this one in, once
      emit('history');
      break;
    case 'historyCleared':
      S.hist = [];
      S.histMore = false;
      S.histLoading = false;
      emit('history');
      break;
    case 'history': {
      const last = S.hist.length ? S.hist[S.hist.length - 1].id : Infinity;
      S.hist.push(...m.entries.filter((e) => e.id < last));
      S.histMore = m.more;
      S.histLoading = false;
      emit('history');
      break;
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
export function renderDown(box, c) {
  box.hidden = !c.down;
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

// ---- levels

export const allLevels = () => S.levels.map((l) => l.ui);

// Levels are shown and typed per cfg.levelNumbering: "1" (1-17), "0"
// (0-16) or "va" (V, 1-16: "0" with level 0 shown as V). Internally they
// stay 1..N.
// With no mode it's the server's label (Level.Label); a mode is for
// previewing one that isn't saved yet.
export function levelLabel(ui, mode) {
  if (mode === undefined) {
    const l = S.levelBy.get(ui);
    if (l) return l.label;
    mode = S.cfg.levelNumbering;
  }
  if (mode === '0') return String(ui - 1);
  if (mode === 'va') return ui === 1 ? 'V' : String(ui - 1);
  return String(ui);
}

// One typed level ("3", or in "va" also "v" and "a2" = 2) → UI level, or
// NaN.
function levelTok(tok, mode) {
  const t = tok.toLowerCase();
  if (mode === 'va') {
    if (t === 'v') return 1;
    const a = t.match(/^a(\d+)$/);
    if (a) return +a[1] >= 1 ? +a[1] + 1 : NaN;
  }
  return /^\d+$/.test(t) ? +t + (mode === '1' ? 0 : 1) : NaN;
}

// "1-3,5" (or "v,1-4") in mode → sorted UI levels within 1..max, or null.
// A part starting with ! is left out: "!0" is every level but 0, and
// "1-8 !3" (or "1-8!3") is 1-8 without 3. + is the same as a comma, t,
// thru or through as -, and n or not as !: "1 thru 8 not 3".
export function parseLevelsAs(text, mode, max) {
  const out = new Set(), not = new Set();
  const norm = text.replace(/not|n/gi, '!').replace(/through|thru|t/gi, '-')
    .replace(/\s*-\s*/g, '-').replace(/!\s*/g, ' !');
  for (const part of norm.split(/[\s,+]+/).filter(Boolean)) {
    const m = part.match(/^(!?)([^-!]+)(?:-([^-!]+))?$/);
    if (!m) return null;
    let a = levelTok(m[2], mode), b = m[3] ? levelTok(m[3], mode) : a;
    if (!Number.isInteger(a) || !Number.isInteger(b)) return null;
    if (a > b) [a, b] = [b, a];
    for (let i = a; i <= b; i++) {
      if (i < 1 || i > max) return null;
      (m[1] ? not : out).add(i);
    }
  }
  if (!out.size && not.size) for (let i = 1; i <= max; i++) out.add(i);
  for (const i of not) out.delete(i);
  return out.size ? [...out].sort((x, y) => x - y) : null;
}

// What a levels box means. Blank is the default (stored as plain 1-based
// levels); none: the picker was cleared to no levels, so blank means none.
export function parseLevels(text, none = false) {
  if (none && !text.trim()) return null;
  if (!text.trim()) return parseLevelsAs(S.cfg.defaultLevels, '1', S.levels.length);
  return parseLevelsAs(text, S.cfg.levelNumbering, S.levels.length);
}

// A levels box's text rewritten for a numbering that changed with the
// last hello, so it keeps meaning the same levels. Text that didn't parse
// before is left as it is.
export function relabelLevels(text) {
  const p = prevNumbering;
  if (!p || !text.trim() || p.mode === S.cfg.levelNumbering) return text;
  const list = parseLevelsAs(text, p.mode, p.max);
  return list ? levelInput(list) : text;
}

// The default levels as they'd be typed, for a box's placeholder.
export const defaultLevelsText = () => levelInput(parseLevels('') || []);

// Collapses a sorted level list into runs: [[1,3],[5,5]].
function levelRuns(list) {
  const out = [];
  for (let i = 0; i < list.length;) {
    let j = i;
    while (j + 1 < list.length && list[j + 1] === list[j] + 1) j++;
    out.push([list[i], list[j]]);
    i = j + 1;
  }
  return out;
}
// "1–3, 5" for display; "1-3,5" for a levels box. Labels follow mode.
const runsText = (list, dash, sep, mode) => levelRuns(list)
  .map(([a, b]) => (a === b ? levelLabel(a, mode) : `${levelLabel(a, mode)}${dash}${levelLabel(b, mode)}`)).join(sep);
export const levelText = (list) => runsText(list, '–', ', ');
export const levelInput = (list, mode) => runsText(list, '-', ',', mode);

// A configured #rrggbb is one color for both themes, so its number gets
// black or white, whichever contrasts more (WCAG relative luminance).
function inkFor(hex) {
  const lin = (i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };
  const y = 0.2126 * lin(1) + 0.7152 * lin(3) + 0.0722 * lin(5);
  return (y + 0.05) / 0.05 > 1.05 / (y + 0.05) ? '#000' : '#fff';
}

// Sets --sode-c (and --sode-ink for a configured color) on a level's
// pick or slot.
export function levelColor(node, l) {
  if (l.color) {
    node.style.setProperty('--sode-c', l.color);
    node.style.setProperty('--sode-ink', inkFor(l.color));
  } else {
    node.style.setProperty('--sode-c', `var(--pb-signal-${l.swatch})`);
    node.style.removeProperty('--sode-ink');
  }
  return node;
}

// Every level in one fixed row: filled when in `on`, outlined otherwise,
// so rows line up like monospace columns. The numbers mean it never
// relies on color alone.
export function levelBar(on) {
  const set = new Set(on);
  const row = el('span', 'sode__bar', ...S.levels.map((l) => {
    const slot = el('span', 'sode__slot' + (set.has(l.ui) ? ' sode__slot--on' : ''), l.label);
    return levelColor(slot, l);
  }));
  // Two equal rows; an odd first level (0 or V) stands on its own to the left.
  const odd = S.levels.length % 2;
  if (odd) row.classList.add('sode__bar--odd');
  row.style.setProperty('--sode-bar-cols', Math.floor(S.levels.length / 2) + odd);
  const sorted = [...set].sort((a, b) => a - b);
  row.setAttribute('role', 'img');
  row.setAttribute('aria-label', sorted.length ? `Levels ${levelText(sorted)}` : 'No levels');
  return row;
}

// One numbered toggle per level, for a picker. onToggle(ui) on tap.
export function pickButtons(onToggle) {
  return S.levels.map((l) => {
    const b = el('button', 'sode__pick', l.label);
    b.type = 'button';
    b.tabIndex = -1;
    b.dataset.ui = l.ui;
    levelColor(b, l);
    b.setAttribute('aria-label', `Level ${l.label}`);
    b.addEventListener('click', () => onToggle(l.ui));
    return b;
  });
}

// ---- routing views

// One entry per source feeding dst on any of levels, in order of each
// source's first level: [{src, levels}]. src null = unknown, -1 = nothing.
export function bySource(dst, levels) {
  const t = S.tally.get(dst) || new Map();
  const groups = new Map();
  for (const u of levels) {
    const s = t.has(u) ? t.get(u) : null;
    if (!groups.has(s)) groups.set(s, []);
    groups.get(s).push(u);
  }
  return [...groups].map(([src, lv]) => ({ src, levels: lv }));
}

// Destinations src feeds on any of levels, sorted.
export function fedBy(src, levels) {
  const fed = [];
  for (const [d, t] of S.tally) {
    if (levels.some((u) => t.get(u) === src)) fed.push(d);
  }
  return fed.sort((a, b) => a - b);
}

export const srcRow = (levels, srcText) =>
  el('div', 'sode__route', levelBar(levels), el('span', 'sode__end sode__end--src', srcText));

export function routeRow(levels, srcText, dstText) {
  return el('div', 'sode__route',
    levelBar(levels),
    el('span', 'sode__end sode__end--src', srcText),
    el('span', 'sode__arrow', '→'),
    el('span', 'sode__end sode__end--dst', dstText));
}

// The whole of dst: a heading, one row per source, and any protect.
export function currentlyNodes(dst) {
  const all = allLevels();
  const rows = bySource(dst.n, all).map((r) => srcRow(r.levels, tallyText(r.src)));
  const p = protectedOn(dst.n, all);
  const prot = p.levels.length
    ? el('div', 'sode__route', levelBar(p.levels), el('span', 'pb-badge pb-badge--warn', 'Protected'),
      `by device ${p.devices.join(', ')}`)
    : null;
  return [el('div', 'sode__label', `Currently on ${dst.label}`), ...rows, prot].filter(Boolean);
}

// ---- history

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

// Clock, per browser: '24' (default), '12', or 'locale' to follow the
// browser's language. Pages can't read the OS 12/24-hour switch.
const CLOCK_KEY = 'sode-clock';
export function clockPref() {
  let v = null;
  try { v = localStorage.getItem(CLOCK_KEY); } catch (e) {}
  return v === '12' || v === 'locale' ? v : '24';
}

// Scroll to the top when a box is focused (desktop): per browser, on unless
// turned off.
const SCROLL_KEY = 'sode-scroll-top';
export function scrollTopPref() {
  try { return localStorage.getItem(SCROLL_KEY) !== 'off'; } catch (e) { return true; }
}
export function setScrollTopPref(on) {
  try {
    if (on) localStorage.removeItem(SCROLL_KEY);
    else localStorage.setItem(SCROLL_KEY, 'off');
  } catch (e) {}
}

// The time formatter, built once and again only when the Clock changes.
let timeFmt = null;
function timeFormat() {
  if (timeFmt) return timeFmt;
  const pref = clockPref();
  const opts = { minute: '2-digit', second: '2-digit' };
  // hourCycle, not hour12: Chrome shows 24:05 for hour12: false.
  if (pref !== 'locale') opts.hourCycle = pref === '12' ? 'h12' : 'h23';
  // 3:04:05 PM but 03:04:05 / 15:04:05.
  const h12 = /h1[12]/.test(new Intl.DateTimeFormat([], { hour: 'numeric', ...opts }).resolvedOptions().hourCycle);
  timeFmt = new Intl.DateTimeFormat([], { hour: h12 ? 'numeric' : '2-digit', ...opts });
  return timeFmt;
}

export const fmtTime = (iso) => {
  const d = new Date(iso);
  const t = timeFormat().format(d);
  return d.toDateString() === new Date().toDateString() ? t : `${d.toLocaleDateString()} ${t}`;
};

// ---- plans: { verb, items: [{ dst, groups: [{src, levels}], skipped, undo, retry, prot }] }
// One item per destination. undo/retry are the history entries an item
// undoes or retries; prot is set by planBlocked when dst is protected.

// Groups an entry's previous routing into restorable takes.
function undoItem(e) {
  const groups = new Map();
  const skipped = [];
  for (const [u, s] of e.prev) {
    if (!S.levelBy.has(u)) continue;
    if (s < 0) { skipped.push(u); continue; }
    if (!groups.has(s)) groups.set(s, []);
    groups.get(s).push(u);
  }
  const known = new Set(e.prev.map(([u]) => u));
  for (const u of e.levels) if (!known.has(u)) skipped.push(u);
  return {
    dst: dstItem(e.dst, e.dstName),
    groups: [...groups].map(([s, levels]) => ({ src: srcItem(s), levels: levels.sort((a, b) => a - b) })),
    skipped: skipped.sort((a, b) => a - b),
    undo: e, retry: null,
  };
}

// A failed take, tried again as a fresh patch of the same route.
const retryItem = (e) => ({
  dst: dstItem(e.dst, e.dstName),
  groups: [{ src: srcItem(e.src, e.srcName), levels: e.levels.filter((u) => S.levelBy.has(u)).sort((a, b) => a - b) }],
  skipped: [], undo: null, retry: e,
});

export const patchPlan = (src, dsts, levels) => ({
  verb: 'patch',
  items: dsts.map((dst) => ({ dst, groups: [{ src, levels }], skipped: [], undo: null, retry: null })),
});
export const undoPlan = (e) => ({ verb: 'undo', items: [undoItem(e)] });
export const retryPlan = (e) => ({ verb: 'retry', items: [retryItem(e)] });
export const folderUndoPlan = (f) => ({ verb: 'undo', items: f.entries.map(undoItem) });
export const folderRetryPlan = (f) => ({ verb: 'retry', items: f.entries.filter((e) => !e.ok).map(retryItem) });
// Undo for a history row: a single entry or a whole folder.
export const rowUndoPlan = (row) => (row.f ? folderUndoPlan(row.f) : undoPlan(row.e));

// 'undo', 'retry' or 'take' (a 'patch' plan), for wording confirms.
export const planVerb = (plan) => (plan.verb === 'patch' ? 'take' : plan.verb);
// True when a plan has nothing to send (an undo with nothing routed before).
export const planEmpty = (plan) => !plan.items.some((i) => i.groups.length);

// What dst had on a take's levels just before it, one entry per source in
// order of first level: [{src, levels}]. src -1 = nothing routed, null =
// not known at the time.
export function beforeGroups(e) {
  const prev = new Map(e.prev);
  const groups = new Map();
  for (const u of [...e.levels].sort((a, b) => a - b)) {
    if (!S.levelBy.has(u)) continue;
    const s = prev.has(u) ? prev.get(u) : null;
    if (!groups.has(s)) groups.set(s, []);
    groups.get(s).push(u);
  }
  return [...groups].map(([src, levels]) => ({ src, levels }));
}

// " (undo of #3; 4–17 already)" for a history entry, or null.
export function entryNote(e) {
  const parts = [];
  if (e.undo != null) parts.push(`undo of #${e.undo}`);
  if (e.retry != null) parts.push(`retry of #${e.retry}`);
  if (e.already?.length) parts.push(`${levelText(e.already)} already`);
  return parts.length ? el('span', 'sode__hist-note', ` (${parts.join('; ')})`) : null;
}

// History as rows, newest first: { e } for a take on its own, or { f } for
// a folder holding the takes of one multi-take confirm. A folder sits
// where its newest take is.
export function histRows(list) {
  const rows = [];
  const folders = new Map();
  for (const e of list) {
    if (!e.group || e.groupN < 2) { rows.push({ e }); continue; }
    let f = folders.get(e.group);
    if (!f) {
      f = { group: e.group, n: e.groupN, entries: [] };
      folders.set(e.group, f);
      rows.push({ f });
    }
    f.entries.push(e);
  }
  return rows;
}

// A folder's takes arrive one by one as each finishes; one still filling
// is "in flight" for a while, after that missing takes just aren't loaded.
const IN_FLIGHT_MS = 30000;

// Master status of a folder: { dot, text, pulse, title }.
export function folderStatus(f) {
  const got = f.entries.length;
  const bad = f.entries.filter((e) => !e.ok).length;
  if (got < f.n && Date.now() - Date.parse(f.entries[0].time) < IN_FLIGHT_MS) {
    return { dot: bad ? 'warn' : 'info', text: `${got}/${f.n} done…`, pulse: true };
  }
  const title = got < f.n ? `${got} of ${f.n} loaded` : '';
  if (!bad) return { dot: 'ok', text: 'ok', title };
  if (bad === got) return { dot: 'error', text: 'failed', title };
  return { dot: 'warn', text: `${bad}/${got} failed`, title };
}

// A pb-status node for a single take or a folder.
export function statusNode(row) {
  const st = row.f ? folderStatus(row.f) : { dot: row.e.ok ? 'ok' : 'error', text: row.e.ok ? 'ok' : 'failed' };
  const dot = el('span', `pb-dot pb-dot--${st.dot}${st.pulse ? ' pb-dot--pulse' : ''}`);
  dot.setAttribute('aria-hidden', 'true');
  const node = el('span', 'pb-status', dot, st.text);
  if (row.e && !row.e.ok) node.title = row.e.err || '';
  if (st.title) node.title = st.title;
  return node;
}

// "CAM 1 → 5 destinations", as nodes, with a note for undos and retries.
export function folderTitle(f) {
  const srcs = [...new Set(f.entries.map((e) => e.src))];
  const dsts = [...new Set(f.entries.map((e) => e.dst))];
  const strong = (t) => el('strong', null, t);
  const src = srcs.length === 1 ? srcItem(srcs[0], f.entries[0].srcName).label : `${srcs.length} sources`;
  const dst = dsts.length === 1 ? dstItem(dsts[0], f.entries[0].dstName).label : `${dsts.length} destinations`;
  const kind = f.entries.every((e) => e.undo != null) ? 'undo' : f.entries.every((e) => e.retry != null) ? 'retry' : '';
  return [strong(src), ' → ', strong(dst), kind ? el('span', 'sode__hist-note', ` (${kind})`) : null].filter(Boolean);
}

// Every level a folder's takes touched.
export const folderLevels = (f) => [...new Set(f.entries.flatMap((e) => e.levels))].filter((u) => S.levelBy.has(u)).sort((a, b) => a - b);

// "#4–#9", oldest to newest.
export function folderIds(f) {
  const ids = f.entries.map((e) => e.id);
  return `#${Math.min(...ids)}–#${Math.max(...ids)}`;
}

// Asks for the next page of history. True if a request went out.
export function requestHistory() {
  if (!S.histMore || S.histLoading || !S.hist.length) return false;
  S.histLoading = wsSend({ t: 'history', before: S.hist[S.hist.length - 1].id });
  return S.histLoading;
}

// Shows history `step` rows at a time. more() reveals the next step,
// fetching another page from Sode once the loaded entries run out.
export function historyPager(step) {
  const pg = {
    shown: step,
    reset() { pg.shown = step; },
    list: () => histRows(S.hist).slice(0, pg.shown),
    hasMore: () => histRows(S.hist).length > pg.shown || S.histMore,
    more() {
      pg.shown += step;
      if (histRows(S.hist).length < pg.shown) requestHistory();
      emit('history');
    },
    // The "Load more" button, or null when there's nothing more.
    button() {
      if (!S.hist.length || !pg.hasMore()) return null;
      const b = el('button', 'pb-btn pb-btn--outline', S.histLoading ? 'Loading…' : 'Load more');
      b.type = 'button';
      b.disabled = S.histLoading;
      b.addEventListener('click', pg.more);
      return b;
    },
  };
  return pg;
}

// ---- results

export function showResult(kind, title, text) {
  S.result = { kind, title, text };
  emit('result', S.result);
}

export function resultNode(r) {
  return el('div', `pb-alert pb-alert--${r.kind}`, el('div', null, el('div', 'pb-alert__title', r.title), r.text));
}

// "A → B (levels 1–3; 2 was already on A)" for one take.
function takeText(t, already = []) {
  return `${t.g.src.label} → ${t.dst.label} (levels ${levelText(t.g.levels)}` +
    (already.length ? `; ${levelText(already)} ${already.length === 1 ? 'was' : 'were'} already on ${t.g.src.label}` : '') + ')';
}

// Joins the first few of list, then "and N more".
function capped(list, n = 3) {
  return list.length <= n ? list.join('; ') : `${list.slice(0, n).join('; ')}; and ${list.length - n} more`;
}

// A batch is the takes from one confirm. It reports once, after every
// take has answered.
function takeResult(m) {
  const b = takes.get(m.id);
  if (!b) return;
  takes.delete(m.id);
  b.results.push({ ...b.byId.get(m.id), ok: m.ok, err: m.err, already: m.already || [] });
  if (b.results.length < b.byId.size) return;
  const undo = b.verb === 'undo';
  const dsts = new Set(b.results.map((r) => r.dst.n)).size;
  const failed = b.results.filter((r) => !r.ok);
  if (!failed.length) {
    const many = dsts > 1 ? ` ${dsts} destinations` : '';
    const title = undo ? `Undone${many && ' on' + many}` : `Success${many && ':' + many}`;
    showResult('ok', title, capped(b.results.map((r) => takeText(r, r.already))));
    return;
  }
  const partly = failed.length < b.results.length;
  const count = b.results.length > 1 ? ` (${failed.length} of ${b.results.length})` : '';
  const title = `${undo ? 'Undo' : 'Take'} ${partly ? 'partly failed' : 'failed'}${count}`;
  showResult('error', title, capped(failed.map((r) => `${takeText(r)}: ${r.err}`), 5));
}

// Why a plan can't be confirmed right now, as a result banner, or null.
// Marks each protected item's prot. A plan only blocks when every
// destination is protected; otherwise those are refused and the rest go.
export function planBlocked(plan) {
  if (!linked()) {
    return { kind: 'error', title: 'Not connected', text: 'Sode is not connected to the router.' };
  }
  const live = plan.items.filter((i) => i.groups.length);
  for (const i of live) {
    const p = protectedOn(i.dst.n, i.groups.flatMap((g) => g.levels));
    i.prot = p.levels.length ? p : null;
  }
  const prot = live.filter((i) => i.prot);
  if (!prot.length || prot.length < live.length) return null;
  if (prot.length === 1) {
    const [i] = prot;
    return {
      kind: 'error', title: 'Destination protected',
      text: `${i.dst.label} is protected on levels ${levelText(i.prot.levels)} by device ${i.prot.devices.join(', ')}.`,
    };
  }
  return { kind: 'error', title: 'Destinations protected', text: `All ${prot.length} destinations are protected: ${prot.map((i) => i.dst.label).join(', ')}.` };
}

function planQuestion(plan) {
  const strong = (t) => el('strong', null, t);
  const n = plan.items.length;
  const to = n === 1 ? ['Destination ', strong(plan.items[0].dst.label)] : [strong(`${n} destinations`)];
  if (plan.verb === 'undo') {
    const srcs = [...new Set(plan.items.map((i) => srcItem(i.undo.src, i.undo.srcName).label))];
    return el('div', null, 'Are you sure you want to undo the take of ', srcs.length === 1 ? strong(srcs[0]) : `${srcs.length} sources`,
      ' to ', ...to, '?');
  }
  const srcs = [...new Set(plan.items.flatMap((i) => i.groups.map((g) => g.src.label)))];
  return el('div', null, `Are you sure you want to ${plan.verb === 'retry' ? 'retry taking' : 'take'} `,
    ...(srcs.length === 1 ? ['Source ', strong(srcs[0])] : [strong(`${srcs.length} sources`)]), ' to ', ...to, '?');
}

// One destination's part of a confirm: new routes, then what they replace.
function itemSummary(item, boxed) {
  const all = item.groups.flatMap((g) => g.levels).sort((a, b) => a - b);
  const before = bySource(item.dst.n, all).map((r) => srcRow(r.levels, tallyText(r.src)));
  return [
    ...item.groups.map((g) => routeRow(g.levels, g.src.label, item.dst.label)),
    item.prot
      ? el('div', 'sode__route', levelBar(item.prot.levels), el('span', 'pb-badge pb-badge--warn', 'Protected'),
        `by device ${item.prot.devices.join(', ')}; will be refused`)
      : null,
    item.skipped.length
      ? el('div', 'sode__hist-note', `Level${item.skipped.length === 1 ? '' : 's'} ${levelText(item.skipped)}: nothing was routed before, left as is.`)
      : null,
    ...(!before.length ? [] : boxed
      ? [el('div', 'sode__plan-before', el('div', 'sode__label', 'Replacing'), ...before)]
      : [el('div', 'sode__label', 'Replacing'), ...before]),
  ].filter(Boolean);
}

// What a confirm shows: the question, then each destination's new routes
// and what they replace. With several destinations each gets its own
// block, headed by its name, so the groups are easy to tell apart.
export function planSummary(plan) {
  if (plan.items.length === 1) return [planQuestion(plan), ...itemSummary(plan.items[0], false)];
  return [
    planQuestion(plan),
    ...plan.items.map((i, n) => el('section', 'sode__plan-item',
      el('div', 'sode__plan-head', el('span', 'sode__plan-n', `${n + 1}/${plan.items.length}`), el('strong', null, i.dst.label)),
      ...itemSummary(i, true))),
  ];
}

// Sends every take in a plan as one batch. False if the connection is gone.
export function sendTakes(p) {
  const batch = { verb: p.verb, byId: new Map(), results: [] };
  const items = [];
  for (const it of p.items) {
    for (const g of it.groups) {
      const id = ++takeSeq;
      const msg = { id, src: g.src.n, dst: it.dst.n, levels: g.levels };
      if (it.undo) msg.undo = it.undo.id;
      if (it.retry) msg.retry = it.retry.id;
      items.push(msg);
      batch.byId.set(id, { dst: it.dst, g });
    }
  }
  if (!wsSend({ t: 'batch', items })) {
    showResult('error', 'Not connected', 'Lost connection to Sode before the take was sent.');
    return false;
  }
  for (const id of batch.byId.keys()) takes.set(id, batch);
  const list = [...batch.byId.values()];
  const dsts = new Set(list.map((t) => t.dst.n)).size;
  const text = dsts > 1 && new Set(list.map((t) => t.g.src.n)).size === 1
    ? `${list[0].g.src.label} → ${dsts} destinations (levels ${levelText(list[0].g.levels)})`
    : capped(list.map((t) => takeText(t)));
  showResult('info', p.verb === 'undo' ? 'Undoing…' : 'Taking…', text);
  return true;
}

// ---- theme

// Follows the OS until set in Settings (or with Alt/Option+Shift+T); the
// choice is remembered per browser.
export function initTheme() {
  const root = document.documentElement;
  const osDark = matchMedia('(prefers-color-scheme: dark)');
  const isDark = () => (root.dataset.theme ? root.dataset.theme === 'dark' : osDark.matches);
  const select = $('cfg-theme');
  const sync = () => { select.value = root.dataset.theme || 'system'; };

  // pref: 'system', 'light' or 'dark'.
  function setTheme(pref) {
    const ms = parseFloat(getComputedStyle(root).getPropertyValue('--pb-motion-theme')) || 0;
    root.classList.add('pb-theme-transition');
    if (pref === 'system') delete root.dataset.theme;
    else root.dataset.theme = pref;
    setTimeout(() => root.classList.remove('pb-theme-transition'), ms);
    try {
      if (pref === 'system') localStorage.removeItem('sode-theme');
      else localStorage.setItem('sode-theme', pref);
    } catch (e) {}
    sync();
  }

  // Alt/Option+Shift+T: browsers keep Mod+Shift+T for reopening tabs.
  // Matched by e.code because Option changes e.key on macOS.
  if (/Mac|iPhone|iPad/.test(navigator.platform)) {
    $('cfg-theme-hint').replaceChildren(el('code', null, '⌥⇧T'), ' toggles');
  }
  select.addEventListener('change', () => setTheme(select.value));
  document.addEventListener('keydown', (e) => {
    if (e.code !== 'KeyT' || !e.shiftKey || !e.altKey || e.ctrlKey || e.metaKey || e.repeat) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    setTheme(isDark() ? 'light' : 'dark');
  }, true);
  sync();
}

// The Clock select in Settings (synced when Settings opens) applies at
// once and redraws every time.
export function initClock() {
  const select = $('cfg-clock');
  select.addEventListener('change', () => {
    try {
      if (select.value === '24') localStorage.removeItem(CLOCK_KEY);
      else localStorage.setItem(CLOCK_KEY, select.value);
    } catch (e) {}
    timeFmt = null;
    emit('history');
    emit('namesChecked');
  });
}
