// The Tie line picker (docs/tielines-design.md, The Tie line picker; GLOSSARY.md). It opens for each route across
// routers of a take, one after the other (tieflow.js). The server gives the ranked paths (tiepage.go, the message
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
// No path: the picker opens with each reason by name (The Tie line picker, rule 10), and Take stays off. A route that
// comes back on the live update makes the choices show.
//
// One-off tie line: a person types a destination on the router of the source and a source on the router of the
// destination. The plan then has a path through that One-off tie line, and the picker chooses it. Its form stays out
// of the painted plan, so a live update keeps what the person typed.

import { el, on, request, srcItem, dstItem, labelNode, routerData, tallyText } from './core.js';
import { chord, keyLabel } from './keys.js';
import { trapDialog } from './dialogkeys.js';
import { lockout } from './lockout.js';
import { fmtClock } from './device.js';
import { showResult } from './banner.js';
import { levelsPhrase } from './levelview.js';
import { chosen, choices, choose, keyStep, ready, changedText, stepsOf, countUp, refresh, tieMarked, hitMarked, pathUsable } from './tiepickrules.js';
import { withOneOffs, oneOffEnds, addOneOff, chooseOneOff } from './tiepickrules.js';
import { reasonText } from './tietakerules.js';

let ui = null; // { root, dialog, head, body, plan, one, clear, take, trap }
// { st, o, oneOffs, lock, unlocked, timer, wait, asking, again }. o: the options of pickRoute. oneOffs: the One-off
// tie lines that the person stated ({from, to}). wait: the timer of the next plan; asking: a plan is on its way;
// again: a change came while it was.
let cur = null;

// REPLAN_MS joins the changes of one burst (a take moves many crosspoints) into one plan.
const REPLAN_MS = 150;

const srcOf = (e) => srcItem(e.n, e.name || '', e.router);
const dstOf = (e) => dstItem(e.n, e.name || '', e.router);
const prefix = (uid) => routerData(uid).prefix;

// chainText says what a tie line carries now (Via, docs/tielines-design.md).
function chainText(c) {
  if (!c) return 'unknown';
  if (c.loop) return '∞';
  if (c.unknown) return `unknown (router ${prefix(c.unknown)} down)`;
  if (c.nothing || !c.origin) return 'nothing';
  const via = c.via?.length ? ` Via ${c.via.map((v) => v.name || v.id).join(', ')}` : '';
  return `${srcOf(c.origin).label}${via}${c.mixed ? ' (mixed)' : ''}`;
}

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
  const take = el('button', 'pb-btn pb-btn--commit sode-tiepick__take');
  take.type = 'button';
  take.id = 'tiepick-take';
  const dialog = el(
    'div',
    'pb-modal sode-tiepick',
    el('div', 'pb-modal__header', title, head),
    body,
    el('div', 'pb-modal__footer', keyHints(), clear, cancel, take),
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
  ui = { root, dialog, head, body, plan, one, clear, take, trap };
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
  one.from.label.textContent = `Destination on router ${prefix(route.src.router)}`;
  one.to.label.textContent = `Source on router ${prefix(route.dst.router)}`;
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
  const { route, m } = await askPlan(withOneOffs(c.o.routes(), list));
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

function keyHints() {
  const k = (spec) => el('kbd', null, keyLabel(spec));
  return el('span', 'sode-tiepick__keys', k('j'), ' ', k('k'), ' choose, ', k('h'), ' ', k('l'), ' step, ', k('escape'), ' cancel');
}

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
  const { st, act } = keyStep(cur.st, c);
  if (st === cur.st && !act) return;
  e.preventDefault();
  e.stopPropagation();
  if (act === 'cancel') return close();
  if (act === 'send') return send();
  cur.st = st;
  paint();
}

// pickRoute asks the server for the plan of one route and opens the picker on it. o:
//   routes(): the routes of the plan request (tietakerules.js, planRequest); one of them has no ties.
//   state(route): the picker's state from the route of the 409 body (pickState or reopenState).
//   head: the place of the route in the take ("Route 1 of 2"), or ''.
//   final: Take sends the take; else it goes on to the next step of the take ("Next").
//   onPick(st): the person chose a path. clear(): Clear this route, or null for none. onClose(): a cancel.
//   oneOffs: the One-off tie lines that the person stated before ({from, to}), or none.
export async function pickRoute(o) {
  const { route, m } = await askPlan(withOneOffs(o.routes(), o.oneOffs));
  if (route) return open(o, o.state(route));
  showResult('error', 'Not sent', m.body?.error?.message || m.err || 'The server refused the route.');
  o.onClose?.();
}

// askPlan asks the server for the plan of routes, with no take. It gives { route } with the one route of the 409
// body. When the server gave no plan, it gives { m } with the answer.
async function askPlan(routes) {
  const m = await request('tiePlanResult', { t: 'tiePlan', routes });
  const code = m.body?.error?.code;
  if (m.status === 409 && (code === 'tie_line_needed' || code === 'no_path') && m.body.routes?.length === 1) return { route: m.body.routes[0] };
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
    const { route } = await askPlan(withOneOffs(c.o.routes(), c.oneOffs));
    c.asking = false;
    if (cur !== c) return;
    if (route) {
      const { st, changed } = refresh(c.st, route);
      c.st = st;
      if (changed) startLock();
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

function open(o, st) {
  if (!ui) build();
  stop();
  cur = { st, o, oneOffs: o.oneOffs || [], unlocked: false, timer: countUp(tick) };
  resetOneOff(st.route);
  ui.head.textContent = o.head || '';
  ui.clear.hidden = !o.clear;
  ui.take.textContent = `${o.final ? 'Take' : 'Next'} (${keyLabel('enter')})`;
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
  const { st } = cur;
  const { route } = st;
  const p = chosen(st);
  const src = srcOf(route.src);
  const dst = dstOf(route.dst);
  const steps = [];
  for (let s = 0; s < stepsOf(p); s++) steps.push(stepNode(s, src));
  ui.plan.replaceChildren(
    el('div', 'sode-tiepick__route', 'Take ', labelNode(src), ' to ', labelNode(dst), `, ${levelsPhrase(route.levels, dst.router || '')}`),
    ...reasonsNode(route),
    el(
      'div',
      'sode-tiepick__map',
      endNode('Source', src, 'sode-tiepick__end--src'),
      ...steps,
      endNode('Destination', dst, 'sode-tiepick__end--dst', st.marks?.end),
    ),
    ...(p ? [affectedNode(p, src)] : []),
  );
  paintTake();
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
  const word2 = el('span', 'sode-tiepick__word', `${word}, router ${prefix(it.router || '')}`);
  return el('div', `sode-tiepick__end ${cls}`, word2, labelNode(it), ...(marked ? [changedMark()] : []));
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
  list.setAttribute('aria-label', `Tie lines from router ${prefix(hop.router)} to router ${prefix(next.router)}`);
  for (const c of choices(st, s)) list.append(choiceNode(s, c, c.tie === hop.tie.id, src));
  const node = el(
    'div',
    'sode-tiepick__step',
    el('span', 'sode-tiepick__word', `Step ${s + 1}: router ${prefix(hop.router)} to router ${prefix(next.router)}`),
    list,
  );
  node.classList.toggle('sode-tiepick__step--focus', s === st.step);
  return node;
}

function choiceNode(s, c, on, src) {
  const h = c.hop;
  const will = h.sends ? `Will carry ${src.label}` : 'Carries the source now';
  const held = h.protect ? `Protected by ${h.protect}` : h.reserve ? `Reserved by panel ${h.reserve}` : '';
  const changed = el('span', 'sode-tiepick__ago', changedText(h, Date.now(), fmtClock));
  if (h.changed) {
    changed.dataset.changed = h.changed;
    changed.dataset.before = h.before ? '1' : '0';
  }
  const b = el(
    'button',
    'sode-tiepick__choice',
    el('span', 'sode-tiepick__wire'),
    el('span', 'sode-tiepick__tie', c.name),
    el('span', 'sode-tiepick__now', `Now ${chainText(h.now)}`),
    el('span', 'sode-tiepick__will', will),
    changed,
    ...(held && h.sends ? [el('span', 'sode-tiepick__held', held)] : []),
    ...(c.gone ? [el('span', 'sode-tiepick__held', 'Not in the plan now')] : []),
    ...(on && tieMarked(cur.st, c.tie) ? [changedMark()] : []),
  );
  b.type = 'button';
  b.dataset.tie = c.tie;
  b.setAttribute('role', 'option');
  b.setAttribute('aria-selected', on ? 'true' : 'false');
  b.classList.toggle('sode-tiepick__choice--on', on);
  b.classList.toggle('sode-tiepick__choice--held', c.held);
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
  for (let i = 0; i + 1 < p.hops.length; i++) {
    const down = p.hops[i + 1];
    if (down.router === e.router && down.src.n === s) return chainText(p.hops[i].now);
  }
  return tallyText(s, e.router);
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

// send: the person chose the path. tieflow.js sends the take.
function send() {
  if (!cur?.unlocked || !ready(cur.st)) return;
  const { st, o } = cur;
  close(true);
  o.onPick(st);
}

function clearRoute() {
  const o = cur?.o;
  if (!o?.clear) return;
  close(true);
  o.clear();
}

// A Lock closes the picker, as it closes the take Confirm.
on('locking', () => close());
