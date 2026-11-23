// Phone layout: one text field steps Source → Destination → Levels; a bottom sheet with a slider
// confirms. A blank Source or Destination goes to the other first (to see what is on it) and
// comes back.

import {
  $,
  el,
  S,
  on,
  connState,
  routerTitle,
  linked,
  setLinkDisabled,
  renderDown,
  setDot,
  boxSearch,
  sameEnd,
  dstItem,
  srcItem,
  labelNode,
  routerOfItem,
  linkList,
} from './core.js';
import { defaultLevelsWord, allLevels, levelText, levelSource, oneLevel } from './levelview.js';
import { levelBox } from './levelbox.js';
import { levelRouterOf } from './levels.js';
import { fedBy, currentlyNodes, fedThroughTies } from './routes.js';
import { flipped } from './device.js';
import { paintRouters } from './connview.js';
import { routerMarks } from './routerlinkrules.js';
import { renderDegraded } from './degradedview.js';
import { resultNode, patchPlan, rightsOff, lastTake, nextTake, armedAdd, boxPatch, lastChip } from './plans.js';
import { layout, onLayout } from './layout.js';
import { kbdUp } from './keyboard.js';
import { presetHost } from './presets.js';
import { lastFill } from './historyrows.js';
import { presetAdd } from './presetadd.js';
import { combobox, feeding } from './widgets.js';
import { mobileHistory, closeSwipes } from './mobilehistory.js';
import { mobileConfirm } from './mobileconfirm.js';

const STEPS = ['src', 'dst', 'lvl'];
const PROMPT = { src: 'Source', dst: 'Destination', lvl: 'Levels' };
const order = () => (flipped() ? ['dst', 'src', 'lvl'] : STEPS);
const inputs = { src: $('m-src'), dst: $('m-dst'), lvl: $('m-lvl') };
const view = $('mobile-view');
const M = {
  step: order()[0],
  sel: { src: null },
  dsts: [],
  dstFirst: false,
  feedOpen: false,
};
// The best match sits nearest the input (column-reverse list), so ↑ moves to the next-best.
const matches = combobox(() => inputs[M.step], $('m-matches'), {
  source: (text) =>
    M.step === 'lvl' || M.sel[M.step] ? [] : boxSearch(M.step, text, { n: 50, keep: M.step === 'dst' ? (d) => !M.dsts.some((x) => sameEnd(x, d)) : undefined }),
  onPick: () => advance(),
  add: (it) => {
    addDst(it, true);
    inputs.dst.focus();
  },
  canAdd: () => M.step === 'dst',
  outside: (it) => rightsOff(M.step === 'src' ? 'sources' : 'dsts', it),
  press: 'click',
  cls: 'sode-m__match',
  hiCls: 'sode-m__match--hi',
  reverse: true,
});
// Typed levels are levels of the router of the route's destination, as on the desktop. The picks follow it
// (followLevels).
// What source src feeds: on its own router on every level, and through tie lines.
const feedList = (src) => {
  const uid = routerOfItem(src);
  return [
    ...fedBy(src.n, allLevels(uid), uid).map((d) => dstItem(d, '', uid)),
    ...fedThroughTies(src).map((f) => ({ ...dstItem(f.dst, '', f.router), via: f.row })),
  ];
};
const lvlRouter = () => levelRouterOf({ dsts: M.dsts, src: M.sel.src }, routerOfItem);
let lvlAt = '';
const lvl = levelBox(inputs.lvl, $('m-picks'), $('m-all'), () => levelSource(lvlRouter()), {
  hold: 'pointerdown',
  onChange: () => renderThread(),
});
function followLevels() {
  if (lvlRouter() === lvlAt) return;
  lvlAt = lvlRouter();
  lvl.repick();
}
const active = () => layout() === 'phone' && document.documentElement.dataset.view !== 'settings' && !document.documentElement.hasAttribute('data-sode-setup');

function renderConn() {
  const c = connState();
  setDot($('m-ws-dot'), $('m-ws-text'), [c.ws[0], 'Server'], { base: 'server', label: c.ws[1] });
  $('m-ws-text').parentElement.title = c.ws[1];
  const list = linkList();
  paintRouters($('m-routers'), routerMarks(list), true);
  const [label, isName] = list.length === 1 ? routerTitle() : ['', false];
  $('m-rt-addr').textContent = label;
  $('m-rt-addr').title = label;
  $('m-rt-addr').classList.toggle('sode__conn-addr--name', isName);
  renderDown($('m-down'), c);
  renderDegraded($('m-degraded'), c);
  syncGo();
}

// Next works offline; Go shows disabled without the link (a press says "Not connected").
function syncGo() {
  setLinkDisabled($('m-go'), M.step === 'lvl' && !linked());
}

// Go on the last step. While a preset adds patches (presetadd.js) it is Add in the add colour: it adds the patch and
// sends nothing.
const takeState = () => ({ word: presetAdd.active() ? 'Add' : 'Go', ok: presetAdd.active(), disabled: false });
function paintGo() {
  const last = M.step === 'lvl';
  const t = takeState();
  $('m-go').textContent = last ? t.word : 'Next';
  $('m-go').classList.toggle('pb-btn--commit', last && !t.ok);
  $('m-go').classList.toggle('sode-btn--primary', last && t.ok);
  syncGo();
}
on('presetAdd', paintGo);

function setStep(step, focus = true) {
  M.step = step;
  $('m-bar').dataset.step = step;
  for (const k of STEPS) inputs[k].hidden = k !== step;
  $('m-step').textContent = PROMPT[step];
  $('m-step').htmlFor = inputs[step].id;
  paintGo();
  $('m-add').hidden = step !== 'dst';
  $('m-picker').hidden = step !== 'lvl';
  $('m-ops').hidden = step !== 'lvl';
  matches.refresh();
  lvl.render();
  if (focus) inputs[step].focus();
  renderThread();
}

function shake(node) {
  node.classList.remove('sode-m__shake');
  void node.offsetWidth;
  node.classList.add('sode-m__shake');
}

// Focus moves within the same handler so the phone keeps its keyboard open.
function advance() {
  const step = M.step;
  if (step === 'src') {
    if (!M.sel.src) {
      if (!inputs.src.value.trim() && !M.dsts.length) {
        M.dstFirst = true;
        return setStep('dst');
      }
      const it = matches.current();
      if (!it) return shake(inputs.src);
      choose(it);
    }
    return setStep(M.dsts.length ? 'lvl' : 'dst');
  }
  if (step === 'dst') {
    if (!inputs.dst.value.trim() && !M.dsts.length && !M.sel.src) return setStep('src');
    if (inputs.dst.value.trim() || !M.dsts.length) {
      const it = matches.current();
      if (!it) return shake(inputs.dst);
      addDst(it);
    }
    return setStep(M.sel.src ? 'lvl' : 'src');
  }
  const missing = order().find((k) => (k === 'src' ? !M.sel.src : k === 'dst' && !M.dsts.length));
  if (missing) return setStep(missing);
  const levels = lvl.levels();
  if (!levels) return shake(inputs.lvl);
  inputs.lvl.blur();
  if (presetAdd.active()) return addBoxes();
  sheet.open(patchPlan(M.sel.src, M.dsts, levels));
}

// The boxes' patch as a preset row: { ok, text }. The boxes clear after an add.
function addBoxes() {
  const r = presetAdd.addPatch(boxPatch(M.sel.src, [...M.dsts], lvl));
  if (r.ok) reset();
  return r;
}

// + empties the box after the add, as the desktop does, so the next digits start a new search. On the empty box, +
// adds the next add (nextAdd, boxlist.js), which the box shows (paintArmed).
function plus() {
  if (M.step !== 'dst') return;
  const it = matches.current() || (!inputs.dst.value.trim() && armed());
  if (!it) return shake(inputs.dst);
  addDst(it, true);
  inputs.dst.focus();
}

function choose(it) {
  M.sel.src = it;
  M.feedOpen = false;
  inputs.src.value = '';
  matches.close();
  renderThread();
}

// The last + add arms the next add on the empty box, by the Next match behavior setting as on the desktop. Typing, a
// Next, an edit and a reset end it.
let lastAdd = null;
const armed = () => (lastAdd && !inputs.dst.value ? armedAdd(lastAdd, (d) => M.dsts.some((x) => sameEnd(x, d))) : null);

function paintArmed() {
  const d = armed();
  const text = d ? `+ adds ${d.label}` : 'Destination';
  if (inputs.dst.placeholder !== text) inputs.dst.placeholder = text;
}

// plus: the add came from +, so it arms the next add.
function addDst(it, plus = false) {
  if (!M.dsts.some((d) => sameEnd(d, it))) M.dsts.push(it);
  inputs.dst.value = '';
  lastAdd = plus ? it : null;
  renderThread();
  matches.close();
}

function editDst(i) {
  lastAdd = null;
  const [it] = M.dsts.splice(i, 1);
  inputs.dst.value = editText(it);
  setStep('dst');
  inputs.dst.select();
  matches.refresh();
}

function reset() {
  M.sel = { src: null };
  M.dsts = [];
  M.dstFirst = false;
  for (const k of ['src', 'dst']) inputs[k].value = '';
  lastAdd = null;
  lvl.clear();
  setStep(order()[0], false);
}

// "Last:" chip as on desktop: tap fills the step from the last take. Hidden when the step has
// it, and for default levels.
const boxState = () => ({ src: M.sel.src, dsts: M.dsts, levels: lvl.levels() || [] });

function renderLast() {
  const slot = $('m-last');
  const k = M.step;
  const state = S.cfg && boxState();
  const what = { src: 'source', dst: 'destinations', lvl: 'levels' }[k];
  slot.replaceChildren(
    ...['', 'Next']
      .filter((n) => state && (!n || k !== 'lvl'))
      .map((n) => {
        const v = lastChip(n ? nextTake(state) : lastTake(), state)[k];
        if (!v) return null;
        const b = el('button', 'pb-chip sode__last', el('span', 'sode__tag', n || 'Last'), el('span', 'sode__last-text', ...v));
        b.type = 'button';
        b.setAttribute('aria-label', n ? `Next: use the ${what} after the last take's` : `Use the last take's ${what}`);
        b.addEventListener('pointerdown', (e) => e.preventDefault());
        b.addEventListener('click', () => useLast(k, !!n));
        return b;
      })
      .filter(Boolean),
  );
}

function pickSrc(n) {
  closeSwipes();
  choose(srcItem(n));
  setStep(M.dsts.length ? 'lvl' : 'dst');
}

function useLast(k, next = false) {
  const fill = lastFill(k, next, boxState());
  if (!fill || fill.none) return;
  const t = fill.t;
  if (k === 'src') {
    choose(t.src);
    setStep(M.dsts.length ? 'lvl' : 'dst');
  } else if (k === 'dst') {
    for (const d of t.dsts) if (!M.dsts.some((x) => sameEnd(x, d))) M.dsts.push(d);
    inputs.dst.value = '';
    matches.close();
    setStep(M.sel.src ? 'lvl' : 'src');
  } else {
    lvl.set(t.levels);
  }
}

const editText = (it) => (it.label.startsWith(it.num + ' ') ? it.name : it.label);

function editSrc() {
  const it = M.sel.src;
  M.sel.src = null;
  inputs.src.value = editText(it);
  setStep('src');
  inputs.src.select();
  matches.refresh();
}

function tap(node, label, onTap) {
  if (label) node.setAttribute('aria-label', label);
  node.addEventListener('pointerdown', (e) => e.preventDefault());
  node.addEventListener('click', (e) => {
    e.stopPropagation();
    onTap();
  });
  return node;
}

const maybe = (k) => (M.step === k && inputs[k].value.trim() ? matches.current() : null);

function sumRow(k, ...vals) {
  const row = el(
    'div',
    `sode-m__sum-row sode-m__sum-row--${k}` + (M.step === k ? ' sode-m__sum-row--on' : ''),
    el('span', 'sode-m__sum-key', PROMPT[k]),
    el('span', 'sode-m__sum-vals', ...vals),
  );
  return tap(row, null, () => {
    if (M.step !== k) setStep(k);
  });
}

const sumVal = (it, cls = '') => el('span', 'sode-m__sum-val' + cls, labelNode(it));
const sumNone = (text) => el('span', 'sode__hint', text);

function summary() {
  const src = M.sel.src
    ? tap(el('button', 'sode-m__sum-val', labelNode(M.sel.src)), `Source ${M.sel.src.label}, tap to change`, editSrc)
    : maybe('src')
      ? sumVal(maybe('src'), ' sode-m__sum-val--maybe')
      : sumNone(M.step === 'src' && !M.dsts.length && !inputs.src.value.trim() ? '— Press Next to select a destination first.' : '—');
  const blankDst = M.step === 'dst' && !M.sel.src && !M.dsts.length && !inputs.dst.value.trim();
  const dsts = M.dsts.map((it, i) => tap(el('button', 'sode-m__sum-val', labelNode(it)), `Destination ${it.label}, tap to change`, () => editDst(i)));
  const next = maybe('dst');
  if (next) dsts.push(sumVal(next, ' sode-m__sum-val--maybe'));
  const levels = lvl.levels();
  const rows = {
    src: () => sumRow('src', src),
    dst: () => sumRow('dst', ...(dsts.length ? dsts : [sumNone(blankDst ? '— Press Next to select a source first.' : '—')])),
    lvl: () =>
      sumRow(
        'lvl',
        levels
          ? el('span', 'sode-m__sum-lvls', oneLevel(lvlRouter()) ? 'All levels' : levelText(levels, lvlRouter()))
          : el('span', 'pb-badge pb-badge--error', 'Not valid'),
        levels && lvl.empty() && !oneLevel(lvlRouter()) ? sumNone(`(${defaultLevelsWord(lvlRouter())})`) : null,
      ),
  };
  return el('div', 'sode-m__sum', ...order().map((k) => rows[k]()));
}

function renderThread() {
  followLevels();
  paintArmed();
  if (!S.cfg) return;
  const end = atEnd();
  const nodes = [];
  const typing = view.classList.contains('sode-m--typing');
  if (typing || M.sel.src || M.dsts.length) nodes.push(summary());
  // Room is short: Currently only for the newest destination, or the one being typed.
  const last = M.dsts[M.dsts.length - 1] || maybe('dst');
  const others = M.dsts.length - 1;
  if (last) {
    // With no source, the route view of one destination offers Protect and Free Protect.
    const protect = !M.sel.src && others <= 0 ? (p) => sheetFor(p) : null;
    nodes.push(el('div', 'sode-m__card', ...currentlyNodes(last, null, protect), others > 0 ? el('div', 'sode__hint', `+${others} more`) : null));
  } else if (M.sel.src) {
    nodes.push(
      el(
        'div',
        'sode-m__card',
        feeding(feedList(M.sel.src), {
          open: M.feedOpen,
          onToggle: () => {
            M.feedOpen = !M.feedOpen;
            renderThread();
          },
          onPick: (d) => {
            addDst(dstItem(d.n, '', d.router || ''));
            setStep('lvl');
          },
          hold: 'pointerdown',
        }),
      ),
    );
  }
  $('m-compose').replaceChildren(...nodes);
  renderLast();
  if (end) scrollThread();
}

// Redraws keep the thread's end in view only if it was in view, so a person who reads back
// through History is not interrupted. While the person types, the summary stays pinned at the
// top and nothing scrolls.
function atEnd() {
  const t = $('m-thread');
  return t.scrollHeight - t.scrollTop - t.clientHeight < 24;
}
function scrollThread() {
  requestAnimationFrame(() => {
    if (view.classList.contains('sode-m--typing')) return;
    const t = $('m-thread');
    t.scrollTop = t.scrollHeight;
  });
}

const sheetFor = (plan) => {
  closeSwipes();
  sheet.open(plan);
};
const hist = mobileHistory({ sheetFor, pickSrc });
const sheet = mobileConfirm(reset);

inputs.src.addEventListener('input', () => {
  M.sel.src = null;
  matches.refresh();
  renderThread();
});
// Read from the value: phone keyboards often send no usable key code.
inputs.dst.addEventListener('input', () => {
  const v = inputs.dst.value;
  if (v.includes('+')) {
    inputs.dst.value = v.replace(/\+/g, '');
    matches.refresh();
    plus();
    return;
  }
  lastAdd = null;
  matches.refresh();
  renderThread();
});
for (const k of STEPS) {
  inputs[k].addEventListener('keydown', (e) => {
    if (!active() || matches.nav(e)) return;
    if (e.key === 'Enter') {
      e.preventDefault();
      advance();
    } else if (e.key === 'Backspace' && !inputs[k].value) {
      // Backspace on an empty step goes back to edit the previous pick (Source-first: Levels →
      // Destination's newest → source; Destination-first: Source → newest destination).
      const back = k === 'lvl' ? order().slice(0, 2).reverse() : k === 'dst' ? ['dst', 'src'] : flipped() || M.dstFirst ? ['dst'] : [];
      const to = back.find((b) => (b === 'dst' ? M.dsts.length : M.sel.src));
      if (!to) return;
      e.preventDefault();
      if (to === 'dst') {
        const it = M.dsts.pop();
        inputs.dst.value = it.name || it.num;
      } else {
        const it = M.sel.src;
        M.sel.src = null;
        inputs.src.value = it.name || it.num;
      }
      setStep(to);
      matches.refresh();
    }
  });
}
// The number pad has no ! , or -: these keys type the words for them at the cursor, spaced.
for (const b of $('m-ops').children) {
  b.addEventListener('pointerdown', (e) => e.preventDefault());
  b.addEventListener('click', () => {
    const i = inputs.lvl;
    const a = i.selectionStart ?? i.value.length,
      z = i.selectionEnd ?? a;
    const pre = /(^|\s)$/.test(i.value.slice(0, a)) ? '' : ' ';
    i.setRangeText(`${pre}${b.dataset.op} `, a, z, 'end');
    i.dispatchEvent(new Event('input'));
  });
}
// Backspace right after such a word takes the word and its spaces. beforeinput: phone keyboards
// often send no key code.
inputs.lvl.addEventListener('beforeinput', (e) => {
  const i = inputs.lvl;
  if (e.inputType !== 'deleteContentBackward' || i.selectionStart !== i.selectionEnd) return;
  const before = i.value.slice(0, i.selectionStart);
  const m = before.match(/\s*\b(not|and|through)\s*$/i);
  if (!m) return;
  e.preventDefault();
  i.setRangeText('', m.index, i.selectionStart, 'end');
  i.dispatchEvent(new Event('input'));
});
for (const id of ['m-go', 'm-add']) $(id).addEventListener('pointerdown', (e) => e.preventDefault());
$('m-go').addEventListener('click', advance);
$('m-add').addEventListener('click', plus);
$('m-cancel').addEventListener('click', () => {
  reset();
  document.activeElement?.blur();
});

// While a composer input has focus the keyboard is up and room is short, so History hides. Focus moves between steps in one task, so the check runs after the blur.
function syncTyping() {
  const typing = STEPS.some((k) => document.activeElement === inputs[k]);
  if (view.classList.contains('sode-m--typing') === typing) return;
  const t = $('m-thread');
  view.classList.toggle('sode-m--typing', typing);
  renderThread();
  if (typing) t.scrollTop = 0;
  else
    requestAnimationFrame(() => {
      t.scrollTop = 0;
    });
}
const filterBox = () => !!document.activeElement?.closest('#m-recent [data-search]');
const syncFiltering = () => view.classList.toggle('sode-m--filtering', filterBox());
$('m-recent').addEventListener('focusin', syncFiltering);
$('m-recent').addEventListener('focusout', () => setTimeout(syncFiltering, 0));
for (const k of STEPS) {
  inputs[k].addEventListener('focus', syncTyping);
  inputs[k].addEventListener('blur', () => setTimeout(syncTyping, 0));
}

// iOS Safari does not shrink the layout viewport for the keyboard: size the phone view to the
// visual viewport. Android Chrome resizes itself (interactive-widget).
const vv = window.visualViewport;
if (vv) {
  const fit = () => {
    const r = document.documentElement.style;
    r.setProperty('--vvh', `${vv.height}px`);
    r.setProperty('--vvt', `${vv.offsetTop}px`);
    document.documentElement.toggleAttribute('data-kbd', kbdUp(vv, window.innerHeight));
  };
  vv.addEventListener('resize', fit);
  vv.addEventListener('scroll', fit);
  fit();
}

on('hello', () => {
  hist.pager.reset();
  lvlAt = lvlRouter();
  lvl.reset();
  setStep(M.step, false);
  hist.renderHistory();
});
// The thread and History are hidden on the other layouts, so only the phone layout draws them. onLayout draws them when it comes back.
const phone = () => layout() === 'phone';
on('names', () => {
  matches.refresh();
  if (phone()) renderThread();
});
on('tally', () => phone() && renderThread());
on('myRights', () => phone() && renderThread());
on('conn', renderConn);
// A Protect row's Undo follows the router's state, the sign-in and the Rights (historyrows.js protectActions).
for (const ev of ['protectedRows', 'myRights', 'conn']) on(ev, () => phone() && hist.renderHistory());
on('history', () => {
  if (!phone()) return;
  hist.renderHistory();
  renderLast();
  // A Next match behavior change comes as history (device.js).
  paintArmed();
});
on('last', renderLast);
on('result', (r) => {
  const end = atEnd();
  $('m-result').replaceChildren(resultNode(r));
  if (end) scrollThread();
});
on('flip', () => {
  if (!M.sel.src && !M.dsts.length) setStep(order()[0], false);
  else renderThread();
});
onLayout((l) => {
  hist.placeBar();
  if (l !== 'phone') sheet.close();
  else {
    renderThread();
    hist.renderHistory();
    renderLast();
  }
});

// Start on the right step so a Destination-first browser does not show Source until hello.
setStep(order()[0], false);

presetHost({
  active,
  confirm: sheetFor,
  patch: () => boxPatch(M.sel.src, [...M.dsts], lvl),
  // The phone's Grid popup has its own Take and Clear (gridpaneltake.js), so these are Go and the reset.
  take: () => (presetAdd.active() ? addBoxes() : (advance(), null)),
  clearAll: reset,
  takeState,
  picks: () => ({ src: M.sel.src, dsts: [...M.dsts], levels: lvl.empty() ? [] : lvl.levels() || [] }),
  clear: reset,
  focus: () => setStep(order()[0], false),
});
