// What the panels page (/panels) groups, labels and offers. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { groupPanels, actionsFor, deleteStep, thumbCells, actsLabel, actsShort, shareLabel, launchHref, editHref, matches } from '../panelsrules.js';

const P = (id, owner, extra = {}) => ({
  id,
  name: `Panel ${id}`,
  owner,
  acts: 'session',
  share: { mode: 'me' },
  rows: 4,
  cols: 8,
  cells: [],
  canEdit: false,
  ...extra,
});

test('a user sees My panels first, then Shared with me, and no empty group', () => {
  const list = [P('a', 'Bo'), P('b', 'Ana', { canEdit: true }), P('c', 'ana', { canEdit: true })];
  const g = groupPanels(list, 'Ana', false);
  assert.deepEqual(
    g.map((x) => [x.title, x.panels.map((p) => p.id)]),
    [
      ['My panels', ['b', 'c']],
      ['Shared with me', ['a']],
    ],
  );
  assert.deepEqual(
    groupPanels([P('a', 'Bo')], 'Ana', false).map((x) => x.title),
    ['Shared with me'],
  );
});

test('a Trusted user sees every panel grouped by owner, their own first', () => {
  const list = [P('a', 'Cy'), P('b', 'Bo'), P('c', 'Ana'), P('d', 'Bo')];
  const g = groupPanels(list, 'Ana', true);
  assert.deepEqual(
    g.map((x) => [x.title, x.panels.map((p) => p.id)]),
    [
      ['My panels', ['c']],
      ['Bo', ['b', 'd']],
      ['Cy', ['a']],
    ],
  );
});

test('the filter matches name or owner, ignoring case', () => {
  assert.equal(matches(P('a', 'Bo', { name: 'Studio A' }), 'studio'), true);
  assert.equal(matches(P('a', 'Bo', { name: 'Studio A' }), 'BO'), true);
  assert.equal(matches(P('a', 'Bo', { name: 'Studio A' }), 'zz'), false);
  assert.equal(groupPanels([P('a', 'Bo', { name: 'Studio A' })], 'Ana', false, 'zz').length, 0);
});

test('an editable panel offers Edit, Share and Delete, a use-only panel offers Save a copy and nothing destructive', () => {
  assert.deepEqual(actionsFor(P('a', 'Ana', { canEdit: true })), ['launch', 'edit', 'share', 'push', 'delete']);
  assert.deepEqual(actionsFor(P('a', 'Bo')), ['launch', 'copy']);
});

test('Delete needs a second press on the same panel, and another panel starts over', () => {
  assert.deepEqual(deleteStep('', 'a'), { armed: 'a', go: false });
  assert.deepEqual(deleteStep('a', 'a'), { armed: '', go: true });
  assert.deepEqual(deleteStep('a', 'b'), { armed: 'b', go: false });
});

test('the mini picture draws whole cells only, with their kind', () => {
  const p = P('a', 'Ana', {
    rows: 2,
    cols: 2,
    cells: [
      { r: 1, c: 1, kind: 'source', n: 3 },
      { r: 1, c: 2, kind: 'take', cs: 1, rs: 2 },
      { r: 2, c: 1, kind: 'destination' },
      { r: 5, c: 5, kind: 'take' },
    ],
  });
  assert.deepEqual(thumbCells(p), [
    { r: 1, c: 1, rs: 1, cs: 1, kind: 'source' },
    { r: 1, c: 2, rs: 2, cs: 1, kind: 'take' },
  ]);
});

test('labels say who a panel acts as and who may use it', () => {
  assert.equal(actsLabel(P('a', 'Ana')), 'Acts as the user');
  assert.equal(actsLabel(P('a', 'Ana', { acts: 'api' })), 'Acts as an API client');
  assert.equal(actsShort(P('a', 'Ana')), 'signs in as the user');
  assert.equal(actsShort(P('a', 'Ana', { acts: 'api' })), 'API client');
  assert.equal(shareLabel({ mode: 'me' }), 'Only the owner');
  assert.equal(shareLabel({ mode: 'everyone' }), 'Everyone');
  assert.equal(shareLabel({ mode: 'named', users: ['Bo', 'Cy'] }), 'Bo, Cy');
  assert.equal(shareLabel({ mode: 'named', users: [] }), 'Nobody yet');
});

test('Launch opens the panel and Edit opens the editor on it, both relative to Sode root', () => {
  assert.equal(launchHref('p-1'), 'panels/p-1');
  assert.equal(editHref('p-1'), 'panels/editor?panel=p-1');
});
