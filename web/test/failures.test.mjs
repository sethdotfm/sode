// Failed takes: what the router reported, undo and changedSince.
// Run: node --test web/test/
import test from 'node:test';
import assert from 'node:assert/strict';

// core.js reads these at load.
const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

test('failed takes: reasons, undo and changedSince', async () => {
  const { S, undoPlan, failureGroups, changedSince } = await import('../core.js');
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.levelBy = new Map([[1, {}], [2, {}], [3, {}], [4, {}]]);
  S.srcBy = new Map([[0, { n: 0, num: 1, name: 'CAM 1', label: 'CAM 1' }], [8, { n: 8, num: 9, name: 'CAM 9', label: 'CAM 9' }]]);
  // Take of src 7 on levels 1-4, from src 0. Level 1 was left on 0, level 2
  // is on 8 (someone else's), level 3 gave no answer, level 4 worked.
  const e = {
    id: 4, dst: 5, src: 7, srcName: 'X', dstName: 'Y', time: new Date().toISOString(), ok: false, levels: [1, 2, 3, 4],
    prev: [[1, 0], [2, 0], [3, 0], [4, 0]], failed: [1, 2, 3], now: [[1, 0], [2, 8]], err: 'no confirmation from router: ...',
  };
  assert.deepEqual(failureGroups(e).map((g) => [g.levels, g.text]), [[[1], 'still on CAM 1'], [[2], 'on CAM 9'], [[3], 'no answer']]);
  assert.equal(failureGroups({ ...e, ok: true }).length, 0);
  assert.equal(failureGroups({ ...e, failed: undefined }).length, 0); // an older entry
  // Undo restores the levels that may have changed: 3 (unknown) and 4.
  const item = undoPlan(e).items[0];
  assert.deepEqual(item.groups.map((g) => g.levels), [[3, 4]]);
  assert.deepEqual(item.untouched, [1, 2]);
  // Level 4 moved on since; level 3 can't tell (it never confirmed).
  S.tally = new Map([[5, new Map([[3, 0], [4, 9]])]]);
  assert.deepEqual(changedSince(item).map((r) => r.levels), [[4]]);
  // An old failed entry without the data checks nothing, as before.
  assert.deepEqual(changedSince(undoPlan({ ...e, failed: undefined, now: undefined }).items[0]), []);
});

test('late confirms: note, putEntry, undo and changedSince', async () => {
  const { S, entryNote, putEntry, undoPlan, changedSince } = await import('../core.js');
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.levelBy = new Map([[1, {}], [2, {}], [3, {}], [4, {}]]);
  const base = { id: 9, dst: 5, src: 7, srcName: 'X', dstName: 'Y', time: new Date().toISOString(), levels: [1, 2, 3, 4], prev: [[1, 0], [2, 0], [3, 0], [4, 0]] };
  // The note is an opaque el() result under the Proxy stub, so only its presence is checked.
  const note = entryNote({ ...base, ok: true, late: [3, 4] });
  const note1 = entryNote({ ...base, ok: true, late: [3] });
  assert.ok(note && note1);

  // putEntry: new goes first, a known id is replaced in place.
  S.hist = [{ id: 3 }, { id: 2 }, { id: 1 }];
  const before = S.hist;
  assert.equal(putEntry({ id: 4 }), 'new');
  assert.deepEqual(S.hist.map((e) => e.id), [4, 3, 2, 1]);
  assert.equal(putEntry({ id: 2, fixed: true }, S, true), 'replaced');
  assert.deepEqual(S.hist.map((e) => e.id), [4, 3, 2, 1]);
  assert.equal(S.hist[2].fixed, true);
  assert.notEqual(S.hist, before);
  // An amendment of an entry that isn't loaded is dropped, not added as new.
  const loaded = S.hist;
  assert.equal(putEntry({ id: 40, fixed: true }, S, true), 'skipped');
  assert.equal(S.hist, loaded);

  // Corrected entry (ok now): undo uses prev for every level, nothing untouched.
  const fixed = { ...base, id: 10, ok: true, late: [3, 4] };
  const item = undoPlan(fixed).items[0];
  assert.deepEqual(item.groups.map((g) => g.levels), [[1, 2, 3, 4]]);
  assert.deepEqual(item.untouched, []);
  // Late level 3 moved on since: counted like any level of the take.
  S.tally = new Map([[5, new Map([[3, 9], [4, 7]])]]);
  assert.deepEqual(changedSince(item).map((r) => r.levels), [[3]]);

  // Still partly failed: late level 3 is undone, even if `now` lists it.
  const part = { ...base, id: 11, ok: false, failed: [1], late: [3], now: [[1, 0], [3, 7]], err: 'x' };
  const p = undoPlan(part).items[0];
  assert.deepEqual(p.untouched, [1]);
  assert.ok(p.groups.flatMap((g) => g.levels).includes(3));
});

test('histRows: followers of a confirm go in its folder', async () => {
  const { S, histRows, setFilter, entryNote } = await import('../core.js');
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.levelBy = new Map([[1, {}]]);
  const t = (id, dst) => ({ id, dst, src: 1, srcName: 'A', dstName: `D${dst}`, time: new Date(Date.now() - (20 - id) * 1000).toISOString(), ok: true, levels: [1], group: 'g', groupN: 3 });
  const ext = (id, dst, extra = {}) => ({ id, dst, src: 1, srcName: 'A', dstName: `D${dst}`, time: new Date(Date.now() - (20 - id) * 1000).toISOString(), ok: true, ext: true, levels: [1], ...extra });
  const takes = [t(1, 1), t(2, 2), t(3, 3)];
  // Same dst and levels within 30s: would chain if they were plain panel changes.
  const list = [ext(8, 7, { followGroup: 'g' }), ext(7, 7, { followGroup: 'g' }), ...takes.reverse()];
  let rows = histRows(list);
  assert.equal(rows.length, 1);
  assert.equal(rows[0].f.entries.length, 3);
  assert.equal(rows[0].f.shown.length, 3);
  assert.deepEqual(rows[0].f.followers.map((e) => e.id), [8, 7]);
  assert.equal(rows[0].f.followers[0].merged, undefined);
  assert.ok(entryNote(list[0]));
  // Unknown folder: stays a standalone row.
  rows = histRows([ext(9, 7, { followGroup: 'zz' }), ...list.slice(2)]);
  assert.equal(rows.length, 2);
  assert.equal(rows[0].e.id, 9);
  assert.equal(rows[1].f.followers, undefined);
  // A filter hiding one follower drops it from the folder only.
  setFilter({ sode: true, ext: true, hidden: true, src: '', dst: '!D8' });
  rows = histRows([ext(8, 7, { followGroup: 'g' }), ext(6, 8, { followGroup: 'g' }), ext(5, 9, { followGroup: 'g' }), ...list.slice(2)]);
  assert.deepEqual(rows[0].f.followers.map((e) => e.id), [8, 5]);
  setFilter({ sode: true, ext: true, hidden: true, src: '', dst: '' });
  // Sode takes hidden: the folder doesn't show, so a follower that passes
  // stands on its own rather than vanishing with it.
  setFilter({ sode: false, ext: true, hidden: true, src: '', dst: '' });
  rows = histRows([ext(8, 7, { followGroup: 'g' }), ext(7, 7, { followGroup: 'g' }), ...takes]);
  assert.ok(rows.every((r) => !r.f));
  assert.ok(rows.length >= 1 && rows.some((r) => r.e.id === 8 || r.e.merged));
  setFilter({ sode: true, ext: true, hidden: true, src: '', dst: '' });
});

test('failure banner: grouped lines, shrinks as takes resolve', async () => {
  const { S, failureBanner } = await import('../core.js');
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.levelBy = new Map([[1, { label: '1' }], [2, { label: '2' }], [3, { label: '3' }]]);
  S.srcBy = new Map([[0, { n: 0, num: 1, name: 'CAM 1', label: 'CAM 1' }]]);
  const dst = (n) => ({ n, label: `D${n}` });
  const mk = (id, extra = {}) => ({ id, dst: id, src: 7, ok: false, levels: [1, 2, 3], prev: [[1, 0], [2, 0], [3, 0]], failed: [1, 2, 3], ...extra });
  const hist = [mk(10), mk(11), mk(12), mk(13, { now: [[1, 0], [2, 0], [3, 0]] })];
  const t = { verb: 'patch', total: 5, items: [10, 11, 12, 13].map((id) => ({ entry: id, dst: dst(id), err: 'x' })) };
  const b = failureBanner(t, hist);
  assert.equal(b.kind, 'error');
  assert.equal(b.title, 'Take partly failed (4 of 5)');
  // Three destinations share "no answer" (one line), one is "still on" (another).
  assert.match(b.key, /D10, D11, D12: levels 1–3 no answer/);
  assert.match(b.key, /D13: levels 1–3 still on CAM 1/);
  // One confirmed late, one retried: two left, and it says "still".
  const h2 = hist.map((e) => (e.id === 10 ? { ...e, ok: true, failed: [], late: [1, 2, 3] } : e)).concat([{ id: 20, ok: true, retry: 11, dst: 11, levels: [1, 2, 3] }]);
  const b2 = failureBanner(t, h2);
  assert.equal(b2.title, 'Take partly failed (2 of 5 still)');
  assert.notEqual(b2.key, b.key);
  // All resolved: a confirmation.
  const h3 = h2.concat([{ id: 21, ok: true, retry: 12, dst: 12, levels: [1] }, { id: 22, ok: true, retry: 13, dst: 13, levels: [1] }]);
  const b3 = failureBanner(t, h3);
  assert.equal(b3.kind, 'ok');
  assert.match(b3.title, /resolved: all 4 takes confirmed/);
  // Many outcomes are capped.
  const many = [1, 2, 3, 4, 5].map((i) => mk(30 + i, { failed: [1], now: [[1, i]] }));
  const t2 = { verb: 'patch', total: 5, items: many.map((e) => ({ entry: e.id, dst: dst(e.id), err: 'x' })) };
  assert.match(failureBanner(t2, many).key, /and 2 more; see Recent/);
});
