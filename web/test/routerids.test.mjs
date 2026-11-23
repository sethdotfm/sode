import test from 'node:test';
import assert from 'node:assert/strict';
import {
  SPAN,
  itemId,
  placeOf,
  routerN,
  makePlaces,
  acrossRouters,
  wireRoutes,
  undoIn,
  rightsIn,
  mergeTally,
  mergeProtected,
  mergeNames,
  numText,
} from '../routerids.js';
import { panelIn, placesUsed } from '../panelitems.js';
import { namesFrom, rightsAllow } from '../panelrules.js';
import { findMatches } from '../gridpanelfind.js';
import { findRows } from '../editorgridrules.js';
import { gridAxes } from '../gridpaneledges.js';
import { pressRefusal, skippedText } from '../gridpanelrights.js';

const [A, B] = ['a1a1a1a1a1a1a1a1', 'b2b2b2b2b2b2b2b2'];
const places = () =>
  makePlaces([
    { uid: A, prefix: 'A' },
    { uid: B, prefix: 'B' },
  ]);

test('an item number is the router number on the first router, and past SPAN on the next', () => {
  assert.equal(itemId(0, 12), 12);
  assert.equal(itemId(1, 12), SPAN + 12);
  assert.deepEqual([placeOf(SPAN + 12), routerN(SPAN + 12)], [1, 12]);
  // Nothing routed stays -1 on any router.
  assert.deepEqual([itemId(1, -1), placeOf(-1), routerN(-1)], [-1, 0, -1]);
  const p = places();
  assert.deepEqual([p.id(A, 3), p.id(B, 3), p.id('', 3)], [3, SPAN + 3, 3]);
  assert.deepEqual(p.ref(SPAN + 3), { router: B, n: 3 });
});

test('a take names the router of each end while Sode has two routers, and bare numbers with one', () => {
  const routes = [{ src: SPAN + 1, dst: SPAN + 4, levels: [1] }];
  assert.deepEqual(wireRoutes(routes, places()), [{ src: { router: B, n: 1 }, dst: { router: B, n: 4 }, levels: [1] }]);
  assert.deepEqual(wireRoutes([{ src: 1, dst: 4, levels: [1] }], makePlaces([{ uid: A }])), [{ src: 1, dst: 4, levels: [1] }]);
  assert.equal(acrossRouters(1, SPAN + 4), true);
  assert.equal(acrossRouters(SPAN + 1, SPAN + 4), false);
  assert.equal(acrossRouters(-1, SPAN + 4), false);
});

test("the Undo cell's routes come in on the item numbers of their router", () => {
  const u = undoIn({ id: 9, routes: [{ src: 2, dst: 5, levels: [1], router: B }] }, places());
  assert.deepEqual(u.routes, [{ src: SPAN + 2, dst: SPAN + 5, levels: [1], router: B }]);
  assert.equal(undoIn(null, places()), null);
});

test('Rights on item numbers allow an item on its own router only', () => {
  const r = rightsIn({ sources: [{ router: B, n: 2 }, 7], dsts: [{ router: A, n: 3 }], levels: { [A]: [1] } }, places());
  assert.equal(rightsAllow(r, 'sources', SPAN + 2), true);
  assert.equal(rightsAllow(r, 'sources', 2), false);
  assert.equal(rightsAllow(r, 'sources', 7), true);
  assert.equal(rightsAllow(r, 'dsts', 3), true);
  assert.equal(rightsAllow(r, 'dsts', SPAN + 3), false);
  assert.deepEqual(r.levels, { [A]: [1] });
});

const tallyA = { router: A, levels: [{ level: 1 }], routes: [{ dst: 0, level: 1, src: 3 }] };
const tallyB = {
  router: B,
  levels: [{ level: 1 }, { level: 2 }],
  routes: [
    { dst: 0, level: 1, src: 5 },
    { dst: 1, level: 1, src: -1 },
  ],
};

test("the routers' answers join on item numbers, with the first router's levels", () => {
  const t = mergeTally(tallyA, [{ place: 1, reply: tallyB }]);
  assert.equal(t.router, A);
  assert.deepEqual(t.levels, tallyA.levels);
  assert.deepEqual(t.routes, [
    { dst: 0, level: 1, src: 3 },
    { dst: SPAN, level: 1, src: SPAN + 5 },
    { dst: SPAN + 1, level: 1, src: -1 },
  ]);
  assert.equal(mergeTally(tallyA, []), tallyA);
  const p = mergeProtected({ protected: [{ dst: 2, level: 1 }] }, [{ place: 1, reply: { protected: [{ dst: 2, level: 1 }] } }]);
  assert.deepEqual(
    p.protected.map((x) => x.dst),
    [2, SPAN + 2],
  );
});

test('names of two routers show each number with its prefix and offset', () => {
  const nm = mergeNames(
    { numberOffset: 1, sources: [{ n: 0, name: 'CAM 1' }], destinations: [] },
    [{ place: 1, reply: { numberOffset: 0, sources: [{ n: 0, name: 'B CAM' }], destinations: [{ n: 4, name: 'B MON' }] } }],
    places(),
  );
  assert.deepEqual(
    nm.sources.map((x) => x.n),
    [0, SPAN],
  );
  const num = numText(nm);
  assert.deepEqual([num(0), num(SPAN), num(SPAN + 7)], ['A.1', 'B.0', 'B.7']);
  const names = namesFrom(nm);
  assert.equal(names.destination(SPAN + 4), 'B MON');
  assert.equal(names.source(SPAN + 9), 'Source B.9');
  // One router: no prefix.
  assert.equal(numText({ numberOffset: 1 })(4), '5');
  assert.equal(mergeNames({ numberOffset: 1 }, [], makePlaces([{ uid: A }])).prefixes, undefined);
});

test('a typed prefix finds only that router, on the Grid panel and in the editor lists', () => {
  const nm = mergeNames(
    { numberOffset: 0, sources: [{ n: 12, name: 'CAM' }], destinations: [] },
    [{ place: 1, reply: { numberOffset: 0, sources: [{ n: 12, name: 'Stage Cam' }], destinations: [] } }],
    places(),
  );
  const order = [12, SPAN + 12];
  assert.deepEqual(findMatches(order, nm.sources, '12', nm), [12, SPAN + 12]);
  assert.deepEqual(findMatches(order, nm.sources, 'B.12', nm), [SPAN + 12]);
  assert.deepEqual(findMatches(order, nm.sources, 'b12', nm), [SPAN + 12]);
  assert.deepEqual(findMatches(order, nm.sources, 'stage', nm), [SPAN + 12]);
  const rows = nm.sources.map((x) => ({ n: x.n, name: x.name }));
  assert.deepEqual(
    findRows(rows, 'B.12', nm).map((r) => r.n),
    [SPAN + 12],
  );
  assert.deepEqual(
    findRows(rows, '12', nm).map((r) => r.n),
    [12, SPAN + 12],
  );
});

test("a Grid panel shows each router's members, and all follows the first router", () => {
  const p = places();
  const nm = mergeNames(
    { sources: [0, 1].map((n) => ({ n, name: `A${n}` })), destinations: [{ n: 0, name: 'MON' }] },
    [{ place: 1, reply: { sources: [{ n: 0, name: 'B0' }], destinations: [{ n: 3, name: 'B MON' }] } }],
    p,
  );
  const panel = panelIn(
    {
      type: 'grid',
      sources: { members: 'all' },
      destinations: {
        members: [
          { router: A, n: 0 },
          { router: B, n: 3 },
        ],
      },
      rights: {
        dsts: [
          { router: B, n: 3 },
          { router: A, n: 0 },
        ],
      },
    },
    p,
  );
  assert.deepEqual(gridAxes(panel, nm), { srcs: [0, 1], dsts: [0, SPAN + 3] });
  assert.deepEqual(placesUsed(panel), [0, 1]);
});

test('a Buttons panel page reads its cells and Fixed destination on item numbers', () => {
  const panel = panelIn(
    {
      pages: [
        {
          id: 'p',
          cells: [
            { kind: 'source', n: 2, router: B, r: 1, c: 1 },
            { kind: 'take', r: 1, c: 2 },
          ],
        },
      ],
      settings: { fixedDests: [{ router: B, n: 1 }] },
    },
    places(),
  );
  assert.equal(panel.pages[0].cells[0].n, SPAN + 2);
  assert.deepEqual(panel.settings.fixedDests, [{ router: B, n: SPAN + 1 }]);
  assert.deepEqual(placesUsed(panel), [0, 1]);
  // Read again, as the Live Editor's show does, it stays the same.
  assert.deepEqual(panelIn(panel, places()), panel);
});

test('a press across two routers is refused with its reason, and a Diagonal skips it', () => {
  const env = { protectedDsts: new Set() };
  assert.deepEqual(pressRefusal({ src: 1, dst: SPAN + 2 }, env), { code: 'across-routers', src: 1, dst: SPAN + 2 });
  assert.equal(pressRefusal({ src: SPAN + 1, dst: SPAN + 2 }, env), null);
  const names = { destination: (d) => `D${d}` };
  assert.equal(skippedText([{ src: 1, dst: SPAN + 2 }], names), 'Skipped a route across routers: Sode has no tie lines yet');
});
