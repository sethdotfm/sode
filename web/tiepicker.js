// The Tie line picker (docs/tielines-design.md, The Tie line picker; GLOSSARY.md). It opens before a take of one route
// across routers, and it is the only confirm of that take. The server gives the ranked paths (tiepage.go, the message
// tieTake): the page never computes paths. The keys and the selection are in tiepickrules.js.
//
// Look: the source on the left, then one step for each tie line, then the destination. Each step lists its choices.
// A choice has a wire: the chosen one is lit with --pb-tieline (docs/color-roles.md). A Protected or Reserved choice is
// grey and says why. Under the steps are the Affected destinations of the chosen path.

import { el, on, request, srcItem, dstItem, labelNode, routerData, tallyText } from './core.js';
import { chord, keyLabel } from './keys.js';
import { trapDialog } from './dialogkeys.js';
import { lockout } from './lockout.js';
import { fmtClock } from './device.js';
import { showResult } from './banner.js';
import { levelsPhrase } from './levelview.js';
import { pickState, chosen, choices, choose, keyStep, ready, takeRoute, changedText, stepsOf } from './tiepickrules.js';

const TICK_MS = 1000;

let ui = null; // { root, dialog, body, take, trap }
let cur = null; // { st, plan, opts, lock, unlocked, timer }

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
export async function tiePick(plan, item, opts = {}) {
  const g = item.groups[0];
  const route = { src: ref(g.src), dst: ref(item.dst), levels: g.levels };
  const m = await request('tieTakeResult', { t: 'tieTake', routes: [route] });
  const code = m.body?.error?.code;
  if (m.status === 409 && code === 'tie_line_needed' && m.body.routes?.length === 1) return open(plan, m.body.routes[0], opts);
  if (m.status === 409 && code === 'no_path') return showResult('error', 'No tie line path', reasonsText(m.body.routes?.[0]));
  showResult('error', 'Not sent', m.body?.error?.message || m.err || 'The server refused the route.');
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

function open(plan, route, opts) {
  if (!ui) build();
  stop();
  cur = { st: pickState(route), plan, opts, unlocked: false, timer: setInterval(tick, TICK_MS) };
  cur.lock = lockout(() => {
    if (!cur) return;
    cur.unlocked = true;
    paintTake();
  });
  paint();
  ui.trap.show();
  ui.dialog.focus();
}

// stop ends the lockout and the timer of the open picker.
function stop() {
  if (!cur) return;
  cur.lock?.cancel();
  clearInterval(cur.timer);
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
    el('div', 'sode-tiepick__map', endNode('Source', src, 'sode-tiepick__end--src'), ...steps, endNode('Destination', dst, 'sode-tiepick__end--dst')),
    affectedNode(p, src),
  );
  paintTake();
}

function endNode(word, it, cls) {
  return el('div', `sode-tiepick__end ${cls}`, el('span', 'sode-tiepick__word', `${word}, router ${prefix(it.router || '')}`), labelNode(it));
}

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
  const rows = p.affected.map((e) => el('li', 'sode-tiepick__hit', labelNode(dstOf(e)), `: now shows ${nowShows(e, p)}, will show ${src.label}`));
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
  return el('div', 'sode-tiepick__affected', head, el('ul', 'sode-tiepick__hits', ...rows, ...extra));
}

async function send() {
  if (!cur?.unlocked || !ready(cur.st)) return;
  const { st, plan, opts } = cur;
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
  answer(await m, text);
}

function answer(m, text) {
  const r = m.body?.results?.[0];
  if (m.status === 200 && r) {
    if (m.body.ok) return showResult('ok', 'Success', text);
    const bad = (r.hops || []).find((h) => h.result === 'failed');
    const why = bad ? `the hop to ${bad.tieName} failed` : r.error || 'failed';
    return showResult('error', 'Take failed', `${text}: ${why}`);
  }
  if (m.body?.error?.code === 'tie_line_changed') return showResult('error', 'Tie line changed', m.body.error.message);
  showResult('error', 'Not sent', m.body?.error?.message || m.err || 'The server refused the take.');
}

// A Lock closes the picker, as it closes the take Confirm.
on('locking', () => close());
