// Rights levels and the level subset of a panel name their router. Keep in step with config.RouterLevels and
// internal/app/panellevels.go.
import test from 'node:test';
import assert from 'node:assert/strict';
import { levelsOn, putLevels } from '../routerlevels.js';
import { allowed, planOutside, draftOf, rightsOf } from '../rightsrules.js';
import { rightsAllow, envFrom } from '../panelrules.js';
import { createState, pressDestination, pressSource, errorOf } from '../panelmodel.js';
import { offeredLevels, boxFill } from '../gridpanellevels.js';
import { setSetting, rightsRows } from '../sidebarrules.js';

test('levels by router: a router the levels do not name is not limited', () => {
  const lv = { a: [1], b: [] };
  assert.deepEqual(levelsOn(lv, 'a'), [1]);
  assert.deepEqual(levelsOn(lv, 'b'), []);
  assert.equal(levelsOn(lv, 'c'), null);
  assert.equal(levelsOn(null, 'a'), null);
  assert.deepEqual(levelsOn([2], 'a'), [2], 'a plain list is the one router');
  assert.deepEqual(putLevels(lv, 'a', [1, 2]), { a: [1, 2], b: [] });
  assert.deepEqual(putLevels(lv, 'b', null), { a: [1] });
  assert.equal(putLevels({ a: [1] }, 'a', null), null);
  assert.deepEqual(putLevels(null, '', [3]), [3]);
});

test('a Rights level limit on router A does not limit router B', () => {
  const r = { levels: { a: [1] } };
  for (const check of [allowed, rightsAllow]) {
    assert.equal(check(r, 'levels', 1, 'a'), true);
    assert.equal(check(r, 'levels', 2, 'a'), false);
    assert.equal(check(r, 'levels', 2, 'b'), true);
  }
  const plan = (router) => ({ verb: 'patch', items: [{ dst: { n: 5, router }, groups: [{ src: { n: 5, router }, levels: [2] }] }] });
  const uid = (end) => end.router;
  assert.equal(planOutside(r, plan('a'), 1, String, uid), 'level 2 is outside your rights');
  assert.equal(planOutside(r, plan('b'), 1, String, uid), '');
});

test('the Rights editor edits the levels of its router and keeps the others', () => {
  const d = draftOf({ levels: { a: [1, 2], b: [3] } }, 1, [], 'a');
  assert.equal(d.levels.all, false);
  assert.equal(d.levels.text, '1-2');
  d.levels.text = '2';
  assert.deepEqual(rightsOf(d, 1, 4).rights.levels, { a: [2], b: [3] });
  d.levels.all = true;
  assert.deepEqual(rightsOf(d, 1, 4).rights.levels, { b: [3] });
  const open = draftOf({ levels: { b: [3] } }, 1, [], 'a');
  assert.equal(open.levels.all, true, 'a limit on B leaves A open');
  open.levels.all = false;
  open.levels.text = '';
  assert.deepEqual(rightsOf(open, 1, 4).rights.levels, { a: [], b: [3] });
  assert.equal(rightsOf(draftOf({}, 1, [], 'a'), 1, 4).rights.levels, undefined);
  assert.deepEqual(rightsRows({ levels: { a: [1, 2], b: [3] } })[2], ['Levels', '3']);
});

test('a panel uses the Rights levels and the level subset of its router', () => {
  const tally = { router: 'a', levels: [1, 2, 3].map((level) => ({ level, label: String(level), swatch: 'video' })), routes: [] };
  const env = (rights) => envFrom(tally, {}, {}, rights);
  assert.equal(env(null).router, 'a');
  const press = (e, settings) => pressSource(pressDestination(createState(settings), { kind: 'destination', n: 0 }, e, 1), { kind: 'source', n: 4 }, e, 1);
  assert.deepEqual(press(env({ levels: { a: [2], b: [1] } })).staged[0].levels, [2]);
  assert.deepEqual(press(env({ levels: { b: [1] } })).staged[0].levels, [1, 2, 3]);
  assert.deepEqual(press(env(null), { levels: { a: [3], b: [1] } }).staged[0].levels, [3]);
  assert.equal(errorOf(press(env({ levels: { a: [1] } }), { levels: { a: [3] } }), 1).code, 'no-common-levels');

  assert.deepEqual(
    offeredLevels(tally, { levels: { a: [1, 3], b: [2] } }).map((l) => l.level),
    [1, 3],
  );
  assert.equal(offeredLevels(tally, { levels: { b: [2] } }).length, 3);
  assert.deepEqual(boxFill({ levels: { a: [2], b: [1] } }, tally, null), [2]);
  assert.equal(boxFill({ levels: { b: [1] } }, tally, null), null);
});

test('the editor stores the level subset by router, sorted, and drops empty lists', () => {
  assert.deepEqual(setSetting({}, 'levels', { a: [3, 1], b: [] }).levels, { a: [1, 3] });
  assert.equal('levels' in setSetting({ levels: { a: [1] } }, 'levels', { b: [] }), false);
  assert.deepEqual(setSetting({}, 'levels', [2, 1]).levels, [1, 2]);
  assert.equal('levels' in setSetting({ levels: [1] }, 'levels', null), false);
});
