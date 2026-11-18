// Rights, protection and other sources on a Grid panel (gridpanelrights.js).
import test from 'node:test';
import assert from 'node:assert/strict';
import { otherSources, otherText, pressRefusal, stageable, skippedText, refusalText, leftOut, keepLive, leftOutText, leftOutLine } from '../gridpanelrights.js';
import { createStage, pressCrosspoint, routesOf } from '../gridpanelstage.js';
import { envFrom, namesFrom } from '../panelrules.js';
import { namesReply, tallyReply } from './gridpanelfixture.mjs';

test('the skipped note names the protected rows, three at most', () => {
  const names = { destination: (d) => `D${d}` };
  assert.equal(skippedText([], names), '');
  assert.equal(skippedText([{ src: 0, dst: 1 }], names), 'Skipped a protected destination: D1');
  const many = [1, 2, 3, 4, 5].map((dst) => ({ src: 0, dst }));
  assert.equal(skippedText(many, names), 'Skipped protected destinations: D1, D2, D3 +2');
});

const envOf = (routes, prot = []) => envFrom(tallyReply({ routes }), { protected: prot.map(([dst, level]) => ({ dst, level })) });

test('a destination whose current source is in the grid has no other source', () => {
  assert.deepEqual(otherSources(envOf([[0, 1, 1]]), 0, new Set([0, 1])), []);
});

test('the sources a destination has that are not in the grid, by level, each once', () => {
  const env = envOf([
    [0, 1, 7],
    [0, 2, 1],
    [0, 3, 7],
    [0, 4, 9],
    [1, 1, 8],
  ]);
  assert.deepEqual(otherSources(env, 0, new Set([0, 1])), [7, 9]);
});

test('nothing routed (-1) is not another source', () => {
  assert.deepEqual(otherSources(envOf([[0, 1, -1]]), 0, new Set([0])), []);
});

test('the Destination bar names the first other source, plus "+N" for the rest', () => {
  const names = namesFrom(
    namesReply({
      sources: [
        [7, 'SAT 1'],
        [9, 'SAT 2'],
        [4, 'GFX'],
      ],
    }),
  );
  assert.equal(otherText([], names), '');
  assert.equal(otherText([7], names), 'SAT 1');
  assert.equal(otherText([7, 9, 4], names), 'SAT 1 +2');
});

test('any protected level makes the whole destination protected, and a press on it stages nothing', () => {
  const env = envOf([], [[1, 3]]);
  const names = namesFrom(namesReply());
  const err = pressRefusal({ src: 0, dst: 1 }, env);
  assert.deepEqual(err, { code: 'protected', dst: 1 });
  assert.equal(refusalText(err, names), 'MON B is protected');
  assert.equal(pressRefusal({ src: 0, dst: 0 }, env), null);
  assert.equal(refusalText(null, names), '');
});

test('a Diagonal skips protected destinations and stages the rest; all protected stages nothing and says why', () => {
  const env = envOf([], [[1, 1]]);
  const pairs = [
    { src: 0, dst: 0 },
    { src: 1, dst: 1 },
    { src: 2, dst: 2 },
  ];
  assert.deepEqual(stageable(pairs, env), {
    pairs: [
      { src: 0, dst: 0 },
      { src: 2, dst: 2 },
    ],
    err: null,
  });
  assert.deepEqual(stageable([{ src: 0, dst: 1 }], env), { pairs: [], err: { code: 'protected', dst: 1 } });
  assert.deepEqual(stageable([], env), { pairs: [], err: null });
});

// Staged routes against polled state. A world is what the last polls said: env, the viewer's Rights, GET /names.
const staged = (...presses) => presses.reduce((st, [src, dst, levels = [1, 2]]) => pressCrosspoint(st, { src, dst }, levels).st, createStage());
const world = ({ routes = [], prot = [], rights = null, names = namesReply(), levels } = {}) => ({
  env: envFrom(tallyReply({ routes, ...(levels ? { levels } : {}) }), { protected: prot.map(([dst, level]) => ({ dst, level })) }),
  rights,
  names,
});

test('staged routes that still apply are all live', () => {
  const st = staged([0, 0], [1, 1]);
  assert.deepEqual(leftOut(st, world()), []);
  assert.equal(keepLive(st, world()), st);
});

test('a staged destination that becomes protected is left out with the reason, and dropped at the next stage or confirm', () => {
  const st = staged([0, 0], [2, 1]);
  const w = world({ prot: [[1, 2]] });
  assert.deepEqual(leftOut(st, w), [{ src: 2, dst: 1, levels: [1, 2], why: 'protected' }]);
  assert.deepEqual(routesOf(keepLive(st, w)), [{ src: 0, dst: 0, levels: [1, 2] }]);
});

test('a staged route whose destination, source or level leaves the Rights is left out', () => {
  const st = staged([0, 0], [2, 1]);
  assert.deepEqual(leftOut(st, world({ rights: { dsts: [0] } })), [{ src: 2, dst: 1, levels: [1, 2], why: 'rights' }]);
  assert.deepEqual(leftOut(st, world({ rights: { sources: [2] } })), [{ src: 0, dst: 0, levels: [1, 2], why: 'rights' }]);
  // Only the level outside the Rights is left out; the rest of the route stays.
  const w = world({ rights: { levels: [1] } });
  assert.deepEqual(leftOut(st, w), [
    { src: 0, dst: 0, levels: [2], why: 'rights' },
    { src: 2, dst: 1, levels: [2], why: 'rights' },
  ]);
  assert.deepEqual(routesOf(keepLive(st, w)), [
    { src: 0, dst: 0, levels: [1] },
    { src: 2, dst: 1, levels: [1] },
  ]);
});

test('a staged route whose source, destination or level the router no longer reports is left out', () => {
  const st = staged([0, 0], [2, 1]);
  const noSrc2 = namesReply({
    sources: [
      [0, 'CAM 1'],
      [1, 'CAM 2'],
    ],
  });
  assert.deepEqual(leftOut(st, world({ names: noSrc2 })), [{ src: 2, dst: 1, levels: [1, 2], why: 'gone' }]);
  const noDst0 = namesReply({ destinations: [[1, 'MON B']] });
  assert.deepEqual(leftOut(st, world({ names: noDst0 })), [{ src: 0, dst: 0, levels: [1, 2], why: 'gone' }]);
  assert.deepEqual(
    leftOut(st, world({ levels: [1] })).map((x) => [x.dst, x.levels, x.why]),
    [
      [0, [2], 'gone'],
      [1, [2], 'gone'],
    ],
  );
});

test('before the first names and tally reads, nothing is left out', () => {
  const st = staged([0, 0]);
  assert.deepEqual(leftOut(st, { env: envFrom({}, {}), rights: null, names: {} }), []);
});

test('the reason names what changed', () => {
  const names = namesFrom(namesReply());
  assert.equal(leftOutText([], names), '');
  // The confirm lists each route left out on its own line.
  assert.equal(leftOutLine({ src: 0, dst: 0, levels: [1], why: 'protected' }, names), 'CAM 1 to MON A, since MON A is protected');
  assert.equal(leftOutText([{ src: 2, dst: 1, levels: [1], why: 'protected' }], names), 'Left out of the take: VT 1 to MON B, since MON B is protected');
  assert.equal(
    leftOutText([{ src: 2, dst: 1, levels: [1], why: 'rights' }], names),
    "Left out of the take: VT 1 to MON B, since it is outside this panel's Rights",
  );
  assert.equal(leftOutText([{ src: 2, dst: 1, levels: [1], why: 'gone' }], names), 'Left out of the take: VT 1 to MON B, since the router does not report it');
  assert.equal(
    leftOutText(
      [
        { src: 2, dst: 1, levels: [1], why: 'protected' },
        { src: 0, dst: 0, levels: [1], why: 'gone' },
      ],
      names,
    ),
    'Left out of the take: 2 staged routes that are protected, outside Rights or not on the router',
  );
});
