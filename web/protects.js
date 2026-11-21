// Protect and Free Protect from the page (GLOSSARY.md Protect). Both go through the Confirm as a plan with
// plan.protect, so the desktop key Confirm and the phone sheet send them like a take. The server decides; the page
// only offers what it would allow. The Approval and PIN recheck popups load when a refusal needs them, so plans.js
// stays free of their page setup.

import { el, S, on, emit, onMsg, wsSend, icon, linked, dstItem, labelNode, LINK_DOWN } from './core.js';
import { keyLabel } from './keys.js';
import { levelBar, levelText, allLevels } from './levelview.js';
import { showResult } from './banner.js';
import { holdsOn, holdLabel, holdShort, protectedText, protectOffer, protectRight, holdLook, entryChip } from './protectrules.js';

let seq = 0;
const pending = new Map(); // id → { plan, answer }
let role = '';
let me = ''; // the signed-in user's name, so a Protect they set reads "Protected by you"
on('access', (a) => {
  role = a.role || '';
  if ((a.user || '') === me) return;
  me = a.user || '';
  // The views that show holds, and History's Protect Undo, draw again for the new user.
  emit('tally');
  emit('protectedRows');
});

const device = () => S.cfg?.protectDevice || 0;

// The signed-in Rights carry the Protect right set from the Role (protectRight).
const right = () => protectRight(S.myRights, role, S.users);

// The holds on dst n over levels (every level when none), with the owner of each seen from this browser's sign-in.
export const holdsAt = (n, levels = allLevels()) => holdsOn(S.prot, S.protBy, n, levels, device(), me);

export const protectState = (n) => holdsAt(n);

// The one chip of a Protect or Router lock, in every view and in History: icon and words, with no status square.
export const protectChip = (look, text) => el('span', `pb-badge pb-badge--${look.status} sode__status-badge sode__hold-chip`, icon(look.icon), text);

// The chip of one hold: its icon and color (holdLook), and the words (holdLabel).
export function holdBadge(h) {
  const b = protectChip(holdLook(h), holdLabel(h));
  b.dataset.owner = h.owner;
  return b;
}

// A Grid name's mark (gridpanelnodes.js paintDstMarks): the first hold's icon and short words, with every hold in
// the tip. null when n is free.
export function gridHold(n) {
  const p = holdsAt(n);
  const [h] = p.holds;
  if (!h) return null;
  const more = p.holds.length > 1 ? ` +${p.holds.length - 1}` : '';
  return { icon: holdLook(h).icon, text: holdShort(h) + more, tip: protectedText(p), owner: h.owner };
}

// The badge Sode's own new Protect gets: it is the signed-in user's, or Sode's with nobody signed in.
const newHold = () => holdBadge(me ? { mine: true, owner: 'you' } : { sode: true, owner: 'sode', by: '' });

// on: true sets, false frees. undo: the History entry this undoes.
export function protectPlan(dst, on, undo = null) {
  return { verb: on ? 'protect' : 'unprotect', protect: { dst, on, undo }, items: [] };
}

// The Protect picker's plan (protectpick.js): one Confirm for many destinations. One destination is a usual plan.
export function protectManyPlan(dsts, on) {
  if (dsts.length === 1) return protectPlan(dsts[0], on);
  return { verb: on ? 'protect' : 'unprotect', protect: { dst: dsts[0], dsts, on, undo: null }, items: [] };
}

function manySummary(dsts, on) {
  const rows = dsts.map((d) => {
    const p = protectState(d.n);
    const hold = !on && p.sode ? [holdBadge(p.sode)] : [];
    return el('div', 'sode__route', el('strong', null, labelNode(d)), ...hold);
  });
  return [
    el('div', null, on ? `Protect these ${dsts.length} destinations on every level?` : `Free the Protect on these ${dsts.length} destinations?`),
    ...(on ? [el('div', 'sode__route', levelBar(allLevels()), newHold())] : []),
    el('div', 'sode-protpick__plan', ...rows),
  ];
}

export function protectSummary(plan) {
  if (plan.protect.dsts) return manySummary(plan.protect.dsts, plan.protect.on);
  const { dst, on, undo } = plan.protect;
  const strong = el('strong', null, labelNode(dst));
  const all = allLevels();
  if (on) {
    return [
      el('div', null, undo ? 'Undo the Free Protect: Protect ' : 'Protect ', strong, ' on every level?'),
      el('div', 'sode__route', levelBar(all), newHold()),
    ];
  }
  const p = protectState(dst.n);
  return [
    el('div', null, undo ? 'Undo the Protect: free it on ' : 'Free the Protect on ', strong, '?'),
    ...(p.sode ? [el('div', 'sode__route', levelBar(p.sode.levels), holdBadge(p.sode))] : []),
  ];
}

// The badge row of each hold on dst, then the button the route view offers. confirm opens the Confirm with a plan;
// none means badges only.
export function protectNodes(dst, confirm = null) {
  const p = protectState(dst.n);
  const rows = p.holds.map((h) => el('div', 'sode__route', levelBar(h.levels), holdBadge(h)));
  const b = confirm && protectButton(dst, confirm);
  if (b) rows.push(el('div', 'sode__route sode__protect', b));
  return rows;
}

function protectButton(dst, confirm) {
  const offer = protectOfferOf(dst.n);
  if (!offer) return null;
  const set = offer === 'set';
  const b = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm', icon(set ? 'shield' : 'shield-off'));
  b.type = 'button';
  b.dataset.protect = offer;
  const tip = set ? `Protect ${dst.label} on every level` : `Free the Protect on ${dst.label}`;
  b.dataset.pbTip = `${tip} (x${set ? '' : `, ${keyLabel('shift+x')}`} in Keyboard mode)`;
  b.setAttribute('aria-label', tip);
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', () => confirm(protectPlan(dst, set)));
  return b;
}

// What the route view would offer on destination n: 'set', 'clear' or ''. The Grid popup's shield uses it.
export const protectOfferOf = (n) => protectOffer(protectState(n), { right: right(), device: device(), linked: linked() });

// The keyboard mode key: x sets, Shift+x frees, when the route view offers it. false when it does not.
export function protectKey(dst, set, confirm) {
  if (!dst) return false;
  const want = set ? 'set' : 'clear';
  if (protectOfferOf(dst.n) !== want) return false;
  confirm(protectPlan(dst, set));
  return true;
}

// A plan of many destinations sends one request each. The server sets each one alone, so each gets its own History
// row, and one banner tells the result of all.
function sendMany(plan) {
  const { dsts, on } = plan.protect;
  const many = { on, left: dsts.length, done: [], failed: [] };
  for (const dst of dsts) {
    const id = ++seq;
    if (!wsSend({ t: on ? 'protectSet' : 'protectClear', id, protect: { dst: dst.n } })) {
      for (const [k, p] of pending) if (p.many === many) pending.delete(k);
      showResult('error', 'Not sent', `${LINK_DOWN} Try again.`);
      return false;
    }
    pending.set(id, { plan: protectPlan(dst, on), many });
  }
  showResult('info', on ? 'Protecting…' : 'Freeing the Protects…', `${dsts.length} destinations, every level`);
  return true;
}

function manyResult(many, dst, m) {
  if (m.ok) many.done.push(dst);
  else many.failed.push(`${dst.label}: ${m.admin ? 'another user set it, so an Administrator must free it' : m.err || 'refused'}`);
  if (--many.left > 0) return;
  const n = many.done.length;
  const head = many.on ? `${n} Protected` : `${n} Protects freed`;
  if (!many.failed.length) return showResult('ok', head, 'Every level.');
  showResult(n ? 'warn' : 'error', `${head}, ${many.failed.length} failed`, `${many.failed.join('. ')}.`);
}

// ap: an Approval in person; answer: the Approval popup's { fail, done }.
export function sendProtect(plan, ap = null, answer = null) {
  if (plan.protect.dsts) return sendMany(plan);
  const { dst, on, undo } = plan.protect;
  const id = ++seq;
  const msg = on ? { t: 'protectSet', id, protect: { dst: dst.n } } : { t: 'protectClear', id, protect: { dst: dst.n } };
  if (undo) msg.protect.undo = undo.id;
  if (ap) msg.approval = ap;
  if (!wsSend(msg)) {
    showResult('error', 'Not sent', `${LINK_DOWN} Try again.`);
    return false;
  }
  pending.set(id, { plan, answer });
  showResult('info', on ? 'Protecting…' : 'Freeing the Protect…', `${dst.label}, every level`);
  return true;
}

function onResult(m) {
  const p = pending.get(m.id);
  // A request the server ran after an Approval answers with the frozen id, which this page no longer holds.
  if (!p) return void (m.t === 'protectClearResult' && m.ok && showResult('ok', 'Protect freed', 'Approved.'));
  pending.delete(m.id);
  const { dst, on } = p.plan.protect;
  if (p.many) return manyResult(p.many, dst, m);
  if (p.answer && m.approval) return void p.answer.fail(m.err);
  p.answer?.done();
  if (m.ok) {
    return showResult('ok', on ? 'Protected' : 'Protect freed', `${dst.label}, every level`);
  }
  if (m.admin && !p.answer && role && role !== 'administrator') {
    const send = (ap, ans) => sendProtect(p.plan, ap, ans);
    import('./approval.js').then(({ askApproval }) =>
      askApproval("Freeing another user's Protect", 'an Administrator', send, {
        action: () => ({ t: 'protectClear', id: m.id, protect: { dst: dst.n } }),
      }),
    );
    return;
  }
  const still = m.still?.length ? ` Still Protected on levels ${levelText(m.still)}.` : '';
  showResult('error', on ? 'Protect failed' : 'Free Protect failed', `${dst.label}: ${m.err || 'refused'}.${still}`);
}
onMsg('protectSetResult', onResult);
onMsg('protectClearResult', onResult);

const clearing = new Map(); // id → done
// Free Sode's Protects (Settings > Routers > Protects): Critical, so a refusal asks the PIN again and sends once more.
// done(ok, text).
export function clearSodeProtects(done) {
  const id = ++seq;
  if (!wsSend({ t: 'protectClearAll', id })) return done(false, LINK_DOWN);
  clearing.set(id, done);
}
onMsg('protectClearAllResult', (m) => {
  const done = clearing.get(m.id);
  if (!done) return;
  clearing.delete(m.id);
  if (m.recheck) {
    import('./recheck.js').then(({ askRecheck }) => askRecheck("Freeing Sode's Protects")).then((ok) => (ok ? clearSodeProtects(done) : done(false, m.err)));
    return;
  }
  done(m.ok, m.ok ? `${m.cleared || 0} freed` : m.err);
});

// What a History row of a Protect says (its chip) and what its Undo does. null for a take. Another system's change has
// no Undo.
export function protectRow(e) {
  if (!e.protect) return null;
  const dst = dstItem(e.dst, e.dstName);
  const set = e.protect === 'set';
  const look = entryChip(e);
  return {
    text: look.text,
    chip: () => protectChip(look, look.text),
    undo: e.ext ? { verb: 'undo', items: [] } : protectPlan(dst, !set, e),
    undoLabel: e.ext ? `Another system made this change; Sode cannot undo it` : set ? `Free the Protect on ${dst.label}` : `Protect ${dst.label} again`,
  };
}
