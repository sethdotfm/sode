import test from 'node:test';
import assert from 'node:assert/strict';
import {
  KINDS,
  ALL_ROUTERS,
  rowKey,
  findMatch,
  matches,
  filtered,
  countLine,
  kindCounts,
  routerCounts,
  freeBlock,
  groupsOf,
  groupCount,
  staleNote,
  sodeDsts,
  freeLines,
  freeButton,
  freeResult,
  rowResult,
  rowStatus,
} from '../protectlistrules.js';
import { KIND_PROTECT, KIND_ROUTER_LOCK } from '../protectrules.js';

const sode = (by = 'Bo', mine = false) => ({ kind: KIND_PROTECT, device: 200, sode: true, mine, owner: mine ? 'you' : 'sode', by, levels: [1] });
const other = (device = 112) => ({ kind: KIND_PROTECT, device, sode: false, mine: false, owner: 'other', levels: [1] });
const lock = (device = 9) => ({ kind: KIND_ROUTER_LOCK, device, sode: false, mine: false, owner: 'other', levels: [1] });
const row = (uid, prefix, n, name, hold) => ({ key: rowKey(uid, n, hold), uid, n, num: `${prefix}.${n}`, label: `${prefix}.${n} ${name}`, hold });

const ROUTERS = [
  { uid: '', prefix: 'A', title: 'Main', link: 'up', device: 200 },
  { uid: 'b', prefix: 'B', title: 'Studio', link: 'up', device: 201 },
  { uid: 'c', prefix: 'C', title: 'Edit', link: 'up', device: 0 },
  { uid: 'd', prefix: 'D', title: 'MCR', link: 'down', device: 202, at: new Date(2026, 10, 26, 14, 2, 10) },
];
const ROWS = [
  row('', 'A', 1, 'PGM', sode('Ana', true)),
  row('', 'A', 2, 'PVW', sode('Bo')),
  row('', 'A', 3, 'DSM1', other(300)),
  row('', 'A', 4, 'DSM2', lock(9)),
  row('b', 'B', 1, 'TX 1', sode('Ana', true)),
  row('b', 'B', 7, 'TX 2', other(44)),
  row('c', 'C', 12, 'Rec A', lock(9)),
  row('d', 'D', 2, 'MV 1', sode('Ana', true)),
  row('d', 'D', 5, 'MV 2', sode('Bo')),
];
const NONE = { q: '', kind: 'all', router: ALL_ROUTERS };

test('a row key tells two holds on one destination apart', () => {
  assert.notEqual(rowKey('', 3, sode()), rowKey('', 3, other()));
  assert.notEqual(rowKey('', 3, other(1)), rowKey('', 3, other(2)));
  assert.notEqual(rowKey('', 3, other(9)), rowKey('', 3, lock(9)));
  assert.notEqual(rowKey('', 3, sode()), rowKey('b', 3, sode()));
});

test('find takes a name, part of a name, or a number with or without its prefix', () => {
  const r = ROWS[5]; // B.7 TX 2
  for (const q of ['TX 2', 'tx', 'b.7', 'B.7', '7', '  7 ', '']) assert.ok(findMatch(r, q), q);
  for (const q of ['70', 'A.7', 'PGM']) assert.ok(!findMatch(r, q), q);
});

test('the kind filters: Sode set, Mine, another device, Router lock', () => {
  const ids = (kind) => ROWS.filter((r) => matches(r, { ...NONE, kind })).map((r) => r.num);
  assert.deepEqual(
    KINDS.map((k) => k.id),
    ['all', 'sode', 'mine', 'other', 'lock'],
  );
  assert.equal(ids('all').length, ROWS.length);
  assert.deepEqual(ids('sode'), ['A.1', 'A.2', 'B.1', 'D.2', 'D.5']);
  assert.deepEqual(ids('mine'), ['A.1', 'B.1', 'D.2']);
  assert.deepEqual(ids('other'), ['A.3', 'B.7']);
  assert.deepEqual(ids('lock'), ['A.4', 'C.12']);
});

test('the router filter and the find work with the kind filter', () => {
  const f = { q: '', kind: 'sode', router: 'd' };
  assert.deepEqual(
    ROWS.filter((r) => matches(r, f)).map((r) => r.num),
    ['D.2', 'D.5'],
  );
  assert.deepEqual(
    ROWS.filter((r) => matches(r, { ...f, router: ALL_ROUTERS, q: 'mv 2' })).map((r) => r.num),
    ['D.5'],
  );
  assert.ok(!filtered(NONE));
  assert.ok(filtered({ ...NONE, q: ' x ' }));
  assert.ok(!filtered({ ...NONE, q: '  ' }));
  assert.ok(filtered({ ...NONE, kind: 'lock' }));
  assert.ok(filtered({ ...NONE, router: '' }), 'the first router is a filter too');
});

test('the count line: all of them, or what a filter shows', () => {
  assert.equal(countLine(ROWS, ROUTERS, NONE), '9 on 4 routers. Sode set 5.');
  assert.equal(countLine(ROWS, ROUTERS, { ...NONE, kind: 'lock' }), 'Shows 2 of 9.');
  assert.equal(countLine(ROWS.slice(0, 1), ROUTERS.slice(0, 1), NONE), '1 on 1 router. Sode set 1.');
  assert.equal(countLine([], ROUTERS, NONE), 'No Protects or Router locks.');
});

test('each filter button counts what a press would show', () => {
  const f = { q: '', kind: 'sode', router: 'd' };
  assert.deepEqual(kindCounts(ROWS, f), { all: 2, sode: 2, mine: 1, other: 0, lock: 0 });
  assert.deepEqual(routerCounts(ROWS, ROUTERS, f), { [ALL_ROUTERS]: 5, '': 2, b: 1, c: 0, d: 2 });
});

test('Free is blocked with a reason: no link to Sode, Protect off, the router not connected', () => {
  assert.equal(freeBlock(ROUTERS[0]), '');
  assert.match(freeBlock(ROUTERS[0], false), /link to the Sode server/);
  assert.match(freeBlock(ROUTERS[2]), /Protect is off/);
  assert.match(freeBlock(ROUTERS[3]), /not connected/);
});

test('groups: one for each router, in router order, with counts; a filter leaves out empty groups', () => {
  const gs = groupsOf(ROWS, ROUTERS, NONE);
  assert.deepEqual(
    gs.map((g) => [g.router.prefix, g.rows.length, g.total, g.sode, g.other, g.stale]),
    [
      ['A', 4, 4, 2, 2, false],
      ['B', 2, 2, 1, 1, false],
      ['C', 1, 1, 0, 1, false],
      ['D', 2, 2, 2, 0, true],
    ],
  );
  const lockOnly = groupsOf(ROWS, ROUTERS, { ...NONE, kind: 'lock' });
  assert.deepEqual(
    lockOnly.map((g) => [g.router.prefix, g.rows.length, g.total]),
    [
      ['A', 1, 4],
      ['C', 1, 1],
    ],
  );
  // A router with a result to show stays, so the result does not go out of sight.
  assert.deepEqual(
    groupsOf(ROWS, ROUTERS, { ...NONE, kind: 'lock' }, { b: { ok: true, text: 'x' } }).map((g) => g.router.prefix),
    ['A', 'B', 'C'],
  );
  assert.deepEqual(
    groupsOf(ROWS, ROUTERS, { ...NONE, router: 'b' }).map((g) => g.router.prefix),
    ['B'],
  );
  // With no filter, a router with no row has its group too.
  assert.deepEqual(
    groupsOf([], ROUTERS.slice(0, 2), NONE).map((g) => g.total),
    [0, 0],
  );
  assert.equal(groupCount(gs[0]), '4 in all. Sode set 2, other devices 2.');
  assert.equal(groupCount({ total: 0 }), 'None.');
});

test('a router that is not up says its rows are the last known ones, and when', () => {
  assert.equal(staleNote(ROUTERS[0]), '');
  assert.equal(staleNote(ROUTERS[3]), 'These are the last known rows, read at 14:02:10.');
  assert.match(staleNote({ link: 'down' }), /can be old/);
});

test('Free Sode’s Protects counts destinations, and ticks only routers that can free', () => {
  const two = [...ROWS, { ...row('', 'A', 2, 'PVW', sode('Bo')), key: 'second hold' }];
  assert.equal(sodeDsts(two, ''), 2, 'two holds on one destination count once');
  const lines = freeLines(ROWS, ROUTERS);
  assert.deepEqual(
    lines.map((l) => [l.router.prefix, l.count, l.can, l.ticked]),
    [
      ['A', 2, true, true],
      ['B', 1, true, true],
      ['C', 0, false, false],
      ['D', 2, false, false],
    ],
  );
  assert.match(lines[2].why, /Protect is off/);
  assert.match(lines[3].why, /not connected/);
  const onlyB = freeLines(ROWS, ROUTERS, 'b');
  assert.deepEqual(
    onlyB.map((l) => l.ticked),
    [false, true, false, false],
  );
  assert.ok(
    freeLines(ROWS, ROUTERS, '', false).every((l) => !l.can),
    'no link to Sode: nothing can be ticked',
  );
  const none = freeLines([], ROUTERS.slice(0, 1));
  assert.match(none[0].why, /no Protect on this router/);
});

test('the button says how many Protects on how many routers', () => {
  const lines = freeLines(ROWS, ROUTERS);
  assert.deepEqual(freeButton(lines, new Set(['', 'b'])), { text: 'Free 3 Protects on 2 routers', disabled: false });
  assert.deepEqual(freeButton(lines, new Set(['b'])), { text: 'Free 1 Protect on 1 router', disabled: false });
  assert.deepEqual(freeButton(lines, new Set()), { text: 'Free', disabled: true });
  assert.deepEqual(freeButton(lines, new Set(['d'])), { text: 'Free', disabled: true }, 'a router that cannot be ticked counts for nothing');
});

test('the result of Free Sode’s Protects shows for each router and in one status line', () => {
  const at = new Date(2026, 10, 26, 9, 5, 0);
  const prefixOf = (id) => ({ ua: 'A', ub: 'B' })[id];
  const ok = freeResult(
    {
      ok: true,
      cleared: 3,
      routers: [
        { router: 'ua', cleared: 2 },
        { router: 'ub', cleared: 1 },
      ],
    },
    prefixOf,
    at,
  );
  assert.equal(ok.text, 'Sode freed 3 Protects: 2 on A, 1 on B.');
  assert.deepEqual(ok.per, {
    ua: { ok: true, text: 'Sode freed 2 Protects at 09:05:00.' },
    ub: { ok: true, text: 'Sode freed 1 Protect at 09:05:00.' },
  });
  const part = freeResult(
    {
      ok: false,
      cleared: 2,
      err: '1 destination is still Protected',
      routers: [
        { router: 'ua', cleared: 2 },
        { router: 'ub', cleared: 0, still: 1, err: '1 destination is still Protected' },
      ],
    },
    prefixOf,
    at,
  );
  assert.equal(part.ok, false);
  assert.equal(part.text, 'Sode freed 2 Protects, with problems: 2 on A, B: 1 destination is still Protected.');
  assert.equal(part.per.ub.text, 'Not freed: 1 destination is still Protected.');
  assert.equal(part.per.ub.ok, false);
  const down = freeResult({ ok: false, err: 'The Server link is down.' }, prefixOf, at);
  assert.equal(down.text, 'Not freed: The Server link is down.');
  const none = freeResult({ ok: true, cleared: 0, routers: [{ router: 'ua', cleared: 0 }] }, prefixOf, at);
  assert.equal(none.per.ua.text, 'Sode had no Protect to free at 09:05:00.');
});

test('a row result is neutral when freed, and says why when refused', () => {
  assert.deepEqual(rowResult(true), { ok: true, text: 'Protect freed' });
  assert.deepEqual(rowResult(false, 'the router did not confirm.'), { ok: false, text: 'Not freed: the router did not confirm.' });
  assert.deepEqual(rowResult(false, ''), { ok: false, text: 'Not freed: refused.' });
  assert.equal(rowStatus(true, 'B.7 TX 2'), 'Protect freed: B.7 TX 2, every level.');
  assert.equal(rowStatus(false, 'B.7 TX 2', 'refused.'), 'Free Protect failed: B.7 TX 2: refused.');
});
