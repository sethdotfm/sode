'use strict';
(() => {
  const $ = (id) => document.getElementById(id);

  // el('div', 'cls', child, 'text', …) — builds DOM without innerHTML so
  // router-supplied names can never inject markup.
  function el(tag, cls, ...kids) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    for (const k of kids) if (k != null) e.append(k);
    return e;
  }

  // ---- state

  const S = {
    cfg: null,
    srcs: [], dsts: [],
    srcBy: new Map(), dstBy: new Map(),
    tally: new Map(), // dst → Map(ui level → src)
    levelBy: new Map(),
    router: null,
    ws: null,
    hist: [],         // newest first
    histMore: false,
    histLoading: false,
    feedOpen: false,  // "and N others" expanded
  };
  const sel = { src: null, dst: null };
  const boxes = {
    src: { input: $('src'), menu: $('src-menu'), list: () => S.srcs, matches: [], hi: 0 },
    dst: { input: $('dst'), menu: $('dst-menu'), list: () => S.dsts, matches: [], hi: 0 },
  };
  const lvlInput = $('lvl');

  const off = () => (S.cfg ? S.cfg.numOffset : 0);

  function item(n, name) {
    const num = String(n + off());
    return { n, name, num, lname: name.toLowerCase(), label: name ? `${num} ${name}` : num };
  }
  const srcItem = (n, name = '') => S.srcBy.get(n) || item(n, name);
  const dstItem = (n, name = '') => S.dstBy.get(n) || item(n, name);
  const srcLabel = (n) => srcItem(n).label;
  const dstLabel = (n) => dstItem(n).label;
  // Tally source text: null = never reported, -1 = router says nothing routed.
  const tallyText = (s) => (s == null ? 'unknown' : s < 0 ? 'nothing' : srcLabel(s));

  // ---- websocket

  let retry = 1000;
  let takeSeq = 0;
  const takes = new Map();

  function connect() {
    const base = location.pathname.replace(/[^/]*$/, '');
    const ws = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + base + 'ws');
    S.ws = ws;
    setDot('ws', 'warn', 'WebSocket connecting');
    ws.onopen = () => { retry = 1000; setDot('ws', 'ok', 'WebSocket connected'); };
    ws.onclose = () => {
      S.ws = null;
      S.histLoading = false;
      setDot('ws', 'error', 'WebSocket disconnected');
      setDot('rt', 'idle', 'SW-P-08 unknown');
      setTimeout(connect, retry);
      retry = Math.min(retry * 2, 10000);
    };
    ws.onmessage = (e) => handle(JSON.parse(e.data));
  }

  function wsSend(m) {
    if (!S.ws || S.ws.readyState !== WebSocket.OPEN) return false;
    S.ws.send(JSON.stringify(m));
    return true;
  }

  function handle(m) {
    switch (m.t) {
      case 'hello':
        S.cfg = m.cfg;
        S.levelBy = new Map(m.cfg.levels.map((l) => [l.ui, l]));
        lvlInput.placeholder = m.cfg.defaultLevels;
        setNames(m.srcs, m.dsts);
        S.tally = new Map();
        applyTally(m.tally || []);
        setStatus(m.status);
        S.hist = m.recent || [];
        S.histMore = m.more;
        renderLevels();
        renderTally();
        renderHistory();
        break;
      case 'names':
        setNames(m.srcs, m.dsts);
        for (const k of ['src', 'dst']) if (document.activeElement === boxes[k].input && !sel[k]) refresh(k);
        renderTally();
        break;
      case 'tally':
        applyTally(m.u);
        renderTally();
        break;
      case 'status':
        setStatus(m);
        break;
      case 'takeResult':
        takeResult(m);
        break;
      case 'recent':
        if (!S.hist.some((e) => e.id === m.e.id)) S.hist.unshift(m.e);
        renderHistory();
        break;
      case 'history': {
        const last = S.hist.length ? S.hist[S.hist.length - 1].id : Infinity;
        S.hist.push(...m.entries.filter((e) => e.id < last));
        S.histMore = m.more;
        S.histLoading = false;
        renderHistory();
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

  function setDot(which, state, text) {
    $(`${which}-dot`).className = `pb-dot pb-dot--${state}`;
    $(`${which}-text`).textContent = text;
  }

  function setStatus(st) {
    S.router = st.router;
    $('rt-name').textContent = `Router ${st.addr}`;
    const map = { up: ['ok', 'connected'], connecting: ['warn', 'connecting'], down: ['error', 'down'] };
    const [dot, word] = map[st.router] || ['idle', st.router];
    setDot('rt', dot, `SW-P-08 ${word}`);
    $('rt-text').parentElement.title = st.err || '';
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

  function search(list, q) {
    const out = [];
    for (const it of list) {
      const s = score(it, q);
      if (s >= 0) out.push([s, it]);
    }
    out.sort((a, b) => b[0] - a[0] || a[1].n - b[1].n);
    return out.slice(0, 8).map((x) => x[1]);
  }

  function refresh(k) {
    const b = boxes[k];
    const q = b.input.value.trim().toLowerCase();
    b.matches = q ? search(b.list(), q) : [];
    b.hi = 0;
    renderMenu(k);
  }

  function renderMenu(k) {
    const b = boxes[k];
    const open = b.matches.length > 0 && !sel[k];
    b.menu.hidden = !open;
    b.input.setAttribute('aria-expanded', String(open));
    if (!open) { b.input.removeAttribute('aria-activedescendant'); return; }
    b.menu.replaceChildren(...b.matches.map((it, i) => {
      const li = el('li', 'pb-menu__item' + (i === b.hi ? ' active' : ''), el('span', 'sode__num', it.num), it.name);
      li.id = `${k}-opt-${i}`;
      li.setAttribute('role', 'option');
      li.setAttribute('aria-selected', String(i === b.hi));
      li.addEventListener('mousedown', (e) => { e.preventDefault(); b.hi = i; accept(k); });
      return li;
    }));
    b.input.setAttribute('aria-activedescendant', `${k}-opt-${b.hi}`);
  }

  function select(k, it) {
    sel[k] = it;
    if (k === 'src') S.feedOpen = false;
    const b = boxes[k];
    b.input.value = it ? it.label : '';
    b.input.classList.remove('pb-input--invalid');
    b.matches = [];
    renderMenu(k);
    renderTally();
  }

  function accept(k) {
    const it = boxes[k].matches[boxes[k].hi];
    if (!it) return false;
    select(k, it);
    return true;
  }

  // ---- levels

  function parseLevels(text) {
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

  // "Video, A1–A16" for a sorted list of UI levels.
  function levelText(list) {
    const parts = [];
    for (let i = 0; i < list.length;) {
      let j = i;
      while (j + 1 < list.length && list[j + 1] === list[j] + 1) j++;
      const a = S.levelBy.get(list[i]).label, b = S.levelBy.get(list[j]).label;
      parts.push(i === j ? a : `${a}–${b}`);
      i = j + 1;
    }
    return parts.join(', ');
  }

  // Numbered colour swatch per level. The number repeats the level so the
  // swatch never relies on colour alone.
  const swatches = (list) => list.map((u) => {
    const s = el('span', `pb-level pb-level--${S.levelBy.get(u).swatch} sode__swatch`, String(u));
    s.setAttribute('aria-hidden', 'true');
    return s;
  });

  function renderLevels() {
    if (!S.cfg) return;
    const list = parseLevels(lvlInput.value);
    lvlInput.classList.toggle('pb-input--invalid', !list);
    const sum = $('lvl-sum');
    if (!list) { sum.replaceChildren('Unknown level'); return; }
    sum.replaceChildren(...swatches(list), el('span', 'sode__levels-text', levelText(list)));
  }

  // ---- tally display

  // Group levels that share a source: [{levels:[…], src}]
  function runs(dst, levels) {
    const t = S.tally.get(dst) || new Map();
    const out = [];
    for (const u of levels) {
      const s = t.has(u) ? t.get(u) : null;
      const last = out[out.length - 1];
      if (last && last.src === s) last.levels.push(u);
      else out.push({ levels: [u], src: s });
    }
    return out;
  }

  function routeRow(levels, srcText, dstText) {
    return el('div', 'sode__route',
      el('span', 'sode__route-levels', ...swatches(levels), el('span', null, levelText(levels))),
      el('span', 'sode__end sode__end--src', srcText),
      el('span', 'sode__arrow', '→'),
      el('span', 'sode__end sode__end--dst', dstText));
  }

  function renderTally() {
    const box = $('tally');
    if (!S.cfg) return box.replaceChildren();
    const levels = parseLevels(lvlInput.value) || S.cfg.levels.map((l) => l.ui);
    if (sel.dst) {
      const rows = runs(sel.dst.n, levels).map((r) => routeRow(r.levels, tallyText(r.src), sel.dst.label));
      return box.replaceChildren(el('div', 'sode__label', 'Currently'), ...rows);
    }
    if (sel.src) {
      const fed = [];
      for (const [d, t] of S.tally) {
        if (levels.some((u) => t.get(u) === sel.src.n)) fed.push(d);
      }
      fed.sort((a, b) => a - b);
      return box.replaceChildren(el('div', 'sode__label', 'Currently'), feedingRow(fed));
    }
    box.replaceChildren();
  }

  function feedingRow(fed) {
    if (!fed.length) return el('div', 'sode__route', 'Not feeding any destination on these levels');
    if (S.feedOpen || fed.length <= 2) {
      const list = el('div', 'sode__chips', ...fed.map((d) => {
        const chip = el('button', 'pb-chip', dstLabel(d));
        chip.type = 'button';
        chip.title = 'Use as destination';
        chip.addEventListener('click', () => { select('dst', dstItem(d)); lvlInput.focus(); });
        return chip;
      }));
      const row = el('div', 'sode__feed', el('span', null, `Feeding ${fed.length} destination${fed.length === 1 ? '' : 's'}`), list);
      if (fed.length > 2) {
        const less = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Show less');
        less.type = 'button';
        less.setAttribute('aria-expanded', 'true');
        less.addEventListener('click', () => { S.feedOpen = false; renderTally(); });
        row.append(el('div', null, less));
      }
      return row;
    }
    const more = el('button', 'pb-btn pb-btn--ghost sode__more', `and ${fed.length - 2} others`);
    more.type = 'button';
    more.setAttribute('aria-expanded', 'false');
    more.addEventListener('click', () => { S.feedOpen = true; renderTally(); });
    return el('div', 'sode__route', `Feeding ${fed.slice(0, 2).map(dstLabel).join(', ')},`, more);
  }

  // ---- history

  const fmtTime = (iso) => {
    const d = new Date(iso);
    const t = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
    return d.toDateString() === new Date().toDateString() ? t : `${d.toLocaleDateString()} ${t}`;
  };

  // Groups an entry's previous routing into restorable takes.
  function undoPlan(e) {
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

  function histRow(e) {
    const src = srcItem(e.src, e.srcName), dst = dstItem(e.dst, e.dstName);
    const known = e.levels.filter((u) => S.levelBy.has(u));
    const status = el('span', 'pb-status',
      el('span', `pb-dot pb-dot--${e.ok ? 'ok' : 'error'}`),
      e.ok ? 'ok' : 'failed');
    status.firstChild.setAttribute('aria-hidden', 'true');
    if (!e.ok) status.title = e.err || '';

    const plan = undoPlan(e);
    const btn = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Undo');
    btn.type = 'button';
    btn.setAttribute('aria-label', `Undo ${src.label} to ${dst.label}`);
    if (!plan.groups.length) {
      btn.disabled = true;
      btn.title = 'Nothing to restore';
    }
    btn.addEventListener('click', () => openConfirm(plan));

    return el('li', 'pb-input sode__hist-row',
      el('span', 'sode__hist-time', fmtTime(e.time)),
      el('span', 'sode__route-levels', ...swatches(known), el('span', null, levelText(known))),
      el('span', 'sode__hist-route', el('strong', null, src.label), ' → ', el('strong', null, dst.label),
        e.undo != null ? el('span', 'sode__hist-note', ` (undo of #${e.undo})`) : null),
      status,
      btn);
  }

  function renderHistory() {
    $('history-head').hidden = !S.hist.length;
    $('history').replaceChildren(...S.hist.map(histRow));
    $('history-more').textContent = S.histLoading ? 'Loading…' : '';
    maybeLoadMore();
  }

  function maybeLoadMore() {
    if (!S.histMore || S.histLoading || !S.hist.length) return;
    const r = $('history-more').getBoundingClientRect();
    if (r.top > window.innerHeight + 200) return;
    S.histLoading = wsSend({ t: 'history', before: S.hist[S.hist.length - 1].id });
    if (S.histLoading) $('history-more').textContent = 'Loading…';
  }

  new IntersectionObserver(maybeLoadMore, { rootMargin: '200px' }).observe($('history-more'));

  // ---- results

  function showResult(kind, title, text) {
    $('result').replaceChildren(el('div', `pb-alert pb-alert--${kind}`,
      el('div', null, el('div', 'pb-alert__title', title), text)));
  }

  function takeResult(m) {
    const t = takes.get(m.id);
    takes.delete(m.id);
    const what = t ? `${t.src.label} → ${t.dst.label} (${levelText(t.levels)})` : '';
    if (m.ok) showResult('ok', t && t.undo ? 'Undone' : 'Patched', what);
    else showResult('error', 'Patch failed', `${what}: ${m.err}`);
  }

  // ---- confirm

  const KEYS = 'abcdefghjkmnpqrstuvwxyz';
  let lastKey = '';
  let pending = null; // { dst, groups: [{src, levels}], skipped, undo, key }
  let readyAt = 0;

  function tryConfirm() {
    if (!S.cfg) return;
    for (const k of ['src', 'dst']) {
      if (!sel[k]) {
        boxes[k].input.classList.add('pb-input--invalid');
        boxes[k].input.focus();
        return;
      }
    }
    const levels = parseLevels(lvlInput.value);
    if (!levels) { lvlInput.focus(); return; }
    openConfirm({ dst: sel.dst, groups: [{ src: sel.src, levels }], skipped: [], undo: null });
  }

  function openConfirm(plan) {
    if (!S.ws || S.router !== 'up') {
      showResult('error', 'Not connected', 'Sode is not connected to the router.');
      return;
    }
    let key;
    do key = KEYS[Math.floor(Math.random() * KEYS.length)]; while (key === lastKey);
    lastKey = key;
    pending = { ...plan, key };

    const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
    const all = plan.groups.flatMap((g) => g.levels).sort((a, b) => a - b);
    const before = runs(plan.dst.n, all).map((r) => routeRow(r.levels, tallyText(r.src), plan.dst.label));
    const after = plan.groups.map((g) => routeRow(g.levels, g.src.label, plan.dst.label));

    const question = plan.undo
      ? el('div', null, 'Are you sure you want to undo the patch of ', el('strong', null, srcItem(plan.undo.src, plan.undo.srcName).label),
        ' to Destination ', el('strong', null, plan.dst.label), '?')
      : el('div', null, 'Are you sure you want to patch Source ', el('strong', null, plan.groups[0].src.label),
        ' to Destination ', el('strong', null, plan.dst.label), '?');

    $('confirm-title').textContent = plan.undo ? 'Confirm undo' : 'Confirm patch';
    $('confirm-body').replaceChildren(...[
      question,
      ...after,
      plan.skipped.length
        ? el('div', 'sode__hist-note', `${levelText(plan.skipped)}: nothing was routed before, left as is.`)
        : null,
      el('div', 'sode__label', 'Replacing'),
      ...before,
      el('div', 'sode__prompt', 'Press ', keyEl, S.cfg.confirmEnter ? ' or Enter' : '', ' to confirm, or escape to cancel.'),
    ].filter(Boolean));
    $('confirm-ok').textContent = `${plan.undo ? 'Undo' : 'Patch'} (${key.toUpperCase()})`;
    $('confirm-ok').disabled = true;
    $('confirm').hidden = false;
    $('confirm-dialog').focus();

    readyAt = performance.now() + S.cfg.lockoutMs;
    setTimeout(() => {
      if (pending && pending.key === key) {
        keyEl.classList.remove('sode__key--locked');
        $('confirm-ok').disabled = false;
      }
    }, S.cfg.lockoutMs);
  }

  function closeConfirm() {
    const wasUndo = pending && pending.undo;
    pending = null;
    $('confirm').hidden = true;
    if (!wasUndo) lvlInput.focus();
  }

  function doTake() {
    const p = pending;
    pending = null;
    $('confirm').hidden = true;
    for (const g of p.groups) {
      const id = ++takeSeq;
      const msg = { t: 'take', id, src: g.src.n, dst: p.dst.n, levels: g.levels };
      if (p.undo) msg.undo = p.undo.id;
      if (!wsSend(msg)) {
        showResult('error', 'Not connected', 'Lost connection to Sode before the patch was sent.');
        return;
      }
      takes.set(id, { src: g.src, dst: p.dst, levels: g.levels, undo: p.undo });
    }
    const g = p.groups[0];
    showResult('info', p.undo ? 'Undoing…' : 'Patching…', `${g.src.label} → ${p.dst.label} (${levelText(g.levels)})`);
    if (!p.undo) clearAll();
  }

  function onConfirmKey(e) {
    e.preventDefault();
    e.stopPropagation();
    if (e.key === 'Escape') return closeConfirm();
    if (e.repeat || e.ctrlKey || e.metaKey || e.altKey) return;
    if (performance.now() < readyAt) return;
    if (e.key.toLowerCase() === pending.key || (S.cfg.confirmEnter && e.key === 'Enter')) doTake();
  }

  // ---- keyboard & input wiring

  function clearAll() {
    for (const k of ['src', 'dst']) select(k, null);
    lvlInput.value = '';
    renderLevels();
    renderTally();
    boxes.src.input.focus();
  }

  for (const k of ['src', 'dst']) {
    const b = boxes[k];
    b.input.addEventListener('input', () => {
      sel[k] = null;
      if (k === 'src') S.feedOpen = false;
      b.input.classList.remove('pb-input--invalid');
      refresh(k);
      renderTally();
    });
    b.input.addEventListener('blur', () => { b.matches = []; renderMenu(k); });
  }
  for (const input of [boxes.src.input, boxes.dst.input, lvlInput]) {
    input.addEventListener('focus', () => input.select());
  }
  lvlInput.addEventListener('input', () => { renderLevels(); renderTally(); });

  document.addEventListener('keydown', (e) => {
    if (pending) return onConfirmKey(e);
    const id = e.target.id;
    const k = id === 'src' || id === 'dst' ? id : null;
    const b = k && boxes[k];
    const inBox = k || id === 'lvl';

    if (b && !b.menu.hidden && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      e.preventDefault();
      const n = b.matches.length;
      b.hi = (b.hi + (e.key === 'ArrowDown' ? 1 : n - 1)) % n;
      renderMenu(k);
      return;
    }
    if (e.key === 'Tab' && !e.shiftKey) {
      if (b && !sel[k] && b.input.value.trim() && !accept(k)) {
        e.preventDefault();
        b.input.classList.add('pb-input--invalid');
        return;
      }
      if (id === 'lvl') { e.preventDefault(); boxes.src.input.focus(); }
      return;
    }
    if (!inBox) return; // leave buttons and the history list to the browser
    if (e.key === 'Enter') {
      e.preventDefault();
      if (b && !sel[k] && b.input.value.trim()) accept(k);
      tryConfirm();
      return;
    }
    if (e.key === 'Escape') {
      if (b && !b.menu.hidden) { b.matches = []; renderMenu(k); return; }
      if (e.target.value) {
        if (k) select(k, null);
        else e.target.value = '';
        renderLevels();
        renderTally();
      } else {
        clearAll();
      }
    }
  });

  // ---- theme

  // Follows the OS until toggled; the choice is remembered per browser.
  const root = document.documentElement;
  const osDark = matchMedia('(prefers-color-scheme: dark)');
  const isDark = () => (root.dataset.theme ? root.dataset.theme === 'dark' : osDark.matches);
  const themeBtns = document.querySelectorAll('[data-theme-set]');
  const syncTheme = () => {
    const dark = isDark();
    for (const b of themeBtns) b.setAttribute('aria-pressed', String((b.dataset.themeSet === 'dark') === dark));
  };

  function setTheme(next) {
    if (next === (isDark() ? 'dark' : 'light')) return;
    const ms = parseFloat(getComputedStyle(root).getPropertyValue('--pb-motion-theme')) || 0;
    root.classList.add('pb-theme-transition');
    root.dataset.theme = next;
    setTimeout(() => root.classList.remove('pb-theme-transition'), ms);
    try { localStorage.setItem('sode-theme', next); } catch (e) {}
    syncTheme();
  }

  // Alt/Option+Shift+T: browsers keep Mod+Shift+T for reopening tabs.
  // Matched by e.code because Option changes e.key on macOS.
  const mac = /Mac|iPhone|iPad/.test(navigator.platform);
  const shortcut = mac ? '⌥⇧T' : 'Alt+Shift+T';
  for (const b of themeBtns) {
    const name = b.dataset.themeSet === 'dark' ? 'Dark theme' : 'Light theme';
    b.dataset.tip = `${name} (${shortcut})`;
    b.addEventListener('click', () => setTheme(b.dataset.themeSet));
  }
  document.addEventListener('keydown', (e) => {
    if (e.code !== 'KeyT' || !e.shiftKey || !e.altKey || e.ctrlKey || e.metaKey || e.repeat) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    setTheme(isDark() ? 'light' : 'dark');
  }, true);
  osDark.addEventListener('change', syncTheme);
  syncTheme();

  $('confirm-cancel').addEventListener('click', closeConfirm);
  $('confirm-ok').addEventListener('click', () => {
    if (pending && performance.now() >= readyAt) doTake();
  });

  connect();
})();
