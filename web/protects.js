// Protect and Clear Protect from the page (GLOSSARY.md Protect). Both go through the Confirm as a plan with
// plan.protect, so the desktop key Confirm and the phone sheet send them like a take. The server decides; the page
// only offers what it would allow. The Approval and PIN recheck popups load when a refusal needs them, so plans.js
// stays free of their page setup.

import { el, S, on, onMsg, wsSend, icon, linked, dstItem, labelNode, LINK_DOWN } from './core.js';
import { keyLabel } from './keys.js';
import { levelBar, levelText, allLevels } from './levelview.js';
import { showResult } from './banner.js';
import { holdsOn, holdLabel, protectOffer } from './protectrules.js';

let seq = 0;
const pending = new Map(); // id → { plan, answer }
let role = '';
on('access', (a) => {
  role = a.role || '';
});

const device = () => S.cfg?.protectDevice || 0;

// The signed-in Rights carry the Protect right set from the Role. With nobody signed in there are none: only a
// Sode with no users (every browser an Administrator) offers it then.
const right = () => (S.myRights ? S.myRights.protect === true : role === 'administrator' && !S.users?.length);

export const protectState = (n) => holdsOn(S.prot, S.protBy, n, allLevels(), device());

// on: true sets, false clears. undo: the History entry this undoes.
export function protectPlan(dst, on, undo = null) {
  return { verb: on ? 'protect' : 'unprotect', protect: { dst, on, undo }, items: [] };
}

export function protectSummary(plan) {
  const { dst, on, undo } = plan.protect;
  const strong = el('strong', null, labelNode(dst));
  const all = allLevels();
  if (on) {
    return [
      el('div', null, undo ? 'Undo the Clear Protect: Protect ' : 'Protect ', strong, ' on every level?'),
      el('div', 'sode__route', levelBar(all), el('span', 'pb-badge pb-badge--warn', 'Protected by Sode')),
      el('div', 'sode__hist-note', 'Nobody can change it until it is cleared: not panels, not DashBoard, not Sode.'),
    ];
  }
  const p = protectState(dst.n);
  return [
    el('div', null, undo ? 'Undo the Protect: clear it on ' : 'Clear the Protect on ', strong, '?'),
    ...(p.sode ? [el('div', 'sode__route', levelBar(p.sode.levels), el('span', 'pb-badge pb-badge--warn', holdLabel(p.sode)))] : []),
  ];
}

// The badge row of each hold on dst, then the button the route view offers. confirm opens the Confirm with a plan;
// none means badges only.
export function protectNodes(dst, confirm = null) {
  const p = protectState(dst.n);
  const rows = p.holds.map((h) => el('div', 'sode__route', levelBar(h.levels), el('span', 'pb-badge pb-badge--warn', holdLabel(h))));
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
  const tip = set ? `Protect ${dst.label} on every level` : `Clear the Protect on ${dst.label}`;
  b.dataset.pbTip = `${tip} (x${set ? '' : `, ${keyLabel('shift+x')}`} in Keyboard mode)`;
  b.setAttribute('aria-label', tip);
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', () => confirm(protectPlan(dst, set)));
  return b;
}

// What the route view would offer on destination n: 'set', 'clear' or ''. The Grid popup's shield uses it.
export const protectOfferOf = (n) => protectOffer(protectState(n), { right: right(), device: device(), linked: linked() });

// The keyboard mode key: x sets, Shift+x clears, when the route view offers it. false when it does not.
export function protectKey(dst, set, confirm) {
  if (!dst) return false;
  const want = set ? 'set' : 'clear';
  if (protectOfferOf(dst.n) !== want) return false;
  confirm(protectPlan(dst, set));
  return true;
}

// ap: an Approval in person; answer: the Approval popup's { fail, done }.
export function sendProtect(plan, ap = null, answer = null) {
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
  showResult('info', on ? 'Protecting…' : 'Clearing the Protect…', `${dst.label}, every level`);
  return true;
}

function onResult(m) {
  const p = pending.get(m.id);
  // A request the server ran after an Approval answers with the frozen id, which this page no longer holds.
  if (!p) return void (m.t === 'protectClearResult' && m.ok && showResult('ok', 'Protect cleared', 'Approved.'));
  pending.delete(m.id);
  const { dst, on } = p.plan.protect;
  if (p.answer && m.approval) return void p.answer.fail(m.err);
  p.answer?.done();
  if (m.ok) {
    return showResult('ok', on ? 'Protected' : 'Protect cleared', `${dst.label}, every level`);
  }
  if (m.admin && !p.answer && role && role !== 'administrator') {
    const send = (ap, ans) => sendProtect(p.plan, ap, ans);
    import('./approval.js').then(({ askApproval }) =>
      askApproval("Clearing another user's Protect", 'an Administrator', send, {
        action: () => ({ t: 'protectClear', id: m.id, protect: { dst: dst.n } }),
      }),
    );
    return;
  }
  const still = m.still?.length ? ` Still Protected on levels ${levelText(m.still)}.` : '';
  showResult('error', on ? 'Protect failed' : 'Clear Protect failed', `${dst.label}: ${m.err || 'refused'}.${still}`);
}
onMsg('protectSetResult', onResult);
onMsg('protectClearResult', onResult);

const clearing = new Map(); // id → done
// Clear Sode's Protects (Settings > Routers > Protects): Critical, so a refusal asks the PIN again and sends once more.
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
    import('./recheck.js').then(({ askRecheck }) => askRecheck("Clearing Sode's Protects")).then((ok) => (ok ? clearSodeProtects(done) : done(false, m.err)));
    return;
  }
  done(m.ok, m.ok ? `${m.cleared || 0} cleared` : m.err);
});

// What a History row of a Protect says and what its Undo does. null for a take.
export function protectRow(e) {
  if (!e.protect) return null;
  const dst = dstItem(e.dst, e.dstName);
  const set = e.protect === 'set';
  return {
    text: set ? 'Protect' : 'Clear Protect',
    undo: protectPlan(dst, !set, e),
    undoLabel: set ? `Clear the Protect on ${dst.label}` : `Protect ${dst.label} again`,
  };
}
