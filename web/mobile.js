// Phone layout: a text-message style composer. One input at a time steps
// Source → Destination → Levels; the keyboard's Next/Go advances, and a
// bottom sheet with a slider confirms. On the Destination step, + (typed
// or the button) adds another destination and stays on the step.

import {
  $, el, S, on, connState, routerLabel, linked, needsLink, setLinkDisabled, renderDown, setDot, search, parseLevels, levelInput, defaultLevelsText, relabelLevels, levelBar, pickButtons, allLevels,
  fedBy, dstItem, dstLabel, srcItem, labelNode, currentlyNodes, fmtTime, undoPlan, retryPlan, planVerb, entryNote, historyPager,
  chevron, statusNode, folderTitle, folderPartial, folderLevels, folderIds, folderUndoPlan, folderRetryPlan, planEmpty,
  resultNode, showResult,
  patchPlan, planBlocked, planSummary, sendTakes,
} from './core.js';
import { layout, onLayout } from './layout.js';
import { emptyNote } from './filters.js';

const STEPS = ['src', 'dst', 'lvl'];
const PROMPT = { src: 'Source', dst: 'Destination', lvl: 'Levels' };
const inputs = { src: $('m-src'), dst: $('m-dst'), lvl: $('m-lvl') };
const M = {
  step: 'src',
  sel: { src: null },
  dsts: [], // destinations so far, one bubble each
  matches: [],
  hi: 0,
  lvlNone: false,
  feedOpen: false,
};
const active = () => layout() === 'phone' && document.documentElement.dataset.view !== 'settings';

// ---- header

function renderConn() {
  const c = connState();
  setDot($('m-ws-dot'), $('m-ws-text'), [c.ws[0], 'Server']);
  $('m-ws-text').parentElement.title = c.ws[1];
  setDot($('m-rt-dot'), $('m-rt-text'), [c.rt[0], 'Router']);
  $('m-rt-text').parentElement.title = c.rt[1];
  const [label, isName] = routerLabel();
  $('m-rt-addr').textContent = label;
  $('m-rt-addr').title = label;
  $('m-rt-addr').classList.toggle('sode__conn-addr--name', isName);
  renderDown($('m-down'), c);
  syncGo();
}

// Next works offline (it only moves through the steps); Go, which patches,
// shows as disabled without the link (a press gets "Not connected").
function syncGo() {
  setLinkDisabled($('m-go'), M.step === 'lvl' && !linked());
}

// ---- composer

function setStep(step, focus = true) {
  M.step = step;
  for (const k of STEPS) inputs[k].hidden = k !== step;
  $('m-step').textContent = PROMPT[step];
  $('m-step').htmlFor = inputs[step].id;
  $('m-go').textContent = step === 'lvl' ? 'Go' : 'Next';
  syncGo();
  $('m-add').hidden = step !== 'dst';
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
    // One entry per destination: those already added don't come up again.
    const list = M.step === 'src' ? S.srcs : S.dsts.filter((d) => !M.dsts.some((x) => x.n === d.n));
    M.matches = search(list, inputs[M.step].value, 50);
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
  if (step === 'src') {
    if (!M.sel.src) {
      const it = M.matches[M.hi];
      if (!it) return shake(inputs.src);
      choose(it);
    }
    return setStep(M.dsts.length ? 'lvl' : 'dst');
  }
  if (step === 'dst') {
    // Next with an empty box goes on with the destinations already added.
    if (inputs.dst.value.trim() || !M.dsts.length) {
      const it = M.matches[M.hi];
      if (!it) return shake(inputs.dst);
      addDst(it);
    }
    return setStep('lvl');
  }
  if (!M.sel.src) return setStep('src');
  if (!M.dsts.length) return setStep('dst');
  const levels = parseLevels(inputs.lvl.value, M.lvlNone);
  if (!levels) return shake(inputs.lvl);
  inputs.lvl.blur();
  openSheet(patchPlan(M.sel.src, M.dsts, levels));
}

// +: the best match becomes a bubble and the step starts over for another.
function plus() {
  if (M.step !== 'dst') return;
  const it = M.matches[M.hi];
  if (!it) return shake(inputs.dst);
  addDst(it);
  inputs.dst.focus();
}

function choose(it) {
  M.sel.src = it;
  M.feedOpen = false;
  inputs.src.value = '';
  M.matches = [];
  renderMatches();
  renderThread();
}

function addDst(it) {
  if (!M.dsts.some((d) => d.n === it.n)) M.dsts.push(it);
  inputs.dst.value = '';
  M.matches = [];
  renderMatches();
  renderThread();
}

// Puts a destination bubble back in the box to edit.
function editDst(i) {
  const [it] = M.dsts.splice(i, 1);
  inputs.dst.value = it.label.startsWith(it.num + ' ') ? it.name : it.label;
  setStep('dst');
  inputs.dst.select();
  refresh();
}

function reset() {
  M.sel = { src: null };
  M.dsts = [];
  M.lvlNone = false;
  for (const k of STEPS) inputs[k].value = '';
  setStep('src', false);
}

// ---- levels

function renderLevels() {
  if (!S.cfg) return;
  const list = parseLevels(inputs.lvl.value, M.lvlNone);
  const on = new Set(list || []);
  inputs.lvl.placeholder = M.lvlNone ? 'none' : `${defaultLevelsText()} (default)`;
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

function bubble(k, it, onTap) {
  const b = el('button', `sode-m__bubble sode-m__bubble--${k}` + (k === 'src' && M.step === k ? ' sode-m__bubble--editing' : ''),
    el('span', 'sode-m__bubble-label', PROMPT[k]), it.label);
  b.type = 'button';
  b.setAttribute('aria-label', `${PROMPT[k]} ${it.label}, tap to change`);
  b.addEventListener('pointerdown', (e) => e.preventDefault());
  b.addEventListener('click', onTap);
  return b;
}

function srcBubble() {
  const it = M.sel.src;
  return bubble('src', it, () => {
    M.sel.src = null;
    inputs.src.value = it.label.startsWith(it.num + ' ') ? it.name : it.label;
    setStep('src');
    inputs.src.select();
    refresh();
  });
}

function feeding() {
  const fed = fedBy(M.sel.src.n, allLevels());
  if (!fed.length) return el('div', 'sode__hint', 'Not feeding any destination');
  const chip = (d) => {
    const c = el('button', 'pb-chip', dstLabel(d));
    c.type = 'button';
    c.addEventListener('pointerdown', (e) => e.preventDefault());
    c.addEventListener('click', () => { addDst(dstItem(d)); setStep('lvl'); });
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
  if (M.sel.src) nodes.push(srcBubble());
  nodes.push(...M.dsts.map((it, i) => bubble('dst', it, () => editDst(i))));
  // Room is short: Currently for the newest destination only.
  const last = M.dsts[M.dsts.length - 1];
  const others = M.dsts.length - 1;
  if (last) {
    nodes.push(el('div', 'sode-m__card', ...currentlyNodes(last),
      others ? el('div', 'sode__hint', `and ${others} more destination${others === 1 ? '' : 's'}`) : null));
  } else if (M.sel.src) nodes.push(el('div', 'sode-m__card', feeding()));
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
const entered = new Set(); // live entries already animated in

const openFolders = new Set(); // history groups showing their takes

function undoButton(plan, label, text = 'Undo') {
  const undo = el('button', 'sode-m__undo', text);
  undo.type = 'button';
  if (planEmpty(plan)) undo.disabled = true;
  else needsLink(undo);
  undo.setAttribute('aria-label', label);
  undo.addEventListener('click', () => { closeSwipes(); openSheet(plan); });
  return undo;
}

function recentRow(e) {
  const src = srcItem(e.src, e.srcName), dst = dstItem(e.dst, e.dstName);
  const undo = undoButton(undoPlan(e), `Undo ${src.label} to ${dst.label}`);
  const status = statusNode({ e });
  let retry = null;
  if (!e.ok) {
    retry = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Retry');
    retry.type = 'button';
    needsLink(retry);
    retry.setAttribute('aria-label', `Retry ${src.label} to ${dst.label}`);
    retry.addEventListener('click', () => { closeSwipes(); openSheet(retryPlan(e)); });
  }
  const face = el('div', 'sode-m__recent-face',
    el('div', 'sode-m__recent-top', el('span', 'sode__hist-time', fmtTime(e.time)),
      el('span', 'sode-m__recent-status', status, retry)),
    el('div', 'sode-m__recent-route', el('strong', null, labelNode(src)), ' → ', el('strong', null, labelNode(dst)), entryNote(e)),
    e.chain ? el('div', 'sode__hint', `Before that: ${e.chain.slice(1).map((c) => `${srcItem(c.src, c.srcName).label} ${fmtTime(c.time)}`).join(', ')}`) : null,
    e.follows ? el('div', 'sode__hint', `Followed by: ${[...new Set(e.follows.map((f) => dstItem(f.dst, f.dstName).label))].join(', ')}`) : null,
    levelBar(e.levels.filter((u) => S.levelBy.has(u))));
  const fresh = e.id === S.newId && !entered.has(e.id);
  if (fresh) entered.add(e.id);
  const row = el('li', 'sode-m__recent-row' + (fresh ? ' sode--enter' : ''), undo, face);
  swipeable(row, face);
  return row;
}

// A folder: one confirm's takes behind a summary row. Swipe it to undo
// them all; open it to swipe or retry them one by one.
function folderRows(f) {
  const open = openFolders.has(f.group);
  const kidsId = `m-folder-${f.group}`;
  const undo = undoButton(folderUndoPlan(f), `Undo all ${f.entries.length} takes of ${folderIds(f)}`, folderPartial(f) ? `Undo all ${f.entries.length}` : 'Undo all');
  const failed = f.entries.filter((e) => !e.ok).length;
  let retry = null;
  if (failed) {
    retry = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Retry');
    retry.type = 'button';
    needsLink(retry);
    retry.setAttribute('aria-label', `Retry the ${failed} failed take${failed === 1 ? '' : 's'} of ${folderIds(f)}`);
    retry.addEventListener('click', () => { closeSwipes(); openSheet(folderRetryPlan(f)); });
  }
  const toggle = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode__hist-toggle', chevron(open), `${f.shown.length}`);
  toggle.type = 'button';
  toggle.setAttribute('aria-expanded', String(open));
  toggle.setAttribute('aria-controls', kidsId);
  toggle.setAttribute('aria-label', `${open ? 'Hide' : 'Show'} the ${f.shown.length} takes`);
  toggle.addEventListener('click', () => {
    if (openFolders.has(f.group)) openFolders.delete(f.group);
    else openFolders.add(f.group);
    renderRecent();
  });

  const face = el('div', 'sode-m__recent-face',
    el('div', 'sode-m__recent-top', el('span', 'sode__hist-time', fmtTime(f.entries[0].time)),
      el('span', 'sode-m__recent-status', statusNode({ f }), retry, toggle)),
    el('div', 'sode-m__recent-route', ...folderTitle(f)),
    levelBar(folderLevels(f)));
  const arrived = f.entries.some((e) => e.id === S.newId);
  const fresh = arrived && !entered.has(f.group);
  if (fresh) entered.add(f.group);
  if (!open && arrived) entered.add(S.newId); // don't replay it on opening
  const row = el('li', 'sode-m__recent-row' + (fresh && !open ? ' sode--enter' : ''), undo, face);
  swipeable(row, face);
  if (!open) return [row];
  const kids = el('li', 'sode-m__recent-kids', el('ul', 'sode-m__recent-list', ...f.shown.map(recentRow)));
  kids.id = kidsId;
  return [row, kids];
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

const pager = historyPager(5, () => layout() === 'phone');

function renderRecent() {
  const list = pager.list();
  $('m-recent').hidden = !S.hist.length;
  $('m-recent-list').replaceChildren(...list.flatMap((r) => (r.f ? folderRows(r.f) : [recentRow(r.e)])));
  $('m-recent-more').replaceChildren(...[emptyNote(list, pager.fetching()), pager.button()].filter(Boolean));
}

// ---- confirm sheet with slider

let pending = null;
let readyAt = 0;

function openSheet(plan) {
  const blocked = planBlocked(plan);
  if (blocked) return showResult(blocked.kind, blocked.title, blocked.text);
  pending = plan;
  $('m-sheet-title').textContent = `Confirm ${planVerb(plan)}`;
  $('m-sheet-body').replaceChildren(...planSummary(plan));
  $('m-slide-label').textContent = `Slide to ${planVerb(plan)}`;
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
  if (sendTakes(p) && p.verb !== 'undo') reset();
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

inputs.src.addEventListener('input', () => {
  M.sel.src = null;
  refresh();
});
// A typed + adds the destination. Read from the value, since phone
// keyboards often send no usable key code.
inputs.dst.addEventListener('input', () => {
  const v = inputs.dst.value;
  if (v.includes('+')) {
    inputs.dst.value = v.replace(/\+/g, '');
    refresh();
    plus();
    return;
  }
  refresh();
});
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
      const items = $('m-matches').children;
      items[M.hi].classList.remove('sode-m__match--hi');
      items[M.hi].setAttribute('aria-selected', 'false');
      M.hi = (M.hi + (e.key === 'ArrowUp' ? 1 : n - 1)) % n;
      items[M.hi].classList.add('sode-m__match--hi');
      items[M.hi].setAttribute('aria-selected', 'true');
      items[M.hi].scrollIntoView({ block: 'nearest' });
    } else if (e.key === 'Backspace' && !inputs[k].value && k !== 'src') {
      // Backspace on an empty step goes back to edit the previous one:
      // the newest destination, then the source.
      e.preventDefault();
      if (M.dsts.length) {
        const it = M.dsts.pop();
        inputs.dst.value = it.name || it.num;
        setStep('dst');
        refresh();
        return;
      }
      if (k === 'lvl') return setStep('dst');
      const it = M.sel.src;
      M.sel.src = null;
      inputs.src.value = it ? it.name || it.num : '';
      setStep('src');
      refresh();
    }
  });
}
for (const id of ['m-go', 'm-add']) $(id).addEventListener('pointerdown', (e) => e.preventDefault()); // keep the keyboard up
$('m-go').addEventListener('click', advance);
$('m-add').addEventListener('click', plus);
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
  pager.reset();
  inputs.lvl.value = relabelLevels(inputs.lvl.value);
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
