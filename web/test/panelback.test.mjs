// A Public panel page that comes back by itself, and the Open now list's Reload and keyless rename. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { lineType, backWaitMs, publicAskPath, reloadAllState } from '../panelbackrules.js';
import { streamState } from '../panelassignrules.js';
import { pollFail, PUBLIC_STOPPED } from '../panelkeyrules.js';

test('the stream lines the page acts on', () => {
  assert.equal(lineType('{"t":"public"}'), 'public');
  assert.equal(lineType('{"t":"reload"}'), 'reload');
  assert.equal(lineType('{"keyless":true}'), '');
  assert.equal(lineType('{"t":"assign","panel":"p-1"}'), '');
  assert.equal(lineType('not json'), '');
});

test('each page waits a random 0 to 5 s, so a wall of tablets does not ask at once', () => {
  assert.equal(
    backWaitMs(() => 0),
    0,
  );
  assert.equal(
    backWaitMs(() => 0.5),
    2500,
  );
  assert.equal(
    backWaitMs(() => 0.9999999),
    5000,
  );
});

test('only the ask after a public line is marked, for its Log row', () => {
  assert.equal(publicAskPath('p-0011aabb', false), 'api/v1/panels/p-0011aabb/public');
  assert.equal(publicAskPath('lobby', true), 'api/v1/panels/lobby/public?back=1');
});

test('a stopped Public page says it comes back by itself', () => {
  assert.equal(PUBLIC_STOPPED, 'Not Public now. Reconnects when Public again.');
  const f = pollFail(401, 'key_revoked', undefined, false, true);
  assert.deepEqual(f, { stop: true, forget: false, text: PUBLIC_STOPPED });
  // A link's key that ended still says to ask for a new link.
  assert.match(pollFail(401, 'key_revoked', undefined, true, false).text, /new link/);
});

test('Reload and rename reach only a live stream; an offline row says Offline', () => {
  assert.equal(streamState({ device: 'Lobby', page: true, stream: true }), 'ready');
  assert.equal(streamState({ device: 'Lobby', page: true, stream: false }), 'offline');
  assert.equal(streamState({ device: 'Lobby', online: true }), '');
  assert.equal(streamState({ keyless: 'ab', stream: false }), 'offline');
  assert.equal(reloadAllState({ instances: [{ page: true, stream: false }], keyless: [{ keyless: 'ab', stream: true }] }), 'ready');
  assert.equal(reloadAllState({ instances: [{ page: true, stream: false }], keyless: [] }), 'offline');
  assert.equal(reloadAllState({ instances: [] }), '');
});
