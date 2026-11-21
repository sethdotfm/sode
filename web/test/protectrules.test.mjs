import test from 'node:test';
import assert from 'node:assert/strict';
import {
  holdsOn,
  holdLabel,
  protectedText,
  protectOffer,
  protectRight,
  pickRows,
  pickSplit,
  pickRange,
  filterRows,
  KIND_PROTECT,
  KIND_ROUTER_LOCK,
  holdLook,
  holdShort,
  shieldTargets,
  protectUndoable,
  entryChip,
  protectFoldKey,
  protectFoldOpts,
  foldedNote,
  newestProtects,
} from '../protectrules.js';
import { fuzzyScore } from '../fuzzy.js';
import { mergeRuns } from '../historyruns.js';

const prot = (rows) => {
  const m = new Map();
  for (const [d, l, dev, kind] of rows) {
    if (!m.has(d)) m.set(d, new Map());
    m.get(d).set(l, { dev, kind });
  }
  return m;
};
const by = new Map([[4, { dst: 4, by: 'Ana', kind: 'user', device: 126, levels: [1, 2, 3] }]]);
const all = [1, 2, 3];

test("Sode's own Protect names the Sode user; others say another system, with the device and kind", () => {
  const p = prot([
    [4, 1, 126, KIND_PROTECT],
    [4, 2, 126, KIND_PROTECT],
    [6, 1, 112, KIND_PROTECT],
    [6, 2, 9, KIND_ROUTER_LOCK],
  ]);
  const four = holdsOn(p, by, 4, all, 126);
  assert.deepEqual(four.levels, [1, 2]);
  assert.equal(holdLabel(four.sode), 'Protected by Ana in Sode');
  assert.equal(four.sode.owner, 'sode');
  assert.equal(four.others.length, 0);
  const six = holdsOn(p, by, 6, all, 126);
  assert.equal(six.sode, null);
  assert.equal(protectedText(six), 'Router lock by another system (device 9); Protected by another system (device 112)');
  assert.deepEqual(
    six.holds.map((h) => h.owner),
    ['other', 'other'],
  );
});

test('the three owners: you, another Sode user, another system', () => {
  const p = prot([
    [4, 1, 126, KIND_PROTECT],
    [6, 1, 112, KIND_PROTECT],
  ]);
  const mine = holdsOn(p, by, 4, all, 126, 'Ana').sode;
  assert.equal(holdLabel(mine), 'Protected by you');
  assert.equal(mine.owner, 'you');
  assert.equal(holdLabel(holdsOn(p, by, 4, all, 126, 'Bo').sode), 'Protected by Ana in Sode');
  assert.equal(holdsOn(p, by, 6, all, 126, 'Ana').holds[0].owner, 'other');
  // An API client or a guest with the signed-in user's name is not the user.
  for (const kind of ['client', 'guest']) {
    const rec = new Map([[4, { dst: 4, by: 'Ana', kind, device: 126, levels: [1] }]]);
    const h = holdsOn(p, rec, 4, all, 126, 'Ana').sode;
    assert.equal(h.owner, 'sode', kind);
  }
  const guest = new Map([[4, { dst: 4, by: '10.0.0.9', kind: 'guest', device: 126, levels: [1] }]]);
  assert.equal(holdLabel(holdsOn(p, guest, 4, all, 126).sode), 'Protected by a guest in Sode (10.0.0.9)');
  const gone = new Map([[4, { dst: 4, by: 'Cy', kind: 'gone', device: 126, levels: [1] }]]);
  assert.equal(holdLabel(holdsOn(p, gone, 4, all, 126, 'Cy').sode), 'Protected by Cy in Sode');
  const client = new Map([[4, { dst: 4, by: 'Desk', kind: 'client', device: 126, levels: [1] }]]);
  assert.equal(holdLabel(holdsOn(p, client, 4, all, 126, 'Ana').sode), 'Protected by API client Desk in Sode');
  // Every Protect has the shield, whoever holds it; a Router lock has shield-ban. Only yours is info.
  const look = (owner, kind = KIND_PROTECT) => holdLook({ owner, kind });
  assert.deepEqual(
    [look('you'), look('sode'), look('other')].map((l) => l.icon),
    ['shield', 'shield', 'shield'],
  );
  assert.deepEqual(look('other', KIND_ROUTER_LOCK), { icon: 'shield-ban', status: 'warn' });
  assert.deepEqual(
    [look('you'), look('sode'), look('other')].map((l) => l.status),
    ['info', 'warn', 'warn'],
  );
});

test("a Protect with the device of an old record is still Sode's", () => {
  const p = prot([[4, 1, 126, KIND_PROTECT]]);
  assert.equal(holdsOn(p, by, 4, all, 200).sode?.by, 'Ana');
  // A Router lock with Sode's number is never Sode's: SW-P-08 cannot set one.
  assert.equal(holdsOn(prot([[5, 1, 126, KIND_ROUTER_LOCK]]), new Map(), 5, all, 126).sode, null);
  assert.equal(holdLabel(holdsOn(prot([[5, 1, 126, KIND_PROTECT]]), new Map(), 5, all, 126).sode), 'Protected by Sode');
});

test("the route view offers set on a free destination, clear on Sode's own, nothing on another hold", () => {
  const env = { right: true, device: 126, linked: true };
  const free = holdsOn(new Map(), new Map(), 3, all, 126);
  assert.equal(protectOffer(free, env), 'set');
  assert.equal(protectOffer(free, { ...env, device: 0 }), '');
  assert.equal(protectOffer(free, { ...env, right: false }), '');
  assert.equal(protectOffer(free, { ...env, linked: false }), '');
  assert.equal(protectOffer(holdsOn(prot([[4, 1, 126, 1]]), by, 4, all, 126), env), 'clear');
  assert.equal(protectOffer(holdsOn(prot([[6, 1, 112, 1]]), by, 6, all, 126), env), '');
  // An Administrator clears any of Sode's Protects without the Protect right, but sets none without it.
  const noRight = { ...env, right: false };
  assert.equal(protectOffer(holdsOn(prot([[4, 1, 126, 1]]), by, 4, all, 126), noRight), '');
  assert.equal(protectOffer(holdsOn(prot([[4, 1, 126, 1]]), by, 4, all, 126), { ...noRight, admin: true }), 'clear');
  assert.equal(protectOffer(free, { ...noRight, admin: true }), '');
  assert.equal(protectOffer(holdsOn(prot([[6, 1, 112, 1]]), by, 6, all, 126), { ...noRight, admin: true }), '');
  assert.equal(
    protectOffer(
      holdsOn(
        prot([
          [4, 1, 126, 1],
          [4, 2, 9, 2],
        ]),
        by,
        4,
        all,
        126,
      ),
      env,
    ),
    '',
  );
});

test('protectRight: the Rights decide; with nobody signed in only a Sode with no users has it', () => {
  assert.equal(protectRight({ protect: true }, 'standard', [{ name: 'Ana' }]), true);
  assert.equal(protectRight({}, 'administrator', [{ name: 'Ana' }]), false);
  assert.equal(protectRight(null, 'administrator', []), true);
  assert.equal(protectRight(null, 'administrator', [{ name: 'Ana' }]), false);
});

test('pickRows: the destinations in Rights, by filter, each with its offer; filterRows then finds', () => {
  const it = (n, name) => ({ n, num: String(n + 1), name, lname: name.toLowerCase() });
  const dsts = [it(0, 'CAM 1'), it(1, 'CAM 2'), it(2, 'MON 1'), it(3, 'CAM 3'), it(4, 'MON 2')];
  // 1 is Sode's own Protect, 4 another system's; 3 is outside the Rights.
  const env = { inRights: (n) => n !== 3, held: (n) => n === 1 || n === 4, offerOf: (n) => ({ 1: 'clear', 4: '' })[n] ?? 'set' };
  const ns = (rows) => rows.map((d) => d.n);
  assert.deepEqual(ns(pickRows(dsts, 'all', env)), [0, 1, 2, 4]);
  assert.deepEqual(ns(pickRows(dsts, 'protected', env)), [1, 4]);
  assert.deepEqual(ns(pickRows(dsts, 'free', env)), [0, 2]);
  const rows = pickRows(dsts, 'all', env);
  assert.deepEqual(
    rows.map((d) => d.offer),
    ['set', 'clear', 'set', ''],
  );
  assert.deepEqual(filterRows(rows, '  ', fuzzyScore), rows);
  assert.deepEqual(ns(filterRows(rows, 'MON', fuzzyScore)), [2, 4]);
  assert.deepEqual(filterRows(rows, 'zzz', fuzzyScore), []);
  // A Shift+click range takes only the rows that can be picked.
  assert.deepEqual(pickRange(rows, 3, 0), [0, 1, 2]);
});

test('pickSplit: a mix of free and protected picks protects first', () => {
  const offerOf = (n) => ({ 1: 'set', 2: 'clear', 3: 'set', 4: '' })[n];
  assert.deepEqual(pickSplit([1, 2, 3, 4], offerOf), { protect: [1, 3], clear: [2], first: 'set' });
  assert.deepEqual(pickSplit([2], offerOf), { protect: [], clear: [2], first: 'clear' });
  assert.equal(pickSplit([4], offerOf).first, '');
});
test('holdShort: the Grid mark names the owner in few words', () => {
  const p = prot([
    [4, 1, 126, KIND_PROTECT],
    [6, 1, 112, KIND_PROTECT],
    [7, 1, 9, KIND_ROUTER_LOCK],
  ]);
  assert.equal(holdShort(holdsOn(p, by, 4, all, 126, 'Ana').sode), 'Protected by you');
  assert.equal(holdShort(holdsOn(p, by, 4, all, 126, 'Bo').sode), 'Protected by Ana');
  assert.equal(holdShort(holdsOn(p, by, 6, all, 126).holds[0]), 'Protected by device 112');
  assert.equal(holdShort(holdsOn(p, by, 7, all, 126).holds[0]), 'Router lock by device 9');
});

test('shieldTargets: a picked name acts on every picked destination with the same offer', () => {
  const offers = { 1: 'set', 2: 'set', 3: 'clear', 4: '', 5: 'set' };
  const offerOf = (n) => offers[n] || '';
  assert.deepEqual(shieldTargets(5, [1, 2, 3], offerOf), { offer: 'set', dsts: [5] });
  assert.deepEqual(shieldTargets(2, [1, 2, 3, 4], offerOf), { offer: 'set', dsts: [1, 2] });
  assert.deepEqual(shieldTargets(3, [1, 2, 3], offerOf), { offer: 'clear', dsts: [3] });
  assert.equal(shieldTargets(4, [1, 4], offerOf), null);
});

test('protectUndoable: only the newest ok Protect entry of a destination, and only when the opposite is offered', () => {
  const hist = [
    { id: 5, dst: 4, protect: 'set', ok: true },
    { id: 6, dst: 4, protect: 'clear', ok: true },
    { id: 7, dst: 4, protect: 'set', ok: true },
    { id: 8, dst: 9, protect: 'set', ok: false },
    { id: 9, dst: 4, src: 2, ok: true },
  ];
  assert.equal(protectUndoable(hist[2], newestProtects(hist), 'clear'), true);
  assert.equal(protectUndoable(hist[2], newestProtects(hist), ''), false, 'someone else holds it now');
  assert.equal(protectUndoable(hist[2], newestProtects(hist), 'set'), false, 'cleared since');
  assert.equal(protectUndoable(hist[0], newestProtects(hist), 'clear'), false, 'an older entry');
  assert.equal(protectUndoable(hist[1], newestProtects(hist), 'set'), false, 'an older entry');
  assert.equal(protectUndoable(hist[3], newestProtects(hist), 'set'), false, 'failed');
});

test('entryChip: one chip for each History Protect entry, Free with its own icon', () => {
  const chip = (e) => {
    const c = entryChip(e);
    return `${c.icon} ${c.text}`;
  };
  assert.equal(chip({ protect: 'set' }), 'shield Protect');
  assert.equal(chip({ protect: 'clear' }), 'shield-off Free Protect');
  assert.equal(chip({ protect: 'set', ext: true, holdKind: 'protect' }), 'shield Protect');
  assert.equal(chip({ protect: 'clear', ext: true, holdKind: 'protect' }), 'shield-off Free Protect');
  assert.equal(chip({ protect: 'set', ext: true, holdKind: 'routerLock' }), 'shield-ban Router lock');
  assert.equal(chip({ protect: 'clear', ext: true, holdKind: 'routerLock' }), 'shield-off Free Router lock');
  // The icons are the route view's (holdLook), so a chip looks the same in every place.
  assert.equal(entryChip({ protect: 'set', ext: true, holdKind: 'routerLock' }).icon, holdLook({ kind: KIND_ROUTER_LOCK, owner: 'other' }).icon);
  assert.equal(entryChip({ protect: 'set' }).icon, holdLook({ kind: KIND_PROTECT, owner: 'other' }).icon);
});

test('protectUndoable: an External Protect entry has no Undo, and blocks the Undo of an older one of Sode', () => {
  const hist = [
    { id: 2, ok: true, protect: 'set', ext: true, holdKind: 'protect', dst: 4 },
    { id: 1, ok: true, protect: 'clear', dst: 4 },
  ];
  assert.equal(protectUndoable(hist[0], newestProtects(hist), 'set'), false);
  assert.equal(protectUndoable(hist[0], newestProtects(hist), 'clear'), false);
  assert.equal(protectUndoable(hist[1], newestProtects(hist), 'set'), false);
});

// History rows of Protect actions (newest first). t is seconds after a fixed start.
const pe = (id, t, o = {}) => ({ id, time: new Date(Date.UTC(2026, 0, 1, 0, 0, t)).toISOString(), protect: 'set', client: 'Ana', dst: id, ok: true, ...o });
const grp = (group, ...es) => ({ f: { group, n: es.length, entries: es, shown: es } });

test('protectFoldKey: the same direction by the same person; no undo and no other system', () => {
  assert.equal(protectFoldKey([pe(1, 0)]), protectFoldKey([pe(2, 5)]));
  assert.notEqual(protectFoldKey([pe(1, 0)]), protectFoldKey([pe(2, 0, { protect: 'clear' })]));
  assert.notEqual(protectFoldKey([pe(1, 0)]), protectFoldKey([pe(2, 0, { client: 'Bo' })]));
  assert.notEqual(protectFoldKey([pe(1, 0)]), protectFoldKey([pe(2, 0, { browserOf: 'Bo' })]));
  assert.equal(protectFoldKey([pe(1, 0, { undo: 4 })]), '');
  assert.equal(protectFoldKey([pe(1, 0, { ext: true })]), '');
  assert.equal(protectFoldKey([{ id: 1, src: 3, dst: 1 }]), '');
  assert.equal(protectFoldKey([pe(1, 0), pe(2, 0, { protect: 'clear' })]), '');
});

const fold = (rows, gap) => mergeRuns(rows, protectFoldOpts(gap));
const show = (r) => ({ ...r, shows: true });

test('folding: neighbours within the gap fold into one Group, keyed by the oldest', () => {
  const rows = [grp('p2', pe(40, 20), pe(39, 20)), { e: pe(30, 10) }, grp('p1', pe(20, 0), pe(19, 0))].map(show);
  const out = fold(rows, 15000);
  assert.equal(out.length, 1);
  const f = out[0].f;
  assert.equal(f.group, 'p1');
  assert.equal(f.n, 5);
  assert.equal(f.folded, 3);
  assert.deepEqual(
    f.entries.map((e) => e.id),
    [40, 39, 30, 20, 19],
  );
  // The gap is between neighbours, not the whole span.
  assert.equal(fold(rows, 9000).length, 3);
  assert.equal(fold(rows, 0), rows);
  // A lone oldest entry gives an id no server Group has.
  assert.equal(fold([{ e: pe(30, 10) }, { e: pe(20, 0) }].map(show), 30000)[0].f.group, 'fold-20');
});

test('folding: another row, the other direction, an undo or a hidden row ends the fold', () => {
  const take = show({ e: { id: 25, time: pe(0, 5).time, src: 3, dst: 9, ok: true } });
  assert.equal(fold([show({ e: pe(30, 10) }), take, show({ e: pe(20, 0) })], 30000).length, 3);
  assert.equal(fold([{ e: pe(30, 10, { protect: 'clear' }) }, { e: pe(20, 0) }].map(show), 30000).length, 2);
  assert.equal(fold([{ e: pe(30, 10, { undo: 20 }) }, { e: pe(20, 0) }].map(show), 30000).length, 2);
  assert.equal(fold([show({ e: pe(30, 10) }), { e: pe(25, 5), shows: false }, show({ e: pe(20, 0) })], 30000).length, 3);
  const one = [show({ e: pe(30, 10) })];
  assert.deepEqual(fold(one, 30000), one);
});

test('mergeRuns: a run goes on through other lanes, and a row of its lane that does not join ends it', () => {
  const r = (id, lane, t, k = 'k', shows = true) => ({ id, lane, t, k, shows });
  const opts = { lane: (x) => x.lane, key: (x) => x.k, gapMs: 10, span: (x) => [x.t, x.t], join: (run) => ({ id: run.map((x) => x.id).join('+') }) };
  const ids = (rows) => mergeRuns(rows, opts).map((x) => x.id);
  assert.deepEqual(ids([r(3, 'a', 20), r(2, 'b', 15), r(1, 'a', 10)]), ['3+1', 2]);
  assert.deepEqual(ids([r(3, 'a', 20), r(2, 'a', 15, ''), r(1, 'a', 10)]), [3, 2, 1]);
  assert.deepEqual(ids([r(3, 'a', 20), r(2, 'a', 15, 'k', false), r(1, 'a', 10)]), [3, 2, 1]);
  assert.deepEqual(ids([r(3, 'a', 20), r(1, 'a', 9)]), [3, 1]);
});

test('foldedNote: "N actions" only when an action had more than one destination', () => {
  const f = (folded, dsts) => ({ folded, entries: dsts.map((dst) => ({ dst })) });
  assert.equal(foldedNote(f(2, [1, 2])), '');
  assert.equal(foldedNote(f(2, [1, 2, 3, 4])), '2 actions');
  assert.equal(foldedNote(f(undefined, [1, 2])), '');
});
