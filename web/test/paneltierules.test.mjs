import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  tiesOn,
  crossing,
  planOf,
  withTies,
  affectedLines,
  acrossOffText,
  linksOf,
  reservedOn,
  setReserve,
  heldBy,
  tiesKey,
  storedClash,
  bare,
} from '../paneltierules.js';
import { pressPreset, createState } from '../panelmodel.js';
import { pressRefusal, skippedText, refusalText } from '../gridpanelrights.js';
import { envFrom, errorText } from '../panelrules.js';
import { SPAN } from '../routerids.js';

test('only a Reserved setting lets a panel cross routers', () => {
  assert.ok(tiesOn({ tieLines: { mode: 'reserved', reserved: ['t1'] } }));
  assert.ok(!tiesOn({ tieLines: { mode: 'off' } }));
  assert.ok(!tiesOn({}));
  assert.ok(!tiesOn(null));
  assert.ok(
    crossing([
      { src: 1, dst: 2 },
      { src: 3, dst: SPAN + 4 },
    ]),
  );
  assert.ok(!crossing([{ src: SPAN + 1, dst: SPAN + 2 }]));
});

test('a press across two routers is refused at Off, with its reason, and a Diagonal skips it', () => {
  const off = { protectedDsts: new Set() };
  assert.deepEqual(pressRefusal({ src: 1, dst: SPAN + 2 }, off), { code: 'across-routers', src: 1, dst: SPAN + 2 });
  assert.equal(pressRefusal({ src: SPAN + 1, dst: SPAN + 2 }, off), null);
  const names = { source: (s) => `S${s}`, destination: (d) => `D${d}` };
  assert.equal(skippedText([{ src: 1, dst: SPAN + 2 }], names), 'Skipped a route across routers: tie lines are Off for this panel');
  const err = { code: 'across-routers', src: 1, dst: 2 };
  assert.equal(refusalText(err, names), 'S1 and D2 are on two routers. Tie lines are Off for this panel.');
  assert.equal(errorText(err, names), refusalText(err, names));
  assert.equal(acrossOffText('A', 'B'), 'A and B are on two routers. Tie lines are Off for this panel.');
  // At Reserved the press stages; the server uses the Reserved tie lines.
  const on = envFrom({}, {}, {}, null, true);
  assert.ok(on.ties && !envFrom({}, {}).ties);
  assert.equal(pressRefusal({ src: 1, dst: SPAN + 2 }, on), null);
});

// The 409 tie_line_needed body that a Reserved panel gets: one path for each route across routers.
const body = {
  error: { code: 'tie_line_needed' },
  routes: [
    {
      route: 1,
      paths: [
        {
          cost: 3,
          hops: [
            { router: 'a1', tie: { id: 't2', name: 'TIE 4' }, sends: true, now: { origin: { router: 'a1', n: 3, name: 'CAM 3' }, via: [] } },
            { router: 'b2', tie: null, sends: true },
          ],
          affected: [
            { router: 'b2', n: 20, name: 'B MON 20' },
            { router: 'b2', n: 21, name: '' },
          ],
          others: 1,
        },
      ],
    },
  ],
};

test('the plan gives each route across routers its ties and what the confirm shows', () => {
  const plan = planOf(body);
  assert.deepEqual(plan.get(1).ties, [{ id: 't2', expect: { router: 'a1', n: 3 } }]);
  assert.deepEqual(plan.get(1).affected, [
    { router: 'b2', n: 20 },
    { router: 'b2', n: 21 },
  ]);
  const wire = [
    { src: { router: 'a1', n: 1 }, dst: { router: 'a1', n: 2 }, levels: [1] },
    { src: { router: 'a1', n: 4 }, dst: { router: 'b2', n: 2 }, levels: [1] },
  ];
  const out = withTies(wire, plan);
  assert.equal(out[0], wire[0], 'a route on one router is as it was');
  assert.deepEqual(out[1].ties, plan.get(1).ties);
  assert.deepEqual(out[1].affected, plan.get(1).affected);
  assert.deepEqual(affectedLines(plan), ['Via TIE 4 also changes B MON 20, 21, 1 other']);
  const none = planOf({ routes: [{ route: 0, paths: [{ hops: [{ tie: { id: 't1', name: 'TIE 3' }, now: { nothing: true } }, {}], affected: [] }] }] });
  assert.deepEqual(none.get(0).ties, [{ id: 't1', expect: null }]);
  assert.deepEqual(affectedLines(none), ['Via TIE 3: no other destination changes']);
  assert.equal(planOf({ routes: [{ route: 0, paths: [] }] }), null);
  assert.equal(planOf({}), null);
});

test('the editor Reserves one tie line for each link', () => {
  const list = [
    { id: 't1', name: 'TIE 3', from: { router: 'a', n: 10 }, to: { router: 'b', n: 5 } },
    { id: 't9', name: 'TIE 9', from: { router: 'b', n: 30 }, to: { router: 'a', n: 12 } },
    { id: 't2', name: 'TIE 4', from: { router: 'a', n: 11 }, to: { router: 'b', n: 6 }, reserve: { id: 'p-2', name: 'Studio' } },
    { id: 'hid', name: 'TIE 7', from: null, to: { router: 'b', n: 7 } },
  ];
  const links = linksOf(list);
  assert.deepEqual(
    links.map((l) => [l.key, l.ties.map((t) => t.id)]),
    [
      ['a b', ['t1', 't2']],
      ['b a', ['t9']],
    ],
  );
  let s = setReserve(undefined, links[0], 't1');
  assert.deepEqual(s, { mode: 'reserved', reserved: ['t1'] });
  s = setReserve(s, links[1], 't9');
  s = setReserve(s, links[0], 't2');
  assert.deepEqual(s.reserved, ['t9', 't2']);
  assert.equal(reservedOn(s, links[0]), 't2');
  assert.equal(reservedOn(setReserve(s, links[0], ''), links[0]), '');
  assert.equal(heldBy(list[2], 'p-1'), 'Studio');
  assert.equal(heldBy(list[2], 'p-2'), '', 'its own panel holds it');
  assert.equal(heldBy(list[0], 'p-1'), '');
  assert.equal(tiesKey({ mode: 'reserved', reserved: ['t2', 't1'] }), tiesKey({ mode: 'reserved', reserved: ['t1', 't2'] }));
  assert.equal(tiesKey({ mode: 'off' }), tiesKey(undefined));
});

// A Preset cell's route across routers keeps the preset's tie lines. The panel sends it only on its Reserved tie lines
// (docs/tielines-design.md, Presets, rule 6).
test('a Preset cell keeps its stored tie lines, under the Reserved rule', () => {
  const plan = planOf(body);
  const routes = [
    { dst: 2, src: 1, levels: [1] },
    { dst: SPAN + 2, src: 4, levels: [1], ties: ['t2'] },
  ];
  assert.equal(storedClash(routes, plan), '', 'the stored tie line is the Reserved one');
  assert.match(storedClash([routes[0], { ...routes[1], ties: ['t9'] }], plan), /does not Reserve \(the panel uses TIE 4\)/);
  assert.deepEqual(bare(routes)[1], { dst: SPAN + 2, src: 4, levels: [1] });
  const one = [{ level: 1 }];
  const env = envFrom(
    {
      routes: [],
      levels: one,
      byPlace: [
        { router: 'a1', levels: one },
        { router: 'b2', levels: one },
      ],
    },
    { protected: [] },
    { presets: [{ scope: 'everyone', name: 'Far', id: 'f', items: [{ src: 4, dsts: [SPAN + 2], levels: [1], ties: ['t2'] }] }] },
    null,
    true,
  );
  const st = pressPreset(createState(), { kind: 'preset', scope: 'everyone', name: 'Far' }, env, 0);
  assert.deepEqual(st.staged, [{ dst: SPAN + 2, src: 4, levels: [1], ties: ['t2'] }]);
});
