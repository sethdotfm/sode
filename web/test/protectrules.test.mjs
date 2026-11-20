import test from 'node:test';
import assert from 'node:assert/strict';
import { holdsOn, holdLabel, protectedText, protectOffer, protectRight, pickRows, filterRows, KIND_PROTECT, KIND_ROUTER_LOCK } from '../protectrules.js';
import { fuzzyScore } from '../fuzzy.js';

const prot = (rows) => {
  const m = new Map();
  for (const [d, l, dev, kind] of rows) {
    if (!m.has(d)) m.set(d, new Map());
    m.get(d).set(l, { dev, kind });
  }
  return m;
};
const by = new Map([[4, { dst: 4, by: 'Ana', device: 126, levels: [1, 2, 3] }]]);
const all = [1, 2, 3];

test("Sode's own Protect names the Sode user; others name the device and kind", () => {
  const p = prot([
    [4, 1, 126, KIND_PROTECT],
    [4, 2, 126, KIND_PROTECT],
    [6, 1, 112, KIND_PROTECT],
    [6, 2, 9, KIND_ROUTER_LOCK],
  ]);
  const four = holdsOn(p, by, 4, all, 126);
  assert.deepEqual(four.levels, [1, 2]);
  assert.equal(holdLabel(four.sode), 'Protected by Sode (Ana)');
  assert.equal(four.others.length, 0);
  const six = holdsOn(p, by, 6, all, 126);
  assert.equal(six.sode, null);
  assert.equal(protectedText(six), 'Router lock by 9; Protected by 112');
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

test('pickRows and filterRows: the offered destinations in Rights, then the find', () => {
  const it = (n, name) => ({ n, num: String(n + 1), name, lname: name.toLowerCase() });
  const dsts = [it(0, 'CAM 1'), it(1, 'CAM 2'), it(2, 'MON 1'), it(3, 'CAM 3')];
  const offer = (n) => (n === 1 ? 'clear' : 'set');
  const rows = pickRows(dsts, 'set', offer, (n) => n !== 3);
  assert.deepEqual(
    rows.map((d) => d.n),
    [0, 2],
  );
  assert.deepEqual(
    pickRows(dsts, 'clear', offer, () => true).map((d) => d.n),
    [1],
  );
  assert.deepEqual(filterRows(rows, '  ', fuzzyScore), rows);
  assert.deepEqual(
    filterRows(rows, 'MON', fuzzyScore).map((d) => d.n),
    [2],
  );
  assert.deepEqual(filterRows(rows, 'zzz', fuzzyScore), []);
});
