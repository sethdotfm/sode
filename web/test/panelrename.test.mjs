// Renaming an instance from the panels page: the Open now row's control, the name checks, and the panel page's
// handling of the stream's rename line. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { renameState, renameProblem, renameOf, renamedSearch, assignOf, renameRefusal } from '../panelassignrules.js';

test('only a Sode panel page can be renamed, and only while it holds its stream', () => {
  assert.equal(renameState({ device: 'Lobby', page: true, stream: true }), 'ready');
  assert.equal(renameState({ device: 'Lobby', page: true, stream: false }), 'offline');
  // A keepalive-only API client and a watched name never heard have no stream to tell.
  assert.equal(renameState({ device: 'Lobby', online: true }), '');
  assert.equal(renameState({ device: 'lobby', online: false, seen: '' }), '');
});

test('the name checks', () => {
  assert.equal(renameProblem('  '), 'Name the instance, such as Lobby tablet.');
  assert.equal(renameProblem('x'.repeat(41)), 'Use at most 40 characters.');
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

test('a refusal for want of rights asks for the Advanced Settings PIN', () => {
  const err = (status, code, message) => Object.assign(new Error(message), { status, code });
  assert.deepEqual(renameRefusal(err(403, 'trusted_required', 'only the owner')), { why: 'only the owner', pin: true });
  assert.deepEqual(renameRefusal(err(409, 'conflict', 'Lobby is offline')), { why: 'Lobby is offline', pin: false });
});
