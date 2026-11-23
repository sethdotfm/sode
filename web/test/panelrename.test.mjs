import test from 'node:test';
import assert from 'node:assert/strict';
import { streamState, renameProblem, renameOf, renamedSearch, assignOf, renameRefusal } from '../panelassignrules.js';

test('only a Sode panel page can be renamed, and only while it holds its stream', () => {
  assert.equal(streamState({ device: 'Lobby', page: true, stream: true }), 'ready');
  assert.equal(streamState({ device: 'Lobby', page: true, stream: false }), 'offline');
  // An API client that only sends keepalives has no stream. A watched name that never connected has none also.
  assert.equal(streamState({ device: 'Lobby', online: true }), '');
  assert.equal(streamState({ device: 'lobby', online: false, seen: '' }), '');
});

test('the name checks', () => {
  assert.equal(renameProblem('  '), 'Name the instance.');
  assert.equal(renameProblem(' Front desk '), '');
});

test('the stream line', () => {
  assert.equal(renameOf(JSON.stringify({ t: 'rename', instance: 'Front desk' })), 'Front desk');
  assert.equal(renameOf(JSON.stringify({ t: 'rename', instance: '  ' })), '');
  assert.equal(renameOf('{"online":true,"graceSeconds":8}'), '');
  assert.equal(renameOf('not json'), '');
  assert.equal(assignOf(JSON.stringify({ t: 'rename', instance: 'Front desk' })), null);
});

test('a page named by its link keeps the new name in the address', () => {
  assert.equal(renamedSearch('?instance=Lobby&kiosk=1', 'Front desk'), '?instance=Front+desk&kiosk=1');
  assert.equal(renamedSearch('?instance=Lobby', 'Gate'), '?instance=Gate');
});

test('a refusal gives its reason and asks for no PIN', () => {
  const err = (status, code, message) => Object.assign(new Error(message), { status, code });
  assert.deepEqual(renameRefusal(err(403, 'trusted_required', 'only the owner')), { why: 'only the owner' });
  assert.deepEqual(renameRefusal(err(409, 'conflict', 'Lobby is offline')), { why: 'Lobby is offline' });
});
