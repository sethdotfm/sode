// What the panels page (/panels) groups, labels and offers. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  groupPanels,
  actionsFor,
  deleteStep,
  thumbCells,
  actsLabel,
  actsShort,
  hasClient,
  shareLabel,
  launchHref,
  editHref,
  matches,
  panelScore,
  listKey,
  stepPanel,
  panelStatus,
  shownInstances,
  hiddenShown,
} from '../panelsrules.js';

test('a panel reads All Online, Some Online or All Offline across its instances', () => {
  assert.deepEqual(panelStatus({ instancesOnline: 2, instancesKnown: 2 }), { text: 'All Online', tone: 'ok' });
  assert.deepEqual(panelStatus({ instancesOnline: 1, instancesKnown: 3 }), { text: 'Some Online', tone: 'warn' });
  assert.deepEqual(panelStatus({ instancesOnline: 0, instancesKnown: 3 }), { text: 'All Offline', tone: '' });
  // Nothing open, or an older Sode without the counts.
  assert.deepEqual(panelStatus({ instancesOnline: 0, instancesKnown: 0 }), { text: 'All Offline', tone: '' });
  assert.deepEqual(panelStatus({ online: true }), { text: 'All Offline', tone: '' });
});

test('Open now lists known and watched instances, less the hidden ones', () => {
  const p = {
    instances: [
      { device: 'Lobby', online: true, seen: 'x' },
      { device: 'Desk', online: false, seen: 'y' },
    ],
    watchingInstances: ['lobby', 'studio'],
    hiddenInstances: ['desk', 'gone'],
  };
  assert.deepEqual(
    shownInstances(p).map((i) => i.device),
    ['Lobby', 'studio'],
  );
  assert.deepEqual(hiddenShown(p), ['desk']);
  assert.deepEqual(shownInstances({ ...p, hiddenInstances: ['studio'] }).map((i) => i.device), ['Lobby', 'Desk']);
  // A use-only reader gets no instances at all.
  assert.deepEqual(shownInstances({}), []);
  assert.deepEqual(hiddenShown({}), []);
});

// Both: a client for keyed links, Session mode without one. Keep in step with actsHasClient in internal/app/panelstore.go.
test('a panel holds a client when it acts as an API client or as both', () => {
  assert.equal(hasClient({ acts: 'api' }), true);
  assert.equal(hasClient({ acts: 'both' }), true);
  assert.equal(hasClient({ acts: 'session' }), false);
  assert.equal(hasClient({}), false);
  assert.equal(hasClient(null), false);
});

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
  assert.equal(actsLabel(P('a', 'Ana', { acts: 'both' })), 'Acts as an API client with a key, else as the user');
  assert.equal(actsShort(P('a', 'Ana', { acts: 'both' })), 'API client or the user');
  assert.equal(shareLabel({ mode: 'me' }), 'Only the owner');
  assert.equal(shareLabel({ mode: 'everyone' }), 'Everyone');
  assert.equal(shareLabel({ mode: 'named', users: ['Bo', 'Cy'] }), 'Bo, Cy');
  assert.equal(shareLabel({ mode: 'named', users: [] }), 'Nobody yet');
});

test('Launch opens the panel and Edit opens the editor on it, both relative to Sode root', () => {
  assert.equal(launchHref('p-1'), 'panels/p-1');
  assert.equal(editHref('p-1'), 'panels/editor?panel=p-1');
});

test('the filter is fuzzy over name, short name and owner, best match first inside each group', () => {
  const list = [P('a', 'Ana', { name: 'Studio B' }), P('b', 'Ana', { name: 'Bstudio' }), P('c', 'Ana', { name: 'Lobby', slug: 'wall' })];
  assert.equal(matches(list[0], 'stdo'), true);
  assert.equal(matches(list[2], 'wal'), true);
  assert.equal(matches(list[0], 'zq'), false);
  assert.ok(panelScore(list[0], 'stu') > panelScore(list[1], 'stu'));
  assert.deepEqual(
    groupPanels(list, 'Ana', false, 'stu')[0].panels.map((p) => p.id),
    ['a', 'b'],
  );
  assert.deepEqual(
    groupPanels(list, 'Ana', false, '')[0].panels.map((p) => p.id),
    ['a', 'b', 'c'],
  );
  assert.equal(groupPanels([P('a', 'Bo', { name: 'X' })], 'Ana', false, 'bo')[0].title, 'Shared with me');
});

test('Ctrl/Cmd+Enter launches, Ctrl/Cmd+E edits, and Ctrl/Cmd+J and K step', () => {
  const k = (key, extra = {}, mac = false) => listKey({ key, ...extra }, mac);
  assert.equal(k('Enter', { ctrlKey: true }), 'launch');
  assert.equal(k('e', { ctrlKey: true }), 'edit');
  assert.equal(k('j', { ctrlKey: true }), 'next');
  assert.equal(k('k', { ctrlKey: true }), 'prev');
  assert.equal(k('e', { metaKey: true }, true), 'edit');
  assert.equal(k('e', { metaKey: true }), '', 'Cmd counts on a Mac only');
  assert.equal(k('e'), '', 'a bare key belongs to the page');
  assert.equal(k('Enter'), '');
  assert.equal(k('e', { ctrlKey: true, shiftKey: true }), '');
  assert.equal(k('e', { ctrlKey: true, altKey: true }), '');
  assert.equal(k('e', { ctrlKey: true, defaultPrevented: true }), '');
});

test('stepping moves the highlight and stops at the ends', () => {
  assert.equal(stepPanel(['a', 'b', 'c'], 'a', 1), 'b');
  assert.equal(stepPanel(['a', 'b', 'c'], 'c', 1), 'c');
  assert.equal(stepPanel(['a', 'b', 'c'], 'a', -1), 'a');
  assert.equal(stepPanel(['a', 'b'], 'zz', 1), 'b');
  assert.equal(stepPanel([], 'a', 1), '');
});
