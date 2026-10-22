// Phone layout: a text-message style composer. One input at a time steps
// Source → Destination → Levels; the keyboard's Next/Go advances, and a
// bottom sheet with a slider confirms.

import {
  $, el, S, on, connState, setDot, search, parseLevels, levelInput, levelBar, pickButtons, allLevels,
  fedBy, dstItem, dstLabel, srcItem, currentlyNodes, fmtTime, undoPlan, retryPlan, resultNode, showResult,
  patchPlan, planBlocked, planSummary, sendTakes,
} from './core.js';
import { layout, onLayout } from './layout.js';

const STEPS = ['src', 'dst', 'lvl'];
const PROMPT = { src: 'Source', dst: 'Destination', lvl: 'Levels' };
const inputs = { src: $('m-src'), dst: $('m-dst'), lvl: $('m-lvl') };
const M = {
  step: 'src',
  sel: { src: null, dst: null },
  matches: [],
  hi: 0,
  lvlNone: false,
  feedOpen: false,
};
const active = () => layout() === 'phone' && document.documentElement.dataset.view !== 'settings';

// ---- header

function renderConn() {
  const c = connState();
  setDot($('m-ws-dot'), $('m-ws-text'), [c.ws[0], 'Socket']);
  $('m-ws-text').parentElement.title = c.ws[1];
  setDot($('m-rt-dot'), $('m-rt-text'), [c.rt[0], 'Router']);
  $('m-rt-text').parentElement.title = c.rt[1];
}

// ---- composer

function setStep(step, focus = true) {
  M.step = step;
  for (const k of STEPS) inputs[k].hidden = k !== step;
  $('m-step').textContent = PROMPT[step];
  $('m-step').htmlFor = inputs[step].id;
  $('m-go').textContent = step === 'lvl' ? 'Go' : 'Next';
  $('m-picker').hidden = step !== 'lvl';
  M.matches = [];
  if (step !== 'lvl') refresh();
  renderMatches();
  renderLevels();
  if (focus) inputs[step].focus();
  renderThread();
}

function refresh() {
  if (M.step === 'lvl' || M.sel[M.step]) {
    M.matches = [];
  } else {
    M.matches = search(M.step === 'src' ? S.srcs : S.dsts, inputs[M.step].value, 5);
  }
  M.hi = 0;
  renderMatches();
}

function renderMatches() {
  const ul = $('m-matches');
  ul.hidden = !M.matches.length;
  // The list is column-reverse: the best match sits nearest the input, and
  // if space runs short the worst ones scroll away first.
  ul.replaceChildren(...M.matches.map((it, i) => {
    const li = el('li', 'sode-m__match' + (i === M.hi ? ' sode-m__match--hi' : ''), el('span', 'sode__num', it.num), it.name);
    li.setAttribute('role', 'option');
    li.setAttribute('aria-selected', String(i === M.hi));
    li.addEventListener('pointerdown', (e) => e.preventDefault()); // keep the keyboard up
    li.addEventListener('click', () => { M.hi = i; advance(); });
    return li;
  }));
}

function shake(node) {
  node.classList.remove('sode-m__shake');
  void node.offsetWidth;
  node.classList.add('sode-m__shake');
}

// Next/Go: accept the step and move on. Focus moves within the same
// handler so the phone keeps its keyboard open.
function advance() {
  const step = M.step;
  if (step !== 'lvl') {
    if (!M.sel[step]) {
      const it = M.matches[M.hi];
      if (!it) return shake(inputs[step]);
      choose(step, it);
    }
    const next = step === 'src' && !M.sel.dst ? 'dst' : 'lvl';
    return setStep(next);
  }
  if (!M.sel.src) return setStep('src');
  if (!M.sel.dst) return setStep('dst');
  const levels = parseLevels(inputs.lvl.value, M.lvlNone);
  if (!levels) return shake(inputs.lvl);
  inputs.lvl.blur();
  openSheet(patchPlan(M.sel.src, M.sel.dst, levels));
}

function choose(k, it) {
  M.sel[k] = it;
  if (k === 'src') M.feedOpen = false;
  inputs[k].value = '';
  M.matches = [];
  renderMatches();
  renderThread();
}

function reset() {
  M.sel = { src: null, dst: null };
  M.lvlNone = false;
  for (const k of STEPS) inputs[k].value = '';
  setStep('src', false);
}

// ---- levels

function renderLevels() {
  if (!S.cfg) return;
  const list = parseLevels(inputs.lvl.value, M.lvlNone);
  const on = new Set(list || []);
  inputs.lvl.placeholder = M.lvlNone ? 'none' : `${S.cfg.defaultLevels} (default)`;
  inputs.lvl.classList.toggle('pb-input--invalid', !list);
  for (const b of $('m-picks').children) b.setAttribute('aria-pressed', String(on.has(+b.dataset.ui)));
  $('m-all').textContent = on.size === S.levels.length ? 'None' : 'All';
}

function setLevels(list) {
  M.lvlNone = list.length === 0;
  inputs.lvl.value = levelInput(list);
  renderLevels();
}

function buildPicker() {
  $('m-picks').replaceChildren(...pickButtons((ui) => {
    const on = new Set(parseLevels(inputs.lvl.value, M.lvlNone) || []);
    if (on.has(ui)) on.delete(ui);
    else on.add(ui);
    setLevels([...on].sort((x, y) => x - y));
  }));
}

// ---- thread: recent, result, bubbles, currently

function bubble(k) {
  const it = M.sel[k];
  const b = el('button', 'sode-m__bubble' + (M.step === k ? ' sode-m__bubble--editing' : ''),
    el('span', 'sode-m__bubble-label', PROMPT[k]), it.label);
  b.type = 'button';
  b.setAttribute('aria-label', `${PROMPT[k]} ${it.label}, tap to change`);
  b.addEventListener('pointerdown', (e) => e.preventDefault());
  b.addEventListener('click', () => {
    M.sel[k] = null;
    inputs[k].value = it.label.startsWith(it.num + ' ') ? it.name : it.label;
    setStep(k);
    inputs[k].select();
    refresh();
  });
  return b;
}

function feeding() {
  const fed = fedBy(M.sel.src.n, allLevels());
  if (!fed.length) return el('div', 'sode__hint', 'Not feeding any destination');
  const chip = (d) => {
    const c = el('button', 'pb-chip', dstLabel(d));
    c.type = 'button';
    c.addEventListener('pointerdown', (e) => e.preventDefault());
    c.addEventListener('click', () => { choose('dst', dstItem(d)); setStep('lvl'); });
    return c;
  };
  const shown = M.feedOpen ? fed : fed.slice(0, 2);
  const out = el('div', 'sode__feed', el('span', 'sode__label', `Feeding ${fed.length}`), el('div', 'sode__chips', ...shown.map(chip)));
  if (fed.length > 2) {
    const more = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', M.feedOpen ? 'Show less' : `and ${fed.length - 2} others`);
    more.type = 'button';
    more.setAttribute('aria-expanded', String(M.feedOpen));
    more.addEventListener('pointerdown', (e) => e.preventDefault());
    more.addEventListener('click', () => { M.feedOpen = !M.feedOpen; renderThread(); });
    out.append(more);
  }
  return out;
}

function renderThread() {
  if (!S.cfg) return;
  const nodes = [];
  if (M.sel.src) nodes.push(bubble('src'));
  if (M.sel.dst) nodes.push(bubble('dst'));
  if (M.sel.dst) nodes.push(el('div', 'sode-m__card', ...currentlyNodes(M.sel.dst)));
  else if (M.sel.src) nodes.push(el('div', 'sode-m__card', feeding()));
  $('m-compose').replaceChildren(...nodes);
  scrollThread();
}

// After layout settles (the composer may just have grown or shrunk), keep
// the newest content in view at the bottom of the thread.
function scrollThread() {
  requestAnimationFrame(() => {
    const t = $('m-thread');
    t.scrollTop = t.scrollHeight;
  });
}

// ---- recent: swipe left to reveal Undo

const REVEAL = 96;

function recentRow(e) {
  const src = srcItem(e.src, e.srcName), dst = dstItem(e.dst, e.dstName);
  const plan = undoPlan(e);
  const undo = el('button', 'sode-m__undo', 'Undo');
  undo.type = 'button';
  undo.disabled = !plan.groups.length;
  undo.setAttribute('aria-label', `Undo ${src.label} to ${dst.label}`);
  undo.addEventListener('click', () => { closeSwipes(); openSheet(plan); });

  const status = el('span', 'pb-status', el('span', `pb-dot pb-dot--${e.ok ? 'ok' : 'error'}`), e.ok ? 'ok' : 'failed');
  status.firstChild.setAttribute('aria-hidden', 'true');
  let retry = null;
  if (!e.ok) {
    retry = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Retry');
    retry.type = 'button';
    retry.setAttribute('aria-label', `Retry ${src.label} to ${dst.label}`);
    retry.addEventListener('click', () => { closeSwipes(); openSheet(retryPlan(e)); });
  }
  const face = el('div', 'sode-m__recent-face',
    el('div', 'sode-m__recent-top', el('span', 'sode__hist-time', fmtTime(e.time)),
      el('span', 'sode-m__recent-status', status, retry)),
    el('div', 'sode-m__recent-route', el('strong', null, src.label), ' → ', el('strong', null, dst.label)),
    levelBar(e.levels.filter((u) => S.levelBy.has(u))));
  const row = el('li', 'sode-m__recent-row', undo, face);
  swipeable(row, face);
  return row;
}

function swipeable(row, face) {
  let x0 = null, y0 = 0, dx = 0, base = 0, horiz = null;
  const set = (x) => { face.style.transform = `translateX(${x}px)`; };
  face.addEventListener('pointerdown', (e) => {
    x0 = e.clientX; y0 = e.clientY; dx = 0; horiz = null;
    base = row.classList.contains('sode-m__recent-row--open') ? -REVEAL : 0;
  });
  face.addEventListener('pointermove', (e) => {
    if (x0 == null) return;
    const mx = e.clientX - x0, my = e.clientY - y0;
    if (horiz == null && Math.abs(mx) + Math.abs(my) > 8) {
      horiz = Math.abs(mx) > Math.abs(my);
      if (horiz) face.setPointerCapture(e.pointerId);
    }
    if (!horiz) return;
    dx = mx;
    face.classList.add('sode-m__recent-face--drag');
    set(Math.min(0, Math.max(-REVEAL * 1.3, base + dx)));
  });
  const end = () => {
    if (x0 == null) return;
    face.classList.remove('sode-m__recent-face--drag');
    const open = horiz ? base + dx < -REVEAL / 2 : false;
    if (!horiz && base) { closeSwipes(); x0 = null; return; } // tap on an open row closes it
    if (horiz) {
      closeSwipes(row);
      row.classList.toggle('sode-m__recent-row--open', open);
    }
    set(row.classList.contains('sode-m__recent-row--open') ? -REVEAL : 0);
    x0 = null;
  };
  face.addEventListener('pointerup', end);
  face.addEventListener('pointercancel', end);
}

function closeSwipes(except) {
  for (const r of document.querySelectorAll('.sode-m__recent-row--open')) {
    if (r === except) continue;
    r.classList.remove('sode-m__recent-row--open');
    r.querySelector('.sode-m__recent-face').style.transform = '';
  }
}

function renderRecent() {
  const list = S.hist.slice(0, 5);
  $('m-recent').hidden = !list.length;
  $('m-recent-list').replaceChildren(...list.map(recentRow));
  scrollThread();
}

// ---- confirm sheet with slider

let pending = null;
let readyAt = 0;

function openSheet(plan) {
  const blocked = planBlocked(plan);
  if (blocked) return showResult(blocked.kind, blocked.title, blocked.text);
  pending = plan;
  $('m-sheet-title').textContent = plan.undo ? 'Confirm undo' : 'Confirm patch';
  $('m-sheet-body').replaceChildren(...planSummary(plan));
  $('m-slide-label').textContent = plan.undo ? 'Slide to undo' : 'Slide to patch';
  resetSlide(true);
  $('m-sheet').hidden = false;
  readyAt = performance.now() + S.cfg.confirmLockoutMs;
  setTimeout(() => {
    if (pending === plan) $('m-slide').classList.remove('sode__slide--locked');
  }, S.cfg.confirmLockoutMs);
}

function closeSheet() {
  pending = null;
  $('m-sheet').hidden = true;
}

function confirmSheet() {
  const p = pending;
  closeSheet();
  if (sendTakes(p) && !p.undo) reset();
}

const handle = $('m-slide-handle');
const track = $('m-slide');
let drag = null;

const maxX = () => track.clientWidth - handle.offsetWidth;

function setSlide(x) {
  const pct = Math.round((x / maxX()) * 100);
  handle.style.transform = `translateX(${x}px)`;
  track.style.setProperty('--slide', `${x + handle.offsetWidth}px`);
  handle.setAttribute('aria-valuenow', String(pct));
}

function resetSlide(locked) {
  track.classList.toggle('sode__slide--locked', locked);
  track.classList.remove('sode__slide--drag');
  setSlide(0);
}

handle.addEventListener('pointerdown', (e) => {
  if (!pending || performance.now() < readyAt) return;
  drag = { x0: e.clientX };
  handle.setPointerCapture(e.pointerId);
  track.classList.add('sode__slide--drag');
});
handle.addEventListener('pointermove', (e) => {
  if (!drag) return;
  setSlide(Math.max(0, Math.min(maxX(), e.clientX - drag.x0)));
});
const release = (e) => {
  if (!drag) return;
  const x = Math.max(0, Math.min(maxX(), e.clientX - drag.x0));
  drag = null;
  track.classList.remove('sode__slide--drag');
  if (x >= maxX() * 0.9) {
    setSlide(maxX());
    confirmSheet();
  } else {
    setSlide(0);
  }
};
handle.addEventListener('pointerup', release);
handle.addEventListener('pointercancel', release);
// Keyboard fallback for the slider: → or End completes it.
handle.addEventListener('keydown', (e) => {
  if (!pending || performance.now() < readyAt) return;
  if (e.key === 'ArrowRight' || e.key === 'End') {
    e.preventDefault();
    setSlide(maxX());
    confirmSheet();
  }
});
$('m-sheet-cancel').addEventListener('click', closeSheet);
$('m-sheet').addEventListener('click', (e) => { if (e.target === $('m-sheet')) closeSheet(); });

// ---- inputs

for (const k of ['src', 'dst']) {
  inputs[k].addEventListener('input', () => {
    M.sel[k] = null;
    refresh();
  });
}
inputs.lvl.addEventListener('input', () => { M.lvlNone = false; renderLevels(); });
for (const k of STEPS) {
  inputs[k].addEventListener('keydown', (e) => {
    if (!active()) return;
    if (e.key === 'Enter') {
      e.preventDefault();
      advance();
    } else if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
      if (!M.matches.length) return;
      e.preventDefault();
      const n = M.matches.length;
      // The list is drawn bottom-up, so Up moves to the next-best match.
      M.hi = (M.hi + (e.key === 'ArrowUp' ? 1 : n - 1)) % n;
      renderMatches();
    } else if (e.key === 'Backspace' && !inputs[k].value && k !== 'src') {
      // Backspace on an empty step goes back to edit the previous one.
      e.preventDefault();
      const prev = k === 'lvl' ? 'dst' : 'src';
      const it = M.sel[prev];
      M.sel[prev] = null;
      inputs[prev].value = it ? it.name || it.num : '';
      setStep(prev);
      refresh();
    }
  });
}
$('m-go').addEventListener('pointerdown', (e) => e.preventDefault()); // keep the keyboard up
$('m-go').addEventListener('click', advance);
// Cancel: clear the whole entry and let the keyboard go.
$('m-cancel').addEventListener('click', () => {
  reset();
  document.activeElement?.blur();
});
for (const id of ['m-picks', 'm-all']) $(id).addEventListener('pointerdown', (e) => e.preventDefault());
$('m-all').addEventListener('click', () => {
  const full = (parseLevels(inputs.lvl.value, M.lvlNone) || []).length === S.levels.length;
  setLevels(full ? [] : allLevels());
});

// ---- typing: keyboard up means Recent steps aside

// While a composer input has focus the keyboard is up and room is short,
// so Recent hides and the bubbles, levels and Currently get the space.
// It returns when the keyboard goes away. Focus moving between steps
// happens synchronously, so check after the blur settles.
const view = $('mobile-view');
function syncTyping() {
  const typing = STEPS.some((k) => document.activeElement === inputs[k]);
  if (view.classList.contains('sode-m--typing') === typing) return;
  view.classList.toggle('sode-m--typing', typing);
  scrollThread();
}
for (const k of STEPS) {
  inputs[k].addEventListener('focus', syncTyping);
  inputs[k].addEventListener('blur', () => setTimeout(syncTyping, 0));
}

// ---- keyboard overlap (iOS)

// iOS Safari doesn't shrink the layout viewport for the keyboard, so size
// the phone view to the visual viewport; the composer then sits on top of
// the keyboard. Android Chrome resizes for us (interactive-widget).
const vv = window.visualViewport;
if (vv) {
  const fit = () => {
    const r = document.documentElement.style;
    r.setProperty('--vvh', `${vv.height}px`);
    r.setProperty('--vvt', `${vv.offsetTop}px`);
  };
  vv.addEventListener('resize', fit);
  vv.addEventListener('scroll', fit);
  fit();
}

// ---- subscriptions

on('hello', () => {
  buildPicker();
  setStep(M.step, false);
  renderRecent();
});
on('names', () => { refresh(); renderThread(); });
on('tally', renderThread);
on('conn', renderConn);
on('history', renderRecent);
on('result', (r) => {
  $('m-result').replaceChildren(resultNode(r));
  scrollThread();
});
onLayout((l) => {
  if (l !== 'phone') closeSheet();
  else { renderThread(); renderRecent(); }
});
