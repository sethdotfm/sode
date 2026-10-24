// Phone layout: a text-message style composer. One input at a time steps
// Source → Destination → Levels; the keyboard's Next/Go advances, and a
// bottom sheet with a slider confirms. On the Destination step, + (typed
// or the button) adds another destination and stays on the step.

import {
  $, el, S, on, connState, routerLabel, linked, setLinkDisabled, renderDown, setDot, search, defaultLevelsText, levelBar, allLevels,
  fedBy, dstItem, srcItem, labelNode, currentlyNodes, fmtTime, undoPlan, retryPlan, planVerb, entryNote, historyPager,
  statusNode, folderTitle, folderPartial, folderLevels, folderIds, folderUndoPlan, folderRetryPlan,
  resultNode, showResult,
  patchPlan, planBlocked, planSummary, sendTakes,
} from './core.js';
import { layout, onLayout } from './layout.js';
import { emptyNote } from './filters.js';
import { levelBox, combobox, disclosure, undoButton, retryButton, arrivals, feeding, lockout, stackSteps } from './widgets.js';

const STEPS = ['src', 'dst', 'lvl'];
const PROMPT = { src: 'Source', dst: 'Destination', lvl: 'Levels' };
const inputs = { src: $('m-src'), dst: $('m-dst'), lvl: $('m-lvl') };
const M = {
  step: 'src',
  sel: { src: null },
  dsts: [], // destinations so far, one bubble each
  feedOpen: false,
};
// The best match sits nearest the input (the list is column-reverse), so
// ↑ moves to the next-best one.
const matches = combobox(() => inputs[M.step], $('m-matches'), {
  // One entry per destination: those already added don't come up again.
  source: (text) => (M.step === 'lvl' || M.sel[M.step] ? []
    : search(M.step === 'src' ? S.srcs : S.dsts.filter((d) => !M.dsts.some((x) => x.n === d.n)), text, 50)),
  onPick: () => advance(),
  press: 'click',
  cls: 'sode-m__match',
  hiCls: 'sode-m__match--hi',
  reverse: true,
});
const lvl = levelBox(inputs.lvl, $('m-picks'), $('m-all'), { hold: 'pointerdown', placeholder: () => `${defaultLevelsText()} (default)` });
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
  matches.refresh(); // none on the Levels step
  lvl.render();
  if (focus) inputs[step].focus();
  renderThread();
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
      const it = matches.current();
      if (!it) return shake(inputs.src);
      choose(it);
    }
    return setStep(M.dsts.length ? 'lvl' : 'dst');
  }
  if (step === 'dst') {
    // Next with an empty box goes on with the destinations already added.
    if (inputs.dst.value.trim() || !M.dsts.length) {
      const it = matches.current();
      if (!it) return shake(inputs.dst);
      addDst(it);
    }
    return setStep('lvl');
  }
  if (!M.sel.src) return setStep('src');
  if (!M.dsts.length) return setStep('dst');
  const levels = lvl.levels();
  if (!levels) return shake(inputs.lvl);
  inputs.lvl.blur();
  openSheet(patchPlan(M.sel.src, M.dsts, levels));
}

// +: the best match becomes a bubble and the step starts over for another.
function plus() {
  if (M.step !== 'dst') return;
  const it = matches.current();
  if (!it) return shake(inputs.dst);
  addDst(it);
  inputs.dst.focus();
}

function choose(it) {
  M.sel.src = it;
  M.feedOpen = false;
  inputs.src.value = '';
  matches.close();
  renderThread();
}

function addDst(it) {
  if (!M.dsts.some((d) => d.n === it.n)) M.dsts.push(it);
  inputs.dst.value = '';
  matches.close();
  renderThread();
}

// Puts a destination bubble back in the box to edit.
function editDst(i) {
  const [it] = M.dsts.splice(i, 1);
  inputs.dst.value = it.label.startsWith(it.num + ' ') ? it.name : it.label;
  setStep('dst');
  inputs.dst.select();
  matches.refresh();
}

function reset() {
  M.sel = { src: null };
  M.dsts = [];
  for (const k of ['src', 'dst']) inputs[k].value = '';
  lvl.clear();
  setStep('src', false);
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
    matches.refresh();
  });
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
  } else if (M.sel.src) {
    nodes.push(el('div', 'sode-m__card', feeding(fedBy(M.sel.src.n, allLevels()), {
      open: M.feedOpen,
      onToggle: () => { M.feedOpen = !M.feedOpen; renderThread(); },
      onPick: (d) => { addDst(dstItem(d)); setStep('lvl'); },
      hold: 'pointerdown',
    })));
  }
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
const arrived = arrivals();

const openFolders = new Set(); // history groups showing their takes
const openChanges = new Set(); // merged rows (history IDs) listing their changes

// Recent's buttons close any swiped-open row, then confirm.
const sheetFor = (plan) => { closeSwipes(); openSheet(plan); };
const undoBtn = (plan, label, text = 'Undo') => undoButton(plan, label, sheetFor, { text, cls: 'sode-m__undo' });

function recentRow(e) {
  const src = srcItem(e.src, e.srcName), dst = dstItem(e.dst, e.dstName);
  const undo = e.chain
    ? undoBtn(undoPlan(e), `Undo ${e.chain.length} changes on ${dst.label}`, `Undo ${e.chain.length}`)
    : undoBtn(undoPlan(e), `Undo ${src.label} to ${dst.label}`);
  const status = statusNode({ e });
  const retry = retryButton(!e.ok, `Retry ${src.label} to ${dst.label}`, () => sheetFor(retryPlan(e)));
  // A stacked row's changes, behind a toggle as on desktop.
  const open = !!e.chain && openChanges.has(e.id);
  const changesId = `m-changes-${e.id}`;
  const toggle = e.chain && disclosure(open, 'Changes', changesId, () => {
    if (openChanges.has(e.id)) openChanges.delete(e.id);
    else openChanges.add(e.id);
    renderRecent();
  });
  let changes = null;
  if (open) {
    changes = el('div', 'sode-m__changes', ...stackSteps(e, sheetFor));
    changes.id = changesId;
  }
  const face = el('div', 'sode-m__recent-face',
    el('div', 'sode-m__recent-top', el('span', 'sode__hist-time', fmtTime(e.time)),
      el('span', 'sode-m__recent-status', status, retry, toggle)),
    el('div', 'sode-m__recent-route', el('strong', null, labelNode(src)), ' → ', el('strong', null, labelNode(dst)), entryNote(e)),
    changes,
    e.follows ? el('div', 'sode__hint', `Followed by: ${[...new Set(e.follows.map((f) => dstItem(f.dst, f.dstName).label))].join(', ')}`) : null,
    levelBar(e.levels.filter((u) => S.levelBy.has(u))));
  const row = el('li', 'sode-m__recent-row' + (arrived.entry(e) ? ' sode--enter' : ''), undo, face);
  swipeable(row, face);
  return row;
}

// A folder: one confirm's takes behind a summary row. Swipe it to undo
// them all; open it to swipe or retry them one by one.
function folderRows(f) {
  const open = openFolders.has(f.group);
  const kidsId = `m-folder-${f.group}`;
  const undo = undoBtn(folderUndoPlan(f), `Undo all ${f.entries.length} takes of ${folderIds(f)}`, folderPartial(f) ? `Undo all ${f.entries.length}` : 'Undo all');
  const failed = f.entries.filter((e) => !e.ok).length;
  const retry = retryButton(failed, `Retry the ${failed} failed take${failed === 1 ? '' : 's'} of ${folderIds(f)}`, () => sheetFor(folderRetryPlan(f)));
  const toggle = disclosure(open, `${f.shown.length}`, kidsId, () => {
    if (openFolders.has(f.group)) openFolders.delete(f.group);
    else openFolders.add(f.group);
    renderRecent();
  });
  toggle.setAttribute('aria-label', `${open ? 'Hide' : 'Show'} the ${f.shown.length} takes`);

  const face = el('div', 'sode-m__recent-face',
    el('div', 'sode-m__recent-top', el('span', 'sode__hist-time', fmtTime(f.entries[0].time)),
      el('span', 'sode-m__recent-status', statusNode({ f }), retry, toggle)),
    el('div', 'sode-m__recent-route', ...folderTitle(f)),
    levelBar(folderLevels(f)));
  const row = el('li', 'sode-m__recent-row' + (arrived.folder(f, open) ? ' sode--enter' : ''), undo, face);
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
  const rows = pager.list();
  $('m-recent').hidden = !S.hist.length;
  $('m-recent-list').replaceChildren(...rows.flatMap((r) => (r.f ? folderRows(r.f) : [recentRow(r.e)])));
  $('m-recent-more').replaceChildren(...[emptyNote(rows, pager.fetching()), pager.button()].filter(Boolean));
}

// ---- confirm sheet with slider

let pending = null; // plan + lock (a shallow copy: planBlocked's prot is keyed by its items)

function openSheet(plan) {
  const { block, prot } = planBlocked(plan);
  if (block) return showResult(block.kind, block.title, block.text);
  $('m-sheet-title').textContent = `Confirm ${planVerb(plan)}`;
  $('m-sheet-body').replaceChildren(...planSummary(plan, { prot }));
  $('m-slide-label').textContent = `Slide to ${planVerb(plan)}`;
  resetSlide(true);
  $('m-sheet').hidden = false;
  pending = { ...plan, lock: lockout(() => $('m-slide').classList.remove('sode__slide--locked')) };
}

function closeSheet() {
  pending?.lock.cancel();
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
  if (!pending || !pending.lock.ready()) return;
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
  if (!pending || !pending.lock.ready()) return;
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
  matches.refresh();
});
// A typed + adds the destination. Read from the value, since phone
// keyboards often send no usable key code.
inputs.dst.addEventListener('input', () => {
  const v = inputs.dst.value;
  if (v.includes('+')) {
    inputs.dst.value = v.replace(/\+/g, '');
    matches.refresh();
    plus();
    return;
  }
  matches.refresh();
});
for (const k of STEPS) {
  inputs[k].addEventListener('keydown', (e) => {
    if (!active() || matches.nav(e)) return;
    if (e.key === 'Enter') {
      e.preventDefault();
      advance();
    } else if (e.key === 'Backspace' && !inputs[k].value && k !== 'src') {
      // Backspace on an empty step goes back to edit the previous one:
      // the newest destination, then the source.
      e.preventDefault();
      if (M.dsts.length) {
        const it = M.dsts.pop();
        inputs.dst.value = it.name || it.num;
        setStep('dst');
        matches.refresh();
        return;
      }
      if (k === 'lvl') return setStep('dst');
      const it = M.sel.src;
      M.sel.src = null;
      inputs.src.value = it ? it.name || it.num : '';
      setStep('src');
      matches.refresh();
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
  lvl.reset();
  setStep(M.step, false);
  renderRecent();
});
on('names', () => { matches.refresh(); renderThread(); });
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
