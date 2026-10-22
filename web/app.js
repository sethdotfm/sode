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
  const srcLabel = (n) => (S.srcBy.get(n) || item(n, '')).label;
  const dstLabel = (n) => (S.dstBy.get(n) || item(n, '')).label;

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
      setDot('ws', 'error', 'WebSocket disconnected');
      setDot('rt', 'idle', 'SW-P-08 unknown');
      setTimeout(connect, retry);
      retry = Math.min(retry * 2, 10000);
    };
    ws.onmessage = (e) => handle(JSON.parse(e.data));
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
        renderLevels();
        renderTally();
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

  function accept(k) {
    const b = boxes[k];
    const it = b.matches[b.hi];
    if (!it) return false;
    sel[k] = it;
    b.input.value = it.label;
    b.input.classList.remove('pb-input--invalid');
    b.matches = [];
    renderMenu(k);
    renderTally();
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

  const swatches = (list) => list.map((u) => {
    const s = el('span', `pb-level pb-level--${S.levelBy.get(u).swatch}`);
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
      const rows = runs(sel.dst.n, levels).map((r) =>
        routeRow(r.levels, r.src == null ? 'unknown' : srcLabel(r.src), sel.dst.label));
      return box.replaceChildren(el('div', 'sode__label', 'Currently'), ...rows);
    }
    if (sel.src) {
      const fed = [];
      for (const [d, t] of S.tally) {
        if (levels.some((u) => t.get(u) === sel.src.n)) fed.push(d);
      }
      fed.sort((a, b) => a - b);
      let text = 'Not feeding any destination on these levels';
      if (fed.length) {
        const shown = fed.slice(0, 2).map(dstLabel);
        text = 'Feeding ' + shown.join(', ') + (fed.length > 2 ? `, and ${fed.length - 2} others` : '');
      }
      return box.replaceChildren(el('div', 'sode__label', 'Currently'), el('div', 'sode__route', text));
    }
    box.replaceChildren();
  }

  // ---- results

  function showResult(kind, title, text) {
    $('result').replaceChildren(el('div', `pb-alert pb-alert--${kind}`,
      el('div', null, el('div', 'pb-alert__title', title), text)));
  }

  function takeResult(m) {
    const t = takes.get(m.id);
    takes.delete(m.id);
    const what = t ? `${t.src.label} → ${t.dst.label} (${levelText(t.levels)})` : '';
    if (m.ok) showResult('ok', 'Patched', what);
    else showResult('error', 'Patch failed', `${what}: ${m.err}`);
  }

  // ---- confirm

  const KEYS = 'abcdefghjkmnpqrstuvwxyz';
  let lastKey = '';
  let pending = null;
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
    if (!S.ws || S.router !== 'up') {
      showResult('error', 'Not connected', 'Sode is not connected to the router.');
      return;
    }
    openConfirm(levels);
  }

  function openConfirm(levels) {
    let key;
    do key = KEYS[Math.floor(Math.random() * KEYS.length)]; while (key === lastKey);
    lastKey = key;
    pending = { src: sel.src, dst: sel.dst, levels, key };

    const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
    const before = runs(sel.dst.n, levels).map((r) =>
      routeRow(r.levels, r.src == null ? 'unknown' : srcLabel(r.src), sel.dst.label));
    $('confirm-body').replaceChildren(
      el('div', null, 'Are you sure you want to patch Source ', el('strong', null, sel.src.label),
        ' to Destination ', el('strong', null, sel.dst.label), '?'),
      routeRow(levels, sel.src.label, sel.dst.label),
      el('div', 'sode__label', 'Replacing'),
      ...before,
      el('div', null, 'Press ', keyEl, S.cfg.confirmEnter ? ' or Enter' : '', ' to confirm, or escape to cancel.'),
    );
    $('confirm-ok').textContent = `Patch (${key.toUpperCase()})`;
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
    pending = null;
    $('confirm').hidden = true;
    lvlInput.focus();
  }

  function doTake() {
    const p = pending;
    pending = null;
    $('confirm').hidden = true;
    if (!S.ws || S.ws.readyState !== WebSocket.OPEN) {
      showResult('error', 'Not connected', 'Lost connection to Sode before the patch was sent.');
      return;
    }
    const id = ++takeSeq;
    takes.set(id, p);
    S.ws.send(JSON.stringify({ t: 'take', id, src: p.src.n, dst: p.dst.n, levels: p.levels }));
    showResult('info', 'Patching…', `${p.src.label} → ${p.dst.label} (${levelText(p.levels)})`);
    clearAll();
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
    for (const k of ['src', 'dst']) {
      sel[k] = null;
      boxes[k].input.value = '';
      boxes[k].matches = [];
      boxes[k].input.classList.remove('pb-input--invalid');
      renderMenu(k);
    }
    lvlInput.value = '';
    renderLevels();
    renderTally();
    boxes.src.input.focus();
  }

  for (const k of ['src', 'dst']) {
    const b = boxes[k];
    b.input.addEventListener('input', () => {
      sel[k] = null;
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
    if (e.key === 'Enter') {
      e.preventDefault();
      if (b && !sel[k] && b.input.value.trim()) accept(k);
      tryConfirm();
      return;
    }
    if (e.key === 'Escape') {
      if (b && !b.menu.hidden) { b.matches = []; renderMenu(k); return; }
      if (e.target.value) {
        e.target.value = '';
        if (k) sel[k] = null;
        renderLevels();
        renderTally();
      } else {
        clearAll();
      }
    }
  });

  $('confirm-cancel').addEventListener('click', closeConfirm);
  $('confirm-ok').addEventListener('click', () => {
    if (pending && performance.now() >= readyAt) doTake();
  });

  connect();
})();
