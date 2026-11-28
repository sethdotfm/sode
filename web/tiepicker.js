// The Tie line picker (docs/tielines-design.md, The Tie line picker; GLOSSARY.md). It opens for each question of a
// take, one after the other (tieflow.js). A question is one hop that one or more routes share. The picker shows the
// plan of the first route, and the destinations of every route of the question. The server gives the ranked paths (tiepage.go, the message
// tiePlan): the page never computes paths. The keys and the selection are in tiepickrules.js. The picker only
// chooses: tieflow.js sends the take.
//
// Look: the source on the left, then one step for each tie line, then the destination. Each step lists its choices.
// A choice has a wire: the chosen one is lit with --pb-tieline (docs/color-roles.md). A Protected or Reserved choice is
// grey and says why. Under the steps are the Affected destinations of the chosen path.
//
// Live update: the server pushes each change that can change the plan (a tally, a Protect, a router status, the tie
// lines). While the picker is open, each one makes the page ask for the plan again with tiePlan. A change on the chosen
// path starts the lockout again and marks the changed rows (refresh). The selection never moves by itself. The server
// keeps nothing for an open picker.
//
// Protect (docs/tielines-design.md, Protect in a tie take): a person with the Protect right presses P or the shield
// of a step. The take then asks Sode to protect the upstream destination of that step's tie line after every hop
// (tieprotect.go). Without the right, the shield and the key hint are hidden and P does nothing.
//
// No path: the picker opens with each reason by name (The Tie line picker, rule 10), and Take stays off. A route that
// comes back on the live update makes the choices show.
//
// One-off tie line: a person types a destination on the router of the source and a source on the router of the
// destination. The plan then has a path through that One-off tie line, and the picker chooses it. Its form stays out
// of the painted plan, so a live update keeps what the person typed.

import { el, on, icon, request, srcItem, dstItem, labelNode, routerData, tallyText } from './core.js';
import { chord, keyLabel } from './keys.js';
import { trapDialog } from './dialogkeys.js';
import { lockout } from './lockout.js';
import { fmtClock } from './device.js';
import { showResult } from './banner.js';
import { levelsPhrase } from './levelview.js';
import {
  chosen,
  choices,
  choose,
  keyStep,
  ready,
  changedText,
  shownSteps,
  memberChanges,
  countUp,
  refresh,
  tieMarked,
  hitMarked,
  pathUsable,
  heldText,
  reserveAsk,
  confirmReserve,
  reservedHops,
  reserveText,
  pathKey,
  protectable,
  protectOn,
  toggleProtect,
  usedLast,
  choiceLook,
} from './tiepickrules.js';
import { withOneOffs, oneOffEnds, addOneOff, chooseOneOff, storedText, chainWords, feedingHop, destsWord } from './tiepickrules.js';
import { mayProtect } from './protects.js';
import { reasonText } from './tietakerules.js';

let ui = null; // { root, dialog, head, body, plan, one, clear, split, take, trap, pKey, dKey }
// { st, o, oneOffs, plans, hits, lock, unlocked, timer, wait, asking, again }. o: the options of pickRoute. oneOffs:
// the One-off tie lines that the person stated ({from, to}). plans: the routes of the last 409 body. hits: the
// members whose path the live update found changed. wait: the timer of the next plan; asking: a plan is on its way;
// again: a change came while it was.
let cur = null;

// REPLAN_MS joins the changes of one burst (a take moves many crosspoints) into one plan.
const REPLAN_MS = 150;

const srcOf = (e) => srcItem(e.n, e.name || '', e.router);
const dstOf = (e) => dstItem(e.n, e.name || '', e.router);
const prefix = (uid) => routerData(uid).prefix;

// chainText says what a tie line carries now (Via, docs/tielines-design.md).
const chainText = (c) => chainWords(c, (o) => srcOf(o).label, prefix);

function build() {
  const title = el('span', 'pb-modal__title');
  title.id = 'tiepick-title';
  title.textContent = 'Tie line picker';
  const head = el('span', 'sode-tiepick__count');
  const plan = el('div', 'sode-tiepick__plan');
  const one = oneOffForm();
  const body = el('div', 'pb-modal__body sode-tiepick__body', plan, one.node);
  const clear = el('button', 'pb-btn pb-btn--outline');
  clear.type = 'button';
  clear.id = 'tiepick-clear';
  clear.textContent = 'Clear this route';
  const cancel = el('button', 'pb-btn pb-btn--outline');
  cancel.type = 'button';
  cancel.id = 'tiepick-cancel';
  cancel.textContent = 'Cancel';
  const split = el('button', 'pb-btn pb-btn--outline');
  split.type = 'button';
  split.id = 'tiepick-split';
  split.textContent = 'Choose per destination';
  split.dataset.pbTip = `Give each destination of this hop its own choice (${keyLabel('d')})`;
  const pKey = el('span', null, ', ', el('kbd', null, keyLabel('p')), ' protect');
  const dKey = el('span', null, ', ', el('kbd', null, keyLabel('d')), ' per destination');
  const take = el('button', 'pb-btn pb-btn--commit sode-tiepick__take');
  take.type = 'button';
  take.id = 'tiepick-take';
  const dialog = el(
    'div',
    'pb-modal sode-tiepick',
    el('div', 'pb-modal__header', title, head),
    body,
    el('div', 'pb-modal__footer', keyHints(pKey, dKey), split, clear, cancel, take),
  );
  Object.assign(dialog, { id: 'tiepick-dialog', tabIndex: -1 });
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'tiepick-title');
  const root = el('div', 'pb-modal-backdrop sode__scrim', dialog);
  root.id = 'tiepick';
  root.hidden = true;
  document.body.append(root);
  // The keys go to the picker before the trap: Tab moves between steps here.
  dialog.addEventListener('keydown', onKey);
  const trap = trapDialog(root, dialog, () => close());
  cancel.addEventListener('click', () => close());
  take.addEventListener('click', () => send());
  clear.addEventListener('click', () => clearRoute());
  split.addEventListener('click', () => splitHop());
  ui = { root, dialog, head, body, plan, one, clear, split, take, trap, pKey, dKey };
}

// oneOffForm: the One-off tie line button and its form. The labels name the routers when the picker opens.
function oneOffForm() {
  const toggle = el('button', 'pb-btn pb-btn--outline pb-btn--sm');
  toggle.type = 'button';
  toggle.id = 'tiepick-oneoff';
  toggle.textContent = 'One-off tie line';
  toggle.dataset.pbTip = 'Use a cable that is not a tie line in Sode, for this take only';
  const field = (id) => {
    const label = el('label');
    label.htmlFor = id;
    const input = el('input', 'pb-input');
    Object.assign(input, { id, type: 'number', min: '0' });
    return { label, input, node: el('div', 'sode__field', label, el('span', 'pb-number-wrap', input)) };
  };
  const from = field('tiepick-oneoff-from');
  const to = field('tiepick-oneoff-to');
  const use = el('button', 'pb-btn pb-btn--outline pb-btn--sm');
  use.type = 'button';
  use.id = 'tiepick-oneoff-use';
  use.textContent = 'Use';
  const why = el('p', 'sode__hint');
  why.id = 'tiepick-oneoff-why';
  why.setAttribute('role', 'status');
  const form = el(
    'div',
    'sode-tiepick__oneoff-form',
    el('p', 'sode__hint', 'Sode does not know what this cable feeds on the far router, so the Affected destinations there show as unknown.'),
    el('div', 'sode__fields', from.node, to.node),
    use,
    why,
  );
  form.id = 'tiepick-oneoff-form';
  form.hidden = true;
  toggle.setAttribute('aria-expanded', 'false');
  toggle.setAttribute('aria-controls', form.id);
  toggle.addEventListener('click', () => showOneOff(form.hidden));
  use.addEventListener('click', () => useOneOff());
  return { node: el('div', 'sode-tiepick__oneoff', toggle, form), toggle, form, from, to, why };
}

function showOneOff(show) {
  const { one } = ui;
  one.form.hidden = !show;
  one.toggle.setAttribute('aria-expanded', String(show));
  if (show) one.from.input.focus();
  else ui.dialog.focus({ preventScroll: true });
}

// resetOneOff clears the form for the route of a new picker.
function resetOneOff(route) {
  const { one } = ui;
  one.from.label.textContent = `Destination on Router ${prefix(route.src.router)}`;
  one.to.label.textContent = `Source on Router ${prefix(route.dst.router)}`;
  one.from.input.value = '';
  one.to.input.value = '';
  one.why.textContent = '';
  one.form.hidden = true;
  one.toggle.setAttribute('aria-expanded', 'false');
}

// useOneOff asks for the plan with the typed One-off tie line and chooses the path through it.
async function useOneOff() {
  const c = cur;
  if (!c) return;
  const { one } = ui;
  const r = oneOffEnds(c.st.route, one.from.input.value, one.to.input.value, (uid) => routerData(uid).off);
  if (r.why) {
    one.why.textContent = `${r.why}.`;
    return;
  }
  one.why.textContent = 'Asking the server…';
  const list = addOneOff(c.oneOffs, r.ends);
  const { route, m } = await askPlan(withOneOffs(c.o.routes(), list), c.o.at);
  if (cur !== c) return;
  if (!route) {
    one.why.textContent = m.body?.error?.message || m.err || 'The server refused the One-off tie line.';
    return;
  }
  c.oneOffs = list;
  c.st = chooseOneOff(refresh(c.st, route).st, r.ends);
  one.why.textContent = '';
  startLock();
  paint();
  showOneOff(false);
}

// pKey: the hint of P, which shows only with the Protect right. dKey: the hint of D, which shows only on a question
// with more than one destination.
function keyHints(pKey, dKey) {
  const k = (spec) => el('kbd', null, keyLabel(spec));
  return el('span', 'sode-tiepick__keys', k('j'), ' ', k('k'), ' choose, ', k('h'), ' ', k('l'), ' step', pKey, dKey, ', ', k('escape'), ' cancel');
}

// mayShield: the person has the Protect right, and the picker can send a Protect (not the preset editor's).
const mayShield = () => mayProtect() && !cur?.o.use;

function onKey(e) {
  if (!cur) return;
  // In the One-off tie line form, the keys type. Enter uses the cable.
  if (e.target.closest?.('.sode-tiepick__oneoff-form')) {
    if (chord(e) === 'enter') {
      e.preventDefault();
      e.stopPropagation();
      useOneOff();
    }
    return;
  }
  const c = chord(e);
  const { st, act } = keyStep(cur.st, c, mayShield());
  if ((st === cur.st && !act) || (act === 'split' && !cur.o.split)) return;
  e.preventDefault();
  e.stopPropagation();
  if (act === 'split') return splitHop();
  if (act === 'cancel') return close();
  if (act === 'send') return send();
  cur.st = st;
  paint();
}

// pickRoute asks the server for the plan of one route and opens the picker on it. o:
//   routes(): the routes of the plan request (tietakerules.js, planRequest). at: the place in it of the route that
//   the picker shows; none for a request of one route.
//   first: { route, all }, a plan that the caller has already, so the picker does not ask again.
//   members: the routes of the question ({ at, dst }), the route at first. Their destinations show, and a change on
//   any of them marks the hop. None: the destination of the route.
//   state(route): the picker's state from the route of the 409 body (pickState or reopenState).
//   head: the hop of the question and its destinations ("A → B, for B.1 B.2"), or a function of the state that gives
//   it, or ''.
//   final: Take sends the take; else it goes on to the next step of the take ("Next").
//   onPick(st, oneOffs): the person chose a path. clear(): Clear this route, or null for none; clearText its word.
//   split(): give each destination its own question, or null. onClose(): a cancel.
//   oneOffs: the One-off tie lines that the person stated before ({from, to}), or none.
//   use: the word of the button that chooses the path, in place of Take or Next. The preset editor uses it: the picker
//   then only chooses saved tie lines, so it has no One-off tie line form and no Protect.
export async function pickRoute(o) {
  const { route, all, m } = o.first?.route ? o.first : await askPlan(withOneOffs(o.routes(), o.oneOffs), o.at);
  if (route) return open(o, o.state(route), all);
  showResult('error', 'Not sent', m.body?.error?.message || m.err || 'The server refused the route.');
  o.onClose?.();
}

// askPlan asks the server for the plan of routes, with no take. It gives { route, all }. route is the route of the
// 409 body at place at of the request, and all are every route of the body. With no at, the body must have one
// route. When the server gave no plan, it gives { m } with the answer.
async function askPlan(routes, at) {
  const m = await request('tiePlanResult', { t: 'tiePlan', routes });
  const code = m.body?.error?.code;
  const all = m.body?.routes || [];
  const route = at == null ? (all.length === 1 ? all[0] : null) : all.find((r) => r.route === at);
  if (m.status === 409 && (code === 'tie_line_needed' || code === 'no_path') && route) return { route, all };
  return { m };
}

// replan asks for the plan again while the picker is open. A change that comes while a plan is on its way asks again
// after it.
function replan() {
  const c = cur;
  if (!c || c.wait) return;
  c.wait = setTimeout(async () => {
    c.wait = 0;
    if (c.asking) {
      c.again = true;
      return;
    }
    c.asking = true;
    const { route, all } = await askPlan(withOneOffs(c.o.routes(), c.oneOffs), c.o.at);
    c.asking = false;
    if (cur !== c) return;
    if (route) {
      const { st, changed } = refresh(c.st, route);
      // A change on the path of another route of the question marks its destination.
      const p = chosen(st);
      const prefix = (p?.hops || []).slice(0, shownSteps(st).to + 1).map((h) => h.tie?.id || '');
      const others = (c.o.members || []).slice(1).map((x) => x.at);
      const hit = p ? memberChanges(c.plans, all, others, prefix) : [];
      c.plans = all;
      c.hits = [...new Set([...c.hits, ...hit])];
      c.st = st;
      if (changed || hit.length) startLock();
      paint();
    }
    if (c.again) {
      c.again = false;
      replan();
    }
  }, REPLAN_MS);
}

for (const ev of ['tally', 'protectedRows', 'protectedOther', 'conn', 'tieLines']) on(ev, replan);

// startLock starts the lockout of the open picker again: Take waits until it ends.
function startLock() {
  const c = cur;
  c.lock?.cancel();
  c.unlocked = false;
  c.lock = lockout(() => {
    if (cur !== c) return;
    c.unlocked = true;
    paintTake();
  });
}

function open(o, st, plans) {
  if (!ui) build();
  stop();
  cur = { st, o, oneOffs: o.oneOffs || [], plans, hits: [], unlocked: false, timer: countUp(tick) };
  resetOneOff(st.route);
  ui.clear.hidden = !o.clear;
  ui.clear.textContent = o.clearText || 'Clear this route';
  ui.split.hidden = !o.split;
  ui.dKey.hidden = !o.split;
  ui.take.textContent = `${o.use || (o.final ? 'Take' : 'Next')} (${keyLabel('enter')})`;
  ui.one.node.hidden = !!o.use;
  startLock();
  paint();
  ui.trap.show();
  ui.dialog.focus();
}

// stop ends the lockout, the timer and a waiting plan of the open picker.
function stop() {
  if (!cur) return;
  cur.lock?.cancel();
  cur.timer.stop();
  clearTimeout(cur.wait);
  cur.wait = 0;
}

function close(sent = false) {
  if (!cur) return;
  const c = cur;
  stop();
  cur = null;
  ui.trap.hide();
  if (!sent) c.o.onClose?.();
}

// tiePickShown: the picker is open (the page leaves its keys to it).
export const tiePickShown = () => cur != null;

function paintTake() {
  ui.take.disabled = !cur.unlocked || !ready(cur.st);
}

function tick() {
  if (!cur) return;
  const now = Date.now();
  for (const n of ui.body.querySelectorAll('[data-changed]')) {
    n.textContent = changedText({ changed: n.dataset.changed, before: n.dataset.before === '1' }, now, fmtClock);
  }
}

function paint() {
  const { st, o } = cur;
  const { route } = st;
  const p = chosen(st);
  const src = srcOf(route.src);
  const dst = dstOf(route.dst);
  const steps = [];
  const shown = shownSteps(st);
  for (let s = shown.from; s <= shown.to; s++) steps.push(stepNode(s, src));
  ui.head.textContent = typeof o.head === 'function' ? o.head(st) : o.head || '';
  const dsts = o.members?.length ? o.members.map((x) => x.dst) : [dst];
  const to = dsts.flatMap((d, i) => (i ? [', ', labelNode(d)] : [labelNode(d)]));
  ui.plan.replaceChildren(
    el('div', 'sode-tiepick__route', 'Take ', labelNode(src), ' to ', ...to, `, ${levelsPhrase(route.levels, dst.router || '')}`),
    ...reasonsNode(route),
    el('div', 'sode-tiepick__map', endNode('Source', src, 'sode-tiepick__end--src'), ...steps, dstsNode(st)),
    ...(p ? [affectedNode(p, src)] : []),
    ...reserveNode(st),
    ...storedNode(st),
    ...lastNode(st),
  );
  ui.pKey.hidden = !mayShield();
  paintTake();
}

// reserveNode is the extra confirm of a Reserved tie line: it shows after the first Take, until the path changes.
function reserveNode(st) {
  if (!reservedHops(st).length || st.reserveOk !== pathKey(chosen(st))) return [];
  const n = el('div', 'pb-alert pb-alert--warn sode-tiepick__reserve', reserveText(st));
  n.setAttribute('role', 'alert');
  return [n];
}

// storedNode says what the tie confirm page of a preset found on its stored tie lines. A caution when the person
// must choose again, with text.
function storedNode(st) {
  const { text, warn } = storedText(st);
  if (!text) return [];
  const n = warn ? el('div', 'pb-alert pb-alert--warn sode-tiepick__stored', text) : el('p', 'sode__hint sode-tiepick__stored', text);
  n.setAttribute('role', 'status');
  return [n];
}

// lastNode says why a Redo could not keep a tie line that the take used last time (lastState). A caution, with text.
function lastNode(st) {
  if (!st.lastWhy) return [];
  const n = el('div', 'pb-alert pb-alert--warn sode-tiepick__stored', st.lastWhy);
  n.setAttribute('role', 'status');
  return [n];
}

// reasonsNode names each reason when no path is usable (The Tie line picker, rule 10). An error alert, with text.
function reasonsNode(route) {
  if (route.paths.some(pathUsable)) return [];
  const list = route.reasons?.length ? route.reasons.map((x) => reasonText(x, prefix)) : ['No tie line path is usable'];
  return [
    el(
      'div',
      'pb-alert pb-alert--error sode-tiepick__reasons',
      el('div', 'pb-alert__title', 'No usable path'),
      el('ul', 'sode-tiepick__why', ...list.map((t) => el('li', null, t))),
    ),
  ];
}

function endNode(word, it, cls, marked = false) {
  const word2 = el('span', 'sode-tiepick__word', `${word} on Router ${prefix(it.router || '')}`);
  return el('div', `sode-tiepick__end ${cls}`, word2, labelNode(it), ...(marked ? [changedMark()] : []));
}

// dstsNode shows the destinations of the question. The destination of the route that the picker shows is marked from
// its own path (st.marks.end), the others from memberChanges.
function dstsNode(st) {
  const { o } = cur;
  if (!o.members?.length) return endNode('Destination', dstOf(st.route.dst), 'sode-tiepick__end--dst', st.marks?.end);
  const routers = [...new Set(o.members.map((x) => prefix(x.dst.router || '')))];
  const word = el('span', 'sode-tiepick__word', destsWord(o.members.length, routers));
  const rows = o.members.map((x, i) => {
    const marked = i === 0 ? st.marks?.end : cur.hits.includes(x.at);
    return el('div', 'sode-tiepick__dst', labelNode(x.dst), ...(marked ? [changedMark()] : []));
  });
  return el('div', 'sode-tiepick__end sode-tiepick__end--dst', word, ...rows);
}

// changedMark marks a row that the live update or a refused take found changed. Caution color, with text.
const changedMark = (text = 'Changed') => el('span', 'pb-badge pb-badge--warn sode-tiepick__mark', text);

function stepNode(s, src) {
  const { st } = cur;
  const p = chosen(st);
  const hop = p.hops[s];
  const next = p.hops[s + 1];
  const list = el('div', 'sode-tiepick__choices');
  list.setAttribute('role', 'listbox');
  list.setAttribute('aria-label', `Tie lines from Router ${prefix(hop.router)} to Router ${prefix(next.router)}`);
  const cs = choices(st, s);
  for (const c of cs) list.append(choiceNode(s, c, c.tie === hop.tie.id, src, cs));
  const node = el(
    'div',
    'sode-tiepick__step',
    el(
      'div',
      'sode-tiepick__stephead',
      el('span', 'sode-tiepick__word', `Step ${s + 1}: Router ${prefix(hop.router)} to Router ${prefix(next.router)}`),
      ...(mayShield() ? [shieldNode(s, hop)] : []),
    ),
    list,
  );
  node.classList.toggle('sode-tiepick__step--focus', s === st.step);
  return node;
}

// shieldNode is the Protect button of step s. It acts on the chosen tie line of the step. A tie line that has a
// Protect already cannot get one, so its button is off.
function shieldNode(s, hop) {
  const id = hop.tie.id;
  const name = hop.tie.name || id;
  const pressed = protectOn(cur.st, id) && protectable(hop);
  const b = el(
    'button',
    'pb-btn pb-btn--outline pb-btn--sm sode-tiepick__shield',
    icon(pressed ? 'shield-check' : 'shield'),
    pressed ? `Protect ${name}: on` : `Protect ${name}`,
  );
  b.type = 'button';
  b.dataset.tie = id;
  b.disabled = !protectable(hop);
  b.setAttribute('aria-pressed', pressed ? 'true' : 'false');
  b.dataset.pbTip = `Protect the upstream destination of ${name} after the take (${keyLabel('p')})`;
  b.classList.add('pb-tip--end');
  b.tabIndex = -1;
  b.addEventListener('pointerdown', (e) => e.preventDefault());
  b.addEventListener('click', () => {
    if (!cur) return;
    cur.st = toggleProtect({ ...cur.st, step: s }, s);
    paint();
    ui.dialog.focus({ preventScroll: true });
  });
  return b;
}

// LOOKS gives the class of each state of choiceLook. Each class is a literal, so the color checks see it
// (web/test/colorroles.test.mjs).
const LOOKS = {
  held: 'sode-tiepick__choice--grey',
  has: 'sode-tiepick__choice--has',
  change: 'sode-tiepick__choice--change',
  free: 'sode-tiepick__choice--free',
};

// lookNodes are the words of the state of a choice. "Has it" has a check, and "3 change" is a caution badge.
// "Free" is text, and "Best" is a neutral badge.
function lookNodes(look) {
  const out = [];
  if (look.state === 'has') out.push(el('span', 'sode-tiepick__has', icon('check'), look.text));
  if (look.state === 'change') out.push(el('span', 'pb-badge pb-badge--warn sode-tiepick__changes', look.text));
  if (look.state === 'free') out.push(el('span', 'sode-tiepick__free', look.text));
  if (look.best) out.push(el('span', 'pb-badge sode-tiepick__best', 'Best'));
  return out;
}

function choiceNode(s, c, on, src, cs) {
  const h = c.hop;
  const look = choiceLook(cur.st, s, c, cs);
  const will = h.sends ? `Will carry ${src.label}` : 'Carries the source now';
  const held = heldText(h, fmtClock);
  const changed = el('span', 'sode-tiepick__ago', changedText(h, Date.now(), fmtClock));
  if (h.changed) {
    changed.dataset.changed = h.changed;
    changed.dataset.before = h.before ? '1' : '0';
  }
  // The wire has no words of its own, so its tip says what a lit wire means.
  const wire = el('span', 'sode-tiepick__wire');
  wire.dataset.pbTip = on ? 'Chosen tie line' : 'Choose this tie line';
  const b = el(
    'button',
    'sode-tiepick__choice',
    wire,
    el('span', 'sode-tiepick__tierow', el('span', 'sode-tiepick__tie', c.name), ...lookNodes(look)),
    el('span', 'sode-tiepick__now', `Now ${chainText(h.now)}`),
    el('span', 'sode-tiepick__will', will),
    changed,
    ...(held ? [el('span', 'sode-tiepick__held', held)] : []),
    ...(on && protectOn(cur.st, c.tie) && protectable(h) ? [el('span', 'sode-tiepick__protect', icon('shield'), 'Protect after the take')] : []),
    ...(c.gone ? [el('span', 'sode-tiepick__held', 'Not in the plan now')] : []),
    ...(usedLast(cur.st, s, c.tie) ? [el('span', 'pb-badge sode-tiepick__last', 'Used last time')] : []),
    ...(on && tieMarked(cur.st, c.tie) ? [changedMark()] : []),
  );
  b.type = 'button';
  b.dataset.tie = c.tie;
  b.setAttribute('role', 'option');
  b.setAttribute('aria-selected', on ? 'true' : 'false');
  b.classList.toggle('sode-tiepick__choice--on', on);
  b.classList.toggle('sode-tiepick__choice--held', c.held);
  b.classList.add(LOOKS[look.state]);
  b.classList.toggle('sode-tiepick__choice--best', look.best);
  // The selection is the focus of the list: a press keeps the focus on the dialog, so the keys stay with the picker.
  b.tabIndex = -1;
  b.addEventListener('pointerdown', (e) => e.preventDefault());
  b.addEventListener('click', () => {
    if (!cur) return;
    cur.st = choose(cur.st, s, c.tie);
    paint();
    ui.dialog.focus({ preventScroll: true });
  });
  return b;
}

// What an Affected destination shows now. When it takes the downstream end of a tie line on the chosen path, it shows
// what that tie line carries. Else it shows the tally of its router.
function nowShows(e, p) {
  const t = routerData(e.router).tally.get(e.n);
  const s = t ? [...t.values()][0] : null;
  const i = feedingHop(p, e.router, s);
  return i >= 0 ? chainText(p.hops[i].now) : tallyText(s, e.router);
}

function affectedNode(p, src) {
  const rows = p.affected.map((e) =>
    el(
      'li',
      'sode-tiepick__hit',
      labelNode(dstOf(e)),
      `: now shows ${nowShows(e, p)}, will show ${src.label}`,
      ...(hitMarked(cur.st, e) ? [' ', changedMark('New')] : []),
    ),
  );
  const extra = [];
  if (p.others) extra.push(el('li', 'sode-tiepick__hit', `${p.others} more that you cannot see`));
  for (const u of p.unknown || []) {
    const what =
      u.why === 'one_off' ? 'unknown downstream of a One-off tie line' : `unknown (router ${prefix(u.router)} ${u.why === 'down' ? 'down' : 'not read'})`;
    extra.push(el('li', 'sode-tiepick__hit', what));
  }
  if (p.loop) extra.push(el('li', 'sode-tiepick__hit', '∞ the chain comes back to a tie line'));
  const n = p.affected.length + (p.others || 0);
  const head = el('span', 'sode-tiepick__word', n ? `Affected destinations: ${n}` : 'Affected destinations: none');
  const mark = cur.st.marks?.affected ? [changedMark()] : [];
  return el('div', 'sode-tiepick__affected', el('div', 'sode-tiepick__head', head, ...mark), el('ul', 'sode-tiepick__hits', ...rows, ...extra));
}

// send: the person chose the path. tieflow.js sends the take. A Reserved tie line asks one more time first, and the
// lockout starts again.
function send() {
  if (!cur?.unlocked || !ready(cur.st)) return;
  if (reserveAsk(cur.st)) {
    cur.st = confirmReserve(cur.st);
    startLock();
    return paint();
  }
  const { st, o, oneOffs } = cur;
  close(true);
  o.onPick(st, oneOffs);
}

// splitHop gives each destination of the question its own question.
function splitHop() {
  const o = cur?.o;
  if (!o?.split) return;
  close(true);
  o.split();
}

function clearRoute() {
  const o = cur?.o;
  if (!o?.clear) return;
  close(true);
  o.clear();
}

// A Lock closes the picker, as it closes the take Confirm.
on('locking', () => close());
