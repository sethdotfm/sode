// Protect and Free Protect from the page (GLOSSARY.md Protect). Both go through the Confirm as a plan with
// plan.protect, so the desktop key Confirm and the phone sheet send them like a take. The server decides; the page
// only offers what it would allow. The Approval and PIN recheck popups load when a refusal needs them, so plans.js
// stays free of their page setup.

import { el, S, on, emit, onMsg, wsSend, icon, dstItem, labelNode, routerOfItem, routerData, protectsOf, LINK_DOWN } from './core.js';
import { keyLabel } from './keys.js';
import { levelBar, levelText, allLevels } from './levelview.js';
import { showResult } from './banner.js';
import {
  holdsOn,
  holdLabel,
  holdShort,
  protectedText,
  protectOffer,
  protectRight,
  holdLook,
  entryChip,
  protectUndoable,
  newestProtects,
  foldedNote,
} from './protectrules.js';
import { groupIds, groupPartial } from './history.js';
import { noteOwn } from './stepback.js';

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

// Sode's device number on router uid (blank: the first).
const device = (uid = '') => protectsOf(uid).device;

// The signed-in Rights carry the Protect right set from the Role (protectRight).
const right = () => protectRight(S.myRights, role, S.users);

// mayProtect: this page can set a Protect. The Tie line picker shows its shield only then (tiepicker.js).
export const mayProtect = () => right();

// The holds on dst n of router uid (blank: the first) over levels (every level when none). The owner of each is as
// this browser's sign-in sees it.
export function holdsAt(n, levels = null, uid = '') {
  const p = protectsOf(uid);
  return holdsOn(p.prot, p.by, n, levels || allLevels(uid), p.device, me);
}

export const protectState = (n, uid = '') => holdsAt(n, null, uid);

// Every hold of every router: { n, hold, uid }, where uid is a routerData UID (blank is the first). The first router
// comes first, and each router is in destination order. Sode's record can name levels the router does not report.
// Free Sode's Protects sends the Free Protect for them too, so they are a hold of their own.
export function everyHold() {
  const uids = S.routers.length ? S.routers.map((r, i) => (i ? r.uid : '')) : [''];
  return uids.flatMap((uid) => {
    const { prot, by } = protectsOf(uid);
    const out = [];
    for (const n of [...new Set([...prot.keys(), ...by.keys()])].sort((a, b) => a - b)) {
      const p = holdsAt(n, null, uid);
      for (const hold of p.holds) out.push({ n, hold, uid });
      const rec = by.get(n);
      const lost = rec ? rec.levels.filter((u) => !p.levels.includes(u)) : [];
      const own = p.sode || { sode: true, owner: 'sode', by: rec?.by, client: rec?.kind === 'client' };
      if (lost.length) out.push({ n, hold: { ...own, device: rec.device, kind: 1, levels: lost }, uid });
    }
    return out;
  });
}
// The holds on a destination item, on its own router.
const stateOf = (dst) => protectState(dst.n, routerOfItem(dst));

// The one chip of a Protect or Router lock, in every view and in History: icon and words, with no status square.
// An 'info' status (your own hold) is neutral: blue has no state job (docs/color-roles.md).
const CHIP = { warn: 'pb-badge pb-badge--warn', error: 'pb-badge pb-badge--error' };
export const protectChip = (look, text) => el('span', `${CHIP[look.status] || 'pb-badge'} sode__status-badge sode__hold-chip`, icon(look.icon), text);

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

// The newest ok Protect entry of each destination in S.hist, once for each History draw. takePage adds to S.hist in
// place, so the length is part of the key, as in histRows.
// Each router has its own map, by the routerData UID of its entries.
let newestCache = null;
function newest(uid) {
  const h = S.hist;
  const c = newestCache;
  if (!(c && c.list === h && c.len === h.length && c.first === h[0])) newestCache = { list: h, len: h.length, first: h[0], maps: new Map() };
  const maps = newestCache.maps;
  if (!maps.has(uid)) maps.set(uid, newestProtects(h.filter((e) => entryRouter(e) === uid)));
  return maps.get(uid);
}

// The routerData UID of an entry's router: blank for the first.
const entryRouter = (e) => routerOfItem({ router: e.router });
const entryDst = (e) => dstItem(e.dst, e.dstName, e.router || '');

// Undo works only on the newest Protect entry of its destination, and only when the route view would offer the
// opposite now (protectUndoable). The layouts draw the rows again on a Protect change, so the button follows it.
const undoable = (e) => protectUndoable(e, newest(entryRouter(e)), protectOfferOf(e.dst, entryRouter(e)));
const NO_UNDO = { verb: 'undo', items: [] };
const NO_REDO = { plan: { verb: 'redo', items: [] }, label: '' };

// Undo all of Protect entries: the opposite on each entry that protectUndoable allows. The rest are skipped and the
// Confirm says how many. No entry to undo gives an empty plan.
function groupUndo(f) {
  const can = f.entries.filter(undoable);
  if (!can.length) return NO_UNDO;
  const set = can[0].protect === 'set';
  const dsts = can.map(entryDst);
  return protectManyPlan(dsts, !set, { undos: can, skipped: f.entries.length - can.length });
}

const chipOf = (e) => {
  const look = entryChip(e);
  return protectChip(look, look.text);
};
const sourceWord = (groups) => (groups.length > 1 ? 'Sources' : 'Source');

// History's descriptor of a Protect entry (historyrows.js rowView has the fields). It has no source, so its chip
// stands where the source is, and its detail lists the sources on the destination when it ran (prev).
export const PROTECT_ENTRY = {
  route: (e) => ({ lead: chipOf(e), src: null, dst: entryDst(e), many: null, notes: [] }),
  disc: (e, groups) => sourceWord(groups),
  detail: (e, dst, groups) => `${sourceWord(groups)} on ${dst.label}`,
  actions(e) {
    const dst = entryDst(e);
    const set = e.protect === 'set';
    const label = e.ext ? 'Another system made this change; Sode cannot undo it' : set ? `Free the Protect on ${dst.label}` : `Protect ${dst.label} again`;
    const plan = !e.ext && undoable(e) ? protectPlan(dst, !set, e) : NO_UNDO;
    return { undo: { plan, label, text: 'Undo', tip: label }, again: NO_REDO };
  },
  empty: (e) => (e.ext ? `#${e.id}: another system made this change.` : `#${e.id}: the Protect on ${entryDst(e).label} changed after it.`),
};

// A Group of Protect entries, all of one kind: one action on many destinations, or folded actions. Undo all undoes
// each entry that PROTECT_ENTRY would. A Protect has no Redo.
export const PROTECT_GROUP = {
  route(f) {
    const seen = new Map(f.entries.map((e) => [`${entryRouter(e)}.${e.dst}`, entryDst(e)]));
    const dsts = [...seen.values()];
    const notes = [foldedNote(f), f.entries.every((e) => e.undo != null) && 'undo', groupPartial(f) && `${f.shown.length} of ${f.entries.length} shown`];
    return {
      lead: chipOf(f.entries[0]),
      src: null,
      dst: dsts.length === 1 ? dsts[0] : `${dsts.length} destinations`,
      many: dsts.length > 1 ? dsts : null,
      notes: notes.filter(Boolean),
    };
  },
  noun: 'destinations',
  actions(f) {
    const n = f.entries.length;
    const what = f.entries[0].protect === 'set' ? 'Protects' : 'Free Protects';
    const label = f.entries[0].ext ? 'Another system made these changes; Sode cannot undo them' : `Undo all ${n} ${what} of ${groupIds(f)}`;
    return { undo: { plan: groupUndo(f), label, text: groupPartial(f) ? `Undo all ${n}` : 'Undo all', tip: label }, again: NO_REDO };
  },
  empty: (f) => (f.entries[0].ext ? `${groupIds(f)}: another system made these changes.` : `${groupIds(f)}: the Protects changed after it.`),
};

// The Confirm's line for the entries an Undo all leaves.
const skippedNote = (plan) =>
  plan.skipped
    ? [el('div', 'sode__hint', `${plan.skipped} skipped: the Protect on ${plan.skipped === 1 ? 'that destination' : 'those destinations'} changed after this.`)]
    : [];

function manySummary(dsts, on) {
  const rows = dsts.map((d) => {
    const p = stateOf(d);
    const hold = !on && p.sode ? [holdBadge(p.sode)] : [];
    return el('div', 'sode__route', el('strong', null, labelNode(d)), ...hold);
  });
  return [
    el('div', null, on ? `Protect these ${dsts.length} destinations on every level?` : `Free the Protect on these ${dsts.length} destinations?`),
    ...(on ? [el('div', 'sode__route', levelBar(allLevels(routerOfItem(dsts[0])), routerOfItem(dsts[0])), newHold())] : []),
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
  const uid = routerOfItem(dst);
  const strong = el('strong', null, labelNode(dst));
  const all = allLevels(uid);
  if (on) {
    return [
      el('div', null, undo ? 'Undo the Free Protect: Protect ' : 'Protect ', strong, ' on every level?'),
      el('div', 'sode__route', levelBar(all, uid), newHold()),
    ];
  }
  const p = stateOf(dst);
  return [
    el('div', null, undo ? 'Undo the Protect: free it on ' : 'Free the Protect on ', strong, '?'),
    ...(p.sode ? [el('div', 'sode__route', levelBar(p.sode.levels, uid), holdBadge(p.sode))] : []),
  ];
}

// The badge row of each hold on dst, with the button the route view offers. confirm opens the Confirm with a plan;
// none means badges only. Each badge stands beside its level strip. Free goes under the badge of the Sode Protect it
// frees, since the column has no room for it beside the badge; Protect has a row of its own.
export function protectNodes(dst, confirm = null) {
  const p = stateOf(dst);
  const b = confirm && protectButton(dst, confirm);
  const at = b?.dataset.protect === 'clear' ? p.holds.findIndex((h) => h.sode) : -1;
  const rows = p.holds.map((h, i) =>
    el('div', 'sode__route sode__hold', levelBar(h.levels, routerOfItem(dst)), el('div', 'sode__hold-what', holdBadge(h), ...(i === at ? [b] : []))),
  );
  if (b && at < 0) rows.push(el('div', 'sode__route sode__protect', b));
  return rows;
}

function protectButton(dst, confirm) {
  const offer = protectOfferOf(dst.n, routerOfItem(dst));
  if (!offer) return null;
  const set = offer === 'set';
  const b = el('button', 'pb-btn pb-btn--ghost', icon(set ? 'shield' : 'shield-off'), set ? 'Protect' : 'Free');
  b.type = 'button';
  b.dataset.protect = offer;
  const tip = set ? `Protect ${dst.label} on every level` : `Free the Protect on ${dst.label}`;
  b.dataset.pbTip = `${tip} (x${set ? '' : `, ${keyLabel('shift+x')}`} in Keyboard mode)`;
  b.setAttribute('aria-label', tip);
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', () => confirm(protectPlan(dst, set)));
  return b;
}

// What the route view would offer on destination n of router uid (blank: the first): 'set', 'clear' or ''. The Grid
// popup's shield uses it.
export const protectOfferOf = (n, uid = '') =>
  protectOffer(protectState(n, uid), {
    right: right(),
    admin: role === 'administrator',
    device: device(uid),
    linked: !!S.ws && S.wsState === 'up' && routerData(uid).link === 'up',
  });

// The keyboard mode key: x sets, Shift+x frees, when the route view offers it. false when it does not.
export function protectKey(dst, set, confirm) {
  if (!dst) return false;
  const want = set ? 'set' : 'clear';
  if (protectOfferOf(dst.n, routerOfItem(dst)) !== want) return false;
  confirm(protectPlan(dst, set));
  return true;
}

// Each destination names its router (blank for the first), so the server acts on the router of the destination.
const wireDsts = (dsts) =>
  dsts.map(({ dst, undo }) => {
    const w = { dst: dst.n };
    if (routerOfItem(dst)) w.router = dst.router;
    if (undo) w.undo = undo.id;
    return w;
  });
const where = (dsts) => (dsts.length === 1 ? dsts[0].dst.label : `${dsts.length} destinations`);

// One message for any number of destinations, so the server writes one History Group. ap: an Approval in person;
// answer: the Approval popup's { fail, done }. report(kind, head, text) takes the result in place of the banner.
// Settings > Routers > Protects uses it, since the banner is out of sight there.
export function sendProtect(plan, ap = null, answer = null, report = null) {
  const { dsts, on } = plan.protect;
  const id = ++seq;
  const protect = { dsts: wireDsts(dsts) };
  // Each type written out, so policy_test.go finds both sent.
  const msg = on ? { t: 'protectSet', id, protect } : { t: 'protectClear', id, protect };
  if (ap) msg.approval = ap;
  const say = report || showResult;
  if (!wsSend(msg)) {
    say('error', 'Not sent', `${LINK_DOWN} Try again.`);
    return false;
  }
  pending.set(id, { plan, answer, say });
  say('info', on ? 'Protecting…' : dsts.length === 1 ? 'Freeing the Protect…' : 'Freeing the Protects…', `${where(dsts)}, every level`);
  return true;
}

const ADMIN_WHY = 'another user set it, so an Administrator must free it';

// m.results has one row for each destination; none, with m.err, is a refusal of the whole message. Rows that another
// Sode user holds ask one Approval when this browser has a Role; one banner tells the rest.
function onResult(m) {
  const p = pending.get(m.id);
  // A request the server ran after an Approval answers with the frozen id, which this page no longer holds.
  if (!p) return void (m.t === 'protectClearResult' && m.ok && showResult('done', 'Protect freed', 'Approved.'));
  pending.delete(m.id);
  const say = p.say;
  const { dsts, on } = p.plan.protect;
  const rows = m.results || [];
  noteOwn(
    rows.map((r) => r.entry),
    S.hist,
  );
  const refused = rows.find((r) => r.approval);
  if (p.answer && refused) return void p.answer.fail(refused.err || m.err);
  p.answer?.done();
  const fail = on ? 'Protect failed' : 'Free Protect failed';
  if (!rows.length) return say('error', fail, `${dsts.length === 1 ? `${dsts[0].dst.label}: ` : ''}${m.err || 'refused'}.`);
  const asks = !p.answer && role && role !== 'administrator' ? rows.filter((r) => r.admin) : [];
  if (asks.length) askAdmin(p.plan, asks, m.id);
  const rest = rows.filter((r) => !asks.includes(r));
  if (!rest.length) return;
  // A row is for the destination sent in its place; two routers can have one number.
  const dstOf = (r) => {
    const d = dsts[rows.indexOf(r)]?.dst;
    return d?.n === r.dst ? d : dsts.find((x) => x.dst.n === r.dst)?.dst;
  };
  const failed = rest
    .filter((r) => !r.ok)
    .map((r) => {
      const d = dstOf(r);
      const still = r.still?.length ? ` Still Protected on levels ${levelText(r.still, routerOfItem(d))}.` : '';
      return `${d?.label ?? r.dst}: ${r.admin ? ADMIN_WHY : r.err || 'refused'}.${still}`;
    });
  const n = rest.length - failed.length;
  if (dsts.length === 1)
    return failed.length ? say('error', fail, failed[0]) : say('done', on ? 'Protected' : 'Protect freed', `${dsts[0].dst.label}, every level`);
  const head = on ? `${n} Protected` : `${n} Protects freed`;
  if (!failed.length) return say('done', head, 'Every level.');
  say(n ? 'warn' : 'error', `${head}, ${failed.length} failed`, failed.join(' '));
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

const clearing = new Map(); // id → { done, routers }
// Free Sode's Protects (Settings > Routers > Protects): Critical, so a refusal asks the PIN again and sends once more.
// routers: the router UIDs to act on (none: every router). done(m) gets the protectClearAllResult, or { ok: false, err }
// when nothing went.
export function clearSodeProtects(done, routers = null) {
  const id = ++seq;
  const msg = { t: 'protectClearAll', id };
  if (routers) msg.routers = routers;
  if (!wsSend(msg)) return done({ ok: false, err: LINK_DOWN });
  clearing.set(id, { done, routers });
}
onMsg('protectClearAllResult', (m) => {
  const c = clearing.get(m.id);
  if (!c) return;
  clearing.delete(m.id);
  if (m.recheck) {
    import('./recheck.js')
      .then(({ askRecheck }) => askRecheck("Freeing Sode's Protects"))
      .then((ok) => (ok ? clearSodeProtects(c.done, c.routers) : c.done({ ok: false, err: m.err })));
    return;
  }
  c.done(m);
});
