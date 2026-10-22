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

// ---- state & events

export const S = {
  cfg: null,
  configPath: '',
  srcs: [], dsts: [],
  srcBy: new Map(), dstBy: new Map(),
  tally: new Map(), // dst → Map(ui level → src)
  prot: new Map(),  // dst → Map(ui level → protecting device)
  levels: [],
  levelBy: new Map(),
  namesAt: null,
  wsState: 'connecting',
  router: null, routerAddr: '', routerErr: '',
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
let takeSeq = 0;
const takes = new Map(); // take id → batch

export function connect() {
  const base = location.pathname.replace(/[^/]*$/, '');
  const ws = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + base + 'ws');
  S.ws = ws;
  S.wsState = 'connecting';
  emit('conn');
  ws.onopen = () => { retry = 1000; S.wsState = 'up'; emit('conn'); };
  ws.onclose = () => {
    S.ws = null;
    S.wsState = 'down';
    S.histLoading = false;
    if (takes.size) {
      // Replies to a dead socket are lost; the outcome is in history.
      takes.clear();
      showResult('warn', 'Connection lost', 'The patch was sent but its result is unknown. Check Recent once reconnected.');
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
      S.cfg = m.cfg;
      S.configPath = m.configPath;
      S.levels = m.levels;
      S.levelBy = new Map(m.levels.map((l) => [l.ui, l]));
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
  S.router = st.router;
  S.routerAddr = st.addr;
  S.routerErr = st.err || '';
}

// Dot state and text for the WebSocket and router links.
export function connState() {
  const ws = { up: ['ok', 'WebSocket connected'], connecting: ['warn', 'WebSocket connecting'], down: ['error', 'WebSocket disconnected'] }[S.wsState];
  const map = { up: ['ok', 'connected'], connecting: ['warn', 'connecting'], down: ['error', 'down'] };
  const [dot, word] = S.wsState === 'up' && S.router ? map[S.router] || ['idle', S.router] : ['idle', 'unknown'];
  return { ws, rt: [dot, `SW-P-08 ${word}`] };
}

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
  return out.slice(0, n).map((x) => x[1]);
}

// ---- levels

export const allLevels = () => S.levels.map((l) => l.ui);

// none: the picker was cleared to no levels, so blank means none rather
// than the default.
export function parseLevels(text, none = false) {
  if (none && !text.trim()) return null;
  const t = text.trim() || S.cfg.defaultLevels;
  const out = new Set();
  for (const part of t.split(/[\s,]+/).filter(Boolean)) {
    const m = part.match(/^(\d+)(?:-(\d+))?$/);
    if (!m) return null;
    let a = +m[1], b = m[2] ? +m[2] : a;
    if (a > b) [a, b] = [b, a];
    for (let i = a; i <= b; i++) {
      if (!S.levelBy.has(i)) return null;
      out.add(i);
    }
  }
  return out.size ? [...out].sort((x, y) => x - y) : null;
}

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
// "1–3, 5" for display; "1-3,5" for a levels box.
export const levelText = (list) => levelRuns(list).map(([a, b]) => (a === b ? a : `${a}–${b}`)).join(', ');
export const levelInput = (list) => levelRuns(list).map(([a, b]) => (a === b ? a : `${a}-${b}`)).join(',');

// Every level in one fixed row: filled when in `on`, outlined otherwise,
// so rows line up like monospace columns. The numbers mean it never
// relies on colour alone.
export function levelBar(on) {
  const set = new Set(on);
  const row = el('span', 'sode__bar', ...S.levels.map((l) => {
    const slot = el('span', 'sode__slot' + (set.has(l.ui) ? ' sode__slot--on' : ''), String(l.ui));
    slot.style.setProperty('--sode-c', `var(--pb-signal-${l.swatch})`);
    return slot;
  }));
  const sorted = [...set].sort((a, b) => a - b);
  row.setAttribute('role', 'img');
  row.setAttribute('aria-label', sorted.length ? `Levels ${levelText(sorted)}` : 'No levels');
  return row;
}

// One numbered toggle per level, for a picker. onToggle(ui) on tap.
export function pickButtons(onToggle) {
  return S.levels.map((l) => {
    const b = el('button', 'sode__pick', String(l.ui));
    b.type = 'button';
    b.tabIndex = -1;
    b.dataset.ui = l.ui;
    b.style.setProperty('--sode-c', `var(--pb-signal-${l.swatch})`);
    b.setAttribute('aria-label', `Level ${l.ui}`);
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

export const fmtTime = (iso) => {
  const d = new Date(iso);
  const t = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
  return d.toDateString() === new Date().toDateString() ? t : `${d.toLocaleDateString()} ${t}`;
};

// Groups an entry's previous routing into restorable takes.
export function undoPlan(e) {
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
    undo: e,
  };
}

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

// A failed take, tried again as a fresh patch of the same route.
export function retryPlan(e) {
  const plan = patchPlan(srcItem(e.src, e.srcName), dstItem(e.dst, e.dstName),
    e.levels.filter((u) => S.levelBy.has(u)).sort((a, b) => a - b));
  plan.retry = e;
  return plan;
}

// 'undo', 'retry' or 'patch', for wording confirms.
export const planVerb = (plan) => (plan.undo ? 'undo' : plan.retry ? 'retry' : 'patch');

// " (undo of #3; 4–17 already)" for a history entry, or null.
export function entryNote(e) {
  const parts = [];
  if (e.undo != null) parts.push(`undo of #${e.undo}`);
  if (e.retry != null) parts.push(`retry of #${e.retry}`);
  if (e.already?.length) parts.push(`${levelText(e.already)} already`);
  return parts.length ? el('span', 'sode__hist-note', ` (${parts.join('; ')})`) : null;
}

// Asks for the next page of history. True if a request went out.
export function requestHistory() {
  if (!S.histMore || S.histLoading || !S.hist.length) return false;
  S.histLoading = wsSend({ t: 'history', before: S.hist[S.hist.length - 1].id });
  return S.histLoading;
}

// Shows history `step` entries at a time. more() reveals the next step,
// fetching another page from Sode once the loaded entries run out.
export function historyPager(step) {
  const pg = {
    shown: step,
    reset() { pg.shown = step; },
    list: () => S.hist.slice(0, pg.shown),
    hasMore: () => S.hist.length > pg.shown || S.histMore,
    more() {
      pg.shown += step;
      if (S.hist.length < pg.shown) requestHistory();
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

// A batch is the takes from one confirm (an undo can restore several
// sources). It reports once, after every take has answered.
function takeResult(m) {
  const b = takes.get(m.id);
  if (!b) return;
  takes.delete(m.id);
  b.results.push({ g: b.byId.get(m.id), ok: m.ok, err: m.err, already: m.already || [] });
  if (b.results.length < b.byId.size) return;
  // Levels the router confirmed were already on the source (no change).
  const what = (g, already = []) => `${g.src.label} → ${b.dst.label} (levels ${levelText(g.levels)}` +
    (already.length ? `; ${levelText(already)} ${already.length === 1 ? 'was' : 'were'} already on ${g.src.label}` : '') + ')';
  const failed = b.results.filter((r) => !r.ok);
  if (!failed.length) {
    showResult('ok', b.undo ? 'Undone' : 'Patched', b.results.map((r) => what(r.g, r.already)).join('; '));
    return;
  }
  const title = b.undo ? (failed.length < b.results.length ? 'Undo partly failed' : 'Undo failed') : 'Patch failed';
  showResult('error', title, failed.map((r) => `${what(r.g)}: ${r.err}`).join('; '));
}

// ---- plans: { dst, groups: [{src, levels}], skipped, undo }

export const patchPlan = (src, dst, levels) => ({ dst, groups: [{ src, levels }], skipped: [], undo: null, retry: null });

// Why a plan can't be confirmed right now, as a result banner, or null.
export function planBlocked(plan) {
  if (!S.ws || S.router !== 'up') {
    return { kind: 'error', title: 'Not connected', text: 'Sode is not connected to the router.' };
  }
  const p = protectedOn(plan.dst.n, plan.groups.flatMap((g) => g.levels));
  if (p.levels.length) {
    return {
      kind: 'error', title: 'Destination protected',
      text: `${plan.dst.label} is protected on levels ${levelText(p.levels)} by device ${p.devices.join(', ')}.`,
    };
  }
  return null;
}

// What a confirm shows: the question, the new routes, and what they replace.
export function planSummary(plan) {
  const all = plan.groups.flatMap((g) => g.levels).sort((a, b) => a - b);
  const before = bySource(plan.dst.n, all).map((r) => srcRow(r.levels, tallyText(r.src)));
  const after = plan.groups.map((g) => routeRow(g.levels, g.src.label, plan.dst.label));
  const question = plan.undo
    ? el('div', null, 'Are you sure you want to undo the patch of ', el('strong', null, srcItem(plan.undo.src, plan.undo.srcName).label),
      ' to Destination ', el('strong', null, plan.dst.label), '?')
    : el('div', null, `Are you sure you want to ${plan.retry ? 'retry patching' : 'patch'} Source `, el('strong', null, plan.groups[0].src.label),
      ' to Destination ', el('strong', null, plan.dst.label), '?');
  return [
    question,
    ...after,
    plan.skipped.length
      ? el('div', 'sode__hist-note', `Level${plan.skipped.length === 1 ? '' : 's'} ${levelText(plan.skipped)}: nothing was routed before, left as is.`)
      : null,
    el('div', 'sode__label', 'Replacing'),
    ...before,
  ].filter(Boolean);
}

// Sends every take in a plan. False if the connection is gone.
export function sendTakes(p) {
  const batch = { dst: p.dst, undo: p.undo, byId: new Map(), results: [] };
  for (const g of p.groups) {
    const id = ++takeSeq;
    const msg = { t: 'take', id, src: g.src.n, dst: p.dst.n, levels: g.levels };
    if (p.undo) msg.undo = p.undo.id;
    if (p.retry) msg.retry = p.retry.id;
    if (!wsSend(msg)) {
      showResult('error', 'Not connected', 'Lost connection to Sode before the patch was sent.');
      return false;
    }
    batch.byId.set(id, g);
    takes.set(id, batch);
  }
  const g = p.groups[0];
  showResult('info', p.undo ? 'Undoing…' : 'Patching…', `${g.src.label} → ${p.dst.label} (levels ${levelText(g.levels)})`);
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
    $('cfg-theme-hint').textContent = '⌥⇧T switches between light and dark from anywhere.';
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
