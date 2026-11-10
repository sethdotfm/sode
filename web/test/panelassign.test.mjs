// Assign: the panels page's list rows and the panel page's switch. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { rankAssign, curRow, stepRow, enterRow, assignRows, canAssignTo, splitLines, assignOf, assignAddress, assignChangesMode, signInText, uptimeText } from '../panelassignrules.js';

const NOW = Date.parse('2026-11-10T12:00:00Z');
const at = (secsAgo) => new Date(NOW - secsAgo * 1000).toISOString();
const inst = (device, extra = {}) => ({
  device,
  online: true,
  stream: true,
  page: true,
  addr: '192.0.2.7',
  signIn: 'client',
  streamSince: at(125),
  seen: at(1),
  ...extra,
});

test('rows are the Sode panel pages, sorted by current panel then name', () => {
  const panels = [
    { id: 'p-2', name: 'Studio B', instances: [inst('Wall'), inst('desk')] },
    { id: 'p-1', name: 'Studio A', instances: [inst('Lobby'), inst('Hall', { stream: false, page: false })] },
    { id: 'p-3', name: 'Nobody', instances: [] },
    { id: 'p-4', name: 'Not mine' },
  ];
  assert.deepEqual(
    assignRows(panels, 'p-2', NOW).map((r) => [r.panelName, r.device, r.here]),
    [
      ['Studio A', 'Lobby', false],
      ['Studio B', 'desk', true],
      ['Studio B', 'Wall', true],
    ],
  );
  assert.deepEqual(assignRows([], 'p-1', NOW), []);
});

test('a row shows the address, uptime since its stream and the sign-in state in words', () => {
  const [r] = assignRows([{ id: 'p-1', name: 'Studio A', instances: [inst('Lobby', { signIn: 'user', user: 'Ana' })] }], 'p-2', NOW);
  assert.deepEqual(r, {
    key: 'p-1\nLobby',
    panel: 'p-1',
    panelName: 'Studio A',
    device: 'Lobby',
    addr: '192.0.2.7',
    state: 'Signed in as Ana',
    uptime: 'up 2 minutes',
    here: false,
    timedOut: false,
    assignable: true,
  });
});

test('sign-in state words', () => {
  assert.equal(signInText({ signIn: 'client' }), "Signed in as the panel's client");
  assert.equal(signInText({ signIn: 'user', user: 'Ana' }), 'Signed in as Ana');
  assert.equal(signInText({ signIn: 'ended' }), 'Key ended');
  assert.equal(signInText({ signIn: 'none' }), 'Not signed in');
  assert.equal(signInText({}), 'Not signed in');
});

test('uptime counts whole minutes, so a poll does not redraw the list every time', () => {
  assert.equal(uptimeText(30), 'under a minute');
  assert.equal(uptimeText(61), '1 minute');
  assert.equal(uptimeText(119), '1 minute');
  assert.equal(uptimeText(3 * 3600 + 5 * 60 + 9), '3 hours 5 minutes');
});

test('the instance on the chosen panel is greyed', () => {
  const [r] = assignRows([{ id: 'p-1', name: 'Studio A', instances: [inst('Lobby')] }], 'p-1', NOW);
  assert.equal(r.here, true);
  assert.equal(r.assignable, false);
});

test('a page whose stream ended is greyed with last seen and cannot be assigned', () => {
  const gone = inst('Lobby', { stream: false, online: false, streamSince: undefined, seen: at(3 * 3600 + 30) });
  const [r] = assignRows([{ id: 'p-1', name: 'Studio A', instances: [gone] }], 'p-2', NOW);
  assert.equal(r.timedOut, true);
  assert.equal(r.assignable, false);
  assert.equal(r.uptime, 'last seen 3 hours ago');
  assert.equal(r.addr, '192.0.2.7');
  // In the grace after a close it still counts online, but there is no stream to send on.
  const grace = inst('Desk', { stream: false, streamSince: undefined, seen: at(3) });
  const [g] = assignRows([{ id: 'p-1', name: 'Studio A', instances: [grace] }], 'p-2', NOW);
  assert.deepEqual([g.timedOut, g.assignable, g.uptime], [true, false, 'last seen under a minute ago']);
});

test('only a panel that acts as an API client can be assigned to', () => {
  assert.equal(canAssignTo({ acts: 'api' }), true);
  assert.equal(canAssignTo({ acts: 'both' }), true);
  assert.equal(canAssignTo({ acts: 'session' }), false);
});

test('stream lines split across chunks', () => {
  assert.deepEqual(splitLines('', '{"a":1}\n{"b"'), { lines: ['{"a":1}'], rest: '{"b"' });
  assert.deepEqual(splitLines('{"b"', ':2}\n\n'), { lines: ['{"b":2}'], rest: '' });
  assert.deepEqual(splitLines('', 'no end'), { lines: [], rest: 'no end' });
});

test('an assign line is read, anything else is not', () => {
  const line = JSON.stringify({ t: 'assign', panel: 'p-0000000b', slug: 'studio-b', instance: 'Lobby', key: 'sode_abc' });
  assert.deepEqual(assignOf(line), { panel: 'p-0000000b', slug: 'studio-b', instance: 'Lobby', key: 'sode_abc' });
  assert.deepEqual(assignOf(JSON.stringify({ t: 'assign', panel: 'p-0000000b', instance: 'Lobby', key: 'sode_abc' })).slug, '');
  assert.equal(assignOf('{"online":true,"graceSeconds":8}'), null);
  assert.equal(assignOf('{"t":"assign","panel":"p-0000000b"}'), null);
  assert.equal(assignOf('not json'), null);
});

test('the new address keeps the instance name and kiosk, and drops the old key', () => {
  const msg = { panel: 'p-0000000b', slug: 'studio-b', instance: 'Lobby', key: 'sode_abc' };
  assert.deepEqual(assignAddress(msg, { search: '?instance=Lobby', hash: '#key=old&kiosk=1' }, true), {
    seg: 'studio-b',
    href: 'panels/studio-b?instance=Lobby#kiosk=1',
  });
  assert.deepEqual(assignAddress({ ...msg, slug: '' }, { search: '', hash: '#setup=t' }, true), { seg: 'p-0000000b', href: 'panels/p-0000000b' });
  // Storage that does not keep the key leaves it in the fragment, the only place a reload can find it.
  assert.deepEqual(assignAddress(msg, { search: '', hash: '#kiosk=1' }, false), { seg: 'studio-b', href: 'panels/studio-b#key=abc&kiosk=1' });
});

test('an assign that lands on another Authentication mode is told apart', () => {
  assert.equal(assignChangesMode({ acts: 'session' }, { acts: 'api' }), true);
  assert.equal(assignChangesMode({ acts: 'api' }, { acts: 'api' }), false);
  assert.equal(assignChangesMode(undefined, { acts: 'both' }), true);
});

const FIND = [
  { id: 'p-1', name: 'Studio A', instances: [inst('Lobby', { addr: '10.1.2.3' }), inst('Hall', { signIn: 'user', user: 'Ana', addr: '10.9.9.9' })] },
  { id: 'p-2', name: 'Master', instances: [inst('Wall', { addr: '10.4.4.4', signIn: 'ended', stream: false, seen: at(60) })] },
];
const rows = () => assignRows(FIND, 'p-3', NOW);
const names = (rs) => rs.map((r) => r.device);

test('Find ranks by instance name, current panel, address and sign-in state', () => {
  assert.deepEqual(names(rankAssign(rows(), 'lob')), ['Lobby']);
  assert.deepEqual(names(rankAssign(rows(), 'master')), ['Wall']);
  assert.deepEqual(names(rankAssign(rows(), '10.9')), ['Hall']);
  assert.deepEqual(names(rankAssign(rows(), 'ana')), ['Hall']);
  assert.deepEqual(names(rankAssign(rows(), 'key ended')), ['Wall']);
  assert.deepEqual(names(rankAssign(rows(), 'zzzz')), []);
});

test('Find needs every word to hit and puts a name hit first', () => {
  assert.deepEqual(names(rankAssign(rows(), 'studio ana')), ['Hall']);
  assert.deepEqual(names(rankAssign(rows(), 'studio zzz')), []);
  const two = assignRows([{ id: 'p-1', name: 'Wall', instances: [inst('Lobby')] }, { id: 'p-2', name: 'Studio', instances: [inst('Wall')] }], 'p-3', NOW);
  assert.deepEqual(names(rankAssign(two, 'wall')), ['Wall', 'Lobby']);
});

test('an empty Find keeps the list order', () => {
  assert.deepEqual(rankAssign(rows(), '  '), rows());
});

test('J and K step the highlight and stop at the ends; the first row is the default', () => {
  const r = rows();
  assert.equal(curRow(r, ''), r[0].key);
  assert.equal(curRow(r, 'gone'), r[0].key);
  assert.equal(stepRow(r, '', 1), r[1].key);
  assert.equal(stepRow(r, r.at(-1).key, 1), r.at(-1).key);
  assert.equal(stepRow(r, r[0].key, -1), r[0].key);
  assert.equal(stepRow([], '', 1), '');
});

test('Enter picks the highlighted row only when it can be assigned', () => {
  const r = rows();
  const wall = r.find((x) => x.device === 'Wall');
  assert.equal(enterRow(r, wall.key), null);
  assert.equal(enterRow(r, r.find((x) => x.device === 'Lobby').key).device, 'Lobby');
  assert.equal(enterRow(assignRows(FIND, 'p-1', NOW), assignRows(FIND, 'p-1', NOW).find((x) => x.device === 'Hall').key), null);
  assert.equal(enterRow([], ''), null);
});
