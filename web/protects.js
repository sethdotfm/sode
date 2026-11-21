// Protect and Free Protect from the page (GLOSSARY.md Protect). Both go through the Confirm as a plan with
// plan.protect, so the desktop key Confirm and the phone sheet send them like a take. The server decides; the page
// only offers what it would allow. The Approval and PIN recheck popups load when a refusal needs them, so plans.js
// stays free of their page setup.

import { el, S, on, emit, onMsg, wsSend, icon, linked, dstItem, labelNode, LINK_DOWN } from './core.js';
import { keyLabel } from './keys.js';
import { levelBar, levelText, allLevels } from './levelview.js';
import { showResult } from './banner.js';
import { holdsOn, holdLabel, holdShort, protectedText, protectOffer, protectRight, holdLook, entryChip, protectUndoable } from './protectrules.js';

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

// One Protect action on one or many destinations: one message and one History Group. on: true sets, false frees.
// protect.dsts is [{dst, undo}], as the wire's protect.dsts (docs/agents/server.md); undo is the History entry or null.
// skipped: entries of the row it cannot undo.
export function protectManyPlan(dsts, on, { undos = null, skipped = 0 } = {}) {
  const rows = dsts.map((dst, i) => ({ dst, undo: undos?.[i] || null }));
  return { verb: on ? 'protect' : 'unprotect', protect: { on, dsts: rows }, items: [], skipped };
}

export const protectPlan = (dst, on, undo = null) => protectManyPlan([dst], on, { undos: [undo] });

// Undo all of a History Group of Protect entries: the opposite on each entry that protectUndoable allows. The rest
// are skipped and the Confirm says how many. No entry to undo gives an empty plan.
export function protectGroupUndo(f) {
  const can = f.entries.filter((e) => protectUndoable(e, S.hist, protectOfferOf(e.dst)));
  if (!can.length) return { verb: 'undo', items: [] };
  const set = can[0].protect === 'set';
  const dsts = can.map((e) => dstItem(e.dst, e.dstName));
  return protectManyPlan(dsts, !set, { undos: can, skipped: f.entries.length - can.length });
}

// The chip and destination of a Group of Protect entries (all of one kind), for its History row.
export function protectGroupHead(f) {
  const look = entryChip(f.entries[0]);
  const seen = new Map(f.entries.map((e) => [e.dst, e.dstName]));
  const dsts = [...seen].map(([n, name]) => dstItem(n, name));
  return { chip: protectChip(look, look.text), text: look.text, dsts, label: dsts.length === 1 ? dsts[0].label : `${dsts.length} destinations` };
}

// The Confirm's line for the entries an Undo all leaves.
const skippedNote = (plan) =>
  plan.skipped
    ? [el('div', 'sode__hint', `${plan.skipped} skipped: the Protect on ${plan.skipped === 1 ? 'that destination' : 'those destinations'} changed after this.`)]
    : [];

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
  return [...oneSummary(plan), ...skippedNote(plan)];
}

function oneSummary(plan) {
  const { dsts, on } = plan.protect;
  const undo = dsts.some((d) => d.undo);
  if (dsts.length > 1) {
    const nodes = manySummary(
      dsts.map((d) => d.dst),
      on,
    );
    if (undo)
      nodes[0].textContent = on
        ? `Undo the Free Protects: Protect these ${dsts.length} destinations on every level?`
        : `Undo the Protects: free the Protect on these ${dsts.length} destinations?`;
    return nodes;
  }
  const { dst } = dsts[0];
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
export const protectOfferOf = (n) =>
  protectOffer(protectState(n), {
    right: right(),
    admin: role === 'administrator',
    device: device(),
    linked: linked(),
  });

// The keyboard mode key: x sets, Shift+x frees, when the route view offers it. false when it does not.
export function protectKey(dst, set, confirm) {
  if (!dst) return false;
  const want = set ? 'set' : 'clear';
  if (protectOfferOf(dst.n) !== want) return false;
  confirm(protectPlan(dst, set));
  return true;
}

const wireDsts = (dsts) => dsts.map(({ dst, undo }) => (undo ? { dst: dst.n, undo: undo.id } : { dst: dst.n }));
const where = (dsts) => (dsts.length === 1 ? dsts[0].dst.label : `${dsts.length} destinations`);

// One message for any number of destinations, so the server writes one History Group. ap: an Approval in person;
// answer: the Approval popup's { fail, done }.
export function sendProtect(plan, ap = null, answer = null) {
  const { dsts, on } = plan.protect;
  const id = ++seq;
  const msg = { t: on ? 'protectSet' : 'protectClear', id, protect: { dsts: wireDsts(dsts) } };
  if (ap) msg.approval = ap;
  if (!wsSend(msg)) {
    showResult('error', 'Not sent', `${LINK_DOWN} Try again.`);
    return false;
  }
  pending.set(id, { plan, answer });
  showResult('info', on ? 'Protecting…' : dsts.length === 1 ? 'Freeing the Protect…' : 'Freeing the Protects…', `${where(dsts)}, every level`);
  return true;
}

const ADMIN_WHY = 'another user set it, so an Administrator must free it';

// m.results has one row for each destination; none, with m.err, is a refusal of the whole message. Rows that another
// Sode user holds ask one Approval when this browser has a Role; one banner tells the rest.
function onResult(m) {
  const p = pending.get(m.id);
  // A request the server ran after an Approval answers with the frozen id, which this page no longer holds.
  if (!p) return void (m.t === 'protectClearResult' && m.ok && showResult('ok', 'Protect freed', 'Approved.'));
  pending.delete(m.id);
  const { dsts, on } = p.plan.protect;
  const rows = m.results || [];
  const refused = rows.find((r) => r.approval);
  if (p.answer && refused) return void p.answer.fail(refused.err || m.err);
  p.answer?.done();
  const fail = on ? 'Protect failed' : 'Free Protect failed';
  if (!rows.length) return showResult('error', fail, `${dsts.length === 1 ? `${dsts[0].dst.label}: ` : ''}${m.err || 'refused'}.`);
  const asks = !p.answer && role && role !== 'administrator' ? rows.filter((r) => r.admin) : [];
  if (asks.length) askAdmin(p.plan, asks, m.id);
  const rest = rows.filter((r) => !asks.includes(r));
  if (!rest.length) return;
  const label = new Map(dsts.map((d) => [d.dst.n, d.dst.label]));
  const failed = rest
    .filter((r) => !r.ok)
    .map((r) => {
      const still = r.still?.length ? ` Still Protected on levels ${levelText(r.still)}.` : '';
      return `${label.get(r.dst) ?? r.dst}: ${r.admin ? ADMIN_WHY : r.err || 'refused'}.${still}`;
    });
  const n = rest.length - failed.length;
  if (dsts.length === 1)
    return failed.length ? showResult('error', fail, failed[0]) : showResult('ok', on ? 'Protected' : 'Protect freed', `${dsts[0].dst.label}, every level`);
  const head = on ? `${n} Protected` : `${n} Protects freed`;
  if (!failed.length) return showResult('ok', head, 'Every level.');
  showResult(n ? 'warn' : 'error', `${head}, ${failed.length} failed`, failed.join(' '));
}

// Another Sode user's Protects: one Approval frees every one of rows, each with its own undo.
function askAdmin(plan, rows, id) {
  const hit = new Set(rows.map((r) => r.dst));
  const dsts = plan.protect.dsts.filter((d) => hit.has(d.dst.n));
  const sub = { ...plan, protect: { ...plan.protect, dsts }, skipped: 0 };
  const what =
    dsts.length === 1
      ? `Freeing another user's Protect on ${dsts[0].dst.label}`
      : `Freeing other users' Protects on ${dsts.map((d) => d.dst.label).join(', ')}`;
  const send = (ap, ans) => sendProtect(sub, ap, ans);
  import('./approval.js').then(({ askApproval }) =>
    askApproval(what, 'an Administrator', send, {
      action: () => ({ t: 'protectClear', id, protect: { dsts: wireDsts(dsts) } }),
    }),
  );
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
