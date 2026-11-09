// Monitoring text and timing. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { durationText, noticeText, badgeText, deviceFrom, shouldNotify, keepaliveBody, KEEPALIVE_MS } from '../panelwatchrules.js';

test('durations read in the largest sensible unit', () => {
  assert.equal(durationText(1), '1 second');
  assert.equal(durationText(46), '46 seconds');
  assert.equal(durationText(60), '1 minute');
  assert.equal(durationText(125), '2 minutes 5 seconds');
  assert.equal(durationText(3600 + 300), '1 hour 5 minutes');
  assert.equal(durationText(2 * 86400), '2 days');
});

test('a notice names the panel, the device and how long', () => {
  const lost = noticeText({ state: 'lost', name: 'Studio wall', device: 'Lobby', secs: 50, offline: true });
  assert.equal(lost.kind, 'error');
  assert.match(lost.title, /Studio wall is offline/);
  assert.match(lost.text, /Lobby/);
  assert.match(lost.text, /50 seconds/);
  const one = noticeText({ state: 'lost', name: 'Studio wall', device: 'Lobby', secs: 50, offline: false });
  assert.equal(one.kind, 'warn');
  assert.match(one.title, /lost a screen/);
  const back = noticeText({ state: 'back', name: 'Studio wall', device: 'Lobby', secs: 130, offline: false });
  assert.equal(back.kind, 'ok');
  assert.match(back.text, /Lobby is back after 2 minutes 10 seconds/);
});

test('the badge counts panels', () => {
  assert.equal(badgeText([{}]), '1 panel offline');
  assert.equal(badgeText([{}, {}]), '2 panels offline');
});

test('a device name is trimmed and capped as the server caps it', () => {
  assert.equal(deviceFrom('  Lobby '), 'Lobby');
  assert.equal(deviceFrom('x'.repeat(60)).length, 40);
  assert.equal(deviceFrom(null), '');
  assert.deepEqual(JSON.parse(keepaliveBody(' Lobby ')), { device: 'Lobby' });
});

test('the keepalive is 15 seconds', () => assert.equal(KEEPALIVE_MS, 15000));

test('a browser notification fires whenever the person allowed them', () => {
  assert.equal(shouldNotify('granted'), true);
  assert.equal(shouldNotify('denied'), false);
  assert.equal(shouldNotify('default'), false);
});
