// The Tie line picker (docs/tielines-design.md, The Tie line picker; GLOSSARY.md). It opens before a take of one route
// across routers, and it is the only confirm of that take. The server gives the ranked paths (tiepage.go, the message
// tieTake): the page never computes paths. The keys and the selection are in tiepickrules.js.
//
// Look: the source on the left, then one step for each tie line, then the destination. Each step lists its choices.
// A choice has a wire: the chosen one is lit with --pb-tieline (docs/color-roles.md). A Protected or Reserved choice is
// grey and says why. Under the steps are the Affected destinations of the chosen path.
//
// Live update: while the picker is open, each change that the server pushes and that can change the plan (a tally,
// a Protect, a router status, the tie lines) makes the page ask for the plan again with tiePlan. A change on the chosen
// path starts the lockout again and marks the changed rows (refresh). The selection never moves by itself. The server
// keeps nothing for an open picker. A take that gets 409 tie_line_changed opens the picker again on the new plan.

import { el, on, emit, request, srcItem, dstItem, labelNode, routerData, tallyText } from './core.js';
import { chord, keyLabel } from './keys.js';
import { trapDialog } from './dialogkeys.js';
import { lockout } from './lockout.js';
import { fmtClock } from './device.js';
import { showResult } from './banner.js';
import { noteLast } from './plans.js';
import { levelsPhrase } from './levelview.js';
import {
  pickState,
  chosen,
  choices,
  choose,
  keyStep,
  ready,
  takeRoute,
  changedText,
  stepsOf,
  countUp,
  refresh,
  reopenState,
  tieMarked,
  hitMarked,
} from './tiepickrules.js';

let ui = null; // { root, dialog, body, take, trap }
// { st, plan, item, opts, lock, unlocked, timer, wait, asking, again }. wait: the timer of the next plan; asking: a
// plan is on its way; again: a change came while it was.
let cur = null;

// REPLAN_MS joins the changes of one burst (a take moves many crosspoints) into one plan.
const REPLAN_MS = 150;

const srcOf = (e) => srcItem(e.n, e.name || '', e.router);
const dstOf = (e) => dstItem(e.n, e.name || '', e.router);
const ref = (it) => ({ router: routerData(it.router || '').uid, n: it.n });
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
  const body = el('div', 'pb-modal__body sode-tiepick__body');
  const cancel = el('button', 'pb-btn pb-btn--outline');
  cancel.type = 'button';
  cancel.id = 'tiepick-cancel';
  cancel.textContent = 'Cancel';
  const take = el('button', 'pb-btn pb-btn--commit sode-tiepick__take');
  take.type = 'button';
  take.id = 'tiepick-take';
  take.textContent = `Take (${keyLabel('enter')})`;
  const dialog = el('div', 'pb-modal sode-tiepick', el('div', 'pb-modal__header', title), body, el('div', 'pb-modal__footer', keyHints(), cancel, take));
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
  ui = { root, dialog, body, take, trap };
}

function keyHints() {
  const k = (spec) => el('kbd', null, keyLabel(spec));
  return el('span', 'sode-tiepick__keys', k('j'), ' ', k('k'), ' choose, ', k('h'), ' ', k('l'), ' step, ', k('escape'), ' cancel');
}

function onKey(e) {
  if (!cur) return;
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

// tiePick asks the server for the paths of the one route across routers of plan and opens the picker on them. item is
// that route's plan item. opts.onSent runs after the take went out; opts.onClose after a cancel.
export function tiePick(plan, item, opts = {}) {
  return pickWith(plan, item, opts, pickState);
}

// askPlan asks the server for the plan of item's route, with no take: { route } with the route of the 409 body, or
// { m } with the answer when the server gave no plan.
async function askPlan(item) {
  const g = item.groups[0];
  const m = await request('tiePlanResult', { t: 'tiePlan', routes: [{ src: ref(g.src), dst: ref(item.dst), levels: g.levels }] });
  const code = m.body?.error?.code;
  if (m.status === 409 && (code === 'tie_line_needed' || code === 'no_path') && m.body.routes?.length === 1) return { route: m.body.routes[0], code };
  return { m };
}

// pickWith opens the picker on a new plan. state makes the picker's state from the route.
async function pickWith(plan, item, opts, state) {
  const { route, code, m } = await askPlan(item);
  if (code === 'tie_line_needed') return open(plan, item, opts, state(route));
  if (code === 'no_path') return showResult('error', 'No tie line path', reasonsText(route));
  showResult('error', 'Not sent', m.body?.error?.message || m.err || 'The server refused the route.');
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
    const { route } = await askPlan(c.item);
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

function reasonsText(r) {
  const why = (x) =>
    ({
      no_tie_line: `no tie line from router ${prefix(x.from)} to router ${prefix(x.to)}`,
      router_down: `router ${prefix(x.router)} is down`,
      protected: `${x.tieName || x.tie} is protected by ${x.by}`,
      taken: `${x.tieName || x.tie} carries another source in this take`,
    })[x.why] || x.why;
  const list = (r?.reasons || []).map(why);
  return list.length ? `${list.join('; ')}.` : 'No tie line path is usable.';
}

function open(plan, item, opts, st) {
  if (!ui) build();
  stop();
  cur = { st, plan, item, opts, unlocked: false, timer: countUp(tick) };
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
  if (!sent) c.opts.onClose?.();
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
  ui.body.replaceChildren(
    el('div', 'sode-tiepick__route', 'Take ', labelNode(src), ' to ', labelNode(dst), `, ${levelsPhrase(route.levels, dst.router || '')}`),
    el(
      'div',
      'sode-tiepick__map',
      endNode('Source', src, 'sode-tiepick__end--src'),
      ...steps,
      endNode('Destination', dst, 'sode-tiepick__end--dst', st.marks?.end),
    ),
    affectedNode(p, src),
  );
  paintTake();
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

async function send() {
  if (!cur?.unlocked || !ready(cur.st)) return;
  const { st, plan, item, opts } = cur;
  const route = takeRoute(st);
  const via = chosen(st)
    .hops.slice(0, -1)
    .map((h) => h.tie.name || h.tie.id)
    .join(', ');
  const text = `${srcOf(st.route.src).label} to ${dstOf(st.route.dst).label} Via ${via}`;
  close(true);
  showResult('info', 'Taking', text);
  const m = request('tieTakeResult', { t: 'tieTake', routes: [route] });
  opts.onSent?.(plan);
  answer(await m, text, { plan, item, opts, ids: route.ties.map((t) => t.id) });
}

function answer(m, text, { plan, item, opts, ids }) {
  const r = m.body?.results?.[0];
  if (m.status === 200 && r) {
    if (m.body.ok) {
      // Last holds the typed source and destination, not the hops, as after a plain take.
      if (noteLast(plan)) emit('last');
      return showResult('ok', 'Success', text);
    }
    const bad = (r.hops || []).find((h) => h.result === 'failed');
    const why = bad ? `the hop to ${bad.tieName} failed` : r.error || 'failed';
    return showResult('error', 'Take failed', `${text}: ${why}`);
  }
  if (m.body?.error?.code === 'tie_line_changed') return reopen(m.body.routes?.[0], { plan, item, opts, ids });
  showResult('error', 'Not sent', m.body?.error?.message || m.err || 'The server refused the take.');
}

// reopen: the server refused the take with 409 tie_line_changed and sent nothing (docs/tielines-design.md, The stale
// check, rule 5). The page says which tie line changed and opens the picker again on the new plan, on the same tie
// lines when they still make a path. r is the route of the 409 body.
function reopen(r, { plan, item, opts, ids }) {
  const changed = (r?.changed || []).map((t) => t.id);
  const names = (r?.changed || []).map((t) => t.name || t.id);
  const fresh = r?.newAffected || [];
  const what = names.length ? `${names.join(', ')} changed` : fresh.length ? 'The Affected destinations changed' : 'A tie line changed';
  showResult('error', 'Tie line changed', `${what}. Sode sent nothing. Choose again.`);
  return pickWith(plan, item, opts, (route) => reopenState(route, ids, changed, fresh));
}

// A Lock closes the picker, as it closes the take Confirm.
on('locking', () => close());
