// Assign: the panels page's list rows and the panel page's switch. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  rankAssign,
  curRow,
  stepRow,
  enterRow,
  assignRows,
  assignWays,
  splitLines,
  assignOf,
  assignAddress,
  assignChangesMode,
  signInText,
  uptimeText,
  clashOf,
  assignNameProblem,
  keylessPath,
} from '../panelassignrules.js';

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
    user: 'Ana',
    uptime: 'up 2 minutes',
    here: false,
    timedOut: false,
    assignable: true,
    notShared: false,
    keyless: '',
  });
});

test('a user who cannot open the target is warned only when the device will sign in', () => {
  const panels = (share, owner = 'Cy') => [
    { id: 'p-1', name: 'Studio A', instances: [inst('Lobby', { signIn: 'user', user: 'ana' }), inst('Wall'), inst('Boss', { signIn: 'user', user: 'Bo', trusted: true })] },
    { id: 'p-2', name: 'Desk', owner, share },
  ];
  const warned = (share, bySignIn, owner) =>
    assignRows(panels(share, owner), 'p-2', NOW, bySignIn)
      .filter((r) => r.notShared)
      .map((r) => r.device);
  assert.deepEqual(warned({ mode: 'me' }, true), ['Lobby']);
  assert.deepEqual(warned({ mode: 'me' }, false), []);
  assert.deepEqual(warned({ mode: 'me' }, true, 'Ana'), []);
  assert.deepEqual(warned({ mode: 'everyone' }, true), []);
  assert.deepEqual(warned({ mode: 'named', users: ['Bo', 'Ana'] }, true), []);
  assert.deepEqual(warned({ mode: 'named', users: ['Bo'] }, true), ['Lobby']);
  // A share list left over from named mode reaches nobody.
  assert.deepEqual(warned({ mode: 'me', users: ['Ana'] }, true), ['Lobby']);
});

test('how a target is assigned: its key, its sign-in, or a choice of the two with the key first', () => {
  assert.deepEqual(assignWays({ acts: 'api' }), ['client']);
  assert.deepEqual(assignWays({ acts: 'session' }), ['user']);
  assert.deepEqual(assignWays({}), ['user']);
  assert.deepEqual(assignWays({ acts: 'both' }), ['client', 'user']);
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

test('stream lines split across chunks', () => {
  assert.deepEqual(splitLines('', '{"a":1}\n{"b"'), { lines: ['{"a":1}'], rest: '{"b"' });
  assert.deepEqual(splitLines('{"b"', ':2}\n\n'), { lines: ['{"b":2}'], rest: '' });
  assert.deepEqual(splitLines('', 'no end'), { lines: [], rest: 'no end' });
});

test('an assign line is read, anything else is not', () => {
  const line = JSON.stringify({ t: 'assign', panel: 'p-0000000b', slug: 'studio-b', instance: 'Lobby', key: 'sode_abc' });
  assert.deepEqual(assignOf(line), { panel: 'p-0000000b', slug: 'studio-b', instance: 'Lobby', key: 'sode_abc' });
  assert.deepEqual(assignOf(JSON.stringify({ t: 'assign', panel: 'p-0000000b', instance: 'Lobby', key: 'sode_abc' })).slug, '');
  // A panel that acts as a user sends no key: the page opens it and signs in there.
  assert.deepEqual(assignOf(JSON.stringify({ t: 'assign', panel: 'p-0000000b', instance: 'Lobby', signIn: 'user' })), {
    panel: 'p-0000000b',
    slug: '',
    instance: 'Lobby',
    key: '',
  });
  assert.equal(assignOf('{"online":true,"graceSeconds":8}'), null);
  assert.equal(assignOf('{"t":"assign","panel":"p-0000000b"}'), null);
  assert.equal(assignOf('{"t":"assign","panel":"p-0000000b","instance":"Lobby"}'), null);
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
  // No key to carry: the address drops the old one and the page signs in.
  assert.equal(assignAddress({ ...msg, key: '' }, { search: '', hash: '#key=old&kiosk=1' }, false).href, 'panels/studio-b#kiosk=1');
});

test('a name chosen after a clash replaces the address name, so a reload keeps it', () => {
  const msg = { panel: 'p-0000000b', slug: 'studio-b', instance: 'Lobby 2', key: 'sode_abc' };
  assert.equal(assignAddress(msg, { search: '?instance=Lobby&x=1', hash: '' }, true).href, 'panels/studio-b?instance=Lobby+2&x=1');
  assert.equal(new URLSearchParams(assignAddress(msg, { search: '?instance=Lobby', hash: '' }, true).href.split('?')[1]).get('instance'), 'Lobby 2');
  assert.equal(assignAddress(msg, { search: '', hash: '' }, true).href, 'panels/studio-b');
});

test('a name clash offers the suggested name, and an edit is checked before it is sent', () => {
  assert.equal(clashOf({ status: 409, code: 'name_taken', suggest: 'Lobby 2' }), 'Lobby 2');
  assert.equal(clashOf({ status: 409, code: 'name_taken' }), '');
  assert.equal(clashOf({ status: 409, code: 'conflict', suggest: 'Lobby 2' }), null);
  assert.equal(clashOf(new Error('down')), null);

  assert.equal(assignNameProblem('Lobby 2', 'Lobby'), '');
  assert.equal(assignNameProblem('  Lobby 2 ', 'Lobby'), '');
  assert.match(assignNameProblem('  ', 'Lobby'), /Name the instance/);
  assert.match(assignNameProblem('LOBBY', 'Lobby'), /already/);
  assert.equal(assignNameProblem('x'.repeat(40), 'Lobby'), '');
  assert.match(assignNameProblem('x'.repeat(41), 'Lobby'), /40/);
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
  const two = assignRows(
    [
      { id: 'p-1', name: 'Wall', instances: [inst('Lobby')] },
      { id: 'p-2', name: 'Studio', instances: [inst('Wall')] },
    ],
    'p-3',
    NOW,
  );
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

test('keyless pages are rows too, assignable to the panel they show, and timed out without a stream', () => {
  const kl = (device, extra = {}) => ({ keyless: `k-${device}`, device, addr: '192.0.2.9', signIn: 'none', stream: true, streamSince: at(30), seen: at(1), ...extra });
  const panels = [{ id: 'p-1', name: 'Studio A', instances: [inst('Lobby')], keyless: [kl('Hall', { signIn: 'ended' }), kl('Gone', { stream: false, seen: at(600) })] }];
  const rows = assignRows(panels, 'p-1', NOW);
  assert.deepEqual(
    rows.map((r) => [r.device, r.keyless, r.state, r.uptime, r.here, r.assignable]),
    [
      ['Gone', 'k-Gone', 'Not signed in', 'last seen 10 minutes ago', false, false],
      ['Hall', 'k-Hall', 'Key ended', 'up under a minute', false, true],
      ['Lobby', '', "Signed in as the panel's client", 'up 2 minutes', true, false],
    ],
  );
  assert.equal(new Set(rows.map((r) => r.key)).size, 3);
  // A keyless row never warns: nobody is signed in there.
  assert.equal(
    assignRows(panels, 'p-2', NOW, true).some((r) => r.notShared),
    false,
  );
});

test('the keyless announcement names the page and says when its key ended', () => {
  assert.equal(keylessPath('studio-a', ' Front desk ', false), 'api/v1/panels/studio-a/instances/keyless?instance=Front+desk');
  assert.equal(keylessPath('p-1', '', true), 'api/v1/panels/p-1/instances/keyless?ended=1');
});
