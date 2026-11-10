// Push target selection and the Push modal's wording. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  groupKey,
  pushCandidates,
  pushGroups,
  pushTargets,
  groupState,
  setGroup,
  pushBody,
  setTargets,
  pushSummary,
  checkpointLabel,
  canSaveSet,
} from '../panelpushrules.js';

const P = (id, name, extra = {}) => ({ id, name, owner: 'Ana', canEdit: true, ...extra });
const wall = [P('p-1', 'EVS 1'), P('p-2', 'EVS 2'), P('p-3', 'EVS 3'), P('p-4', 'EVS 4'), P('p-5', 'Studio B'), P('p-6', 'EVS 5', { canEdit: false })];

test('a group is the name without its trailing number', () => {
  assert.equal(groupKey('EVS 1'), 'EVS');
  assert.equal(groupKey('Wall-12'), 'Wall');
  assert.equal(groupKey('Studio B'), 'Studio B');
  assert.equal(groupKey('21'), '21');
});

test('only panels the caller may change are offered, and never the source', () => {
  assert.deepEqual(
    pushCandidates(wall, 'p-1').map((p) => p.id),
    ['p-2', 'p-3', 'p-4', 'p-5'],
  );
});

test('panels group by name and a lone panel stands apart', () => {
  const { groups, single } = pushGroups(pushCandidates(wall, 'p-1'));
  assert.deepEqual(
    groups.map((g) => [g.key, g.ids]),
    [['EVS', ['p-2', 'p-3', 'p-4']]],
  );
  assert.deepEqual(
    single.map((p) => p.id),
    ['p-5'],
  );
});

test('numbers sort as numbers inside a group', () => {
  const { groups } = pushGroups([P('a', 'EVS 10'), P('b', 'EVS 2')]);
  assert.deepEqual(groups[0].ids, ['b', 'a']);
});

test('EVS 1 to EVS 4: a group and one-by-one picks give each panel once', () => {
  assert.deepEqual(pushTargets(wall, 'p-5', { groups: ['EVS'] }), ['p-1', 'p-2', 'p-3', 'p-4']);
  assert.deepEqual(pushTargets(wall, 'p-5', { groups: ['evs'], ids: ['p-2', 'p-5'] }), ['p-1', 'p-2', 'p-3', 'p-4']);
  assert.deepEqual(pushTargets(wall, 'p-1', { ids: ['p-3', 'p-2'] }), ['p-2', 'p-3']);
});

test('a target the caller cannot change, the source and unknown ids are never chosen', () => {
  assert.deepEqual(pushTargets(wall, 'p-1', { groups: ['EVS'], ids: ['p-6', 'p-1', 'p-404'] }), ['p-2', 'p-3', 'p-4']);
  assert.deepEqual(pushTargets(wall, 'p-1', {}), []);
});

test('a group box is all, some or none, and ticking it sets every panel', () => {
  const g = { ids: ['p-2', 'p-3'] };
  assert.equal(groupState(new Set(), g), 'none');
  assert.equal(groupState(new Set(['p-2']), g), 'some');
  assert.equal(groupState(new Set(['p-2', 'p-3', 'p-9']), g), 'all');
  assert.deepEqual([...setGroup(new Set(['p-9']), g, true)], ['p-9', 'p-2', 'p-3']);
  assert.deepEqual([...setGroup(new Set(['p-2', 'p-3']), g, false)], []);
});

test('the body is the targets and does not alias the selection', () => {
  const ids = ['p-2'];
  const b = pushBody(ids);
  b.targets.push('x');
  assert.deepEqual(ids, ['p-2']);
});

test('a set pushes to its ticked panels, or to all of its panels when none is ticked', () => {
  const set = { entries: [{ panel: 'p-1' }, { panel: 'p-2' }] };
  assert.deepEqual(setTargets(set, new Set()), ['p-1', 'p-2']);
  assert.deepEqual(setTargets(set, new Set(['p-2', 'p-9'])), ['p-2']);
  assert.deepEqual(setTargets(set, new Set(['p-9'])), ['p-1', 'p-2']);
});

test('the summary names the panels and says what was kept', () => {
  assert.equal(
    pushSummary({ pushed: [{ panel: 'p-1' }, { panel: 'p-2' }, { panel: 'p-3' }] }, wall),
    'Pushed to EVS 1, EVS 2 and EVS 3. A checkpoint of each was kept.',
  );
  assert.equal(
    pushSummary({ pushed: [{ panel: 'p-1' }], skipped: [{ panel: 'p-x' }] }, wall),
    'Pushed to EVS 1. A checkpoint of each was kept. Skipped p-x, which no longer exists.',
  );
  assert.equal(pushSummary({ pushed: [] }, wall), 'Nothing was pushed.');
});

test('a checkpoint line gives when, who and what it holds', () => {
  assert.equal(checkpointLabel({ at: '2026-11-08T18:11:29Z', by: 'Ana', rows: 4, cols: 8, cells: 1 }), '2026-11-08 18:11, Ana, 4 by 8, 1 cell');
});

test('a set needs a name and at least one ticked panel', () => {
  assert.equal(canSaveSet('  ', new Set(['p-1'])), false);
  assert.equal(canSaveSet('Show', new Set()), false);
  assert.equal(canSaveSet('Show', new Set(['p-1'])), true);
});

test('a Push names each target whose artwork did not all fit', () => {
  assert.equal(
    pushSummary({ pushed: [{ panel: 'p-1', artSkipped: ['a-1', 'a-2'] }, { panel: 'p-2' }, { panel: 'p-3', artSkipped: ['a-3'] }] }, wall),
    'Pushed to EVS 1, EVS 2 and EVS 3. A checkpoint of each was kept. 2 images did not fit under the 64 MB artwork total of EVS 1 and were left out. 1 image did not fit under the 64 MB artwork total of EVS 3 and was left out.',
  );
});
