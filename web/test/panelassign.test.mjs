// Assign: the panels page's list rows and the panel page's switch. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { assignRows, canAssignTo, splitLines, assignOf, assignAddress, assignChangesMode } from '../panelassignrules.js';

const inst = (device, extra = {}) => ({ device, online: true, stream: true, ...extra });

test('rows are the open Sode panel pages, sorted by current panel then name', () => {
  const panels = [
    { id: 'p-2', name: 'Studio B', instances: [inst('Wall'), inst('desk')] },
    { id: 'p-1', name: 'Studio A', instances: [inst('Lobby'), inst('Kiosk', { stream: false }), inst('Gone', { online: false })] },
    { id: 'p-3', name: 'Nobody', instances: [] },
    { id: 'p-4', name: 'Not mine' },
  ];
  assert.deepEqual(assignRows(panels, 'p-2'), [
    { panel: 'p-1', panelName: 'Studio A', device: 'Lobby', here: false },
    { panel: 'p-2', panelName: 'Studio B', device: 'desk', here: true },
    { panel: 'p-2', panelName: 'Studio B', device: 'Wall', here: true },
  ]);
  assert.deepEqual(assignRows([], 'p-1'), []);
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
