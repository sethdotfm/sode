import test from 'node:test';
import assert from 'node:assert/strict';
import { SPAN, makePlaces, mergeTally, mergeNames, presetsIn, tallyAt } from '../routerids.js';
import { envFrom, cellNamesFrom } from '../panelrules.js';
import { createState, pressDestination, pressSource, pressPreset } from '../panelmodel.js';
import { otherRouterLevels } from '../gridpanellevels.js';
import { stageRoutes, createStage, routesOf } from '../gridpanelstage.js';
import { findEntries, rankFind, applyFind, findPlan } from '../findrules.js';
import { createGrid, select } from '../gridmodel.js';

const [A, B] = ['a1a1a1a1a1a1a1a1', 'b2b2b2b2b2b2b2b2'];
const places = makePlaces([
  { uid: A, prefix: 'A' },
  { uid: B, prefix: 'B' },
]);
const lv = (n) => Array.from({ length: n }, (_, i) => ({ level: i + 1, label: `L${i + 1}` }));
// Router A has 4 levels, router B 2.
const tallyA = { router: A, levels: lv(4), levelNumbering: '1', defaultLevels: '1-4', routes: [] };
const tallyB = { router: B, levels: lv(2), levelNumbering: '1', defaultLevels: '2', routes: [{ dst: 3, level: 1, src: 3 }] };
const tally = mergeTally(tallyA, [{ place: 1, reply: tallyB }]);
const T0 = 1000;

test("a Buttons take on router B sends router B's levels only", () => {
  const env = envFrom(tally, {});
  let st = createState({ mode: 'take' });
  st = pressDestination(st, { kind: 'destination', n: SPAN + 3 });
  st = pressSource(st, { kind: 'source', n: SPAN + 11 }, env, T0);
  assert.deepEqual(st.staged, [{ dst: SPAN + 3, src: SPAN + 11, levels: [1, 2] }]);
  // On router A the same panel sends router A's four levels.
  st = pressDestination(st, { kind: 'destination', n: 2 });
  st = pressSource(st, { kind: 'source', n: 7 }, env, T0);
  assert.deepEqual(st.staged.at(-1), { dst: 2, src: 7, levels: [1, 2, 3, 4] });
});

test("a panel's level subset of router B limits router B only", () => {
  const env = envFrom(tally, {});
  let st = createState({ mode: 'take', levels: { [A]: [3], [B]: [2] } });
  st = pressDestination(st, { kind: 'destination', n: SPAN + 3 });
  st = pressSource(st, { kind: 'source', n: SPAN + 11 }, env, T0);
  assert.deepEqual(st.staged, [{ dst: SPAN + 3, src: SPAN + 11, levels: [2] }]);
});

test('a router whose tally has not come yet stages nothing', () => {
  const env = envFrom(tallyA, {});
  let st = createState({ mode: 'take' });
  st = pressDestination(st, { kind: 'destination', n: SPAN + 3 });
  st = pressSource(st, { kind: 'source', n: SPAN + 11 }, env, T0);
  assert.equal(st.staged.length, 0);
  assert.equal(st.error.code, 'no-common-levels');
});

test("a Preset cell's routes on router B take router B's levels", () => {
  const pr = presetsIn(
    { presets: [{ scope: 'everyone', name: 'Stage', id: 'p1', items: [{ src: { router: B, n: 11 }, dsts: [{ router: B, n: 3 }], levels: [1, 2, 3] }] }] },
    places,
  );
  const env = envFrom(tally, {}, pr);
  const st = pressPreset(createState({ mode: 'take' }), { kind: 'preset', scope: 'everyone', name: 'Stage' }, env, T0);
  assert.deepEqual(st.staged, [{ dst: SPAN + 3, src: SPAN + 11, levels: [1, 2] }]);
});

test('a Grid route on router B takes the box levels that router B has, and its own Default levels for an empty box', () => {
  const t = tallyAt(tally, 1);
  assert.equal(t.router, B);
  assert.deepEqual(otherRouterLevels([1, 3, 4], false, t, null, {}), [1]);
  assert.deepEqual(otherRouterLevels([1, 2, 3, 4], true, t, null, {}), [2]);
  assert.deepEqual(otherRouterLevels([1, 2], true, t, null, { levels: { [B]: [1] } }), [1]);
  assert.deepEqual(otherRouterLevels([1, 2], false, t, { levels: { [B]: [2] } }, {}), [2]);
  assert.deepEqual(otherRouterLevels([1], false, null, null, {}), []);
  const of = ({ dst }) => (dst >= SPAN ? [1, 2] : [1, 2, 3, 4]);
  const r = stageRoutes(
    createStage({ mode: 'take' }),
    [
      { src: 7, dst: 2 },
      { src: SPAN + 11, dst: SPAN + 3 },
    ],
    of,
  );
  assert.deepEqual(routesOf(r.st), [
    { src: 7, dst: 2, levels: [1, 2, 3, 4] },
    { src: SPAN + 11, dst: SPAN + 3, levels: [1, 2] },
  ]);
});

const nm = mergeNames(
  { numberOffset: 1, sources: [{ n: 0, name: 'CAM 1' }], destinations: [{ n: 2, name: 'MON 3' }] },
  [{ place: 1, reply: { numberOffset: 1, sources: [{ n: 11, name: 'Stage Cam' }], destinations: [{ n: 3, name: 'Stage Mon' }] } }],
  places,
);

test("the editor's Find lists every router's items with their prefix, and a fill puts the router on the cell", () => {
  const entries = findEntries(nm, null, null, { places });
  const cam = entries.find((e) => e.label === 'Stage Cam');
  assert.deepEqual([cam.num, cam.rn, cam.router], ['B.12', 11, B]);
  assert.equal(rankFind(entries, 'b.12')[0], cam);
  assert.equal(rankFind(entries, '12', ['source'])[0], cam);
  let g = select(createGrid({ rows: 1, cols: 2, cells: [] }), 1, 1);
  g = applyFind(g, findPlan(g, [cam]));
  assert.deepEqual(
    g.cells.map((c) => [c.kind, c.n, c.router]),
    [['source', 11, B]],
  );
  // The editor names each cell from its own router.
  const names = cellNamesFrom(nm, places);
  assert.equal(names.of({ kind: 'source', n: 11, router: B }), 'Stage Cam');
  assert.equal(names.of({ kind: 'destination', n: 2, router: A }), 'MON 3');
  assert.equal(names.routerName('destination', 3, B), 'Stage Mon');
});
