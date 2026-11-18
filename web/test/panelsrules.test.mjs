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
  launchNote,
  actsSavedNote,
  PUBLIC_WARNING,
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
  actsSwitch,
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
  assert.deepEqual(
    shownInstances({ ...p, hiddenInstances: ['studio'] }).map((i) => i.device),
    ['Lobby', 'Desk'],
  );
  // A use-only reader gets no instances at all.
  assert.deepEqual(shownInstances({}), []);
  assert.deepEqual(hiddenShown({}), []);
});

// Both: a client for keyed links, Session mode without one. Keep in step with actsHasClient in internal/app/panelstore.go.
test('a panel holds a client when it acts as an API client, as both or as a Public panel', () => {
  assert.equal(hasClient({ acts: 'api' }), true);
  assert.equal(hasClient({ acts: 'both' }), true);
  assert.equal(hasClient({ acts: 'public' }), true);
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
  assert.equal(actsLabel(P('a', 'Ana', { acts: 'public' })), 'Acts as an API client for anyone with the link');
  assert.equal(actsShort(P('a', 'Ana', { acts: 'public' })), 'Public');
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

test('choosing an Authentication mode says what was saved', () => {
  assert.match(actsSavedNote('session'), /signed-in user.*every key are gone/);
  assert.match(actsSavedNote('both'), /plain link opens it as the signed-in user/);
  assert.match(actsSavedNote('api'), /Link with key/);
  assert.match(actsSavedNote('public'), /anyone with the link can use the panel/i);
  assert.equal(actsSavedNote('nonsense'), 'Saved.');
});

test('the Access tab warns that sharing does not limit a Public panel', () => {
  assert.match(PUBLIC_WARNING, /anyone with the link/i);
  assert.match(PUBLIC_WARNING, /sharing/i);
});

test('only a panel that may sign in says so after how it acts', () => {
  assert.equal(launchNote({ acts: 'session' }), ' (asks for a PIN at launch)');
  assert.equal(launchNote({}), ' (asks for a PIN at launch)');
  assert.equal(launchNote({ acts: 'both' }), ' (a link without a key asks for a PIN)');
  assert.equal(launchNote({ acts: 'api' }), '');
  assert.equal(launchNote({ acts: 'public' }), '');
});

// Each switch of Authentication mode as apiPanelUpdate does it. A new client gets no key. A switch to User ends the
// client (pruneKeys then ends its keys and setup links). API to Both or back keeps the client and its keys.
test('each Authentication mode switch says what it does to keys, and only those that touch the client ask for a Confirm', () => {
  const s2a = actsSwitch('session', 'api');
  assert.equal(s2a.confirm, true);
  assert.equal(s2a.name, 'API');
  assert.match(s2a.text, /makes the panel its own API client/);
  assert.match(s2a.text, /no keys yet/);
  assert.match(s2a.text, /Link with key/);
  assert.match(s2a.text, /plain link stops opening/);

  const s2b = actsSwitch('session', 'both');
  assert.equal(s2b.confirm, true);
  assert.match(s2b.text, /makes the panel its own API client/);
  assert.match(s2b.text, /no keys yet/);
  assert.match(s2b.text, /plain link still opens it as the signed-in user/);

  for (const from of ['api', 'both']) {
    const s = actsSwitch(from, 'session');
    assert.equal(s.confirm, true, from);
    assert.match(s.text, /ends the panel's API client, every instance key and every setup link/);
    assert.match(s.text, /signs out/);
    assert.match(s.text, /cannot be undone/);
  }

  for (const [from, to] of [
    ['api', 'both'],
    ['both', 'api'],
  ]) {
    const s = actsSwitch(from, to);
    assert.equal(s.confirm, false, `${from} to ${to}`);
    assert.match(s.text, /keeps the panel's API client and every instance key/);
  }
  assert.match(actsSwitch('api', 'both').text, /plain link opens it as the signed-in user/);
  assert.match(actsSwitch('both', 'api').text, /plain link stops opening/);

  // Not a switch: nothing to say or confirm. A panel with no acts yet reads as User.
  assert.equal(actsSwitch('api', 'api'), null);
  assert.equal(actsSwitch(undefined, 'session'), null);
  assert.equal(actsSwitch(undefined, 'api').confirm, true);

  // Public opens the panel to anyone, so going there always asks; leaving it for API or Both says the Public keys end.
  for (const from of ['session', 'api', 'both']) assert.equal(actsSwitch(from, 'public').confirm, true, from);
  assert.match(actsSwitch('session', 'public').text, /makes the panel its own API client/);
  assert.match(actsSwitch('api', 'public').text, /keeps the panel's API client/);
  assert.match(actsSwitch('both', 'public').text, /anyone with the plain link/);
  assert.match(actsSwitch('public', 'api').text, /lose their Public key/);
  assert.equal(actsSwitch('public', 'session').confirm, true);
});

test('a Grid panel on the list says how many sources and destinations it holds, or all', async () => {
  const { gridSize, gridThumb, GRID_MEDIUM_FROM, GRID_LARGE_FROM, GRID_SQUARES } = await import('../panelsrules.js');
  assert.equal(gridSize({ type: 'grid', sources: { members: [1, 2, 3] }, destinations: { members: [4] } }), 'Grid, 3 sources by 1 destination');
  assert.equal(gridSize({ type: 'grid', sources: { members: 'all', hide: [2] } }), 'Grid, all sources by all destinations');
  // The picture: a Source bar across the top, a Destination bar down the side and a diagonal of routes, in one of three sizes.
  const edge = (n) => ({ members: Array.from({ length: n }, (_, i) => i + 1) });
  const t = (s, d) => gridThumb({ sources: edge(s), destinations: edge(d) });
  assert.equal(t(3, 2).size, 'small');
  assert.equal(t(GRID_MEDIUM_FROM - 1, 1).size, 'small');
  assert.equal(t(1, GRID_MEDIUM_FROM).size, 'medium');
  assert.equal(t(GRID_LARGE_FROM - 1, 2).size, 'medium');
  assert.equal(t(2, GRID_LARGE_FROM).size, 'large');
  assert.deepEqual(
    t(40, 12).tiles.map((x) => x.label),
    ['4', '0', '1', '2'],
  );
  const all = gridThumb({ sources: { members: 'all' }, destinations: edge(4) });
  assert.equal(all.size, 'large');
  assert.deepEqual(
    all.tiles.map((x) => x.label),
    ['A', 'L', 'L', '4'],
  );
  // One character per square, centred. Large has 8 squares, so "750" leaves 5 spare and takes a half square; "40" in Medium leaves 3 and does too.
  const s750 = gridThumb({ sources: edge(750), destinations: edge(2) });
  assert.deepEqual(
    s750.tiles.filter((x) => x.kind === 'source').map((x) => [x.label, x.c, x.cs, x.half]),
    [
      ['7', 4, 1, true],
      ['5', 5, 1, true],
      ['0', 6, 1, true],
    ],
  );
  assert.deepEqual(
    t(40, 12)
      .tiles.filter((x) => x.kind === 'source')
      .map((x) => [x.label, x.c, x.half]),
    [
      ['4', 3, true],
      ['0', 4, true],
    ],
  );
  // The Destination bar reads from the bottom up, so its first character takes the lowest square.
  assert.deepEqual(
    t(40, 12)
      .tiles.filter((x) => x.kind === 'destination')
      .map((x) => [x.label, x.r]),
    [
      ['1', 4],
      ['2', 3],
    ],
  );
  for (const k of ['small', 'medium', 'large']) {
    const g = k === 'small' ? t(3, 2) : k === 'medium' ? t(20, 20) : t(200, 8);
    assert.equal(g.cols, GRID_SQUARES[k] + 1);
    assert.equal(g.cells.filter((c) => c.kind === 'source').length, g.cols - 1);
    assert.equal(g.cells.filter((c) => c.kind === 'destination').length, g.rows - 1);
    // Each character has its own square, centred in its bar. Spare squares are even on both sides, or differ by a half square.
    for (const kind of ['source', 'destination']) {
      const ts = g.tiles.filter((x) => x.kind === kind);
      const at = (x) => (kind === 'source' ? x.c : x.r);
      const len = (x) => (kind === 'source' ? x.cs : x.rs);
      const lo = Math.min(...ts.map(at));
      const hi = Math.max(...ts.map((x) => at(x) + len(x) - 1));
      const gap = lo - 2 - (g.cols - hi);
      assert.ok(Math.abs(gap) <= 1, `${k} ${kind}`);
      assert.equal(
        ts.every((x) => x.half),
        Math.abs(gap) === 1,
      );
    }
    assert.ok(g.cells.filter((c) => c.kind === 'on').every((c) => c.c >= 2 && c.c <= g.cols && c.r >= 2 && c.r <= g.rows));
  }
});
