// Levels on a Grid panel (gridpanellevels.js): what a press stages and what a crosspoint's pie shows.
import test from 'node:test';
import assert from 'node:assert/strict';
import { stageLevels, offeredLevels, boxFill, pieOn, pieFill } from '../gridpanellevels.js';
import { tallyReply } from './gridpanelfixture.mjs';
import { envFrom } from '../panelrules.js';

test('a press stages the levels in the Levels box', () => {
  assert.deepEqual(stageLevels([2, 3], tallyReply(), null), [2, 3]);
});

test('an invalid or missing box stages nothing', () => {
  assert.deepEqual(stageLevels(null, tallyReply(), null), []);
  assert.deepEqual(stageLevels([], tallyReply(), null), []);
});

test('levels the router does not have or the Rights leave out are not staged', () => {
  assert.deepEqual(stageLevels([1, 3, 9], tallyReply(), { levels: [1, 2] }), [1]);
});

test('no tally yet stages nothing', () => {
  assert.deepEqual(stageLevels([1], null, null), []);
});

test('the box offers only the levels the Rights allow', () => {
  assert.deepEqual(
    offeredLevels(tallyReply(), { levels: [2, 3] }).map((l) => l.level),
    [2, 3],
  );
  assert.deepEqual(offeredLevels(tallyReply(), null).length, 3);
  assert.deepEqual(offeredLevels(tallyReply(), { levels: [] }), []);
  assert.deepEqual(offeredLevels(null, null), []);
});

test("the panel's levels setting fills the box, kept to what is offered", () => {
  assert.deepEqual(boxFill({ levels: [1, 3, 9] }, tallyReply(), { levels: [3, 9] }), [3]);
  assert.equal(boxFill({}, tallyReply(), null), null);
  assert.equal(boxFill({ levels: [] }, tallyReply(), null), null);
});

test("a crosspoint's pie is the levels on which that destination has that source", () => {
  const env = envFrom(
    tallyReply({
      routes: [
        [7, 1, 3],
        [7, 2, 4],
        [7, 3, 3],
      ],
    }),
    {},
  );
  assert.deepEqual(pieOn(env, 3, 7), [1, 3]);
  assert.deepEqual(pieOn(env, 4, 7), [2]);
  assert.deepEqual(pieOn(env, 5, 7), []);
  assert.deepEqual(pieOn(env, 3, 8), []);
});

test('each level has its own fixed slice of the pie, coloured only when on', () => {
  const levels = tallyReply().levels;
  assert.equal(
    pieFill(levels, [1, 3]),
    'conic-gradient(var(--pb-signal-l1) 0turn 0.3333turn, transparent 0.3333turn 0.6667turn, var(--pb-signal-l3) 0.6667turn 1turn)',
  );
  assert.equal(pieFill(levels, []), '');
});
